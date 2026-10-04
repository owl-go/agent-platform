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

type responseProgress struct {
	texts    []string
	previews []application.ChannelResponsePreview
}

func (p *responseProgress) RecordProgress(context.Context, application.ExecutionJob, application.ExecutionEvent) error {
	return nil
}
func (p *responseProgress) UpdateChannelResponse(_ context.Context, _ application.ExecutionJob, preview application.ChannelResponsePreview) {
	p.previews = append(p.previews, preview)
	if preview.Answer != "" {
		p.texts = append(p.texts, preview.Answer)
	}
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
		if len(p.previews) != 0 {
			t.Fatal("intermediate member or rejected event was streamed")
		}
	}
}

func TestChannelResponseProjectsPublicSummaryAndRealActivity(t *testing.T) {
	p := &responseProgress{}
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: "run"}
	sink := newChannelResponseSink(responseEventSink{}, p, job, [][]byte{[]byte("private-credential")}, true, "步骤 2/2")
	events := []struct {
		kind    agentruntime.EventKind
		payload string
	}{
		{agentruntime.EventRuntimeStarted, `{}`},
		{agentruntime.EventReasoningSummary, `{"summary":"先检查连接状态 private-credential，再确认消息路径。https://private.test/download"}`},
		{agentruntime.EventCommandRequested, `{"command":"internal command private-credential","input":"private arguments"}`},
		{agentruntime.EventCommandCompleted, `{"result":"private tool output"}`},
		{agentruntime.EventMessageDelta, `{"delta":"这是独立的最终回答。后续文字保持足够长度以通过安全缓冲区。This answer is public and remains separate."}`},
	}
	for _, event := range events {
		if err := sink.Publish(context.Background(), agentruntime.Event{RunID: "run", Kind: event.kind, Payload: []byte(event.payload)}); err != nil {
			t.Fatal(err)
		}
	}
	last := p.previews[len(p.previews)-1]
	if last.Summary != "先检查连接状态 [REDACTED]，再确认消息路径。" || last.ToolsCompleted != 1 || last.Status != "步骤 2/2 · 正在整理回答" || !strings.Contains(last.Answer, "独立的最终回答") {
		t.Fatalf("incorrect public projection: %+v", last)
	}
	for _, preview := range p.previews {
		encoded, _ := json.Marshal(preview)
		for _, forbidden := range []string{"private-credential", "private arguments", "private tool output", "internal command", "https://"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatal("leaked", forbidden)
			}
		}
	}
	// Earlier Team Members expose activity labels, without answer or summary text.
	p = &responseProgress{}
	sink = newChannelResponseSink(responseEventSink{}, p, job, nil, false)
	_ = sink.Publish(context.Background(), agentruntime.Event{RunID: "run", Kind: agentruntime.EventReasoningSummary, Payload: []byte(`{"summary":"another member's private draft"}`)})
	if len(p.previews) != 1 || p.previews[0].Summary != "" || p.previews[0].Answer != "" {
		t.Fatal(p.previews)
	}
}

func TestChannelResponseSummaryIsBoundedAndRedactedBeforeCropping(t *testing.T) {
	p := &responseProgress{}
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: "run"}
	secret := "sensitive-value-at-crop-boundary"
	sink := newChannelResponseSink(responseEventSink{}, p, job, [][]byte{[]byte(secret)}, true)
	summary := strings.Repeat("字", 595) + secret + strings.Repeat("公开", 50)
	payload, _ := json.Marshal(map[string]string{"summary": summary})
	if err := sink.Publish(context.Background(), agentruntime.Event{RunID: "run", Kind: agentruntime.EventReasoningSummary, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	got := p.previews[len(p.previews)-1].Summary
	if len([]rune(got)) != 600 || strings.Contains(got, "sensi") || !utf8.ValidString(got) {
		t.Fatal(got)
	}
	_ = sink.Publish(context.Background(), agentruntime.Event{RunID: "other-run", Kind: agentruntime.EventCommandCompleted, Payload: []byte(`{}`)})
	if len(p.previews) != 1 {
		t.Fatal("cross-Run progress accepted")
	}
}
