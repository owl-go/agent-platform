package cliadapter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/platformprotocol"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestAdapterDescribesPinnedRuntimeAndExecutesContract(t *testing.T) {
	driver := fakeDriver{}
	runner := &fakeProcessRunner{}
	adapter := New(driver, Config{
		Command:         []string{"fake-runtime"},
		ExpectedVersion: "1.2.3",
		OutputSink:      discardOutputSink{},
		RunProcess:      runner.Run,
	})

	descriptor, err := adapter.Describe(context.Background())
	if err != nil {
		t.Fatalf("describe runtime: %v", err)
	}
	if descriptor.Name != "fake" || descriptor.Version != "1.2.3" {
		t.Fatalf("descriptor = %+v", descriptor)
	}

	events := &recordingEventSink{}
	result, err := adapter.Execute(context.Background(), agentruntime.ExecuteRequest{
		RunID:          "run-1",
		WorkspacePath:  "/workspace",
		Instruction:    "fix tests",
		Model:          "configured-model",
		EnvironmentRef: "environment-1",
	}, events)
	if err != nil {
		t.Fatalf("execute runtime: %v", err)
	}
	if result.FinalMessage != "done" || result.ExitCode != 0 {
		t.Fatalf("result = %+v", result)
	}
	wantKinds := []agentruntime.EventKind{
		agentruntime.EventRuntimeStarted,
		agentruntime.EventMessageDelta,
		agentruntime.EventMessageCompleted,
		agentruntime.EventRuntimeCompleted,
	}
	if len(events.events) != len(wantKinds) {
		t.Fatalf("events = %+v", events.events)
	}
	for index, event := range events.events {
		if event.Sequence != int64(index+1) || event.Kind != wantKinds[index] {
			t.Fatalf("event %d = %+v", index, event)
		}
	}
}

func TestAdapterStopsWhenEventSinkFails(t *testing.T) {
	sinkErr := errors.New("event store unavailable")
	adapter := New(fakeDriver{}, Config{
		Command:         []string{"fake-runtime"},
		ExpectedVersion: "1.2.3",
		RunProcess:      (&fakeProcessRunner{}).Run,
	})

	_, err := adapter.Execute(context.Background(), agentruntime.ExecuteRequest{
		RunID: "run-1", WorkspacePath: t.TempDir(), Instruction: "fix", Model: "model", EnvironmentRef: "env-1",
	}, &failingEventSink{err: sinkErr, failAfter: 1})
	if agentruntime.ErrorCodeOf(err) != agentruntime.ErrorEventDeliveryFailed || !errors.Is(err, sinkErr) {
		t.Fatalf("event failure = %v", err)
	}
}

func TestAdapterPausesForControlledWorkflowApprovalEvent(t *testing.T) {
	runner := &approvalProcessRunner{}
	adapter := New(fakeDriver{}, Config{Command: []string{"fake-runtime"}, ExpectedVersion: "1.2.3", RunProcess: runner.Run})
	events := &recordingEventSink{}
	_, err := adapter.Execute(context.Background(), agentruntime.ExecuteRequest{
		RunID: "run-1", WorkspacePath: t.TempDir(), Instruction: "fix", Model: "model", EnvironmentRef: "env-1",
	}, events)
	if err != nil {
		t.Fatal(err)
	}
	if runner.controller.pauses != 1 || runner.controller.resumes != 1 {
		t.Fatalf("process controls = %+v", runner.controller)
	}
	found, delivered := false, false
	for _, event := range events.events {
		found = found || event.Kind == agentruntime.EventApprovalRequested
		delivered = delivered || event.Kind == agentruntime.EventWorkflowDelivered
	}
	if !found {
		t.Fatalf("events = %+v", events.events)
	}
	if !delivered {
		t.Fatalf("workflow delivery event was not emitted: %+v", events.events)
	}
}

func TestClassifyProcessErrorPreservesRuntimeClassification(t *testing.T) {
	cause := errors.New("provider rejected credentials")
	authenticationError := &agentruntime.Error{
		Code: agentruntime.ErrorAuthenticationFailed, Message: "runtime authentication failed", Cause: cause,
	}
	classified := classifyProcessError(context.Background(), errors.Join(errors.New("exit status 1"), authenticationError))
	if agentruntime.ErrorCodeOf(classified) != agentruntime.ErrorAuthenticationFailed || !errors.Is(classified, cause) {
		t.Fatalf("classified error = %v", classified)
	}
}

type fakeDriver struct{}

func (fakeDriver) Name() string                               { return "fake" }
func (fakeDriver) VersionArgs() []string                      { return []string{"--version"} }
func (fakeDriver) ParseVersion(output string) (string, error) { return strings.TrimSpace(output), nil }
func (fakeDriver) Build(agentruntime.ExecuteRequest, string) (Invocation, error) {
	return Invocation{Args: []string{"execute"}}, nil
}
func (fakeDriver) NewParser(string) Parser { return &fakeParser{} }

type fakeParser struct {
	result ParsedResult
}

func (p *fakeParser) Parse(_ processharness.Stream, line []byte) ([]ParsedEvent, error) {
	var value struct {
		Delta string `json:"delta"`
		Final string `json:"final"`
	}
	if err := json.Unmarshal(line, &value); err != nil {
		return nil, err
	}
	if value.Final != "" {
		p.result.FinalMessage = value.Final
		return nil, nil
	}
	return []ParsedEvent{{Kind: agentruntime.EventMessageDelta, Payload: map[string]string{"delta": value.Delta}}}, nil
}

func (p *fakeParser) Result() ParsedResult { return p.result }

type fakeProcessRunner struct {
	calls int
}

type approvalProcessRunner struct{ controller recordingController }

func (runner *approvalProcessRunner) Run(ctx context.Context, spec processharness.Spec, sink processharness.OutputSink) (processharness.Result, error) {
	if slicesContain(spec.Command, "--version") {
		_ = sink.Store(ctx, processharness.Output{Stream: processharness.StreamStdout, Reader: strings.NewReader("1.2.3\n"), Size: 6, UTF8: true, Inline: true})
		return processharness.Result{}, nil
	}
	aware, ok := spec.Observer.(processharness.ProcessAwareObserver)
	if !ok {
		return processharness.Result{}, errors.New("observer is not process-aware")
	}
	aware.BindProcess(&runner.controller)
	line, _ := platformprotocol.EncodeApprovalRequest("protected execution", "review")
	line = append(line, '\n')
	if err := spec.Observer.Observe(ctx, processharness.StreamStdout, line); err != nil {
		return processharness.Result{}, err
	}
	delivery, _ := platformprotocol.EncodeWorkflowDelivered("review", "0123456789abcdef0123456789abcdef01234567", []string{"main.go"})
	if err := spec.Observer.Observe(ctx, processharness.StreamStdout, append(delivery, '\n')); err != nil {
		return processharness.Result{}, err
	}
	if err := spec.Observer.Observe(ctx, processharness.StreamStdout, []byte(`{"final":"done"}`+"\n")); err != nil {
		return processharness.Result{}, err
	}
	return processharness.Result{}, nil
}

type recordingController struct{ pauses, resumes int }

func (controller *recordingController) Pause() error  { controller.pauses++; return nil }
func (controller *recordingController) Resume() error { controller.resumes++; return nil }

func (r *fakeProcessRunner) Run(ctx context.Context, spec processharness.Spec, sink processharness.OutputSink) (processharness.Result, error) {
	r.calls++
	if slicesContain(spec.Command, "--version") {
		_ = sink.Store(ctx, processharness.Output{Stream: processharness.StreamStdout, Reader: strings.NewReader("1.2.3\n"), Size: 6, UTF8: true, Inline: true})
		return processharness.Result{}, nil
	}
	for _, chunk := range []string{"{\"delta\":\"work", "ing\"}\n{\"final\":\"done\"}\n"} {
		if err := spec.Observer.Observe(ctx, processharness.StreamStdout, []byte(chunk)); err != nil {
			return processharness.Result{}, err
		}
	}
	return processharness.Result{ExitCode: 0}, nil
}

type recordingEventSink struct {
	events []agentruntime.Event
}

type failingEventSink struct {
	err       error
	count     int
	failAfter int
}

func (s *failingEventSink) Publish(context.Context, agentruntime.Event) error {
	s.count++
	if s.count > s.failAfter {
		return s.err
	}
	return nil
}

func (s *recordingEventSink) Publish(_ context.Context, event agentruntime.Event) error {
	s.events = append(s.events, event)
	return nil
}

func slicesContain(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
