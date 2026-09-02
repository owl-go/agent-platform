package runtimeexecutor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/containerprocess"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/secretcrypto"
)

func TestMaterializeAttachmentsVerifiesAndProtectsCopies(t *testing.T) {
	provider := memory.New()
	content := []byte("attachment contents")
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	key := "attachments/owner-1/11111111-1111-4111-8111-111111111111"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: "text/plain", Metadata: map[string]string{"name": "notes.txt"}}); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{objects: provider, config: platformconfig.Config{Worker: platformconfig.WorkerConfig{SandboxUID: os.Getuid(), SandboxGID: os.Getgid()}}}
	root := t.TempDir()
	paths, err := executor.materializeAttachments(context.Background(), application.ExecutionJob{OwnerID: "owner-1", Attachments: []domain.Attachment{{ID: "11111111-1111-4111-8111-111111111111", Name: "notes.txt", ObjectKey: key, Size: int64(len(content)), SHA256: sha}}}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %v", paths)
	}
	got, err := os.ReadFile(paths[0])
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("copy = %q, %v", got, err)
	}
	info, err := os.Stat(paths[0])
	if err != nil || info.Mode().Perm() != 0o400 {
		t.Fatalf("copy mode = %v, %v", info.Mode().Perm(), err)
	}
}

func TestBuildInstructionAppliesPresetAndCustomPersonality(t *testing.T) {
	tests := []struct {
		name        string
		personality string
		guidance    string
		want        []string
	}{
		{name: "preset", personality: "direct_efficient", guidance: "Prefer bullet points.", want: []string{"Be direct, concise", "Prefer bullet points.", "Do the work"}},
		{name: "custom", personality: "custom", guidance: "Answer like a patient tutor.", want: []string{"Answer like a patient tutor.", "Do the work"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instruction := buildInstruction(application.ExecutionJob{
				Instruction: "Do the work",
				Snapshot:    domain.ExecutionSnapshot{Personality: test.personality, PersonalityInstructions: test.guidance},
			}, nil)
			for _, value := range test.want {
				if !strings.Contains(instruction, value) {
					t.Fatalf("instruction %q does not contain %q", instruction, value)
				}
			}
		})
	}
}

func TestBuildInstructionUsesExecutionInstructionNotCapabilityIntroduction(t *testing.T) {
	job := application.ExecutionJob{Instruction: "Current task", Snapshot: domain.ExecutionSnapshot{Expert: &domain.ExpertSnapshot{Name: "Architect", CapabilityIntroduction: "Public marketing copy", ExecutionInstruction: "Inspect boundaries before proposing changes."}}}
	got := buildInstruction(job, nil)
	if !strings.Contains(got, "Inspect boundaries before proposing changes.") {
		t.Fatalf("instruction does not contain Expert Execution Instruction: %q", got)
	}
	if strings.Contains(got, "Public marketing copy") {
		t.Fatalf("instruction contains display-only Capability Introduction: %q", got)
	}
}

func TestTeamMemberJobPassesOnlyPriorFinalResultsAndOwnExtensions(t *testing.T) {
	base := application.ExecutionJob{Instruction: "Solve the task", CheckpointRef: "native-session", Snapshot: domain.ExecutionSnapshot{ExpertTeam: &domain.ExpertTeamSnapshot{ID: "team-1"}}}
	member := domain.ExpertMemberSnapshot{ExpertSnapshot: domain.ExpertSnapshot{ID: "expert-2", Name: "Builder", ExecutionInstruction: "Implement it."}, Position: 2, MCPServers: []domain.MCPServerSnapshot{{ID: "mcp-2"}}, Skills: []domain.SkillSnapshot{{ID: "skill-2"}}}
	got := teamMemberJob(base, member, []domain.ExpertStage{{ExpertName: "Architect", FinalText: "Use a hexagonal boundary."}})
	if got.CheckpointRef != "" || got.Snapshot.Expert == nil || got.Snapshot.Expert.ID != "expert-2" {
		t.Fatalf("team member execution identity = %#v", got)
	}
	if len(got.Snapshot.MCPServers) != 1 || got.Snapshot.MCPServers[0].ID != "mcp-2" || len(got.Snapshot.Skills) != 1 || got.Snapshot.Skills[0].ID != "skill-2" {
		t.Fatalf("team member extensions = %#v / %#v", got.Snapshot.MCPServers, got.Snapshot.Skills)
	}
	if !strings.Contains(got.Instruction, "Architect:\nUse a hexagonal boundary.") {
		t.Fatalf("team member instruction = %q", got.Instruction)
	}
}

func TestEventSinkStreamsIntermediateDeltasWithoutPublishingAnOfficialTerminalEvent(t *testing.T) {
	recorder := &recordingProgress{}
	sink := &eventSink{runID: "run-1", job: application.ExecutionJob{ID: "run-1"}, progress: recorder, suppressMessages: true}
	for _, kind := range []agentruntime.EventKind{agentruntime.EventMessageCompleted, agentruntime.EventRuntimeCompleted, agentruntime.EventMessageDelta, agentruntime.EventCommandCompleted} {
		if err := sink.Publish(context.Background(), agentruntime.Event{RunID: "run-1", Kind: kind, OccurredAt: time.Now(), Payload: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(recorder.events) != 2 || recorder.events[0].Type != string(agentruntime.EventMessageDelta) || recorder.events[1].Type != string(agentruntime.EventCommandCompleted) {
		t.Fatalf("published events = %#v", recorder.events)
	}
}

func TestExecuteExpertTeamRunsInOrderAndCommitsOnlyTheFinalResult(t *testing.T) {
	executor, job, persistent := newTeamTestExecutor(t)
	var calls []string
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		position := len(calls) + 1
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			calls = append(calls, request.Instruction)
			if position == 1 {
				if err := os.WriteFile(filepath.Join(request.WorkspacePath, "shared.txt"), []byte("stage one"), 0o600); err != nil {
					return agentruntime.Result{}, err
				}
			} else {
				body, err := os.ReadFile(filepath.Join(request.WorkspacePath, "shared.txt"))
				if err != nil || string(body) != "stage one" || !strings.Contains(request.Instruction, "First Expert:\nfirst result") {
					return agentruntime.Result{}, errors.New("second Expert did not receive shared context")
				}
			}
			message := []string{"first result", "final result"}[position-1]
			publishSuccessfulRuntime(t, events, request.RunID, message)
			return agentruntime.Result{FinalMessage: message}, nil
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !strings.Contains(calls[0], "First Expert instruction") || !strings.Contains(calls[1], "Second Expert instruction") {
		t.Fatalf("execution order = %#v", calls)
	}
	if result.FinalMessage != "final result" || len(result.ExpertStages) != 2 || result.ExpertStages[0].FinalText != "first result" || result.ExpertStages[1].FinalText != "final result" {
		t.Fatalf("execution result = %#v", result)
	}
	body, err := os.ReadFile(filepath.Join(persistent, "shared.txt"))
	if err != nil || string(body) != "stage one" {
		t.Fatalf("committed Workspace = %q, %v", body, err)
	}
}

func TestExecuteExpertTeamFailsFastAndRollsBackWorkspace(t *testing.T) {
	executor, job, persistent := newTeamTestExecutor(t)
	adapterCalls := 0
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		adapterCalls++
		position := adapterCalls
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			if err := os.WriteFile(filepath.Join(request.WorkspacePath, "uncommitted.txt"), []byte("partial"), 0o600); err != nil {
				return agentruntime.Result{}, err
			}
			if position == 1 {
				publishSuccessfulRuntime(t, events, request.RunID, "first result")
				return agentruntime.Result{FinalMessage: "first result"}, nil
			}
			publishRuntimeFailure(t, events, request.RunID, "second failed")
			return agentruntime.Result{}, errors.New("second failed")
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err == nil || !strings.Contains(err.Error(), "second failed") {
		t.Fatalf("error = %v", err)
	}
	if adapterCalls != 2 || len(result.ExpertStages) != 2 || result.ExpertStages[1].State != "failed" {
		t.Fatalf("fail-fast result = %#v, calls = %d", result, adapterCalls)
	}
	if _, err := os.Stat(filepath.Join(persistent, "uncommitted.txt")); !os.IsNotExist(err) {
		t.Fatalf("failed Workspace changes were committed: %v", err)
	}
}

func TestExecuteExpertTeamUsesOneOverallTimeout(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	executor.executionTTL = 20 * time.Millisecond
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(ctx context.Context, _ agentruntime.ExecuteRequest, _ agentruntime.EventSink) (agentruntime.Result, error) {
			<-ctx.Done()
			return agentruntime.Result{}, ctx.Err()
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if !errors.Is(err, context.DeadlineExceeded) || len(result.ExpertStages) != 1 || result.ExpertStages[0].State != "cancelled" {
		t.Fatalf("timeout result = %#v, error = %v", result, err)
	}
}

func newTeamTestExecutor(t *testing.T) (*Executor, application.ExecutionJob, string) {
	t.Helper()
	root, credentialsRoot := t.TempDir(), t.TempDir()
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := box.Encrypt([]byte("test-key"), "model-provider:owner-1")
	if err != nil {
		t.Fatal(err)
	}
	warm, err := containerprocess.NewWarmManager("docker", 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	config := platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: root},
		Worker: platformconfig.WorkerConfig{CredentialTempRoot: credentialsRoot, SandboxUID: os.Getuid(), SandboxGID: os.Getgid(), Runtimes: map[string]platformconfig.RuntimeEngineConfig{
			"codex": {Available: true, ImageDigest: "registry.example/runtime@sha256:" + strings.Repeat("0", 64), CLIVersion: "test"},
		}},
	}
	executor, err := New(config, box, memory.New(), warm)
	if err != nil {
		t.Fatal(err)
	}
	workspacePath := "owners/owner-1/workflows/workflow-1"
	persistent := filepath.Join(root, filepath.FromSlash(workspacePath))
	if err := os.MkdirAll(persistent, 0o750); err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{
		ID: "11111111-1111-4111-8111-111111111111", Kind: application.JobWorkflow, OwnerID: "owner-1", WorkflowID: "22222222-2222-4222-8222-222222222222", Instruction: "Solve this task",
		Snapshot: domain.ExecutionSnapshot{
			RuntimeEngine: domain.RuntimeCodex, WorkspacePath: workspacePath,
			ProviderModel: domain.ProviderModelSnapshot{ModelID: "test-model", Endpoint: "https://example.test/v1", APIKeyCiphertext: secret},
			ExpertTeam: &domain.ExpertTeamSnapshot{ID: "team-1", Members: []domain.ExpertMemberSnapshot{
				{ExpertSnapshot: domain.ExpertSnapshot{ID: "expert-1", Name: "First Expert", ExecutionInstruction: "First Expert instruction"}, Position: 1},
				{ExpertSnapshot: domain.ExpertSnapshot{ID: "expert-2", Name: "Second Expert", ExecutionInstruction: "Second Expert instruction"}, Position: 2},
			}},
		},
	}
	return executor, job, persistent
}

type recordingLease struct{}

func (*recordingLease) Start(context.Context, containerprocess.Config) (cliadapter.RunProcess, error) {
	return nil, nil
}
func (*recordingLease) Release(context.Context) error { return nil }

type recordingAdapter struct {
	execute func(context.Context, agentruntime.ExecuteRequest, agentruntime.EventSink) (agentruntime.Result, error)
}

func (*recordingAdapter) Describe(context.Context) (agentruntime.Descriptor, error) {
	return agentruntime.Descriptor{Name: "recording", Version: "test", Capabilities: map[agentruntime.Capability]bool{agentruntime.CapabilityStreaming: true}}, nil
}
func (adapter *recordingAdapter) Execute(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
	return adapter.execute(ctx, request, events)
}

func publishSuccessfulRuntime(t *testing.T, sink agentruntime.EventSink, runID, message string) {
	t.Helper()
	events := []agentruntime.Event{
		{RunID: runID, Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: time.Now(), Payload: []byte(`{}`)},
		{RunID: runID, Sequence: 2, Kind: agentruntime.EventMessageDelta, OccurredAt: time.Now(), Payload: []byte(`{"delta":"` + message + `"}`)},
		{RunID: runID, Sequence: 3, Kind: agentruntime.EventMessageCompleted, OccurredAt: time.Now(), Payload: []byte(`{"message":"` + message + `"}`)},
		{RunID: runID, Sequence: 4, Kind: agentruntime.EventRuntimeCompleted, OccurredAt: time.Now(), Payload: []byte(`{}`)},
	}
	for _, event := range events {
		if err := sink.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}

func publishRuntimeFailure(t *testing.T, sink agentruntime.EventSink, runID, message string) {
	t.Helper()
	for _, event := range []agentruntime.Event{
		{RunID: runID, Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: time.Now(), Payload: []byte(`{}`)},
		{RunID: runID, Sequence: 2, Kind: agentruntime.EventRuntimeFailed, OccurredAt: time.Now(), Payload: []byte(`{"error":"` + message + `"}`)},
	} {
		if err := sink.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}

type recordingProgress struct{ events []application.ExecutionEvent }

func (recorder *recordingProgress) RecordProgress(_ context.Context, _ application.ExecutionJob, event application.ExecutionEvent) error {
	recorder.events = append(recorder.events, event)
	return nil
}

func TestSanitizeNativeStateRemovesTransientConfigAndRedactsSessionFiles(t *testing.T) {
	root := t.TempDir()
	secret := "native-state-secret-canary"
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("token = \""+secret+"\""), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(root, "sessions", "2026", "08")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(sessions, "rollout.jsonl")
	if err := os.WriteFile(rollout, []byte(`{"message":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sanitizeNativeState(root, credentials.NewRedactor([]byte(secret))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("transient config remains: %v", err)
	}
	content, err := os.ReadFile(rollout)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), secret) || !strings.Contains(string(content), "[REDACTED]") {
		t.Fatalf("session state was not redacted: %s", content)
	}
}

func TestPreparePersistentWorkspaceTreeMakesMergedFilesReadableByThePlatformUser(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "output")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "result.txt")
	if err := os.WriteFile(file, []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := preparePersistentWorkspaceTree(root, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]os.FileMode{directory: 0o750, file: 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("%s mode = %o, want %o", path, got, want)
		}
	}
}

func TestPrepareRuntimeScratchCreatesAnInitiallyMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slot", "scratch")
	if err := prepareRuntimeScratch(path, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o750 {
		t.Fatalf("Runtime scratch directory = mode %o, directory=%t", info.Mode().Perm(), info.IsDir())
	}
}

func TestWarmSlotIsStablePerResourceAndRuntime(t *testing.T) {
	executor := &Executor{config: platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: "/workspaces"},
		Worker:    platformconfig.WorkerConfig{CredentialTempRoot: "/credentials"},
	}}
	runtime := platformconfig.RuntimeEngineConfig{ImageDigest: "registry.example/runtime@sha256:" + strings.Repeat("a", 64)}
	first, slot, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobSession, OwnerID: "owner-1", SessionID: "session-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	repeat, _, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobSession, OwnerID: "owner-1", SessionID: "session-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	workflow, _, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobWorkflow, OwnerID: "owner-1", WorkflowID: "workflow-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if first != repeat || first == workflow {
		t.Fatalf("warm container names: first=%q repeat=%q workflow=%q", first, repeat, workflow)
	}
	if !strings.Contains(slot.workspace, strings.TrimPrefix(first, "agent-runtime-warm-")) || !strings.HasPrefix(slot.credentials, "/credentials/") {
		t.Fatalf("warm slot paths = %+v", slot)
	}
}
