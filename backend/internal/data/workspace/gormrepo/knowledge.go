package gormrepo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var knowledgeSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func knowledgeBaseAccess(query *gorm.DB, ownerID string, includeDeleted bool) *gorm.DB {
	query = query.Where("(knowledge_bases.owner_user_id = ? OR (knowledge_bases.platform = true AND knowledge_bases.visibility = 'public'))", ownerID)
	if !includeDeleted {
		query = query.Where("knowledge_bases.deleted_at IS NULL")
	}
	return query
}

func knowledgeMutationAccess(query *gorm.DB, ownerID string, administrator bool) *gorm.DB {
	if administrator {
		return query.Where("knowledge_bases.owner_user_id = ?", ownerID)
	}
	return query.Where("knowledge_bases.owner_user_id = ? AND knowledge_bases.platform = false", ownerID)
}

func (repository *Repository) ListKnowledgeBases(ctx context.Context, ownerID string, administrator, includeDeleted bool) ([]domain.KnowledgeBase, error) {
	query := knowledgeBaseAccess(repository.db.WithContext(ctx).Table("knowledge_bases"), ownerID, includeDeleted)
	if !administrator {
		query = query.Where("knowledge_bases.platform = false OR knowledge_bases.visibility = 'public'")
	}
	var rows []knowledgeBaseRecord
	if err := query.Order("knowledge_bases.updated_at DESC, knowledge_bases.id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Knowledge Bases: %w", err)
	}
	items := make([]domain.KnowledgeBase, 0, len(rows))
	for _, row := range rows {
		item, err := knowledgeBaseDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) GetKnowledgeBase(ctx context.Context, ownerID, knowledgeBaseID string, administrator, includeDeleted bool) (domain.KnowledgeBase, error) {
	query := knowledgeBaseAccess(repository.db.WithContext(ctx).Table("knowledge_bases"), ownerID, includeDeleted).Where("knowledge_bases.id = ?", knowledgeBaseID)
	if !administrator {
		query = query.Where("knowledge_bases.platform = false OR knowledge_bases.visibility = 'public'")
	}
	var row knowledgeBaseRecord
	if err := query.Take(&row).Error; err != nil {
		return domain.KnowledgeBase{}, mapNotFound(err)
	}
	return knowledgeBaseDomain(row)
}

func (repository *Repository) CreateKnowledgeBase(ctx context.Context, ownerID string, administrator bool, input domain.KnowledgeBaseInput) (domain.KnowledgeBase, error) {
	if err := input.Validate(administrator); err != nil {
		return domain.KnowledgeBase{}, err
	}
	row := knowledgeBaseRecord{ID: uuid.NewString(), OwnerID: ownerID, Platform: input.Platform, Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description), Visibility: string(input.Visibility), Version: 1}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.KnowledgeBase{}, fmt.Errorf("create Knowledge Base: %w", err)
	}
	return knowledgeBaseDomain(row)
}

func (repository *Repository) UpdateKnowledgeBase(ctx context.Context, ownerID, knowledgeBaseID string, administrator bool, input domain.KnowledgeBaseInput, expectedVersion int64) (domain.KnowledgeBase, error) {
	if err := input.Validate(administrator); err != nil {
		return domain.KnowledgeBase{}, err
	}
	updates := map[string]any{"name": strings.TrimSpace(input.Name), "description": strings.TrimSpace(input.Description), "visibility": string(input.Visibility), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
	result := knowledgeMutationAccess(repository.db.WithContext(ctx).Table("knowledge_bases"), ownerID, administrator).
		Where("knowledge_bases.id = ? AND knowledge_bases.deleted_at IS NULL AND knowledge_bases.version = ?", knowledgeBaseID, expectedVersion).
		Updates(updates)
	if result.Error != nil {
		return domain.KnowledgeBase{}, fmt.Errorf("update Knowledge Base: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.KnowledgeBase{}, domain.ErrConflict
	}
	return repository.GetKnowledgeBase(ctx, ownerID, knowledgeBaseID, administrator, false)
}

func (repository *Repository) DeleteKnowledgeBase(ctx context.Context, ownerID, knowledgeBaseID string, administrator bool) error {
	result := knowledgeMutationAccess(repository.db.WithContext(ctx).Table("knowledge_bases"), ownerID, administrator).
		Where("knowledge_bases.id = ? AND knowledge_bases.deleted_at IS NULL", knowledgeBaseID).
		Updates(map[string]any{"deleted_at": time.Now().UTC(), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return fmt.Errorf("delete Knowledge Base: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (repository *Repository) ListKnowledgeCategories(ctx context.Context, ownerID, knowledgeBaseID string, administrator bool) ([]domain.KnowledgeCategory, error) {
	query := repository.db.WithContext(ctx).Table("knowledge_categories").Joins("JOIN knowledge_bases ON knowledge_bases.id = knowledge_categories.knowledge_base_id")
	query = knowledgeBaseAccess(query, ownerID, false).Where("knowledge_categories.knowledge_base_id = ? AND knowledge_categories.deleted_at IS NULL", knowledgeBaseID)
	if !administrator {
		query = query.Where("knowledge_bases.platform = false OR knowledge_bases.visibility = 'public'")
	}
	var rows []knowledgeCategoryRecord
	if err := query.Order("knowledge_categories.name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Knowledge Categories: %w", err)
	}
	items := make([]domain.KnowledgeCategory, 0, len(rows))
	for _, row := range rows {
		items = append(items, knowledgeCategoryDomain(row))
	}
	return items, nil
}

func (repository *Repository) CreateKnowledgeCategory(ctx context.Context, ownerID, knowledgeBaseID string, administrator bool, name string) (domain.KnowledgeCategory, error) {
	if err := domain.ValidateKnowledgeCategoryName(name); err != nil {
		return domain.KnowledgeCategory{}, err
	}
	var base knowledgeBaseRecord
	query := knowledgeMutationAccess(repository.db.WithContext(ctx).Table("knowledge_bases"), ownerID, administrator).Where("knowledge_bases.id = ? AND knowledge_bases.deleted_at IS NULL", knowledgeBaseID)
	if err := query.Take(&base).Error; err != nil {
		return domain.KnowledgeCategory{}, mapNotFound(err)
	}
	row := knowledgeCategoryRecord{ID: uuid.NewString(), KnowledgeBaseID: knowledgeBaseID, Name: strings.TrimSpace(name), Version: 1}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.KnowledgeCategory{}, fmt.Errorf("create Knowledge Category: %w", err)
	}
	return knowledgeCategoryDomain(row), nil
}

func (repository *Repository) DeleteKnowledgeCategory(ctx context.Context, ownerID, knowledgeBaseID, categoryID string, administrator bool) error {
	query := knowledgeMutationAccess(repository.db.WithContext(ctx).Table("knowledge_bases"), ownerID, administrator).
		Joins("JOIN knowledge_categories ON knowledge_categories.knowledge_base_id = knowledge_bases.id").
		Where("knowledge_bases.id = ? AND knowledge_categories.id = ? AND knowledge_categories.deleted_at IS NULL", knowledgeBaseID, categoryID)
	var base knowledgeBaseRecord
	if err := query.Take(&base).Error; err != nil {
		return mapNotFound(err)
	}
	result := repository.db.WithContext(ctx).Model(&knowledgeCategoryRecord{}).Where("id = ? AND deleted_at IS NULL", categoryID).Updates(map[string]any{"deleted_at": time.Now().UTC(), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return fmt.Errorf("delete Knowledge Category: %w", result.Error)
	}
	return nil
}

func (repository *Repository) ListKnowledgeDocuments(ctx context.Context, ownerID, knowledgeBaseID string, administrator bool) ([]domain.KnowledgeDocument, error) {
	query := repository.db.WithContext(ctx).Table("knowledge_documents").Joins("JOIN knowledge_bases ON knowledge_bases.id = knowledge_documents.knowledge_base_id")
	query = knowledgeBaseAccess(query, ownerID, false).Where("knowledge_documents.knowledge_base_id = ? AND knowledge_documents.deleted_at IS NULL", knowledgeBaseID)
	if !administrator {
		query = query.Where("knowledge_bases.platform = false OR knowledge_bases.visibility = 'public'")
	}
	var rows []knowledgeDocumentRecord
	if err := query.Order("knowledge_documents.updated_at DESC, knowledge_documents.id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Knowledge Documents: %w", err)
	}
	items := make([]domain.KnowledgeDocument, 0, len(rows))
	for _, row := range rows {
		item := knowledgeDocumentDomain(row)
		var revision knowledgeRevisionRecord
		if err := repository.db.WithContext(ctx).Where("document_id = ?", row.ID).Order("revision DESC").Take(&revision).Error; err == nil {
			item.LatestRevision = knowledgeRevisionDomain(revision)
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) CreateKnowledgeDocument(ctx context.Context, ownerID string, administrator bool, input domain.KnowledgeDocumentInput) (domain.KnowledgeDocument, error) {
	name := strings.TrimSpace(input.Name)
	if len(name) < 1 || len(name) > 255 || strings.TrimSpace(input.NormalizedSource) == "" || strings.TrimSpace(input.ObjectKey) == "" || strings.TrimSpace(input.ContentType) == "" || input.Size < 0 || !knowledgeSHA256Pattern.MatchString(input.SHA256) {
		return domain.KnowledgeDocument{}, fmt.Errorf("%w: invalid Knowledge Document", domain.ErrInvalid)
	}
	sourceURI := ""
	if input.SourceURI != nil {
		sourceURI = *input.SourceURI
	}
	if err := domain.ValidateKnowledgeSource(input.SourceType, sourceURI); err != nil {
		return domain.KnowledgeDocument{}, err
	}
	var result domain.KnowledgeDocument
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var base knowledgeBaseRecord
		query := knowledgeMutationAccess(tx.Table("knowledge_bases"), ownerID, administrator).Where("knowledge_bases.id = ? AND knowledge_bases.deleted_at IS NULL", input.KnowledgeBaseID)
		if err := query.Take(&base).Error; err != nil {
			return mapNotFound(err)
		}
		if input.CategoryID != nil {
			var count int64
			if err := tx.Model(&knowledgeCategoryRecord{}).Where("id = ? AND knowledge_base_id = ? AND deleted_at IS NULL", *input.CategoryID, input.KnowledgeBaseID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("%w: Knowledge Category does not belong to the Knowledge Base", domain.ErrInvalid)
			}
		}

		var document knowledgeDocumentRecord
		documentQuery := tx.Where("knowledge_base_id = ? AND normalized_source = ? AND deleted_at IS NULL", input.KnowledgeBaseID, input.NormalizedSource)
		if input.CategoryID == nil {
			documentQuery = documentQuery.Where("category_id IS NULL")
		} else {
			documentQuery = documentQuery.Where("category_id = ?", *input.CategoryID)
		}
		err := documentQuery.Take(&document).Error
		switch {
		case err == nil:
			document.Name = name
			document.SourceURI = input.SourceURI
			document.State = string(domain.KnowledgeAccepted)
			document.Error = ""
			document.UpdatedAt = time.Now().UTC()
			document.Version++
			if err := tx.Save(&document).Error; err != nil {
				return err
			}
		case err == gorm.ErrRecordNotFound:
			document = knowledgeDocumentRecord{ID: uuid.NewString(), KnowledgeBaseID: input.KnowledgeBaseID, CategoryID: input.CategoryID, Name: name, SourceType: string(input.SourceType), SourceURI: input.SourceURI, State: string(domain.KnowledgeAccepted), Version: 1}
			if err := tx.Table("knowledge_documents").Create(map[string]any{"id": document.ID, "knowledge_base_id": document.KnowledgeBaseID, "category_id": document.CategoryID, "name": document.Name, "source_type": document.SourceType, "source_uri": document.SourceURI, "normalized_source": input.NormalizedSource, "state": document.State, "error": "", "version": document.Version}).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		}

		var latest int
		if err := tx.Model(&knowledgeRevisionRecord{}).Where("document_id = ?", document.ID).Select("COALESCE(MAX(revision), 0)").Scan(&latest).Error; err != nil {
			return err
		}
		revision := knowledgeRevisionRecord{ID: uuid.NewString(), DocumentID: document.ID, Revision: latest + 1, ObjectKey: input.ObjectKey, SHA256: input.SHA256, Size: input.Size, ContentType: input.ContentType, State: string(domain.KnowledgeAccepted)}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		if err := tx.Table("knowledge_ingestion_jobs").Create(map[string]any{"id": uuid.NewString(), "revision_id": revision.ID, "idempotency_key": "knowledge-revision:" + revision.ID, "state": "queued"}).Error; err != nil {
			return err
		}
		result = knowledgeDocumentDomain(document)
		result.LatestRevision = knowledgeRevisionDomain(revision)
		return nil
	})
	if err != nil {
		return domain.KnowledgeDocument{}, fmt.Errorf("create Knowledge Document: %w", err)
	}
	return result, nil
}

func knowledgeBaseDomain(row knowledgeBaseRecord) (domain.KnowledgeBase, error) {
	visibility := domain.KnowledgeVisibility(row.Visibility)
	if visibility != domain.KnowledgePrivate && visibility != domain.KnowledgePublic {
		return domain.KnowledgeBase{}, fmt.Errorf("%w: invalid Knowledge Base visibility", domain.ErrInvalid)
	}
	return domain.KnowledgeBase{ID: row.ID, OwnerID: row.OwnerID, Platform: row.Platform, Name: row.Name, Description: row.Description, Visibility: visibility, DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}, nil
}

func knowledgeCategoryDomain(row knowledgeCategoryRecord) domain.KnowledgeCategory {
	return domain.KnowledgeCategory{ID: row.ID, KnowledgeBaseID: row.KnowledgeBaseID, Name: row.Name, DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}

func knowledgeDocumentDomain(row knowledgeDocumentRecord) domain.KnowledgeDocument {
	return domain.KnowledgeDocument{ID: row.ID, KnowledgeBaseID: row.KnowledgeBaseID, CategoryID: row.CategoryID, Name: row.Name, SourceType: domain.KnowledgeSourceType(row.SourceType), SourceURI: row.SourceURI, State: domain.KnowledgeDocumentState(row.State), Error: row.Error, DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}

func knowledgeRevisionDomain(row knowledgeRevisionRecord) *domain.KnowledgeDocumentRevision {
	return &domain.KnowledgeDocumentRevision{ID: row.ID, DocumentID: row.DocumentID, Revision: row.Revision, ObjectKey: row.ObjectKey, SHA256: row.SHA256, Size: row.Size, ContentType: row.ContentType, State: domain.KnowledgeDocumentState(row.State), Error: row.Error, CreatedAt: row.CreatedAt, ReadyAt: row.ReadyAt}
}
