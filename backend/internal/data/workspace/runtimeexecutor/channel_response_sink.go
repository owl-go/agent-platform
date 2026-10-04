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
// This sink exposes answer text only for the final member, never Runtime events.
type channelResponseSink struct {
	next     agentruntime.EventSink
	progress application.ChannelResponseProgress
	job      application.ExecutionJob
	redactor *credentials.Redactor
	hold     int
	text     []byte
	disabled bool
}

func newChannelResponseSink(next agentruntime.EventSink, progress application.ProgressRecorder, job application.ExecutionJob, patterns [][]byte, finalMember bool) agentruntime.EventSink {
	public, ok := progress.(application.ChannelResponseProgress)
	if !ok || !finalMember || job.Kind != application.JobWorkflow {
		return next
	}
	hold := 32
	for _, value := range patterns {
		if len(value) > hold {
			hold = len(value)
		}
	}
	return &channelResponseSink{next: next, progress: public, job: job, redactor: credentials.NewRedactor(patterns...), hold: hold}
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
	// Never stream links. A signed capability may be recognized only much later
	// in a URL; send its terminal result through the existing private-link guard.
	// Preserve byte offsets when folding URL markers; Unicode case conversion
	// can otherwise change the number of bytes before a link.
	lower := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, string(s.text))
	for _, marker := range []string{"http://", "https://", "/api/v1/", "x-amz-signature", "x-oss-signature"} {
		if at := strings.Index(lower, marker); at >= 0 && at < end {
			end = at
		}
	}
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
		s.progress.UpdateChannelResponse(ctx, s.job, string(safePrefix[:common]))
	}
	clear(safe)
	clear(safePrefix)
	return nil
}

func (s *channelResponseSink) clear() { clear(s.text); s.text = nil }
