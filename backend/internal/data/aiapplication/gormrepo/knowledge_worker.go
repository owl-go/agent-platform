package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProcessNextKnowledgeDocument claims one AI Application ingestion job. The
// generation is built beside the currently active generation and becomes
// visible only after every copied and newly generated chunk has been written.
func (r *Repository) ProcessNextKnowledgeDocument(ctx context.Context) (bool, error) {
	var job aiKnowledgeJobRecord
	claimed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Where("state = ?", "queued").Order("created_at, id").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).First(&job)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if query.Error != nil {
			return query.Error
		}
		if err := tx.Model(&aiKnowledgeJobRecord{}).Where("id = ?", job.ID).Updates(map[string]any{"state": "running", "attempts": gorm.Expr("attempts + 1"), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		return claimed, err
	}

	var document knowledgeDocumentRecord
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", job.DocumentID).Take(&document).Error; err != nil {
		return true, r.failKnowledgeDocument(ctx, job, err)
	}
	chunks := aiChunkDocument(document.Content)
	if configuration, configurationErr := r.GetEmbeddingConfiguration(ctx); configurationErr == nil && configuration.Enabled {
		vectors, embedErr := r.Embed(ctx, chunkTextsForRecords(chunks))
		if embedErr != nil {
			return true, r.failKnowledgeDocument(ctx, job, embedErr)
		}
		if len(vectors) != len(chunks) {
			return true, r.failKnowledgeDocument(ctx, job, fmt.Errorf("embedding count mismatch: got %d, want %d", len(vectors), len(chunks)))
		}
		for index := range chunks {
			chunks[index].Embedding = vectorLiteral(vectors[index])
		}
	}

	if err := r.activateKnowledgeGeneration(ctx, document, job, chunks); err != nil {
		return true, r.failKnowledgeDocument(ctx, job, err)
	}
	return true, nil
}

func (r *Repository) activateKnowledgeGeneration(ctx context.Context, document knowledgeDocumentRecord, job aiKnowledgeJobRecord, chunks []knowledgeChunkRecord) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var latest int64
		if err := tx.Table("knowledge_index_generations").Where("knowledge_base_id = ?", document.KnowledgeBaseID).Select("COALESCE(MAX(generation), 0)").Scan(&latest).Error; err != nil {
			return err
		}
		generation := knowledgeIndexGenerationRecord{ID: uuid.NewString(), KnowledgeBaseID: document.KnowledgeBaseID, Generation: latest + 1, State: "building", CreatedAt: time.Now().UTC()}
		if err := tx.Create(&generation).Error; err != nil {
			return err
		}
		var previous []knowledgeChunkRecord
		if err := tx.Table("knowledge_chunks AS c").Joins("JOIN knowledge_documents AS d ON d.id = c.document_id").Where("d.knowledge_base_id = ? AND d.deleted_at IS NULL AND c.document_id <> ?", document.KnowledgeBaseID, document.ID).Find(&previous).Error; err != nil {
			return err
		}
		for _, previousChunk := range previous {
			previousChunk.ID = uuid.NewString()
			previousChunk.GenerationID = &generation.ID
			if err := tx.Create(&previousChunk).Error; err != nil {
				return err
			}
		}
		for _, chunk := range chunks {
			chunk.ID = uuid.NewString()
			chunk.DocumentID = document.ID
			chunk.GenerationID = &generation.ID
			chunk.CreatedAt = time.Now().UTC()
			if err := tx.Create(&chunk).Error; err != nil {
				return err
			}
		}
		if err := tx.Table("knowledge_documents").Where("id = ?", document.ID).Updates(map[string]any{"state": string(domain.KnowledgeReady), "error": "", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		if err := tx.Table("knowledge_index_generations").Where("id = ?", generation.ID).Update("state", "ready").Error; err != nil {
			return err
		}
		return tx.Table("ai_application_knowledge_jobs").Where("id = ?", job.ID).Updates(map[string]any{"state": "succeeded", "error": "", "updated_at": gorm.Expr("now()")}).Error
	})
}

func (r *Repository) failKnowledgeDocument(ctx context.Context, job aiKnowledgeJobRecord, processingErr error) error {
	errorText := strings.TrimSpace(processingErr.Error())
	if len(errorText) > 500 {
		errorText = errorText[:500]
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("knowledge_documents").Where("id = ?", job.DocumentID).Updates(map[string]any{"state": string(domain.KnowledgeFailed), "error": errorText, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		return tx.Table("ai_application_knowledge_jobs").Where("id = ?", job.ID).Updates(map[string]any{"state": "failed", "error": errorText, "updated_at": gorm.Expr("now()")}).Error
	})
}

func aiChunkDocument(content string) []knowledgeChunkRecord {
	runes := []rune(content)
	const size = 1800
	chunks := make([]knowledgeChunkRecord, 0, (len(runes)+size-1)/size)
	for position, start := 0, 0; start < len(runes); position, start = position+1, start+size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, knowledgeChunkRecord{Position: position, Content: string(runes[start:end])})
	}
	return chunks
}

func chunkTextsForRecords(chunks []knowledgeChunkRecord) []string {
	texts := make([]string, len(chunks))
	for index := range chunks {
		texts[index] = chunks[index].Content
	}
	return texts
}
