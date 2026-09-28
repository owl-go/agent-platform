package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type knowledgeBaseRecord struct {
	ID, OwnerID, Name, Description, Visibility string
	Platform                                   bool
	DeletedAt                                  *time.Time
	CreatedAt, UpdatedAt                       time.Time
	Version                                    int64
}

func (knowledgeBaseRecord) TableName() string { return "knowledge_bases" }

type knowledgeDocumentRecord struct {
	ID, KnowledgeBaseID, Name, SourceType, NormalizedSource, Content, ContentSHA256, State, FailureReason string
	DeletedAt                                                                                             *time.Time
	CreatedAt, UpdatedAt                                                                                  time.Time
	Version                                                                                               int64
}

func (knowledgeDocumentRecord) TableName() string { return "knowledge_documents" }

type knowledgeChunkRecord struct {
	ID, DocumentID, Content string
	GenerationID            *string `gorm:"column:generation_id"`
	Position                int
	Embedding               string `gorm:"column:embedding"`
	CreatedAt               time.Time
}

func (knowledgeChunkRecord) TableName() string { return "knowledge_chunks" }

type knowledgeSearchRecord struct {
	ID, DocumentID, Content string
	Position                int
	Score                   float32
}

type knowledgeIndexGenerationRecord struct {
	ID              string    `gorm:"column:id"`
	KnowledgeBaseID string    `gorm:"column:knowledge_base_id"`
	Generation      int64     `gorm:"column:generation"`
	State           string    `gorm:"column:state"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

type aiKnowledgeJobRecord struct {
	ID         string    `gorm:"column:id"`
	DocumentID string    `gorm:"column:document_id"`
	State      string    `gorm:"column:state"`
	Attempts   int       `gorm:"column:attempts"`
	Error      string    `gorm:"column:error"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (knowledgeIndexGenerationRecord) TableName() string { return "knowledge_index_generations" }
func (aiKnowledgeJobRecord) TableName() string           { return "ai_application_knowledge_jobs" }

func (r *Repository) ListKnowledgeBases(ctx context.Context, owner string) ([]domain.KnowledgeBase, error) {
	var rows []knowledgeBaseRecord
	if err := r.db.WithContext(ctx).Where("(owner_user_id = ? OR (platform = true AND visibility = 'public')) AND deleted_at IS NULL", owner).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.KnowledgeBase, 0, len(rows))
	for _, row := range rows {
		result = append(result, knowledgeBaseFromRecord(row))
	}
	return result, nil
}
func (r *Repository) GetKnowledgeBase(ctx context.Context, owner, id string) (domain.KnowledgeBase, error) {
	var row knowledgeBaseRecord
	err := r.db.WithContext(ctx).Where("(owner_user_id = ? OR (platform = true AND visibility = 'public')) AND id = ? AND deleted_at IS NULL", owner, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.KnowledgeBase{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.KnowledgeBase{}, err
	}
	return knowledgeBaseFromRecord(row), nil
}
func (r *Repository) CreateKnowledgeBase(ctx context.Context, owner string, base domain.KnowledgeBase) (domain.KnowledgeBase, error) {
	now := time.Now().UTC()
	base.ID, base.OwnerID, base.CreatedAt, base.UpdatedAt, base.Version = uuid.NewString(), owner, now, now, 1
	row := knowledgeBaseRecord{ID: base.ID, OwnerID: owner, Name: base.Name, Description: base.Description, Visibility: "private", CreatedAt: now, UpdatedAt: now, Version: 1}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.KnowledgeBase{}, mapDBError(err)
	}
	return base, nil
}
func (r *Repository) ListKnowledgeDocuments(ctx context.Context, owner, baseID string) ([]domain.KnowledgeDocument, error) {
	if _, err := r.GetKnowledgeBase(ctx, owner, baseID); err != nil {
		return nil, err
	}
	var rows []knowledgeDocumentRecord
	if err := r.db.WithContext(ctx).Where("knowledge_base_id = ? AND deleted_at IS NULL", baseID).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.KnowledgeDocument, 0, len(rows))
	for _, row := range rows {
		result = append(result, knowledgeDocumentFromRecord(row))
	}
	return result, nil
}
func (r *Repository) CreateKnowledgeDocument(ctx context.Context, owner, baseID string, document domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) (domain.KnowledgeDocument, error) {
	return r.createKnowledgeDocument(ctx, owner, baseID, document, chunks)
}

func (r *Repository) CreateKnowledgeDocumentWithEmbeddings(ctx context.Context, owner, baseID string, document domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) (domain.KnowledgeDocument, error) {
	return r.createKnowledgeDocument(ctx, owner, baseID, document, chunks)
}

func (r *Repository) EnqueueKnowledgeDocument(ctx context.Context, owner, baseID string, document domain.KnowledgeDocument) (domain.KnowledgeDocument, error) {
	if _, err := r.GetKnowledgeBase(ctx, owner, baseID); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	now := time.Now().UTC()
	document.ID, document.KnowledgeBaseID, document.CreatedAt, document.UpdatedAt, document.Version = uuid.NewString(), baseID, now, now, 1
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := knowledgeDocumentRecord{ID: document.ID, KnowledgeBaseID: baseID, Name: document.Name, SourceType: "upload", NormalizedSource: document.ContentSHA256, Content: document.Content, ContentSHA256: document.ContentSHA256, State: string(domain.KnowledgeProcessing), CreatedAt: now, UpdatedAt: now, Version: 1}
		if err := tx.Create(&row).Error; err != nil {
			return mapDBError(err)
		}
		return tx.Create(&aiKnowledgeJobRecord{ID: uuid.NewString(), DocumentID: document.ID, State: "queued", CreatedAt: now, UpdatedAt: now}).Error
	})
	if err != nil {
		return domain.KnowledgeDocument{}, err
	}
	document.State = domain.KnowledgeProcessing
	document.Content = ""
	return document, nil
}

func (r *Repository) createKnowledgeDocument(ctx context.Context, owner, baseID string, document domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) (domain.KnowledgeDocument, error) {
	if _, err := r.GetKnowledgeBase(ctx, owner, baseID); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	now := time.Now().UTC()
	document.ID, document.KnowledgeBaseID, document.CreatedAt, document.UpdatedAt, document.Version = uuid.NewString(), baseID, now, now, 1
	return document, r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := knowledgeDocumentRecord{ID: document.ID, KnowledgeBaseID: baseID, Name: document.Name, SourceType: "upload", NormalizedSource: document.ContentSHA256, Content: document.Content, ContentSHA256: document.ContentSHA256, State: string(document.State), FailureReason: document.FailureReason, CreatedAt: now, UpdatedAt: now, Version: 1}
		if err := tx.Create(&row).Error; err != nil {
			return mapDBError(err)
		}
		for _, chunk := range chunks {
			row := knowledgeChunkRecord{ID: uuid.NewString(), DocumentID: document.ID, Position: chunk.Position, Content: chunk.Text, CreatedAt: now}
			if len(chunk.Embedding) > 0 {
				row.Embedding = vectorLiteral(chunk.Embedding)
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) SearchKnowledgeVector(ctx context.Context, owner string, baseIDs []string, embedding []float32, limit int) ([]domain.KnowledgeChunk, error) {
	if len(baseIDs) == 0 || len(embedding) == 0 {
		return nil, nil
	}
	var rows []knowledgeSearchRecord
	err := r.db.WithContext(ctx).Table("knowledge_chunks AS c").Select("c.id, c.document_id, c.content, c.position, 1 - (c.embedding <=> ?::vector) AS score", vectorLiteral(embedding)).Joins("JOIN knowledge_documents AS d ON d.id = c.document_id AND d.state = 'ready' AND d.deleted_at IS NULL").Joins("JOIN knowledge_bases AS b ON b.id = d.knowledge_base_id AND b.owner_user_id = ? AND b.deleted_at IS NULL", owner).Joins("LEFT JOIN knowledge_index_generations AS g ON g.id = c.generation_id").Where("d.knowledge_base_id IN ? AND c.embedding IS NOT NULL AND (c.generation_id IS NULL OR (g.state = 'ready' AND g.generation = (SELECT MAX(g2.generation) FROM knowledge_index_generations g2 WHERE g2.knowledge_base_id = d.knowledge_base_id AND g2.state = 'ready')))", baseIDs).Order(gorm.Expr("c.embedding <=> ?::vector ASC, c.position ASC", vectorLiteral(embedding))).Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]domain.KnowledgeChunk, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.KnowledgeChunk{ID: row.ID, DocumentID: row.DocumentID, Position: row.Position, Text: row.Content, Score: row.Score})
	}
	return result, nil
}

func vectorLiteral(values []float32) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.FormatFloat(float64(value), 'f', -1, 32)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ","))
}
func (r *Repository) SearchKnowledge(ctx context.Context, owner string, baseIDs []string, query string, limit int) ([]domain.KnowledgeChunk, error) {
	if len(baseIDs) == 0 {
		return nil, nil
	}
	var rows []knowledgeSearchRecord
	err := r.db.WithContext(ctx).Table("knowledge_chunks AS c").Select("c.id, c.document_id, c.content, c.position, ts_rank(to_tsvector('simple', c.content), plainto_tsquery('simple', ?)) AS score", query).Joins("JOIN knowledge_documents AS d ON d.id = c.document_id AND d.state = 'ready' AND d.deleted_at IS NULL").Joins("JOIN knowledge_bases AS b ON b.id = d.knowledge_base_id AND b.owner_user_id = ? AND b.deleted_at IS NULL", owner).Joins("LEFT JOIN knowledge_index_generations AS g ON g.id = c.generation_id").Where("d.knowledge_base_id IN ? AND to_tsvector('simple', c.content) @@ plainto_tsquery('simple', ?) AND (c.generation_id IS NULL OR (g.state = 'ready' AND g.generation = (SELECT MAX(g2.generation) FROM knowledge_index_generations g2 WHERE g2.knowledge_base_id = d.knowledge_base_id AND g2.state = 'ready')))", baseIDs, query).Order("score DESC, c.position ASC").Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]domain.KnowledgeChunk, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.KnowledgeChunk{ID: row.ID, DocumentID: row.DocumentID, Position: row.Position, Text: row.Content, Score: row.Score})
	}
	return result, nil
}

func knowledgeBaseFromRecord(row knowledgeBaseRecord) domain.KnowledgeBase {
	state := domain.KnowledgeReady
	if row.DeletedAt != nil {
		state = domain.KnowledgeDisabled
	}
	return domain.KnowledgeBase{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Description: row.Description, State: state, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}
func knowledgeDocumentFromRecord(row knowledgeDocumentRecord) domain.KnowledgeDocument {
	return domain.KnowledgeDocument{ID: row.ID, KnowledgeBaseID: row.KnowledgeBaseID, Name: row.Name, Content: row.Content, ContentSHA256: row.ContentSHA256, State: domain.KnowledgeState(row.State), FailureReason: row.FailureReason, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}
