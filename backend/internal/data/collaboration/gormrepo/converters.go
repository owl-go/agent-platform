package gormrepo

import (
	"encoding/json"
	"fmt"

	"agent-platform/backend/internal/biz/collaboration/domain"
)

func restoreTask(record taskRecord) (domain.Task, error) {
	var issue *domain.IssueSnapshot
	if len(record.IssueSnapshot) > 0 {
		var value domain.IssueSnapshot
		if err := json.Unmarshal(record.IssueSnapshot, &value); err != nil {
			return domain.Task{}, fmt.Errorf("decode Issue Snapshot: %w", err)
		}
		issue = &value
	}
	return domain.RestoreTask(domain.Task{
		ID: record.ID, OrganizationID: record.OrganizationID, TeamID: record.TeamID,
		AgentReleaseID: record.AgentReleaseID, CreatedBy: record.CreatedBy, Title: record.Title,
		RequestText: record.RequestText, IssueSnapshot: issue, State: record.State,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, CompletedAt: record.CompletedAt, Version: record.Version,
	})
}

func taskToRecord(task domain.Task) (taskRecord, error) {
	var issue jsonValue
	if task.IssueSnapshot != nil {
		encoded, err := json.Marshal(task.IssueSnapshot)
		if err != nil {
			return taskRecord{}, err
		}
		issue = encoded
	}
	return taskRecord{
		ID: task.ID, OrganizationID: task.OrganizationID, TeamID: task.TeamID,
		AgentReleaseID: task.AgentReleaseID, CreatedBy: task.CreatedBy, Title: task.Title,
		RequestText: task.RequestText, IssueSnapshot: issue, State: task.State,
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, CompletedAt: task.CompletedAt, Version: task.Version,
	}, nil
}

func restoreSession(record sessionRecord) (domain.Session, error) {
	memory := domain.SessionMemory{}
	if len(record.SessionMemory) > 0 {
		if err := json.Unmarshal(record.SessionMemory, &memory); err != nil {
			return domain.Session{}, fmt.Errorf("decode Session Memory: %w", err)
		}
	}
	return domain.RestoreSession(domain.Session{
		ID: record.ID, CodingTaskID: record.CodingTaskID, RepositoryBindingID: record.RepositoryBindingID,
		TargetBranch: record.TargetBranch, ReviewBranch: record.ReviewBranch, WorkspaceVolume: record.WorkspaceVolume,
		Memory: memory, RunCount: record.RunCount, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, Version: record.Version,
	})
}

func sessionToRecord(session domain.Session) (sessionRecord, error) {
	memory, err := json.Marshal(session.Memory)
	if err != nil {
		return sessionRecord{}, err
	}
	return sessionRecord{
		ID: session.ID, CodingTaskID: session.CodingTaskID, RepositoryBindingID: session.RepositoryBindingID,
		TargetBranch: session.TargetBranch, ReviewBranch: session.ReviewBranch, WorkspaceVolume: session.WorkspaceVolume,
		SessionMemory: memory, RunCount: session.RunCount, CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt, Version: session.Version,
	}, nil
}

func restoreCandidate(record candidateRecord) domain.MemoryCandidate {
	value := domain.MemoryCandidate{
		ID: record.ID, AgentID: record.AgentID, CodingTaskID: record.CodingTaskID,
		ProposedContent: record.ProposedContent, State: record.State, ProposedAt: record.ProposedAt,
		DecidedAt: record.DecidedAt,
	}
	if record.DecidedBy != nil {
		value.DecidedBy = *record.DecidedBy
	}
	if record.ResultingMemoryID != nil {
		value.ResultingMemoryID = *record.ResultingMemoryID
	}
	return value
}

func restoreMemory(record memoryRecord) domain.AgentMemory {
	value := domain.AgentMemory{
		ID: record.ID, AgentID: record.AgentID, Content: record.Content, Enabled: record.Enabled,
		ApprovedBy: record.ApprovedBy, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		DeletedAt: record.DeletedAt, Version: record.Version,
	}
	if record.SourceTaskID != nil {
		value.SourceTaskID = *record.SourceTaskID
	}
	return value
}
