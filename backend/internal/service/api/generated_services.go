package api

import (
	"fmt"

	agentlifecyclev1 "agent-platform/backend/api/agentlifecycle/v1"
	approvalv1 "agent-platform/backend/api/approval/v1"
	artifactv1 "agent-platform/backend/api/artifact/v1"
	auditv1 "agent-platform/backend/api/audit/v1"
	collaborationv1 "agent-platform/backend/api/collaboration/v1"
	executionv1 "agent-platform/backend/api/execution/v1"
	identityv1 "agent-platform/backend/api/identity/v1"
	modelcatalogv1 "agent-platform/backend/api/modelcatalog/v1"
	runtimecatalogv1 "agent-platform/backend/api/runtimecatalog/v1"
	sourcecontrolv1 "agent-platform/backend/api/sourcecontrol/v1"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type GeneratedServices struct {
	dependencies Dependencies
}

func NewGeneratedServices(dependencies Dependencies) (*GeneratedServices, error) {
	if err := dependencies.validateProtectedAPI(); err != nil {
		return nil, err
	}
	return &GeneratedServices{dependencies: dependencies}, nil
}

func (service *GeneratedServices) RegisterHTTP(server *kratoshttp.Server) {
	agentlifecyclev1.RegisterAgentLifecycleServiceHTTPServer(server, service)
	approvalv1.RegisterRunApprovalServiceHTTPServer(server, service)
	artifactv1.RegisterArtifactServiceHTTPServer(server, service)
	auditv1.RegisterAuditServiceHTTPServer(server, service)
	collaborationv1.RegisterCollaborationServiceHTTPServer(server, service)
	executionv1.RegisterExecutionServiceHTTPServer(server, service)
	identityv1.RegisterIdentityServiceHTTPServer(server, service)
	modelcatalogv1.RegisterModelCatalogServiceHTTPServer(server, service)
	runtimecatalogv1.RegisterRuntimeCatalogServiceHTTPServer(server, service)
	sourcecontrolv1.RegisterSourceControlServiceHTTPServer(server, service)
}

func (dependencies Dependencies) validateProtectedAPI() error {
	if dependencies.Database == nil || dependencies.Runs == nil || dependencies.RunSearch == nil || dependencies.RunControls == nil ||
		dependencies.Approvals == nil || dependencies.Artifacts == nil || dependencies.Audit == nil || dependencies.RuntimeImages == nil ||
		dependencies.ModelCatalog == nil || dependencies.SourceControl == nil || dependencies.RepositoryBindings == nil ||
		dependencies.AgentLifecycle == nil || dependencies.Collaboration == nil || dependencies.CatalogWrites == nil {
		return fmt.Errorf("all API query, command, and Unit of Work dependencies are required")
	}
	if dependencies.Access == nil || dependencies.ResourceAccess == nil || dependencies.CurrentUserAccess == nil || dependencies.RunSearchAccess == nil ||
		dependencies.RunControlAccess == nil || dependencies.AuditAccess == nil || dependencies.RuntimeAccess == nil ||
		dependencies.ModelAccess == nil || dependencies.AgentAccess == nil || dependencies.CollaborationAccess == nil ||
		dependencies.CatalogWriteAccess == nil {
		return fmt.Errorf("Token Verifier-backed Identity authorization dependencies are required")
	}
	return nil
}
