package gormrepo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	accountrepo "agent-platform/backend/internal/data/account/gormrepo"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"
	"agent-platform/backend/internal/secretcrypto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type channelRecordingAdapter struct {
	mu     sync.Mutex
	sends  []string
	result application.ChannelSendResult
}

func (a *channelRecordingAdapter) Identify(context.Context, application.ChannelCredentials, string) (application.ChannelIdentity, error) {
	return application.ChannelIdentity{ID: "test-bot", Name: "test"}, nil
}
func (a *channelRecordingAdapter) Configure(context.Context, application.ChannelStored, application.ChannelCredentials, string) error {
	return nil
}
func (a *channelRecordingAdapter) Callback(context.Context, application.ChannelStored, application.ChannelCredentials, http.Header, []byte) (application.ChannelCallback, error) {
	return application.ChannelCallback{}, nil
}
func (a *channelRecordingAdapter) Send(_ context.Context, _ application.ChannelStored, _ application.ChannelCredentials, _ domain.ChannelMessage, text, key string) application.ChannelSendResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sends = append(a.sends, text)
	return a.result
}

type channelFixture struct {
	db              *gorm.DB
	repo            *Repository
	app             *application.MessageChannels
	adapter         *channelRecordingAdapter
	channel         domain.MessageChannel
	owner, workflow string
}

func newChannelFixture(t *testing.T) *channelFixture {
	t.Helper()
	db := conversationTestDatabase(t)
	repo := New(db, creditsrepo.New(db))
	t.Cleanup(func() {
		if err := repo.releaseWorkerClaimLock(context.Background()); err != nil {
			t.Error(err)
		}
	})
	owner, connection, model, workflow := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner)
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','test')`, connection, owner)
	exec(`INSERT INTO model_provider_credential_versions(connection_id,connection_version,api_key_ciphertext) VALUES(?,1,'test')`, connection)
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name) VALUES(?,?,'model','Model')`, model, connection)
	defaults, _ := json.Marshal(map[string]string{"codex": model})
	exec(`INSERT INTO personal_settings(user_id,default_runtime_engine,runtime_model_defaults) VALUES(?,'codex',?::jsonb)`, owner, string(defaults))
	exec(`INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Workflow','Original goal','workspace/test')`, workflow, owner)
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	adapter := &channelRecordingAdapter{result: application.ChannelSendResult{State: "sent", MessageID: "receipt"}}
	app := application.NewMessageChannels(repo, box, map[string]application.ChannelTransport{"telegram": {Account: adapter, WebhookReceiver: adapter, Sender: adapter}}, true, "https://workspace.example.test")
	channel, err := app.Save(context.Background(), owner, workflow, "", 0, domain.MessageChannel{Provider: "telegram", Name: "Test bot", Audience: domain.ChannelAudience{SenderIDs: []string{"alice", "bob"}, AllowDirect: true}}, application.ChannelCredentials{"bot_token": "protected-test-token"})
	if err != nil {
		t.Fatal(err)
	}
	return &channelFixture{db: db, repo: repo, app: app, adapter: adapter, channel: channel, owner: owner, workflow: workflow}
}
func (f *channelFixture) stored(t *testing.T) application.ChannelStored {
	t.Helper()
	c, err := f.repo.GetMessageChannel(context.Background(), f.owner, f.workflow, f.channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.channel = c.Channel
	return c
}
func (f *channelFixture) readySend(t *testing.T) {
	t.Helper()
	if err := f.db.Model(&channelRecord{}).Where("id=?", f.channel.ID).Update("send_after", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
}
func (f *channelFixture) enable(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	c, err := f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "validate")
	if err != nil {
		t.Fatal(err)
	}
	f.channel = c
	stored := f.stored(t)
	message := f.message("validation-"+c.ValidationCode, "alice", c.ValidationCode)
	if err = f.app.Receive(ctx, stored, message); err != nil {
		t.Fatal(err)
	}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("validation send: %v %v", worked, err)
	}
	f.stored(t)
	if f.channel.ValidationState != "passed" {
		t.Fatal("validation not committed")
	}
	f.channel, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "enable")
	if err != nil {
		t.Fatal(err)
	}
	f.readySend(t)
}
func (f *channelFixture) message(id, sender, text string) domain.ChannelMessage {
	return domain.ChannelMessage{EventID: id, MessageID: id, SenderID: sender, ChatID: "chat", Text: text, OccurredAt: time.Now().UTC(), Reply: map[string]string{}}
}
func (f *channelFixture) receive(t *testing.T, id, sender, text string) {
	t.Helper()
	if err := f.app.Receive(context.Background(), f.stored(t), f.message(id, sender, text)); err != nil {
		t.Fatal(err)
	}
}
func (f *channelFixture) admit(t *testing.T) {
	t.Helper()
	if worked, err := f.app.ProcessInbox(context.Background()); err != nil || !worked {
		t.Fatalf("admission: %v %v", worked, err)
	}
}

func TestChannelValidationDedupConversationAndSnapshots(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	stored := f.stored(t)
	message := f.message("event-1", "alice", "first")
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- f.app.Receive(ctx, stored, message) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	f.admit(t)
	if worked, err := f.app.ProcessInbox(ctx); err != nil || worked {
		t.Fatalf("duplicate queued: %v %v", worked, err)
	}
	if err := f.db.Model(&workflowRecord{}).Where("id=?", f.workflow).Update("goal", "Changed goal for future conversations").Error; err != nil {
		t.Fatal(err)
	}
	f.receive(t, "event-2", "alice", "follow-up")
	f.admit(t)
	f.receive(t, "event-3", "bob", "separate")
	f.admit(t)
	var runs []runRecord
	if err := f.db.Order("queued_at,id").Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("Run count: %d", len(runs))
	}
	if runs[0].ID != runs[1].ConversationID || runs[1].TurnNumber != 2 || runs[2].ConversationID == runs[0].ID {
		t.Fatal("conversation isolation failed")
	}
	for _, run := range runs {
		if run.Trigger != "message_channel" {
			t.Fatal("wrong provenance")
		}
		var plan domain.ExecutionSnapshot
		_ = json.Unmarshal(run.WorkflowSnapshot, &plan)
		expectedGoal := "Original goal"
		if run.ConversationID == runs[2].ID {
			expectedGoal = "Changed goal for future conversations"
		}
		if plan.Goal != expectedGoal {
			t.Fatal("frozen or new-conversation goal incorrect")
		}

		if strings.Contains(string(run.WorkflowSnapshot), "protected-test-token") {
			t.Fatal("channel credential leaked")
		}
	}
	if _, err := f.repo.GetMessageChannel(ctx, uuid.NewString(), f.workflow, f.channel.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("owner isolation failed")
	}
}

type failingChannelCommit struct{}

func (failingChannelCommit) Commit() error   { return errors.New("forced commit failure") }
func (failingChannelCommit) Rollback() error { return nil }
func (failingChannelCommit) Cleanup() error  { return nil }
func TestChannelTerminalOutboxRollbackRetryAndDisable(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	f.receive(t, "question", "alice", "question")
	f.admit(t)
	var run runRecord
	if err := f.db.Where("trigger='message_channel'").Take(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&run).Update("state", "running").Error; err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: run.ID, OwnerID: f.owner, WorkflowID: f.workflow}
	var rateID string
	if err := f.db.Table("model_credit_rate_revisions").Select("id").Where("provider_type IS NULL").Scan(&rateID).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	admission := creditsdomain.Admission{UserID: f.owner, ExecutionID: run.ID, Source: "channel-credit-test", Timezone: "Asia/Shanghai", CreditDay: started.In(time.FixedZone("Shanghai", 8*3600)).Format(time.DateOnly), StagePosition: 1, StartedAt: started, Rate: creditsdomain.ModelCreditRate{RevisionID: rateID, Fallback: 100, InputMultiplierMicros: 1_000_000, OutputMultiplierMicros: 1_000_000}}
	if _, err := creditsrepo.New(f.db).Admit(ctx, admission); err != nil {
		t.Fatal(err)
	}
	result := application.ExecutionResult{FinalMessage: strings.Repeat("答复😀", 700), SuccessCommit: failingChannelCommit{}}
	result.CreditSettlements = []application.CreditSettlement{{UserID: f.owner, ExecutionID: run.ID, Source: admission.Source, Timezone: admission.Timezone, CreditDay: admission.CreditDay, RateRevisionID: rateID, StagePosition: 1, StartedAt: started, SettledAt: started, Amount: 50, Fallback: 100, InputMultiplierMicros: 1_000_000, OutputMultiplierMicros: 1_000_000}}
	if err := f.repo.FinishSucceeded(ctx, job, result); err == nil {
		t.Fatal("commit failure lost")
	}
	var count int64
	f.db.Model(&channelDeliveryRecord{}).Where("run_id=?", run.ID).Count(&count)
	if count != 0 {
		t.Fatal("Outbox survived rollback")
	}
	var consumed int64
	f.db.Table("credit_ledger").Where("source=?", admission.Source).Count(&consumed)
	if consumed != 0 {
		t.Fatal("Credit settlement survived terminal rollback")
	}

	f.db.Where("id=?", run.ID).Take(&run)
	if run.State != "running" {
		t.Fatal("terminal state survived rollback")
	}
	result.SuccessCommit = nil
	if err := f.repo.FinishSucceeded(ctx, job, result); err != nil {
		t.Fatal(err)
	}
	f.db.Table("credit_ledger").Where("source=?", admission.Source).Count(&consumed)
	if consumed != 1 {
		t.Fatal("terminal settlement missing or duplicated")
	}
	f.adapter.result = application.ChannelSendResult{State: "outcome_unknown", Code: "timeout"}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("send: %v %v", worked, err)
	}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || worked {
		t.Fatal("unknown was retried or next chunk escaped")
	}
	rows, err := f.app.Deliveries(ctx, f.owner, f.workflow, f.channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	var unknown domain.ChannelDelivery
	for _, d := range rows {
		if d.State == "outcome_unknown" {
			unknown = d
		}
	}
	if unknown.ID == "" {
		t.Fatal("unknown delivery missing")
	}
	if err = f.app.Retry(ctx, f.owner, f.workflow, f.channel.ID, unknown.ID, f.channel.Version, false); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("unknown retry lacked explicit confirmation")
	}
	if err = f.app.Retry(ctx, f.owner, f.workflow, f.channel.ID, unknown.ID, f.channel.Version, true); err != nil {
		t.Fatal(err)
	}
	f.adapter.result = application.ChannelSendResult{State: "sent", MessageID: "sent"}
	for i := 0; i < 5; i++ {
		f.readySend(t)
		worked, err := f.app.ProcessDelivery(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	f.db.Model(&channelDeliveryRecord{}).Where("run_id=? AND state<>'sent'", run.ID).Count(&count)
	if count != 0 {
		t.Fatal("retry did not finish all answer chunks")
	}
	var executions int64
	f.db.Model(&runRecord{}).Count(&executions)
	if executions != 1 {
		t.Fatal("resend executed a model")
	}
	f.receive(t, "queued", "alice", "cancel me")
	f.admit(t)
	old := f.stored(t)
	f.channel, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "disable")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.app.Receive(ctx, old, f.message("after-disable", "alice", "must not run")); err != nil {
		t.Fatal(err)
	}
	if worked, err := f.app.ProcessInbox(ctx); err != nil || worked {
		t.Fatal("disabled channel admitted input")
	}
	f.db.Model(&runRecord{}).Where("state='queued'").Count(&count)
	if count != 0 {
		t.Fatal("disable left queued channel Run")
	}
}
func TestChannelQueueAndCreditsRejectWithoutRun(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	for i := 0; i < 6; i++ {
		f.receive(t, uuid.NewString(), "alice", "question")
		f.admit(t)
	}
	var runs, rejected int64
	f.db.Model(&runRecord{}).Count(&runs)
	f.db.Model(&channelInboxRecord{}).Where("reason='queue_full'").Count(&rejected)
	if runs != 5 || rejected != 1 {
		t.Fatalf("queue admission: runs=%d rejection=%d", runs, rejected)
	}
	if err := f.db.Exec("UPDATE credit_accounts SET daily_remaining_hundredths=0,persistent_hundredths=0 WHERE user_id=?", f.owner).Error; err != nil {
		t.Fatal(err)
	}
	f.receive(t, "no-credits", "bob", "question")
	f.admit(t)
	f.db.Model(&channelInboxRecord{}).Where("reason='insufficient_credits'").Count(&rejected)
	if rejected != 1 {
		t.Fatal("nonpositive credit admitted")
	}
}

type channelExecutionRecorder struct{ instructions []string }

func (r *channelExecutionRecorder) Execute(ctx context.Context, job application.ExecutionJob, progress application.ProgressRecorder) (application.ExecutionResult, error) {
	r.instructions = append(r.instructions, job.Instruction)
	return application.ExecutionResult{FinalMessage: "saved answer"}, nil
}
func TestChannelUsesExistingWorkerAndPriorAnswers(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	executor := &channelExecutionRecorder{}
	worker, err := application.NewWorker(f.repo, executor, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.receive(t, "one", "alice", "first question")
	f.admit(t)
	if worked, err := worker.ProcessNext(ctx); err != nil || !worked {
		t.Fatalf("Worker: %v %v", worked, err)
	}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("reply: %v %v", worked, err)
	}
	f.receive(t, "two", "alice", "follow-up question")
	f.admit(t)
	if worked, err := worker.ProcessNext(ctx); err != nil || !worked {
		t.Fatalf("follow-up: %v %v", worked, err)
	}
	f.receive(t, "three", "bob", "other question")
	f.admit(t)
	if worked, err := worker.ProcessNext(ctx); err != nil || !worked {
		t.Fatalf("separate: %v %v", worked, err)
	}
	if len(executor.instructions) != 3 || !strings.Contains(executor.instructions[1], "saved answer") || strings.Contains(executor.instructions[2], "first question") {
		t.Fatalf("context isolation: %#v", executor.instructions)
	}
	var rows []runRecord
	f.db.Find(&rows)
	for _, run := range rows {
		if run.MessageChannelID == nil || *run.MessageChannelID != f.channel.ID || run.MessageChannelName != "Test bot" || run.MessageChannelProvider != "telegram" || runDomain(run).MessageChannelProvider != "telegram" || run.State != "succeeded" {
			t.Fatal("source or terminal missing")
		}
	}
	history, err := f.repo.ListRuns(ctx, f.owner, f.workflow)
	if err != nil || len(history) != 2 {
		t.Fatalf("one history entry per participant: count=%d error=%v", len(history), err)
	}
	var totalTurns int
	for _, summary := range history {
		if summary.ID != summary.ConversationID {
			t.Fatal("history entry is not the stable conversation root")
		}
		turns, err := f.repo.ListRunTurns(ctx, f.owner, f.workflow, summary.ID)
		if err != nil || len(turns) != int(summary.TurnNumber) {
			t.Fatalf("conversation lost turns: count=%d latest=%d error=%v", len(turns), summary.TurnNumber, err)
		}
		totalTurns += len(turns)
	}
	if totalTurns != 3 {
		t.Fatalf("grouped history lost executions: %d", totalTurns)
	}
	f.stored(t)
	f.channel, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "disable")
	if err != nil {
		t.Fatal(err)
	}
	renamed := f.channel
	renamed.Name = "Renamed bot"
	f.channel, err = f.app.Save(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, renamed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "delete"); err != nil {
		t.Fatal(err)
	}
	history, err = f.repo.ListRuns(ctx, f.owner, f.workflow)
	if err != nil || len(history) != 2 {
		t.Fatalf("history after deletion: count=%d error=%v", len(history), err)
	}
	for _, run := range history {
		if run.MessageChannelName != "Test bot" || run.MessageChannelProvider != "telegram" || run.Trigger != "message_channel" {
			t.Fatalf("channel edit rewrote Run origin: %#v", run)
		}
	}
}
func TestChannelAccountOffboardingNeverReactivatesOnEnable(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	f.receive(t, "queued", "alice", "question")
	f.admit(t)
	accounts := accountrepo.New(f.db)
	if _, err := accounts.SetEnabled(ctx, f.owner, f.owner, false, 1, "integration test"); err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.SetEnabled(ctx, f.owner, f.owner, true, 2, "integration test"); err != nil {
		t.Fatal(err)
	}
	stored := f.stored(t)
	if stored.Channel.Enabled || stored.Channel.ValidationState != "unverified" {
		t.Fatal("account re-enable resurrected a channel")
	}
	if _, err := f.repo.ClaimNext(ctx); err != nil {
		t.Fatal(err)
	}
	var queued int64
	f.db.Model(&runRecord{}).Where("state='queued'").Count(&queued)
	if queued != 0 {
		t.Fatal("offboarded queue resurrected")
	}
	if _, err := f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, stored.Channel.Version, "enable"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("offboarded validation reused")
	}
}
func TestChannelMaintenanceScrubsReplyCapabilitiesKeepsDedup(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	f.receive(t, "old", "alice", "private question")
	f.admit(t)
	if err := f.db.Exec(`UPDATE message_channel_inbox SET received_at=now()-interval '31 days'`).Error; err != nil {
		t.Fatal(err)
	}
	if worked, err := f.app.ProcessMaintenance(ctx); err != nil || !worked {
		t.Fatalf("maintenance: %v %v", worked, err)
	}
	var inbox channelInboxRecord
	f.db.Where("event_id='old'").Take(&inbox)
	if len(inbox.ReplyCiphertext) != 0 || strings.Contains(string(inbox.Message), "private question") {
		t.Fatal("private body or capability retained")
	}
	if inbox.EventID != "old" || inbox.State != "admitted" {
		t.Fatal("dedup tombstone removed")
	}
}

func TestChannelBindingResetRotationAndExpiredLease(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	if _, err := f.app.Save(ctx, f.owner, f.workflow, "", 0, domain.MessageChannel{Provider: "telegram", Name: "Duplicate", Audience: f.channel.Audience}, application.ChannelCredentials{"bot_token": "other-token"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate binding accepted: %v", err)
	}
	f.receive(t, "before-reset", "alice", "first")
	f.admit(t)
	before := f.channel
	next, err := f.app.Control(ctx, f.owner, f.workflow, before.ID, before.Version, "reset")
	if err != nil {
		t.Fatal(err)
	}
	f.channel = next
	f.receive(t, "after-reset", "alice", "second")
	f.admit(t)
	var runs []runRecord
	f.db.Order("queued_at,id").Find(&runs)
	if len(runs) != 2 || runs[0].ConversationID == runs[1].ConversationID {
		t.Fatal("reset retained previous conversation")
	}
	// A persisted sending lease is uncertain after restart, never blindly resent.
	if err := f.db.Model(&runs[0]).Update("state", "running").Error; err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: runs[0].ID, OwnerID: f.owner, WorkflowID: f.workflow}
	if err := f.repo.FinishSucceeded(ctx, job, application.ExecutionResult{FinalMessage: "answer protected-test-token"}); err != nil {
		t.Fatal(err)
	}
	delivery, err := f.repo.ClaimChannelDelivery(ctx)
	if err != nil || delivery == nil {
		t.Fatalf("claim: %v %v", delivery, err)
	}
	if strings.Contains(delivery.Text, "protected-test-token") {
		t.Fatal("outbox persisted plaintext credential")
	}
	f.db.Model(&channelDeliveryRecord{}).Where("id=?", delivery.Delivery.ID).Update("lease_until", time.Now().Add(-time.Second))
	f.readySend(t)
	if next, err := f.repo.ClaimChannelDelivery(ctx); err != nil || next != nil {
		t.Fatalf("expired sending retried: %v %v", next, err)
	}
	if err := f.repo.FinishChannelDelivery(ctx, delivery, application.ChannelSendResult{State: "sent"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale sender completed lease")
	}
	disabled, err := f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "disable")
	if err != nil {
		t.Fatal(err)
	}
	f.channel = disabled
	stale := f.stored(t)
	updated, err := f.app.Save(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, domain.MessageChannel{Provider: "telegram", Name: "Rotated", Audience: f.channel.Audience}, application.ChannelCredentials{"bot_token": "replacement-token"})
	if err != nil {
		t.Fatal(err)
	}
	f.channel = updated
	if updated.Enabled || updated.ValidationState != "unverified" || updated.ConfigVersion <= stale.Channel.ConfigVersion {
		t.Fatal("rotation reused validation")
	}
	if err := f.app.Receive(ctx, stale, f.message("stale-event", "alice", "must not execute")); err != nil {
		t.Fatal(err)
	}
	var count int64
	f.db.Model(&channelInboxRecord{}).Where("event_id='stale-event'").Count(&count)
	if count != 0 {
		t.Fatal("old credentials accepted a new event")
	}
}

func TestChannelSendCooldownAndBacklogBudget(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	f.repo.db = f.repo.db.Set(channelLimitsKey, application.ChannelLimits{MaxPendingMessages: 1}.Effective())
	f.receive(t, "first", "alice", "question")
	if err := f.app.Receive(ctx, f.stored(t), f.message("second", "bob", "question")); err == nil {
		t.Fatal("backlog budget exceeded")
	}
	f.admit(t)
	var run runRecord
	f.db.Where("trigger='message_channel'").Take(&run)
	f.db.Model(&run).Update("state", "running")
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: run.ID, OwnerID: f.owner, WorkflowID: f.workflow}
	if err := f.repo.FinishSucceeded(ctx, job, application.ExecutionResult{FinalMessage: strings.Repeat("long", 600)}); err != nil {
		t.Fatal(err)
	}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("first chunk: %v %v", worked, err)
	}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || worked {
		t.Fatalf("send cooldown ignored: %v %v", worked, err)
	}
	f.readySend(t)
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("next chunk: %v %v", worked, err)
	}
	f.readySend(t)
	last, err := f.repo.ClaimChannelDelivery(ctx)
	if err != nil || last == nil {
		t.Fatalf("last chunk: %v %v", last, err)
	}
	if err := f.repo.FinishChannelDelivery(ctx, last, application.ChannelSendResult{State: "retry_wait", RetryAfter: 2 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	var pending channelDeliveryRecord
	f.db.Where("id=?", last.Delivery.ID).Take(&pending)
	now, err := channelDatabaseTime(f.db)
	if err != nil {
		t.Fatal(err)
	}
	if pending.RetryAt.Sub(now) < 119*time.Minute {
		t.Fatal("Retry-After shortened")
	}
}

func TestChannelTerminalOutboxDoesNotDeadlockWithControlLock(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	f.receive(t, "question", "alice", "question")
	f.admit(t)
	var run runRecord
	f.db.Where("trigger='message_channel'").Take(&run)
	f.db.Model(&run).Update("state", "running")
	tx := f.db.WithContext(ctx).Begin()
	defer tx.Rollback()
	if err := tx.Exec("SELECT id FROM workflow_message_channels WHERE id=? FOR NO KEY UPDATE", f.channel.ID).Error; err != nil {
		t.Fatal(err)
	}
	// Control may wait for the Run lock. Terminal insertion must still acquire
	// FK key-share locks on the channel, avoiding the reverse lock cycle.
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: run.ID, OwnerID: f.owner, WorkflowID: f.workflow}
	if err := f.repo.FinishSucceeded(ctx, job, application.ExecutionResult{FinalMessage: "answer"}); err != nil {
		t.Fatal(err)
	}
	var count int64
	f.db.Model(&channelDeliveryRecord{}).Where("run_id=?", run.ID).Count(&count)
	if count != 1 {
		t.Fatal("terminal outbox missing")
	}
}

func TestChannelFreshValidationRecoversUnknownFixedReply(t *testing.T) {
	f := newChannelFixture(t)
	ctx := context.Background()
	current, err := f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "validate")
	if err != nil {
		t.Fatal(err)
	}
	f.channel = current
	f.receive(t, "first-validation", "alice", current.ValidationCode)
	f.adapter.result = application.ChannelSendResult{State: "outcome_unknown", Code: "timeout"}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("first validation: %v %v", worked, err)
	}
	current, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "validate")
	if err != nil {
		t.Fatal(err)
	}
	f.channel = current
	f.receive(t, "new-validation", "alice", current.ValidationCode)
	f.readySend(t)
	f.adapter.result = application.ChannelSendResult{State: "sent", MessageID: "verified"}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("replacement validation blocked: %v %v", worked, err)
	}
	f.stored(t)
	if f.channel.ValidationState != "passed" {
		t.Fatal("new validation did not pass")
	}
	var runs int64
	f.db.Model(&runRecord{}).Count(&runs)
	if runs != 0 {
		t.Fatal("validation created a model Run")
	}
}

func TestChannelReceiveCursorFencingRotationAndDeletion(t *testing.T) {
	for _, workflowDelete := range []bool{false, true} {
		t.Run(map[bool]string{false: "channel", true: "workflow"}[workflowDelete], func(t *testing.T) {
			f := newChannelFixture(t)
			f.enable(t)
			ctx := context.Background()
			stored := f.stored(t)
			if err := f.repo.StoreChannelReceiveCursor(ctx, stored, []byte("ciphertext")); err != nil {
				t.Fatal(err)
			}
			data, err := f.repo.LoadChannelReceiveCursor(ctx, stored)
			if err != nil || string(data) != "ciphertext" {
				t.Fatal("lost cursor")
			}
			stale := stored
			stale.Channel.Version--
			if err := f.repo.StoreChannelReceiveCursor(ctx, stale, []byte("stale")); !errors.Is(err, domain.ErrConflict) {
				t.Fatal("stale writer accepted")
			}
			f.channel, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "disable")
			if err != nil {
				t.Fatal(err)
			}
			if err := f.repo.StoreChannelReceiveCursor(ctx, stored, []byte("disabled")); !errors.Is(err, domain.ErrConflict) {
				t.Fatal("disabled writer accepted")
			}
			f.channel, err = f.app.Save(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, f.channel, application.ChannelCredentials{"bot_token": "rotated-secret"})
			if err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := f.db.Model(&channelReceiveCursorRecord{}).Where("channel_id=?", f.channel.ID).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("rotation retained cursor")
			}
			f.enable(t)
			stored = f.stored(t)
			if err := f.repo.StoreChannelReceiveCursor(ctx, stored, []byte("new ciphertext")); err != nil {
				t.Fatal(err)
			}
			if workflowDelete {
				err = f.repo.DeleteWorkflow(ctx, f.owner, f.workflow)
			} else {
				_, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "delete")
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := f.db.Model(&channelReceiveCursorRecord{}).Where("channel_id=?", f.channel.ID).Count(&count).Error; err != nil || count != 0 {
				t.Fatal("deleted cursor retained")
			}
		})
	}
}
func TestChannelMigrationAllowsAllThirteenProviders(t *testing.T) {
	f := newChannelFixture(t)
	for _, provider := range []string{"telegram", "discord", "slack", "dingtalk", "feishu", "matrix", "whatsapp", "signal", "wecom", "wechat", "qqbot", "bluebubbles", "yuanbao"} {
		if err := f.db.Model(&channelRecord{}).Where("id=?", f.channel.ID).Update("provider", provider).Error; err != nil {
			t.Fatalf("provider %s: %v", provider, err)
		}
	}
	if err := f.db.Model(&channelRecord{}).Where("id=?", f.channel.ID).Update("provider", "unknown").Error; err == nil {
		t.Fatal("unknown provider persisted")
	}
}

func TestChannelWorkflowCanConfigureEntireRequestedSetWithinBound(t *testing.T) {
	f := newChannelFixture(t)
	ctx := context.Background()
	providers := []string{"discord", "slack", "dingtalk", "feishu", "matrix", "whatsapp", "signal", "wecom", "wechat", "qqbot", "bluebubbles", "yuanbao", "telegram", "telegram", "telegram"}
	for i, provider := range providers {
		c := f.channel
		c.ID = uuid.NewString()
		c.Provider = provider
		c.BindingID = c.ID
		c.AccountID = c.ID
		c.Name = provider
		c.Version = 1
		c.ConfigVersion = 1
		if _, err := f.repo.SaveMessageChannel(ctx, application.ChannelStored{Channel: c, Ciphertext: []byte("test-ciphertext")}, 0); err != nil {
			t.Fatalf("configuration %d (%s): %v", i+2, provider, err)
		}
	}
	c := f.channel
	c.ID = uuid.NewString()
	c.BindingID = c.ID
	c.AccountID = c.ID
	if _, err := f.repo.SaveMessageChannel(ctx, application.ChannelStored{Channel: c, Ciphertext: []byte("test-ciphertext")}, 0); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("unbounded channel configurations")
	}
}
