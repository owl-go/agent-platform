package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrAgentNotFound        = errors.New("Agent not found")
	ErrAgentNameExists      = errors.New("Agent name already exists")
	ErrConcurrentUpdate     = errors.New("Agent Lifecycle resource was modified concurrently")
	ErrInvalidAgent         = errors.New("invalid Agent input")
	ErrDraftNotFound        = errors.New("Agent Draft not found")
	ErrDraftNotReady        = errors.New("Agent Draft is not ready for release")
	ErrReleaseNotFound      = errors.New("Agent Release not found")
	ErrDraftAlreadyReleased = errors.New("Agent Draft already has a Release")
	ErrApprovalRequired     = errors.New("high-risk Agent Release requires approval")
	ErrApprovalNotFound     = errors.New("Agent Release Approval not found")
	ErrApprovalExists       = errors.New("Agent Release Approval already exists")
)

type Agent struct {
	ID             string
	OrganizationID string
	TeamID         string
	Name           string
	Description    string
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

type AgentRegistration struct {
	ID             string
	OrganizationID string
	TeamID         string
	Name           string
	Description    string
	CreatedBy      string
	Now            time.Time
}

func RegisterAgent(input AgentRegistration) (Agent, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.OrganizationID) == "" || strings.TrimSpace(input.TeamID) == "" || strings.TrimSpace(input.CreatedBy) == "" {
		return Agent{}, invalidAgentf("Agent identity, Organization, Team, and creator are required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 200 || len(input.Description) > 4000 || input.Now.IsZero() {
		return Agent{}, invalidAgentf("Agent name, description, or creation time is invalid")
	}
	now := input.Now.UTC()
	return Agent{
		ID: input.ID, OrganizationID: input.OrganizationID, TeamID: input.TeamID,
		Name: name, Description: input.Description, CreatedBy: input.CreatedBy,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func RestoreAgent(input AgentRegistration, createdAt, updatedAt time.Time, version int64) (Agent, error) {
	input.Now = createdAt
	agent, err := RegisterAgent(input)
	if err != nil {
		return Agent{}, err
	}
	if version <= 0 || updatedAt.IsZero() || updatedAt.Before(createdAt) {
		return Agent{}, invalidAgentf("persisted Agent timestamps or Version are invalid")
	}
	agent.UpdatedAt = updatedAt.UTC()
	agent.Version = version
	return agent, nil
}

func (agent *Agent) Rename(name, description string, now time.Time) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 || len(description) > 4000 || now.IsZero() || now.Before(agent.UpdatedAt) {
		return invalidAgentf("Agent name, description, or update time is invalid")
	}
	if agent.Name == name && agent.Description == description {
		return nil
	}
	agent.Name = name
	agent.Description = description
	agent.UpdatedAt = now.UTC()
	agent.Version++
	return nil
}

func invalidAgentf(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidAgent, fmt.Sprintf(format, arguments...))
}
