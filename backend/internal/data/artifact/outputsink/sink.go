package outputsink

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/objectstore"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type Sink struct {
	provider  objectstore.Provider
	recorder  Recorder
	runID     string
	attemptID string
}

type Recorder interface {
	RecordRuntimeOutput(context.Context, string, string, objectstore.Object) error
}

func New(provider objectstore.Provider, runID string) (*Sink, error) {
	return NewWithRecorder(provider, nil, runID)
}

func NewWithRecorder(provider objectstore.Provider, recorder Recorder, runID string) (*Sink, error) {
	return NewForAttempt(provider, recorder, runID, "")
}

func NewForAttempt(provider objectstore.Provider, recorder Recorder, runID, attemptID string) (*Sink, error) {
	if provider == nil || !runIDPattern.MatchString(runID) || attemptID != "" && !runIDPattern.MatchString(attemptID) {
		return nil, fmt.Errorf("Object Store Provider and valid Run ID are required")
	}
	return &Sink{provider: provider, recorder: recorder, runID: runID, attemptID: attemptID}, nil
}

func FactoryWithRecorder(provider objectstore.Provider, recorder Recorder) func(string, string) processharness.OutputSink {
	return func(runID, attemptID string) processharness.OutputSink {
		sink, err := NewForAttempt(provider, recorder, runID, attemptID)
		if err != nil {
			return &errorSink{err: err}
		}
		return sink
	}
}

func Factory(provider objectstore.Provider) func(string) processharness.OutputSink {
	return func(runID string) processharness.OutputSink {
		sink, err := New(provider, runID)
		if err != nil {
			return &errorSink{err: err}
		}
		return sink
	}
}

func (sink *Sink) Store(ctx context.Context, output processharness.Output) error {
	if output.Stream != processharness.StreamStdout && output.Stream != processharness.StreamStderr {
		return fmt.Errorf("unsupported Runtime output stream %q", output.Stream)
	}
	file, err := os.CreateTemp("", "agent-runtime-artifact-*")
	if err != nil {
		return fmt.Errorf("create Runtime output upload spool: %w", err)
	}
	path := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(path)
	}()
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(output.Reader, output.Size+1))
	if err != nil {
		return fmt.Errorf("spool Runtime output: %w", err)
	}
	if written != output.Size {
		return fmt.Errorf("Runtime output size changed during storage")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	key := fmt.Sprintf("runs/%s/runtime/%s.log", sink.runID, output.Stream)
	if sink.attemptID != "" {
		key = fmt.Sprintf("runs/%s/attempts/%s/runtime/%s.log", sink.runID, sink.attemptID, output.Stream)
	}
	metadata := map[string]string{
		"run_id": sink.runID, "stream": string(output.Stream),
		"utf8": fmt.Sprintf("%t", output.UTF8), "inline": fmt.Sprintf("%t", output.Inline),
	}
	if sink.attemptID != "" {
		metadata["attempt_id"] = sink.attemptID
	}
	stored, err := sink.provider.Put(ctx, key, file, objectstore.PutOptions{
		Size: output.Size, SHA256: hex.EncodeToString(hash.Sum(nil)), ContentType: "text/plain; charset=utf-8",
		Metadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("store Runtime output Artifact: %w", err)
	}
	if sink.recorder != nil {
		if err := sink.recorder.RecordRuntimeOutput(ctx, sink.runID, string(output.Stream), stored); err != nil {
			deleteErr := sink.provider.Delete(ctx, key)
			return errors.Join(fmt.Errorf("record Runtime output Artifact: %w", err), deleteErr)
		}
	}
	return nil
}

type errorSink struct{ err error }

func (sink *errorSink) Store(context.Context, processharness.Output) error { return sink.err }
