package apiprocess

import (
	"net/http"

	agentapplication "agent-platform/backend/internal/biz/agentlifecycle/application"
	approvalapplication "agent-platform/backend/internal/biz/approval/application"
	artifactapplication "agent-platform/backend/internal/biz/artifact/application"
	auditapplication "agent-platform/backend/internal/biz/audit/application"
	collaborationapplication "agent-platform/backend/internal/biz/collaboration/application"
	executionapplication "agent-platform/backend/internal/biz/execution/application"
	identityapplication "agent-platform/backend/internal/biz/identity/application"
	modelapplication "agent-platform/backend/internal/biz/modelcatalog/application"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	sourceapplication "agent-platform/backend/internal/biz/sourcecontrol/application"
	transaction "agent-platform/backend/internal/biz/transaction"
	"agent-platform/backend/internal/infrastructure/gormdb"
	platformserver "agent-platform/backend/internal/server"
	apiservice "agent-platform/backend/internal/service/api"
	executionservice "agent-platform/backend/internal/service/execution"
	identityservice "agent-platform/backend/internal/service/identity"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewDependencies,
	NewGeneratedServices,
	NewRunSSEHandler,
	NewAuthenticationFilter,
	NewHTTPHandlers,
)

func NewDependencies(
	database *gormdb.Database,
	runs *executionapplication.Service,
	access *identityapplication.AccessService,
	approvals *approvalapplication.Service,
	artifacts *artifactapplication.Service,
	audit *auditapplication.Service,
	runtimeImages *runtimeapplication.Service,
	models *modelapplication.Service,
	source *sourceapplication.Service,
	bindings *sourceapplication.BindingService,
	agents *agentapplication.Service,
	collaboration *collaborationapplication.Service,
	writes transaction.IdempotentTransactionManager,
) apiservice.Dependencies {
	return apiservice.Dependencies{
		Database: database, Events: runs, Runs: runs, RunSearch: runs, RunControls: runs,
		Approvals: approvals, Artifacts: artifacts, Audit: audit, RuntimeImages: runtimeImages,
		ModelCatalog: models, SourceControl: source, RepositoryBindings: bindings,
		AgentLifecycle: agents, Collaboration: collaboration, CatalogWrites: writes,
		RunSearchAccess: access, Access: access, ResourceAccess: access, RunControlAccess: access,
		CurrentUserAccess: access,
		AuditAccess:       access, RuntimeAccess: access, ModelAccess: access, AgentAccess: access,
		CollaborationAccess: access, CatalogWriteAccess: access,
	}
}

func NewGeneratedServices(dependencies apiservice.Dependencies) (*apiservice.GeneratedServices, error) {
	return apiservice.NewGeneratedServices(dependencies)
}

type RunSSEHandler struct{ http.Handler }

func NewRunSSEHandler(runs *executionapplication.Service, access *identityapplication.AccessService) (RunSSEHandler, error) {
	handler, err := executionservice.NewRunEventSSE(runs, access)
	return RunSSEHandler{Handler: handler}, err
}

func NewAuthenticationFilter(access *identityapplication.AccessService) (kratoshttp.FilterFunc, error) {
	return identityservice.NewAuthenticationFilter(access)
}

func NewHTTPHandlers(business *apiservice.GeneratedServices, sse RunSSEHandler, authentication kratoshttp.FilterFunc) (platformserver.HTTPHandlers, error) {
	return platformserver.NewHTTPHandlers(business, sse.Handler, authentication)
}
