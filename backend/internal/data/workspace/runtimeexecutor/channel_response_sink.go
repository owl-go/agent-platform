package runtimeexecutor

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/credentials"
)

// Assemble before redaction: a credential can span any number of model deltas.
// Hold a bounded suffix until all possible credential/URL prefixes are resolved.
// Only the final member supplies text; other members expose fixed activity labels.
type channelResponseSink struct {
	next        agentruntime.EventSink
	progress    application.ChannelResponseProgress
	job         application.ExecutionJob
	redactor    *credentials.Redactor
	hold        int
	text        []byte
	disabled    bool
	finalMember bool
	preview     application.ChannelResponsePreview
	stage       string
}

func newChannelResponseSink(next agentruntime.EventSink, progress application.ProgressRecorder, job application.ExecutionJob, patterns [][]byte, finalMember bool, stage ...string) agentruntime.EventSink {
	public, ok := progress.(application.ChannelResponseProgress)
	if !ok || job.Kind != application.JobWorkflow {
		return next
	}
	hold := 32
	for _, value := range patterns {
		if len(value) > hold {
			hold = len(value)
		}
	}
	label := ""
	if len(stage) > 0 {
		label = stage[0]
	}
	return &channelResponseSink{next: next, progress: public, job: job, redactor: credentials.NewRedactor(patterns...), hold: hold, finalMember: finalMember, stage: label}
}
func (s *channelResponseSink) Publish(ctx context.Context, event agentruntime.Event) error {
	// Persist ordinary progress first. A failed persistence call cannot result in
	// external progress from an event that the platform rejected.
	if err := s.next.Publish(ctx, event); err != nil {
		return err
	}
	if event.RunID != s.job.ID || s.disabled {
		return nil
	}
	switch event.Kind {
	case agentruntime.EventRuntimeStarted:
		s.preview = application.ChannelResponsePreview{Status: "正在分析任务"}
	case agentruntime.EventReasoningSummary:
		if s.finalMember {
			var summary struct {
				Summary string `json:"summary"`
			}
			if len(event.Payload) <= 16*1024 && json.Unmarshal(event.Payload, &summary) == nil {
				// A summary is a complete public event, not a raw thinking delta.
				safe := s.redactor.Bytes([]byte(summary.Summary))
				end := publicTextEnd(safe, len(safe))
				runes := []rune(strings.TrimSpace(string(safe[:end])))
				s.preview.Summary = string(runes[:min(600, len(runes))])
				clear(safe)
			}
		}
		s.preview.Status = "正在分析任务"
	case agentruntime.EventCommandRequested:
		s.preview.Status = "正在调用工具"
	case agentruntime.EventCommandCompleted:
		s.preview.ToolsCompleted++
		s.preview.Status = "工具调用已结束，正在整理结果"
	case agentruntime.EventFileChanged:
		s.preview.Status = "正在更新文件"
	case agentruntime.EventApprovalRequested:
		s.preview.Status = "等待工作流拥有者确认"
	case agentruntime.EventRuntimeCompleted:
		s.preview.Status = "正在校验并保存结果"
	case agentruntime.EventMessageDelta, agentruntime.EventMessageCompleted:
		if !s.finalMember {
			return nil
		}
		s.preview.Status = "正在整理回答"
	default:
		return nil
	}
	defer func() {
		preview := s.preview
		if s.stage != "" {
			preview.Status = s.stage + " · " + preview.Status
		}
		s.progress.UpdateChannelResponse(ctx, s.job, preview)
	}()
	var value struct {
		Delta   string `json:"delta"`
		Message string `json:"message"`
	}
	switch event.Kind {
	case agentruntime.EventMessageDelta:
		if json.Unmarshal(event.Payload, &value) != nil {
			return nil
		}
		s.text = append(s.text, value.Delta...)
	case agentruntime.EventMessageCompleted:
		if json.Unmarshal(event.Payload, &value) != nil {
			return nil
		}
		// Completed messages can replace earlier drafts, but do not flush a trailing
		// secret prefix: a subsequent assistant message may still finish it.
		s.text = append(s.text[:0], value.Message...)
	default:
		return nil
	}
	if len(s.text) > 256*1024 {
		clear(s.text)
		s.text = nil
		s.disabled = true
		return nil
	}
	end := len(s.text) - s.hold
	if end <= 0 {
		return nil
	}
	end = publicTextEnd(s.text, end)
	// Redact the complete buffer before cropping. Cropping first would expose a
	// credential prefix whose remaining bytes lie in the withheld suffix.
	safe := s.redactor.Bytes(s.text)
	// Redaction changes offsets; use the original prefix length as an upper bound
	// only after ensuring its boundary is not inside a matching credential.
	prefix := string(s.text[:end])
	safePrefix := s.redactor.Bytes([]byte(prefix))
	// Match boundaries that would expose partial secrets inside the prefix.
	// Compare redacted full text against redacted prefix; keep only their common
	// prefix (a split match produces different bytes at its first secret byte).
	common := 0
	for common < len(safePrefix) && common < len(safe) && safePrefix[common] == safe[common] {
		common++
	}
	for common > 0 && !utf8.Valid(safePrefix[:common]) {
		common--
	}
	if common > 0 {
		s.preview.Answer = string(safePrefix[:common])
	}
	clear(safe)
	clear(safePrefix)
	return nil
}

func (s *channelResponseSink) clear() { clear(s.text); s.text = nil }

// Preserve byte offsets while folding ASCII URL markers. External progress never
// contains links: private capabilities can be recognized only later in a URL.
func publicTextEnd(text []byte, end int) int {
	lower := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, string(text))
	for _, marker := range []string{"http://", "https://", "/api/v1/", "x-amz-signature", "x-oss-signature"} {
		if at := strings.Index(lower, marker); at >= 0 && at < end {
			end = at
		}
	}
	return end
}
