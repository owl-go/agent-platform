package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/objectstore"
)

type connectorPackageRepository interface {
	CreateConnectorRevision(context.Context, domain.ConnectorRevision) (domain.ConnectorRevision, error)
	InstallConnector(context.Context, domain.ConnectorInstallation) (domain.ConnectorInstallation, error)
	InstallConnectorWithAudit(context.Context, domain.ConnectorInstallation, domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error)
	ListConnectorInstallations(context.Context, string) ([]domain.ConnectorInstallation, error)
	SetConnectorInstallationState(context.Context, string, string, domain.ConnectorInstallationState, int64) (domain.ConnectorInstallation, error)
	SetConnectorInstallationStateWithAudit(context.Context, string, string, domain.ConnectorInstallationState, int64, domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error)
	DisconnectConnectorInstallationAuthorization(context.Context, string, string, int64) (domain.ConnectorInstallation, error)
	DisconnectConnectorInstallationAuthorizationWithAudit(context.Context, string, string, int64, domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error)
	CreateConnectorAuthorization(context.Context, domain.ConnectorAuthorization) (domain.ConnectorAuthorization, error)
	CreateConnectorAuthorizationWithAudit(context.Context, domain.ConnectorAuthorization, domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error)
	RecordConnectorAudit(context.Context, domain.ConnectorAuditRecord) error
}

type guidedConnectorInput struct {
	Source, Version, Type, Name, Description, AuthMode string
	MCPJSON, CLIJSON, SkillName, SkillMarkdown         string
}

func (service *Service) connectorPackages() (connectorPackageRepository, error) {
	repository, ok := service.workspace.Repository().(connectorPackageRepository)
	if !ok {
		return nil, fmt.Errorf("Connector Package repository is unavailable")
	}
	return repository, nil
}

func (service *Service) ListConnectorInstallations(ctx context.Context, _ *workspacev1.ListConnectorInstallationsRequest) (*workspacev1.ListConnectorInstallationsResponse, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListConnectorInstallations(ctx, ownerID)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListConnectorInstallationsResponse{Items: make([]*workspacev1.ConnectorInstallation, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, connectorInstallationResponse(item))
	}
	return response, nil
}

func (service *Service) UploadConnectorPackage(ctx context.Context, request *workspacev1.UploadConnectorPackageRequest) (*workspacev1.ConnectorInstallation, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	pkg, err := connectorpackage.Parse(request.Archive)
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", domain.ErrInvalid, err))
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	revision, key := connectorRevisionFromPackage(pkg)
	if _, err := service.objects.Put(ctx, key, bytes.NewReader(pkg.NormalizedArchive), objectstore.PutOptions{Size: int64(len(pkg.NormalizedArchive)), SHA256: pkg.SHA256, ContentType: "application/zip", Metadata: map[string]string{"source": pkg.Metadata.Source, "version": pkg.Metadata.Version}}); err != nil {
		return nil, publicError(err)
	}
	revision.ObjectKey = key
	revision, err = repository.CreateConnectorRevision(ctx, revision)
	if err != nil {
		return nil, publicError(err)
	}
	installation, err := repository.InstallConnectorWithAudit(ctx, domain.ConnectorInstallation{OwnerID: ownerID, PackageSource: pkg.Metadata.Source, ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive}, domain.ConnectorAuditRecord{OwnerID: ownerID, RevisionID: revision.ID, Mode: domain.ConnectorMode(pkg.Metadata.Type), Operation: "install", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	return connectorInstallationResponse(installation), nil
}

func (service *Service) CreateConnectorPackage(ctx context.Context, request *workspacev1.CreateConnectorPackageRequest) (*workspacev1.ConnectorInstallation, error) {
	archive, err := buildGuidedConnectorPackage(guidedConnectorInput{Source: request.Source, Version: request.Version, Type: request.Type, Name: request.Name, Description: request.Description, AuthMode: request.AuthMode, MCPJSON: request.McpJson, CLIJSON: request.CliJson, SkillName: request.SkillName, SkillMarkdown: request.SkillMarkdown})
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", domain.ErrInvalid, err))
	}
	return service.UploadConnectorPackage(ctx, &workspacev1.UploadConnectorPackageRequest{Archive: archive})
}

func (service *Service) DisableConnectorInstallation(ctx context.Context, request *workspacev1.DisableConnectorInstallationRequest) (*workspacev1.ConnectorInstallation, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.SetConnectorInstallationStateWithAudit(ctx, ownerID, request.InstallationId, domain.ConnectorInstallationDisabled, request.ExpectedVersion, domain.ConnectorAuditRecord{OwnerID: ownerID, InstallationID: request.InstallationId, Operation: "disable", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	return connectorInstallationResponse(item), nil
}

func (service *Service) UninstallConnector(ctx context.Context, request *workspacev1.UninstallConnectorRequest) (*workspacev1.DeleteResponse, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	_, err = repository.SetConnectorInstallationStateWithAudit(ctx, ownerID, request.InstallationId, domain.ConnectorInstallationUninstalled, request.ExpectedVersion, domain.ConnectorAuditRecord{OwnerID: ownerID, InstallationID: request.InstallationId, Operation: "uninstall", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) ConnectConnector(ctx context.Context, request *workspacev1.ConnectConnectorRequest) (*workspacev1.ConnectorInstallation, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.IdentityRef) == "" || len(request.CredentialsJson) == 0 || len(request.CredentialsJson) > 64*1024 || !json.Valid(request.CredentialsJson) {
		return nil, publicError(fmt.Errorf("%w: connector authorization requires a bounded identity and credentials JSON", domain.ErrInvalid))
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	ciphertext, err := service.box.Encrypt(request.CredentialsJson, "connector-authorization:"+ownerID)
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListConnectorInstallations(ctx, ownerID)
	if err != nil {
		return nil, publicError(err)
	}
	var installation domain.ConnectorInstallation
	for _, item := range items {
		if item.ID == request.InstallationId {
			installation = item
			break
		}
	}
	if installation.ID == "" {
		return nil, publicError(domain.ErrNotFound)
	}
	_, err = repository.CreateConnectorAuthorizationWithAudit(ctx, domain.ConnectorAuthorization{OwnerID: ownerID, InstallationID: request.InstallationId, IdentityRef: strings.TrimSpace(request.IdentityRef), Scopes: append([]string(nil), request.Scopes...), CredentialCiphertext: ciphertext, State: domain.ConnectorAuthorizationActive}, domain.ConnectorAuditRecord{OwnerID: ownerID, InstallationID: installation.ID, RevisionID: installation.ActiveRevisionID, Operation: "authorize", IdentityRef: strings.TrimSpace(request.IdentityRef), Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	items, err = repository.ListConnectorInstallations(ctx, ownerID)
	if err != nil {
		return nil, publicError(err)
	}
	for _, item := range items {
		if item.ID == request.InstallationId {
			return connectorInstallationResponse(item), nil
		}
	}
	return nil, publicError(fmt.Errorf("%w: connector installation not found after authorization", domain.ErrNotFound))
}

func (service *Service) DisconnectConnectorAuthorization(ctx context.Context, request *workspacev1.DisconnectConnectorAuthorizationRequest) (*workspacev1.ConnectorInstallation, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.DisconnectConnectorInstallationAuthorizationWithAudit(ctx, ownerID, request.InstallationId, request.ExpectedVersion, domain.ConnectorAuditRecord{OwnerID: ownerID, InstallationID: request.InstallationId, Operation: "disconnect", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	return connectorInstallationResponse(item), nil
}

func connectorRevisionFromPackage(pkg connectorpackage.Package) (domain.ConnectorRevision, string) {
	policy, _ := json.Marshal(map[string]any{"auth_mode": pkg.Metadata.AuthMode, "mcp": pkg.MCP, "cli": pkg.CLI})
	revision := domain.ConnectorRevision{PackageSource: pkg.Metadata.Source, Version: pkg.Metadata.Version, Mode: domain.ConnectorMode(pkg.Metadata.Type), PackageSHA256: pkg.SHA256, RuntimePolicy: policy}
	return revision, fmt.Sprintf("connectors/%s/%s/%s.zip", pkg.Metadata.Source, pkg.Metadata.Version, pkg.SHA256)
}

func buildGuidedConnectorPackage(input guidedConnectorInput) ([]byte, error) {
	if input.Source == "" || input.Version == "" || input.Type == "" || input.Name == "" || input.Description == "" || input.SkillName == "" || strings.TrimSpace(input.SkillMarkdown) == "" {
		return nil, fmt.Errorf("guided Connector Package requires source, version, type, name, description, skill name, and instructions")
	}
	if input.AuthMode == "" {
		input.AuthMode = "none"
	}
	meta, err := json.Marshal(map[string]any{"source": input.Source, "version": input.Version, "type": input.Type, "name": input.Name, "description": input.Description, "examples_zh": []string{"使用连接器"}, "examples_en": []string{"Use this connector"}, "minPlatformVersion": "1.0.0", "auth_mode": input.AuthMode})
	if err != nil {
		return nil, err
	}
	manifestName, manifest := "mcp.json", input.MCPJSON
	if input.Type == "cli" {
		manifestName, manifest = "cli.json", input.CLIJSON
	}
	if strings.TrimSpace(manifest) == "" {
		return nil, fmt.Errorf("guided Connector Package requires %s", manifestName)
	}
	skill := input.SkillMarkdown
	if !strings.HasPrefix(skill, "---\n") {
		skill = fmt.Sprintf("---\nname: %s\ndisplay_name: %s\ndescription: %s\nversion: %s\nauthor: guided\n---\n\n%s\n", input.SkillName, input.Name, input.Description, input.Version, strings.TrimSpace(skill))
	}
	files := map[string][]byte{"connector-meta.json": meta, "icon.svg": []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"), manifestName: []byte(manifest), "skills/" + input.SkillName + "/SKILL.md": []byte(skill)}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(body); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func connectorInstallationResponse(item domain.ConnectorInstallation) *workspacev1.ConnectorInstallation {
	return &workspacev1.ConnectorInstallation{Id: item.ID, Source: item.PackageSource, ActiveRevisionId: item.ActiveRevisionID, State: string(item.State), Authorized: item.AuthorizationID != "", Version: item.Version}
}
