package sandbox

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCreateFailsClosedWhenRunscIsUnavailable(t *testing.T) {
	executor := &fakeExecutor{responses: []fakeResponse{{output: `{"runc":{"path":"runc"}}`}}}
	runner := NewDockerRunner(executor, validConfig())

	_, err := runner.Create(context.Background(), validCreateSpec())
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Fatalf("expected runtime unavailable, got %v", err)
	}
	if len(executor.calls) != 1 || executor.calls[0][0] != "info" {
		t.Fatalf("unexpected Docker calls: %v", executor.calls)
	}
}

func TestCreateAppliesIsolationAndResourcePolicy(t *testing.T) {
	executor := &fakeExecutor{responses: []fakeResponse{
		{output: `{"runsc":{"path":"runsc"},"runc":{"path":"runc"}}`},
		{output: "container-id\n"},
	}}
	runner := NewDockerRunner(executor, validConfig())

	container, err := runner.Create(context.Background(), validCreateSpec())
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}
	if container.ID != "container-id" || container.RunID != "run-1" {
		t.Fatalf("container = %+v", container)
	}
	args := executor.calls[1]
	for _, required := range [][]string{
		{"create"},
		{"--runtime", "runsc"},
		{"--user", "65532:65532"},
		{"--read-only"},
		{"--cap-drop", "ALL"},
		{"--security-opt", "no-new-privileges"},
		{"--network", "agent-public-egress"},
		{"--memory", "536870912"},
		{"--pids-limit", "256"},
		{"--cpus", "2"},
		{"--mount", "type=volume,src=workspace-run-1,dst=/workspace,rw"},
		{"--mount", "type=bind,src=/private/credentials/run-1,dst=/run/agent-credentials,ro"},
		{"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=67108864"},
		{"--label", "agent-platform.managed=true"},
		{"--label", "agent-platform.run-id=run-1"},
		{"--label", "agent-platform.egress=public"},
		{"registry.example/runtime@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"agent-entrypoint", "execute"},
	} {
		if !containsSequence(args, required) {
			t.Errorf("Docker args do not contain %q: %v", required, args)
		}
	}
	if slices.Contains(args, "/var/run/docker.sock") {
		t.Fatalf("Docker socket was mounted: %v", args)
	}
}

func TestStopUsesGracePeriodAndDestroyIsIdempotent(t *testing.T) {
	notFound := &CommandError{ExitCode: 1, Stderr: "No such container: container-id"}
	executor := &fakeExecutor{responses: []fakeResponse{
		{},
		{},
		{err: notFound},
	}}
	runner := NewDockerRunner(executor, validConfig())

	if err := runner.Stop(context.Background(), "container-id", 1500*time.Millisecond); err != nil {
		t.Fatalf("stop sandbox: %v", err)
	}
	if !containsSequence(executor.calls[0], []string{"stop", "--time", "2", "container-id"}) {
		t.Fatalf("stop args: %v", executor.calls[0])
	}
	if err := runner.Destroy(context.Background(), "container-id"); err != nil {
		t.Fatalf("destroy sandbox: %v", err)
	}
	if err := runner.Destroy(context.Background(), "container-id"); err != nil {
		t.Fatalf("repeat destroy sandbox: %v", err)
	}
}

func TestStartFailsClosedWhenCreatedContainerDriftsFromPolicy(t *testing.T) {
	executor := &fakeExecutor{responses: []fakeResponse{{output: `[{
		"Id":"container-id",
		"Created":"2026-08-15T00:00:00Z",
		"Config":{"User":"65532:65532","Labels":{"agent-platform.managed":"true","agent-platform.run-id":"run-1"}},
		"HostConfig":{"Runtime":"runc","ReadonlyRootfs":true,"NetworkMode":"agent-public-egress"},
		"State":{"Status":"created"}
	}]`}}}
	runner := NewDockerRunner(executor, validConfig())

	err := runner.Start(context.Background(), "container-id")
	if !errors.Is(err, ErrIsolationDrift) {
		t.Fatalf("expected isolation drift, got %v", err)
	}
	if len(executor.calls) != 1 || executor.calls[0][0] != "inspect" {
		t.Fatalf("container was started despite drift: %v", executor.calls)
	}
}

func TestStartRunsContainerAfterInspectingFullPolicy(t *testing.T) {
	executor := &fakeExecutor{responses: []fakeResponse{
		{output: validInspectionJSON("container-id", "run-1", "created", "2026-08-15T00:00:00Z")},
		{},
	}}
	runner := NewDockerRunner(executor, validConfig())

	if err := runner.Start(context.Background(), "container-id"); err != nil {
		t.Fatalf("start sandbox: %v", err)
	}
	if len(executor.calls) != 2 || executor.calls[0][0] != "inspect" || executor.calls[1][0] != "start" {
		t.Fatalf("start calls = %v", executor.calls)
	}
}

func TestInspectReturnsSandboxState(t *testing.T) {
	executor := &fakeExecutor{responses: []fakeResponse{{output: `[{
		"Id":"container-id",
		"Created":"2026-08-15T00:00:00Z",
		"Config":{"User":"65532:65532","Labels":{"agent-platform.managed":"true","agent-platform.run-id":"run-1"}},
		"HostConfig":{"Runtime":"runsc","ReadonlyRootfs":true,"NetworkMode":"agent-public-egress"},
		"State":{"Status":"running"}
	}]`}}}
	runner := NewDockerRunner(executor, validConfig())

	inspection, err := runner.Inspect(context.Background(), "container-id")
	if err != nil {
		t.Fatalf("inspect sandbox: %v", err)
	}
	if inspection.RunID != "run-1" || inspection.State != "running" || inspection.Runtime != "runsc" || !inspection.ReadOnlyRootfs {
		t.Fatalf("inspection = %+v", inspection)
	}
}

func TestReconcileDestroysOnlyOldInactiveContainers(t *testing.T) {
	old := "2026-08-15T00:00:00Z"
	recent := "2026-08-15T02:00:00Z"
	executor := &fakeExecutor{responses: []fakeResponse{
		{output: "active-id\norphan-id\nrecent-id\n"},
		{output: inspectJSON("active-id", "run-active", old)},
		{output: inspectJSON("orphan-id", "run-orphan", old)},
		{},
		{output: inspectJSON("recent-id", "run-recent", recent)},
	}}
	runner := NewDockerRunner(executor, validConfig())
	active := map[string]struct{}{"run-active": {}}

	destroyed, err := runner.Reconcile(context.Background(), active, time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("reconcile sandboxes: %v", err)
	}
	if !slices.Equal(destroyed, []string{"orphan-id"}) {
		t.Fatalf("destroyed containers = %v", destroyed)
	}
	if !containsSequence(executor.calls[3], []string{"rm", "--force", "--volumes", "orphan-id"}) {
		t.Fatalf("orphan destroy call = %v", executor.calls[3])
	}
}

func validConfig() DockerConfig {
	return DockerConfig{
		Runtime:             "runsc",
		PublicEgressNetwork: "agent-public-egress",
		UID:                 65532,
		GID:                 65532,
		CredentialRoot:      "/private/credentials",
	}
}

func validCreateSpec() CreateSpec {
	return CreateSpec{
		RunID:                "run-1",
		Image:                "registry.example/runtime@sha256:" + strings.Repeat("a", 64),
		Command:              []string{"agent-entrypoint", "execute"},
		WorkspaceVolume:      "workspace-run-1",
		CredentialDirectory:  "/private/credentials/run-1",
		NonSecretEnvironment: []string{"RUNTIME=codex"},
		Egress:               EgressPublic,
		Limits: Limits{
			CPUs:        2,
			MemoryBytes: 512 * 1024 * 1024,
			PIDs:        256,
			TempBytes:   64 * 1024 * 1024,
		},
	}
}

type fakeResponse struct {
	output string
	err    error
}

type fakeExecutor struct {
	responses []fakeResponse
	calls     [][]string
}

func (e *fakeExecutor) Run(_ context.Context, args ...string) (string, error) {
	e.calls = append(e.calls, slices.Clone(args))
	if len(e.responses) == 0 {
		return "", nil
	}
	response := e.responses[0]
	e.responses = e.responses[1:]
	return response.output, response.err
}

func containsSequence(values, sequence []string) bool {
	for index := 0; index+len(sequence) <= len(values); index++ {
		if slices.Equal(values[index:index+len(sequence)], sequence) {
			return true
		}
	}
	return false
}

func inspectJSON(id, runID, created string) string {
	return `[{"Id":"` + id + `","Created":"` + created + `","Config":{"User":"65532:65532","Labels":{"agent-platform.managed":"true","agent-platform.run-id":"` + runID + `"}},"HostConfig":{"Runtime":"runsc","ReadonlyRootfs":true,"NetworkMode":"agent-public-egress"},"State":{"Status":"exited"}}]`
}

func validInspectionJSON(id, runID, state, created string) string {
	return `[{"Id":"` + id + `","Created":"` + created + `","Config":{"User":"65532:65532","Labels":{"agent-platform.managed":"true","agent-platform.run-id":"` + runID + `","agent-platform.egress":"public"}},"HostConfig":{"Runtime":"runsc","ReadonlyRootfs":true,"NetworkMode":"agent-public-egress","CapDrop":["ALL"],"SecurityOpt":["no-new-privileges"],"Memory":536870912,"NanoCpus":2000000000,"PidsLimit":256,"Tmpfs":{"/tmp":"rw,noexec,nosuid,nodev,size=67108864"}},"State":{"Status":"` + state + `"},"Mounts":[{"Type":"volume","Source":"workspace-run-1","Destination":"/workspace","RW":true},{"Type":"bind","Source":"/private/credentials/run-1","Destination":"/run/agent-credentials","RW":false}]}]`
}
