package runtimeexecutor

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/claude"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/codex"
	"agent-platform/backend/internal/agentruntime/containerprocess"
	"agent-platform/backend/internal/agentruntime/hermes"
	"agent-platform/backend/internal/agentruntime/openclaw"
	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/runworker"
	"agent-platform/backend/internal/sandbox"
	"agent-platform/backend/internal/secretcrypto"
	"agent-platform/backend/internal/workspacefs"
)

type Executor struct {
	config       platformconfig.Config
	box          *secretcrypto.Box
	materializer credentials.Materializer
	objects      objectstore.Provider
	warm         *containerprocess.WarmManager
}

func New(config platformconfig.Config, box *secretcrypto.Box, objects objectstore.Provider, warm *containerprocess.WarmManager) (*Executor, error) {
	if box == nil || objects == nil || warm == nil {
		return nil, fmt.Errorf("Runtime Executor encryption, Object Store, and warm container services are required")
	}
	if err := ensureDirectory(config.Worker.CredentialTempRoot); err != nil {
		return nil, err
	}
	if err := ensureDirectory(config.Workspace.Root); err != nil {
		return nil, err
	}
	owner := &credentials.Owner{UID: config.Worker.SandboxUID, GID: config.Worker.SandboxGID}
	return &Executor{config: config, box: box, objects: objects, warm: warm, materializer: credentials.Materializer{Root: config.Worker.CredentialTempRoot, Owner: owner}}, nil
}

func (executor *Executor) Execute(ctx context.Context, job application.ExecutionJob, progress application.ProgressRecorder) (result application.ExecutionResult, returnErr error) {
	startedAt := time.Now()
	var runtimeStartedAt time.Time
	var runtimeFinishedAt time.Time
	defer func() {
		attributes := []any{"run_id", job.ID, "job_kind", job.Kind, "total_ms", time.Since(startedAt).Milliseconds(), "success", returnErr == nil}
		if !runtimeStartedAt.IsZero() {
			attributes = append(attributes, "setup_ms", runtimeStartedAt.Sub(startedAt).Milliseconds())
		}
		if !runtimeFinishedAt.IsZero() {
			attributes = append(attributes, "runtime_ms", runtimeFinishedAt.Sub(runtimeStartedAt).Milliseconds(), "post_runtime_ms", time.Since(runtimeFinishedAt).Milliseconds())
		}
		slog.InfoContext(context.WithoutCancel(ctx), "Runtime execution timing", attributes...)
	}()
	if job.Kind == application.JobMCPTest {
		return executor.testMCP(ctx, job)
	}
	runtimeConfig, ok := executor.config.Worker.Runtimes[string(job.Snapshot.RuntimeEngine)]
	if !ok || !runtimeConfig.Available {
		return result, fmt.Errorf("Runtime %s is unavailable", job.Snapshot.RuntimeEngine)
	}
	variables, err := executor.environment(job)
	if err != nil {
		return result, err
	}
	files, extensionVariables, redactValues, err := executor.extensionFiles(ctx, job)
	if err != nil {
		return result, err
	}
	for name, value := range extensionVariables {
		if _, exists := variables[name]; exists {
			return result, fmt.Errorf("MCP credential environment collides with %s", name)
		}
		variables[name] = value
	}
	containerName, slot, err := executor.warmSlot(job, runtimeConfig)
	if err != nil {
		return result, err
	}
	lease, err := executor.warm.Checkout(ctx, containerName)
	if err != nil {
		return result, err
	}
	var environment *credentials.Environment
	workspace := slot.workspace
	nativeState := ""
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		returnErr = errors.Join(returnErr, lease.Release(cleanupCtx))
		if environment != nil {
			returnErr = errors.Join(returnErr, environment.Cleanup())
		}
		returnErr = errors.Join(returnErr, os.RemoveAll(workspace), os.RemoveAll(slot.scratch))
		if nativeState != "" {
			returnErr = errors.Join(returnErr, os.RemoveAll(nativeState))
		}
	}()
	environment, err = executor.materializer.CreateAt(credentials.Request{Ref: job.ID, Variables: variables, Files: files, RedactValues: redactValues}, slot.credentials)
	if err != nil {
		return result, err
	}
	workspace, persistent, baseline, err := executor.stageWorkspaceAt(job, slot.workspace)
	if err != nil {
		return result, err
	}
	if err := prepareRuntimeScratch(slot.scratch, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
		return result, err
	}
	attachmentPaths, err := executor.materializeAttachments(ctx, job, slot.scratch)
	if err != nil {
		return result, err
	}
	nativeState, nativePersistent, err := executor.nativeStateDirectoriesAt(job, runtimeConfig, slot.nativeState)
	if err != nil {
		return result, err
	}
	containerConfig := containerprocess.Config{
		Image: runtimeConfig.ImageDigest, RuntimeCommand: string(job.Snapshot.RuntimeEngine), RunID: strings.TrimPrefix(containerName, "agent-runtime-warm-"),
		Runtime: executor.config.Sandbox.Runtime, WorkspaceDirectory: workspace, ContainerWorkspace: "/workspace",
		CredentialDirectory: environment.Directory(), NativeStateDirectory: nativeState, ScratchDirectory: slot.scratch, PublicEgressNetwork: executor.config.Sandbox.EgressNetwork,
		ResolverConfigFile: executor.config.Sandbox.ResolverConfig, Egress: sandbox.EgressPublic,
		Limits: sandbox.Limits{CPUs: 2, MemoryBytes: 4 << 30, PIDs: 512, TempBytes: 2 << 30},
		UID:    executor.config.Worker.SandboxUID, GID: executor.config.Worker.SandboxGID,
	}
	runProcess, err := lease.Start(ctx, containerConfig)
	if err != nil {
		return result, err
	}
	sink := &eventSink{runID: job.ID, job: job, progress: progress}
	adapter, err := runtimeAdapter(job.Snapshot.RuntimeEngine, cliadapter.Config{
		ExpectedVersion: runtimeConfig.CLIVersion, RunProcess: runProcess,
		ScratchRoot:          slot.scratch,
		OutputSink:           processharness.NewRedactingSink(environment.Redactor(), discardOutput{}),
		VerifiedCapabilities: map[agentruntime.Capability]bool{agentruntime.CapabilityStreaming: true, agentruntime.CapabilityNativeResume: runtimeConfig.NativeResume},
	})
	if err != nil {
		return result, err
	}
	executionCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	if _, err := adapter.Describe(executionCtx); err != nil {
		return result, err
	}
	instruction := buildInstruction(job, attachmentPaths)
	checkpoint := job.CheckpointRef
	if !runtimeConfig.NativeResume {
		checkpoint = ""
	}
	redactor := environment.Redactor()
	runtimeStartedAt = time.Now()
	runtimeResult, err := runworker.New(adapter).Execute(executionCtx, agentruntime.ExecuteRequest{
		RunID: job.ID, WorkspacePath: workspace, Instruction: instruction, Model: job.Snapshot.ProviderModel.ModelID,
		ModelEndpoint: job.Snapshot.ProviderModel.Endpoint, ModelProvider: job.Snapshot.ProviderModel.ProviderType, ModelProtocols: job.Snapshot.ProviderModel.Protocols, CheckpointRef: checkpoint, EnvironmentRef: job.ID, MCPConfigPath: mcpConfigPath(job),
	}, agentruntime.NewRedactingEventSink(redactor, sink))
	runtimeFinishedAt = time.Now()
	if err != nil {
		return result, err
	}
	if nativeState != "" {
		nativeSessions := filepath.Join(nativeState, "sessions")
		if err := sanitizeNativeState(nativeSessions, redactor); err != nil {
			return result, err
		}
		if err := mergeSuccessfulWorkspace(nativePersistent, nativeSessions); err != nil {
			return result, fmt.Errorf("persist native Runtime state: %w", err)
		}
	}
	finalMessage := string(redactor.Bytes([]byte(runtimeResult.FinalMessage)))
	var artifacts []application.ExecutionArtifact
	if persistent != "" {
		used, err := executionWorkspaceSize(workspace)
		if err != nil {
			return result, err
		}
		if used > workspacefs.WorkspaceLimit {
			return result, fmt.Errorf("Runtime output exceeds the 1 GiB Workspace limit")
		}
		artifacts, err = executor.persistChangedFiles(executionCtx, job, workspace, baseline, redactor)
		if err != nil {
			return result, err
		}
	}
	if persistent != "" {
		if err := preparePersistentWorkspaceTree(workspace, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
			return result, fmt.Errorf("prepare persistent Workflow Workspace: %w", err)
		}
		if err := mergeSuccessfulWorkspace(persistent, workspace); err != nil {
			return result, err
		}
	}
	return application.ExecutionResult{FinalMessage: finalMessage, CheckpointRef: runtimeResult.CheckpointRef, Artifacts: artifacts}, nil
}

func prepareRuntimeScratch(path string, uid, gid int) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("clear Runtime scratch directory: %w", err)
	}
	if err := os.MkdirAll(path, 0o750); err != nil {
		return fmt.Errorf("create Runtime scratch directory: %w", err)
	}
	return prepareSandboxDirectory(path, uid, gid)
}

type warmSlot struct {
	workspace   string
	credentials string
	nativeState string
	scratch     string
}

func (executor *Executor) warmSlot(job application.ExecutionJob, runtime platformconfig.RuntimeEngineConfig) (string, warmSlot, error) {
	scope := ""
	switch job.Kind {
	case application.JobSession:
		scope = "session:" + job.OwnerID + ":" + job.SessionID
	case application.JobWorkflow:
		scope = "workflow:" + job.OwnerID + ":" + job.WorkflowID
	default:
		return "", warmSlot{}, fmt.Errorf("job kind %q cannot use a warm Runtime container", job.Kind)
	}
	name, err := containerprocess.WarmContainerName(scope, string(job.Snapshot.RuntimeEngine), runtime.ImageDigest)
	if err != nil {
		return "", warmSlot{}, err
	}
	token := strings.TrimPrefix(name, "agent-runtime-warm-")
	workspaceRoot := filepath.Join(filepath.Clean(executor.config.Workspace.Root), ".runtime-containers", token)
	credentialRoot := filepath.Join(filepath.Clean(executor.config.Worker.CredentialTempRoot), ".runtime-containers", token)
	return name, warmSlot{
		workspace: filepath.Join(workspaceRoot, "workspace"), credentials: filepath.Join(credentialRoot, "credentials"),
		nativeState: filepath.Join(workspaceRoot, "native-state"), scratch: filepath.Join(workspaceRoot, "scratch"),
	}, nil
}

func (executor *Executor) nativeStateDirectoriesAt(job application.ExecutionJob, runtime platformconfig.RuntimeEngineConfig, temporary string) (string, string, error) {
	if job.Kind != application.JobSession || !runtime.NativeResume || job.Snapshot.RuntimeEngine != workspacedomain.RuntimeCodex {
		return "", "", nil
	}
	persistent, err := workspacefs.NativeSessionStatePath(executor.config.Workspace.Root, job.OwnerID, job.SessionID, string(job.Snapshot.RuntimeEngine))
	if err != nil {
		return "", "", err
	}
	hiddenRoot := filepath.Join(filepath.Clean(executor.config.Workspace.Root), ".native-session-state")
	parent := filepath.Dir(persistent)
	current := hiddenRoot
	relativeParent, _ := filepath.Rel(hiddenRoot, parent)
	for _, segment := range append([]string{""}, strings.Split(relativeParent, string(filepath.Separator))...) {
		if segment != "" && segment != "." {
			current = filepath.Join(current, segment)
		}
		if err := os.MkdirAll(current, 0o750); err != nil {
			return "", "", fmt.Errorf("create native Runtime state parent: %w", err)
		}
		if err := ensureSandboxDirectoryOwnership(current, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
			return "", "", err
		}
	}
	if err := os.RemoveAll(temporary); err != nil {
		return "", "", fmt.Errorf("clear native Runtime state staging: %w", err)
	}
	if err := os.MkdirAll(temporary, 0o750); err != nil {
		return "", "", fmt.Errorf("stage native Runtime state: %w", err)
	}
	if err := prepareSandboxDirectory(temporary, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
		_ = os.RemoveAll(temporary)
		return "", "", err
	}
	sessions := filepath.Join(temporary, "sessions")
	if err := os.MkdirAll(sessions, 0o750); err != nil {
		_ = os.RemoveAll(temporary)
		return "", "", fmt.Errorf("prepare native Runtime sessions: %w", err)
	}
	if err := copyTree(persistent, sessions); err != nil {
		_ = os.RemoveAll(temporary)
		return "", "", fmt.Errorf("restore native Runtime state: %w", err)
	}
	if err := prepareNativeStateTree(temporary, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
		_ = os.RemoveAll(temporary)
		return "", "", err
	}
	return temporary, persistent, nil
}

func ensureSandboxDirectoryOwnership(path string, uid, gid int) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) == uid && int(stat.Gid) == gid && info.Mode().Perm() == 0o750 {
		return nil
	}
	return prepareSandboxDirectory(path, uid, gid)
}

func prepareNativeStateTree(root string, uid, gid int) error {
	return prepareOwnedTree(root, uid, gid, "native Runtime state")
}

func preparePersistentWorkspaceTree(root string, uid, gid int) error {
	return prepareOwnedTree(root, uid, gid, "Workspace")
}

func prepareOwnedTree(root string, uid, gid int, label string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s contains a symbolic link", label)
		}
		if path == root {
			return nil
		}
		mode := os.FileMode(0o600)
		if entry.IsDir() {
			mode = 0o750
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
		if err := os.Chown(path, uid, gid); err != nil {
			return err
		}
		return nil
	})
}

func sanitizeNativeState(root string, redactor *credentials.Redactor) error {
	for _, name := range []string{"config.toml", "auth.json"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove transient Codex state: %w", err)
		}
	}
	var total int64
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("native Runtime state contains unsupported file type")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > 256<<20 {
			return fmt.Errorf("native Runtime state exceeds 256 MiB")
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		temporary, err := os.CreateTemp(filepath.Dir(path), ".redacted-native-*")
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(temporary, redactor.Reader(input))
		closeErr := errors.Join(input.Close(), temporary.Close())
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(temporary.Name())
			return errors.Join(copyErr, closeErr)
		}
		if err := os.Chmod(temporary.Name(), 0o600); err != nil {
			_ = os.Remove(temporary.Name())
			return err
		}
		if err := os.Rename(temporary.Name(), path); err != nil {
			_ = os.Remove(temporary.Name())
			return err
		}
		return nil
	})
}

func executionWorkspaceSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Workspace symlinks are not allowed")
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
			if total > workspacefs.WorkspaceLimit {
				return fmt.Errorf("Runtime output exceeds the 1 GiB Workspace limit")
			}
		}
		return nil
	})
	return total, err
}

func (executor *Executor) environment(job application.ExecutionJob) (map[string]string, error) {
	secret, err := executor.box.Decrypt(job.Snapshot.ProviderModel.APIKeyCiphertext, "model-provider:"+job.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("decrypt Model Provider credential: %w", err)
	}
	defer clear(secret)
	variables := make(map[string]string)
	for _, variable := range job.Snapshot.Environment {
		if !variable.Secret {
			variables[variable.Name] = variable.Value
		}
	}
	if len(job.Snapshot.EnvironmentSecretCiphertext) > 0 {
		plaintext, err := executor.box.Decrypt(job.Snapshot.EnvironmentSecretCiphertext, "workflow-environment:"+job.OwnerID)
		if err != nil {
			return nil, fmt.Errorf("decrypt Workflow environment: %w", err)
		}
		defer clear(plaintext)
		if err := json.Unmarshal(plaintext, &variables); err != nil {
			return nil, fmt.Errorf("decode Workflow secret environment: %w", err)
		}
	}
	variables["ANTHROPIC_API_KEY"] = string(secret)
	variables["ANTHROPIC_BASE_URL"] = job.Snapshot.ProviderModel.Endpoint
	variables["OPENAI_API_KEY"] = string(secret)
	variables["OPENAI_BASE_URL"] = job.Snapshot.ProviderModel.Endpoint
	if len(job.Snapshot.MCPServers) > 0 {
		variables["AGENT_WORKSPACE_MCP_CONFIG"] = "/run/agent-credentials/extensions/mcp.json"
	}
	return variables, nil
}

func (executor *Executor) extensionFiles(ctx context.Context, job application.ExecutionJob) (map[string][]byte, map[string]string, [][]byte, error) {
	files, extensionVariables, redactValues, err := executor.nativeMCPFiles(job)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, skill := range job.Snapshot.Skills {
		body, object, err := executor.objects.Get(ctx, skill.ObjectKey)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("load Skill %q: %w", skill.Name, err)
		}
		archive, readErr := io.ReadAll(io.LimitReader(body, 50<<20+1))
		closeErr := body.Close()
		if readErr != nil || closeErr != nil {
			return nil, nil, nil, errors.Join(readErr, closeErr)
		}
		digest := sha256.Sum256(archive)
		actual := hex.EncodeToString(digest[:])
		if len(archive) > 50<<20 || actual != skill.SHA256 || object.SHA256 != skill.SHA256 {
			return nil, nil, nil, fmt.Errorf("Skill %q archive integrity check failed", skill.Name)
		}
		reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open Skill %q archive: %w", skill.Name, err)
		}
		for _, entry := range reader.File {
			name := filepath.ToSlash(filepath.Clean(entry.Name))
			if entry.FileInfo().IsDir() {
				continue
			}
			if name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") || entry.Mode()&os.ModeSymlink != 0 {
				return nil, nil, nil, fmt.Errorf("Skill %q contains an unsafe path", skill.Name)
			}
			content, err := readZipFile(entry)
			if err != nil {
				return nil, nil, nil, err
			}
			files[filepath.ToSlash(filepath.Join("skills", skill.ID, name))] = content
		}
	}
	return files, extensionVariables, redactValues, nil
}

func mcpConfigPath(job application.ExecutionJob) string {
	if len(job.Snapshot.MCPServers) == 0 {
		return ""
	}
	switch job.Snapshot.RuntimeEngine {
	case workspacedomain.RuntimeClaude:
		return "/run/agent-credentials/extensions/claude-mcp.json"
	case workspacedomain.RuntimeCodex:
		return "/run/agent-credentials/extensions/codex-config.toml"
	case workspacedomain.RuntimeOpenClaw:
		return "/run/agent-credentials/extensions/openclaw.json"
	default:
		return ""
	}
}

func readZipFile(entry *zip.File) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(reader, 10<<20+1))
	closeErr := reader.Close()
	if len(content) > 10<<20 {
		return nil, fmt.Errorf("Skill file %q exceeds 10 MiB", entry.Name)
	}
	return content, errors.Join(readErr, closeErr)
}

func (executor *Executor) stageWorkspaceAt(job application.ExecutionJob, temporary string) (workspace, persistent string, baseline map[string]string, err error) {
	root := filepath.Clean(executor.config.Workspace.Root)
	if err := os.RemoveAll(temporary); err != nil {
		return "", "", nil, fmt.Errorf("clear warm Runtime Workspace: %w", err)
	}
	if err := os.MkdirAll(temporary, 0o750); err != nil {
		return "", "", nil, fmt.Errorf("create warm Runtime Workspace: %w", err)
	}
	if err := prepareSandboxDirectory(temporary, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
		_ = os.RemoveAll(temporary)
		return "", "", nil, err
	}
	if job.Kind == application.JobSession {
		return temporary, "", nil, nil
	}
	persistent = filepath.Join(root, filepath.FromSlash(job.Snapshot.WorkspacePath))
	relative, err := filepath.Rel(root, persistent)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		_ = os.RemoveAll(temporary)
		return "", "", nil, fmt.Errorf("Workflow Workspace path escapes its configured root")
	}
	baseline, err = workspaceManifest(persistent)
	if err != nil {
		_ = os.RemoveAll(temporary)
		return "", "", nil, err
	}
	if err := copyTree(persistent, temporary); err != nil {
		_ = os.RemoveAll(temporary)
		return "", "", nil, err
	}
	return temporary, persistent, baseline, nil
}

func (executor *Executor) persistChangedFiles(ctx context.Context, job application.ExecutionJob, workspace string, baseline map[string]string, redactor *credentials.Redactor) ([]application.ExecutionArtifact, error) {
	manifest, err := workspaceManifest(workspace)
	if err != nil {
		return nil, err
	}
	var artifacts []application.ExecutionArtifact
	for relative, digest := range manifest {
		if baseline[relative] == digest {
			continue
		}
		path := filepath.Join(workspace, filepath.FromSlash(relative))
		input, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		temporary, err := os.CreateTemp(filepath.Dir(path), ".redacted-*")
		if err != nil {
			_ = input.Close()
			return nil, err
		}
		temporaryPath := temporary.Name()
		hasher := sha256.New()
		size, copyErr := io.Copy(io.MultiWriter(temporary, hasher), redactor.Reader(input))
		closeErr := errors.Join(input.Close(), temporary.Close())
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(temporaryPath)
			return nil, errors.Join(copyErr, closeErr)
		}
		if err := os.Chmod(temporaryPath, 0o600); err != nil {
			_ = os.Remove(temporaryPath)
			return nil, err
		}
		if err := os.Rename(temporaryPath, path); err != nil {
			_ = os.Remove(temporaryPath)
			return nil, err
		}
		digest = hex.EncodeToString(hasher.Sum(nil))
		objectPathDigest := sha256.Sum256([]byte(relative))
		objectKey := "artifacts/" + job.OwnerID + "/" + job.WorkflowID + "/" + job.ID + "/" + hex.EncodeToString(objectPathDigest[:])
		artifactFile, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		_, putErr := executor.objects.Put(ctx, objectKey, artifactFile, objectstore.PutOptions{Size: size, SHA256: digest, ContentType: "application/octet-stream"})
		closeErr = artifactFile.Close()
		if putErr != nil || closeErr != nil {
			return nil, fmt.Errorf("store changed Workspace file %q: %w", relative, errors.Join(putErr, closeErr))
		}
		artifact := application.ExecutionArtifact{Name: filepath.Base(relative), Path: relative, ObjectKey: objectKey, Size: size, SHA256: digest, ExpiresAt: time.Now().UTC().Add(90 * 24 * time.Hour)}
		if size <= 1<<20 {
			preview, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if utf8.Valid(preview) {
				artifact.TextPreview = string(preview)
			}
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(hasher, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return "", errors.Join(copyErr, closeErr)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func workspaceManifest(root string) (map[string]string, error) {
	manifest := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("Workspace contains unsupported file type")
		}
		digest, err := fileDigest(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		manifest[filepath.ToSlash(relative)] = digest
		return nil
	})
	return manifest, err
}

func buildInstruction(job application.ExecutionJob, attachmentPaths []string) string {
	var sections []string
	preferences := personalityGuidance(job.Snapshot.Personality)
	if value := strings.TrimSpace(job.Snapshot.PersonalityInstructions); value != "" {
		if preferences != "" {
			preferences += "\n"
		}
		preferences += value
	}
	if preferences != "" {
		sections = append(sections, "Personal response preferences:\n"+preferences)
	}
	if len(job.Snapshot.Skills) > 0 {
		names := make([]string, 0, len(job.Snapshot.Skills))
		for _, skill := range job.Snapshot.Skills {
			names = append(names, skill.Name+"@"+skill.SHA256[:12]+" (/run/agent-credentials/skills/"+skill.ID+")")
		}
		sections = append(sections, "Available isolated Skills: "+strings.Join(names, ", "))
	}
	if len(attachmentPaths) > 0 {
		sections = append(sections, "Files attached to the current user message (read-only; inspect them when relevant):\n- "+strings.Join(attachmentPaths, "\n- "))
	}
	sections = append(sections, job.Instruction)
	return strings.Join(sections, "\n\n")
}

func (executor *Executor) materializeAttachments(ctx context.Context, job application.ExecutionJob, scratch string) ([]string, error) {
	if len(job.Attachments) == 0 {
		return nil, nil
	}
	root := filepath.Join(scratch, "attachments")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create attachment directory: %w", err)
	}
	paths := make([]string, 0, len(job.Attachments))
	for _, attachment := range job.Attachments {
		expectedKey := "attachments/" + job.OwnerID + "/" + attachment.ID
		if attachment.ObjectKey != expectedKey || filepath.Base(attachment.Name) != attachment.Name || attachment.Name == "." || attachment.Name == ".." {
			return nil, fmt.Errorf("invalid attachment reference")
		}
		reader, object, err := executor.objects.Get(ctx, expectedKey)
		if err != nil {
			return nil, fmt.Errorf("open attachment %q: %w", attachment.Name, err)
		}
		directory := filepath.Join(root, attachment.ID)
		if err := os.MkdirAll(directory, 0o750); err != nil {
			_ = reader.Close()
			return nil, fmt.Errorf("create attachment staging directory: %w", err)
		}
		path := filepath.Join(directory, attachment.Name)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o400)
		if err != nil {
			_ = reader.Close()
			return nil, fmt.Errorf("create attachment copy: %w", err)
		}
		digest := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(file, digest), io.LimitReader(reader, attachment.Size+1))
		closeErr := errors.Join(file.Close(), reader.Close())
		actualDigest := hex.EncodeToString(digest.Sum(nil))
		if copyErr != nil || closeErr != nil || written != attachment.Size || object.Size != attachment.Size || actualDigest != attachment.SHA256 || object.SHA256 != attachment.SHA256 {
			return nil, fmt.Errorf("attachment %q failed size or checksum verification: %w", attachment.Name, errors.Join(copyErr, closeErr, objectstore.ErrChecksumMismatch))
		}
		if err := os.Chown(path, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
			return nil, fmt.Errorf("set attachment ownership: %w", err)
		}
		if err := os.Chmod(path, 0o400); err != nil {
			return nil, fmt.Errorf("protect attachment copy: %w", err)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func personalityGuidance(personality string) string {
	switch personality {
	case "gentle_professional":
		return "Use a calm, considerate, and professional tone. Explain important details without sounding abrupt."
	case "direct_efficient":
		return "Be direct, concise, and action-oriented. Lead with the result and avoid unnecessary preamble."
	case "lively_friendly":
		return "Use an upbeat, approachable, and friendly tone while keeping the answer useful and clear."
	case "custom":
		return ""
	default:
		return ""
	}
}

func runtimeAdapter(engine workspacedomain.RuntimeEngine, config cliadapter.Config) (agentruntime.Adapter, error) {
	switch engine {
	case workspacedomain.RuntimeClaude:
		return claude.New(config), nil
	case workspacedomain.RuntimeCodex:
		return codex.New(config), nil
	case workspacedomain.RuntimeHermes:
		return hermes.New(config), nil
	case workspacedomain.RuntimeOpenClaw:
		return openclaw.New(config), nil
	default:
		return nil, fmt.Errorf("unsupported Runtime %q", engine)
	}
}

type eventSink struct {
	runID    string
	job      application.ExecutionJob
	progress application.ProgressRecorder
}

func (sink *eventSink) Publish(ctx context.Context, event agentruntime.Event) error {
	if event.RunID != sink.runID {
		return fmt.Errorf("Runtime Event Run ID mismatch")
	}
	if sink.progress == nil {
		return fmt.Errorf("Runtime progress recorder is required")
	}
	return sink.progress.RecordProgress(ctx, sink.job, application.ExecutionEvent{Type: string(event.Kind), Payload: append([]byte(nil), event.Payload...)})
}

type discardOutput struct{}

func (discardOutput) Store(context.Context, processharness.Output) error { return nil }

func ensureDirectory(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) == string(filepath.Separator) {
		return fmt.Errorf("Worker directory must be absolute and non-root")
	}
	return os.MkdirAll(path, 0o700)
}

func copyTree(source, target string) error {
	if _, err := os.Stat(source); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Workspace symlinks are not allowed")
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("Workspace contains unsupported file type")
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		closeErr := output.Close()
		return errors.Join(copyErr, inputCloseErr, closeErr)
	})
}

func mergeSuccessfulWorkspace(persistent, temporary string) error {
	backup := persistent + ".previous"
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	if err := os.Rename(persistent, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporary, persistent); err != nil {
		_ = os.Rename(backup, persistent)
		return err
	}
	return os.RemoveAll(backup)
}
