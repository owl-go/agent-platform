package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"agent-platform/internal/agentruntime"
	"agent-platform/internal/agentruntime/claude"
	"agent-platform/internal/agentruntime/cliadapter"
	"agent-platform/internal/agentruntime/codex"
	"agent-platform/internal/agentruntime/containerprocess"
	"agent-platform/internal/agentruntime/hermes"
	"agent-platform/internal/agentruntime/openclaw"
	"agent-platform/internal/agentruntime/processharness"
	"agent-platform/internal/credentials"
	"agent-platform/internal/runworker"
	"agent-platform/internal/sandbox"
)

type options struct {
	runtime       string
	image         string
	model         string
	workspace     string
	credentialDir string
	outputDir     string
	runID         string
	network       string
	instruction   string
	timeout       time.Duration
}

type report struct {
	RunID       string                  `json:"run_id"`
	Runtime     agentruntime.Descriptor `json:"runtime"`
	Image       string                  `json:"image"`
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt time.Time               `json:"completed_at"`
	DurationMS  int64                   `json:"duration_ms"`
	Result      agentruntime.Result     `json:"result"`
	ErrorCode   agentruntime.ErrorCode  `json:"error_code,omitempty"`
	Error       string                  `json:"error,omitempty"`
	Artifacts   map[string]string       `json:"artifacts"`
}

func main() {
	opts := parseFlags()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, opts); err != nil {
		log.Fatal(err)
	}
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.runtime, "runtime", "", "claude, codex, hermes, or openclaw")
	flag.StringVar(&opts.image, "image", "", "immutable runtime image repo digest")
	flag.StringVar(&opts.model, "model", "", "configured model identifier")
	flag.StringVar(&opts.workspace, "workspace", "", "absolute path to a prepared Git workspace")
	flag.StringVar(&opts.credentialDir, "credentials", "", "absolute per-run credential directory")
	flag.StringVar(&opts.outputDir, "output", "", "evidence output directory outside the workspace")
	flag.StringVar(&opts.runID, "run-id", "", "stable conformance run identifier")
	flag.StringVar(&opts.network, "network", "agent-public-egress", "Docker network with public-only egress")
	flag.StringVar(&opts.instruction, "instruction", "", "coding task instruction")
	flag.DurationVar(&opts.timeout, "timeout", 30*time.Minute, "runtime execution timeout")
	flag.Parse()
	return opts
}

func run(parent context.Context, opts options) error {
	if err := validateOptions(opts); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.outputDir, 0o700); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}
	patterns, err := loadCredentialPatterns(opts.credentialDir)
	if err != nil {
		return err
	}
	redactor := credentials.NewRedactor(patterns...)
	baseline, err := gitOutput(opts.workspace, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read baseline commit: %w", err)
	}

	runProcess, err := containerprocess.New(containerprocess.Config{
		Image: opts.image, RuntimeCommand: opts.runtime, RunID: opts.runID,
		Runtime: "runsc", WorkspaceDirectory: opts.workspace, CredentialDirectory: opts.credentialDir,
		PublicEgressNetwork: opts.network, Egress: sandbox.EgressPublic, UID: 65532, GID: 65532,
		Limits: sandbox.Limits{CPUs: 2, MemoryBytes: 4 * 1024 * 1024 * 1024, PIDs: 512, TempBytes: 512 * 1024 * 1024},
	})
	if err != nil {
		return fmt.Errorf("configure container process: %w", err)
	}
	outputSink := processharness.NewRedactingSink(redactor, &fileOutputSink{directory: opts.outputDir})
	adapter, err := newAdapter(opts.runtime, cliadapter.Config{RunProcess: runProcess, OutputSink: outputSink})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()
	started := time.Now().UTC()
	descriptor, describeErr := adapter.Describe(ctx)
	if describeErr != nil {
		return writeFailure(opts, report{RunID: opts.runID, Image: opts.image, StartedAt: started}, describeErr)
	}
	events, err := newJSONLEventSink(filepath.Join(opts.outputDir, "events.jsonl"))
	if err != nil {
		return err
	}
	defer events.Close()
	result, executeErr := runworker.New(adapter).Execute(ctx, agentruntime.ExecuteRequest{
		RunID: opts.runID, WorkspacePath: opts.workspace, Instruction: opts.instruction,
		Model: opts.model, EnvironmentRef: "production-conformance",
	}, agentruntime.NewRedactingEventSink(redactor, events))
	result = redactResult(redactor, result)

	diff, diffErr := gitOutput(opts.workspace, "diff", "--binary", "--no-ext-diff", strings.TrimSpace(string(baseline)), "--")
	if diffErr == nil {
		diff = redactor.Bytes(diff)
		diffErr = os.WriteFile(filepath.Join(opts.outputDir, "workspace.diff"), diff, 0o600)
	}
	if diffErr == nil && len(bytes.TrimSpace(diff)) == 0 {
		diffErr = errors.New("runtime produced no repository diff")
	}
	secretErr := scanWorkspace(opts.workspace, patterns)

	completed := time.Now().UTC()
	r := report{
		RunID: opts.runID, Runtime: descriptor, Image: opts.image, StartedAt: started, CompletedAt: completed,
		DurationMS: completed.Sub(started).Milliseconds(), Result: result,
		Artifacts: map[string]string{"events": "events.jsonl", "stdout": "stdout.log", "stderr": "stderr.log", "diff": "workspace.diff"},
	}
	combinedErr := errors.Join(executeErr, diffErr, secretErr)
	if combinedErr != nil {
		r.ErrorCode = agentruntime.ErrorCodeOf(combinedErr)
		r.Error = string(redactor.Bytes([]byte(combinedErr.Error())))
	}
	if err := writeReport(opts.outputDir, r); err != nil {
		return err
	}
	return combinedErr
}

func redactResult(redactor *credentials.Redactor, result agentruntime.Result) agentruntime.Result {
	result.FinalMessage = string(redactor.Bytes([]byte(result.FinalMessage)))
	result.DiffArtifact = string(redactor.Bytes([]byte(result.DiffArtifact)))
	return result
}

func validateOptions(opts options) error {
	for name, value := range map[string]string{
		"runtime": opts.runtime, "image": opts.image, "model": opts.model, "workspace": opts.workspace,
		"credentials": opts.credentialDir, "output": opts.outputDir, "run ID": opts.runID, "network": opts.network,
		"instruction": opts.instruction,
	} {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	for name, path := range map[string]string{"workspace": opts.workspace, "credentials": opts.credentialDir, "output": opts.outputDir} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s path must be absolute", name)
		}
	}
	workspace := filepath.Clean(opts.workspace) + string(os.PathSeparator)
	output := filepath.Clean(opts.outputDir) + string(os.PathSeparator)
	if strings.HasPrefix(output, workspace) {
		return fmt.Errorf("evidence output must be outside the agent workspace")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

func newAdapter(name string, config cliadapter.Config) (agentruntime.Adapter, error) {
	switch name {
	case "claude":
		return claude.New(config), nil
	case "codex":
		return codex.New(config), nil
	case "hermes":
		return hermes.New(config), nil
	case "openclaw":
		return openclaw.New(config), nil
	default:
		return nil, fmt.Errorf("unsupported runtime %q", name)
	}
}

func loadCredentialPatterns(root string) ([][]byte, error) {
	patterns := make([][]byte, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 1024*1024 {
			return fmt.Errorf("credential file %q exceeds 1 MiB", path)
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(value) >= 4 {
			patterns = append(patterns, value)
			trimmed := bytes.TrimRight(value, "\r\n")
			if len(trimmed) >= 4 && len(trimmed) != len(value) {
				patterns = append(patterns, bytes.Clone(trimmed))
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load credential redaction patterns: %w", err)
	}
	if len(patterns) == 0 {
		return nil, fmt.Errorf("credential directory contains no redaction patterns")
	}
	return patterns, nil
}

func scanWorkspace(root string, patterns [][]byte) error {
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 128*1024*1024 {
			return fmt.Errorf("workspace file %q exceeds secret scan limit", path)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, pattern := range patterns {
			if bytes.Contains(contents, pattern) {
				return fmt.Errorf("credential value found in workspace file %q", path)
			}
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	return scanGitObjects(root, patterns)
}

func scanGitObjects(workspace string, patterns [][]byte) error {
	command := exec.Command("git", "-C", workspace, "cat-file", "--batch-all-objects", "--batch")
	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open Git object scan: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Git object scan: %w", err)
	}
	detector := newPatternDetector(patterns)
	_, copyErr := io.Copy(detector, stdout)
	waitErr := command.Wait()
	if copyErr != nil || waitErr != nil {
		return fmt.Errorf("scan Git objects: %s: %w", strings.TrimSpace(stderr.String()), errors.Join(copyErr, waitErr))
	}
	if detector.found {
		return fmt.Errorf("credential value found in Git object database")
	}
	return nil
}

func gitOutput(workspace string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", workspace}, args...)...)
	return command.CombinedOutput()
}

func writeFailure(opts options, r report, cause error) error {
	r.CompletedAt = time.Now().UTC()
	r.DurationMS = r.CompletedAt.Sub(r.StartedAt).Milliseconds()
	r.ErrorCode = agentruntime.ErrorCodeOf(cause)
	r.Error = cause.Error()
	return errors.Join(cause, writeReport(opts.outputDir, r))
}

func writeReport(directory string, value report) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode conformance report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "report.json"), append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write conformance report: %w", err)
	}
	return nil
}

type fileOutputSink struct {
	directory string
	mu        sync.Mutex
}

func (s *fileOutputSink) Store(_ context.Context, output processharness.Output) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := "stderr.log"
	if output.Stream == processharness.StreamStdout {
		name = "stdout.log"
	}
	file, err := os.OpenFile(filepath.Join(s.directory, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, output.Reader)
	return errors.Join(copyErr, file.Close())
}

type jsonlEventSink struct {
	mu   sync.Mutex
	file *os.File
}

func newJSONLEventSink(path string) (*jsonlEventSink, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create event evidence: %w", err)
	}
	return &jsonlEventSink{file: file}, nil
}

func (s *jsonlEventSink) Publish(_ context.Context, event agentruntime.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := struct {
		RunID      string          `json:"run_id"`
		Sequence   int64           `json:"sequence"`
		Kind       string          `json:"kind"`
		OccurredAt time.Time       `json:"occurred_at"`
		Payload    json.RawMessage `json:"payload"`
	}{event.RunID, event.Sequence, string(event.Kind), event.OccurredAt, event.Payload}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = s.file.Write(append(encoded, '\n'))
	return err
}

func (s *jsonlEventSink) Close() error { return s.file.Close() }

type patternDetector struct {
	patterns [][]byte
	pending  []byte
	keep     int
	found    bool
}

func newPatternDetector(patterns [][]byte) *patternDetector {
	keep := 0
	for _, pattern := range patterns {
		if len(pattern) > keep {
			keep = len(pattern)
		}
	}
	if keep > 0 {
		keep--
	}
	return &patternDetector{patterns: patterns, keep: keep}
}

func (d *patternDetector) Write(value []byte) (int, error) {
	combined := append(append([]byte(nil), d.pending...), value...)
	for _, pattern := range d.patterns {
		if len(pattern) > 0 && bytes.Contains(combined, pattern) {
			d.found = true
		}
	}
	if len(combined) > d.keep {
		d.pending = append(d.pending[:0], combined[len(combined)-d.keep:]...)
	} else {
		d.pending = append(d.pending[:0], combined...)
	}
	return len(value), nil
}
