package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/collaboration/domain"

	"github.com/google/uuid"
)

type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID() string }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

type Service struct {
	repository domain.Repository
	launches   domain.LaunchCoordinator
	clock      Clock
	ids        IDGenerator
}

func New(repository domain.Repository) *Service {
	launches, _ := repository.(domain.LaunchCoordinator)
	return NewWithDependencies(repository, launches, systemClock{}, uuidGenerator{})
}

func NewWithLaunchCoordinator(repository domain.Repository, launches domain.LaunchCoordinator) *Service {
	return NewWithDependencies(repository, launches, systemClock{}, uuidGenerator{})
}

func NewWithDependencies(repository domain.Repository, launches domain.LaunchCoordinator, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, launches: launches, clock: clock, ids: ids}
}

type CreateTaskCommand struct {
	OrganizationID, TeamID, AgentReleaseID, CreatedBy string
	Title, RequestText                                string
	IssueSnapshot                                     *domain.IssueSnapshot
}

func (service *Service) CreateTask(ctx context.Context, command CreateTaskCommand) (domain.Launch, error) {
	if err := service.dependencies(); err != nil {
		return domain.Launch{}, err
	}
	now := service.clock.Now()
	taskID := service.ids.NewID()
	task, err := domain.RegisterTask(domain.TaskRegistration{
		ID: taskID, OrganizationID: command.OrganizationID, TeamID: command.TeamID,
		AgentReleaseID: command.AgentReleaseID, CreatedBy: command.CreatedBy,
		Title: command.Title, RequestText: command.RequestText, IssueSnapshot: command.IssueSnapshot, Now: now,
	})
	if err != nil {
		return domain.Launch{}, err
	}
	branchSuffix := strings.ReplaceAll(taskID, "-", "")
	if len(branchSuffix) > 16 {
		branchSuffix = branchSuffix[:16]
	}
	return service.launches.CreateLaunch(ctx, domain.LaunchRegistration{
		Task: task, SessionID: service.ids.NewID(),
		Run:          domain.RunSeed{ID: service.ids.NewID(), RequestText: task.RequestText, CreatedBy: command.CreatedBy},
		ReviewBranch: "agent-platform/backend/task-" + branchSuffix, WorkspaceVolume: "agent-platform-session-" + service.ids.NewID(), Now: now,
	})
}

type ContinueTaskCommand struct {
	OrganizationID, TeamID, TaskID, CreatedBy, RequestText string
	ExpectedTaskVersion, ExpectedSessionVersion            int64
}

func (service *Service) ContinueTask(ctx context.Context, command ContinueTaskCommand) (domain.Launch, error) {
	if err := service.dependencies(); err != nil {
		return domain.Launch{}, err
	}
	if strings.TrimSpace(command.RequestText) == "" || len(command.RequestText) > 100_000 || command.CreatedBy == "" {
		return domain.Launch{}, fmt.Errorf("continuation request and creator are required")
	}
	return service.launches.Continue(ctx, domain.ContinueRegistration{
		OrganizationID: command.OrganizationID, TeamID: command.TeamID, TaskID: command.TaskID,
		Run:                 domain.RunSeed{ID: service.ids.NewID(), RequestText: strings.TrimSpace(command.RequestText), CreatedBy: command.CreatedBy},
		ExpectedTaskVersion: command.ExpectedTaskVersion, ExpectedSessionVersion: command.ExpectedSessionVersion, Now: service.clock.Now(),
	})
}

func (service *Service) GetTask(ctx context.Context, organizationID, teamID, taskID string) (domain.Task, error) {
	if service.repository == nil || organizationID == "" || teamID == "" || taskID == "" {
		return domain.Task{}, fmt.Errorf("Collaboration Repository and Coding Task scope are required")
	}
	return service.repository.GetTask(ctx, organizationID, teamID, taskID)
}

func (service *Service) ListTasks(ctx context.Context, organizationID, teamID string) ([]domain.Task, error) {
	if service.repository == nil || organizationID == "" || teamID == "" {
		return nil, fmt.Errorf("Collaboration Repository and Team scope are required")
	}
	return service.repository.ListTasks(ctx, organizationID, teamID)
}

func (service *Service) ListLaunchOptions(ctx context.Context, organizationID, teamID string) (domain.LaunchCatalog, error) {
	if service.repository == nil || organizationID == "" || teamID == "" {
		return domain.LaunchCatalog{}, fmt.Errorf("Collaboration Repository and Team scope are required")
	}
	return service.repository.ListLaunchOptions(ctx, organizationID, teamID)
}

func (service *Service) GetSession(ctx context.Context, organizationID, teamID, taskID string) (domain.Session, error) {
	if _, err := service.GetTask(ctx, organizationID, teamID, taskID); err != nil {
		return domain.Session{}, err
	}
	return service.repository.GetSessionByTask(ctx, taskID)
}

func (service *Service) ListMessages(ctx context.Context, organizationID, teamID, taskID string, afterID int64, limit int) ([]domain.Message, error) {
	session, err := service.GetSession(ctx, organizationID, teamID, taskID)
	if err != nil {
		return nil, err
	}
	if afterID < 0 || limit <= 0 || limit > 200 {
		return nil, fmt.Errorf("message cursor or limit is invalid")
	}
	return service.repository.ListMessages(ctx, session.ID, afterID, limit)
}

func (service *Service) UpdateSessionMemory(ctx context.Context, organizationID, teamID, taskID string, memory domain.SessionMemory, expectedVersion int64) (domain.Session, error) {
	session, err := service.GetSession(ctx, organizationID, teamID, taskID)
	if err != nil {
		return domain.Session{}, err
	}
	if expectedVersion <= 0 || session.Version != expectedVersion {
		return domain.Session{}, domain.ErrConcurrentUpdate
	}
	if err := session.UpdateMemory(memory, service.clock.Now()); err != nil {
		return domain.Session{}, err
	}
	if err := service.repository.UpdateSession(ctx, session, expectedVersion); err != nil {
		return domain.Session{}, err
	}
	return session, nil
}

func (service *Service) ChangeTaskState(ctx context.Context, organizationID, teamID, taskID string, expectedVersion int64, state domain.TaskState) (domain.Task, error) {
	task, err := service.GetTask(ctx, organizationID, teamID, taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if expectedVersion <= 0 || task.Version != expectedVersion {
		return domain.Task{}, domain.ErrConcurrentUpdate
	}
	now := service.clock.Now()
	switch state {
	case domain.TaskStateWaitingForUser:
		err = task.WaitForUser(now)
	case domain.TaskStateCompleted:
		err = task.Complete(now)
	case domain.TaskStateCancelled:
		err = task.Cancel(now)
	default:
		err = fmt.Errorf("unsupported explicit Coding Task state %s", state)
	}
	if err != nil {
		return domain.Task{}, err
	}
	if err := service.repository.UpdateTask(ctx, task, expectedVersion); err != nil {
		return domain.Task{}, err
	}
	return task, nil
}

func (service *Service) ProposeMemory(ctx context.Context, organizationID, teamID, taskID, agentID, content string) (domain.MemoryCandidate, error) {
	if _, err := service.GetTask(ctx, organizationID, teamID, taskID); err != nil {
		return domain.MemoryCandidate{}, err
	}
	candidate, err := domain.ProposeMemory(service.ids.NewID(), agentID, taskID, content, service.clock.Now())
	if err != nil {
		return domain.MemoryCandidate{}, err
	}
	if err := service.repository.CreateMemoryCandidate(ctx, candidate); err != nil {
		return domain.MemoryCandidate{}, err
	}
	return candidate, nil
}

func (service *Service) DecideMemory(ctx context.Context, organizationID, teamID, candidateID, decidedBy string, approve bool) (domain.MemoryCandidate, *domain.AgentMemory, error) {
	candidate, err := service.repository.GetMemoryCandidate(ctx, organizationID, teamID, candidateID)
	if err != nil {
		return domain.MemoryCandidate{}, nil, err
	}
	var memory *domain.AgentMemory
	if approve {
		created, err := candidate.Approve(service.ids.NewID(), decidedBy, service.clock.Now())
		if err != nil {
			return domain.MemoryCandidate{}, nil, err
		}
		memory = &created
	} else if err := candidate.Reject(decidedBy, service.clock.Now()); err != nil {
		return domain.MemoryCandidate{}, nil, err
	}
	if err := service.repository.DecideMemoryCandidate(ctx, candidate, memory); err != nil {
		return domain.MemoryCandidate{}, nil, err
	}
	return candidate, memory, nil
}

func (service *Service) ListMemoryCandidates(ctx context.Context, organizationID, teamID, taskID string) ([]domain.MemoryCandidate, error) {
	if _, err := service.GetTask(ctx, organizationID, teamID, taskID); err != nil {
		return nil, err
	}
	return service.repository.ListMemoryCandidates(ctx, organizationID, teamID, taskID)
}

func (service *Service) ListAgentMemories(ctx context.Context, organizationID, teamID, agentID string, includeDeleted bool) ([]domain.AgentMemory, error) {
	if service.repository == nil || organizationID == "" || teamID == "" || agentID == "" {
		return nil, fmt.Errorf("Collaboration Repository and Agent scope are required")
	}
	return service.repository.ListAgentMemories(ctx, organizationID, teamID, agentID, includeDeleted)
}

func (service *Service) UpdateAgentMemory(ctx context.Context, organizationID, teamID, memoryID, content string, enabled bool, expectedVersion int64) (domain.AgentMemory, error) {
	memory, err := service.repository.GetAgentMemory(ctx, organizationID, teamID, memoryID)
	if err != nil {
		return domain.AgentMemory{}, err
	}
	if expectedVersion <= 0 || memory.Version != expectedVersion {
		return domain.AgentMemory{}, domain.ErrConcurrentUpdate
	}
	if err := memory.Edit(content, enabled, service.clock.Now()); err != nil {
		return domain.AgentMemory{}, err
	}
	if err := service.repository.UpdateAgentMemory(ctx, memory, expectedVersion); err != nil {
		return domain.AgentMemory{}, err
	}
	return memory, nil
}

func (service *Service) DeleteAgentMemory(ctx context.Context, organizationID, teamID, memoryID string, expectedVersion int64) (domain.AgentMemory, error) {
	memory, err := service.repository.GetAgentMemory(ctx, organizationID, teamID, memoryID)
	if err != nil {
		return domain.AgentMemory{}, err
	}
	if expectedVersion <= 0 || memory.Version != expectedVersion {
		return domain.AgentMemory{}, domain.ErrConcurrentUpdate
	}
	if err := memory.Delete(service.clock.Now()); err != nil {
		return domain.AgentMemory{}, err
	}
	if err := service.repository.UpdateAgentMemory(ctx, memory, expectedVersion); err != nil {
		return domain.AgentMemory{}, err
	}
	return memory, nil
}

func (service *Service) dependencies() error {
	if service.repository == nil || service.launches == nil || service.clock == nil || service.ids == nil {
		return fmt.Errorf("Collaboration Service dependencies are required")
	}
	return nil
}
