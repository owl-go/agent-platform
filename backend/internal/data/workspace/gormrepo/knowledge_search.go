package gormrepo

import (
	"context"
	"errors"

	"agent-platform/backend/internal/biz/workspace/domain"
	"gorm.io/gorm"
)

func (repository *Repository) ReadyKnowledgeSearchGeneration(ctx context.Context, ownerID, baseID string, administrator bool) (int64, error) {
	if _, err := repository.GetKnowledgeBase(ctx, ownerID, baseID, administrator, false); err != nil {
		return 0, err
	}
	var generation int64
	err := repository.db.WithContext(ctx).Table("knowledge_index_generations").
		Where("knowledge_base_id = ? AND state = 'ready'", baseID).
		Select("COALESCE(MAX(generation), 0)").Scan(&generation).Error
	return generation, err
}

func (repository *Repository) ResolveKnowledgeSearchSource(ctx context.Context, ownerID, baseID, revisionID string, administrator bool) (domain.KnowledgeSearchSource, error) {
	return repository.resolveKnowledgeSource(ctx, ownerID, baseID, revisionID, administrator, 0)
}

func (repository *Repository) resolveKnowledgeSource(ctx context.Context, ownerID, baseID, revisionID string, administrator bool, generation int64) (domain.KnowledgeSearchSource, error) {
	var row struct {
		DocumentID   string `gorm:"column:document_id"`
		RevisionID   string `gorm:"column:revision_id"`
		DocumentName string `gorm:"column:document_name"`
		CategoryName string `gorm:"column:category_name"`
	}
	query := repository.db.WithContext(ctx).Table("knowledge_document_revisions AS revision").
		Select("document.id AS document_id, revision.id AS revision_id, document.name AS document_name, COALESCE(category.name, '') AS category_name").
		Joins("JOIN knowledge_documents AS document ON document.id = revision.document_id AND document.deleted_at IS NULL").
		Joins("JOIN knowledge_bases AS knowledge_bases ON knowledge_bases.id = document.knowledge_base_id AND knowledge_bases.deleted_at IS NULL").
		Joins("LEFT JOIN knowledge_categories AS category ON category.id = document.category_id").
		Where("knowledge_bases.id = ? AND revision.id = ? AND revision.state = 'ready'", baseID, revisionID).
		Where("(document.category_id IS NULL OR category.deleted_at IS NULL)")
	if generation == 0 {
		query = query.Where("revision.revision = (SELECT MAX(candidate.revision) FROM knowledge_document_revisions AS candidate WHERE candidate.document_id = document.id AND candidate.state = 'ready')")
	} else {
		query = query.Where("EXISTS (SELECT 1 FROM knowledge_generation_revisions membership JOIN knowledge_index_generations generation ON generation.id = membership.generation_id WHERE membership.revision_id = revision.id AND generation.knowledge_base_id = ? AND generation.generation = ? AND generation.state = 'ready')", baseID, generation)
	}
	query = knowledgeBaseAccess(query, ownerID, false)
	if !administrator {
		query = query.Where("knowledge_bases.platform = false OR knowledge_bases.visibility = 'public'")
	}
	if err := query.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.KnowledgeSearchSource{}, domain.ErrNotFound
		}
		return domain.KnowledgeSearchSource{}, err
	}
	return domain.KnowledgeSearchSource{DocumentID: row.DocumentID, RevisionID: row.RevisionID, DocumentName: row.DocumentName, CategoryName: row.CategoryName}, nil
}
