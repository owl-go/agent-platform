package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	events          chan string
	updates         chan ChannelResponsePreview
	createState     string
	createCode      string
	updateState     string
	updateCode      string
	updateID        string
	receivedReplies chan string
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
func (r *responseTestSender) CreateResponse(_ context.Context, _ ChannelStored, _ ChannelCredentials, m domain.ChannelMessage, _ string, preview ChannelResponsePreview, final bool) ChannelSendResult {
	if r.receivedReplies != nil {
		r.receivedReplies <- m.Reply["req_id"]
	}
	if final {
		if r.createState != "" {
			r.events <- "create"
			return ChannelSendResult{State: r.createState, Code: r.createCode}
		}
		r.updates <- preview
		r.events <- "send"
		return ChannelSendResult{State: "sent", MessageID: "card"}
	}
	r.events <- "create"
	if r.createState != "" {
		return ChannelSendResult{State: r.createState, Code: r.createCode}
	}
	return ChannelSendResult{State: "sent", MessageID: "card"}
}
func (r *responseTestSender) UpdateResponse(_ context.Context, _ ChannelStored, _ ChannelCredentials, m domain.ChannelMessage, id string, preview ChannelResponsePreview, final bool) ChannelSendResult {
	if r.receivedReplies != nil {
		r.receivedReplies <- m.Reply["req_id"]
	}
	r.updates <- preview
	if r.updateState != "" {
		r.events <- "update"
		return ChannelSendResult{State: r.updateState, Code: r.updateCode}
	}
	if final {
		r.events <- "final:" + id + ":" + preview.Answer
	} else {
		r.events <- "update:" + id + ":" + preview.Answer
	}
	if r.updateID != "" {
		id = r.updateID
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

func TestChannelResponseFastRunCreatesStructuredTerminalPreview(t *testing.T) {
	app, _, sender, _ := responseFixture()
	result := app.sendResponse(context.Background(), &ChannelSendJob{Stored: ChannelStored{Channel: domain.MessageChannel{Provider: "feishu"}}, Response: []byte(`{"phase":"received","summary":"公开摘要"}`), Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}, ChannelCredentials{}, "**最终回答**", sender)
	if result.State != "sent" {
		t.Fatal(result)
	}
	awaitResponse(t, sender, "send")
	preview := <-sender.updates
	if preview.Summary != "公开摘要" || preview.Answer != "**最终回答**" {
		t.Fatal("flattened summary into answer", preview)
	}
}

func dingTalkResponseApplicationFixture() (*MessageChannels, *responseTestRepository, *responseTestSender, ExecutionJob) {
	return responseApplicationFixture("dingtalk")
}

func responseApplicationFixture(provider string) (*MessageChannels, *responseTestRepository, *responseTestSender, ExecutionJob) {
	app, repo, sender, job := responseFixture()
	repo.current.Stored.Channel.Provider = provider
	if provider == "wecom" {
		repo.current.Message.Reply = map[string]string{"req_id": "callback", "received_at": "123"}
		repo.current.ReplyCiphertext, _ = json.Marshal(repo.current.Message.Reply)
	}
	// This wrapper implements cards, without inventing unsupported reactions.
	app.transports = map[string]ChannelTransport{provider: {Sender: sender, Response: cardOnlyResponse{sender}, ResponseFallback: sender}}
	return app, repo, sender, job
}

type cardOnlyResponse struct{ sender *responseTestSender }

func (r cardOnlyResponse) CreateResponse(ctx context.Context, s ChannelStored, c ChannelCredentials, m domain.ChannelMessage, key string, preview ChannelResponsePreview, final bool) ChannelSendResult {
	return r.sender.CreateResponse(ctx, s, c, m, key, preview, final)
}
func (r cardOnlyResponse) UpdateResponse(ctx context.Context, s ChannelStored, c ChannelCredentials, m domain.ChannelMessage, id string, preview ChannelResponsePreview, final bool) ChannelSendResult {
	return r.sender.UpdateResponse(ctx, s, c, m, id, preview, final)
}

func TestChannelResponseFeedbackCreatesOneReceiptThenStreamsAndFinalizesIt(t *testing.T) {
	for _, provider := range []string{"dingtalk", "wecom"} {
		t.Run(provider, func(t *testing.T) {
			app, repo, sender, job := responseApplicationFixture(provider)
			ctx := context.Background()
			current, _ := repo.GetChannelTypingJob(ctx, job)
			app.receiveFeedback(ctx, current.Stored, current.Message)
			awaitResponse(t, sender, "create")
			app.receiveFeedback(ctx, current.Stored, current.Message)
			current, _ = repo.GetChannelTypingJob(ctx, job)
			if current.Response.Phase != "ready" || current.Response.MessageID != "card" || current.Response.ReactionID != "" {
				t.Fatal("receipt lost or invented a reaction", current.Response)
			}
			progress, stop := app.TrackResponse(ctx, job, &responseNoopProgress{})
			defer stop()
			awaitResponse(t, sender, "update:card:")
			progress.(ChannelResponseProgress).UpdateChannelResponse(ctx, job, ChannelResponsePreview{Answer: "partial answer", Summary: "公开摘要", Status: "正在调用工具"})
			awaitResponse(t, sender, "update:card:partial answer")
			stop()
			current, _ = repo.GetChannelTypingJob(ctx, job)
			encoded, _ := json.Marshal(current.Response)
			if strings.Contains(string(encoded), "partial answer") {
				t.Fatal("answer draft persisted in response handles")
			}
			result := app.sendResponse(ctx, &ChannelSendJob{Stored: current.Stored, Message: current.Message, Response: encoded, InboxID: current.InboxID, ResponseRevision: current.ResponseRevision, Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}, ChannelCredentials{}, "complete answer", sender)
			if result.State != "sent" || result.MessageID != "card" {
				t.Fatal(result)
			}
			awaitResponse(t, sender, "final:card:complete answer")
			select {
			case extra := <-sender.events:
				t.Fatal("extra receipt, reaction or duplicate answer", extra)
			default:
			}
		})
	}

}

func TestChannelResponseReceiptUnknownCreationNeverAutomaticallyDuplicates(t *testing.T) {
	for _, provider := range []string{"dingtalk", "wecom"} {
		t.Run(provider, func(t *testing.T) {
			app, repo, sender, job := responseApplicationFixture(provider)
			sender.createState = "outcome_unknown"
			ctx := context.Background()
			current, _ := repo.GetChannelTypingJob(ctx, job)
			app.receiveFeedback(ctx, current.Stored, current.Message)
			awaitResponse(t, sender, "create")
			app.receiveFeedback(ctx, current.Stored, current.Message)
			_, stop := app.TrackResponse(ctx, job, &responseNoopProgress{})
			stop()
			current, _ = repo.GetChannelTypingJob(ctx, job)
			encoded, _ := json.Marshal(current.Response)
			result := app.sendResponse(ctx, &ChannelSendJob{Stored: current.Stored, Message: current.Message, Response: encoded, Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}, ChannelCredentials{}, "final", sender)
			if result.State != "outcome_unknown" {
				t.Fatal("uncertain receipt auto-resent", result)
			}
			select {
			case extra := <-sender.events:
				t.Fatal("unsafe operation", extra)
			default:
			}
		})
	}

}

func TestDingTalkTerminalFallbackOnlyOnDefiniteCardRejection(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, test := range []struct {
			state, code string
			fallback    bool
		}{
			{"failed", "provider_rejected", true},
			{"failed", "provider_reply_invalid", false},
			{"retry_wait", "provider_rate_limited", false},
			{"outcome_unknown", "provider_send_unconfirmed", false},
		} {
			app, repo, sender, job := dingTalkResponseApplicationFixture()
			current, _ := repo.GetChannelTypingJob(context.Background(), job)
			state := ChannelResponseState{Phase: "received"}
			if update {
				state.Phase, state.MessageID = "ready", "card"
				sender.updateState, sender.updateCode = test.state, test.code
			} else {
				sender.createState, sender.createCode = test.state, test.code
			}
			encoded, _ := json.Marshal(state)
			result := app.sendResponse(context.Background(), &ChannelSendJob{Stored: current.Stored, Message: current.Message, Response: encoded, Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}, ChannelCredentials{}, "final", sender)
			if update {
				awaitResponse(t, sender, "update")
			} else {
				awaitResponse(t, sender, "create")
			}
			if test.fallback {
				awaitResponse(t, sender, "send")
				if result.State != "sent" {
					t.Fatal(result)
				}
			} else if result.State != test.state {
				t.Fatal("uncertain/invalid card escaped via fallback", result)
			}
			select {
			case extra := <-sender.events:
				t.Fatal("unexpected operation", extra)
			default:
			}
		}
	}
}

func TestChannelResponseWaitingCardDoesNotExposeActionDetails(t *testing.T) {
	for _, provider := range []string{"dingtalk", "wecom"} {
		t.Run(provider, func(t *testing.T) {
			app, repo, sender, job := responseApplicationFixture(provider)
			repo.current.Running = false
			repo.current.Response = ChannelResponseState{Phase: "ready", MessageID: "card"}
			_, stop := app.TrackResponse(context.Background(), job, &responseNoopProgress{})
			defer stop()
			awaitResponse(t, sender, "update:card:")
			preview := <-sender.updates
			if preview.Status != "等待工作流拥有者处理" || preview.Answer != "" {
				t.Fatal("waiting card lost fixed safe status", preview)
			}
			stop()
		})
	}

}

func TestChannelResponseFastTerminalWaitsForReceiptWithoutSendingAgain(t *testing.T) {
	app, repo, sender, job := dingTalkResponseApplicationFixture()
	current, _ := repo.GetChannelTypingJob(context.Background(), job)
	delivery := &ChannelSendJob{Stored: current.Stored, Message: current.Message, Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}
	for _, started := range []int64{time.Now().UnixMilli(), time.Now().Add(-time.Minute).UnixMilli(), 0} {
		delivery.Response, _ = json.Marshal(ChannelResponseState{Phase: "creating", CreatingAt: started})
		result := app.sendResponse(context.Background(), delivery, ChannelCredentials{}, "final", sender)
		if started == 0 || time.Since(time.UnixMilli(started)) > 15*time.Second {
			if result.State != "outcome_unknown" {
				t.Fatal("stale or legacy intent lost uncertainty", result)
			}
		} else if result.State != "retry_wait" || result.Code != "provider_send_pending" {
			t.Fatal("in-flight receipt blocked a fast terminal answer", result)
		}
	}
	select {
	case extra := <-sender.events:
		t.Fatal("intent wait sent another message", extra)
	default:
	}
	delivery.Response, _ = json.Marshal(ChannelResponseState{Phase: "ready", MessageID: "receipt-card"})
	if result := app.sendResponse(context.Background(), delivery, ChannelCredentials{}, "final", sender); result.State != "sent" {
		t.Fatal(result)
	}
	awaitResponse(t, sender, "final:receipt-card:final")
}

func TestChannelResponseStreamExpiryFallbackOnlyForWeCom(t *testing.T) {
	for _, provider := range []string{"wecom", "dingtalk"} {
		for _, code := range []string{"provider_stream_expired", "provider_reply_expired"} {
			for _, update := range []bool{false, true} {
				app, repo, sender, job := responseApplicationFixture(provider)
				current, _ := repo.GetChannelTypingJob(context.Background(), job)
				state := ChannelResponseState{Phase: "received"}
				if update {
					state.Phase, state.MessageID = "ready", "stream"
					sender.updateState, sender.updateCode = "expired", code
				} else {
					sender.createState, sender.createCode = "expired", code
				}
				encoded, _ := json.Marshal(state)
				result := app.sendResponse(context.Background(), &ChannelSendJob{Stored: current.Stored, Message: current.Message, Response: encoded, Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}, ChannelCredentials{}, "final", sender)
				if update {
					awaitResponse(t, sender, "update")
				} else {
					awaitResponse(t, sender, "create")
				}
				if provider == "wecom" && code == "provider_stream_expired" {
					awaitResponse(t, sender, "send")
					if result.State != "sent" {
						t.Fatal(result)
					}
				} else if result.State != "expired" {
					t.Fatal("reply expiry bypassed", provider, code, result)
				}
				select {
				case extra := <-sender.events:
					t.Fatal("unexpected operation", extra)
				default:
				}
			}
		}
	}
}

func TestWeComLegacyInboxRetainsOrdinaryMarkdownDelivery(t *testing.T) {
	app, repo, sender, job := responseApplicationFixture("wecom")
	current, _ := repo.GetChannelTypingJob(context.Background(), job)
	current.Message.Reply = nil
	result := app.sendResponse(context.Background(), &ChannelSendJob{Stored: current.Stored, Message: current.Message, Response: []byte(`{}`), Delivery: domain.ChannelDelivery{Kind: "answer", Chunk: 1}}, ChannelCredentials{}, "final", sender)
	if result.State != "sent" {
		t.Fatal(result)
	}
	awaitResponse(t, sender, "send")
	select {
	case extra := <-sender.events:
		t.Fatal("legacy inbox opened a stream", extra)
	default:
	}
}

func TestChannelResponseDecryptsBoundCallbackBeforeWorkerUpdates(t *testing.T) {
	app, repo, sender, job := responseApplicationFixture("wecom")
	repo.current.Message.Reply = nil
	repo.current.Response = ChannelResponseState{Phase: "ready", MessageID: "stream"}
	sender.receivedReplies = make(chan string, 10)
	_, stop := app.TrackResponse(context.Background(), job, &responseNoopProgress{})
	defer stop()
	awaitResponse(t, sender, "update:stream:")
	if reqID := <-sender.receivedReplies; reqID != "callback" {
		t.Fatal("worker lost encrypted original callback", reqID)
	}
}

func TestChannelResponseCorruptCallbackNeverReachesTransport(t *testing.T) {
	app, repo, sender, job := responseApplicationFixture("wecom")
	repo.current.ReplyCiphertext = []byte("corrupt")
	repo.current.Response = ChannelResponseState{Phase: "ready", MessageID: "stream"}
	_, stop := app.TrackResponse(context.Background(), job, &responseNoopProgress{})
	stop()
	select {
	case event := <-sender.events:
		t.Fatal("invalid callback was sent", event)
	default:
	}
}

func TestWeComCallbackCapabilityParticipatesInExactSecretRedaction(t *testing.T) {
	values := ChannelReplySecrets(map[string]string{"req_id": "protected-callback"})
	if len(values) != 1 || string(values[0]) != "protected-callback" {
		t.Fatal("callback capability omitted from shared redaction")
	}
}

func (r *responseTestRepository) ReceiveChannelMessage(_ context.Context, _ ChannelStored, m domain.ChannelMessage, reply []byte) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current.Message = m
	r.current.ReplyCiphertext = append([]byte(nil), reply...)
	return true, nil
}

func TestWeComDurableReceiveRehydratesCallbackForImmediateFeedback(t *testing.T) {
	app, repo, sender, job := responseApplicationFixture("wecom")
	stored := repo.current.Stored
	stored.Channel.Enabled = true
	stored.Channel.Audience = domain.ChannelAudience{SenderIDs: []string{"alice"}, AllowDirect: true}
	repo.current.Stored = stored
	sender.receivedReplies = make(chan string, 10)
	m := domain.ChannelMessage{EventID: "event", MessageID: "msg", SenderID: "alice", ChatID: "alice", Text: "question", OccurredAt: time.Now(), Reply: map[string]string{"req_id": "protected-callback", "received_at": "123"}}
	if err := app.Receive(context.Background(), stored, m); err != nil {
		t.Fatal(err)
	}
	awaitResponse(t, sender, "create")
	if reqID := <-sender.receivedReplies; reqID != "protected-callback" {
		t.Fatal("durable receive stripped immediate reply capability", reqID)
	}
	current, _ := repo.GetChannelTypingJob(context.Background(), job)
	if current.Message.Reply != nil || !strings.Contains(string(current.ReplyCiphertext), "protected-callback") {
		t.Fatal("repository boundary did not protect callback")
	}
	encoded, _ := json.Marshal(current.Response)
	if strings.Contains(string(encoded), "protected-callback") {
		t.Fatal("callback capability leaked into response state")
	}
}

func TestChannelResponseCheckpointsConfirmedProviderStreamClosure(t *testing.T) {
	app, repo, sender, job := responseApplicationFixture("wecom")
	repo.current.Response = ChannelResponseState{Phase: "ready", MessageID: "stream"}
	sender.updateID = "stream.closed"
	_, stop := app.TrackResponse(context.Background(), job, &responseNoopProgress{})
	defer stop()
	awaitResponse(t, sender, "update:stream:")
	stop()
	current, _ := repo.GetChannelTypingJob(context.Background(), job)
	if current.Response.MessageID != "stream.closed" {
		t.Fatal("confirmed stream closure lost", current.Response)
	}
}
