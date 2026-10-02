package gormrepo

import (
	"context"
	"errors"
	"fmt"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/ragflow"
	"agent-platform/backend/internal/objectstore"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type knowledgeProviderRevision struct {
	DeploymentID    string
	KnowledgeBaseID string
	RevisionID      string
	DatasetID       string
	DocumentID      string
}

func (knowledgeProviderRevision) TableName() string { return "knowledge_provider_revisions" }

func (r *Repository) GetMapping(ctx context.Context, deployment, revision string) (ragflow.Mapping, error) {
	var row knowledgeProviderRevision
	err := r.db.WithContext(ctx).Where("deployment_id = ? AND revision_id = ?", deployment, revision).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ragflow.Mapping{}, ragflow.ErrMappingNotFound
	}
	return ragflow.Mapping{BaseID: row.KnowledgeBaseID, RevisionID: row.RevisionID, DatasetID: row.DatasetID, DocumentID: row.DocumentID}, err
}
func (r *Repository) SaveMapping(ctx context.Context, deployment string, m ragflow.Mapping) error {
	row := knowledgeProviderRevision{DeploymentID: deployment, KnowledgeBaseID: m.BaseID, RevisionID: m.RevisionID, DatasetID: m.DatasetID, DocumentID: m.DocumentID}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "deployment_id"}, {Name: "revision_id"}}, DoNothing: true}).Create(&row).Error
}
func (r *Repository) DeleteMapping(ctx context.Context, deployment, revision string) error {
	return r.db.WithContext(ctx).Where("deployment_id = ? AND revision_id = ?", deployment, revision).Delete(&knowledgeProviderRevision{}).Error
}
func (r *Repository) GenerationMappings(ctx context.Context, deployment, base string, generation int64) ([]ragflow.Mapping, error) {
	var available int64
	if err := r.db.WithContext(ctx).Table("knowledge_index_generations").Where("knowledge_base_id = ? AND generation = ? AND state = 'ready'", base, generation).Count(&available).Error; err != nil {
		return nil, err
	}
	if available != 1 {
		return nil, fmt.Errorf("%w: frozen Knowledge index is unavailable", domain.ErrInvalid)
	}
	active := func(query *gorm.DB) *gorm.DB {
		return query.Joins("JOIN knowledge_document_revisions source_revision ON source_revision.id = membership.revision_id AND source_revision.state = 'ready'").Joins("JOIN knowledge_documents source_document ON source_document.id = source_revision.document_id AND source_document.deleted_at IS NULL").Joins("LEFT JOIN knowledge_categories source_category ON source_category.id = source_document.category_id").Where("(source_document.category_id IS NULL OR source_category.deleted_at IS NULL)")
	}
	var expected int64
	count := active(r.db.WithContext(ctx).Table("knowledge_generation_revisions membership").Joins("JOIN knowledge_index_generations generation ON generation.id = membership.generation_id").Where("generation.knowledge_base_id = ? AND generation.generation = ? AND generation.state = 'ready'", base, generation))
	if err := count.Count(&expected).Error; err != nil {
		return nil, err
	}
	var rows []knowledgeProviderRevision
	query := active(r.db.WithContext(ctx).Table("knowledge_provider_revisions mapping").Select("mapping.*").Joins("JOIN knowledge_generation_revisions membership ON membership.revision_id = mapping.revision_id").Joins("JOIN knowledge_index_generations generation ON generation.id = membership.generation_id").Where("mapping.deployment_id = ? AND generation.knowledge_base_id = ? AND generation.generation = ? AND generation.state = 'ready'", deployment, base, generation))
	if err := query.Order("mapping.revision_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if int64(len(rows)) != expected {
		return nil, fmt.Errorf("%w: frozen Knowledge index is unavailable", domain.ErrInvalid)
	}
	mappings := make([]ragflow.Mapping, 0, len(rows))
	for _, row := range rows {
		mappings = append(mappings, ragflow.Mapping{BaseID: row.KnowledgeBaseID, RevisionID: row.RevisionID, DatasetID: row.DatasetID, DocumentID: row.DocumentID})
	}
	return mappings, nil
}
func (r *Repository) CleanupMappings(ctx context.Context, deployment string) ([]ragflow.Mapping, error) {
	var rows []knowledgeProviderRevision
	err := r.db.WithContext(ctx).Table("knowledge_provider_revisions mapping").Select("mapping.*").Joins("JOIN knowledge_document_revisions revision ON revision.id = mapping.revision_id").Joins("JOIN knowledge_documents document ON document.id = revision.document_id").Joins("JOIN knowledge_bases base ON base.id = document.knowledge_base_id").Joins("LEFT JOIN knowledge_categories category ON category.id = document.category_id").Where("mapping.deployment_id = ? AND (base.deleted_at < now() - interval '30 days' OR document.deleted_at < now() - interval '30 days' OR category.deleted_at < now() - interval '30 days')", deployment).Limit(100).Find(&rows).Error
	mappings := make([]ragflow.Mapping, 0, len(rows))
	for _, row := range rows {
		mappings = append(mappings, ragflow.Mapping{BaseID: row.KnowledgeBaseID, RevisionID: row.RevisionID, DatasetID: row.DatasetID, DocumentID: row.DocumentID})
	}
	return mappings, err
}

// ValidateKnowledgeGeneration checks both present-day access and the immutable
// generation manifest. Historical Ready flags alone never establish availability.
func (r *Repository) ValidateKnowledgeGeneration(ctx context.Context, owner, base string, generation int64) error {
	if _, err := r.GetKnowledgeBase(ctx, owner, base, true, false); err != nil {
		return err
	}
	var count int64
	err := r.db.WithContext(ctx).Table("knowledge_index_generations generation").Where("generation.knowledge_base_id = ? AND generation.generation = ? AND generation.state = 'ready' AND EXISTS (SELECT 1 FROM knowledge_generation_revisions membership WHERE membership.generation_id = generation.id)", base, generation).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%w: frozen Knowledge generation is unavailable", domain.ErrInvalid)
	}
	return nil
}
func (r *Repository) ResolveKnowledgeGenerationSource(ctx context.Context, owner, base, revision string, generation int64) (domain.KnowledgeSearchSource, error) {
	return r.resolveKnowledgeSource(ctx, owner, base, revision, true, generation)
}

// CleanupExpiredKnowledgeSources runs only after provider mappings are gone.
// Shared source keys (regeneration) are removed only when every reference has
// passed its restore window. Tombstone metadata and retained citations survive.
func (r *Repository) CleanupExpiredKnowledgeSources(ctx context.Context, objects objectstore.Provider) (bool, error) {
	var keys []string
	err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT revision.object_key FROM knowledge_document_revisions revision
 JOIN knowledge_documents document ON document.id = revision.document_id
 JOIN knowledge_bases base ON base.id = document.knowledge_base_id
 LEFT JOIN knowledge_categories category ON category.id = document.category_id
 WHERE revision.object_key <> ''
 AND (base.deleted_at < now() - interval '30 days' OR document.deleted_at < now() - interval '30 days' OR category.deleted_at < now() - interval '30 days')
 AND NOT EXISTS (SELECT 1 FROM knowledge_provider_revisions mapping JOIN knowledge_document_revisions mapped ON mapped.id = mapping.revision_id WHERE mapped.object_key = revision.object_key)
 AND NOT EXISTS (SELECT 1 FROM knowledge_document_revisions retained JOIN knowledge_documents retained_doc ON retained_doc.id = retained.document_id JOIN knowledge_bases retained_base ON retained_base.id = retained_doc.knowledge_base_id LEFT JOIN knowledge_categories retained_category ON retained_category.id = retained_doc.category_id WHERE retained.object_key = revision.object_key AND NOT (COALESCE(retained_base.deleted_at < now() - interval '30 days',false) OR COALESCE(retained_doc.deleted_at < now() - interval '30 days',false) OR COALESCE(retained_category.deleted_at < now() - interval '30 days',false))) LIMIT 100`).Scan(&keys).Error
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		if err = objects.Delete(ctx, key); err != nil {
			return true, err
		}
		if err = r.db.WithContext(ctx).Table("knowledge_document_revisions").Where("object_key = ?", key).Updates(map[string]any{"object_key": "", "state": "blocked", "error": "Knowledge source retention expired"}).Error; err != nil {
			return true, err
		}
	}
	return len(keys) > 0, nil
}
