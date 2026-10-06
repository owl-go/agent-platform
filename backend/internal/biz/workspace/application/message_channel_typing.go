package application

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

// The repository returns only a currently running, authorized channel execution.
// No provider credentials or reply context enter the ExecutionJob or Runtime.
type ChannelTypingJob struct {
	InboxID          string
	Response         ChannelResponseState
	ResponseRevision int64
	Stored           ChannelStored
	Message          domain.ChannelMessage
	ReplyCiphertext  []byte
	Running          bool
}
type ChannelTypingRepository interface {
	GetChannelTypingJob(context.Context, ExecutionJob) (*ChannelTypingJob, error)
}

const channelTypingInterval = 5 * time.Second
const channelTypingRequestTimeout = 3 * time.Second

// Configure before Worker startup. Observations contain only provider and a
// bounded lifecycle event, never identity, tickets, reply context or errors.
func (s *MessageChannels) EnableTypingObserver(observer func(string, string)) {
	s.typingObserver = observer
}
func (s *MessageChannels) observeTyping(provider, event string) {
	if s.typingObserver != nil {
		s.typingObserver(provider, event)
	}
}

// TrackExecution starts optional activity asynchronously. Its cleanup waits for
// the bounded stop request, including after cancellation and Worker shutdown.
func (s *MessageChannels) TrackExecution(ctx context.Context, job ExecutionJob) func() {
	return s.trackExecution(ctx, job, channelTypingInterval)
}
func (s *MessageChannels) trackExecution(ctx context.Context, job ExecutionJob, interval time.Duration) func() {
	repository, ok := s.repository.(ChannelTypingRepository)
	if !s.enabled || job.Kind != JobWorkflow || !ok {
		return func() {}
	}
	activityCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		var session ChannelTypingSession
		provider := ""
		started := false
		stop := func() {
			if session == nil {
				return
			}
			stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), channelTypingRequestTimeout)
			defer stopCancel()
			if err := session.Stop(stopCtx); err != nil {
				s.observeTyping(provider, "stop_failed")
			} else {
				s.observeTyping(provider, "stopped")
			}
			session = nil
			started = false
		}
		defer stop()
		timer := time.NewTicker(interval)
		defer timer.Stop()
		for {
			if activityCtx.Err() != nil {
				return
			}
			requestCtx, requestCancel := context.WithTimeout(activityCtx, channelTypingRequestTimeout)
			current, err := repository.GetChannelTypingJob(requestCtx, job)
			if err != nil {
				requestCancel()
				return
			}
			if current == nil {
				requestCancel()
				return
			}
			if !current.Running {
				requestCancel()
				stop()
			} else {
				provider = current.Stored.Channel.Provider
				typing := s.transports[provider].Typing
				if typing == nil {
					requestCancel()
					return
				}
				if session == nil {
					credentials, credentialErr := s.credentials(current.Stored)
					reply, replyErr := s.cipher.Decrypt(current.ReplyCiphertext, ChannelReplyAAD(current.Stored.Channel.ID, current.Message))
					if credentialErr != nil || replyErr != nil || json.Unmarshal(reply, &current.Message.Reply) != nil {
						requestCancel()
						return
					}
					session, err = typing.BeginTyping(requestCtx, current.Stored, credentials, current.Message)
					if err != nil {
						s.observeTyping(provider, "start_failed")
					}
				}
				if err == nil && session != nil && activityCtx.Err() == nil {
					err = session.Refresh(requestCtx)
					if err != nil {
						s.observeTyping(provider, "refresh_failed")
					} else if !started {
						started = true
						s.observeTyping(provider, "started")
					}
				}
				requestCancel()
				if err != nil {
					return
				}
			}
			select {
			case <-activityCtx.Done():
				return
			case <-timer.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(cancel); <-done }
}
