package containerprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/sandbox"
)

func TestRunWrapsRuntimeCommandInHardenedContainer(t *testing.T) {
	var captured processharness.Spec
	var cleaned string
	runner, err := New(Config{
		Image:               "registry.example/runtime@sha256:" + strings.Repeat("a", 64),
		RuntimeCommand:      "codex",
		RunID:               "run-1",
		Runtime:             "runsc",
		WorkspaceDirectory:  "/workspaces/run-1",
		UID:                 65532,
		GID:                 65532,
		CredentialDirectory: "/credentials/run-1",
		PublicEgressNetwork: "agent-public-egress",
		ResolverConfigFile:  "/etc/agent-platform/sandbox-resolv.conf",
		Egress:              sandbox.EgressPublic,
		Limits: sandbox.Limits{
			CPUs: 2, MemoryBytes: 512 * 1024 * 1024, PIDs: 256, TempBytes: 64 * 1024 * 1024,
		},
		Name: func() (string, error) { return "agent-runtime-test", nil },
		RunHost: func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
			captured = spec
			return processharness.Result{ExitCode: 0}, nil
		},
		Cleanup: func(_ context.Context, name string) error {
			cleaned = name
			return nil
		},
	})
	if err != nil {
		t.Fatalf("new container process: %v", err)
	}

	stdin := strings.NewReader("change the code")
	observer := &recordingObserver{}
	result, err := runner(context.Background(), processharness.Spec{
		Command:        []string{"codex", "exec", "--json", "-"},
		Dir:            "/workspaces/run-1",
		Env:            []string{"NON_SECRET=value"},
		Stdin:          stdin,
		MaxOutputBytes: 2048,
		MaxLineBytes:   512,
		Observer:       observer,
	}, discardSink{})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("run result = %+v, err = %v", result, err)
	}
	if cleaned != "agent-runtime-test" {
		t.Fatalf("cleaned container = %q", cleaned)
	}
	if captured.Stdin != stdin || captured.Observer != observer || captured.MaxOutputBytes != 2048 || captured.MaxLineBytes != 512 {
		t.Fatalf("process controls were not preserved: %+v", captured)
	}
	if !reflect.DeepEqual(captured.Env, []string{"NON_SECRET=value"}) {
		t.Fatalf("host environment = %#v", captured.Env)
	}
	want := []string{
		"docker", "run", "--name", "agent-runtime-test", "--rm", "--interactive",
		"--runtime", "runsc", "--user", "65532:65532", "--read-only",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", "agent-public-egress", "--memory", "536870912",
		"--pids-limit", "256", "--cpus", "2",
		"--mount", "type=bind,src=/workspaces/run-1,dst=/workspace,readonly=false",
		"--mount", "type=bind,src=/credentials/run-1,dst=/run/agent-credentials,readonly=true",
		"--mount", "type=bind,src=/etc/agent-platform/sandbox-resolv.conf,dst=/etc/resolv.conf,readonly=true",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=67108864",
		"--workdir", "/workspace", "--env", "NON_SECRET", "--init",
		"--label", "agent-platform.managed=true", "--label", "agent-platform.run-id=run-1",
		"registry.example/runtime@sha256:" + strings.Repeat("a", 64),
		"exec", "--json", "-",
	}
	if !reflect.DeepEqual(captured.Command, want) {
		t.Fatalf("docker command:\n got: %#v\nwant: %#v", captured.Command, want)
	}
}

func TestRunCanInvokeAllowlistedDirectCommandThroughEntrypoint(t *testing.T) {
	var captured processharness.Spec
	config := validConfig(func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
		captured = spec
		return processharness.Result{}, nil
	}, func(context.Context, string) error { return nil })
	config.RuntimeCommand = "curl"
	config.DirectCommand = true
	runner, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner(context.Background(), processharness.Spec{Command: []string{"curl", "--version"}, Dir: "/workspace"}, discardSink{}); err != nil {
		t.Fatal(err)
	}
	wantTail := []string{digestImage(), "curl", "--version"}
	joined := strings.Join(captured.Command, "\x00")
	if !strings.Contains(joined, "--entrypoint\x00/usr/local/bin/runtime-entrypoint") || len(captured.Command) < len(wantTail) || !reflect.DeepEqual(captured.Command[len(captured.Command)-len(wantTail):], wantTail) {
		t.Fatalf("direct command tail = %#v", captured.Command)
	}
}

func TestRunMountsCodexSessionStateWithoutMountingTheWholeHome(t *testing.T) {
	var captured processharness.Spec
	config := validConfig(func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
		captured = spec
		return processharness.Result{}, nil
	}, func(context.Context, string) error { return nil })
	config.RuntimeCommand = "codex"
	config.NativeStateDirectory = "/workspaces/.native-session-state/owner/session/codex"
	runner, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner(context.Background(), processharness.Spec{Command: []string{"codex", "exec", "-"}, Dir: "/workspace"}, discardSink{}); err != nil {
		t.Fatal(err)
	}
	want := "type=bind,src=/workspaces/.native-session-state/owner/session/codex,dst=/tmp/runtime-home/.codex,readonly=false"
	if !containsPair(captured.Command, "--mount", want) {
		t.Fatalf("native Codex session mount missing from %#v", captured.Command)
	}
	if strings.Contains(strings.Join(captured.Command, "\x00"), "dst=/tmp/runtime-home,readonly=false") {
		t.Fatalf("whole Runtime home was mounted: %#v", captured.Command)
	}
}

func TestRunBindsApprovalObserverToContainerPause(t *testing.T) {
	var controls []string
	observer := &processAwareRecordingObserver{}
	config := validConfig(func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
		aware, ok := spec.Observer.(processharness.ProcessAwareObserver)
		if !ok {
			t.Fatal("container observer is not process-aware")
		}
		aware.BindProcess(&recordingProcessController{})
		if err := observer.process.Pause(); err != nil {
			t.Fatal(err)
		}
		if err := observer.process.Resume(); err != nil {
			t.Fatal(err)
		}
		return processharness.Result{}, nil
	}, func(context.Context, string) error { return nil })
	config.ControlContainer = func(_ context.Context, name string, paused bool) error {
		controls = append(controls, fmt.Sprintf("%s:%t", name, paused))
		return nil
	}
	runner, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner(context.Background(), processharness.Spec{Command: []string{"claude", "--version"}, Dir: "/workspace", Observer: observer}, discardSink{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(controls, []string{"agent-runtime-test:true", "agent-runtime-test:false"}) {
		t.Fatalf("container controls = %#v", controls)
	}
}

func TestRunCleansContainerWhenHostProcessFails(t *testing.T) {
	runErr := errors.New("docker failed")
	cleanupErr := errors.New("cleanup failed")
	cleaned := false
	runner, err := New(validConfig(func(context.Context, processharness.Spec, processharness.OutputSink) (processharness.Result, error) {
		return processharness.Result{ExitCode: 125}, runErr
	}, func(context.Context, string) error {
		cleaned = true
		return cleanupErr
	}))
	if err != nil {
		t.Fatalf("new container process: %v", err)
	}
	_, err = runner(context.Background(), processharness.Spec{Command: []string{"claude", "--version"}, Dir: "/workspace"}, discardSink{})
	if !cleaned || !errors.Is(err, runErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("cleanup = %v, err = %v", cleaned, err)
	}
}

func TestRunSupportsNamedWorkspaceVolume(t *testing.T) {
	var captured processharness.Spec
	config := validConfig(func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
		captured = spec
		return processharness.Result{}, nil
	}, func(context.Context, string) error { return nil })
	config.WorkspaceDirectory = ""
	config.WorkspaceVolume = "agent-workspace-run-1"
	config.WorkflowPlan = "eyJydW5faWQiOiJydW4tMSJ9"
	runner, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner(context.Background(), processharness.Spec{Command: []string{"claude", "--version"}, Dir: "/workspace"}, discardSink{}); err != nil {
		t.Fatal(err)
	}
	if captured.Dir != "" {
		t.Fatalf("host Docker process directory = %q, want empty", captured.Dir)
	}
	if !containsPair(captured.Command, "--mount", "type=volume,src=agent-workspace-run-1,dst=/workspace,readonly=false") {
		t.Fatalf("workspace volume mount missing from %#v", captured.Command)
	}
	if containsPair(captured.Command, "--env", "AGENT_PLATFORM_WORKFLOW_B64="+config.WorkflowPlan) {
		t.Fatalf("safe version probe was wrapped in Approval workflow: %#v", captured.Command)
	}
	if _, err := runner(context.Background(), processharness.Spec{Command: []string{"claude", "execute"}, Dir: "/workspace", Observer: &recordingObserver{}}, discardSink{}); err != nil {
		t.Fatal(err)
	}
	if !containsPair(captured.Command, "--env", "AGENT_PLATFORM_WORKFLOW_B64="+config.WorkflowPlan) {
		t.Fatalf("Execute Git workflow plan missing from %#v", captured.Command)
	}
}

func TestRunMountsAndProtectsAdapterScratchDirectory(t *testing.T) {
	scratch, err := os.MkdirTemp("", "agent-runtime-adapter-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(scratch)
	prompt := filepath.Join(scratch, "instruction.txt")
	if err := os.WriteFile(prompt, []byte("task"), 0o600); err != nil {
		t.Fatal(err)
	}

	var captured processharness.Spec
	prepared := ""
	config := validConfig(func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
		captured = spec
		return processharness.Result{}, nil
	}, func(context.Context, string) error { return nil })
	config.PrepareScratch = func(path string, uid, gid int) error {
		prepared = fmt.Sprintf("%s:%d:%d", path, uid, gid)
		return nil
	}
	config.ScratchDirectory = "/workspaces/runtime-scratch"
	config.AttachmentDirectory = "/workspaces/runtime-scratch/attachments"
	config.ConnectorDirectory = "/workspaces/runtime-scratch/connectors"
	config.CLIBrokerSocket = "/workspaces/runtime-scratch/broker/cli-broker.sock"
	runner, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner(context.Background(), processharness.Spec{
		Command: []string{"claude", "--message-file", prompt}, Dir: "/workspace",
	}, discardSink{})
	if err != nil {
		t.Fatal(err)
	}
	if prepared != scratch+":65532:65532" {
		t.Fatalf("prepared scratch = %q", prepared)
	}
	wantMount := "type=bind,src=" + scratch + ",dst=" + scratch + ",readonly=false"
	if !containsPair(captured.Command, "--mount", wantMount) {
		t.Fatalf("scratch mount %q missing from %#v", wantMount, captured.Command)
	}
	wantAttachmentMount := "type=bind,src=/workspaces/runtime-scratch/attachments,dst=/workspace/.agent-platform-attachments,readonly=true"
	if !containsPair(captured.Command, "--mount", wantAttachmentMount) {
		t.Fatalf("attachment mount %q missing from %#v", wantAttachmentMount, captured.Command)
	}
	wantConnectorMount := "type=bind,src=/workspaces/runtime-scratch/connectors,dst=/opt/agent-platform/connectors,readonly=true"
	if !containsPair(captured.Command, "--mount", wantConnectorMount) {
		t.Fatalf("CLI Connector mount %q missing from %#v", wantConnectorMount, captured.Command)
	}
	wantBrokerMount := "type=bind,src=/workspaces/runtime-scratch/broker,dst=/workspaces/runtime-scratch/broker,readonly=true"
	if !containsPair(captured.Command, "--mount", wantBrokerMount) || !containsPair(captured.Command, "--env", "AGENT_PLATFORM_CLI_SOCKET=/workspaces/runtime-scratch/broker/cli-broker.sock") {
		t.Fatalf("CLI broker socket mount or environment missing from %#v", captured.Command)
	}
}

func TestNewRejectsUnsafeConfiguration(t *testing.T) {
	tests := map[string]Config{
		"tagged image":                  {Image: "runtime:latest"},
		"wrong runtime":                 {Image: digestImage(), Runtime: "runc"},
		"root user":                     {Image: digestImage(), UID: 0, GID: 0},
		"relative credential":           {Image: digestImage(), CredentialDirectory: "credentials"},
		"relative native state":         {Image: digestImage(), CredentialDirectory: "/credentials", NativeStateDirectory: "state"},
		"relative attachment":           {Image: digestImage(), CredentialDirectory: "/credentials", ScratchDirectory: "/scratch", AttachmentDirectory: "attachments"},
		"attachment outside scratch":    {Image: digestImage(), CredentialDirectory: "/credentials", ScratchDirectory: "/scratch", AttachmentDirectory: "/other/attachments"},
		"relative CLI Connector":        {Image: digestImage(), CredentialDirectory: "/credentials", ScratchDirectory: "/scratch", ConnectorDirectory: "connectors"},
		"CLI Connector outside scratch": {Image: digestImage(), CredentialDirectory: "/credentials", ScratchDirectory: "/scratch", ConnectorDirectory: "/other/connectors"},
		"relative CLI broker socket":    {Image: digestImage(), CredentialDirectory: "/credentials", ScratchDirectory: "/scratch", CLIBrokerSocket: "cli-broker.sock"},
		"CLI broker outside scratch":    {Image: digestImage(), CredentialDirectory: "/credentials", ScratchDirectory: "/scratch", CLIBrokerSocket: "/other/broker/cli-broker.sock"},
		"native state for non Codex":    {Image: digestImage(), CredentialDirectory: "/credentials", NativeStateDirectory: "/state"},
		"relative resolver":             {Image: digestImage(), Egress: sandbox.EgressPublic, PublicEgressNetwork: "public", ResolverConfigFile: "resolv.conf"},
	}
	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			config.RuntimeCommand = "claude"
			config.Limits = sandbox.Limits{CPUs: 1, MemoryBytes: 1, PIDs: 1, TempBytes: 1}
			if _, err := New(config); err == nil {
				t.Fatal("expected invalid configuration")
			}
		})
	}
}

func TestRunRejectsCommandOrWorkspaceDrift(t *testing.T) {
	runner, err := New(validConfig(nil, nil))
	if err != nil {
		t.Fatalf("new container process: %v", err)
	}
	for _, spec := range []processharness.Spec{
		{Command: []string{"codex", "--version"}, Dir: "/workspace"},
		{Command: []string{"claude", "--version"}, Dir: "relative"},
		{Command: []string{"claude", "--version"}, Dir: "/other"},
		{Command: []string{"claude", "--version"}, Dir: "/workspace", Env: []string{"BAD-NAME=value"}},
		{Command: []string{"claude", "--version"}, Dir: "/workspace", Env: []string{"AGENT_PLATFORM_CLI_SOCKET=/forged"}},
	} {
		if _, err := runner(context.Background(), spec, discardSink{}); err == nil {
			t.Fatalf("expected drift rejection for %+v", spec)
		}
	}
}

func TestRunRejectsCredentialOverrideOfPlatformEnvironment(t *testing.T) {
	credentialDirectory := t.TempDir()
	environmentDirectory := filepath.Join(credentialDirectory, "env")
	if err := os.Mkdir(environmentDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(environmentDirectory, "OPENCLAW_CONFIG_PATH"), []byte("/untrusted/config.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := validConfig(nil, nil)
	config.CredentialDirectory = credentialDirectory
	runner, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner(context.Background(), processharness.Spec{
		Command: []string{"claude", "--version"}, Dir: "/workspace",
		Env: []string{"OPENCLAW_CONFIG_PATH=/trusted/config.json"},
	}, discardSink{})
	if err == nil || !strings.Contains(err.Error(), "cannot override platform variable") {
		t.Fatalf("override error = %v", err)
	}
}

func validConfig(runHost RunHost, cleanup Cleanup) Config {
	return Config{
		Image: digestImage(), RuntimeCommand: "claude", RunID: "run-1", Runtime: "runsc", UID: 65532, GID: 65532,
		WorkspaceDirectory:  "/workspace",
		CredentialDirectory: "/credentials", Egress: sandbox.EgressNone,
		Limits: sandbox.Limits{CPUs: 1, MemoryBytes: 1, PIDs: 1, TempBytes: 1},
		Name:   func() (string, error) { return "agent-runtime-test", nil }, RunHost: runHost, Cleanup: cleanup,
	}
}

func digestImage() string {
	return "registry.example/runtime@sha256:" + strings.Repeat("a", 64)
}

type recordingObserver struct{}

func (*recordingObserver) Observe(context.Context, processharness.Stream, []byte) error { return nil }

type processAwareRecordingObserver struct {
	process processharness.ProcessController
}

func (*processAwareRecordingObserver) Observe(context.Context, processharness.Stream, []byte) error {
	return nil
}
func (observer *processAwareRecordingObserver) BindProcess(process processharness.ProcessController) {
	observer.process = process
}

type recordingProcessController struct{}

func (*recordingProcessController) Pause() error {
	return errors.New("host process must not be paused directly")
}
func (*recordingProcessController) Resume() error {
	return errors.New("host process must not be resumed directly")
}

type discardSink struct{}

func (discardSink) Store(_ context.Context, output processharness.Output) error {
	_, err := io.Copy(io.Discard, output.Reader)
	return err
}

func containsPair(values []string, first, second string) bool {
	for index := 0; index+1 < len(values); index++ {
		if values[index] == first && values[index+1] == second {
			return true
		}
	}
	return false
}
