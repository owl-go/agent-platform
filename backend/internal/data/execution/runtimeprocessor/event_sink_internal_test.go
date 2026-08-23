package runtimeprocessor

import (
	"context"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/platformprotocol"
)

func TestEventSinkRejectsDeliveryForAnotherReviewBranch(t *testing.T) {
	payload, err := platformprotocol.EncodeWorkflowDelivered("agent-platform/other", "0123456789abcdef0123456789abcdef01234567", []string{"main.go"})
	if err != nil {
		t.Fatal(err)
	}
	event, recognized, err := platformprotocol.Parse(payload)
	if err != nil || !recognized {
		t.Fatalf("Parse() = (%+v, %v, %v)", event, recognized, err)
	}
	sink := eventSink{runID: "run-1", reviewBranch: "agent-platform/expected"}
	err = sink.Publish(context.Background(), agentruntime.Event{RunID: "run-1", Kind: agentruntime.EventWorkflowDelivered, Payload: event.Payload, OccurredAt: time.Now().UTC()})
	if err == nil {
		t.Fatal("workflow delivery for another Review Branch was accepted")
	}
}
