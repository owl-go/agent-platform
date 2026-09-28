package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/application"
	"agent-platform/backend/internal/biz/aiapplication/domain"
	"agent-platform/backend/internal/secretcrypto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db  *gorm.DB
	box *secretcrypto.Box
}

func New(db *gorm.DB, box *secretcrypto.Box) *Repository { return &Repository{db: db, box: box} }

var _ application.Repository = (*Repository)(nil)

type assistantRecord struct {
	ID                 string     `gorm:"column:id"`
	OwnerID            string     `gorm:"column:owner_user_id"`
	Name               string     `gorm:"column:name"`
	Icon               string     `gorm:"column:icon"`
	Description        string     `gorm:"column:description"`
	Introduction       string     `gorm:"column:introduction"`
	Scenario           string     `gorm:"column:scenario"`
	Prompt             string     `gorm:"column:prompt"`
	PreprocessPrompt   string     `gorm:"column:preprocess_prompt"`
	ProviderModelID    string     `gorm:"column:provider_model_id"`
	ServiceGoal        string     `gorm:"column:service_goal"`
	AnswerScope        string     `gorm:"column:answer_scope"`
	OperatingRules     string     `gorm:"column:operating_rules"`
	ResponseStyle      string     `gorm:"column:response_style"`
	KnowledgeBaseIDs   []byte     `gorm:"column:knowledge_base_ids;type:jsonb"`
	ExpertID           *string    `gorm:"column:expert_id"`
	ExpertTeamID       *string    `gorm:"column:expert_team_id"`
	DigitalHumanID     *string    `gorm:"column:digital_human_id"`
	Share              []byte     `gorm:"column:share;type:jsonb"`
	ShareTokenHash     *string    `gorm:"column:share_token_hash"`
	ShareTokenRevision int64      `gorm:"column:share_token_revision"`
	State              string     `gorm:"column:state"`
	LastValidatedAt    *time.Time `gorm:"column:last_validated_at"`
	ValidatedVersion   int64      `gorm:"column:validated_version"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	Version            int64      `gorm:"column:version"`
}

func (assistantRecord) TableName() string { return "smart_assistants" }

type humanRecord struct {
	ID               string    `gorm:"column:id"`
	OwnerID          string    `gorm:"column:owner_user_id"`
	Name             string    `gorm:"column:name"`
	AvatarObjectKey  string    `gorm:"column:avatar_object_key"`
	Voice            string    `gorm:"column:voice"`
	Language         string    `gorm:"column:language"`
	ExpressionStyle  string    `gorm:"column:expression_style"`
	SceneDescription string    `gorm:"column:scene_description"`
	State            string    `gorm:"column:state"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	Version          int64     `gorm:"column:version"`
}

func (humanRecord) TableName() string { return "digital_humans" }

type faqRecord struct {
	ID             string    `gorm:"column:id"`
	AssistantID    string    `gorm:"column:assistant_id"`
	Question       string    `gorm:"column:question"`
	AnswerMarkdown string    `gorm:"column:answer_markdown"`
	DisplayOrder   int       `gorm:"column:display_order"`
	Category       string    `gorm:"column:category"`
	Tag            string    `gorm:"column:tag"`
	Icon           string    `gorm:"column:icon"`
	Enabled        bool      `gorm:"column:enabled"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
	Version        int64     `gorm:"column:version"`
}

type assistantSessionRecord struct {
	SessionID         string    `gorm:"column:session_id"`
	AssistantID       string    `gorm:"column:assistant_id"`
	OwnerID           string    `gorm:"column:owner_user_id"`
	AssistantSnapshot []byte    `gorm:"column:assistant_snapshot;type:jsonb"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (assistantSessionRecord) TableName() string { return "smart_assistant_sessions" }

func (faqRecord) TableName() string { return "smart_assistant_faqs" }

func encode(value any) []byte { encoded, _ := json.Marshal(value); return encoded }
func decode(data []byte, value any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, value)
}

func (r *Repository) ListAssistants(ctx context.Context, owner string) ([]domain.SmartAssistant, error) {
	var rows []assistantRecord
	if err := r.db.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.SmartAssistant, 0, len(rows))
	for _, row := range rows {
		result = append(result, assistantFromRecord(row))
	}
	return result, nil
}
func (r *Repository) GetAssistant(ctx context.Context, owner, id string) (domain.SmartAssistant, error) {
	var row assistantRecord
	err := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", owner, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.SmartAssistant{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	return assistantFromRecord(row), nil
}
func (r *Repository) GetAssistantByShareTokenHash(ctx context.Context, hash string) (domain.SmartAssistant, error) {
	var row assistantRecord
	err := r.db.WithContext(ctx).Where("share_token_hash = ?", hash).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.SmartAssistant{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	return assistantFromRecord(row), nil
}
func (r *Repository) BindAssistantSession(ctx context.Context, owner, assistantID, sessionID string, snapshot []byte) error {
	return r.db.WithContext(ctx).Create(&assistantSessionRecord{SessionID: sessionID, AssistantID: assistantID, OwnerID: owner, AssistantSnapshot: snapshot, CreatedAt: time.Now().UTC()}).Error
}

func (r *Repository) ConsumeShareCall(ctx context.Context, assistantID string, now time.Time, dailyLimit int) (bool, error) {
	var allowed bool
	row := r.db.WithContext(ctx).Raw(`
		WITH consumed AS (
			INSERT INTO smart_assistant_share_usage (assistant_id, usage_day, calls, updated_at)
			VALUES (?, ?, 1, ?)
			ON CONFLICT (assistant_id, usage_day) DO UPDATE
		SET calls = smart_assistant_share_usage.calls + 1, updated_at = EXCLUDED.updated_at
		WHERE smart_assistant_share_usage.calls < ?
		RETURNING assistant_id
		)
		SELECT EXISTS (SELECT 1 FROM consumed)`, assistantID, now.UTC().Format("2006-01-02"), now.UTC(), dailyLimit).Row().Scan(&allowed)
	return allowed, row
}
func (r *Repository) CreateAssistant(ctx context.Context, owner string, assistant domain.SmartAssistant) (domain.SmartAssistant, error) {
	now := time.Now().UTC()
	assistant.ID = uuid.NewString()
	assistant.OwnerID = owner
	assistant.CreatedAt = now
	assistant.UpdatedAt = now
	assistant.Version = 1
	row := assistantRecordFromDomain(assistant)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.SmartAssistant{}, mapDBError(err)
	}
	return assistant, nil
}
func (r *Repository) UpdateAssistant(ctx context.Context, owner, id string, assistant domain.SmartAssistant, version int64) (domain.SmartAssistant, error) {
	share := assistant.Share
	share.Token = ""
	updates := map[string]any{"name": assistant.Name, "icon": assistant.Icon, "description": assistant.Description, "introduction": assistant.Introduction, "scenario": assistant.Scenario, "prompt": assistant.Prompt, "preprocess_prompt": assistant.PreprocessPrompt, "provider_model_id": assistant.ProviderModelID, "service_goal": assistant.ServiceGoal, "answer_scope": assistant.AnswerScope, "operating_rules": assistant.OperatingRules, "response_style": assistant.ResponseStyle, "knowledge_base_ids": encode(assistant.KnowledgeBaseIDs), "expert_id": assistant.ExpertID, "expert_team_id": assistant.ExpertTeamID, "digital_human_id": assistant.DigitalHumanID, "share": encode(share), "share_token_hash": optionalString(assistant.Share.TokenHash), "share_token_revision": assistant.Share.TokenRevision, "state": assistant.State, "last_validated_at": nil, "validated_version": 0, "updated_at": time.Now().UTC(), "version": version + 1}
	result := r.db.WithContext(ctx).Model(&assistantRecord{}).Where("owner_user_id = ? AND id = ? AND version = ?", owner, id, version).Updates(updates)
	if result.Error != nil {
		return domain.SmartAssistant{}, mapDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.SmartAssistant{}, domain.ErrVersionConflict
	}
	return r.GetAssistant(ctx, owner, id)
}

func (r *Repository) RecordPublicationValidation(ctx context.Context, owner, id string, version int64, checkedAt time.Time) (domain.SmartAssistant, error) {
	result := r.db.WithContext(ctx).Model(&assistantRecord{}).
		Where("owner_user_id = ? AND id = ? AND version = ?", owner, id, version).
		Updates(map[string]any{"last_validated_at": checkedAt.UTC(), "validated_version": version})
	if result.Error != nil {
		return domain.SmartAssistant{}, mapDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.SmartAssistant{}, domain.ErrVersionConflict
	}
	return r.GetAssistant(ctx, owner, id)
}

func (r *Repository) PublicationStats(ctx context.Context, owner, assistantID string, since time.Time) (domain.PublicationStats, error) {
	if _, err := r.GetAssistant(ctx, owner, assistantID); err != nil {
		return domain.PublicationStats{}, err
	}
	var stats domain.PublicationStats
	if err := r.db.WithContext(ctx).Raw(`
		WITH scoped_conversations AS (
			SELECT id, created_at
			FROM assistant_conversations
			WHERE owner_user_id = ?
				AND assistant_id = ?
				AND visitor_hash <> ''
		), scoped_turns AS (
			SELECT turn.id, turn.source, turn.state
			FROM assistant_conversation_turns turn
			JOIN scoped_conversations conversation ON conversation.id = turn.conversation_id
			WHERE turn.created_at >= ?
		)
		SELECT
			(SELECT COUNT(*) FROM scoped_conversations WHERE created_at >= ?) AS external_conversations,
			(SELECT COUNT(*) FROM scoped_turns) AS free_text_calls,
			(SELECT COUNT(*) FROM scoped_turns WHERE source = 'faq' AND state = 'completed') AS faq_answers,
			(SELECT COUNT(*) FROM scoped_turns WHERE source <> 'faq' AND state = 'completed') AS model_answers,
			(SELECT COUNT(*) FROM scoped_turns WHERE state IN ('failed', 'cancelled')) AS failed_or_cancelled_answers,
			COALESCE((
				SELECT SUM(-ledger.amount_hundredths)
				FROM credit_ledger ledger
				JOIN scoped_turns turn ON ledger.source LIKE 'assistant-turn-' || turn.id::text || ':%'
				WHERE ledger.user_id = ?
					AND ledger.entry_type = 'consumption'
					AND ledger.created_at >= ?
			), 0) AS credit_consumed_hundredths`, owner, assistantID, since.UTC(), since.UTC(), owner, since.UTC()).Scan(&stats).Error; err != nil {
		return domain.PublicationStats{}, err
	}
	if err := r.db.WithContext(ctx).Table("ai_application_safety_audits").
		Where("owner_user_id = ? AND assistant_id = ? AND access_source = 'public' AND classification = 'refuse' AND created_at >= ?", owner, assistantID, since.UTC()).
		Count(&stats.SafetyRefusals).Error; err != nil {
		return domain.PublicationStats{}, err
	}
	return stats, nil
}
func (r *Repository) DeleteAssistant(ctx context.Context, owner, id string) error {
	result := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", owner, id).Delete(&assistantRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ListDigitalHumans(ctx context.Context, owner string) ([]domain.DigitalHuman, error) {
	var rows []humanRecord
	if err := r.db.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.DigitalHuman, 0, len(rows))
	for _, row := range rows {
		result = append(result, humanFromRecord(row))
	}
	return result, nil
}
func (r *Repository) GetDigitalHuman(ctx context.Context, owner, id string) (domain.DigitalHuman, error) {
	var row humanRecord
	err := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", owner, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.DigitalHuman{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.DigitalHuman{}, err
	}
	return humanFromRecord(row), nil
}
func (r *Repository) CreateDigitalHuman(ctx context.Context, owner string, human domain.DigitalHuman) (domain.DigitalHuman, error) {
	now := time.Now().UTC()
	human.ID = uuid.NewString()
	human.OwnerID = owner
	human.CreatedAt = now
	human.UpdatedAt = now
	human.Version = 1
	row := humanRecordFromDomain(human)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.DigitalHuman{}, mapDBError(err)
	}
	return human, nil
}
func (r *Repository) UpdateDigitalHuman(ctx context.Context, owner, id string, human domain.DigitalHuman, version int64) (domain.DigitalHuman, error) {
	state := human.State
	if state == "" {
		state = domain.StateEnabled
	}
	updates := map[string]any{"name": human.Name, "avatar_object_key": human.AvatarObjectKey, "voice": human.Voice, "language": human.Language, "expression_style": human.ExpressionStyle, "scene_description": human.SceneDescription, "state": state, "updated_at": time.Now().UTC(), "version": version + 1}
	result := r.db.WithContext(ctx).Model(&humanRecord{}).Where("owner_user_id = ? AND id = ? AND version = ?", owner, id, version).Updates(updates)
	if result.Error != nil {
		return domain.DigitalHuman{}, mapDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.DigitalHuman{}, domain.ErrVersionConflict
	}
	return r.GetDigitalHuman(ctx, owner, id)
}
func (r *Repository) DeleteDigitalHuman(ctx context.Context, owner, id string) error {
	result := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", owner, id).Delete(&humanRecord{})
	if result.Error != nil {
		return mapDBError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ListFAQs(ctx context.Context, owner, assistantID string) ([]domain.FAQ, error) {
	if _, err := r.GetAssistant(ctx, owner, assistantID); err != nil {
		return nil, err
	}
	var rows []faqRecord
	if err := r.db.WithContext(ctx).Where("assistant_id = ?", assistantID).Order("display_order, updated_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.FAQ, 0, len(rows))
	for _, row := range rows {
		result = append(result, faqFromRecord(row))
	}
	return result, nil
}
func (r *Repository) CreateFAQ(ctx context.Context, owner, assistantID string, faq domain.FAQ) (domain.FAQ, error) {
	if _, err := r.GetAssistant(ctx, owner, assistantID); err != nil {
		return domain.FAQ{}, err
	}
	now := time.Now().UTC()
	faq.ID = uuid.NewString()
	faq.AssistantID = assistantID
	faq.CreatedAt = now
	faq.UpdatedAt = now
	faq.Version = 1
	row := faqRecordFromDomain(faq)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.FAQ{}, mapDBError(err)
	}
	return faq, nil
}
func (r *Repository) UpdateFAQ(ctx context.Context, owner, assistantID, id string, faq domain.FAQ, version int64) (domain.FAQ, error) {
	if _, err := r.GetAssistant(ctx, owner, assistantID); err != nil {
		return domain.FAQ{}, err
	}
	updates := map[string]any{"question": faq.Question, "answer_markdown": faq.AnswerMarkdown, "display_order": faq.DisplayOrder, "category": faq.Category, "tag": faq.Tag, "icon": faq.Icon, "enabled": faq.Enabled, "updated_at": time.Now().UTC(), "version": version + 1}
	result := r.db.WithContext(ctx).Model(&faqRecord{}).Where("assistant_id = ? AND id = ? AND version = ?", assistantID, id, version).Updates(updates)
	if result.Error != nil {
		return domain.FAQ{}, mapDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.FAQ{}, domain.ErrVersionConflict
	}
	var row faqRecord
	if err := r.db.WithContext(ctx).Where("assistant_id = ? AND id = ?", assistantID, id).Take(&row).Error; err != nil {
		return domain.FAQ{}, err
	}
	return faqFromRecord(row), nil
}
func (r *Repository) DeleteFAQ(ctx context.Context, owner, assistantID, id string) error {
	if _, err := r.GetAssistant(ctx, owner, assistantID); err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Where("assistant_id = ? AND id = ?", assistantID, id).Delete(&faqRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func assistantFromRecord(row assistantRecord) domain.SmartAssistant {
	var ids []string
	_ = decode(row.KnowledgeBaseIDs, &ids)
	var share domain.ShareConfiguration
	_ = decode(row.Share, &share)
	if share.Width == "" {
		share.Width = "100%"
	}
	if share.Height == 0 {
		share.Height = 600
	}
	state := domain.ApplicationState(row.State)
	if state == "" {
		state = domain.StateEnabled
	}
	if row.ShareTokenHash != nil {
		share.TokenHash = *row.ShareTokenHash
	}
	share.TokenRevision = row.ShareTokenRevision
	return domain.SmartAssistant{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Icon: row.Icon, Description: row.Description, Introduction: row.Introduction, Scenario: row.Scenario, Prompt: row.Prompt, PreprocessPrompt: row.PreprocessPrompt, ProviderModelID: row.ProviderModelID, ServiceGoal: row.ServiceGoal, AnswerScope: row.AnswerScope, OperatingRules: row.OperatingRules, ResponseStyle: row.ResponseStyle, KnowledgeBaseIDs: ids, ExpertID: row.ExpertID, ExpertTeamID: row.ExpertTeamID, DigitalHumanID: row.DigitalHumanID, Share: share, State: state, LastValidatedAt: row.LastValidatedAt, ValidatedVersion: row.ValidatedVersion, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}
func assistantRecordFromDomain(value domain.SmartAssistant) assistantRecord {
	state := string(value.State)
	if state == "" {
		state = string(domain.StateEnabled)
	}
	share := value.Share
	share.Token = ""
	return assistantRecord{ID: value.ID, OwnerID: value.OwnerID, Name: value.Name, Icon: value.Icon, Description: value.Description, Introduction: value.Introduction, Scenario: value.Scenario, Prompt: value.Prompt, PreprocessPrompt: value.PreprocessPrompt, ProviderModelID: value.ProviderModelID, ServiceGoal: value.ServiceGoal, AnswerScope: value.AnswerScope, OperatingRules: value.OperatingRules, ResponseStyle: value.ResponseStyle, KnowledgeBaseIDs: encode(value.KnowledgeBaseIDs), ExpertID: value.ExpertID, ExpertTeamID: value.ExpertTeamID, DigitalHumanID: value.DigitalHumanID, Share: encode(share), ShareTokenHash: optionalString(value.Share.TokenHash), ShareTokenRevision: value.Share.TokenRevision, State: state, LastValidatedAt: value.LastValidatedAt, ValidatedVersion: value.ValidatedVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func humanFromRecord(row humanRecord) domain.DigitalHuman {
	state := domain.ApplicationState(row.State)
	if state == "" {
		state = domain.StateEnabled
	}
	return domain.DigitalHuman{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, AvatarObjectKey: row.AvatarObjectKey, Voice: row.Voice, Language: row.Language, ExpressionStyle: row.ExpressionStyle, SceneDescription: row.SceneDescription, State: state, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}
func humanRecordFromDomain(value domain.DigitalHuman) humanRecord {
	state := value.State
	if state == "" {
		state = domain.StateEnabled
	}
	return humanRecord{ID: value.ID, OwnerID: value.OwnerID, Name: value.Name, AvatarObjectKey: value.AvatarObjectKey, Voice: value.Voice, Language: value.Language, ExpressionStyle: value.ExpressionStyle, SceneDescription: value.SceneDescription, State: string(state), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}
func faqFromRecord(row faqRecord) domain.FAQ {
	return domain.FAQ{ID: row.ID, AssistantID: row.AssistantID, Question: row.Question, AnswerMarkdown: row.AnswerMarkdown, DisplayOrder: row.DisplayOrder, Category: row.Category, Tag: row.Tag, Icon: row.Icon, Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}
func faqRecordFromDomain(value domain.FAQ) faqRecord {
	return faqRecord{ID: value.ID, AssistantID: value.AssistantID, Question: value.Question, AnswerMarkdown: value.AnswerMarkdown, DisplayOrder: value.DisplayOrder, Category: value.Category, Tag: value.Tag, Icon: value.Icon, Enabled: value.Enabled, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}
func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) || errors.Is(err, gorm.ErrForeignKeyViolated) {
		return domain.ErrConflict
	}
	return fmt.Errorf("AI Application repository: %w", err)
}
