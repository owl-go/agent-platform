package runtimeexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/biz/workspace/application"
)

type responseProgress struct{ texts []string }

func (p *responseProgress) RecordProgress(context.Context, application.ExecutionJob, application.ExecutionEvent) error {
	return nil
}
func (p *responseProgress) UpdateChannelResponse(_ context.Context, _ application.ExecutionJob, text string) {
	p.texts = append(p.texts, text)
}

type responseEventSink struct{ fail bool }

func (s responseEventSink) Publish(context.Context, agentruntime.Event) error {
	if s.fail {
		return errors.New("storage unavailable")
	}
	return nil
}

func TestChannelResponseRedactsAcrossEveryDeltaBoundary(t *testing.T) {
	secret := "credential-that-must-never-leak"
	text := "这是公开回复。" + strings.Repeat("public ", 10) + secret + strings.Repeat(" trailing", 12)
	for split := 1; split < len(secret); split++ {
		p := &responseProgress{}
		job := application.ExecutionJob{Kind: application.JobWorkflow, ID: "run"}
		sink := newChannelResponseSink(responseEventSink{}, p, job, [][]byte{[]byte(secret)}, true)
		start := strings.Index(text, secret)
		chunks := []string{text[:start+split], text[start+split : start+len(secret)], text[start+len(secret):]}
		for _, chunk := range chunks {
			payload, _ := json.Marshal(map[string]string{"delta": chunk})
			if err := sink.Publish(context.Background(), agentruntime.Event{RunID: "run", Kind: agentruntime.EventMessageDelta, Payload: payload}); err != nil {
				t.Fatal(err)
			}
		}
		if len(p.texts) == 0 {
			t.Fatal("no streaming output")
		}
		for _, sent := range p.texts {
			if strings.Contains(sent, "credential-") || !utf8.ValidString(sent) {
				t.Fatalf("unsafe response at split %d: %q", split, sent)
			}
		}
		if !strings.Contains(p.texts[len(p.texts)-1], "[REDACTED]") {
			t.Fatal("credential was not redacted")
		}
	}
}
func TestChannelResponseDoesNotExposeURLsReasoningOrOtherMembers(t *testing.T) {
	for _, marker := range []string{"https://private.test/key?X-Amz-Signature=secret", "HTTP://private.test/", "/api/v1/files/key", "x-oss-signature=secret"} {
		p := &responseProgress{}
		job := application.ExecutionJob{Kind: application.JobWorkflow, ID: "run"}
		sink := newChannelResponseSink(responseEventSink{}, p, job, nil, true)
		for _, kind := range []agentruntime.EventKind{agentruntime.EventReasoningSummary, agentruntime.EventCommandCompleted} {
			_ = sink.Publish(context.Background(), agentruntime.Event{RunID: "run", Kind: kind, Payload: []byte(`{"delta":"internal secret"}`)})
		}
		text := strings.Repeat("public ", 8) + marker + strings.Repeat("private ", 8)
		for _, character := range text {
			payload, _ := json.Marshal(map[string]string{"delta": string(character)})
			_ = sink.Publish(context.Background(), agentruntime.Event{RunID: "run", Kind: agentruntime.EventMessageDelta, Payload: payload})
		}
		if len(p.texts) == 0 {
			t.Fatal("public prefix missing")
		}
		for _, sent := range p.texts {
			if strings.Contains(sent, "private") || strings.Contains(sent, "internal") || strings.Contains(sent, marker[:4]) {
				t.Fatalf("private output: %q", sent)
			}
		}
	}
	for _, final := range []bool{false, true} {
		p := &responseProgress{}
		sink := newChannelResponseSink(responseEventSink{fail: final}, p, application.ExecutionJob{Kind: application.JobWorkflow, ID: "run"}, nil, final)
		_ = sink.Publish(context.Background(), agentruntime.Event{RunID: "run", Kind: agentruntime.EventMessageDelta, Payload: []byte(`{"delta":"a very long answer that cannot bypass a failing persistence boundary"}`)})
		if len(p.texts) != 0 {
			t.Fatal("intermediate member or rejected event was streamed")
		}
	}
}
