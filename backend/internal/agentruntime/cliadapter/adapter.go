package cliadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/platformprotocol"
	"agent-platform/backend/internal/agentruntime/processharness"
)

type Invocation struct {
	Args  []string
	Env   []string
	Stdin io.Reader
}

type ParsedEvent struct {
	Kind    agentruntime.EventKind
	Payload any
}

type ParsedResult struct {
	FinalMessage  string
	CheckpointRef string
	Usage         agentruntime.Usage
	Error         error
}

type Parser interface {
	Parse(stream processharness.Stream, line []byte) ([]ParsedEvent, error)
	Result() ParsedResult
}

type Driver interface {
	Name() string
	VersionArgs() []string
	ParseVersion(output string) (string, error)
	Build(request agentruntime.ExecuteRequest, scratchDirectory string) (Invocation, error)
	NewParser(scratchDirectory string) Parser
}

type RunProcess func(context.Context, processharness.Spec, processharness.OutputSink) (processharness.Result, error)

type Config struct {
	Command              []string
	ExpectedVersion      string
	VerifiedCapabilities map[agentruntime.Capability]bool
	OutputSink           processharness.OutputSink
	RunProcess           RunProcess
	ScratchRoot          string
	MaxOutputBytes       int64
	MaxLineBytes         int64
	GracePeriod          time.Duration
}

type Adapter struct {
	driver Driver
	config Config
}

var _ agentruntime.Adapter = (*Adapter)(nil)

func New(driver Driver, config Config) *Adapter {
	if config.RunProcess == nil {
		config.RunProcess = processharness.Run
	}
	if config.OutputSink == nil {
		config.OutputSink = discardOutputSink{}
	}
	if config.MaxOutputBytes == 0 {
		config.MaxOutputBytes = 64 * 1024 * 1024
	}
	if config.MaxLineBytes == 0 {
		config.MaxLineBytes = config.MaxOutputBytes
	}
	return &Adapter{driver: driver, config: config}
}

func (a *Adapter) Describe(ctx context.Context) (agentruntime.Descriptor, error) {
	if len(a.config.Command) == 0 || a.config.ExpectedVersion == "" {
		return agentruntime.Descriptor{}, runtimeError(agentruntime.ErrorInvalidConfiguration, "runtime command and expected version are required", nil)
	}
	sink := &bufferSink{}
	command := appendCommand(a.config.Command, a.driver.VersionArgs())
	_, err := a.config.RunProcess(ctx, processharness.Spec{
		Command:        command,
		MaxOutputBytes: 1024 * 1024,
		MaxLineBytes:   64 * 1024,
		GracePeriod:    a.config.GracePeriod,
	}, sink)
	if err != nil {
		return agentruntime.Descriptor{}, runtimeError(agentruntime.ErrorRuntimeUnavailable, "read runtime version", err)
	}
	version, err := a.driver.ParseVersion(sink.stdout.String())
	if err != nil {
		return agentruntime.Descriptor{}, runtimeError(agentruntime.ErrorRuntimeUnavailable, "parse runtime version", err)
	}
	if version != a.config.ExpectedVersion {
		return agentruntime.Descriptor{}, runtimeError(agentruntime.ErrorRuntimeUnavailable, fmt.Sprintf("runtime version %q does not match pinned version %q", version, a.config.ExpectedVersion), nil)
	}
	capabilities := make(map[agentruntime.Capability]bool, len(a.config.VerifiedCapabilities))
	for capability, enabled := range a.config.VerifiedCapabilities {
		capabilities[capability] = enabled
	}
	return agentruntime.Descriptor{Name: a.driver.Name(), Version: version, Capabilities: capabilities}, nil
}

func (a *Adapter) Execute(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
	if err := request.Validate(); err != nil {
		return agentruntime.Result{}, err
	}
	scratch, err := os.MkdirTemp(a.config.ScratchRoot, "agent-runtime-adapter-*")
	if err != nil {
		return agentruntime.Result{}, runtimeError(agentruntime.ErrorInternalAdapter, "create runtime scratch directory", err)
	}
	defer os.RemoveAll(scratch)
	if err := os.Chmod(scratch, 0o700); err != nil {
		return agentruntime.Result{}, runtimeError(agentruntime.ErrorInternalAdapter, "protect runtime scratch directory", err)
	}
	invocation, err := a.driver.Build(request, scratch)
	if err != nil {
		return agentruntime.Result{}, runtimeError(agentruntime.ErrorInvalidConfiguration, "build runtime invocation", err)
	}
	emitter := &eventEmitter{runID: request.RunID, sink: events}
	if err := emitter.publish(ctx, agentruntime.EventRuntimeStarted, map[string]string{"runtime": a.driver.Name(), "model": request.Model}); err != nil {
		return agentruntime.Result{}, err
	}
	parser := a.driver.NewParser(scratch)
	observer := newLineObserver(parser, emitter)
	processResult, processErr := a.config.RunProcess(ctx, processharness.Spec{
		Command:        appendCommand(a.config.Command, invocation.Args),
		Dir:            request.WorkspacePath,
		Env:            invocation.Env,
		Stdin:          invocation.Stdin,
		MaxOutputBytes: a.config.MaxOutputBytes,
		MaxLineBytes:   a.config.MaxLineBytes,
		GracePeriod:    a.config.GracePeriod,
		Observer:       observer,
	}, a.config.OutputSink)
	if flushErr := observer.Flush(ctx); flushErr != nil {
		processErr = errors.Join(processErr, flushErr)
	}
	parsed := parser.Result()
	if parsed.Error != nil {
		processErr = errors.Join(processErr, parsed.Error)
	}
	result := agentruntime.Result{
		FinalMessage:           parsed.FinalMessage,
		ExitCode:               processResult.ExitCode,
		CheckpointRef:          parsed.CheckpointRef,
		Usage:                  parsed.Usage,
		ModelInvocationStarted: processResult.Started,
	}
	if parsed.FinalMessage != "" && processErr == nil && !emitter.published(agentruntime.EventMessageCompleted) {
		if err := emitter.publish(ctx, agentruntime.EventMessageCompleted, map[string]string{"message": parsed.FinalMessage}); err != nil {
			return result, err
		}
	}
	if processErr != nil {
		_ = emitter.publish(context.WithoutCancel(ctx), agentruntime.EventRuntimeFailed, map[string]any{"exit_code": processResult.ExitCode})
		return result, classifyProcessError(ctx, processErr)
	}
	if err := emitter.publish(ctx, agentruntime.EventRuntimeCompleted, map[string]any{"exit_code": processResult.ExitCode}); err != nil {
		return result, err
	}
	return result, nil
}

type eventEmitter struct {
	mu       sync.Mutex
	runID    string
	sequence int64
	sink     agentruntime.EventSink
	kinds    map[agentruntime.EventKind]bool
}

func (e *eventEmitter) publish(ctx context.Context, kind agentruntime.EventKind, payload any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	encoded, err := json.Marshal(payload)
	if err != nil {
		return runtimeError(agentruntime.ErrorInternalAdapter, "encode runtime event", err)
	}
	e.sequence++
	if err := e.sink.Publish(ctx, agentruntime.Event{
		RunID: e.runID, Sequence: e.sequence, Kind: kind, OccurredAt: time.Now().UTC(), Payload: encoded,
	}); err != nil {
		return runtimeError(agentruntime.ErrorEventDeliveryFailed, "publish runtime event", err)
	}
	if e.kinds == nil {
		e.kinds = make(map[agentruntime.EventKind]bool)
	}
	e.kinds[kind] = true
	return nil
}

func (e *eventEmitter) published(kind agentruntime.EventKind) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.kinds[kind]
}

type lineObserver struct {
	mu      sync.Mutex
	parser  Parser
	emitter *eventEmitter
	buffers map[processharness.Stream][]byte
	process processharness.ProcessController
}

func newLineObserver(parser Parser, emitter *eventEmitter) *lineObserver {
	return &lineObserver{parser: parser, emitter: emitter, buffers: make(map[processharness.Stream][]byte)}
}

func (o *lineObserver) BindProcess(process processharness.ProcessController) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.process = process
}

func (o *lineObserver) Observe(ctx context.Context, stream processharness.Stream, data []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buffers[stream] = append(o.buffers[stream], data...)
	for {
		index := bytes.IndexByte(o.buffers[stream], '\n')
		if index < 0 {
			return nil
		}
		line := bytes.Clone(o.buffers[stream][:index])
		o.buffers[stream] = o.buffers[stream][index+1:]
		if err := o.parseLine(ctx, stream, line); err != nil {
			return err
		}
	}
}

func (o *lineObserver) Flush(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, stream := range []processharness.Stream{processharness.StreamStdout, processharness.StreamStderr} {
		if len(o.buffers[stream]) == 0 {
			continue
		}
		if err := o.parseLine(ctx, stream, o.buffers[stream]); err != nil {
			return err
		}
		o.buffers[stream] = nil
	}
	return nil
}

func (o *lineObserver) parseLine(ctx context.Context, stream processharness.Stream, line []byte) error {
	platformEvent, recognized, err := platformprotocol.Parse(line)
	var parsed []ParsedEvent
	if err == nil && recognized {
		parsed = []ParsedEvent{{Kind: agentruntime.EventKind(platformEvent.Kind), Payload: json.RawMessage(platformEvent.Payload)}}
	} else if err == nil {
		parsed, err = o.parser.Parse(stream, line)
	}
	if err != nil {
		return runtimeError(agentruntime.ErrorInternalAdapter, "parse runtime output", err)
	}
	for _, event := range parsed {
		if event.Kind == agentruntime.EventApprovalRequested {
			if o.process == nil {
				return runtimeError(agentruntime.ErrorInternalAdapter, "pause Runtime for Approval", errors.New("process controller is unavailable"))
			}
			if err := o.process.Pause(); err != nil {
				return runtimeError(agentruntime.ErrorRuntimeUnavailable, "pause Runtime for Approval", err)
			}
			if err := o.emitter.publish(ctx, event.Kind, event.Payload); err != nil {
				return err
			}
			if err := o.process.Resume(); err != nil {
				return runtimeError(agentruntime.ErrorRuntimeUnavailable, "resume Runtime after Approval", err)
			}
			continue
		}
		if err := o.emitter.publish(ctx, event.Kind, event.Payload); err != nil {
			return err
		}
	}
	return nil
}

type bufferSink struct {
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (s *bufferSink) Store(_ context.Context, output processharness.Output) error {
	destination := &s.stderr
	if output.Stream == processharness.StreamStdout {
		destination = &s.stdout
	}
	_, err := io.Copy(destination, output.Reader)
	return err
}

type discardOutputSink struct{}

func (discardOutputSink) Store(_ context.Context, output processharness.Output) error {
	_, err := io.Copy(io.Discard, output.Reader)
	return err
}

func appendCommand(command, arguments []string) []string {
	combined := make([]string, 0, len(command)+len(arguments))
	combined = append(combined, command...)
	combined = append(combined, arguments...)
	return combined
}

func classifyProcessError(ctx context.Context, err error) error {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return runtimeError(agentruntime.ErrorTimedOut, "runtime timed out", err)
	case errors.Is(ctx.Err(), context.Canceled):
		return runtimeError(agentruntime.ErrorInterrupted, "runtime interrupted", err)
	case agentruntime.ErrorCodeOf(err) != agentruntime.ErrorInternalAdapter:
		return err
	default:
		return runtimeError(agentruntime.ErrorCommandFailed, "runtime command failed", err)
	}
}

func runtimeError(code agentruntime.ErrorCode, message string, cause error) error {
	return &agentruntime.Error{Code: code, Message: message, Cause: cause}
}

func ParseVersionToken(output, prefix string) (string, error) {
	value := strings.TrimSpace(output)
	value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if value == "" || strings.ContainsAny(value, "\r\n\t ") {
		return "", fmt.Errorf("unexpected version output %q", output)
	}
	return value, nil
}
