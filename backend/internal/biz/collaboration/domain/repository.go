package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrReleaseUnavailable      = errors.New("Agent Release is unavailable for Coding Task creation")
	ErrTaskStateConflict       = errors.New("Coding Task state does not allow this operation")
	ErrMemoryCandidateNotFound = errors.New("Memory Candidate not found")
	ErrAgentMemoryNotFound     = errors.New("Agent Memory not found")
)

type MessageAuthor string

const (
	MessageAuthorUser   MessageAuthor = "user"
	MessageAuthorAgent  MessageAuthor = "agent"
	MessageAuthorSystem MessageAuthor = "system"
)

type Message struct {
	ID           int64
	SessionID    string
	RunID        string
	Author       MessageAuthor
	AuthorUserID string
	Content      json.RawMessage
	CreatedAt    time.Time
}

type RunSeed struct {
	ID                  string
	RequestText         string
	CreatedBy           string
	ModelBudgetOverride json.RawMessage
}

type LaunchRegistration struct {
	Task            Task
	SessionID       string
	Run             RunSeed
	ReviewBranch    string
	WorkspaceVolume string
	Now             time.Time
}

type Launch struct {
	Task    Task
	Session Session
	RunID   string
}

type ContinueRegistration struct {
	OrganizationID, TeamID, TaskID string
	Run                            RunSeed
	ExpectedTaskVersion            int64
	ExpectedSessionVersion         int64
	Now                            time.Time
}

type QueuedRunPlan struct {
	ID, SessionID, CodingTaskID, AgentReleaseID, RuntimeImageID string
	RequestText, CreatedBy                                      string
	ModelBinding, CredentialBindings, ModelBudget               json.RawMessage
	ExecutionLimits                                             json.RawMessage
	CreatedAt                                                   time.Time
}

type Repository interface {
	GetTask(context.Context, string, string, string) (Task, error)
	ListTasks(context.Context, string, string) ([]Task, error)
	UpdateTask(context.Context, Task, int64) error
	GetSessionByTask(context.Context, string) (Session, error)
	UpdateSession(context.Context, Session, int64) error
	ListMessages(context.Context, string, int64, int) ([]Message, error)
	CreateMemoryCandidate(context.Context, MemoryCandidate) error
	GetMemoryCandidate(context.Context, string, string, string) (MemoryCandidate, error)
	ListMemoryCandidates(context.Context, string, string, string) ([]MemoryCandidate, error)
	DecideMemoryCandidate(context.Context, MemoryCandidate, *AgentMemory) error
	GetAgentMemory(context.Context, string, string, string) (AgentMemory, error)
	ListAgentMemories(context.Context, string, string, string, bool) ([]AgentMemory, error)
	UpdateAgentMemory(context.Context, AgentMemory, int64) error
}

type LaunchCoordinator interface {
	CreateLaunch(context.Context, LaunchRegistration) (Launch, error)
	Continue(context.Context, ContinueRegistration) (Launch, error)
}

type CompletionProjector interface {
	ProjectCompletedRun(context.Context, string, string, time.Time) error
}
