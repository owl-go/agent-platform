package domain

import (
	"fmt"
	"strings"
	"time"
)

type MemoryCandidateState string

const (
	MemoryCandidatePending  MemoryCandidateState = "pending"
	MemoryCandidateApproved MemoryCandidateState = "approved"
	MemoryCandidateRejected MemoryCandidateState = "rejected"
)

type MemoryCandidate struct {
	ID, AgentID, CodingTaskID string
	ProposedContent           string
	State                     MemoryCandidateState
	ProposedAt                time.Time
	DecidedBy                 string
	DecidedAt                 *time.Time
	ResultingMemoryID         string
}

type AgentMemory struct {
	ID, AgentID, Content, ApprovedBy, SourceTaskID string
	Enabled                                        bool
	CreatedAt, UpdatedAt                           time.Time
	DeletedAt                                      *time.Time
	Version                                        int64
}

func ProposeMemory(id, agentID, taskID, content string, now time.Time) (MemoryCandidate, error) {
	content = strings.TrimSpace(content)
	if id == "" || agentID == "" || taskID == "" || content == "" {
		return MemoryCandidate{}, fmt.Errorf("Memory Candidate identity, source, and content are required")
	}
	if len(content) > 4_000 {
		return MemoryCandidate{}, fmt.Errorf("Memory Candidate content exceeds its limit")
	}
	return MemoryCandidate{ID: id, AgentID: agentID, CodingTaskID: taskID, ProposedContent: content, State: MemoryCandidatePending, ProposedAt: now.UTC()}, nil
}

func (candidate *MemoryCandidate) Approve(memoryID, decidedBy string, now time.Time) (AgentMemory, error) {
	if candidate.State != MemoryCandidatePending || memoryID == "" || decidedBy == "" {
		return AgentMemory{}, fmt.Errorf("pending Memory Candidate, Memory ID, and decision maker are required")
	}
	decidedAt := now.UTC()
	candidate.State = MemoryCandidateApproved
	candidate.DecidedBy = decidedBy
	candidate.DecidedAt = &decidedAt
	candidate.ResultingMemoryID = memoryID
	return AgentMemory{
		ID: memoryID, AgentID: candidate.AgentID, Content: candidate.ProposedContent, Enabled: true,
		ApprovedBy: decidedBy, SourceTaskID: candidate.CodingTaskID,
		CreatedAt: decidedAt, UpdatedAt: decidedAt, Version: 1,
	}, nil
}

func (candidate *MemoryCandidate) Reject(decidedBy string, now time.Time) error {
	if candidate.State != MemoryCandidatePending || decidedBy == "" {
		return fmt.Errorf("pending Memory Candidate and decision maker are required")
	}
	decidedAt := now.UTC()
	candidate.State = MemoryCandidateRejected
	candidate.DecidedBy = decidedBy
	candidate.DecidedAt = &decidedAt
	return nil
}

func (memory *AgentMemory) Edit(content string, enabled bool, now time.Time) error {
	content = strings.TrimSpace(content)
	if memory.DeletedAt != nil || content == "" || len(content) > 4_000 {
		return fmt.Errorf("active Agent Memory and valid content are required")
	}
	memory.Content = content
	memory.Enabled = enabled
	memory.UpdatedAt = now.UTC()
	memory.Version++
	return nil
}

func (memory *AgentMemory) Delete(now time.Time) error {
	if memory.DeletedAt != nil {
		return fmt.Errorf("Agent Memory is already deleted")
	}
	deleted := now.UTC()
	memory.DeletedAt = &deleted
	memory.Enabled = false
	memory.UpdatedAt = deleted
	memory.Version++
	return nil
}
