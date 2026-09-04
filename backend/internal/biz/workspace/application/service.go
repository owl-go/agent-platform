package application

import (
	"context"
	"fmt"

	"agent-platform/backend/internal/biz/workspace/domain"
)

// Repository is the persistence port for Agent Workspace data. User-owned
// methods receive the owner explicitly; Model Catalog reads are platform-wide
// and its mutation scope is authorized at the service boundary.
type Repository interface {
	ListSessions(context.Context, string, bool) ([]domain.Session, error)
	CreateSession(context.Context, string, *string, *string) (domain.Session, error)
	GetSession(context.Context, string, string) (domain.Session, error)
	UpdateSession(context.Context, string, string, string, int64) (domain.Session, error)
	SetSessionArchived(context.Context, string, string, bool, int64) (domain.Session, error)
	SetSessionExpertSelection(context.Context, string, string, *string, *string, int64) (domain.Session, error)
	DeleteSession(context.Context, string, string) error
	ListMessages(context.Context, string, string, int64, int) ([]domain.Message, error)
	GetMessage(context.Context, string, string, int64) (domain.Message, error)
	CreateMessagePair(context.Context, string, string, string, []domain.Attachment) (domain.Message, domain.Message, error)
	RetryMessage(context.Context, string, string, int64) (domain.Message, domain.Message, error)
	CancelMessage(context.Context, string, string, int64) (domain.Message, error)

	ListWorkflows(context.Context, string, bool) ([]domain.Workflow, error)
	CreateWorkflow(context.Context, string, domain.WorkflowInput, []byte) (domain.Workflow, error)
	GetWorkflow(context.Context, string, string, bool) (domain.Workflow, error)
	UpdateWorkflow(context.Context, string, string, domain.WorkflowInput, []byte, int64) (domain.Workflow, error)
	DeleteWorkflow(context.Context, string, string) error
	SetWorkflowCredential(context.Context, string, string, string, string) (domain.Workflow, error)
	ResolveWorkflowCredential(context.Context, string, string) (string, string, error)
	SetWorkflowGitSource(context.Context, string, string, domain.GitSource, []byte) (domain.Workflow, error)
	GetWorkflowEnvironmentSecret(context.Context, string, string) ([]byte, error)
	ListRuns(context.Context, string, string) ([]domain.Run, error)
	ListRunTurns(context.Context, string, string, string) ([]domain.Run, error)
	GetRun(context.Context, string, string, string) (domain.Run, error)
	ListRunEvents(context.Context, string, string, string, int64, int) ([]domain.RunEvent, error)
	CreateRun(context.Context, string, string, string, *string, map[string]any) (domain.Run, error)
	ContinueRunConversation(context.Context, string, string, string, string, []domain.Attachment) (domain.Run, error)
	CreateRunIdempotent(context.Context, string, string, string, string, *string, map[string]any) (domain.Run, bool, error)
	CancelRun(context.Context, string, string, string) (domain.Run, error)
	Rerun(context.Context, string, string, string) (domain.Run, error)
	ListArtifacts(context.Context, string, string) ([]domain.Artifact, error)
	GetArtifact(context.Context, string, string, string) (domain.Artifact, error)

	ListExperts(context.Context, string) ([]domain.Expert, error)
	GetExpert(context.Context, string, string) (domain.Expert, error)
	CreateExpert(context.Context, string, domain.ExpertInput) (domain.Expert, error)
	UpdateExpert(context.Context, string, string, domain.ExpertInput, int64) (domain.Expert, error)
	DeleteExpert(context.Context, string, string) error
	ListExpertTeams(context.Context, string) ([]domain.ExpertTeam, error)
	GetExpertTeam(context.Context, string, string) (domain.ExpertTeam, error)
	CreateExpertTeam(context.Context, string, domain.ExpertTeamInput) (domain.ExpertTeam, error)
	UpdateExpertTeam(context.Context, string, string, domain.ExpertTeamInput, int64) (domain.ExpertTeam, error)
	DeleteExpertTeam(context.Context, string, string) error

	GetSettings(context.Context, string) (domain.Settings, error)
	UpdateSettings(context.Context, string, domain.Settings, int64) (domain.Settings, error)
	ListModelProviderConnections(context.Context) ([]domain.ModelProviderConnection, error)
	CreateModelProviderConnection(context.Context, string, domain.ModelProviderConnection, []byte, []domain.ProviderModel) (domain.ModelProviderConnection, error)
	UpdateModelProviderConnection(context.Context, string, string, domain.ModelProviderConnection, []byte, []domain.ProviderModel, int64) (domain.ModelProviderConnection, error)
	DeleteModelProviderConnection(context.Context, string, string) error
	GetModelProviderAPIKey(context.Context, string, string) ([]byte, error)
	ReplaceProviderModels(context.Context, string, string, []domain.ProviderModel, string, string) (domain.ModelProviderConnection, error)
	CreateProviderModel(context.Context, string, domain.ProviderModel) (domain.ProviderModel, error)

	ListMCPServers(context.Context, string) ([]domain.MCPServer, error)
	CreateMCPServer(context.Context, string, domain.MCPServer, []byte) (domain.MCPServer, error)
	UpdateMCPServer(context.Context, string, string, domain.MCPServer, []byte, int64) (domain.MCPServer, error)
	GetMCPSecret(context.Context, string, string) ([]byte, error)
	RequestMCPTest(context.Context, string, string) (domain.MCPServer, error)
	SetMCPTestResult(context.Context, string, string, string) (domain.MCPServer, error)
	DeleteMCPServer(context.Context, string, string) error
	ListSkills(context.Context, string) ([]domain.Skill, error)
	CreateSkill(context.Context, string, domain.Skill) (domain.Skill, error)
	UpdateSkill(context.Context, string, string, *string, string, string, int64) (domain.Skill, error)
	DeleteSkill(context.Context, string, string) error
}

type ModelCatalog interface {
	Discover(context.Context, domain.ModelProviderConnection, string) (ModelCatalogResult, error)
}

type ModelCatalogResult struct {
	Models []domain.ProviderModel
	Source string
}

type Service struct {
	repository Repository
	catalog    ModelCatalog
}

func New(repository Repository, catalogs ...ModelCatalog) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("Agent Workspace Repository is required")
	}
	service := &Service{repository: repository}
	if len(catalogs) > 0 {
		service.catalog = catalogs[0]
	}
	return service, nil
}

func (service *Service) Repository() Repository { return service.repository }

func (service *Service) DiscoverProviderModels(ctx context.Context, connection domain.ModelProviderConnection, apiKey string) (ModelCatalogResult, error) {
	if service.catalog == nil {
		return ModelCatalogResult{}, fmt.Errorf("Model Provider catalog is unavailable")
	}
	return service.catalog.Discover(ctx, connection, apiKey)
}
