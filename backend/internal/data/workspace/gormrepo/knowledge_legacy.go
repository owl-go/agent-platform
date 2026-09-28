package gormrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var _ workspaceapplication.LegacyKnowledgeRepository = (*Repository)(nil)

func (repository *Repository) ClaimLegacyKnowledgeDocument(ctx context.Context) (*workspaceapplication.LegacyKnowledgeDocument, error) {
	var claimed *workspaceapplication.LegacyKnowledgeDocument
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			JobID           string `gorm:"column:job_id"`
			DocumentID      string `gorm:"column:document_id"`
			KnowledgeBaseID string `gorm:"column:knowledge_base_id"`
			OwnerID         string `gorm:"column:owner_id"`
			Name            string `gorm:"column:name"`
			Content         string `gorm:"column:content"`
		}
		query := `SELECT job.id AS job_id, doc.id AS document_id, doc.knowledge_base_id,
			base.owner_user_id AS owner_id, doc.name, doc.content
			FROM ai_application_knowledge_jobs AS job
			JOIN knowledge_documents AS doc ON doc.id = job.document_id
			JOIN knowledge_bases AS base ON base.id = doc.knowledge_base_id
			WHERE (job.state = 'queued' OR (job.state = 'running' AND (job.lease_expires_at IS NULL OR job.lease_expires_at < now())))
			AND doc.content <> '' AND doc.deleted_at IS NULL AND base.deleted_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM knowledge_document_revisions AS revision WHERE revision.document_id = doc.id)
			ORDER BY job.created_at, job.id FOR UPDATE OF job SKIP LOCKED LIMIT 1`
		if err := tx.Raw(query).Scan(&row).Error; err != nil {
			return err
		}
		if row.JobID == "" {
			return nil
		}
		if err := tx.Table("ai_application_knowledge_jobs").Where("id = ?", row.JobID).Updates(map[string]any{"state": "running", "attempts": gorm.Expr("attempts + 1"), "lease_expires_at": time.Now().UTC().Add(10 * time.Minute), "updated_at": gorm.Expr("now()")}).Error; err != nil {
			return err
		}
		claimed = &workspaceapplication.LegacyKnowledgeDocument{JobID: row.JobID, DocumentID: row.DocumentID, KnowledgeBaseID: row.KnowledgeBaseID, OwnerID: row.OwnerID, Name: row.Name, Content: row.Content}
		return nil
	})
	return claimed, err
}

func (repository *Repository) CompleteLegacyKnowledgeDocument(ctx context.Context, document workspaceapplication.LegacyKnowledgeDocument, source workspaceapplication.KnowledgeSourceSnapshot) error {
	if document.DocumentID == "" || source.ObjectKey == "" || source.Size <= 0 || !knowledgeSHA256Pattern.MatchString(source.SHA256) || source.ContentType != "text/plain" {
		return fmt.Errorf("%w: invalid legacy Knowledge source", domain.ErrInvalid)
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			State string
		}
		if err := tx.Table("ai_application_knowledge_jobs").Select("state").Where("id = ? AND document_id = ?", document.JobID, document.DocumentID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State != "running" {
			return fmt.Errorf("%w: legacy Knowledge job is not running", domain.ErrConflict)
		}
		var existing int64
		if err := tx.Table("knowledge_document_revisions").Where("document_id = ?", document.DocumentID).Count(&existing).Error; err != nil {
			return err
		}
		if existing == 0 {
			revision := knowledgeRevisionRecord{ID: uuid.NewString(), DocumentID: document.DocumentID, Revision: 1, ObjectKey: source.ObjectKey, SHA256: source.SHA256, Size: source.Size, ContentType: source.ContentType, State: string(domain.KnowledgeAccepted)}
			if err := tx.Create(&revision).Error; err != nil {
				return err
			}
			if err := tx.Table("knowledge_ingestion_jobs").Create(map[string]any{"id": uuid.NewString(), "revision_id": revision.ID, "idempotency_key": "knowledge-revision:" + revision.ID, "state": "queued"}).Error; err != nil {
				return err
			}
			if err := tx.Table("knowledge_documents").Where("id = ? AND deleted_at IS NULL", document.DocumentID).Updates(map[string]any{"state": string(domain.KnowledgeAccepted), "error": "", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		return tx.Table("ai_application_knowledge_jobs").Where("id = ?", document.JobID).Updates(map[string]any{"state": "succeeded", "error": "", "lease_expires_at": nil, "updated_at": gorm.Expr("now()")}).Error
	})
}

func (repository *Repository) FailLegacyKnowledgeDocument(ctx context.Context, document workspaceapplication.LegacyKnowledgeDocument, processingErr error) error {
	errorText := strings.TrimSpace(processingErr.Error())
	if len(errorText) > 500 {
		errorText = errorText[:500]
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("knowledge_documents").Where("id = ? AND deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM knowledge_document_revisions WHERE document_id = ?)", document.DocumentID, document.DocumentID).Updates(map[string]any{"state": string(domain.KnowledgeFailed), "error": errorText, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		return tx.Table("ai_application_knowledge_jobs").Where("id = ?", document.JobID).Updates(map[string]any{"state": "failed", "error": errorText, "lease_expires_at": nil, "updated_at": gorm.Expr("now()")}).Error
	})
}
