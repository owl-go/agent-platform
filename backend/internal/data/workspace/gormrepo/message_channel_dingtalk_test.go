package gormrepo

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
)

func TestDingTalkPinsEnterpriseOnlyDuringAuthorizedValidation(t *testing.T) {
	f := newChannelFixture(t)
	ctx := context.Background()
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	f.app = application.NewMessageChannels(f.repo, box, map[string]application.ChannelTransport{"dingtalk": {Account: f.adapter, WebhookReceiver: f.adapter, Sender: f.adapter}}, true, "https://workspace.example.test")
	f.channel, err = f.app.Save(ctx, f.owner, f.workflow, "", 0, domain.MessageChannel{Provider: "dingtalk", Name: "DingTalk", Audience: domain.ChannelAudience{SenderIDs: []string{"alice"}, AllowDirect: true}}, application.ChannelCredentials{"client_id": "app", "client_secret": "protected"})
	if err != nil {
		t.Fatal(err)
	}
	f.channel, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "validate")
	if err != nil {
		t.Fatal(err)
	}
	stale := f.stored(t)
	for _, tc := range []struct{ id, sender, text, tenant string }{
		{"ordinary", "alice", "question", "corp"},
		{"outsider", "mallory", f.channel.ValidationCode, "corp"},
		{"missing-tenant", "alice", f.channel.ValidationCode, ""},
	} {
		m := f.message(tc.id, tc.sender, tc.text)
		m.TenantID = tc.tenant
		if err := f.app.Receive(ctx, stale, m); err != nil {
			t.Fatal(err)
		}
		if f.stored(t).Channel.TenantID != "" {
			t.Fatalf("%s pinned tenant", tc.id)
		}
	}
	m := f.message("valid", "alice", f.channel.ValidationCode)
	m.TenantID = "corp"
	// Force the transaction to fail after Inbox and identity updates; neither may persist.
	if err := f.db.Exec(`CREATE FUNCTION reject_dingtalk_validation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'recording failure'; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`CREATE TRIGGER reject_dingtalk_validation BEFORE INSERT ON message_channel_deliveries FOR EACH ROW EXECUTE FUNCTION reject_dingtalk_validation()`).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.app.Receive(ctx, stale, m); err == nil {
		t.Fatal("expected failed outbox transaction")
	}
	if f.stored(t).Channel.TenantID != "" {
		t.Fatal("rolled-back transaction pinned tenant")
	}
	if err := f.db.Exec(`DROP TRIGGER reject_dingtalk_validation ON message_channel_deliveries`).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.app.Receive(ctx, stale, m); err != nil {
		t.Fatal(err)
	}
	if f.stored(t).Channel.TenantID != "corp" {
		t.Fatal("validation did not pin tenant")
	}
	// The still-running connection has its old empty-tenant snapshot. Database
	// isolation must reject a different enterprise even before its context closes.
	m.EventID = "other-corp"
	m.MessageID = "other-corp"
	m.TenantID = "other"
	if err := f.app.Receive(ctx, stale, m); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := f.db.Model(&channelInboxRecord{}).Where("channel_id=?", f.channel.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("unexpected inbox count %d", count)
	}
	if worked, err := f.app.ProcessDelivery(ctx); err != nil || !worked {
		t.Fatalf("validation reply: %v %v", worked, err)
	}
	f.stored(t)
	if f.channel.ValidationState != "passed" {
		t.Fatal("validation not completed")
	}
	f.channel, err = f.app.Control(ctx, f.owner, f.workflow, f.channel.ID, f.channel.Version, "enable")
	if err != nil {
		t.Fatal(err)
	}
	stored := f.stored(t)
	m = f.message("question", "alice", "hello")
	m.TenantID = "other"
	if err := f.app.Receive(ctx, stored, m); err != nil {
		t.Fatal(err)
	}
	m.TenantID = "corp"
	if err := f.app.Receive(ctx, stored, m); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&channelInboxRecord{}).Where("channel_id=? AND state='received'", f.channel.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("enterprise isolation count %d", count)
	}
	// Unauthenticated identity values still pass through the normal ID validator.
	m.TenantID = "corp\nforged"
	if m.Validate(time.Now()) == nil {
		t.Fatal("invalid tenant accepted")
	}
}
