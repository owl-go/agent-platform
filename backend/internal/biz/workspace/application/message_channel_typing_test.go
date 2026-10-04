package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type typingTestRepository struct {
	MessageChannelRepository
	mu      sync.Mutex
	current *ChannelTypingJob
}

func (r *typingTestRepository) GetChannelTypingJob(context.Context, ExecutionJob) (*ChannelTypingJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil {
		return nil, nil
	}
	copy := *r.current
	return &copy, nil
}
func (r *typingTestRepository) set(running bool, revoked bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if revoked {
		r.current = nil
	} else {
		r.current.Running = running
	}
}

type typingTestSender struct {
	events      chan string
	fail        bool
	failRefresh bool
}

func (s *typingTestSender) BeginTyping(ctx context.Context, stored ChannelStored, credentials ChannelCredentials, message domain.ChannelMessage) (ChannelTypingSession, error) {
	s.events <- "begin"
	if s.fail {
		return nil, errors.New("typing unavailable")
	}
	if credentials["bot_token"] != "secret" || message.Reply["context_token"] != "context" || message.SenderID != "alice" {
		return nil, errors.New("missing protected typing context")
	}
	return s, nil
}
func (s *typingTestSender) Refresh(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.events <- "refresh"
	if s.failRefresh {
		return errors.New("refresh unavailable")
	}
	return nil
}
func (s *typingTestSender) Stop(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("unbounded stop")
	}
	s.events <- "stop"
	return nil
}
func typingTestFixture() (*MessageChannels, *typingTestRepository, *typingTestSender, ExecutionJob) {
	credentials, _ := json.Marshal(ChannelCredentials{"bot_token": "secret"})
	reply, _ := json.Marshal(map[string]string{"context_token": "context"})
	repo := &typingTestRepository{current: &ChannelTypingJob{Stored: ChannelStored{Channel: domain.MessageChannel{ID: "channel", Provider: "wechat", OwnerID: "owner", ConfigVersion: 1}, Ciphertext: credentials}, Message: domain.ChannelMessage{SenderID: "alice", EventID: "event"}, ReplyCiphertext: reply, Running: true}}
	sender := &typingTestSender{events: make(chan string, 100)}
	app := NewMessageChannels(repo, transportTestCipher{}, map[string]ChannelTransport{"wechat": {Typing: sender}}, true, "")
	return app, repo, sender, ExecutionJob{Kind: JobWorkflow, ID: "run", OwnerID: "owner", WorkflowID: "workflow"}
}
func awaitTypingEvent(t *testing.T, s *typingTestSender, want string) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-s.events:
			if event == want {
				return
			}
		case <-timer.C:
			t.Fatalf("typing event %s missing", want)
		}
	}
}
func TestChannelTypingRefreshStopsOnCancellationAndCleanupIsIdempotent(t *testing.T) {
	app, _, sender, job := typingTestFixture()
	ctx, cancel := context.WithCancel(context.Background())
	stop := app.trackExecution(ctx, job, 10*time.Millisecond)
	defer stop()
	awaitTypingEvent(t, sender, "refresh")
	awaitTypingEvent(t, sender, "refresh")
	cancel()
	stop()
	stop()
	awaitTypingEvent(t, sender, "stop")
	select {
	case event := <-sender.events:
		t.Fatalf("activity after stop: %s", event)
	default:
	}
}
func TestChannelTypingPausesForApprovalAndStopsWhenAuthorizationRevoked(t *testing.T) {
	app, repo, sender, job := typingTestFixture()
	stop := app.trackExecution(context.Background(), job, 10*time.Millisecond)
	defer stop()
	awaitTypingEvent(t, sender, "refresh")
	repo.set(false, false)
	awaitTypingEvent(t, sender, "stop")
	repo.set(true, false)
	awaitTypingEvent(t, sender, "begin")
	awaitTypingEvent(t, sender, "refresh")
	repo.set(false, true)
	awaitTypingEvent(t, sender, "stop")
	stop()
}
func TestChannelTypingDoesNotStartForNonWorkflowOrUnsupportedProvider(t *testing.T) {
	app, repo, sender, job := typingTestFixture()
	job.Kind = JobSession
	app.TrackExecution(context.Background(), job)()
	job.Kind = JobWorkflow
	repo.current.Stored.Channel.Provider = "unsupported"
	app.TrackExecution(context.Background(), job)()
	select {
	case event := <-sender.events:
		t.Fatalf("unsupported activity: %s", event)
	default:
	}
}

type typingWorkerRepository struct {
	terminalRepository
	sender    *typingTestSender
	succeeded bool
}

func (r *typingWorkerRepository) FinishSucceeded(context.Context, ExecutionJob, ExecutionResult) error {
	select {
	case event := <-r.sender.events:
		if event != "stop" {
			return errors.New("terminal committed before typing stopped")
		}
	default:
		if !r.sender.fail {
			return errors.New("typing cleanup missing")
		}
	}
	r.succeeded = true
	return nil
}

type typingWorkerExecutor struct {
	t      *testing.T
	sender *typingTestSender
}

func (e typingWorkerExecutor) Execute(context.Context, ExecutionJob, ProgressRecorder) (ExecutionResult, error) {
	if e.sender.fail {
		awaitTypingEvent(e.t, e.sender, "begin")
	} else {
		awaitTypingEvent(e.t, e.sender, "refresh")
	}
	return ExecutionResult{FinalMessage: "answer"}, nil
}
func TestWorkerTypingFailureDoesNotFailAnswerAndCleanupPrecedesTerminal(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "typing_unavailable"}[fail], func(t *testing.T) {
			app, _, sender, job := typingTestFixture()
			sender.fail = fail
			repo := &typingWorkerRepository{terminalRepository: terminalRepository{job: job}, sender: sender}
			worker, err := NewWorker(repo, typingWorkerExecutor{t: t, sender: sender})
			if err != nil {
				t.Fatal(err)
			}
			worker.EnableMessageChannels(app)
			worked, err := worker.ProcessNext(context.Background())
			if err != nil || !worked || !repo.succeeded {
				t.Fatalf("typing affected model completion: %v %v", worked, err)
			}
		})
	}
}

func TestChannelTypingRefreshFailureStillClearsIndicator(t *testing.T) {
	app, _, sender, job := typingTestFixture()
	sender.failRefresh = true
	events := make(chan string, 10)
	app.EnableTypingObserver(func(_, event string) { events <- event })
	stop := app.trackExecution(context.Background(), job, 10*time.Millisecond)
	defer stop()
	awaitTypingEvent(t, sender, "stop")
	stop()
	if event := <-events; event != "refresh_failed" {
		t.Fatalf("missing safe failure observation: %s", event)
	}
	if event := <-events; event != "stopped" {
		t.Fatalf("missing cleanup observation: %s", event)
	}
}

type typingTerminalRepository struct {
	terminalRepository
	sender *typingTestSender
	state  string
}

func (r *typingTerminalRepository) finish(state string) error {
	select {
	case event := <-r.sender.events:
		if event != "stop" {
			return errors.New("activity not stopped before terminal")
		}
	default:
		return errors.New("missing typing cleanup")
	}
	r.state = state
	return nil
}
func (r *typingTerminalRepository) FinishFailed(context.Context, ExecutionJob, ExecutionResult, string) error {
	return r.finish("failed")
}
func (r *typingTerminalRepository) FinishCancelled(context.Context, ExecutionJob, ExecutionResult) error {
	return r.finish("cancelled")
}

type typingExitExecutor struct {
	t        *testing.T
	sender   *typingTestSender
	repo     *typingTerminalRepository
	outcome  string
	shutdown context.CancelFunc
}

func (e typingExitExecutor) Execute(ctx context.Context, _ ExecutionJob, _ ProgressRecorder) (ExecutionResult, error) {
	awaitTypingEvent(e.t, e.sender, "refresh")
	if e.outcome == "failed" {
		return ExecutionResult{}, errors.New("model failed")
	}
	if e.outcome == "cancelled" {
		e.repo.requested.Store(true)
	} else {
		e.shutdown()
	}
	<-ctx.Done()
	return ExecutionResult{}, ctx.Err()
}
func TestWorkerClearsTypingForFailureUserCancellationAndShutdown(t *testing.T) {
	for _, outcome := range []string{"failed", "cancelled", "shutdown"} {
		t.Run(outcome, func(t *testing.T) {
			app, _, sender, job := typingTestFixture()
			repo := &typingTerminalRepository{terminalRepository: terminalRepository{job: job}, sender: sender}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			worker, err := NewWorker(repo, typingExitExecutor{t: t, sender: sender, repo: repo, outcome: outcome, shutdown: cancel})
			if err != nil {
				t.Fatal(err)
			}
			worker.EnableMessageChannels(app)
			worked, err := worker.ProcessNext(ctx)
			if err != nil || !worked {
				t.Fatalf("execution exit: %v %v", worked, err)
			}
			if outcome == "shutdown" {
				awaitTypingEvent(t, sender, "stop")
				if repo.state != "" {
					t.Fatal("shutdown incorrectly made Run terminal")
				}
			} else if repo.state != outcome {
				t.Fatalf("terminal outcome changed: %s", repo.state)
			}
		})
	}
}
