package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type ChannelResponseRepository interface {
	GetChannelResponseReceipt(context.Context, ChannelStored, domain.ChannelMessage) (*ChannelTypingJob, error)
	SaveChannelResponse(context.Context, *ChannelTypingJob, ChannelResponseState) error
}

func (s *MessageChannels) receiveFeedback(ctx context.Context, stored ChannelStored, message domain.ChannelMessage) {
	repository, ok := s.repository.(ChannelResponseRepository)
	response := s.transports[stored.Channel.Provider].Response
	if !ok || response == nil {
		return
	}
	// Feedback must not consume the provider's event acknowledgement window.
	reaction, reacts := response.(ChannelReactionSender)
	timeout := time.Second
	if !reacts {
		timeout = channelTypingRequestTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	current, err := repository.GetChannelResponseReceipt(requestCtx, stored, message)
	if err != nil || current == nil || current.Response.Phase != "" {
		return
	}
	state := ChannelResponseState{Phase: "reacting"}
	if !reacts {
		state.Phase = "creating"
		state.CreatingAt = time.Now().UnixMilli()
	}
	c, err := s.credentials(current.Stored)
	if err != nil {
		return
	}
	defer clear(c)
	if repository.SaveChannelResponse(requestCtx, current, state) != nil {
		return
	}
	if reacts {
		state.ReactionID, _ = reaction.React(requestCtx, current.Stored, c, current.Message)
		state.Phase = "received"
	} else {
		result := response.CreateResponse(requestCtx, current.Stored, c, current.Message, current.InboxID, ChannelResponsePreview{Status: "已收到，正在准备执行"}, false)
		state.MessageID, state.Phase = result.MessageID, "ready"
		if result.State != "sent" {
			state.Phase = result.State
		}
		state.CreatingAt = 0
	}
	saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer saveCancel()
	saveErr := repository.SaveChannelResponse(saveCtx, current, state)
	// A short Run, rejected admission or revocation may finish while the
	// reaction request is in flight. Do not leave that late feedback behind.
	eligible, readErr := repository.GetChannelResponseReceipt(saveCtx, current.Stored, current.Message)
	if state.ReactionID != "" && (saveErr != nil || readErr != nil || eligible == nil) {
		if reacts && reaction.ClearReaction(saveCtx, current.Stored, c, current.Message, state.ReactionID) == nil && saveErr == nil {
			state.ReactionID = ""
			_ = repository.SaveChannelResponse(saveCtx, current, state)
		}
	}
}

type channelResponseRecorder struct {
	ProgressRecorder
	mu      sync.Mutex
	preview ChannelResponsePreview
	runID   string
}

func (r *channelResponseRecorder) RecordStageSettlement(ctx context.Context, job ExecutionJob, stage domain.ExpertStage, settlement CreditSettlement) error {
	delegate, ok := r.ProgressRecorder.(interface {
		RecordStageSettlement(context.Context, ExecutionJob, domain.ExpertStage, CreditSettlement) error
	})
	if !ok {
		return errors.New("progress recorder cannot atomically settle an intermediate Stage")
	}
	return delegate.RecordStageSettlement(ctx, job, stage, settlement)
}

func (r *channelResponseRecorder) UpdateChannelResponse(ctx context.Context, job ExecutionJob, preview ChannelResponsePreview) {
	if ctx.Err() != nil || job.ID != r.runID {
		return
	}
	r.mu.Lock()
	summary := []rune(preview.Summary)
	preview.Summary = string(summary[:min(600, len(summary))])
	r.preview = preview
	r.mu.Unlock()
}
func (r *channelResponseRecorder) latest() ChannelResponsePreview {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.preview
}

// Network updates are coalesced outside the Runtime progress path. A slow card
// service cannot block model execution or replace its result with an error.
func (s *MessageChannels) TrackResponse(ctx context.Context, job ExecutionJob, progress ProgressRecorder) (ProgressRecorder, func()) {
	repository, ok := s.repository.(ChannelResponseRepository)
	typingRepository, typingOK := s.repository.(ChannelTypingRepository)
	if !s.enabled || job.Kind != JobWorkflow || !ok || !typingOK {
		return progress, func() {}
	}
	recorder := &channelResponseRecorder{ProgressRecorder: progress, runID: job.ID, preview: ChannelResponsePreview{Status: "正在准备执行环境"}}
	activityCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		lastPreview := ChannelResponsePreview{}
		started := time.Now()
		var last *ChannelTypingJob
		defer func() {
			// Flush the final public summary even for Runs shorter than one tick.
			// Query current authorization again; stale state cannot grant access.
			cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), channelTypingRequestTimeout)
			defer cleanupCancel()
			current, err := typingRepository.GetChannelTypingJob(cleanupCtx, job)
			if err == nil && current != nil {
				state := current.Response
				summary := recorder.latest().Summary
				if state.Summary != summary {
					state.Summary = summary
					_ = repository.SaveChannelResponse(cleanupCtx, current, state)
				}
				last = current
			}
			if last == nil || last.Response.ReactionID == "" {
				return
			}
			s.clearResponseReaction(cleanupCtx, repository, last)
		}()
		for activityCtx.Err() == nil {
			requestCtx, requestCancel := context.WithTimeout(activityCtx, channelTypingRequestTimeout)
			current, err := typingRepository.GetChannelTypingJob(requestCtx, job)
			if err != nil || current == nil {
				requestCancel()
				return
			}
			last = current
			transport := s.transports[current.Stored.Channel.Provider].Response
			if transport == nil {
				requestCancel()
				return
			}
			if !current.Running {
				s.clearResponseReaction(requestCtx, repository, current)
				preview := ChannelResponsePreview{Status: "等待工作流拥有者处理", Summary: current.Response.Summary}
				if current.Response.Phase == "ready" && preview != lastPreview {
					if c, err := s.credentials(current.Stored); err == nil {
						result := transport.UpdateResponse(requestCtx, current.Stored, c, current.Response.MessageID, preview, false)
						clear(c)
						if result.State == "sent" {
							lastPreview = preview
						}
					}
				}
			} else {
				c, credentialErr := s.credentials(current.Stored)
				if credentialErr != nil {
					requestCancel()
					return
				}
				state := current.Response
				if state.Phase == "" || state.Phase == "received" {
					state.Phase = "creating"
					state.CreatingAt = time.Now().UnixMilli()
					if repository.SaveChannelResponse(requestCtx, current, state) == nil {
						result := transport.CreateResponse(requestCtx, current.Stored, c, current.Message, current.InboxID, recorder.latest(), false)
						state.MessageID, state.Phase = result.MessageID, "ready"
						if result.State != "sent" {
							state.Phase = result.State
						}
						state.CreatingAt = 0
						saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), channelTypingRequestTimeout)
						_ = repository.SaveChannelResponse(saveCtx, current, state)
						saveCancel()
					}
				}
				if current.Response.Phase == "ready" {
					preview := recorder.latest()
					preview.ElapsedSeconds = int64(time.Since(started) / time.Second)
					if preview.Summary != current.Response.Summary {
						state := current.Response
						state.Summary = preview.Summary
						if repository.SaveChannelResponse(requestCtx, current, state) != nil {
							requestCancel()
							return
						}
					}
					if preview != lastPreview {
						result := transport.UpdateResponse(requestCtx, current.Stored, c, current.Response.MessageID, preview, false)
						if result.State == "sent" {
							lastPreview = preview
						}
					}
				}
			}
			requestCancel()
			select {
			case <-activityCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	var once sync.Once
	return recorder, func() { once.Do(cancel); <-done }
}

func (s *MessageChannels) clearResponseReaction(ctx context.Context, repository ChannelResponseRepository, current *ChannelTypingJob) {
	if current.Response.ReactionID == "" {
		return
	}
	c, err := s.credentials(current.Stored)
	response, ok := s.transports[current.Stored.Channel.Provider].Response.(ChannelReactionSender)
	if err != nil || !ok {
		return
	}
	if response.ClearReaction(ctx, current.Stored, c, current.Message, current.Response.ReactionID) == nil {
		state := current.Response
		state.ReactionID = ""
		_ = repository.SaveChannelResponse(ctx, current, state)
	}
}

func (s *MessageChannels) sendResponse(ctx context.Context, job *ChannelSendJob, c ChannelCredentials, text string, sender ChannelSender) ChannelSendResult {
	response := s.transports[job.Stored.Channel.Provider].Response
	repository, ok := s.repository.(ChannelResponseRepository)
	if response == nil || !ok || job.Delivery.Kind == "validation" || job.Delivery.Chunk > 1 {
		return sender.Send(ctx, job.Stored, c, job.Message, text, job.Delivery.ID)
	}
	var state ChannelResponseState
	if json.Unmarshal(job.Response, &state) != nil {
		return ChannelSendResult{State: "failed", Code: "reply_unavailable"}
	}
	current := &ChannelTypingJob{InboxID: job.InboxID, Stored: job.Stored, Message: job.Message, Response: state, ResponseRevision: job.ResponseRevision}
	s.clearResponseReaction(ctx, repository, current)
	preview := ChannelResponsePreview{Answer: text}
	if job.Delivery.Kind == "answer" {
		preview.Summary = state.Summary
	}
	var result ChannelSendResult
	if state.MessageID == "" {
		if state.Phase == "creating" && state.CreatingAt > 0 {
			elapsed := time.Now().Sub(time.UnixMilli(state.CreatingAt))
			if elapsed >= 0 && elapsed < 15*time.Second {
				// A fast Run can finish during receipt creation. Re-read the
				// persisted handle after a bounded wait, without sending again.
				return ChannelSendResult{State: "retry_wait", Code: "provider_send_pending", RetryAfter: time.Second}
			}
		}
		if state.Phase == "creating" || state.Phase == "outcome_unknown" {
			return ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
		}
		result = response.CreateResponse(ctx, job.Stored, c, job.Message, job.Delivery.ID, preview, true)
	} else {
		result = response.UpdateResponse(ctx, job.Stored, c, state.MessageID, preview, true)
	}
	if fallback := s.transports[job.Stored.Channel.Provider].ResponseFallback; result.State == "failed" && result.Code == "provider_rejected" && fallback != nil {
		return fallback.Send(ctx, job.Stored, c, job.Message, text, job.Delivery.ID)
	}
	return result
}
