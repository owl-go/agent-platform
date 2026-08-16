package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type TaskState string

const (
	TaskStateCreated        TaskState = "created"
	TaskStateActive         TaskState = "active"
	TaskStateWaitingForUser TaskState = "waiting_for_user"
	TaskStateCompleted      TaskState = "completed"
	TaskStateCancelled      TaskState = "cancelled"
)

var (
	ErrTaskNotFound     = errors.New("Coding Task not found")
	ErrSessionNotFound  = errors.New("Session not found")
	ErrConcurrentUpdate = errors.New("Collaboration resource was modified concurrently")
	ErrRunLimitReached  = errors.New("Session reached the maximum of 50 Runs")
)

type IssueSnapshot struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
}

type Task struct {
	ID             string
	OrganizationID string
	TeamID         string
	AgentReleaseID string
	CreatedBy      string
	Title          string
	RequestText    string
	IssueSnapshot  *IssueSnapshot
	State          TaskState
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
	Version        int64
}

type TaskRegistration struct {
	ID, OrganizationID, TeamID, AgentReleaseID, CreatedBy string
	Title, RequestText                                    string
	IssueSnapshot                                         *IssueSnapshot
	Now                                                   time.Time
}

func RegisterTask(registration TaskRegistration) (Task, error) {
	if strings.TrimSpace(registration.ID) == "" || strings.TrimSpace(registration.OrganizationID) == "" ||
		strings.TrimSpace(registration.TeamID) == "" || strings.TrimSpace(registration.AgentReleaseID) == "" ||
		strings.TrimSpace(registration.CreatedBy) == "" {
		return Task{}, fmt.Errorf("Coding Task identity, scope, Agent Release, and creator are required")
	}
	title := strings.TrimSpace(registration.Title)
	requestText := strings.TrimSpace(registration.RequestText)
	if title == "" || requestText == "" {
		return Task{}, fmt.Errorf("Coding Task title and request text are required")
	}
	if len(title) > 200 || len(requestText) > 100_000 {
		return Task{}, fmt.Errorf("Coding Task title or request text exceeds its limit")
	}
	if err := validateIssueSnapshot(registration.IssueSnapshot); err != nil {
		return Task{}, err
	}
	now := registration.Now.UTC()
	return Task{
		ID: registration.ID, OrganizationID: registration.OrganizationID, TeamID: registration.TeamID,
		AgentReleaseID: registration.AgentReleaseID, CreatedBy: registration.CreatedBy, Title: title,
		RequestText: requestText, IssueSnapshot: cloneIssue(registration.IssueSnapshot), State: TaskStateCreated,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func RestoreTask(task Task) (Task, error) {
	if task.Version <= 0 || task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		return Task{}, fmt.Errorf("invalid persisted Coding Task")
	}
	if _, err := ParseTaskState(string(task.State)); err != nil {
		return Task{}, err
	}
	if task.State == TaskStateCompleted && task.CompletedAt == nil {
		return Task{}, fmt.Errorf("completed Coding Task is missing completion time")
	}
	return task, nil
}

func ParseTaskState(value string) (TaskState, error) {
	state := TaskState(value)
	switch state {
	case TaskStateCreated, TaskStateActive, TaskStateWaitingForUser, TaskStateCompleted, TaskStateCancelled:
		return state, nil
	default:
		return "", fmt.Errorf("unknown Coding Task state %q", value)
	}
}

func (task *Task) Activate(now time.Time) error {
	if task.State != TaskStateCreated && task.State != TaskStateWaitingForUser {
		return fmt.Errorf("Coding Task in state %s cannot become active", task.State)
	}
	return task.transition(TaskStateActive, now)
}

func (task *Task) WaitForUser(now time.Time) error {
	if task.State != TaskStateActive {
		return fmt.Errorf("Coding Task in state %s cannot wait for user", task.State)
	}
	return task.transition(TaskStateWaitingForUser, now)
}

func (task *Task) Complete(now time.Time) error {
	if task.State != TaskStateActive && task.State != TaskStateWaitingForUser {
		return fmt.Errorf("Coding Task in state %s cannot be completed", task.State)
	}
	if err := task.transition(TaskStateCompleted, now); err != nil {
		return err
	}
	completed := now.UTC()
	task.CompletedAt = &completed
	return nil
}

func (task *Task) Cancel(now time.Time) error {
	if task.State == TaskStateCompleted || task.State == TaskStateCancelled {
		return fmt.Errorf("terminal Coding Task in state %s cannot be cancelled", task.State)
	}
	if err := task.transition(TaskStateCancelled, now); err != nil {
		return err
	}
	closed := now.UTC()
	task.CompletedAt = &closed
	return nil
}

func (task *Task) transition(state TaskState, now time.Time) error {
	task.State = state
	task.UpdatedAt = now.UTC()
	task.Version++
	return nil
}

func validateIssueSnapshot(snapshot *IssueSnapshot) error {
	if snapshot == nil {
		return nil
	}
	if strings.TrimSpace(snapshot.Title) == "" || len(snapshot.Title) > 500 || len(snapshot.Body) > 100_000 || len(snapshot.URL) > 2_000 {
		return fmt.Errorf("Issue Snapshot title is required and fields must remain within limits")
	}
	return nil
}

func cloneIssue(snapshot *IssueSnapshot) *IssueSnapshot {
	if snapshot == nil {
		return nil
	}
	copy := *snapshot
	return &copy
}
