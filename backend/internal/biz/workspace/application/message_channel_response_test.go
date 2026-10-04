package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type responseTestRepository struct{ *typingTestRepository }

func (r *responseTestRepository) GetChannelResponseReceipt(ctx context.Context, _ ChannelStored, _ domain.ChannelMessage) (*ChannelTypingJob, error) {
	return r.GetChannelTypingJob(ctx, ExecutionJob{})
}
func (r *responseTestRepository) SaveChannelResponse(_ context.Context, job *ChannelTypingJob, state ChannelResponseState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == nil || r.current.ResponseRevision != job.ResponseRevision {
		return domain.ErrConflict
	}
	job.Response = state
	job.ResponseRevision++
	r.current.Response = state
	r.current.ResponseRevision = job.ResponseRevision
	return nil
}

type responseTestSender struct {
	events      chan string
	updates     chan ChannelResponsePreview
	createState string
}

func (r *responseTestSender) React(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage) (string, error) {
	r.events <- "react"
	return "reaction", nil
}
func (r *responseTestSender) ClearReaction(ctx context.Context, _ ChannelStored, _ ChannelCredentials, _ domain.ChannelMessage, _ string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	r.events <- "clear"
	return nil
}
func (r *responseTestSender) CreateResponse(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage, string) ChannelSendResult {
	r.events <- "create"
	if r.createState != "" {
		return ChannelSendResult{State: r.createState}
	}
	return ChannelSendResult{State: "sent", MessageID: "card"}
}
func (r *responseTestSender) UpdateResponse(_ context.Context, _ ChannelStored, _ ChannelCredentials, id string, preview ChannelResponsePreview, final bool) ChannelSendResult {
	r.updates <- preview
	if final {
		r.events <- "final:" + id + ":" + preview.Answer
	} else {
		r.events <- "update:" + id + ":" + preview.Answer
	}
	return ChannelSendResult{State: "sent", MessageID: id}
}
func (r *responseTestSender) Send(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage, string, string) ChannelSendResult {
	r.events <- "send"
	return ChannelSendResult{State: "sent"}
}
func responseFixture() (*MessageChannels, *responseTestRepository, *responseTestSender, ExecutionJob) {
	app, typing, _, job := typingTestFixture()
	typing.current.InboxID = "inbox"
	typing.current.Stored.Channel.Provider = "feishu"
	repo := &responseTestRepository{typing}
	sender := &responseTestSender{events: make(chan string, 100), updates: make(chan ChannelResponsePreview, 100)}
	app.repository = repo
	app.transports = map[string]ChannelTransport{"feishu": {Sender: sender, Response: sender}}
	return app, repo, sender, job
}
func awaitResponse(t *testing.T, s *responseTestSender, want string) {
	t.Helper()
	select {
	case got := <-s.events:
		if got != want {
			t.Fatalf("want %q got %q", want, got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing", want)
	}
}
func TestChannelResponseReceiptStreamAndTerminalShareOneCard(t *testing.T) {
	app, repo, sender, job := responseFixture()
	ctx := context.Background()
	current, _ := repo.GetChannelTypingJob(ctx, job)
	app.receiveFeedback(ctx, current.Stored, current.Message)
	awaitResponse(t, sender, "react")
	app.receiveFeedback(ctx, current.Stored, current.Message)
	progress, stop := app.TrackResponse(ctx, job, &responseNoopProgress{})
	defer stop()
	awaitResponse(t, sender, "create")
	awaitResponse(t, sender, "update:card:")
	progress.(ChannelResponseProgress).UpdateChannelResponse(ctx, job, ChannelResponsePreview{Answer: "public answer", Summary: "公开摘要"})
	awaitResponse(t, sender, "update:card:public answer")
	stop()
	awaitResponse(t, sender, "clear")
	current, _ = repo.GetChannelTypingJob(ctx, job)
	encoded, _ := json.Marshal(current.Response)
	delivery := &ChannelSendJob{Stored: current.Stored, Message: current.Message, InboxID: "inbox", Response: encoded, ResponseRevision: current.ResponseRevision, Delivery: domain.ChannelDelivery{Kind: "answer"}}
	result := app.sendResponse(ctx, delivery, ChannelCredentials{}, "final answer", sender)
	if result.State != "sent" || result.MessageID != "card" {
		t.Fatal(result)
	}
	awaitResponse(t, sender, "final:card:final answer")
	for {
		preview := <-sender.updates
		if preview.Answer == "final answer" {
			if preview.Summary != "公开摘要" {
				t.Fatal("terminal card lost summary", preview)
			}
			break
		}
	}
	select {
	case extra := <-sender.events:
		t.Fatal("duplicate operation", extra)
	default:
	}
}
func TestChannelResponseUnknownCreationIsNotAutomaticallyResent(t *testing.T) {
	app, repo, sender, job := responseFixture()
	sender.createState = "outcome_unknown"
	ctx := context.Background()
	_, stop := app.TrackResponse(ctx, job, &responseNoopProgress{})
	awaitResponse(t, sender, "create")
	stop()
	current, _ := repo.GetChannelTypingJob(ctx, job)
	if current.Response.Phase != "outcome_unknown" {
		t.Fatal(current.Response)
	}
	encoded, _ := json.Marshal(current.Response)
	result := app.sendResponse(ctx, &ChannelSendJob{Stored: current.Stored, Message: current.Message, Response: encoded, Delivery: domain.ChannelDelivery{Kind: "answer"}}, ChannelCredentials{}, "final", sender)
	if result.State != "outcome_unknown" {
		t.Fatal(result)
	}
	select {
	case extra := <-sender.events:
		t.Fatal("unsafe resend", extra)
	default:
	}
}

type responseNoopProgress struct{}

func (*responseNoopProgress) RecordProgress(context.Context, ExecutionJob, ExecutionEvent) error {
	return nil
}
func (*responseNoopProgress) RecordStageSettlement(context.Context, ExecutionJob, domain.ExpertStage, CreditSettlement) error {
	return errors.New("settlement sentinel")
}
func TestChannelResponseRecorderPreservesAtomicStageSettlement(t *testing.T) {
	recorder := &channelResponseRecorder{ProgressRecorder: &responseNoopProgress{}}
	if err := recorder.RecordStageSettlement(context.Background(), ExecutionJob{}, domain.ExpertStage{}, CreditSettlement{}); err == nil || err.Error() != "settlement sentinel" {
		t.Fatal("lost settlement delegation", err)
	}
}

func TestChannelResponseContinuationDoesNotReplaceFirstCard(t *testing.T) {
	app, _, sender, _ := responseFixture()
	result := app.sendResponse(context.Background(), &ChannelSendJob{Stored: ChannelStored{Channel: domain.MessageChannel{Provider: "feishu"}}, Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 2}}, ChannelCredentials{}, "continuation", sender)
	if result.State != "sent" {
		t.Fatal(result)
	}
	awaitResponse(t, sender, "send")
}

func TestChannelResponseInFlightReactionDoesNotBlockTerminalAnswer(t *testing.T) {
	app, _, sender, _ := responseFixture()
	result := app.sendResponse(context.Background(), &ChannelSendJob{Stored: ChannelStored{Channel: domain.MessageChannel{Provider: "feishu"}}, Response: []byte(`{"phase":"reacting"}`), Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}, ChannelCredentials{}, "final", sender)
	if result.State != "sent" {
		t.Fatal("a reaction was treated as unknown answer creation", result)
	}
	awaitResponse(t, sender, "send")
}

func TestChannelResponseRetainsSummaryWhenRunEndsBeforeNextTick(t *testing.T) {
	app, repo, sender, job := responseFixture()
	ctx := context.Background()
	progress, stop := app.TrackResponse(ctx, job, &responseNoopProgress{})
	awaitResponse(t, sender, "create")
	awaitResponse(t, sender, "update:card:")
	progress.(ChannelResponseProgress).UpdateChannelResponse(ctx, job, ChannelResponsePreview{Summary: "公开思考摘要", Status: "正在调用工具"})
	stop()
	current, _ := repo.GetChannelTypingJob(ctx, job)
	if current.Response.Summary != "公开思考摘要" {
		t.Fatal("lost public summary at shutdown", current.Response)
	}
	// A restarted Delivery can recover the summary without retaining an answer draft.
	encoded, _ := json.Marshal(current.Response)
	var recovered ChannelResponseState
	if json.Unmarshal(encoded, &recovered) != nil || recovered.Summary != "公开思考摘要" {
		t.Fatal(string(encoded))
	}
	recorder := progress.(*channelResponseRecorder)
	recorder.UpdateChannelResponse(ctx, ExecutionJob{ID: "another-run"}, ChannelResponsePreview{Summary: "wrong conversation"})
	if recorder.latest().Summary != "公开思考摘要" {
		t.Fatal("cross-Run progress accepted")
	}
}
