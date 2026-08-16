package outputsink

import (
	"context"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestSinkStoresRuntimeOutputWithChecksum(t *testing.T) {
	provider := memory.New()
	sink, err := New(provider, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	contents := "runtime output\n"
	if err := sink.Store(context.Background(), processharness.Output{
		Stream: processharness.StreamStdout, Reader: strings.NewReader(contents), Size: int64(len(contents)), UTF8: true, Inline: true,
	}); err != nil {
		t.Fatal(err)
	}
	reader, object, err := provider.Get(context.Background(), "runs/run-1/runtime/stdout.log")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if object.Size != int64(len(contents)) || object.Metadata["run_id"] != "run-1" {
		t.Fatalf("stored Object = %+v", object)
	}
}

type recorderFunc func(context.Context, string, string, objectstore.Object) error

func (function recorderFunc) RecordRuntimeOutput(ctx context.Context, runID, stream string, object objectstore.Object) error {
	return function(ctx, runID, stream, object)
}

func TestSinkRecordsStoredObjectMetadata(t *testing.T) {
	provider := memory.New()
	recorded := false
	sink, err := NewForAttempt(provider, recorderFunc(func(_ context.Context, runID, stream string, object objectstore.Object) error {
		recorded = runID == "run-1" && stream == "stderr" && object.Key == "runs/run-1/attempts/attempt-1/runtime/stderr.log" && object.Metadata["attempt_id"] == "attempt-1"
		return nil
	}), "run-1", "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	contents := "failure\n"
	if err := sink.Store(context.Background(), processharness.Output{Stream: processharness.StreamStderr, Reader: strings.NewReader(contents), Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if !recorded {
		t.Fatal("stored Runtime output was not recorded as Artifact metadata")
	}
}
