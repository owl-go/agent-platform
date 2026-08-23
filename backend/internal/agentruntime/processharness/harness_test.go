package processharness_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/credentials"
)

func TestRunCapturesStdoutAndStderr(t *testing.T) {
	sink := &recordingSink{}
	result, err := processharness.Run(context.Background(), helperSpec("stdout-stderr"), sink)
	if err != nil {
		t.Fatalf("run helper: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code: got %d, want 0", result.ExitCode)
	}
	if got := sink.text(processharness.StreamStdout); got != "hello stdout\n" {
		t.Fatalf("stdout: got %q", got)
	}
	if got := sink.text(processharness.StreamStderr); got != "hello stderr\n" {
		t.Fatalf("stderr: got %q", got)
	}
}

func TestRunObserverPausesAndResumesRealProcessGroup(t *testing.T) {
	marker := t.TempDir() + "/continued"
	observer := &approvalPauseObserver{paused: make(chan struct{}), decision: make(chan struct{})}
	spec := helperSpec("approval-pause")
	spec.Env = append(spec.Env, "APPROVAL_CONTINUED_MARKER="+marker)
	spec.Observer = observer
	done := make(chan error, 1)
	go func() {
		_, err := processharness.Run(context.Background(), spec, &recordingSink{})
		done <- err
	}()
	select {
	case <-observer.paused:
	case <-time.After(time.Second):
		t.Fatal("process was not paused")
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("paused process crossed the protected action boundary: %v", err)
	}
	close(observer.decision)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("process did not resume")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("resumed process did not continue: %v", err)
	}
}

func TestRunClassifiesBinaryAndLargeOutputAsArtifacts(t *testing.T) {
	tests := map[string]struct {
		scenario string
		utf8     bool
	}{
		"binary": {scenario: "binary", utf8: false},
		"large":  {scenario: "large", utf8: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			sink := &recordingSink{}
			spec := helperSpec(test.scenario)
			spec.InlineLimit = 4

			if _, err := processharness.Run(context.Background(), spec, sink); err != nil {
				t.Fatalf("run helper: %v", err)
			}
			output := sink.output(processharness.StreamStdout)
			if output.UTF8 != test.utf8 {
				t.Fatalf("UTF8: got %v, want %v", output.UTF8, test.utf8)
			}
			if output.Inline {
				t.Fatal("expected artifact output")
			}
		})
	}
}

func TestRunRejectsOutputBeyondHardLimit(t *testing.T) {
	spec := helperSpec("large")
	spec.MaxOutputBytes = 4

	_, err := processharness.Run(context.Background(), spec, &recordingSink{})
	if !errors.Is(err, processharness.ErrOutputLimit) {
		t.Fatalf("expected output limit error, got %v", err)
	}
}

func TestRunCancellationStopsTheProcessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	spec := helperSpec("spawn-child")
	spec.GracePeriod = 50 * time.Millisecond
	sink := &recordingSink{}

	_, err := processharness.Run(ctx, spec, sink)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	childPID, parseErr := strconv.Atoi(strings.TrimSpace(sink.text(processharness.StreamStdout)))
	if parseErr != nil {
		t.Fatalf("parse child PID: %v", parseErr)
	}
	defer syscall.Kill(childPID, syscall.SIGKILL)

	deadline := time.Now().Add(500 * time.Millisecond)
	for processExists(childPID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if processExists(childPID) {
		t.Fatalf("child process %d survived cancellation", childPID)
	}
}

func TestRunStopsProcessWhenOutputReachesHardLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	spec := helperSpec("unbounded-output")
	spec.MaxOutputBytes = 1024
	spec.GracePeriod = 50 * time.Millisecond
	started := time.Now()

	_, err := processharness.Run(ctx, spec, &recordingSink{})
	if !errors.Is(err, processharness.ErrOutputLimit) {
		t.Fatalf("expected output limit error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("output limit took too long to stop process: %s", elapsed)
	}
}

func TestRunAppliesOutputLimitAcrossBothStreams(t *testing.T) {
	spec := helperSpec("split-large")
	spec.MaxOutputBytes = 1024

	_, err := processharness.Run(context.Background(), spec, &recordingSink{})
	if !errors.Is(err, processharness.ErrOutputLimit) {
		t.Fatalf("expected combined output limit error, got %v", err)
	}
}

func TestRunStopsProcessWhenLineReachesHardLimit(t *testing.T) {
	spec := helperSpec("long-line")
	spec.MaxLineBytes = 8

	_, err := processharness.Run(context.Background(), spec, &recordingSink{})
	if !errors.Is(err, processharness.ErrLineLimit) {
		t.Fatalf("expected line limit error, got %v", err)
	}
}

func TestRunReportsNonZeroExitCode(t *testing.T) {
	result, err := processharness.Run(context.Background(), helperSpec("exit-7"), &recordingSink{})
	if err == nil {
		t.Fatal("expected a process exit error")
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code: got %d, want 7", result.ExitCode)
	}
}

func TestRunPreservesOutputSinkFailure(t *testing.T) {
	sinkErr := errors.New("artifact store unavailable")

	_, err := processharness.Run(context.Background(), helperSpec("stdout-stderr"), failingSink{err: sinkErr})
	if !errors.Is(err, sinkErr) {
		t.Fatalf("expected output sink error, got %v", err)
	}
}

func TestRunStopsProcessWhenOutputObserverFails(t *testing.T) {
	observerErr := errors.New("event store unavailable")
	spec := helperSpec("unbounded-output")
	spec.Observer = failingObserver{err: observerErr}
	spec.GracePeriod = 50 * time.Millisecond
	start := time.Now()

	_, err := processharness.Run(context.Background(), spec, &recordingSink{})
	if !errors.Is(err, observerErr) {
		t.Fatalf("expected observer error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("observer failure took too long to stop process: %s", elapsed)
	}
}

func TestRunForceKillsProcessAfterGracePeriod(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	spec := helperSpec("ignore-term")
	spec.GracePeriod = 50 * time.Millisecond
	started := time.Now()

	_, err := processharness.Run(ctx, spec, &recordingSink{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	elapsed := time.Since(started)
	if elapsed < 140*time.Millisecond || elapsed >= time.Second {
		t.Fatalf("force-kill timing outside grace period: %s", elapsed)
	}
}

func TestRunRedactsCredentialsBeforeOutputSink(t *testing.T) {
	const secret = "model-secret"
	spec := helperSpec("credential-output")
	spec.Env = append(spec.Env, "TEST_SECRET="+secret)
	recording := &recordingSink{}
	sink := processharness.NewRedactingSink(credentials.NewRedactor([]byte(secret)), recording)

	if _, err := processharness.Run(context.Background(), spec, sink); err != nil {
		t.Fatalf("run helper: %v", err)
	}
	for _, stream := range []processharness.Stream{processharness.StreamStdout, processharness.StreamStderr} {
		got := recording.text(stream)
		if strings.Contains(got, secret) {
			t.Fatalf("%s contains original credential: %q", stream, got)
		}
		if !strings.Contains(got, "[REDACTED]") {
			t.Fatalf("%s does not contain redaction marker: %q", stream, got)
		}
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(2)
	}

	switch args[1] {
	case "stdout-stderr":
		fmt.Fprintln(os.Stdout, "hello stdout")
		fmt.Fprintln(os.Stderr, "hello stderr")
	case "binary":
		_, _ = os.Stdout.Write([]byte{0xff, 0xfe, 0xfd})
	case "large":
		fmt.Fprint(os.Stdout, "0123456789")
	case "spawn-child":
		child := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "wait")
		child.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		child.Stdout = io.Discard
		child.Stderr = io.Discard
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		fmt.Fprintln(os.Stdout, child.Process.Pid)
		_ = os.Stdout.Sync()
		for {
			time.Sleep(time.Second)
		}
	case "wait":
		for {
			time.Sleep(time.Second)
		}
	case "unbounded-output":
		chunk := strings.Repeat("x", 512)
		for {
			fmt.Fprint(os.Stdout, chunk)
		}
	case "split-large":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 700))
		fmt.Fprint(os.Stderr, strings.Repeat("y", 700))
	case "long-line":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 32))
	case "exit-7":
		os.Exit(7)
	case "ignore-term":
		signal.Ignore(syscall.SIGTERM)
		for {
			time.Sleep(time.Second)
		}
	case "credential-output":
		fmt.Fprintln(os.Stdout, "stdout="+os.Getenv("TEST_SECRET"))
		fmt.Fprintln(os.Stderr, "stderr="+os.Getenv("TEST_SECRET"))
	case "approval-pause":
		fmt.Fprintln(os.Stdout, "approval requested")
		time.Sleep(200 * time.Millisecond)
		if err := os.WriteFile(os.Getenv("APPROVAL_CONTINUED_MARKER"), []byte("continued"), 0o600); err != nil {
			os.Exit(3)
		}
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func helperSpec(scenario string) processharness.Spec {
	return processharness.Spec{
		Command: []string{os.Args[0], "-test.run=TestHelperProcess", "--", scenario},
		Env:     []string{"GO_WANT_HELPER_PROCESS=1"},
	}
}

type recordingSink struct {
	outputs map[processharness.Stream][]byte
	meta    map[processharness.Stream]processharness.Output
}

type failingSink struct {
	err error
}

type failingObserver struct {
	err error
}

type approvalPauseObserver struct {
	controller processharness.ProcessController
	paused     chan struct{}
	decision   chan struct{}
}

func (observer *approvalPauseObserver) BindProcess(controller processharness.ProcessController) {
	observer.controller = controller
}

func (observer *approvalPauseObserver) Observe(context.Context, processharness.Stream, []byte) error {
	if err := observer.controller.Pause(); err != nil {
		return err
	}
	close(observer.paused)
	<-observer.decision
	return observer.controller.Resume()
}

func (o failingObserver) Observe(context.Context, processharness.Stream, []byte) error {
	return o.err
}

func (s failingSink) Store(context.Context, processharness.Output) error {
	return s.err
}

func (s *recordingSink) Store(_ context.Context, output processharness.Output) error {
	if s.outputs == nil {
		s.outputs = make(map[processharness.Stream][]byte)
		s.meta = make(map[processharness.Stream]processharness.Output)
	}
	data, err := io.ReadAll(output.Reader)
	if err != nil {
		return err
	}
	s.outputs[output.Stream] = data
	output.Reader = nil
	s.meta[output.Stream] = output
	return nil
}

func (s *recordingSink) output(stream processharness.Stream) processharness.Output {
	return s.meta[stream]
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

func (s *recordingSink) text(stream processharness.Stream) string {
	return string(s.outputs[stream])
}
