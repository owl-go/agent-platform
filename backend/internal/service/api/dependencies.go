package api

import (
	"context"
	"time"

	agentdomain "agent-platform/backend/internal/biz/agentlifecycle/domain"
	approvaldomain "agent-platform/backend/internal/biz/approval/domain"
	artifactdomain "agent-platform/backend/internal/biz/artifact/domain"
	auditdomain "agent-platform/backend/internal/biz/audit/domain"
	"agent-platform/backend/internal/biz/authz"
	collaborationdomain "agent-platform/backend/internal/biz/collaboration/domain"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	modeldomain "agent-platform/backend/internal/biz/modelcatalog/domain"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	runtimedomain "agent-platform/backend/internal/biz/runtimecatalog/domain"
	sourcedomain "agent-platform/backend/internal/biz/sourcecontrol/domain"
	transaction "agent-platform/backend/internal/biz/transaction"
	"agent-platform/backend/internal/objectstore"
)

type ReadinessChecker interface{ PingContext(context.Context) error }
type EventReader interface {
	ListEventsAfter(context.Context, string, int64, int) ([]executiondomain.Event, error)
}
type RunReader interface {
	Get(context.Context, string) (executiondomain.Details, error)
}
type RunSearcher interface {
	Search(context.Context, executiondomain.SearchQuery) ([]executiondomain.Details, error)
}
type RunSearchAccessController interface {
	AuthorizeTeamRead(context.Context, string, string) (identitydomain.Actor, error)
}
type RunController interface {
	Control(context.Context, string, int64, executiondomain.ControlAction, string) (executiondomain.Details, error)
}
type RunAccessController interface {
	AuthorizeRunRead(context.Context, string, string) error
}
type ResourceScopeController interface {
	ResolveReadScope(context.Context, string) (authz.ReadScope, error)
}
type CurrentUserAccessController interface {
	CurrentUser(context.Context) (identitydomain.Principal, error)
}
type RunControlAccessController interface {
	AuthorizeRunControl(context.Context, string, string, string) (identitydomain.Actor, error)
}
type ApprovalService interface {
	Get(context.Context, string) (approvaldomain.Approval, error)
	GetInScope(context.Context, string, authz.ReadScope) (approvaldomain.Approval, error)
	ListByRun(context.Context, string) ([]approvaldomain.Approval, error)
}
type ArtifactService interface {
	Get(context.Context, string) (artifactdomain.Artifact, error)
	GetInScope(context.Context, string, authz.ReadScope) (artifactdomain.Artifact, error)
	ListByRun(context.Context, string) ([]artifactdomain.Artifact, error)
	PresignDownload(context.Context, artifactdomain.Artifact, time.Duration) (objectstore.SignedURL, error)
}
type AuditSearcher interface {
	Search(context.Context, auditdomain.Query) ([]auditdomain.Event, error)
}
type RuntimeImageReader interface {
	Get(context.Context, string, string) (runtimedomain.RuntimeImage, error)
	List(context.Context, runtimeapplication.ListQuery) (runtimeapplication.Page, error)
}
type RuntimeImageAccessController interface {
	AuthorizeRuntimeImageRead(context.Context, string) (identitydomain.Actor, error)
}
type ModelCatalogReader interface {
	GetCredential(context.Context, string, string) (modeldomain.CredentialProfile, error)
	ListCredentials(context.Context, string) ([]modeldomain.CredentialProfile, error)
	GetModel(context.Context, string, string) (modeldomain.ConfiguredModel, error)
	ListModels(context.Context, string) ([]modeldomain.ConfiguredModel, error)
}
type ModelCatalogAccessController interface {
	AuthorizeModelCatalogRead(context.Context, string) (identitydomain.Actor, error)
}
type SourceControlReader interface {
	Get(context.Context, string, string) (sourcedomain.Provider, error)
	List(context.Context, string) ([]sourcedomain.Provider, error)
}
type CatalogWriteAccessController interface {
	AuthorizeModelCatalogWrite(context.Context, string) (identitydomain.Actor, error)
}
type RepositoryBindingReader interface {
	Get(context.Context, string, string, string) (sourcedomain.RepositoryBinding, error)
	List(context.Context, string, string) ([]sourcedomain.RepositoryBinding, error)
}
type AgentLifecycleAccessController interface {
	AuthorizeTeamRead(context.Context, string, string) (identitydomain.Actor, error)
	AuthorizeAgentBuild(context.Context, string, string) (identitydomain.Actor, error)
}
type AgentLifecycleReader interface {
	GetAgent(context.Context, string, string, string) (agentdomain.Agent, error)
	ListAgents(context.Context, string, string) ([]agentdomain.Agent, error)
	GetDraft(context.Context, string, string, string, string) (agentdomain.Draft, error)
	ListDrafts(context.Context, string, string, string) ([]agentdomain.Draft, error)
	GetRelease(context.Context, string, string, string, string) (agentdomain.Release, error)
	ListReleases(context.Context, string, string, string) ([]agentdomain.Release, error)
}
type CollaborationAccessController interface {
	AuthorizeTeamRead(context.Context, string, string) (identitydomain.Actor, error)
	AuthorizeTaskUse(context.Context, string, string) (identitydomain.Actor, error)
}
type CollaborationReader interface {
	GetTask(context.Context, string, string, string) (collaborationdomain.Task, error)
	ListTasks(context.Context, string, string) ([]collaborationdomain.Task, error)
	GetSession(context.Context, string, string, string) (collaborationdomain.Session, error)
	ListMessages(context.Context, string, string, string, int64, int) ([]collaborationdomain.Message, error)
	ListMemoryCandidates(context.Context, string, string, string) ([]collaborationdomain.MemoryCandidate, error)
	ListAgentMemories(context.Context, string, string, string, bool) ([]collaborationdomain.AgentMemory, error)
}

type Dependencies struct {
	Database            ReadinessChecker
	Events              EventReader
	Runs                RunReader
	RunSearch           RunSearcher
	RunSearchAccess     RunSearchAccessController
	Access              RunAccessController
	ResourceAccess      ResourceScopeController
	CurrentUserAccess   CurrentUserAccessController
	RunControls         RunController
	RunControlAccess    RunControlAccessController
	Approvals           ApprovalService
	Artifacts           ArtifactService
	Audit               AuditSearcher
	AuditAccess         RunSearchAccessController
	RuntimeImages       RuntimeImageReader
	RuntimeAccess       RuntimeImageAccessController
	ModelCatalog        ModelCatalogReader
	ModelAccess         ModelCatalogAccessController
	SourceControl       SourceControlReader
	RepositoryBindings  RepositoryBindingReader
	AgentAccess         AgentLifecycleAccessController
	AgentLifecycle      AgentLifecycleReader
	Collaboration       CollaborationReader
	CollaborationAccess CollaborationAccessController
	CatalogWrites       transaction.IdempotentTransactionManager
	CatalogWriteAccess  CatalogWriteAccessController
}
