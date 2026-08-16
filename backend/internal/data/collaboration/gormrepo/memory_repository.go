package gormrepo

import (
	"context"
	"errors"
	"fmt"

	"agent-platform/backend/internal/biz/collaboration/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) CreateMemoryCandidate(ctx context.Context, candidate domain.MemoryCandidate) error {
	record := candidateRecord{
		ID: candidate.ID, AgentID: candidate.AgentID, CodingTaskID: candidate.CodingTaskID,
		ProposedContent: candidate.ProposedContent, State: candidate.State, ProposedAt: candidate.ProposedAt,
	}
	result := repository.db.WithContext(ctx).Exec(`
		INSERT INTO memory_candidates (id, agent_id, coding_task_id, proposed_content, state, proposed_at)
		SELECT ?, ?, ?, ?, ?, ?
		FROM coding_tasks task
		JOIN agent_releases release ON release.id = task.agent_release_id
		WHERE task.id = ? AND release.agent_id = ?`,
		record.ID, record.AgentID, record.CodingTaskID, record.ProposedContent, record.State, record.ProposedAt,
		record.CodingTaskID, record.AgentID)
	if result.Error != nil {
		return fmt.Errorf("create Memory Candidate: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrTaskNotFound
	}
	return nil
}

func (repository *Repository) GetMemoryCandidate(ctx context.Context, organizationID, teamID, id string) (domain.MemoryCandidate, error) {
	var record candidateRecord
	err := repository.db.WithContext(ctx).Table("memory_candidates AS candidate").
		Select("candidate.*").
		Joins("JOIN coding_tasks task ON task.id = candidate.coding_task_id").
		Where("candidate.id = ? AND task.organization_id = ? AND task.team_id = ?", id, organizationID, teamID).
		Take(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.MemoryCandidate{}, domain.ErrMemoryCandidateNotFound
		}
		return domain.MemoryCandidate{}, fmt.Errorf("load Memory Candidate: %w", err)
	}
	return restoreCandidate(record), nil
}

func (repository *Repository) ListMemoryCandidates(ctx context.Context, organizationID, teamID, taskID string) ([]domain.MemoryCandidate, error) {
	var records []candidateRecord
	err := repository.db.WithContext(ctx).Table("memory_candidates AS candidate").
		Select("candidate.*").
		Joins("JOIN coding_tasks task ON task.id = candidate.coding_task_id").
		Where("task.organization_id = ? AND task.team_id = ? AND task.id = ?", organizationID, teamID, taskID).
		Order("candidate.proposed_at DESC, candidate.id DESC").Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list Memory Candidates: %w", err)
	}
	result := make([]domain.MemoryCandidate, 0, len(records))
	for _, record := range records {
		result = append(result, restoreCandidate(record))
	}
	return result, nil
}

func (repository *Repository) DecideMemoryCandidate(ctx context.Context, candidate domain.MemoryCandidate, memory *domain.AgentMemory) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current candidateRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", candidate.ID).Take(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrMemoryCandidateNotFound
			}
			return err
		}
		if current.State != domain.MemoryCandidatePending {
			return domain.ErrConcurrentUpdate
		}
		updates := map[string]any{
			"state": candidate.State, "decided_by": candidate.DecidedBy,
			"decided_at": candidate.DecidedAt,
		}
		if memory != nil {
			if candidate.State != domain.MemoryCandidateApproved || candidate.ResultingMemoryID != memory.ID {
				return fmt.Errorf("approved Memory Candidate and Agent Memory are inconsistent")
			}
			sourceTaskID := memory.SourceTaskID
			record := memoryRecord{
				ID: memory.ID, AgentID: memory.AgentID, Content: memory.Content, Enabled: memory.Enabled,
				ApprovedBy: memory.ApprovedBy, SourceTaskID: &sourceTaskID,
				CreatedAt: memory.CreatedAt, UpdatedAt: memory.UpdatedAt, Version: memory.Version,
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("create Agent Memory: %w", err)
			}
			updates["resulting_memory_id"] = memory.ID
		} else if candidate.State != domain.MemoryCandidateRejected {
			return fmt.Errorf("rejected Memory Candidate is required when no Agent Memory is created")
		}
		result := tx.Model(&candidateRecord{}).Where("id = ? AND state = ?", candidate.ID, domain.MemoryCandidatePending).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("decide Memory Candidate: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrConcurrentUpdate
		}
		return nil
	})
}

func (repository *Repository) GetAgentMemory(ctx context.Context, organizationID, teamID, id string) (domain.AgentMemory, error) {
	var record memoryRecord
	err := repository.db.WithContext(ctx).Table("agent_memories AS memory").Select("memory.*").
		Joins("JOIN agents agent ON agent.id = memory.agent_id").
		Where("memory.id = ? AND agent.organization_id = ? AND agent.team_id = ?", id, organizationID, teamID).
		Take(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AgentMemory{}, domain.ErrAgentMemoryNotFound
		}
		return domain.AgentMemory{}, fmt.Errorf("load Agent Memory: %w", err)
	}
	return restoreMemory(record), nil
}

func (repository *Repository) ListAgentMemories(ctx context.Context, organizationID, teamID, agentID string, includeDeleted bool) ([]domain.AgentMemory, error) {
	query := repository.db.WithContext(ctx).Table("agent_memories AS memory").Select("memory.*").
		Joins("JOIN agents agent ON agent.id = memory.agent_id").
		Where("memory.agent_id = ? AND agent.organization_id = ? AND agent.team_id = ?", agentID, organizationID, teamID)
	if !includeDeleted {
		query = query.Where("memory.deleted_at IS NULL")
	}
	var records []memoryRecord
	if err := query.Order("memory.created_at DESC, memory.id DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Agent Memories: %w", err)
	}
	result := make([]domain.AgentMemory, 0, len(records))
	for _, record := range records {
		result = append(result, restoreMemory(record))
	}
	return result, nil
}

func (repository *Repository) UpdateAgentMemory(ctx context.Context, memory domain.AgentMemory, expectedVersion int64) error {
	result := repository.db.WithContext(ctx).Model(&memoryRecord{}).
		Where("id = ? AND version = ?", memory.ID, expectedVersion).
		Updates(map[string]any{
			"content": memory.Content, "enabled": memory.Enabled, "updated_at": memory.UpdatedAt,
			"deleted_at": memory.DeletedAt, "version": memory.Version,
		})
	if result.Error != nil {
		return fmt.Errorf("update Agent Memory: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}
