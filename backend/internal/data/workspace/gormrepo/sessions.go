package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) ListSessions(ctx context.Context, ownerID string, archived bool) ([]domain.Session, error) {
	query := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID)
	if archived {
		query = query.Where("archived_at IS NOT NULL")
	} else {
		query = query.Where("archived_at IS NULL")
	}
	var rows []sessionRecord
	if err := query.Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Sessions: %w", err)
	}
	items := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		items = append(items, sessionDomain(row))
	}
	return items, nil
}

func (repository *Repository) CreateSession(ctx context.Context, ownerID string, expertID, expertTeamID *string) (domain.Session, error) {
	if expertID != nil && expertTeamID != nil {
		return domain.Session{}, fmt.Errorf("%w: choose either an Expert or an Expert Team", domain.ErrInvalid)
	}
	row := sessionRecord{ID: uuid.NewString(), OwnerID: ownerID, Title: "New session", ExpertID: expertID, ExpertTeamID: expertTeamID, Version: 1}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if expertID != nil {
			var count int64
			if err := tx.Model(&expertRecord{}).Where("owner_user_id = ? AND id = ? AND introduction <> '' AND core_capability <> '' AND operating_procedure <> '' AND output_standard <> ''", ownerID, *expertID).Count(&count).Error; err != nil || count != 1 {
				return domain.ErrInvalid
			}
		}
		if expertTeamID != nil {
			var team expertTeamRecord
			if err := tx.Where("owner_user_id = ? AND id = ?", ownerID, *expertTeamID).Take(&team).Error; err != nil {
				return domain.ErrInvalid
			}
			var ids []string
			if err := json.Unmarshal(team.ExpertIDs, &ids); err != nil || len(ids) < 2 {
				return domain.ErrInvalid
			}
			var count int64
			if err := tx.Model(&expertRecord{}).Where("owner_user_id = ? AND id IN ? AND introduction <> '' AND core_capability <> '' AND operating_procedure <> '' AND output_standard <> ''", ownerID, ids).Count(&count).Error; err != nil || count != int64(len(ids)) {
				return domain.ErrInvalid
			}
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return domain.Session{}, fmt.Errorf("create Session: %w", err)
	}
	return sessionDomain(row), nil
}

func (repository *Repository) GetSession(ctx context.Context, ownerID, sessionID string) (domain.Session, error) {
	row, err := repository.session(ctx, ownerID, sessionID, false)
	return sessionDomain(row), err
}

func (repository *Repository) UpdateSession(ctx context.Context, ownerID, sessionID, title string, expectedVersion int64) (domain.Session, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 200 {
		return domain.Session{}, fmt.Errorf("%w: Session title must contain 1-200 characters", domain.ErrInvalid)
	}
	result := repository.db.WithContext(ctx).Model(&sessionRecord{}).
		Where("owner_user_id = ? AND id = ? AND version = ?", ownerID, sessionID, expectedVersion).
		Updates(map[string]any{"title": title, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return domain.Session{}, fmt.Errorf("rename Session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.Session{}, domain.ErrConflict
	}
	return repository.GetSession(ctx, ownerID, sessionID)
}

func (repository *Repository) SetSessionArchived(ctx context.Context, ownerID, sessionID string, archived bool, expectedVersion int64) (domain.Session, error) {
	var archivedAt any
	if archived {
		archivedAt = gorm.Expr("now()")
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&sessionRecord{}).
			Where("owner_user_id = ? AND id = ? AND version = ?", ownerID, sessionID, expectedVersion).
			Updates(map[string]any{"archived_at": archivedAt, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		if archived {
			now := time.Now().UTC()
			return tx.Model(&messageRecord{}).Where("session_id = ? AND role = 'assistant' AND state IN ?", sessionID, []string{"queued", "generating"}).Updates(map[string]any{"state": "cancelled", "progress_stage": "", "completed_at": now}).Error
		}
		return nil
	})
	if err != nil {
		return domain.Session{}, fmt.Errorf("archive Session: %w", err)
	}
	return repository.GetSession(ctx, ownerID, sessionID)
}

func (repository *Repository) SetSessionExpertSelection(ctx context.Context, ownerID, sessionID string, expertID, expertTeamID *string, expectedVersion int64) (domain.Session, error) {
	if expertID != nil && expertTeamID != nil {
		return domain.Session{}, fmt.Errorf("%w: choose either an Expert or an Expert Team", domain.ErrInvalid)
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session sessionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND id = ? AND archived_at IS NULL AND version = ?", ownerID, sessionID, expectedVersion).Take(&session).Error; err != nil {
			return mapNotFound(err)
		}
		var messageCount int64
		if err := tx.Model(&messageRecord{}).Where("session_id = ?", sessionID).Count(&messageCount).Error; err != nil {
			return err
		}
		if messageCount != 0 {
			return fmt.Errorf("%w: Expert selection is frozen after the first message", domain.ErrConflict)
		}
		if expertID != nil {
			var count int64
			if err := tx.Model(&expertRecord{}).Where("owner_user_id = ? AND id = ? AND introduction <> '' AND core_capability <> '' AND operating_procedure <> '' AND output_standard <> ''", ownerID, *expertID).Count(&count).Error; err != nil || count != 1 {
				return fmt.Errorf("%w: selected Expert is unavailable", domain.ErrInvalid)
			}
		}
		if expertTeamID != nil {
			var team expertTeamRecord
			if err := tx.Where("owner_user_id = ? AND id = ?", ownerID, *expertTeamID).Take(&team).Error; err != nil {
				return fmt.Errorf("%w: selected Expert Team is unavailable", domain.ErrInvalid)
			}
			var ids []string
			if err := json.Unmarshal(team.ExpertIDs, &ids); err != nil || len(ids) < 2 {
				return fmt.Errorf("%w: selected Expert Team is unavailable", domain.ErrInvalid)
			}
			var count int64
			if err := tx.Model(&expertRecord{}).Where("owner_user_id = ? AND id IN ? AND introduction <> '' AND core_capability <> '' AND operating_procedure <> '' AND output_standard <> ''", ownerID, ids).Count(&count).Error; err != nil || count != int64(len(ids)) {
				return fmt.Errorf("%w: selected Expert Team is unavailable", domain.ErrInvalid)
			}
		}
		result := tx.Model(&sessionRecord{}).Where("id = ? AND version = ?", sessionID, expectedVersion).Updates(map[string]any{"expert_id": expertID, "expert_team_id": expertTeamID, "expert_snapshot": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return nil
	})
	if err != nil {
		return domain.Session{}, fmt.Errorf("set Session Expert selection: %w", err)
	}
	return repository.GetSession(ctx, ownerID, sessionID)
}

func (repository *Repository) DeleteSession(ctx context.Context, ownerID, sessionID string) error {
	result := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, sessionID).Delete(&sessionRecord{})
	if result.Error != nil {
		return fmt.Errorf("delete Session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (repository *Repository) ListMessages(ctx context.Context, ownerID, sessionID string, after int64, limit int) ([]domain.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if _, err := repository.session(ctx, ownerID, sessionID, false); err != nil {
		return nil, err
	}
	var rows []messageRecord
	err := repository.db.WithContext(ctx).Table("session_messages message").
		Joins("JOIN sessions session ON session.id = message.session_id").
		Where("session.owner_user_id = ? AND message.session_id = ? AND message.id > ?", ownerID, sessionID, after).
		Select("message.*").Order("message.id").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list Session Messages: %w", err)
	}
	items := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		items = append(items, messageDomain(row))
	}
	if err := repository.loadSessionArtifacts(ctx, ownerID, sessionID, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (repository *Repository) GetMessage(ctx context.Context, ownerID, sessionID string, messageID int64) (domain.Message, error) {
	var row messageRecord
	err := repository.db.WithContext(ctx).Table("session_messages message").
		Joins("JOIN sessions session ON session.id = message.session_id").
		Where("session.owner_user_id = ? AND message.session_id = ? AND message.id = ?", ownerID, sessionID, messageID).
		Select("message.*").Take(&row).Error
	if err != nil {
		return domain.Message{}, mapNotFound(err)
	}
	item := messageDomain(row)
	items := []domain.Message{item}
	if err := repository.loadSessionArtifacts(ctx, ownerID, sessionID, items); err != nil {
		return domain.Message{}, err
	}
	return items[0], nil
}

func (repository *Repository) loadSessionArtifacts(ctx context.Context, ownerID, sessionID string, messages []domain.Message) error {
	if len(messages) == 0 {
		return nil
	}
	messageIndexes := make(map[int64]int, len(messages))
	messageIDs := make([]int64, 0, len(messages))
	for index := range messages {
		messageIndexes[messages[index].ID] = index
		messageIDs = append(messageIDs, messages[index].ID)
	}
	var rows []sessionArtifactRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND session_id = ? AND message_id IN ?", ownerID, sessionID, messageIDs).Order("created_at, id").Find(&rows).Error; err != nil {
		return fmt.Errorf("list Session Artifacts: %w", err)
	}
	for _, row := range rows {
		index, ok := messageIndexes[row.MessageID]
		if !ok {
			continue
		}
		messages[index].Artifacts = append(messages[index].Artifacts, sessionArtifactDomain(row))
	}
	return nil
}

func (repository *Repository) GetSessionArtifact(ctx context.Context, ownerID, sessionID, artifactID string) (domain.Artifact, error) {
	var row sessionArtifactRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND session_id = ? AND id = ?", ownerID, sessionID, artifactID).Take(&row).Error; err != nil {
		return domain.Artifact{}, mapNotFound(err)
	}
	return sessionArtifactDomain(row), nil
}

func sessionArtifactDomain(row sessionArtifactRecord) domain.Artifact {
	item := domain.Artifact{ID: row.ID, MessageID: row.MessageID, Kind: "file", Name: row.Name, Path: row.Path, ObjectKey: row.ObjectKey, Size: row.Size, SHA256: row.SHA256, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt}
	if len(row.TextResult) > 0 {
		_ = json.Unmarshal(row.TextResult, &item.TextPreview)
	}
	return item
}

func (repository *Repository) CreateMessagePair(ctx context.Context, ownerID, sessionID, content string, attachments []domain.Attachment) (domain.Message, domain.Message, error) {
	return repository.createMessagePair(ctx, ownerID, sessionID, content, attachments, nil)
}

func (repository *Repository) createMessagePair(ctx context.Context, ownerID, sessionID, content string, attachments []domain.Attachment, frozen *domain.ResponseSnapshot) (domain.Message, domain.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" && len(attachments) == 0 || len(content) > 100_000 {
		return domain.Message{}, domain.Message{}, fmt.Errorf("%w: message must contain text or an attachment", domain.ErrInvalid)
	}
	encodedAttachments, err := marshal(attachments)
	if err != nil {
		return domain.Message{}, domain.Message{}, fmt.Errorf("encode message attachments: %w", err)
	}
	var user, assistant messageRecord
	err = repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session sessionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND id = ? AND archived_at IS NULL", ownerID, sessionID).Take(&session).Error; err != nil {
			return mapNotFound(err)
		}
		snapshot := frozen
		if snapshot == nil {
			selected, err := responseSnapshotOnTx(tx, session)
			if err != nil {
				return err
			}
			snapshot = &selected
		}
		if len(session.ExpertSnapshot) == 0 {
			if _, err := loadSessionSnapshot(tx, session, *snapshot); err != nil {
				return err
			}
		}
		encodedSnapshot, err := marshal(snapshot)
		if err != nil {
			return err
		}
		user, assistant = sessionMessagePairRecords(sessionID, content, encodedAttachments, encodedSnapshot)
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if err := tx.Create(&assistant).Error; err != nil {
			return err
		}
		updates := map[string]any{"updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
		if session.Title == "New session" {
			title := content
			if strings.TrimSpace(title) == "" && len(attachments) > 0 {
				title = attachments[0].Name
			}
			updates["title"] = sessionTitle(title)
		}
		return tx.Model(&sessionRecord{}).Where("id = ?", sessionID).Updates(updates).Error
	})
	if err != nil {
		return domain.Message{}, domain.Message{}, fmt.Errorf("append Session message: %w", err)
	}
	return messageDomain(user), messageDomain(assistant), nil
}

func sessionMessagePairRecords(sessionID, content string, attachments, responseSnapshot []byte) (messageRecord, messageRecord) {
	emptyJSONList := []byte("[]")
	user := messageRecord{
		SessionID: sessionID, Role: "user", State: "completed", Content: content,
		Attachments: attachments, ExpertStages: emptyJSONList, RuntimeActivities: emptyJSONList,
	}
	assistant := messageRecord{
		SessionID: sessionID, Role: "assistant", State: "queued", ProgressStage: "preparing",
		ResponseSnapshot: responseSnapshot, Attachments: emptyJSONList, ExpertStages: emptyJSONList, RuntimeActivities: emptyJSONList,
	}
	return user, assistant
}

func sessionTitle(content string) string {
	value := []rune(strings.Join(strings.Fields(content), " "))
	if len(value) > 60 {
		value = value[:60]
	}
	return string(value)
}

func (repository *Repository) RetryMessage(ctx context.Context, ownerID, sessionID string, messageID int64) (domain.Message, domain.Message, error) {
	var original messageRecord
	err := repository.db.WithContext(ctx).Table("session_messages message").
		Joins("JOIN sessions session ON session.id = message.session_id").
		Where("session.owner_user_id = ? AND message.session_id = ? AND message.id = ? AND message.role = 'user'", ownerID, sessionID, messageID).
		Select("message.*").Take(&original).Error
	if err != nil {
		return domain.Message{}, domain.Message{}, mapNotFound(err)
	}
	var assistant messageRecord
	if err := repository.db.WithContext(ctx).Where("session_id = ? AND id > ? AND role = 'assistant'", sessionID, original.ID).Order("id").Take(&assistant).Error; err != nil {
		return domain.Message{}, domain.Message{}, mapNotFound(err)
	}
	if len(assistant.ResponseSnapshot) == 0 {
		return domain.Message{}, domain.Message{}, fmt.Errorf("%w: original response has no execution snapshot", domain.ErrConflict)
	}
	var snapshot domain.ResponseSnapshot
	if err := json.Unmarshal(assistant.ResponseSnapshot, &snapshot); err != nil {
		return domain.Message{}, domain.Message{}, fmt.Errorf("decode original Response Snapshot: %w", err)
	}
	var attachments []domain.Attachment
	if len(original.Attachments) > 0 {
		_ = json.Unmarshal(original.Attachments, &attachments)
	}
	return repository.createMessagePair(ctx, ownerID, sessionID, original.Content, attachments, &snapshot)
}

func (repository *Repository) CancelMessage(ctx context.Context, ownerID, sessionID string, messageID int64) (domain.Message, error) {
	var row messageRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`
			SELECT message.* FROM session_messages message
			JOIN sessions session ON session.id = message.session_id
			WHERE session.owner_user_id = ? AND message.session_id = ? AND message.id = ? AND message.role = 'assistant'
			FOR UPDATE OF message`, ownerID, sessionID, messageID).Scan(&row).Error; err != nil {
			return err
		}
		if row.ID == 0 {
			return domain.ErrNotFound
		}
		now := time.Now().UTC()
		switch row.State {
		case "queued":
			if err := tx.Model(&messageRecord{}).Where("id = ? AND state = 'queued'", row.ID).Updates(map[string]any{
				"state": "cancelled", "progress_stage": "", "cancel_requested_at": now, "completed_at": now,
				"elapsed_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - created_at)) * 1000)::bigint", now),
			}).Error; err != nil {
				return err
			}
		case "generating":
			if row.CancelRequested == nil {
				if err := tx.Model(&messageRecord{}).Where("id = ? AND state = 'generating'", row.ID).Update("cancel_requested_at", now).Error; err != nil {
					return err
				}
			}
		case "cancelled":
			return nil
		default:
			return domain.ErrConflict
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	})
	if err != nil {
		return domain.Message{}, fmt.Errorf("cancel Session message: %w", err)
	}
	return messageDomain(row), nil
}

func (repository *Repository) session(ctx context.Context, ownerID, sessionID string, lock bool) (sessionRecord, error) {
	query := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, sessionID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row sessionRecord
	if err := query.Take(&row).Error; err != nil {
		return row, mapNotFound(err)
	}
	return row, nil
}

func sessionDomain(row sessionRecord) domain.Session {
	return domain.Session{ID: row.ID, OwnerID: row.OwnerID, Title: row.Title, ExpertID: row.ExpertID, ExpertTeamID: row.ExpertTeamID, ArchivedAt: row.ArchivedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}

func messageDomain(row messageRecord) domain.Message {
	value := domain.Message{ID: row.ID, SessionID: row.SessionID, Role: row.Role, State: row.State, Content: row.Content, ProgressStage: row.ProgressStage, ElapsedMS: row.ElapsedMS, CreatedAt: row.CreatedAt}
	if row.Error != nil {
		value.Error = *row.Error
	}
	if len(row.ResponseSnapshot) > 0 && string(row.ResponseSnapshot) != "null" {
		value.ResponseSnapshot = &domain.ResponseSnapshot{}
		if err := json.Unmarshal(row.ResponseSnapshot, value.ResponseSnapshot); err != nil {
			value.ResponseSnapshot = nil
		}
	}
	if len(row.Attachments) > 0 && string(row.Attachments) != "null" {
		_ = json.Unmarshal(row.Attachments, &value.Attachments)
	}
	if len(row.ExpertStages) > 0 && string(row.ExpertStages) != "null" {
		_ = json.Unmarshal(row.ExpertStages, &value.ExpertStages)
	}
	if len(row.CreditConsumption) > 0 && string(row.CreditConsumption) != "null" {
		value.CreditConsumption = &domain.CreditConsumption{}
		if err := json.Unmarshal(row.CreditConsumption, value.CreditConsumption); err != nil {
			value.CreditConsumption = nil
		}
	}
	if len(row.RuntimeActivities) > 0 && string(row.RuntimeActivities) != "null" {
		_ = json.Unmarshal(row.RuntimeActivities, &value.Activities)
	}
	return value
}

func responseSnapshotOnTx(tx *gorm.DB, session sessionRecord) (domain.ResponseSnapshot, error) {
	if len(session.ExpertSnapshot) > 0 && string(session.ExpertSnapshot) != "null" {
		var frozen domain.ExecutionSnapshot
		if err := json.Unmarshal(session.ExpertSnapshot, &frozen); err != nil {
			return domain.ResponseSnapshot{}, fmt.Errorf("decode frozen Session execution plan: %w", err)
		}
		return responseSnapshotFromExecution(frozen)
	}
	fake := workflowRecord{OwnerID: session.OwnerID, Name: session.Title, ExpertID: session.ExpertID, ExpertTeamID: session.ExpertTeamID, WorkspacePath: "sessions/" + session.OwnerID + "/" + session.ID}
	plan, err := loadExecutionSnapshot(tx, fake)
	if err != nil {
		return domain.ResponseSnapshot{}, err
	}
	return responseSnapshotFromExecution(plan)
}

func responseSnapshotFromExecution(plan domain.ExecutionSnapshot) (domain.ResponseSnapshot, error) {
	stages, err := plan.OrderedStages()
	if err != nil {
		return domain.ResponseSnapshot{}, err
	}
	return domain.ResponseSnapshot{SchemaVersion: 2, Stages: stages}, nil
}

var _ = time.Second
