package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/aicreation/application"
	"agent-platform/backend/internal/biz/aicreation/domain"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	"agent-platform/backend/internal/secretcrypto"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type creditReservations interface {
	ReserveImageTx(*gorm.DB, creditsdomain.ImageReservation) (creditsdomain.ImageReservation, error)
	SettleImageTx(*gorm.DB, string, creditsdomain.Amount, time.Time) error
}

type Repository struct {
	db      *gorm.DB
	credits creditReservations
	box     *secretcrypto.Box
}

func New(db *gorm.DB, credits creditReservations, box *secretcrypto.Box) *Repository {
	return &Repository{db: db, credits: credits, box: box}
}

var _ application.Repository = (*Repository)(nil)

type modelRecord struct {
	ID                string     `gorm:"column:id;primaryKey"`
	ModelID           string     `gorm:"column:image_model_id"`
	PredecessorID     *string    `gorm:"column:predecessor_id"`
	DisplayName       string     `gorm:"column:display_name"`
	Endpoint          string     `gorm:"column:endpoint"`
	APIKeyCiphertext  []byte     `gorm:"column:api_key_ciphertext"`
	ProviderModelID   string     `gorm:"column:provider_model_id"`
	Protocol          string     `gorm:"column:api_protocol"`
	Modes             []byte     `gorm:"column:modes;type:jsonb"`
	Sizes             []byte     `gorm:"column:sizes;type:jsonb"`
	Qualities         []byte     `gorm:"column:qualities;type:jsonb"`
	Formats           []byte     `gorm:"column:formats;type:jsonb"`
	Backgrounds       []byte     `gorm:"column:backgrounds;type:jsonb"`
	DefaultSize       string     `gorm:"column:default_size"`
	DefaultQuality    string     `gorm:"column:default_quality"`
	DefaultFormat     string     `gorm:"column:default_format"`
	DefaultBackground string     `gorm:"column:default_background"`
	Rates             []byte     `gorm:"column:rates;type:jsonb"`
	State             string     `gorm:"column:state"`
	VerifiedAt        *time.Time `gorm:"column:verified_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
	Version           int64      `gorm:"column:version"`
}

func (modelRecord) TableName() string { return "image_model_revisions" }

type stableModelRecord struct {
	ID                string    `gorm:"column:id;primaryKey"`
	CurrentRevisionID *string   `gorm:"column:current_revision_id"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (stableModelRecord) TableName() string { return "image_models" }

func (repository *Repository) SaveModel(ctx context.Context, model domain.ImageModelRevision, replacementAPIKey []byte) (domain.ImageModelRevision, error) {
	row, err := fromModel(model)
	if err != nil {
		return model, err
	}
	if len(replacementAPIKey) > 0 {
		row.APIKeyCiphertext, err = repository.box.Encrypt(replacementAPIKey, "image-model:"+model.ID)
		if err != nil {
			return model, fmt.Errorf("encrypt Image Model API Key: %w", err)
		}
	} else {
		credentialSourceID := model.RevisionID
		if model.PredecessorID != "" {
			credentialSourceID = model.PredecessorID
		}
		var previous struct {
			APIKeyCiphertext []byte `gorm:"column:api_key_ciphertext"`
		}
		if err := repository.db.WithContext(ctx).Table("image_model_revisions").Select("api_key_ciphertext").Where("id = ?", credentialSourceID).Take(&previous).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return model, fmt.Errorf("read previous Image Model API Key: %w", err)
		}
		row.APIKeyCiphertext = previous.APIKeyCiphertext
	}
	if len(row.APIKeyCiphertext) == 0 {
		return model, fmt.Errorf("%w: Image Model API Key is required", domain.ErrInvalid)
	}
	err = repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		stable := stableModelRecord{ID: model.ID, CreatedAt: model.CreatedAt}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(&stable).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", model.ID).Take(&stable).Error; err != nil {
			return err
		}
		var existing modelRecord
		err := tx.Select("id", "version").Where("id = ?", row.ID).Take(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if model.PredecessorID != "" {
				var predecessor modelRecord
				if err := tx.Where("id = ? AND image_model_id = ? AND version = ? AND state <> ?", model.PredecessorID, model.ID, model.PredecessorVersion, domain.ModelDeleted).Take(&predecessor).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return domain.ErrVersionConflict
					}
					return err
				}
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			result := tx.Model(&modelRecord{}).Where("id = ? AND version = ?", row.ID, row.Version-1).Updates(map[string]any{"state": row.State, "verified_at": row.VerifiedAt, "updated_at": row.UpdatedAt, "version": row.Version})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrVersionConflict
			}
		}
		current := tx.Model(&stableModelRecord{}).Where("id = ?", model.ID)
		if model.PredecessorID != "" {
			current = current.Where("current_revision_id = ?", model.PredecessorID)
		} else {
			current = current.Where("current_revision_id IS NULL OR current_revision_id = ?", model.RevisionID)
		}
		result := current.Update("current_revision_id", model.RevisionID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrVersionConflict
		}
		return nil
	})
	return model, err
}

func (repository *Repository) DeleteModel(ctx context.Context, id string, expectedVersion int64, at time.Time) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stable stableModelRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&stable).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNotFound
			}
			return err
		}
		var active int64
		if err := tx.Model(&generationRecord{}).Where("image_model_id = ? AND state IN ? AND deleted_at IS NULL", id, []domain.State{domain.StatePending, domain.StateRunning}).Count(&active).Error; err != nil {
			return err
		}
		if active != 0 {
			return domain.ErrConflict
		}
		result := tx.Model(&modelRecord{}).
			Where("image_model_id = ? AND id = (SELECT current_revision_id FROM image_models WHERE id = ?) AND version = ?", id, id, expectedVersion).
			Updates(map[string]any{"state": domain.ModelDeleted, "updated_at": at, "version": expectedVersion + 1})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrVersionConflict
		}
		return nil
	})
}

func (repository *Repository) GetModel(ctx context.Context, id string) (domain.ImageModelRevision, error) {
	var row modelRecord
	err := repository.db.WithContext(ctx).Table("image_model_revisions revision").
		Joins("JOIN image_models model ON model.current_revision_id = revision.id").
		Where("model.id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ImageModelRevision{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ImageModelRevision{}, fmt.Errorf("get Image Model: %w", err)
	}
	return toModel(row)
}

func (repository *Repository) ListModels(ctx context.Context, availableOnly bool) ([]domain.ImageModelRevision, error) {
	query := repository.db.WithContext(ctx).Table("image_model_revisions revision").Joins("JOIN image_models model ON model.current_revision_id = revision.id")
	if availableOnly {
		query = query.Where("revision.state = ?", domain.ModelAvailable)
	}
	var rows []modelRecord
	if err := query.Order("revision.updated_at DESC, revision.id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Image Models: %w", err)
	}
	result := make([]domain.ImageModelRevision, 0, len(rows))
	for _, row := range rows {
		model, err := toModel(row)
		if err != nil {
			return nil, err
		}
		result = append(result, model)
	}
	return result, nil
}

type referenceUploadRecord struct {
	ID        string     `gorm:"column:id;primaryKey"`
	OwnerID   string     `gorm:"column:owner_user_id"`
	ObjectKey string     `gorm:"column:object_key"`
	SHA256    string     `gorm:"column:sha256"`
	MediaType string     `gorm:"column:media_type"`
	Size      int64      `gorm:"column:encoded_size"`
	Width     int        `gorm:"column:width"`
	Height    int        `gorm:"column:height"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	BoundAt   *time.Time `gorm:"column:bound_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
}

func (referenceUploadRecord) TableName() string { return "image_reference_uploads" }

func (repository *Repository) SaveReferenceUpload(ctx context.Context, upload domain.ReferenceUpload) (domain.ReferenceUpload, error) {
	row := referenceUploadRecord{ID: upload.ID, OwnerID: upload.OwnerID, ObjectKey: upload.Image.ObjectKey, SHA256: upload.Image.SHA256, MediaType: upload.Image.MediaType, Size: upload.Image.Size, Width: upload.Image.Width, Height: upload.Image.Height, ExpiresAt: upload.Image.ExpiresAt, BoundAt: upload.BoundAt, CreatedAt: upload.CreatedAt}
	return upload, repository.db.WithContext(ctx).Create(&row).Error
}

func (repository *Repository) GetReferenceUploads(ctx context.Context, ownerID string, ids []string) ([]domain.ReferenceUpload, error) {
	var rows []referenceUploadRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id IN ?", ownerID, ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.ReferenceUpload, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.ReferenceUpload{ID: row.ID, OwnerID: row.OwnerID, CreatedAt: row.CreatedAt, BoundAt: row.BoundAt, Image: domain.ReferenceImage{ObjectKey: row.ObjectKey, SHA256: row.SHA256, MediaType: row.MediaType, Size: row.Size, Width: row.Width, Height: row.Height, ExpiresAt: row.ExpiresAt}})
	}
	return result, nil
}

func (repository *Repository) DeleteReferenceUpload(ctx context.Context, ownerID, id string) (domain.ReferenceUpload, error) {
	var row referenceUploadRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND bound_at IS NULL", id, ownerID).Take(&row).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ReferenceUpload{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ReferenceUpload{}, err
	}
	return domain.ReferenceUpload{ID: row.ID, OwnerID: row.OwnerID, CreatedAt: row.CreatedAt, BoundAt: row.BoundAt, Image: domain.ReferenceImage{ObjectKey: row.ObjectKey, SHA256: row.SHA256, MediaType: row.MediaType, Size: row.Size, Width: row.Width, Height: row.Height, ExpiresAt: row.ExpiresAt}}, nil
}

type promptCandidateRecord struct {
	ID               bool      `gorm:"column:singleton;primaryKey"`
	ModelID          string    `gorm:"column:model_id"`
	Endpoint         string    `gorm:"column:endpoint"`
	APIKeyCiphertext []byte    `gorm:"column:api_key_ciphertext"`
	Instruction      string    `gorm:"column:instruction"`
	UpdatedBy        string    `gorm:"column:updated_by_user_id"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (promptCandidateRecord) TableName() string { return "prompt_optimization_settings" }

func (repository *Repository) ListPromptCandidates(ctx context.Context) ([]domain.PromptOptimizationCandidate, error) {
	var row promptCandidateRecord
	err := repository.db.WithContext(ctx).Take(&row, "singleton = ?", true).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []domain.PromptOptimizationCandidate{}, nil
	}
	if err != nil {
		return nil, err
	}
	return []domain.PromptOptimizationCandidate{{ProviderModelID: row.ModelID, DisplayName: row.ModelID, ProviderType: "openai", ModelID: row.ModelID, Protocol: "openai_responses", Endpoint: row.Endpoint, APIKeyConfigured: len(row.APIKeyCiphertext) > 0, Instruction: row.Instruction}}, nil
}

func (repository *Repository) ReplacePromptCandidates(ctx context.Context, administratorID string, candidates []domain.PromptOptimizationCandidate, replacementAPIKey []byte) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current promptCandidateRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Take(&current, "singleton = ?", true).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		ciphertext := current.APIKeyCiphertext
		if len(replacementAPIKey) > 0 {
			ciphertext, err = repository.box.Encrypt(replacementAPIKey, "prompt-optimization")
			if err != nil {
				return fmt.Errorf("encrypt Prompt Optimization API Key: %w", err)
			}
		}
		if len(ciphertext) == 0 {
			return fmt.Errorf("%w: Prompt Optimization API Key is required", domain.ErrInvalid)
		}
		now := time.Now().UTC()
		row := promptCandidateRecord{ID: true, ModelID: candidates[0].ModelID, Endpoint: candidates[0].Endpoint, APIKeyCiphertext: ciphertext, Instruction: candidates[0].Instruction, UpdatedBy: administratorID, CreatedAt: current.CreatedAt, UpdatedAt: now}
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		return tx.Save(&row).Error
	})
}

type generationRecord struct {
	ID                   string     `gorm:"column:id;primaryKey"`
	RequestID            string     `gorm:"column:request_id"`
	RequestFingerprint   string     `gorm:"column:request_fingerprint"`
	OwnerID              string     `gorm:"column:owner_user_id"`
	ModelID              string     `gorm:"column:image_model_id"`
	ModelRevisionID      string     `gorm:"column:image_model_revision_id"`
	ModelSnapshot        []byte     `gorm:"column:model_snapshot;type:jsonb"`
	OriginalPrompt       string     `gorm:"column:original_prompt"`
	SubmittedPrompt      string     `gorm:"column:submitted_prompt"`
	RequestSnapshot      []byte     `gorm:"column:request_snapshot;type:jsonb"`
	RequestedCount       int        `gorm:"column:requested_count"`
	ValidatedCount       int        `gorm:"column:validated_count"`
	Reservation          int64      `gorm:"column:reservation_hundredths"`
	Consumption          int64      `gorm:"column:consumption_hundredths"`
	ReservationCreditDay time.Time  `gorm:"column:reservation_credit_day;type:date"`
	State                string     `gorm:"column:state"`
	DispatchMarked       bool       `gorm:"column:dispatch_marked"`
	CancellationAsked    bool       `gorm:"column:cancellation_requested"`
	SafeError            *string    `gorm:"column:safe_error"`
	LeaseExpiresAt       *time.Time `gorm:"column:lease_expires_at"`
	CreatedAt            time.Time  `gorm:"column:created_at"`
	StartedAt            *time.Time `gorm:"column:started_at"`
	CompletedAt          *time.Time `gorm:"column:completed_at"`
	DeletedAt            *time.Time `gorm:"column:deleted_at"`
	Version              int64      `gorm:"column:version"`
}

func (generationRecord) TableName() string { return "image_generation_records" }

type generatedImageRecord struct {
	RecordID  string    `gorm:"column:record_id;primaryKey"`
	Position  int       `gorm:"column:position;primaryKey"`
	ObjectKey string    `gorm:"column:object_key"`
	SHA256    string    `gorm:"column:sha256"`
	MediaType string    `gorm:"column:media_type"`
	Size      int64     `gorm:"column:encoded_size"`
	Width     int       `gorm:"column:width"`
	Height    int       `gorm:"column:height"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

type generationEventRecord struct {
	RecordID  string    `gorm:"column:record_id;primaryKey"`
	Sequence  int64     `gorm:"column:sequence;primaryKey"`
	EventType string    `gorm:"column:event_type"`
	Payload   []byte    `gorm:"column:payload;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

type cleanupObjectRecord struct {
	ObjectKey string    `gorm:"column:object_key;primaryKey"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

func (cleanupObjectRecord) TableName() string { return "ai_creation_object_cleanup" }

func (generationEventRecord) TableName() string { return "image_generation_events" }

type referenceImageRecord struct {
	RecordID  string    `gorm:"column:record_id;primaryKey"`
	Position  int       `gorm:"column:position;primaryKey"`
	ObjectKey string    `gorm:"column:object_key"`
	SHA256    string    `gorm:"column:sha256"`
	MediaType string    `gorm:"column:media_type"`
	Size      int64     `gorm:"column:encoded_size"`
	Width     int       `gorm:"column:width"`
	Height    int       `gorm:"column:height"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

func (referenceImageRecord) TableName() string { return "image_generation_references" }

func (generatedImageRecord) TableName() string { return "generated_images" }

func (repository *Repository) SubmitRecord(ctx context.Context, record domain.ImageGenerationRecord, timezone string) (domain.ImageGenerationRecord, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return record, err
	}
	record.ReservationCreditDay = record.CreatedAt.In(location).Format(time.DateOnly)
	row, err := fromRecord(record)
	if err != nil {
		return record, err
	}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stable stableModelRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", record.Model.ID).Take(&stable).Error; err != nil {
			return err
		}
		if stable.CurrentRevisionID == nil || *stable.CurrentRevisionID != record.Model.RevisionID {
			return domain.ErrConflict
		}
		var revision modelRecord
		if err := tx.Where("id = ? AND image_model_id = ? AND state = ?", record.Model.RevisionID, record.Model.ID, domain.ModelAvailable).Take(&revision).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrConflict
			}
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := appendEventTx(tx, record, "record.pending", record.CreatedAt); err != nil {
			return err
		}
		boundAt := record.CreatedAt
		for _, image := range record.Request.References {
			stored := referenceImageRecord{RecordID: record.ID, Position: image.Position, ObjectKey: image.ObjectKey, SHA256: image.SHA256, MediaType: image.MediaType, Size: image.Size, Width: image.Width, Height: image.Height, ExpiresAt: record.CreatedAt.Add(90 * 24 * time.Hour)}
			if err := tx.Create(&stored).Error; err != nil {
				return err
			}
			if image.SourceID != "" {
				result := tx.Model(&referenceUploadRecord{}).Where("id = ? AND owner_user_id = ? AND bound_at IS NULL", image.SourceID, record.OwnerID).Update("bound_at", boundAt)
				if result.Error != nil || result.RowsAffected != 1 {
					return domain.ErrConflict
				}
			}
		}
		reservation, err := repository.credits.ReserveImageTx(tx, creditsdomain.ImageReservation{RecordID: record.ID, UserID: record.OwnerID, CreditDay: record.ReservationCreditDay, Timezone: timezone, Amount: creditsdomain.Amount(record.ReservationAmount), CreatedAt: record.CreatedAt})
		if err != nil {
			return err
		}
		record.ReservationCreditDay = reservation.CreditDay
		return nil
	}); err != nil {
		if errors.Is(err, creditsdomain.ErrInsufficientCredits) {
			return record, application.ErrInsufficientCredits
		}
		if isConflict(err) {
			var existing generationRecord
			if findErr := repository.db.WithContext(ctx).Where("owner_user_id = ? AND request_id = ? AND deleted_at IS NULL", record.OwnerID, record.RequestID).Take(&existing).Error; findErr == nil {
				if existing.RequestFingerprint != record.RequestFingerprint {
					return record, domain.ErrConflict
				}
				return repository.toRecord(ctx, existing)
			}
			return record, domain.ErrConflict
		}
		return record, fmt.Errorf("create Image Generation Record: %w", err)
	}
	return record, nil
}

func (repository *Repository) SettleRecord(ctx context.Context, record domain.ImageGenerationRecord) (domain.ImageGenerationRecord, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current generationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", record.ID, record.OwnerID).Take(&current).Error; err != nil {
			return err
		}
		if domain.State(current.State).Terminal() {
			return domain.ErrConflict
		}
		settledAt := time.Now().UTC()
		if record.CompletedAt != nil {
			settledAt = *record.CompletedAt
		}
		if err := repository.credits.SettleImageTx(tx, record.ID, creditsdomain.Amount(record.ConsumptionAmount), settledAt); err != nil {
			return err
		}
		if err := repository.updateRecordTx(tx, record); err != nil {
			return err
		}
		for _, image := range record.Images {
			if err := tx.Where("object_key = ?", image.ObjectKey).Delete(&cleanupObjectRecord{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return record, err
}

func (repository *Repository) GetRecord(ctx context.Context, ownerID, id string) (domain.ImageGenerationRecord, error) {
	var row generationRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, ownerID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ImageGenerationRecord{}, domain.ErrNotFound
		}
		return domain.ImageGenerationRecord{}, fmt.Errorf("get Image Generation Record: %w", err)
	}
	return repository.toRecord(ctx, row)
}

func (repository *Repository) GetRecordByRequestID(ctx context.Context, ownerID, requestID string) (domain.ImageGenerationRecord, error) {
	var row generationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND request_id = ? AND deleted_at IS NULL", ownerID, requestID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ImageGenerationRecord{}, domain.ErrNotFound
		}
		return domain.ImageGenerationRecord{}, err
	}
	return repository.toRecord(ctx, row)
}

func (repository *Repository) ListRecords(ctx context.Context, ownerID string, limit int) ([]domain.ImageGenerationRecord, error) {
	var rows []generationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND deleted_at IS NULL", ownerID).Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Image Generation Records: %w", err)
	}
	result := make([]domain.ImageGenerationRecord, 0, len(rows))
	for _, row := range rows {
		record, err := repository.toRecord(ctx, row)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

func (repository *Repository) ListEvents(ctx context.Context, ownerID, recordID string, after int64, limit int) ([]application.GenerationEvent, error) {
	var rows []generationEventRecord
	err := repository.db.WithContext(ctx).Table("image_generation_events event").
		Joins("JOIN image_generation_records record ON record.id = event.record_id").
		Where("event.record_id = ? AND record.owner_user_id = ? AND record.deleted_at IS NULL AND event.sequence > ?", recordID, ownerID, after).
		Order("event.sequence").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list Image Generation events: %w", err)
	}
	events := make([]application.GenerationEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, application.GenerationEvent{Sequence: row.Sequence, Type: row.EventType, CreatedAt: row.CreatedAt})
	}
	return events, nil
}

func (repository *Repository) DeleteRecord(ctx context.Context, ownerID, id string, at time.Time) (domain.ImageGenerationRecord, error) {
	var record domain.ImageGenerationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row generationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", id, ownerID).Take(&row).Error; err != nil {
			return err
		}
		if !domain.State(row.State).Terminal() {
			return domain.ErrConflict
		}
		var err error
		record, err = repository.toRecordTx(tx, row)
		if err != nil {
			return err
		}
		if err := tx.Model(&generationRecord{}).Where("id = ? AND owner_user_id = ?", id, ownerID).Updates(map[string]any{"deleted_at": at, "original_prompt": "", "submitted_prompt": "", "request_snapshot": []byte(`{}`), "safe_error": nil}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM image_generation_events WHERE record_id = ?", id).Error; err != nil {
			return err
		}
		source := "image:" + id
		return tx.Model(&ledgerRecord{}).Where("source = ?", source).Updates(map[string]any{"source": nil, "reason": "Image Generation"}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ImageGenerationRecord{}, domain.ErrNotFound
	}
	return record, err
}

type ledgerRecord struct {
	Source *string `gorm:"column:source"`
	Reason *string `gorm:"column:reason"`
}

func (ledgerRecord) TableName() string { return "credit_ledger" }

func (repository *Repository) FinalizeDeletedRecord(ctx context.Context, id string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return repository.finalizeDeletedRecordTx(tx, id) })
}

func (repository *Repository) ResolveExpiredDispatch(ctx context.Context, at time.Time) (bool, error) {
	resolved := false
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row generationRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("state = ? AND dispatch_marked AND lease_expires_at < ? AND deleted_at IS NULL", domain.StateRunning, at).
			Order("lease_expires_at, id").Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		record, err := repository.toRecordTx(tx, row)
		if err != nil {
			return err
		}
		if err := record.Unknown(at); err != nil {
			return err
		}
		if err := repository.credits.SettleImageTx(tx, record.ID, 0, at); err != nil {
			return err
		}
		if err := repository.updateRecordTx(tx, record); err != nil {
			return err
		}
		resolved = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("resolve abandoned Image Generation Record: %w", err)
	}
	return resolved, nil
}

func (repository *Repository) ListExpiredObjects(ctx context.Context, at time.Time, limit int) ([]application.ExpiredObject, error) {
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	var rows []struct {
		Kind     string
		RecordID string
		UploadID string
		Position int
		Key      string
	}
	err := repository.db.WithContext(ctx).Raw(`
SELECT kind, record_id, upload_id, position, object_key AS key FROM (
  SELECT 'upload' AS kind, '' AS record_id, id::text AS upload_id, 0 AS position, object_key, expires_at
    FROM image_reference_uploads upload WHERE expires_at <= ?
      AND NOT EXISTS (SELECT 1 FROM image_generation_references reference WHERE reference.object_key = upload.object_key)
  UNION ALL
  SELECT 'reference', reference.record_id::text, '' AS upload_id, reference.position, reference.object_key, reference.expires_at
    FROM image_generation_references reference JOIN image_generation_records record ON record.id = reference.record_id
    WHERE reference.expires_at <= ? OR record.deleted_at IS NOT NULL
  UNION ALL
  SELECT 'output', output.record_id::text, '' AS upload_id, output.position, output.object_key, output.expires_at
    FROM generated_images output JOIN image_generation_records record ON record.id = output.record_id
    WHERE output.expires_at <= ? OR record.deleted_at IS NOT NULL
  UNION ALL
  SELECT 'orphan', '', '', 0, object_key, expires_at
    FROM ai_creation_object_cleanup WHERE expires_at <= ?
) expired ORDER BY expires_at, object_key LIMIT ?`, at, at, at, at, limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list expired AI Creation objects: %w", err)
	}
	items := make([]application.ExpiredObject, 0, len(rows))
	for _, row := range rows {
		items = append(items, application.ExpiredObject{Kind: row.Kind, RecordID: row.RecordID, UploadID: row.UploadID, Position: row.Position, Key: row.Key})
	}
	return items, nil
}

func (repository *Repository) ForgetExpiredObject(ctx context.Context, item application.ExpiredObject, at time.Time) error {
	query := repository.db.WithContext(ctx)
	switch item.Kind {
	case "upload":
		return query.Where("id = ? AND object_key = ? AND expires_at <= ?", item.UploadID, item.Key, at).Delete(&referenceUploadRecord{}).Error
	case "reference":
		return query.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("record_id = ? AND position = ? AND object_key = ? AND (expires_at <= ? OR EXISTS (SELECT 1 FROM image_generation_records WHERE id = ? AND deleted_at IS NOT NULL))", item.RecordID, item.Position, item.Key, at, item.RecordID).Delete(&referenceImageRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("object_key = ?", item.Key).Delete(&referenceUploadRecord{}).Error; err != nil {
				return err
			}
			return repository.finalizeDeletedRecordTx(tx, item.RecordID)
		})
	case "output":
		return query.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("record_id = ? AND position = ? AND object_key = ? AND (expires_at <= ? OR EXISTS (SELECT 1 FROM image_generation_records WHERE id = ? AND deleted_at IS NOT NULL))", item.RecordID, item.Position, item.Key, at, item.RecordID).Delete(&generatedImageRecord{}).Error; err != nil {
				return err
			}
			return repository.finalizeDeletedRecordTx(tx, item.RecordID)
		})
	case "orphan":
		return query.Where("object_key = ?", item.Key).Delete(&cleanupObjectRecord{}).Error
	default:
		return fmt.Errorf("unknown expired AI Creation object kind %q", item.Kind)
	}
}

func (repository *Repository) TrackObject(ctx context.Context, key string, expiresAt time.Time) error {
	return repository.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&cleanupObjectRecord{ObjectKey: key, ExpiresAt: expiresAt}).Error
}

func (repository *Repository) UntrackObject(ctx context.Context, key string) error {
	return repository.db.WithContext(ctx).Where("object_key = ?", key).Delete(&cleanupObjectRecord{}).Error
}

func (repository *Repository) finalizeDeletedRecordTx(tx *gorm.DB, id string) error {
	var row generationRecord
	if err := tx.Select("id", "deleted_at").Where("id = ? AND deleted_at IS NOT NULL", id).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	var remaining int64
	if err := tx.Table("image_generation_references").Where("record_id = ?", id).Count(&remaining).Error; err != nil || remaining != 0 {
		return err
	}
	if err := tx.Table("generated_images").Where("record_id = ?", id).Count(&remaining).Error; err != nil || remaining != 0 {
		return err
	}
	if err := tx.Exec("DELETE FROM image_credit_reservations WHERE record_id = ?", id).Error; err != nil {
		return err
	}
	return tx.Exec("DELETE FROM image_generation_records WHERE id = ?", id).Error
}

func (repository *Repository) ClaimNext(ctx context.Context, at time.Time) (domain.ImageGenerationRecord, bool, error) {
	var record domain.ImageGenerationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row generationRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("deleted_at IS NULL AND (state = ? OR (state = ? AND NOT dispatch_marked AND lease_expires_at < ?))", domain.StatePending, domain.StateRunning, at).
			Order("created_at, id").Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var convertErr error
		record, convertErr = repository.toRecordTx(tx, row)
		if convertErr != nil {
			return convertErr
		}
		if record.State == domain.StateRunning {
			record.State = domain.StatePending
			record.StartedAt = nil
		}
		if err := record.Start(at); err != nil {
			return err
		}
		lease := at.Add(5 * time.Minute)
		result := tx.Model(&generationRecord{}).Where("id = ? AND state = ?", row.ID, row.State).Updates(map[string]any{"state": record.State, "started_at": record.StartedAt, "lease_expires_at": lease, "version": record.Version})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return appendEventTx(tx, record, "record.running", at)
	})
	if err != nil {
		return domain.ImageGenerationRecord{}, false, fmt.Errorf("claim Image Generation Record: %w", err)
	}
	return record, record.ID != "", nil
}

func (repository *Repository) UpdateRecord(ctx context.Context, record domain.ImageGenerationRecord) (domain.ImageGenerationRecord, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return repository.updateRecordTx(tx, record) })
	return record, err
}

func (repository *Repository) updateRecordTx(tx *gorm.DB, record domain.ImageGenerationRecord) error {
	row, err := fromRecord(record)
	if err != nil {
		return err
	}
	return func() error {
		updates := map[string]any{
			"validated_count": row.ValidatedCount, "consumption_hundredths": row.Consumption, "state": row.State,
			"dispatch_marked": row.DispatchMarked, "cancellation_requested": row.CancellationAsked,
			"safe_error": row.SafeError, "started_at": row.StartedAt, "completed_at": row.CompletedAt, "version": row.Version,
		}
		result := tx.Model(&generationRecord{}).
			Where("id = ? AND owner_user_id = ? AND version = ? AND state IN ?", row.ID, row.OwnerID, row.Version-1, []domain.State{domain.StatePending, domain.StateRunning}).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		for _, image := range record.Images {
			stored := generatedImageRecord{RecordID: record.ID, Position: image.Position, ObjectKey: image.ObjectKey, SHA256: image.SHA256, MediaType: image.MediaType, Size: image.Size, Width: image.Width, Height: image.Height, ExpiresAt: image.ExpiresAt}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&stored).Error; err != nil {
				return err
			}
		}
		eventType := "record." + string(record.State)
		if record.State == domain.StateRunning && record.DispatchMarked {
			eventType = "record.dispatched"
		}
		at := time.Now().UTC()
		if record.CompletedAt != nil {
			at = *record.CompletedAt
		}
		return appendEventTx(tx, record, eventType, at)
	}()
}

func appendEventTx(tx *gorm.DB, record domain.ImageGenerationRecord, eventType string, at time.Time) error {
	event := generationEventRecord{RecordID: record.ID, Sequence: record.Version, EventType: eventType, Payload: []byte(`{}`), CreatedAt: at}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error
}

func (repository *Repository) toRecord(ctx context.Context, row generationRecord) (domain.ImageGenerationRecord, error) {
	return repository.toRecordTx(repository.db.WithContext(ctx), row)
}

func (repository *Repository) toRecordTx(tx *gorm.DB, row generationRecord) (domain.ImageGenerationRecord, error) {
	var model domain.ImageModelRevision
	var request domain.GenerationRequest
	if err := json.Unmarshal(row.ModelSnapshot, &model); err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	if err := json.Unmarshal(row.RequestSnapshot, &request); err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	var imageRows []generatedImageRecord
	if err := tx.Where("record_id = ?", row.ID).Order("position").Find(&imageRows).Error; err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	images := make([]domain.GeneratedImage, 0, len(imageRows))
	for _, image := range imageRows {
		images = append(images, domain.GeneratedImage{Position: image.Position, ObjectKey: image.ObjectKey, SHA256: image.SHA256, MediaType: image.MediaType, Size: image.Size, Width: image.Width, Height: image.Height, ExpiresAt: image.ExpiresAt})
	}
	safeError := ""
	if row.SafeError != nil {
		safeError = *row.SafeError
	}
	return domain.ImageGenerationRecord{
		ID: row.ID, RequestID: row.RequestID, RequestFingerprint: row.RequestFingerprint, OwnerID: row.OwnerID, Model: model, OriginalPrompt: row.OriginalPrompt, Prompt: row.SubmittedPrompt,
		Request: request, RequestedCount: row.RequestedCount, ValidatedCount: row.ValidatedCount,
		ReservationAmount: row.Reservation, ConsumptionAmount: row.Consumption,
		ReservationCreditDay: row.ReservationCreditDay.Format(time.DateOnly), State: domain.State(row.State),
		DispatchMarked: row.DispatchMarked, CancellationAsked: row.CancellationAsked, SafeError: safeError,
		Images: images, CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, Version: row.Version,
	}, nil
}

func fromModel(model domain.ImageModelRevision) (modelRecord, error) {
	encode := func(value any) ([]byte, error) { return json.Marshal(value) }
	modes, err := encode(model.Modes)
	if err != nil {
		return modelRecord{}, err
	}
	sizes, err := encode(model.Sizes)
	if err != nil {
		return modelRecord{}, err
	}
	qualities, err := encode(model.Qualities)
	if err != nil {
		return modelRecord{}, err
	}
	formats, err := encode(model.Formats)
	if err != nil {
		return modelRecord{}, err
	}
	backgrounds, err := encode(model.Backgrounds)
	if err != nil {
		return modelRecord{}, err
	}
	rates, err := encode(model.Rates)
	if err != nil {
		return modelRecord{}, err
	}
	var verifiedAt *time.Time
	if !model.VerifiedAt.IsZero() {
		verifiedAt = &model.VerifiedAt
	}
	var predecessor *string
	if model.PredecessorID != "" {
		predecessor = &model.PredecessorID
	}
	return modelRecord{ID: model.RevisionID, ModelID: model.ID, PredecessorID: predecessor, DisplayName: model.DisplayName, Endpoint: model.Endpoint, ProviderModelID: model.ModelID, Protocol: model.Protocol, Modes: modes, Sizes: sizes, Qualities: qualities, Formats: formats, Backgrounds: backgrounds, DefaultSize: model.DefaultSize, DefaultQuality: model.DefaultQuality, DefaultFormat: model.DefaultFormat, DefaultBackground: model.DefaultBackground, Rates: rates, State: string(model.State), VerifiedAt: verifiedAt, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt, Version: model.Version}, nil
}

func toModel(row modelRecord) (domain.ImageModelRevision, error) {
	model := domain.ImageModelRevision{ID: row.ModelID, RevisionID: row.ID, DisplayName: row.DisplayName, Endpoint: row.Endpoint, APIKeyConfigured: len(row.APIKeyCiphertext) > 0, ModelID: row.ProviderModelID, Protocol: row.Protocol, DefaultSize: row.DefaultSize, DefaultQuality: row.DefaultQuality, DefaultFormat: row.DefaultFormat, DefaultBackground: row.DefaultBackground, State: domain.ModelState(row.State), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
	if row.PredecessorID != nil {
		model.PredecessorID = *row.PredecessorID
	}
	if row.VerifiedAt != nil {
		model.VerifiedAt = *row.VerifiedAt
	}
	for _, pair := range []struct {
		data   []byte
		target any
	}{
		{row.Modes, &model.Modes}, {row.Sizes, &model.Sizes}, {row.Qualities, &model.Qualities},
		{row.Formats, &model.Formats}, {row.Backgrounds, &model.Backgrounds}, {row.Rates, &model.Rates},
	} {
		if err := json.Unmarshal(pair.data, pair.target); err != nil {
			return model, err
		}
	}
	return model, nil
}

func fromRecord(record domain.ImageGenerationRecord) (generationRecord, error) {
	model, err := json.Marshal(record.Model)
	if err != nil {
		return generationRecord{}, err
	}
	request, err := json.Marshal(record.Request)
	if err != nil {
		return generationRecord{}, err
	}
	day, err := time.Parse(time.DateOnly, record.ReservationCreditDay)
	if err != nil {
		return generationRecord{}, err
	}
	var safeError *string
	if record.SafeError != "" {
		safeError = &record.SafeError
	}
	return generationRecord{ID: record.ID, RequestID: record.RequestID, RequestFingerprint: record.RequestFingerprint, OwnerID: record.OwnerID, ModelID: record.Model.ID, ModelRevisionID: record.Model.RevisionID, ModelSnapshot: model, OriginalPrompt: record.OriginalPrompt, SubmittedPrompt: record.Prompt, RequestSnapshot: request, RequestedCount: record.RequestedCount, ValidatedCount: record.ValidatedCount, Reservation: record.ReservationAmount, Consumption: record.ConsumptionAmount, ReservationCreditDay: day, State: string(record.State), DispatchMarked: record.DispatchMarked, CancellationAsked: record.CancellationAsked, SafeError: safeError, CreatedAt: record.CreatedAt, StartedAt: record.StartedAt, CompletedAt: record.CompletedAt, Version: record.Version}, nil
}

func isConflict(err error) bool {
	return err != nil && (errors.Is(err, gorm.ErrDuplicatedKey) || stringsContains(err.Error(), "duplicate key"))
}

func stringsContains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}
