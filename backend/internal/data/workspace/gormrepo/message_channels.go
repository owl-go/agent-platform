package gormrepo

import (
	"agent-platform/backend/internal/credentials"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type channelRecord struct {
	BindingID                                                  string
	ID, OwnerUserID, WorkflowID, Provider, AccountID, TenantID string
	Configuration                                              []byte `gorm:"type:jsonb"`
	CredentialCiphertext                                       []byte
	ConfigVersion, Version, Generation                         int64
	Enabled                                                    bool
	ValidationState, ValidationCode                            string
	ValidationUntil                                            *time.Time
	Health, ErrorCode                                          string
	DeletedAt                                                  *time.Time
	CreatedAt                                                  time.Time
	SendAfter                                                  time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (channelRecord) TableName() string { return "workflow_message_channels" }
func (r channelRecord) stored() (application.ChannelStored, error) {
	var c domain.MessageChannel
	if err := json.Unmarshal(r.Configuration, &c); err != nil {
		return application.ChannelStored{}, err
	}
	c.ID, c.OwnerID, c.WorkflowID = r.ID, r.OwnerUserID, r.WorkflowID
	c.BindingID = r.BindingID
	c.Version, c.ConfigVersion = r.Version, r.ConfigVersion
	c.Enabled, c.ValidationState, c.ValidationCode, c.ValidationUntil = r.Enabled, r.ValidationState, r.ValidationCode, r.ValidationUntil
	c.Health, c.ErrorCode = r.Health, r.ErrorCode
	if c.ValidationUntil != nil && time.Now().After(*c.ValidationUntil) {
		c.ValidationCode = ""
		if c.ValidationState == "testing" {
			c.ValidationState = "unverified"
		}
	}
	return application.ChannelStored{Channel: c, Ciphertext: r.CredentialCiphertext}, nil
}

type channelInboxRecord struct {
	ID, ChannelID                        string
	ConfigVersion, Generation            int64
	EventID, MessageID, ChatID, SenderID string
	Message                              []byte `gorm:"type:jsonb"`
	ReplyCiphertext                      []byte
	State, Reason                        string
	RunID                                *string
	ReceivedAt                           time.Time
}

func (channelInboxRecord) TableName() string { return "message_channel_inbox" }

type channelDeliveryRecord struct {
	ID, ChannelID, InboxID       string
	RunID                        *string
	Kind                         string
	Chunk                        int
	Payload                      string
	ConfigVersion                int64
	State                        string
	Attempts                     int
	ErrorCode, ProviderMessageID string
	Lease                        *string
	LeaseUntil                   *time.Time
	RetryAt, Deadline, CreatedAt time.Time
}

func (channelDeliveryRecord) TableName() string { return "message_channel_deliveries" }
func (r channelDeliveryRecord) projection() domain.ChannelDelivery {
	d := domain.ChannelDelivery{ID: r.ID, ChannelID: r.ChannelID, Kind: r.Kind, Chunk: r.Chunk, State: r.State, Attempts: r.Attempts, ErrorCode: r.ErrorCode, ProviderMessageID: r.ProviderMessageID, CreatedAt: r.CreatedAt}
	if r.RunID != nil {
		d.RunID = *r.RunID
	}
	return d
}

var _ application.MessageChannelRepository = (*Repository)(nil)

func (r *Repository) ListMessageChannels(ctx context.Context, owner, workflow string) ([]domain.MessageChannel, error) {
	if _, err := r.GetWorkflow(ctx, owner, workflow, false); err != nil {
		return nil, err
	}
	var rows []channelRecord
	err := r.db.WithContext(ctx).Where("owner_user_id=? AND workflow_id=? AND deleted_at IS NULL", owner, workflow).Order("created_at,id").Find(&rows).Error
	items := make([]domain.MessageChannel, 0, len(rows))
	for _, row := range rows {
		stored, e := row.stored()
		if e != nil {
			return nil, e
		}
		items = append(items, stored.Channel)
	}
	return items, err
}
func (r *Repository) GetMessageChannel(ctx context.Context, owner, workflow, id string) (application.ChannelStored, error) {
	if _, err := uuid.Parse(id); err != nil {
		return application.ChannelStored{}, domain.ErrNotFound
	}
	query := r.db.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id)
	if owner != "" {
		query = query.Where("owner_user_id=? AND workflow_id=?", owner, workflow)
	}
	var row channelRecord
	if err := query.Take(&row).Error; err != nil {
		return application.ChannelStored{}, mapNotFound(err)
	}
	return row.stored()
}
func lockChannelWorkflow(tx *gorm.DB, owner, workflow string) error {
	var w workflowRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_user_id=? AND deleted_at IS NULL AND EXISTS (SELECT 1 FROM users WHERE users.id=workflows.owner_user_id AND disabled_at IS NULL)", workflow, owner).Take(&w).Error
	return mapNotFound(err)
}
func (r *Repository) SaveMessageChannel(ctx context.Context, stored application.ChannelStored, version int64) (application.ChannelStored, error) {
	c := stored.Channel
	data, err := json.Marshal(c)
	if err != nil {
		return stored, err
	}
	row := channelRecord{ID: c.ID, OwnerUserID: c.OwnerID, WorkflowID: c.WorkflowID, Provider: c.Provider, AccountID: c.AccountID, BindingID: c.BindingID, TenantID: c.TenantID, Configuration: data, CredentialCiphertext: stored.Ciphertext, ConfigVersion: c.ConfigVersion, Version: c.Version, Generation: 1, ValidationState: "unverified", Health: "disconnected", CreatedAt: time.Now().UTC()}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockChannelWorkflow(tx, c.OwnerID, c.WorkflowID); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&channelRecord{}).Where("workflow_id=? AND deleted_at IS NULL", c.WorkflowID).Count(&count).Error; err != nil {
			return err
		}
		if version == 0 && count >= 16 {
			return domain.ErrInvalid
		}
		if version == 0 {
			return tx.Create(&row).Error
		}
		var old channelRecord
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=? AND owner_user_id=? AND workflow_id=? AND deleted_at IS NULL", c.ID, c.OwnerID, c.WorkflowID).Take(&old).Error; err != nil {
			return mapNotFound(err)
		}
		if old.Version != version || old.Enabled || old.Provider != c.Provider {
			return domain.ErrConflict
		}
		if err := stopChannelTasks(tx, old.ID); err != nil {
			return err
		}
		if err := tx.Where("channel_id=?", old.ID).Delete(&channelReceiveCursorRecord{}).Error; err != nil {
			return err
		}
		row.Generation = old.Generation + 1
		return tx.Model(&old).Select("configuration", "credential_ciphertext", "account_id", "binding_id", "tenant_id", "config_version", "version", "generation", "enabled", "validation_state", "validation_code", "validation_until", "health", "error_code").Updates(row).Error
	})
	if err != nil {
		if strings.Contains(err.Error(), "message_channel_account_binding") {
			return stored, fmt.Errorf("%w: bot already has a channel binding", domain.ErrConflict)
		}
		return stored, err
	}
	return row.stored()
}
func (r *Repository) ControlMessageChannel(ctx context.Context, owner, workflow, id string, version int64, action, code string) (application.ChannelStored, error) {
	var row channelRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockChannelWorkflow(tx, owner, workflow); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=? AND owner_user_id=? AND workflow_id=? AND deleted_at IS NULL", id, owner, workflow).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Version != version {
			return domain.ErrConflict
		}
		now := time.Now().UTC()
		updates := map[string]any{"version": version + 1}
		switch action {
		case "validate":
			if row.Enabled {
				return domain.ErrConflict
			}
			if err := tx.Model(&channelDeliveryRecord{}).Where("channel_id=? AND kind='validation' AND state IN ('pending','retry_wait','outcome_unknown')", id).Update("state", "cancelled").Error; err != nil {
				return err
			}
			updates["validation_state"], updates["validation_code"], updates["validation_until"] = "testing", code, now.Add(10*time.Minute)
		case "enable":
			if row.ValidationState != "passed" {
				return domain.ErrInvalid
			}
			var w workflowRecord
			if err := tx.Where("id=?", workflow).Take(&w).Error; err != nil {
				return err
			}
			snapshot, err := loadExecutionSnapshot(tx, w)
			if err != nil {
				return err
			}
			if err := validateQueuedSnapshotAvailability(tx, snapshot, owner); err != nil {
				return err
			}
			updates["enabled"] = true
		case "disable", "delete":
			updates["enabled"] = false
			updates["health"] = "disconnected"
			updates["validation_code"] = ""
			if action == "delete" {
				updates["deleted_at"] = now
				updates["credential_ciphertext"] = nil
				if err := tx.Where("channel_id=?", id).Delete(&channelReceiveCursorRecord{}).Error; err != nil {
					return err
				}
				if err := tx.Model(&channelInboxRecord{}).Where("channel_id=?", id).Update("reply_ciphertext", []byte{}).Error; err != nil {
					return err
				}
				updates["validation_state"] = "unverified"
			}
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return err
			}
			if err := stopChannelTasks(tx, id); err != nil {
				return err
			}
			return tx.Where("id=?", id).Take(&row).Error
		case "reset":
			updates["generation"] = row.Generation + 1
		default:
			return domain.ErrInvalid
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id=?", id).Take(&row).Error
	})
	if err != nil {
		return application.ChannelStored{}, err
	}
	return row.stored()
}

// Workflow/channel locks precede Inbox/Run/Delivery locks. Terminal execution
// writes an Outbox without acquiring these locks in the reverse direction.
// Channel NO KEY UPDATE locks permit the Outbox FK key-share checks while a
// control operation waits for a terminal Run; an UPDATE lock would deadlock.
func stopChannelTasks(tx *gorm.DB, id string) error {
	now := time.Now().UTC()
	if err := tx.Model(&channelInboxRecord{}).Where("channel_id=? AND state='received'", id).Updates(map[string]any{"state": "ignored", "reason": "channel_disabled"}).Error; err != nil {
		return err
	}
	if err := tx.Model(&channelDeliveryRecord{}).Where("channel_id=? AND state IN ('pending','retry_wait')", id).Update("state", "cancelled").Error; err != nil {
		return err
	}
	var runs []runRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN (SELECT run_id FROM message_channel_inbox WHERE channel_id=?) AND state IN ('queued','waiting_for_user','running')", id).Find(&runs).Error; err != nil {
		return err
	}
	for _, run := range runs {
		updates := map[string]any{"cancel_requested_at": now, "version": gorm.Expr("version+1")}
		if run.State != "running" {
			updates["state"], updates["ended_at"] = "cancelled", now
		}
		if err := tx.Model(&run).Updates(updates).Error; err != nil {
			return err
		}
		if run.State != "running" {
			if err := appendRunEvents(tx, run.ID, nil, "run.cancelled", now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Repository) ReceiveChannelMessage(ctx context.Context, stored application.ChannelStored, m domain.ChannelMessage, reply []byte) (bool, error) {
	accepted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockChannelWorkflow(tx, stored.Channel.OwnerID, stored.Channel.WorkflowID); errors.Is(err, domain.ErrNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		var row channelRecord
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=? AND deleted_at IS NULL", stored.Channel.ID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		current, err := row.stored()
		if err != nil {
			return err
		}
		if row.ConfigVersion != stored.Channel.ConfigVersion || !current.Channel.Audience.Allows(m) || m.Validate(time.Now()) != nil {
			return nil
		}
		validation := !row.Enabled && row.ValidationState == "testing" && row.ValidationUntil != nil && time.Now().Before(*row.ValidationUntil) && strings.TrimSpace(m.Text) == row.ValidationCode
		if !row.Enabled && !validation {
			return nil
		}
		var duplicate int64
		if err := tx.Model(&channelInboxRecord{}).Where("channel_id=? AND (event_id=? OR (chat_id=? AND message_id=?))", row.ID, m.EventID, m.ChatID, m.MessageID).Count(&duplicate).Error; err != nil {
			return err
		}
		if duplicate > 0 {
			return nil
		}
		var pending, rate int64
		if err := tx.Model(&channelInboxRecord{}).Where("channel_id=? AND state='received'", row.ID).Count(&pending).Error; err != nil {
			return err
		}
		var outgoing int64
		if err := tx.Model(&channelDeliveryRecord{}).Where("channel_id=? AND state IN ('pending','sending','retry_wait','outcome_unknown')", row.ID).Count(&outgoing).Error; err != nil {
			return err
		}
		if pending+outgoing >= int64(channelLimits(tx).MaxPendingMessages) {
			return fmt.Errorf("channel_backlog_full")
		}
		if err := tx.Model(&channelInboxRecord{}).Where("channel_id=? AND sender_id=? AND received_at>?", row.ID, m.SenderID, time.Now().Add(-time.Minute)).Count(&rate).Error; err != nil {
			return err
		}
		if rate >= int64(channelLimits(tx).MaxSenderMessagesPerMinute) {
			return nil
		}
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		inbox := channelInboxRecord{ID: uuid.NewString(), ChannelID: row.ID, ConfigVersion: row.ConfigVersion, Generation: row.Generation, EventID: m.EventID, MessageID: m.MessageID, ChatID: m.ChatID, SenderID: m.SenderID, Message: b, ReplyCiphertext: reply, State: "received", ReceivedAt: time.Now().UTC()}
		if validation {
			inbox.State = "ignored"
			inbox.Reason = "validation"
		}
		if err := tx.Create(&inbox).Error; err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{"health": "connected", "error_code": ""}).Error; err != nil {
			return err
		}
		accepted = true
		if validation {
			return enqueueChannelDelivery(tx, row, inbox, "validation", "接入验证成功 / Channel connection verified.")
		}
		return nil
	})
	return accepted, err
}

type channelCreditBalance interface {
	BalanceTx(*gorm.DB, string, string, time.Time) (creditsdomain.Balance, error)
}

func (r *Repository) AdmitChannelMessage(ctx context.Context) (bool, error) {
	if err := r.acquireWorkerClaimLock(ctx); err != nil {
		return false, err
	}
	var candidate channelInboxRecord
	if err := r.db.WithContext(ctx).Where("state='received'").Order("received_at,id").Take(&candidate).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	processed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var channel channelRecord
		if err := tx.Where("id=?", candidate.ChannelID).Take(&channel).Error; err != nil {
			return err
		}
		var w workflowRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("id=?", channel.WorkflowID).Take(&w).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=?", channel.ID).Take(&channel).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND state='received'", candidate.ID).Take(&candidate).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		processed = true
		stored, err := channel.stored()
		if err != nil {
			return err
		}
		var m domain.ChannelMessage
		if err = json.Unmarshal(candidate.Message, &m); err != nil {
			return err
		}
		var enabled int64
		if err := tx.Table("users").Where("id=? AND disabled_at IS NULL", channel.OwnerUserID).Count(&enabled).Error; err != nil {
			return err
		}
		if !channel.Enabled || channel.DeletedAt != nil || w.DeletedAt != nil || enabled == 0 || channel.ConfigVersion != candidate.ConfigVersion || !stored.Channel.Audience.Allows(m) {
			return tx.Model(&candidate).Updates(map[string]any{"state": "ignored", "reason": "authorization_revoked"}).Error
		}
		reject := func(reason string) error {
			if err := tx.Model(&candidate).Updates(map[string]any{"state": "rejected", "reason": reason}).Error; err != nil {
				return err
			}
			return enqueueChannelDelivery(tx, channel, candidate, "status", "暂时无法执行，请稍后发送新问题 / Unable to execute; please send a new question later.")
		}
		balanceRepo, ok := r.credits.(channelCreditBalance)
		if !ok {
			return reject("credits_unavailable")
		}
		var settings settingsRecord
		if err := tx.Where("user_id=?", channel.OwnerUserID).Take(&settings).Error; err != nil {
			return reject("dependencies_unavailable")
		}
		balance, err := balanceRepo.BalanceTx(tx, channel.OwnerUserID, settings.Timezone, time.Now().UTC())
		if err != nil {
			return err
		}
		if balance.Available <= 0 {
			return reject("insufficient_credits")
		}
		if err := ensureWorkflowQueueCapacity(tx, w.ID); err != nil {
			if errors.Is(err, domain.ErrQueueFull) {
				return reject("queue_full")
			}
			return err
		}
		key := m.ConversationKey(stored.Channel)
		var mapping struct{ RunID string }
		if err := tx.Table("message_channel_conversations").Where("channel_id=? AND generation=? AND conversation_key=?", channel.ID, candidate.Generation, key).Scan(&mapping).Error; err != nil {
			return err
		}
		if err := tx.SavePoint("channel_admission").Error; err != nil {
			return err
		}
		var runID string
		if mapping.RunID == "" {
			created, e := createRunOnTx(tx, channel.OwnerUserID, w.ID, "message_channel", &m.Text, nil, "", false)
			err = e
			runID = created.ID
			if err == nil {
				err = tx.Table("message_channel_conversations").Create(map[string]any{"channel_id": channel.ID, "generation": candidate.Generation, "conversation_key": key, "run_id": runID}).Error
			}
		} else {
			nested := &Repository{db: tx, credits: r.credits}
			created, e := nested.continueRunConversationTriggered(ctx, channel.OwnerUserID, w.ID, mapping.RunID, m.Text, nil, "", "", false, "message_channel")
			err = e
			runID = created.ID
		}
		if err == nil {
			var frozen runRecord
			if readErr := tx.Where("id=?", runID).Take(&frozen).Error; readErr != nil {
				return readErr
			}
			var snapshot domain.ExecutionSnapshot
			if json.Unmarshal(frozen.WorkflowSnapshot, &snapshot) != nil || validateQueuedSnapshotAvailability(tx, snapshot, channel.OwnerUserID) != nil {
				err = fmt.Errorf("%w: channel_snapshot_unavailable", domain.ErrInvalid)
			}
		}

		if err != nil {
			if rollbackErr := tx.RollbackTo("channel_admission").Error; rollbackErr != nil {
				return rollbackErr
			}
			if errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrConflict) {
				return reject("dependencies_unavailable")
			}
			return err
		}
		if err := tx.Model(&runRecord{}).Where("id=?", runID).Updates(map[string]any{"message_channel_id": channel.ID, "message_channel_name": stored.Channel.Name, "message_channel_provider": stored.Channel.Provider}).Error; err != nil {
			return err
		}

		return tx.Model(&candidate).Updates(map[string]any{"state": "admitted", "run_id": runID}).Error
	})
	return processed, err
}

func enqueueChannelDelivery(tx *gorm.DB, c channelRecord, inbox channelInboxRecord, kind, text string) error {
	now, err := channelDatabaseTime(tx)
	if err != nil {
		return err
	}
	for index, chunk := range domain.SplitChannelText(text) {
		row := channelDeliveryRecord{ID: uuid.NewString(), ChannelID: c.ID, InboxID: inbox.ID, RunID: inbox.RunID, Kind: kind, Chunk: index + 1, Payload: chunk, ConfigVersion: c.ConfigVersion, State: "pending", RetryAt: now, Deadline: now.Add(24 * time.Hour), CreatedAt: now}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "inbox_id"}, {Name: "kind"}, {Name: "chunk"}}, DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// Called by the sole terminal Event seam, inside the settlement transaction.
func enqueueRunChannelDelivery(tx *gorm.DB, id string) error {
	var inbox channelInboxRecord
	if err := tx.Where("run_id=?", id).Take(&inbox).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	var c channelRecord
	if err := tx.Where("id=?", inbox.ChannelID).Take(&c).Error; err != nil {
		return err
	}
	if !c.Enabled || c.DeletedAt != nil || c.ConfigVersion != inbox.ConfigVersion {
		return nil
	}
	var run runRecord
	if err := tx.Where("id=?", id).Take(&run).Error; err != nil {
		return err
	}
	text := "执行已取消 / Execution cancelled."
	kind := "status"
	if run.State == "succeeded" {
		result := runDomain(run)
		kind = "answer"
		if result.FinalText != nil && strings.TrimSpace(*result.FinalText) != "" {
			text = *result.FinalText
		} else if result.FinalJSON != nil {
			b, err := json.Marshal(result.FinalJSON)
			if err != nil {
				return err
			}
			text = string(b)
		} else {
			text = "执行完成，没有文本结果 / Completed without a text result."
		}
	} else if run.State == "failed" {
		text = "执行失败，请联系工作流拥有者 / Execution failed; please contact the Workflow owner."
	}
	// Private download capabilities must stay within the authenticated product.
	for _, marker := range []string{"/api/v1/", "X-Amz-Signature=", "x-oss-signature="} {
		if strings.Contains(text, marker) {
			text = "执行完成，请联系工作流拥有者查看结果 / Completed; please contact the Workflow owner to view the result."
			break
		}
	}
	return enqueueChannelDelivery(tx, c, inbox, kind, text)
}

func (r *Repository) ClaimChannelDelivery(ctx context.Context) (*application.ChannelSendJob, error) {
	if err := r.acquireWorkerClaimLock(ctx); err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&channelDeliveryRecord{}).Where("state='sending' AND lease_until<now()").Updates(map[string]any{"state": "outcome_unknown", "error_code": "send_interrupted", "lease": nil, "lease_until": nil}).Error; err != nil {
		return nil, err
	}

	var candidate channelDeliveryRecord
	if err := r.db.WithContext(ctx).Raw(`SELECT d.* FROM message_channel_deliveries d JOIN message_channel_inbox source ON source.id=d.inbox_id JOIN workflow_message_channels channel ON channel.id=d.channel_id
 WHERE channel.send_after<=now() AND d.state IN ('pending','retry_wait') AND d.retry_at<=now() AND NOT EXISTS (
 SELECT 1 FROM message_channel_deliveries prior JOIN message_channel_inbox i ON i.id=prior.inbox_id
 WHERE prior.channel_id=d.channel_id AND ((prior.inbox_id=d.inbox_id AND prior.kind=d.kind AND prior.chunk<d.chunk AND prior.state<>'sent')
 OR (i.chat_id=source.chat_id AND i.sender_id=source.sender_id AND i.generation=source.generation AND i.message->>'thread_id'=source.message->>'thread_id'
 AND (i.received_at,i.id)<(source.received_at,source.id) AND prior.kind<>'validation' AND prior.state IN ('pending','sending','retry_wait','outcome_unknown'))))
 ORDER BY d.created_at,d.chunk,d.id LIMIT 1`).Scan(&candidate).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if candidate.ID == "" {
		return nil, nil
	}
	var job *application.ChannelSendJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var c channelRecord
		if err := tx.Where("id=?", candidate.ChannelID).Take(&c).Error; err != nil {
			return err
		}
		var w workflowRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("id=?", c.WorkflowID).Take(&w).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=?", c.ID).Take(&c).Error; err != nil {
			return err
		}
		if err := tx.Model(&channelDeliveryRecord{}).Where("channel_id=? AND state='sending' AND lease_until<now()", c.ID).Updates(map[string]any{"state": "outcome_unknown", "error_code": "send_interrupted", "lease": nil, "lease_until": nil}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND state IN ('pending','retry_wait')", candidate.ID).Take(&candidate).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		var inbox channelInboxRecord
		if err := tx.Where("id=?", candidate.InboxID).Take(&inbox).Error; err != nil {
			return err
		}
		stored, err := c.stored()
		if err != nil {
			return err
		}
		var message domain.ChannelMessage
		if err = json.Unmarshal(inbox.Message, &message); err != nil {
			return err
		}
		var ownerEnabled int64
		if err := tx.Table("users").Where("id=? AND disabled_at IS NULL", c.OwnerUserID).Count(&ownerEnabled).Error; err != nil {
			return err
		}
		valid := c.Enabled || candidate.Kind == "validation" && c.ValidationState == "testing" && c.ValidationUntil != nil && time.Now().Before(*c.ValidationUntil)
		if !valid || ownerEnabled == 0 || w.DeletedAt != nil || c.DeletedAt != nil || c.ConfigVersion != candidate.ConfigVersion || !stored.Channel.Audience.Allows(message) {
			return tx.Model(&candidate).Update("state", "cancelled").Error
		}
		now, err := channelDatabaseTime(tx)
		if err != nil {
			return err
		}
		if now.After(candidate.Deadline) {
			return tx.Model(&candidate).Update("state", "expired").Error
		}
		var blocked int64
		query := `SELECT count(*) FROM message_channel_deliveries d JOIN message_channel_inbox i ON i.id=d.inbox_id WHERE d.channel_id=? AND ((d.inbox_id=? AND d.kind=? AND d.chunk<? AND d.state<>'sent') OR (i.chat_id=? AND i.sender_id=? AND i.generation=? AND i.message->>'thread_id'=? AND (i.received_at,i.id)<(?,?::uuid) AND d.kind<>'validation' AND d.state IN ('pending','sending','retry_wait','outcome_unknown')))`
		if err := tx.Raw(query, c.ID, inbox.ID, candidate.Kind, candidate.Chunk, inbox.ChatID, inbox.SenderID, inbox.Generation, message.ThreadID, inbox.ReceivedAt, inbox.ID).Scan(&blocked).Error; err != nil {
			return err
		}
		if blocked > 0 {
			return nil
		}
		if now.Before(c.SendAfter) {
			return nil
		}
		if err := tx.Model(&c).Update("send_after", now.Add(channelLimits(tx).SendInterval)).Error; err != nil {
			return err
		}
		lease := uuid.NewString()
		until := now.Add(time.Minute)
		if err := tx.Model(&candidate).Updates(map[string]any{"state": "sending", "attempts": gorm.Expr("attempts+1"), "lease": lease, "lease_until": until}).Error; err != nil {
			return err
		}
		candidate.State = "sending"
		candidate.Attempts++
		job = &application.ChannelSendJob{Delivery: candidate.projection(), Stored: stored, Message: message, ReplyCiphertext: inbox.ReplyCiphertext, InboxID: inbox.ID, Text: candidate.Payload, Lease: lease}
		return nil
	})
	return job, err
}

func (r *Repository) FinishChannelDelivery(ctx context.Context, job *application.ChannelSendJob, result application.ChannelSendResult) error {
	if result.State != "sent" && result.State != "retry_wait" && result.State != "failed" && result.State != "expired" && result.State != "outcome_unknown" {
		result = application.ChannelSendResult{State: "outcome_unknown", Code: "invalid_send_result"}
	}
	if len(result.Code) > 100 || len(result.MessageID) > 256 {
		result = application.ChannelSendResult{State: "outcome_unknown", Code: "invalid_send_result"}
	}
	if result.State == "retry_wait" && job.Delivery.Attempts >= channelLimits(r.db).MaxSendAttempts {
		result.State = "failed"
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockChannelWorkflow(tx, job.Stored.Channel.OwnerID, job.Stored.Channel.WorkflowID); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		var c channelRecord
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=?", job.Stored.Channel.ID).Take(&c).Error; err != nil {
			return err
		}
		now, err := channelDatabaseTime(tx)
		if err != nil {
			return err
		}
		retry := now.Add(max(time.Second*time.Duration(1<<min(job.Delivery.Attempts, 8)), min(result.RetryAfter, 24*time.Hour)) + time.Duration(rand.IntN(500))*time.Millisecond)
		update := tx.Model(&channelDeliveryRecord{}).Where("id=? AND state='sending' AND lease=?", job.Delivery.ID, job.Lease).Updates(map[string]any{"state": result.State, "error_code": result.Code, "provider_message_id": result.MessageID, "retry_at": retry, "lease": nil, "lease_until": nil})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return domain.ErrConflict
		}
		if job.Delivery.Kind == "validation" && result.State == "sent" && c.ConfigVersion == job.Stored.Channel.ConfigVersion && c.ValidationState == "testing" && c.ValidationUntil != nil && time.Now().Before(*c.ValidationUntil) && strings.TrimSpace(job.Message.Text) == c.ValidationCode {
			return tx.Model(&c).Updates(map[string]any{"validation_state": "passed", "validation_code": "", "validation_until": nil, "version": gorm.Expr("version+1"), "health": "connected", "error_code": ""}).Error
		}
		if result.State == "failed" || result.State == "expired" {
			return tx.Model(&channelDeliveryRecord{}).Where("inbox_id=? AND kind=? AND chunk>? AND state IN ('pending','retry_wait')", job.InboxID, job.Delivery.Kind, job.Delivery.Chunk).Update("state", "cancelled").Error
		}
		return nil
	})
}
func (r *Repository) ListChannelDeliveries(ctx context.Context, owner, workflow, id string) ([]domain.ChannelDelivery, error) {
	if _, err := r.GetMessageChannel(ctx, owner, workflow, id); err != nil {
		return nil, err
	}
	var rows []channelDeliveryRecord
	err := r.db.WithContext(ctx).Where("channel_id=?", id).Order("created_at DESC,chunk").Limit(100).Find(&rows).Error
	items := make([]domain.ChannelDelivery, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.projection())
	}
	return items, err
}
func (r *Repository) RetryChannelDelivery(ctx context.Context, owner, workflow, id, delivery string, version int64, confirm bool) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockChannelWorkflow(tx, owner, workflow); err != nil {
			return err
		}
		var c channelRecord
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=? AND owner_user_id=? AND workflow_id=? AND deleted_at IS NULL", id, owner, workflow).Take(&c).Error; err != nil {
			return mapNotFound(err)
		}
		if !c.Enabled || c.Version != version {
			return domain.ErrConflict
		}
		var row channelDeliveryRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND channel_id=?", delivery, id).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.ConfigVersion != c.ConfigVersion || time.Now().After(row.Deadline) || row.State != "failed" && row.State != "outcome_unknown" {
			return domain.ErrConflict
		}
		if row.State == "outcome_unknown" && !confirm {
			return domain.ErrInvalid
		}
		if err := tx.Model(&row).Updates(map[string]any{"state": "pending", "retry_at": time.Now(), "attempts": 0, "error_code": ""}).Error; err != nil {
			return err
		}
		return tx.Model(&channelDeliveryRecord{}).Where("inbox_id=? AND kind=? AND chunk>? AND state='cancelled' AND config_version=?", row.InboxID, row.Kind, row.Chunk, c.ConfigVersion).Updates(map[string]any{"state": "pending", "retry_at": time.Now(), "attempts": 0, "error_code": ""}).Error
	})
}
func (r *Repository) ActiveMessageChannels(ctx context.Context) ([]application.ChannelStored, error) {
	if err := r.acquireWorkerClaimLock(ctx); err != nil {
		return nil, err
	}
	var rows []channelRecord
	err := r.db.WithContext(ctx).Where("deleted_at IS NULL AND (enabled OR (validation_state='testing' AND validation_until>now())) AND EXISTS(SELECT 1 FROM users WHERE users.id=workflow_message_channels.owner_user_id AND disabled_at IS NULL) AND EXISTS(SELECT 1 FROM workflows WHERE workflows.id=workflow_message_channels.workflow_id AND deleted_at IS NULL)").Order("id").Find(&rows).Error
	items := make([]application.ChannelStored, 0, len(rows))
	for _, row := range rows {
		stored, e := row.stored()
		if e != nil {
			return nil, e
		}
		items = append(items, stored)
	}
	return items, err
}
func (r *Repository) SetChannelHealth(ctx context.Context, id string, version int64, health, code string) error {
	return r.db.WithContext(ctx).Model(&channelRecord{}).Where("id=? AND config_version=? AND deleted_at IS NULL", id, version).Updates(map[string]any{"health": health, "error_code": code}).Error
}

func (r *Repository) ChannelAccountReceiving(ctx context.Context, provider, binding string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&channelRecord{}).Where("deleted_at IS NULL AND provider=? AND binding_id=? AND (enabled OR (validation_state='testing' AND validation_until>now()))", provider, binding).Count(&count).Error
	return count > 0, err
}

func enqueueChannelWaiting(tx *gorm.DB, runID string) error {
	var inbox channelInboxRecord
	if err := tx.Where("run_id=?", runID).Take(&inbox).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	var channel channelRecord
	if err := tx.Where("id=?", inbox.ChannelID).Take(&channel).Error; err != nil {
		return err
	}
	if !channel.Enabled || channel.DeletedAt != nil || channel.ConfigVersion != inbox.ConfigVersion {
		return nil
	}
	return enqueueChannelDelivery(tx, channel, inbox, "waiting", "等待工作流拥有者处理 / Waiting for the Workflow owner to take action.")
}

// Bound each maintenance pass. Keep dedup identities after scrubbing private
// text and short-lived reply capabilities; old messages also fail the 24h gate.
func (r *Repository) MaintainMessageChannels(ctx context.Context) (bool, error) {
	if err := r.acquireWorkerClaimLock(ctx); err != nil {
		return false, err
	}
	affected := int64(0)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		update := tx.Exec(`UPDATE message_channel_deliveries SET state='expired',lease=NULL,lease_until=NULL WHERE id IN (SELECT id FROM message_channel_deliveries WHERE deadline<now() AND state IN ('pending','retry_wait','outcome_unknown') ORDER BY deadline LIMIT 1000 FOR UPDATE SKIP LOCKED)`)
		if update.Error != nil {
			return update.Error
		}
		affected += update.RowsAffected
		update = tx.Exec(`UPDATE message_channel_inbox SET message=jsonb_set(message,'{text}','""'::jsonb),reply_ciphertext=''::bytea WHERE id IN (SELECT id FROM message_channel_inbox WHERE received_at<now()-interval '30 days' AND state<>'received' AND (length(reply_ciphertext)>0 OR message->>'text'<>'') ORDER BY received_at LIMIT 1000 FOR UPDATE SKIP LOCKED)`)
		if update.Error != nil {
			return update.Error
		}
		affected += update.RowsAffected
		update = tx.Exec(`UPDATE message_channel_deliveries SET payload='' WHERE id IN (SELECT id FROM message_channel_deliveries WHERE created_at<now()-interval '30 days' AND state NOT IN ('pending','sending','retry_wait','outcome_unknown') AND payload<>'' ORDER BY created_at LIMIT 1000 FOR UPDATE SKIP LOCKED)`)
		if update.Error != nil {
			return update.Error
		}
		affected += update.RowsAffected
		return nil
	})
	return affected > 0, err
}

const channelProtectionKey = "workspace.message-channel-protection"
const channelEnabledKey = "workspace.message-channel-enabled"

func (r *Repository) ConfigureChannelProtection(cipher application.ChannelCipher, enabled bool, limits application.ChannelLimits) {
	// Set initializes a mutable Statement. Restore a reusable Session while
	// preserving the settings so later subqueries cannot mutate the repository.
	r.db = r.db.Set(channelProtectionKey, cipher).Set(channelEnabledKey, enabled).Set(channelLimitsKey, limits.Effective()).Session(&gorm.Session{})
}

func channelRedactionValues(tx *gorm.DB, runID string) ([][]byte, error) {
	var inbox channelInboxRecord
	if err := tx.Where("run_id=?", runID).Take(&inbox).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var row channelRecord
	if err := tx.Where("id=?", inbox.ChannelID).Take(&row).Error; err != nil {
		return nil, err
	}
	if len(row.CredentialCiphertext) == 0 && row.DeletedAt != nil {
		return nil, nil
	}
	stored, err := row.stored()
	if err != nil {
		return nil, err
	}
	value, ok := tx.Get(channelProtectionKey)
	cipher, valid := value.(application.ChannelCipher)
	if !ok || !valid {
		return nil, fmt.Errorf("channel_protection_unavailable")
	}
	plain, err := cipher.Decrypt(row.CredentialCiphertext, application.ChannelCredentialAAD(stored.Channel))
	if err != nil {
		return nil, fmt.Errorf("channel_credentials_unavailable")
	}
	var c application.ChannelCredentials
	if json.Unmarshal(plain, &c) != nil {
		return nil, fmt.Errorf("channel_credentials_unavailable")
	}
	values := [][]byte{}
	for _, key := range application.ChannelSecretKeys() {
		if c[key] != "" {
			values = append(values, []byte(c[key]))
		}
	}
	if len(inbox.ReplyCiphertext) > 0 {
		var message domain.ChannelMessage
		var reply map[string]string
		if json.Unmarshal(inbox.Message, &message) != nil {
			return nil, fmt.Errorf("channel_reply_unavailable")
		}
		plain, err = cipher.Decrypt(inbox.ReplyCiphertext, application.ChannelReplyAAD(row.ID, message))
		if err != nil || json.Unmarshal(plain, &reply) != nil {
			return nil, fmt.Errorf("channel_reply_unavailable")
		}
		values = append(values, application.ChannelReplySecrets(reply)...)
	}
	return values, nil
}
func channelRedactor(tx *gorm.DB, job application.ExecutionJob) (*credentials.Redactor, error) {
	if job.Kind != application.JobWorkflow {
		return credentials.NewRedactor(job.AdditionalRedactionValues...), nil
	}
	values, err := channelRedactionValues(tx, job.ID)
	if err != nil {
		return nil, err
	}
	values = append(values, job.AdditionalRedactionValues...)
	for _, value := range append([][]byte(nil), values...) {
		encoded, err := json.Marshal(string(value))
		if err == nil && len(encoded) > 2 {
			values = append(values, encoded[1:len(encoded)-1])
		}
	}
	return credentials.NewRedactor(values...), nil
}
func redactChannelJSON(value any, redactor *credentials.Redactor) any {
	switch v := value.(type) {
	case string:
		return string(redactor.Bytes([]byte(v)))
	case map[string]any:
		for k, item := range v {
			v[k] = redactChannelJSON(item, redactor)
		}
		return v
	case []any:
		for index, item := range v {
			v[index] = redactChannelJSON(item, redactor)
		}
		return v
	default:
		return value
	}
}

const channelLimitsKey = "workspace.message-channel-limits"

func channelLimits(tx *gorm.DB) application.ChannelLimits {
	value, _ := tx.Get(channelLimitsKey)
	limits, _ := value.(application.ChannelLimits)
	return limits.Effective()
}

// Eligibility, cooldown and leases share the database clock. Host/VM clock
// skew must not make newly enqueued work randomly appear not yet due.
func channelDatabaseTime(tx *gorm.DB) (time.Time, error) {
	var now time.Time
	if err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error; err != nil {
		return time.Time{}, err
	}
	return now.UTC(), nil
}
