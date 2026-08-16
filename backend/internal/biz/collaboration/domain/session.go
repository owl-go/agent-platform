package domain

import (
	"fmt"
	"strings"
	"time"
)

const MaximumSessionRuns = 50

type SessionMemory struct {
	Summary            string   `json:"summary,omitempty"`
	ConfirmedDecisions []string `json:"confirmed_decisions,omitempty"`
	Results            []string `json:"results,omitempty"`
	WorkspaceSnapshots []string `json:"workspace_snapshots,omitempty"`
}

type Session struct {
	ID                  string
	CodingTaskID        string
	RepositoryBindingID string
	TargetBranch        string
	ReviewBranch        string
	WorkspaceVolume     string
	Memory              SessionMemory
	RunCount            int
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int64
}

type SessionRegistration struct {
	ID, CodingTaskID, RepositoryBindingID       string
	TargetBranch, ReviewBranch, WorkspaceVolume string
	Now                                         time.Time
}

func OpenSession(registration SessionRegistration) (Session, error) {
	if strings.TrimSpace(registration.ID) == "" || strings.TrimSpace(registration.CodingTaskID) == "" ||
		strings.TrimSpace(registration.RepositoryBindingID) == "" || strings.TrimSpace(registration.WorkspaceVolume) == "" {
		return Session{}, fmt.Errorf("Session identity, Coding Task, Repository Binding, and Workspace are required")
	}
	if err := validateBranch(registration.TargetBranch); err != nil {
		return Session{}, fmt.Errorf("invalid target branch: %w", err)
	}
	if err := validateBranch(registration.ReviewBranch); err != nil {
		return Session{}, fmt.Errorf("invalid Review Branch: %w", err)
	}
	if registration.TargetBranch == registration.ReviewBranch {
		return Session{}, fmt.Errorf("Review Branch must differ from target branch")
	}
	now := registration.Now.UTC()
	return Session{
		ID: registration.ID, CodingTaskID: registration.CodingTaskID,
		RepositoryBindingID: registration.RepositoryBindingID, TargetBranch: registration.TargetBranch,
		ReviewBranch: registration.ReviewBranch, WorkspaceVolume: registration.WorkspaceVolume,
		Memory: SessionMemory{}, CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func RestoreSession(session Session) (Session, error) {
	if session.Version <= 0 || session.RunCount < 0 || session.RunCount > MaximumSessionRuns || session.CreatedAt.IsZero() || session.UpdatedAt.IsZero() {
		return Session{}, fmt.Errorf("invalid persisted Session")
	}
	if err := validateMemory(session.Memory); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (session *Session) AddRun(now time.Time) error {
	if session.RunCount >= MaximumSessionRuns {
		return ErrRunLimitReached
	}
	session.RunCount++
	session.UpdatedAt = now.UTC()
	session.Version++
	return nil
}

func (session *Session) UpdateMemory(memory SessionMemory, now time.Time) error {
	if err := validateMemory(memory); err != nil {
		return err
	}
	session.Memory = cloneMemory(memory)
	session.UpdatedAt = now.UTC()
	session.Version++
	return nil
}

func validateMemory(memory SessionMemory) error {
	if len(memory.Summary) > 20_000 || len(memory.ConfirmedDecisions) > 200 || len(memory.Results) > 200 || len(memory.WorkspaceSnapshots) > 200 {
		return fmt.Errorf("Session Memory exceeds its limit")
	}
	for _, values := range [][]string{memory.ConfirmedDecisions, memory.Results, memory.WorkspaceSnapshots} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" || len(value) > 4_000 {
				return fmt.Errorf("Session Memory entries must be non-empty and within limits")
			}
		}
	}
	return nil
}

func cloneMemory(memory SessionMemory) SessionMemory {
	memory.ConfirmedDecisions = append([]string(nil), memory.ConfirmedDecisions...)
	memory.Results = append([]string(nil), memory.Results...)
	memory.WorkspaceSnapshots = append([]string(nil), memory.WorkspaceSnapshots...)
	return memory
}

func validateBranch(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "/") ||
		strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") ||
		strings.Contains(value, "@{") || strings.ContainsAny(value, " ~^:?*[\\") {
		return fmt.Errorf("unsafe Git ref name")
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return fmt.Errorf("unsafe Git ref component")
		}
	}
	return nil
}
