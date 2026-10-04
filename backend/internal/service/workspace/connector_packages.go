package workspace

import (
	"agent-platform/backend/internal/xiaoemcp"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/dingtalkcli"
	"agent-platform/backend/internal/feishucli"
	"agent-platform/backend/internal/githubcli"
	"agent-platform/backend/internal/klingmcp"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/tianyanchamcp"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type connectorPackageRepository interface {
	CreateConnectorRevision(context.Context, domain.ConnectorRevision) (domain.ConnectorRevision, error)
	GetConnectorRevision(context.Context, string) (domain.ConnectorRevision, error)
	ListConnectorRevisions(context.Context) ([]domain.ConnectorRevision, error)
	PublishConnectorRevision(context.Context, string, string, int64) (domain.ConnectorPublication, error)
	SetConnectorPublicationState(context.Context, string, string, domain.ConnectorPublicationState, int64) (domain.ConnectorPublication, error)
	ListConnectorPublications(context.Context, bool) ([]domain.ConnectorPublication, error)
	ListConnectorPublicationHealth(context.Context) ([]domain.ConnectorPublicationHealth, error)
	HasConnectorBundleRuntimeConformance(context.Context, string, string) (bool, error)
	InstallConnector(context.Context, domain.ConnectorInstallation) (domain.ConnectorInstallation, error)
	InstallConnectorWithAudit(context.Context, domain.ConnectorInstallation, domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error)
	ListConnectorInstallations(context.Context, string) ([]domain.ConnectorInstallation, error)
	SetConnectorInstallationState(context.Context, string, string, domain.ConnectorInstallationState, int64) (domain.ConnectorInstallation, error)
	SetConnectorInstallationStateWithAudit(context.Context, string, string, domain.ConnectorInstallationState, int64, domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error)
	DisconnectConnectorInstallationAuthorization(context.Context, string, string, int64) (domain.ConnectorInstallation, error)
	DisconnectConnectorInstallationAuthorizationWithAudit(context.Context, string, string, int64, domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error)
	CreateConnectorAuthorization(context.Context, domain.ConnectorAuthorization) (domain.ConnectorAuthorization, error)
	CreateConnectorAuthorizationWithAudit(context.Context, domain.ConnectorAuthorization, domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error)
	ListConnectorAuthorizations(context.Context, string, string) ([]domain.ConnectorAuthorization, error)
	SelectConnectorAuthorizationWithAudit(context.Context, string, string, string, int64, domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error)
	RefreshConnectorAuthorization(context.Context, domain.ConnectorAuthorization, int64, domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error)
	DisconnectConnectorAuthorization(context.Context, string, string) (domain.ConnectorAuthorization, error)
	DisconnectConnectorAuthorizationWithAudit(context.Context, string, string, domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error)
	ActivateConnectorRevision(context.Context, string, string, string, time.Time) (domain.ConnectorInstallation, error)
	ActivateConnectorRevisionWithVersion(context.Context, string, string, string, int64, time.Time) (domain.ConnectorInstallation, error)
	BeginConnectorSetup(context.Context, domain.ConnectorSetup) (domain.ConnectorSetup, error)
	GetConnectorSetup(context.Context, string, string) (domain.ConnectorSetup, error)
	GetConnectorProviderApplication(context.Context, string, string) (domain.ConnectorProviderApplication, error)
	CompleteConnectorSetup(context.Context, string, string, domain.ConnectorProviderApplication) (domain.ConnectorSetup, error)
	BeginConnectorAuthorizationFlow(context.Context, domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error)
	UpdateConnectorAuthorizationFlow(context.Context, string, string, []byte, []byte, string) error
	ConsumeConnectorAuthorizationFlow(context.Context, string, string, []byte) error
	GetConnectorAuthorizationFlow(context.Context, string, string) (domain.ConnectorAuthorizationAttempt, error)
	DeleteConnectorAuthorizationFlow(context.Context, string, string) error
	RecordConnectorAudit(context.Context, domain.ConnectorAuditRecord) error
}

type connectorRevisionPolicy struct {
	AuthMode         string                        `json:"auth_mode"`
	Metadata         connectorpackage.Metadata     `json:"metadata"`
	CLI              *connectorpackage.CLIManifest `json:"cli"`
	MCP              *connectorpackage.MCPManifest `json:"mcp"`
	BundleObjectKey  string                        `json:"cli_bundle_object_key"`
	BundleSHA256     string                        `json:"cli_bundle_sha256"`
	LegacyProjection bool                          `json:"legacy_projection"`
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
	publications, _ := repository.ListConnectorPublications(ctx, false)
	publishedRevisions := make(map[string]string, len(publications))
	for _, publication := range publications {
		publishedRevisions[publication.PackageSource] = publication.ActiveRevisionID
	}
	for _, item := range items {
		revision, readErr := repository.GetConnectorRevision(ctx, item.ActiveRevisionID)
		if readErr != nil {
			return nil, publicError(readErr)
		}
		response.Items = append(response.Items, connectorInstallationDetailsResponse(item, revision, publishedRevisions[item.PackageSource] != "" && publishedRevisions[item.PackageSource] != item.ActiveRevisionID))
	}
	return response, nil
}

func (service *Service) ListConnectorPublications(ctx context.Context, _ *workspacev1.ListConnectorPublicationsRequest) (*workspacev1.ListConnectorPublicationsResponse, error) {
	if _, err := service.owner(ctx); err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListConnectorPublications(ctx, false)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListConnectorPublicationsResponse{Items: make([]*workspacev1.ConnectorPublication, 0, len(items))}
	for _, item := range items {
		revision, readErr := repository.GetConnectorRevision(ctx, item.ActiveRevisionID)
		if readErr != nil {
			return nil, publicError(readErr)
		}
		response.Items = append(response.Items, connectorPublicationResponse(item, revision))
	}
	return response, nil
}

func (service *Service) StageConnectorPackage(ctx context.Context, request *workspacev1.StageConnectorPackageRequest) (*workspacev1.ConnectorRevision, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	pkg, err := connectorpackage.Parse(request.Archive)
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", domain.ErrInvalid, err))
	}
	if err := validatePlatformConnectorPackage(pkg); err != nil {
		return nil, publicError(err)
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	if pkg.CLI != nil {
		verified, verifyErr := repository.HasConnectorBundleRuntimeConformance(ctx, pkg.CLIBundleSHA256, pkg.CLI.Runtime.Digest)
		if verifyErr != nil {
			return nil, publicError(verifyErr)
		}
		if !verified {
			return nil, publicError(fmt.Errorf("%w: exact bundle and Runtime RepoDigest Conformance evidence is unavailable", domain.ErrInvalid))
		}
	}
	revision, err := service.storeConnectorRevision(ctx, repository, pkg)
	if err != nil {
		return nil, publicError(err)
	}
	return connectorRevisionResponse(revision), nil
}

func (service *Service) ListConnectorPublicationRevisions(ctx context.Context, _ *workspacev1.ListConnectorPublicationRevisionsRequest) (*workspacev1.ListConnectorPublicationRevisionsResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	revisions, err := repository.ListConnectorRevisions(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	publications, err := repository.ListConnectorPublications(ctx, true)
	if err != nil {
		return nil, publicError(err)
	}
	bySource := make(map[string]domain.ConnectorPublication, len(publications))
	for _, item := range publications {
		bySource[item.PackageSource] = item
	}
	response := &workspacev1.ListConnectorPublicationRevisionsResponse{Items: make([]*workspacev1.ConnectorPublicationRevision, 0, len(revisions))}
	for _, revision := range revisions {
		item := &workspacev1.ConnectorPublicationRevision{Revision: connectorRevisionResponse(revision)}
		if publication, ok := bySource[revision.PackageSource]; ok && publication.ActiveRevisionID == revision.ID {
			item.Publication = connectorPublicationResponse(publication, revision)
		}
		response.Items = append(response.Items, item)
	}
	return response, nil
}

func (service *Service) PublishConnectorRevision(ctx context.Context, request *workspacev1.PublishConnectorRevisionRequest) (*workspacev1.ConnectorPublication, error) {
	principal, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	revision, err := repository.GetConnectorRevision(ctx, request.RevisionId)
	if err != nil {
		return nil, publicError(err)
	}
	if err := validateStagedRevision(revision); err != nil {
		return nil, publicError(err)
	}
	publication, err := repository.PublishConnectorRevision(ctx, principal.UserID, revision.ID, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return connectorPublicationResponse(publication, revision), nil
}

func (service *Service) DisableConnectorPublication(ctx context.Context, request *workspacev1.DisableConnectorPublicationRequest) (*workspacev1.ConnectorPublication, error) {
	principal, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	publication, err := repository.SetConnectorPublicationState(ctx, principal.UserID, request.Source, domain.ConnectorPublicationDisabled, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	revision, err := repository.GetConnectorRevision(ctx, publication.ActiveRevisionID)
	if err != nil {
		return nil, publicError(err)
	}
	return connectorPublicationResponse(publication, revision), nil
}

func (service *Service) ListConnectorPublicationHealth(ctx context.Context, _ *workspacev1.ListConnectorPublicationHealthRequest) (*workspacev1.ListConnectorPublicationHealthResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListConnectorPublicationHealth(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListConnectorPublicationHealthResponse{Items: make([]*workspacev1.ConnectorPublicationHealth, 0, len(items))}
	for _, item := range items {
		policy, _ := decodeConnectorRevisionPolicy(item.Revision)
		health := &workspacev1.ConnectorPublicationHealth{Source: item.Publication.PackageSource, State: string(item.Publication.State), ActiveRevisionId: item.Revision.ID, PackageSha256: item.Revision.PackageSHA256, RuntimeDigests: connectorRuntimeDigests(policy), ConformanceAvailable: connectorConformanceAvailable(item.Revision, policy), InstallationCount: item.InstallationCount, ActiveInstallationCount: item.ActiveInstallationCount, ActiveAuthorizationCount: item.ActiveAuthorizationCount}
		if policy.BundleSHA256 != "" {
			health.BundleSha256 = &policy.BundleSHA256
		}
		response.Items = append(response.Items, health)
	}
	return response, nil
}

func (service *Service) InstallPublishedConnector(ctx context.Context, request *workspacev1.InstallPublishedConnectorRequest) (*workspacev1.ConnectorInstallation, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	publications, err := repository.ListConnectorPublications(ctx, false)
	if err != nil {
		return nil, publicError(err)
	}
	var publication domain.ConnectorPublication
	for _, item := range publications {
		if item.PackageSource == request.Source {
			publication = item
			break
		}
	}
	if publication.PackageSource == "" {
		return nil, publicError(domain.ErrNotFound)
	}
	installation, err := repository.InstallConnectorWithAudit(ctx, domain.ConnectorInstallation{OwnerID: ownerID, PackageSource: publication.PackageSource, ActiveRevisionID: publication.ActiveRevisionID, State: domain.ConnectorInstallationActive}, domain.ConnectorAuditRecord{OwnerID: ownerID, RevisionID: publication.ActiveRevisionID, Operation: "install_published", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	revision, err := repository.GetConnectorRevision(ctx, publication.ActiveRevisionID)
	if err != nil {
		return nil, publicError(err)
	}
	policy, _ := decodeConnectorRevisionPolicy(revision)
	installation.Authorized = policy.AuthMode == "none"
	if current, listErr := repository.ListConnectorInstallations(ctx, ownerID); listErr == nil {
		for _, item := range current {
			if item.ID == installation.ID {
				installation.Authorized = item.Authorized
				break
			}
		}
	}
	return connectorInstallationDetailsResponse(installation, revision, false), nil
}

func (service *Service) UpgradeConnectorInstallation(ctx context.Context, request *workspacev1.UpgradeConnectorInstallationRequest) (*workspacev1.ConnectorInstallation, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	installations, err := repository.ListConnectorInstallations(ctx, ownerID)
	if err != nil {
		return nil, publicError(err)
	}
	var installation domain.ConnectorInstallation
	for _, item := range installations {
		if item.ID == request.InstallationId {
			installation = item
			break
		}
	}
	if installation.ID == "" {
		return nil, publicError(domain.ErrNotFound)
	}
	if installation.Version != request.ExpectedVersion {
		return nil, publicError(domain.ErrConflict)
	}
	publications, err := repository.ListConnectorPublications(ctx, false)
	if err != nil {
		return nil, publicError(err)
	}
	var revisionID string
	for _, item := range publications {
		if item.PackageSource == installation.PackageSource {
			revisionID = item.ActiveRevisionID
			break
		}
	}
	if revisionID == "" {
		return nil, publicError(domain.ErrNotFound)
	}
	updated, err := repository.ActivateConnectorRevisionWithVersion(ctx, ownerID, installation.ID, revisionID, request.ExpectedVersion, time.Now().UTC())
	if err != nil {
		return nil, publicError(err)
	}
	revision, err := repository.GetConnectorRevision(ctx, revisionID)
	if err != nil {
		return nil, publicError(err)
	}
	return connectorInstallationDetailsResponse(updated, revision, false), nil
}

func (service *Service) ListConnectorAuthorizations(ctx context.Context, request *workspacev1.ListConnectorAuthorizationsRequest) (*workspacev1.ListConnectorAuthorizationsResponse, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListConnectorAuthorizations(ctx, ownerID, request.InstallationId)
	if err != nil {
		return nil, publicError(err)
	}
	installations, _ := repository.ListConnectorInstallations(ctx, ownerID)
	selected := ""
	for _, item := range installations {
		if item.ID == request.InstallationId {
			selected = item.AuthorizationID
		}
	}
	response := &workspacev1.ListConnectorAuthorizationsResponse{Items: make([]*workspacev1.ConnectorAuthorization, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, connectorAuthorizationResponse(item, item.ID == selected))
	}
	return response, nil
}

func (service *Service) SelectConnectorAuthorization(ctx context.Context, request *workspacev1.SelectConnectorAuthorizationRequest) (*workspacev1.ConnectorInstallation, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.SelectConnectorAuthorizationWithAudit(ctx, ownerID, request.InstallationId, request.AuthorizationId, request.ExpectedVersion, domain.ConnectorAuditRecord{OwnerID: ownerID, InstallationID: request.InstallationId, Operation: "select_authorization", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	revision, err := repository.GetConnectorRevision(ctx, item.ActiveRevisionID)
	if err != nil {
		return nil, publicError(err)
	}
	return connectorInstallationDetailsResponse(item, revision, false), nil
}

func (service *Service) RefreshConnectorAuthorization(ctx context.Context, request *workspacev1.RefreshConnectorAuthorizationRequest) (*workspacev1.ConnectorAuthorization, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	if locker, ok := repository.(interface {
		LockConnectorAuthorizationRefresh(context.Context, string, string, string) (func(), bool, error)
	}); ok {
		release, locked, lockErr := locker.LockConnectorAuthorizationRefresh(ctx, principal.UserID, request.InstallationId, request.AuthorizationId)
		if lockErr != nil {
			return nil, publicError(lockErr)
		}
		if !locked {
			return nil, publicError(domain.ErrConflict)
		}
		defer release()
	}

	items, err := repository.ListConnectorAuthorizations(ctx, principal.UserID, request.InstallationId)
	if err != nil {
		return nil, publicError(err)
	}
	var current domain.ConnectorAuthorization
	for _, item := range items {
		if item.ID == request.AuthorizationId {
			current = item
			break
		}
	}
	if current.ID == "" {
		return nil, publicError(domain.ErrNotFound)
	}
	if current.Version != request.ExpectedVersion {
		return nil, publicError(domain.ErrConflict)
	}

	_, _, policy, err := connectorInstallationPolicy(ctx, repository, principal.UserID, request.InstallationId)
	if err != nil {
		return nil, publicError(err)
	}
	driver, err := service.interactiveConnectorDriver(policy, repository)
	if err != nil {
		return nil, publicError(err)
	}
	nativeDomestic := ""
	refreshToken := ""
	appID := ""
	switch current.CredentialFormat {
	case "json":
		if current.CredentialAAD == "" {
			return nil, publicError(fmt.Errorf("%w: this authorization must be completed again", domain.ErrConflict))
		}
		plaintext, decryptErr := service.box.Decrypt(current.CredentialCiphertext, current.CredentialAAD)
		if decryptErr != nil {
			return nil, publicError(decryptErr)
		}
		var credentials struct {
			RefreshToken string `json:"refresh_token"`
			ClientID     string `json:"client_id"`
			UserID       string `json:"user_id"`
			IsDomestic   string `json:"is_domestic"`
		}
		if unmarshalErr := json.Unmarshal(plaintext, &credentials); unmarshalErr == nil {
			refreshToken = credentials.RefreshToken
			if isCamScannerCLILoginPolicy(policy) {
				appID = credentials.UserID
				nativeDomestic = credentials.IsDomestic
			}
			if isBrowserOAuthPolicy(policy) || policy.CLI != nil && policy.CLI.AuthenticationDriver == "dingtalk" {
				// The OAuth Client ID is bound to the selected authorization, not the current CLI deployment.
				appID = credentials.ClientID
			}
		}
		clear(plaintext)
	case "access_token":
		if len(current.RefreshCredentialCiphertext) > 0 && current.RefreshCredentialAAD != "" {
			plaintext, decryptErr := service.box.Decrypt(current.RefreshCredentialCiphertext, current.RefreshCredentialAAD)
			if decryptErr != nil {
				return nil, publicError(decryptErr)
			}
			refreshToken = string(plaintext)
		}
	}
	if refreshToken == "" && (isBrowserOAuthPolicy(policy) || isCamScannerCLILoginPolicy(policy)) && len(current.RefreshCredentialCiphertext) > 0 {
		plaintext, decryptErr := service.box.Decrypt(current.RefreshCredentialCiphertext, current.RefreshCredentialAAD)
		if decryptErr != nil {
			return nil, publicError(decryptErr)
		}
		refreshToken = string(plaintext)
		clear(plaintext)
	}
	if strings.TrimSpace(refreshToken) == "" {
		return nil, publicError(fmt.Errorf("%w: this authorization must be completed again", domain.ErrConflict))
	}
	resolvedAppID, appSecret, err := driver.Application(ctx, principal.UserID, request.InstallationId)
	if err != nil {
		return nil, publicError(err)
	}
	if appID == "" {
		appID = resolvedAppID
	}
	result, err := driver.Refresh(ctx, appID, appSecret, refreshToken)
	if err != nil {
		return nil, publicError(err)
	}
	if result.ExternalID != "" && current.ExternalIdentityID != "" && current.ExternalIdentityID != result.ExternalID {
		return nil, publicError(fmt.Errorf("%w: refreshed authorization belongs to a different account", domain.ErrConflict))
	}
	if result.ExternalID == "" {
		result.ExternalID = current.ExternalIdentityID
		result.DisplayName = current.ExternalDisplayName
	}
	if len(result.Scopes) == 0 {
		result.Scopes = append([]string(nil), current.Scopes...)
	}
	if result.RefreshToken == "" {
		result.RefreshToken = refreshToken
	}
	if isCamScannerCLILoginPolicy(policy) {
		result.IsDomestic = nativeDomestic
	}
	refreshedCredentials, err := json.Marshal(connectorAuthorizationCredentialFields(policy, result))
	if err != nil {
		return nil, publicError(err)
	}
	aad := connectorAuthorizationAAD(principal.UserID, request.InstallationId, result.ExternalID)
	ciphertext, err := service.box.Encrypt(refreshedCredentials, aad)
	clear(refreshedCredentials)
	if err != nil {
		return nil, publicError(err)
	}
	if isBrowserOAuthPolicy(policy) || isCamScannerCLILoginPolicy(policy) {
		current.RefreshCredentialAAD = aad + ":refresh"
		current.RefreshCredentialCiphertext, err = service.box.Encrypt([]byte(result.RefreshToken), current.RefreshCredentialAAD)
		if err != nil {
			return nil, publicError(err)
		}
	} else {
		current.RefreshCredentialCiphertext = nil
		current.RefreshCredentialAAD = ""
	}
	current.ExternalIdentityID = result.ExternalID
	current.ExternalDisplayName = result.DisplayName
	current.Scopes = result.Scopes
	current.CredentialCiphertext = ciphertext
	current.CredentialAAD = aad
	current.CredentialFormat = "json"
	current.State = domain.ConnectorAuthorizationActive
	expiry := result.ExpiresAt
	if !result.RefreshExpiresAt.IsZero() && !isBrowserOAuthPolicy(policy) {
		expiry = result.RefreshExpiresAt
	}
	current.ExpiresAt = &expiry
	updated, err := repository.RefreshConnectorAuthorization(ctx, current, request.ExpectedVersion, domain.ConnectorAuditRecord{OwnerID: principal.UserID, InstallationID: request.InstallationId, Operation: "refresh_authorization", IdentityRef: current.IdentityRef, Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	return connectorAuthorizationResponse(updated, false), nil
}

func (service *Service) DisconnectPublishedConnectorAuthorization(ctx context.Context, request *workspacev1.DisconnectPublishedConnectorAuthorizationRequest) (*workspacev1.ConnectorAuthorization, error) {
	ownerID, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	item, err := repository.DisconnectConnectorAuthorizationWithAudit(ctx, ownerID, request.AuthorizationId, domain.ConnectorAuditRecord{OwnerID: ownerID, Operation: "disconnect_authorization", Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	return connectorAuthorizationResponse(item, false), nil
}

func (service *Service) BeginConnectorSetup(ctx context.Context, request *workspacev1.BeginConnectorSetupRequest) (*workspacev1.ConnectorSetup, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	installation, revision, policy, err := connectorInstallationPolicy(ctx, repository, principal.UserID, request.InstallationId)
	if err != nil {
		return nil, publicError(err)
	}
	if policy.CLI == nil || policy.CLI.AuthenticationDriver != "feishu" {
		return nil, publicError(fmt.Errorf("%w: Connector does not use the Feishu setup driver", domain.ErrInvalid))
	}
	if existing, existingErr := repository.GetConnectorProviderApplication(ctx, principal.UserID, installation.ID); existingErr == nil {
		return connectorSetupResponse(domain.ConnectorSetup{ID: installation.ID, InstallationID: installation.ID, State: "completed", ProviderName: existing.ProviderName, DeveloperConsoleURL: existing.DeveloperConsoleURL}), nil
	} else if !errors.Is(existingErr, domain.ErrNotFound) {
		return nil, publicError(existingErr)
	}
	registration, err := service.feishu.Begin(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Encrypt([]byte(registration.DeviceCode), connectorSetupAAD(principal.UserID, installation.ID))
	if err != nil {
		return nil, publicError(err)
	}
	flow, err := repository.BeginConnectorSetup(ctx, domain.ConnectorSetup{OwnerID: principal.UserID, InstallationID: installation.ID, State: "waiting_for_user", ActionURL: registration.ActionURL, ExpiresAt: &registration.ExpiresAt, DeviceCodeCiphertext: deviceCode})
	if err != nil {
		return nil, publicError(err)
	}
	_ = revision
	return connectorSetupResponse(flow), nil
}

func (service *Service) CompleteConnectorSetup(ctx context.Context, request *workspacev1.CompleteConnectorSetupRequest) (*workspacev1.ConnectorSetup, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	flow, err := repository.GetConnectorSetup(ctx, principal.UserID, request.FlowId)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Decrypt(flow.DeviceCodeCiphertext, connectorSetupAAD(principal.UserID, flow.InstallationID))
	if err != nil {
		return nil, publicError(err)
	}
	application, err := service.feishu.Poll(ctx, string(deviceCode))
	if errors.Is(err, feishucli.ErrPending) {
		return connectorSetupResponse(flow), nil
	}
	if err != nil {
		return nil, publicError(err)
	}
	appID, err := service.box.Encrypt([]byte(application.AppID), feishuApplicationAAD(principal.UserID))
	if err != nil {
		return nil, publicError(err)
	}
	appSecret, err := service.box.Encrypt([]byte(application.AppSecret), feishuApplicationAAD(principal.UserID))
	if err != nil {
		return nil, publicError(err)
	}
	providerName := strings.TrimSpace(application.UserName)
	if providerName == "" {
		providerName = strings.TrimSpace(principal.DisplayName)
	}
	if providerName == "" {
		providerName = principal.Username
	}
	providerName += "的飞书CLI"
	completed, err := repository.CompleteConnectorSetup(ctx, principal.UserID, flow.ID, domain.ConnectorProviderApplication{OwnerID: principal.UserID, InstallationID: flow.InstallationID, AppIDCiphertext: appID, AppSecretCiphertext: appSecret, ProviderName: providerName, DeveloperConsoleURL: "https://open.feishu.cn/app/" + url.PathEscape(application.AppID)})
	if err != nil {
		return nil, publicError(err)
	}
	return connectorSetupResponse(completed), nil
}

func (service *Service) BeginConnectorAuthorizationFlow(ctx context.Context, request *workspacev1.BeginConnectorAuthorizationFlowRequest) (*workspacev1.ConnectorAuthorizationFlow, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if request.Identity != "user" {
		return nil, publicError(fmt.Errorf("%w: interactive authorization only supports user identity", domain.ErrInvalid))
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	installation, _, policy, err := connectorInstallationPolicy(ctx, repository, principal.UserID, request.InstallationId)
	if err != nil {
		return nil, publicError(err)
	}
	driver, err := service.interactiveConnectorDriver(policy, repository)
	if err != nil {
		return nil, publicError(err)
	}
	allowedScopes := map[string]struct{}{}
	if isTianyanchaMCPPolicy(policy) {
		allowedScopes["mcp:tools.call"] = struct{}{}
	}
	if isPixsoMCPPolicy(policy) {
		allowedScopes["mcp:connect"] = struct{}{}
	}
	if isLinearMCPPolicy(policy) {
		allowedScopes["read"] = struct{}{}
		allowedScopes["write"] = struct{}{}
	}
	if isKlingMCPLoginPolicy(policy) {
		for _, scope := range klingmcp.Scopes() {
			allowedScopes[scope] = struct{}{}
		}
	}
	if policy.CLI != nil {
		for _, capability := range policy.CLI.Capabilities {
			for _, identity := range capability.Identities {
				if identity == "user" {
					for _, scope := range capability.Scopes {
						allowedScopes[scope] = struct{}{}
					}
				}
			}
		}
	}
	for _, scope := range request.Scopes {
		if _, ok := allowedScopes[scope]; !ok {
			return nil, publicError(fmt.Errorf("%w: requested scope is outside the reviewed Connector policy", domain.ErrInvalid))
		}
	}
	appID, appSecret, err := driver.Application(ctx, principal.UserID, installation.ID)
	if err != nil {
		return nil, publicError(err)
	}
	providerFlow, err := driver.Begin(ctx, appID, appSecret, request.Scopes)
	if errors.Is(err, tianyanchamcp.ErrRegionBlocked) {
		return nil, kratoserrors.New(http.StatusBadGateway, "tianyancha_region_blocked", "Tianyancha does not support the deployment server region")
	}
	if errors.Is(err, xiaoemcp.ErrCallbackBlocked) {
		return nil, kratoserrors.New(http.StatusBadGateway, "xiaoe_oauth_callback_blocked", "Xiaoe blocked registration of this platform callback domain")
	}
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Encrypt([]byte(providerFlow.State), connectorAuthorizationFlowAAD(principal.UserID, installation.ID, request.Identity))
	if err != nil {
		return nil, publicError(err)
	}
	flow, err := repository.BeginConnectorAuthorizationFlow(ctx, domain.ConnectorAuthorizationAttempt{OwnerID: principal.UserID, InstallationID: installation.ID, Identity: request.Identity, Scopes: providerFlow.Scopes, ActionURL: providerFlow.ActionURL, ExpiresAt: providerFlow.ExpiresAt, DeviceCodeCiphertext: deviceCode})
	if err != nil {
		return nil, publicError(err)
	}
	if isBrowserOAuthPolicy(policy) {
		flow, err = service.sealBrowserOAuthCallback(ctx, repository, flow, browserOAuthProfileFor(policy))
		if err != nil {
			_ = repository.DeleteConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID)
			return nil, publicError(err)
		}
	}
	return connectorAuthorizationFlowResponse(flow, "waiting_for_user", nil), nil
}

func (service *Service) CompleteConnectorAuthorizationFlow(ctx context.Context, request *workspacev1.CompleteConnectorAuthorizationFlowRequest) (*workspacev1.ConnectorAuthorizationFlow, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	flow, err := repository.GetConnectorAuthorizationFlow(ctx, principal.UserID, request.FlowId)
	if err != nil {
		return nil, publicError(err)
	}
	_, _, policy, err := connectorInstallationPolicy(ctx, repository, principal.UserID, flow.InstallationID)
	if err != nil {
		return nil, publicError(err)
	}
	driver, err := service.interactiveConnectorDriver(policy, repository)
	if err != nil {
		return nil, publicError(err)
	}
	appID, appSecret, err := driver.Application(ctx, principal.UserID, flow.InstallationID)
	if err != nil {
		return nil, publicError(err)
	}
	deviceCode, err := service.box.Decrypt(flow.DeviceCodeCiphertext, connectorAuthorizationFlowAAD(principal.UserID, flow.InstallationID, flow.Identity))
	if err != nil {
		return nil, publicError(err)
	}
	defer clear(deviceCode)
	if isBrowserOAuthPolicy(policy) {
		var state struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(deviceCode, &state) != nil {
			return nil, publicError(domain.ErrInvalid)
		}
		if state.Code != "" {
			// Authorization codes are single-use. Claim this exact encrypted callback
			// once before exchange; failures require a new browser authorization.
			if err := repository.ConsumeConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID, flow.DeviceCodeCiphertext); err != nil {
				return connectorAuthorizationFlowResponse(flow, "invalid", nil), nil
			}
		}
	}
	if isGitHubCLILoginPolicy(policy) {
		next, reserveErr := githubcli.ReservePoll(string(deviceCode))
		if errors.Is(reserveErr, githubcli.ErrPending) {
			return connectorAuthorizationFlowResponse(flow, "waiting_for_user", nil), nil
		}
		if errors.Is(reserveErr, githubcli.ErrExpired) {
			_ = repository.DeleteConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID)
			return connectorAuthorizationFlowResponse(flow, "invalid", nil), nil
		}
		if reserveErr != nil {
			return nil, publicError(reserveErr)
		}
		encrypted, encryptErr := service.box.Encrypt([]byte(next), connectorAuthorizationFlowAAD(principal.UserID, flow.InstallationID, flow.Identity))
		if encryptErr != nil {
			return nil, publicError(encryptErr)
		}
		if updateErr := repository.UpdateConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID, flow.DeviceCodeCiphertext, encrypted, flow.ActionURL); updateErr != nil {
			return nil, publicError(updateErr)
		}
		clear(deviceCode)
		deviceCode = []byte(next)
		defer clear(deviceCode)
		flow.DeviceCodeCiphertext = encrypted
	}
	result, err := driver.Poll(ctx, appID, appSecret, string(deviceCode))
	if isGitHubCLILoginPolicy(policy) {
		if errors.Is(err, githubcli.ErrSlowDown) {
			next, slowErr := githubcli.SlowPoll(string(deviceCode))
			if slowErr != nil {
				return nil, publicError(slowErr)
			}
			encrypted, encryptErr := service.box.Encrypt([]byte(next), connectorAuthorizationFlowAAD(principal.UserID, flow.InstallationID, flow.Identity))
			if encryptErr != nil {
				return nil, publicError(encryptErr)
			}
			if updateErr := repository.UpdateConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID, flow.DeviceCodeCiphertext, encrypted, flow.ActionURL); updateErr != nil {
				return nil, publicError(updateErr)
			}
			return connectorAuthorizationFlowResponse(flow, "waiting_for_user", nil), nil
		}
		if errors.Is(err, githubcli.ErrConsumed) {
			_ = repository.DeleteConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID)
			return connectorAuthorizationFlowResponse(flow, "invalid", nil), nil
		}
		if err == nil {
			if consumeErr := repository.ConsumeConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID, flow.DeviceCodeCiphertext); consumeErr != nil {
				return nil, publicError(consumeErr)
			}
		}
	}

	if errors.Is(err, errConnectorAuthorizationPending) {
		return connectorAuthorizationFlowResponse(flow, "waiting_for_user", nil), nil
	}
	if errors.Is(err, errConnectorAuthorizationDenied) || errors.Is(err, errConnectorAuthorizationExpired) {
		_ = repository.DeleteConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID)
		return connectorAuthorizationFlowResponse(flow, "invalid", nil), nil
	}
	if err != nil {
		if isBrowserOAuthPolicy(policy) {
			_ = repository.DeleteConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID)
			return connectorAuthorizationFlowResponse(flow, "invalid", nil), nil
		}
		if policy.CLI != nil && policy.CLI.AuthenticationDriver == "dingtalk" {
			if errors.Is(err, dingtalkcli.ErrApprovalConsumed) {
				// DingTalk authorization codes are single-use. A failed exchange or
				// CLI permission check cannot be repaired by polling this flow again.
				if deleteErr := repository.DeleteConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID); deleteErr != nil {
					return nil, publicError(deleteErr)
				}
			}
			slog.WarnContext(ctx, "DingTalk Connector authorization failed", "cause", err.Error())
			var restriction *dingtalkcli.CLIRestrictionError
			switch {
			case errors.As(err, &restriction) && restriction.Reason == "enterprise_not_authorized":
				return nil, kratoserrors.New(http.StatusUnprocessableEntity, "dingtalk_cli_enterprise_denied", "DingTalk organization denied CLI access")
			case errors.As(err, &restriction) && (restriction.Reason == "user_forbidden" || restriction.Reason == "user_not_allowed"):
				return nil, kratoserrors.New(http.StatusUnprocessableEntity, "dingtalk_cli_user_denied", "DingTalk user is outside CLI access scope")
			case errors.As(err, &restriction) && restriction.Reason == "channel_required":
				return nil, kratoserrors.New(http.StatusUnprocessableEntity, "dingtalk_cli_channel_required", "DingTalk organization requires a CLI channel")
			case errors.As(err, &restriction) && restriction.Reason == "no_auth":
				return nil, kratoserrors.New(http.StatusUnprocessableEntity, "dingtalk_cli_auth_expired", "DingTalk CLI authorization is unavailable")
			case errors.Is(err, dingtalkcli.ErrCLIAuthDisabled):
				return nil, kratoserrors.New(http.StatusUnprocessableEntity, "dingtalk_cli_access_disabled", "DingTalk organization has not enabled CLI access")
			case errors.Is(err, dingtalkcli.ErrIdentityMismatch):
				return nil, kratoserrors.New(http.StatusUnprocessableEntity, "dingtalk_identity_mismatch", "DingTalk authorization returned a different organization")
			default:
				return nil, kratoserrors.New(http.StatusBadGateway, "dingtalk_authorization_failed", "DingTalk authorization could not be completed")
			}
		}
		return nil, publicError(err)
	}
	if isCamScannerCLILoginPolicy(policy) {
		// Polling may return the same upstream grant more than once. Only one owner
		// request may consume this flow and persist a grant.
		if err := repository.ConsumeConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID, flow.DeviceCodeCiphertext); err != nil {
			return connectorAuthorizationFlowResponse(flow, "invalid", nil), nil
		}
	}
	credentials, err := json.Marshal(connectorAuthorizationCredentialFields(policy, result))
	if err != nil {
		return nil, publicError(err)
	}
	aad := connectorAuthorizationAAD(principal.UserID, flow.InstallationID, result.ExternalID)
	ciphertext, err := service.box.Encrypt(credentials, aad)
	clear(credentials)
	if err != nil {
		return nil, publicError(err)
	}
	var expiry *time.Time
	if !result.ExpiresAt.IsZero() {
		value := result.ExpiresAt
		if !result.RefreshExpiresAt.IsZero() && !isBrowserOAuthPolicy(policy) {
			value = result.RefreshExpiresAt
		}
		expiry = &value
	}
	var refreshCiphertext []byte
	refreshAAD := ""
	if (isBrowserOAuthPolicy(policy) || isCamScannerCLILoginPolicy(policy)) && result.RefreshToken != "" {
		refreshAAD = aad + ":refresh"
		refreshCiphertext, err = service.box.Encrypt([]byte(result.RefreshToken), refreshAAD)
		if err != nil {
			return nil, publicError(err)
		}
	}
	authorization, err := repository.CreateConnectorAuthorizationWithAudit(ctx, domain.ConnectorAuthorization{OwnerID: principal.UserID, InstallationID: flow.InstallationID, IdentityRef: flow.Identity, ExternalIdentityID: result.ExternalID, ExternalDisplayName: result.DisplayName, Scopes: result.Scopes, CredentialCiphertext: ciphertext, CredentialAAD: aad, CredentialFormat: "json", RefreshCredentialCiphertext: refreshCiphertext, RefreshCredentialAAD: refreshAAD, State: domain.ConnectorAuthorizationActive, ExpiresAt: expiry}, domain.ConnectorAuditRecord{OwnerID: principal.UserID, InstallationID: flow.InstallationID, Operation: "authorize", IdentityRef: flow.Identity, Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	if err := repository.DeleteConnectorAuthorizationFlow(ctx, principal.UserID, flow.ID); err != nil {
		return nil, publicError(err)
	}
	return connectorAuthorizationFlowResponse(flow, "completed", &authorization), nil
}

func connectorInstallationPolicy(ctx context.Context, repository connectorPackageRepository, ownerID, installationID string) (domain.ConnectorInstallation, domain.ConnectorRevision, connectorRevisionPolicy, error) {
	installations, err := repository.ListConnectorInstallations(ctx, ownerID)
	if err != nil {
		return domain.ConnectorInstallation{}, domain.ConnectorRevision{}, connectorRevisionPolicy{}, err
	}
	var installation domain.ConnectorInstallation
	for _, item := range installations {
		if item.ID == installationID {
			installation = item
			break
		}
	}
	if installation.ID == "" || installation.State != domain.ConnectorInstallationActive {
		return domain.ConnectorInstallation{}, domain.ConnectorRevision{}, connectorRevisionPolicy{}, domain.ErrNotFound
	}
	revision, err := repository.GetConnectorRevision(ctx, installation.ActiveRevisionID)
	if err != nil {
		return domain.ConnectorInstallation{}, domain.ConnectorRevision{}, connectorRevisionPolicy{}, err
	}
	policy, err := decodeConnectorRevisionPolicy(revision)
	return installation, revision, policy, err
}

func validateInteractiveConnectorDriver(policy connectorRevisionPolicy) error {
	if connectorAuthorizationMode(policy) == "interactive" {
		return nil
	}
	return fmt.Errorf("%w: this Connector revision has no interactive authorization adapter", domain.ErrInvalid)
}

func connectorAuthorizationMode(policy connectorRevisionPolicy) string {
	if isTianyanchaMCPPolicy(policy) || isXiaoeMCPLoginPolicy(policy) || isLinearMCPPolicy(policy) || isPixsoMCPPolicy(policy) || isKlingMCPLoginPolicy(policy) {
		return "interactive"
	}
	if policy.CLI != nil {
		if isNotionCLILoginPolicy(policy) || isBrowserOAuthPolicy(policy) || isCamScannerCLILoginPolicy(policy) || isGitHubCLILoginPolicy(policy) {
			return "interactive"
		}
		switch policy.CLI.AuthenticationDriver {
		case "feishu", "dingtalk":
			return "interactive"
		case "connector_package":
			return "provided"
		}
	}
	if policy.MCP != nil && policy.AuthMode != "none" {
		return "provided"
	}
	return "none"
}

func isGitHubCLILoginPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "github" && policy.AuthMode == "oauth" && policy.CLI != nil && policy.CLI.AuthenticationDriver == "connector_package"
}

func isXiaoeMCPLoginPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "xiaoe" && policy.AuthMode == "oauth" && policy.CLI == nil && policy.MCP != nil && policy.MCP.Transport == "streamable_http" && policy.MCP.URL == xiaoemcp.Resource && len(policy.MCP.EgressHosts) == 1 && policy.MCP.EgressHosts[0] == "agent.xiaoe-tech.com" && len(policy.MCP.Headers) == 0 && len(policy.MCP.Environment) == 0
}

func isTeambitionCLILoginPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "teambition" && policy.AuthMode == "oauth" && policy.CLI != nil && policy.CLI.AuthenticationDriver == "connector_package"
}

func isCamScannerCLILoginPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "camscanner" && policy.AuthMode == "oauth" && policy.CLI != nil && policy.CLI.AuthenticationDriver == "connector_package"
}

func isNotionCLILoginPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "notion" && policy.CLI != nil && policy.CLI.AuthenticationDriver == "connector_package"
}

func connectorAuthorizationCredentialFields(policy connectorRevisionPolicy, result connectorAuthorizationGrant) map[string]string {
	if isTianyanchaMCPPolicy(policy) || isXiaoeMCPLoginPolicy(policy) || isLinearMCPPolicy(policy) || isPixsoMCPPolicy(policy) {
		return map[string]string{"MCP_BEARER_TOKEN": result.AccessToken, "client_id": result.ClientID, "access_expires_at": result.ExpiresAt.UTC().Format(time.RFC3339)}
	}

	if isKlingMCPLoginPolicy(policy) {
		return map[string]string{"MCP_BEARER_TOKEN": result.AccessToken, "client_id": result.ClientID}
	}
	if isCamScannerCLILoginPolicy(policy) {
		return map[string]string{"access_token": result.AccessToken, "access_expires_at": result.ExpiresAt.UTC().Format(time.RFC3339), "user_id": result.ExternalID, "is_domestic": result.IsDomestic}
	}
	if isGitHubCLILoginPolicy(policy) {
		return map[string]string{"access_token": result.AccessToken}
	}
	if isNotionCLILoginPolicy(policy) {
		return map[string]string{"token": result.AccessToken}
	}
	fields := map[string]string{"access_token": result.AccessToken, "refresh_token": result.RefreshToken, "client_id": result.ClientID, "access_expires_at": result.ExpiresAt.UTC().Format(time.RFC3339)}
	if isBrowserOAuthPolicy(policy) {
		delete(fields, "refresh_token")
	}
	return fields
}

func connectorSetupAAD(ownerID, installationID string) string {
	return "connector-feishu-registration:" + ownerID + ":" + installationID
}

func connectorAuthorizationFlowAAD(ownerID, installationID, identity string) string {
	return "connector-feishu-authorization-flow:" + ownerID + ":" + installationID + ":" + identity
}

func connectorAuthorizationAAD(ownerID, installationID, identity string) string {
	return "connector-authorization:" + ownerID + ":" + installationID + ":" + identity
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
	if err := validatePrivateConnectorPackage(pkg); err != nil {
		return nil, publicError(err)
	}
	repository, err := service.connectorPackages()
	if err != nil {
		return nil, publicError(err)
	}
	revision, key := connectorRevisionFromPackage(pkg)
	if _, err := service.objects.Put(ctx, key, bytes.NewReader(pkg.NormalizedArchive), objectstore.PutOptions{Size: int64(len(pkg.NormalizedArchive)), SHA256: pkg.SHA256, ContentType: "application/zip", Metadata: map[string]string{"source": pkg.Metadata.Source, "version": pkg.Metadata.Version}}); err != nil {
		return nil, publicError(err)
	}
	if len(pkg.CLIBundle) > 0 {
		if _, err := service.objects.Put(ctx, connectorBundleObjectKey(pkg), bytes.NewReader(pkg.CLIBundle), objectstore.PutOptions{Size: int64(len(pkg.CLIBundle)), SHA256: pkg.CLIBundleSHA256, ContentType: "application/gzip", Metadata: map[string]string{"artifact-kind": "connector-package-cli-bundle", "source": pkg.Metadata.Source, "version": pkg.Metadata.Version}}); err != nil {
			return nil, publicError(err)
		}
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
	installation.Authorized = pkg.Metadata.AuthMode == "none" || installation.AuthorizationID != ""
	return connectorInstallationResponse(installation), nil
}

func validatePrivateConnectorPackage(pkg connectorpackage.Package) error {
	if pkg.Metadata.Source == "xiaoe" {
		return fmt.Errorf("%w: Xiaoe login is reserved for the platform publication", domain.ErrInvalid)
	}
	if pkg.Metadata.Source == "notion" {
		return fmt.Errorf("%w: Notion login is reserved for the platform publication", domain.ErrInvalid)
	}
	if pkg.CLI != nil && (pkg.CLI.AuthenticationDriver == "feishu" || pkg.CLI.AuthenticationDriver == "dingtalk") {
		return fmt.Errorf("%w: interactive authentication drivers are reserved for Conformance-backed platform publications", domain.ErrInvalid)
	}
	return nil
}

func validatePlatformConnectorPackage(pkg connectorpackage.Package) error {
	if pkg.Metadata.Source == "xiaoe" && !isXiaoeMCPLoginPolicy(connectorRevisionPolicy{Metadata: pkg.Metadata, AuthMode: pkg.Metadata.AuthMode, MCP: pkg.MCP, CLI: pkg.CLI}) {
		return fmt.Errorf("%w: Xiaoe requires the reviewed remote MCP policy", domain.ErrInvalid)
	}
	if pkg.CLI == nil {
		return nil
	}
	if (pkg.Metadata.Source == "dingtalk") != (pkg.CLI.AuthenticationDriver == "dingtalk") {
		return fmt.Errorf("%w: DingTalk authorization driver must match the reviewed DingTalk package source", domain.ErrInvalid)
	}
	if len(pkg.CLIBundle) == 0 || len(pkg.CLIBundleSHA256) != 64 {
		return fmt.Errorf("%w: a platform CLI publication requires an immutable executable bundle", domain.ErrInvalid)
	}
	if len(pkg.CLI.Capabilities) == 0 || len(pkg.CLI.Runtime.Digest) != 71 || !strings.HasPrefix(pkg.CLI.Runtime.Digest, "sha256:") {
		return fmt.Errorf("%w: a platform CLI publication requires reviewed capabilities and an exact Runtime RepoDigest", domain.ErrInvalid)
	}
	return nil
}

func (service *Service) storeConnectorRevision(ctx context.Context, repository connectorPackageRepository, pkg connectorpackage.Package) (domain.ConnectorRevision, error) {
	revision, key := connectorRevisionFromPackage(pkg)
	if _, err := service.objects.Put(ctx, key, bytes.NewReader(pkg.NormalizedArchive), objectstore.PutOptions{Size: int64(len(pkg.NormalizedArchive)), SHA256: pkg.SHA256, ContentType: "application/zip", Metadata: map[string]string{"source": pkg.Metadata.Source, "version": pkg.Metadata.Version}}); err != nil {
		return domain.ConnectorRevision{}, err
	}
	if len(pkg.CLIBundle) > 0 {
		if _, err := service.objects.Put(ctx, connectorBundleObjectKey(pkg), bytes.NewReader(pkg.CLIBundle), objectstore.PutOptions{Size: int64(len(pkg.CLIBundle)), SHA256: pkg.CLIBundleSHA256, ContentType: "application/gzip", Metadata: map[string]string{"artifact-kind": "connector-package-cli-bundle", "source": pkg.Metadata.Source, "version": pkg.Metadata.Version}}); err != nil {
			return domain.ConnectorRevision{}, err
		}
	}
	revision.ObjectKey = key
	return repository.CreateConnectorRevision(ctx, revision)
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
	installation, _, policy, err := connectorInstallationPolicy(ctx, repository, ownerID, request.InstallationId)
	if err != nil {
		return nil, publicError(err)
	}
	if err := validateProvidedConnectorCredentials(policy, request.Scopes); err != nil {
		return nil, publicError(err)
	}
	if err := validatePKULawCredentials(policy, request.CredentialsJson); err != nil {
		return nil, publicError(err)
	}
	ciphertext, err := service.box.Encrypt(request.CredentialsJson, "connector-authorization:"+ownerID)
	if err != nil {
		return nil, publicError(err)
	}
	_, err = repository.CreateConnectorAuthorizationWithAudit(ctx, domain.ConnectorAuthorization{OwnerID: ownerID, InstallationID: request.InstallationId, IdentityRef: strings.TrimSpace(request.IdentityRef), Scopes: append([]string(nil), request.Scopes...), CredentialCiphertext: ciphertext, CredentialAAD: "connector-authorization:" + ownerID, CredentialFormat: "json", State: domain.ConnectorAuthorizationActive}, domain.ConnectorAuditRecord{OwnerID: ownerID, InstallationID: installation.ID, RevisionID: installation.ActiveRevisionID, Operation: "authorize", IdentityRef: strings.TrimSpace(request.IdentityRef), Outcome: "succeeded", CreatedAt: time.Now().UTC()})
	if err != nil {
		return nil, publicError(err)
	}
	items, err := repository.ListConnectorInstallations(ctx, ownerID)
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

func validatePKULawCredentials(policy connectorRevisionPolicy, credentials []byte) error {
	if policy.Metadata.Source != "pkulaw" {
		return nil
	}
	var values map[string]string
	if err := json.Unmarshal(credentials, &values); err != nil || len(values) != 1 {
		return fmt.Errorf("%w: PKULaw requires one MCP Bearer token", domain.ErrInvalid)
	}
	token := values["MCP_BEARER_TOKEN"]
	if token == "" || len(token) > 4096 || strings.IndexFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return fmt.Errorf("%w: PKULaw requires a bounded token without whitespace or control characters", domain.ErrInvalid)
	}
	return nil
}

func validateProvidedConnectorCredentials(policy connectorRevisionPolicy, requestedScopes []string) error {
	if connectorAuthorizationMode(policy) != "provided" {
		return fmt.Errorf("%w: this Connector revision does not accept provided credentials", domain.ErrInvalid)
	}
	if policy.MCP != nil {
		if len(requestedScopes) == 0 {
			return nil
		}
		return fmt.Errorf("%w: MCP Connector package has no reviewed scopes", domain.ErrInvalid)
	}
	allowed := make(map[string]struct{})
	for _, capability := range policy.CLI.Capabilities {
		for _, scope := range capability.Scopes {
			allowed[scope] = struct{}{}
		}
	}
	for _, scope := range requestedScopes {
		if _, ok := allowed[scope]; !ok {
			return fmt.Errorf("%w: provided Connector scope is outside the reviewed policy", domain.ErrInvalid)
		}
	}
	return nil
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
	policyValue := map[string]any{"auth_mode": pkg.Metadata.AuthMode, "metadata": pkg.Metadata, "mcp": pkg.MCP, "cli": pkg.CLI}
	if len(pkg.CLIBundle) > 0 {
		policyValue["cli_bundle_object_key"] = connectorBundleObjectKey(pkg)
		policyValue["cli_bundle_sha256"] = pkg.CLIBundleSHA256
	}
	policy, _ := json.Marshal(policyValue)
	revision := domain.ConnectorRevision{PackageSource: pkg.Metadata.Source, Version: pkg.Metadata.Version, Mode: domain.ConnectorMode(pkg.Metadata.Type), PackageSHA256: pkg.SHA256, RuntimePolicy: policy}
	return revision, fmt.Sprintf("connectors/%s/%s/%s.zip", pkg.Metadata.Source, pkg.Metadata.Version, pkg.SHA256)
}

func connectorBundleObjectKey(pkg connectorpackage.Package) string {
	return fmt.Sprintf("cli-connectors/packages/%s/%s/%s.tgz", pkg.Metadata.Source, pkg.Metadata.Version, pkg.CLIBundleSHA256)
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
	return &workspacev1.ConnectorInstallation{Id: item.ID, Source: item.PackageSource, ActiveRevisionId: item.ActiveRevisionID, State: string(item.State), Authorized: item.Authorized, Version: item.Version}
}

func decodeConnectorRevisionPolicy(revision domain.ConnectorRevision) (connectorRevisionPolicy, error) {
	var policy connectorRevisionPolicy
	if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil {
		return connectorRevisionPolicy{}, fmt.Errorf("decode Connector Revision policy: %w", err)
	}
	return policy, nil
}

func connectorRuntimeDigests(policy connectorRevisionPolicy) []string {
	if policy.CLI == nil || policy.CLI.Runtime.Digest == "" {
		return nil
	}
	return []string{policy.CLI.Runtime.Digest}
}

func connectorConformanceAvailable(revision domain.ConnectorRevision, policy connectorRevisionPolicy) bool {
	if revision.Mode == domain.ConnectorModeMCP {
		return policy.MCP != nil
	}
	return policy.CLI != nil && len(policy.CLI.Capabilities) > 0 && len(policy.BundleSHA256) == 64 && len(connectorRuntimeDigests(policy)) > 0
}

func validateStagedRevision(revision domain.ConnectorRevision) error {
	policy, err := decodeConnectorRevisionPolicy(revision)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	if policy.LegacyProjection || !connectorConformanceAvailable(revision, policy) {
		return fmt.Errorf("%w: Connector Revision has no exact Conformance evidence", domain.ErrInvalid)
	}
	return nil
}

func connectorRevisionResponse(item domain.ConnectorRevision) *workspacev1.ConnectorRevision {
	policy, _ := decodeConnectorRevisionPolicy(item)
	name := policy.Metadata.Name
	if name == "" {
		name = item.PackageSource
	}
	response := &workspacev1.ConnectorRevision{Id: item.ID, Source: item.PackageSource, PackageVersion: item.Version, Mode: string(item.Mode), Sha256: item.PackageSHA256, Name: connectorpackage.DisplayName(item.PackageSource, name), Description: connectorpackage.DisplayDescription(item.PackageSource, policy.Metadata.Description), ExamplesZh: append([]string(nil), policy.Metadata.ExamplesZH...), ExamplesEn: append([]string(nil), policy.Metadata.ExamplesEN...), Icon: connectorpackage.DisplayIcon(item.PackageSource), RuntimeDigests: connectorRuntimeDigests(policy), ConformanceAvailable: connectorConformanceAvailable(item, policy)}
	if policy.CLI != nil {
		response.AuthenticationDriver = policy.CLI.AuthenticationDriver
		if len(policy.CLI.ActivationScopes) > 0 {
			response.RequiredScopes = append(response.RequiredScopes, policy.CLI.ActivationScopes...)
		} else {
			seenScopes := map[string]struct{}{}
			for _, capability := range policy.CLI.Capabilities {
				for _, scope := range capability.Scopes {
					if _, exists := seenScopes[scope]; !exists {
						seenScopes[scope] = struct{}{}
						response.RequiredScopes = append(response.RequiredScopes, scope)
					}
				}
			}
		}
	}
	if isTianyanchaMCPPolicy(policy) {
		response.RequiredScopes = []string{"mcp:tools.call"}
	}
	if isPixsoMCPPolicy(policy) {
		response.RequiredScopes = []string{"mcp:connect"}
	}
	if isLinearMCPPolicy(policy) {
		response.RequiredScopes = []string{"read", "write"}
	}
	if isKlingMCPLoginPolicy(policy) {
		response.RequiredScopes = klingmcp.Scopes()
	}
	if response.AuthenticationDriver == "" {
		response.AuthenticationDriver = policy.AuthMode
	}
	if policy.BundleSHA256 != "" {
		response.BundleSha256 = &policy.BundleSHA256
	}
	return response
}

func connectorPublicationResponse(item domain.ConnectorPublication, revision domain.ConnectorRevision) *workspacev1.ConnectorPublication {
	return &workspacev1.ConnectorPublication{Source: item.PackageSource, ActiveRevisionId: item.ActiveRevisionID, State: string(item.State), Version: item.Version, Revision: connectorRevisionResponse(revision)}
}

func connectorInstallationDetailsResponse(item domain.ConnectorInstallation, revision domain.ConnectorRevision, upgradeAvailable bool) *workspacev1.ConnectorInstallation {
	base := connectorInstallationResponse(item)
	policy, _ := decodeConnectorRevisionPolicy(revision)
	base.PackageVersion = revision.Version
	base.Name = policy.Metadata.Name
	if base.Name == "" {
		base.Name = item.PackageSource
	}
	base.Name = connectorpackage.DisplayName(item.PackageSource, base.Name)
	base.Description = connectorpackage.DisplayDescription(item.PackageSource, policy.Metadata.Description)
	base.ExamplesZh = append([]string(nil), policy.Metadata.ExamplesZH...)
	base.ExamplesEn = append([]string(nil), policy.Metadata.ExamplesEN...)
	base.Mode = string(revision.Mode)
	base.Icon = connectorpackage.DisplayIcon(item.PackageSource)
	base.AuthenticationDriver = policy.AuthMode
	if policy.CLI != nil && policy.CLI.AuthenticationDriver != "" {
		base.AuthenticationDriver = policy.CLI.AuthenticationDriver
	}
	if item.AuthorizationID != "" {
		base.SelectedAuthorizationId = &item.AuthorizationID
	}
	base.UpgradeAvailable = upgradeAvailable
	return base
}

func connectorAuthorizationResponse(item domain.ConnectorAuthorization, selected bool) *workspacev1.ConnectorAuthorization {
	state := item.State
	if state == domain.ConnectorAuthorizationActive && item.ExpiresAt != nil && !time.Now().UTC().Before(*item.ExpiresAt) {
		state = domain.ConnectorAuthorizationExpired
	}
	response := &workspacev1.ConnectorAuthorization{Id: item.ID, InstallationId: item.InstallationID, IdentityRef: item.IdentityRef, ExternalIdentityId: item.ExternalIdentityID, ExternalDisplayName: item.ExternalDisplayName, Scopes: item.Scopes, State: string(state), Version: item.Version, Selected: selected}
	if item.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*item.ExpiresAt)
	}
	return response
}

func connectorSetupResponse(item domain.ConnectorSetup) *workspacev1.ConnectorSetup {
	response := &workspacev1.ConnectorSetup{Id: item.ID, InstallationId: item.InstallationID, State: item.State}
	if item.ActionURL != "" {
		response.ActionUrl = &item.ActionURL
	}
	if item.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*item.ExpiresAt)
	}
	if item.ProviderName != "" {
		response.ProviderName = &item.ProviderName
	}
	if item.DeveloperConsoleURL != "" {
		response.DeveloperConsoleUrl = &item.DeveloperConsoleURL
	}
	return response
}

func connectorAuthorizationFlowResponse(item domain.ConnectorAuthorizationAttempt, state string, authorization *domain.ConnectorAuthorization) *workspacev1.ConnectorAuthorizationFlow {
	response := &workspacev1.ConnectorAuthorizationFlow{Id: item.ID, InstallationId: item.InstallationID, Identity: item.Identity, Scopes: item.Scopes, State: state}
	if state == "waiting_for_user" {
		response.ActionUrl = &item.ActionURL
		response.ExpiresAt = timestamppb.New(item.ExpiresAt)
	}
	if authorization != nil {
		response.Authorization = connectorAuthorizationResponse(*authorization, true)
	}
	return response
}

func isLinearMCPPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "linear" && policy.AuthMode == "oauth" && policy.MCP != nil && policy.CLI == nil && policy.MCP.Transport == "streamable_http" && policy.MCP.URL == "https://mcp.linear.app/mcp" && len(policy.MCP.EgressHosts) == 1 && policy.MCP.EgressHosts[0] == "mcp.linear.app" && len(policy.MCP.Headers) == 0 && len(policy.MCP.Environment) == 0
}

func isPixsoMCPPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "pixso" && policy.AuthMode == "oauth" && policy.MCP != nil && policy.CLI == nil && policy.MCP.Transport == "streamable_http" && policy.MCP.URL == "https://pixso.net/mcp" && len(policy.MCP.EgressHosts) == 1 && policy.MCP.EgressHosts[0] == "pixso.net" && len(policy.MCP.Headers) == 0 && len(policy.MCP.Environment) == 0
}

func isTianyanchaMCPPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "tianyancha" && policy.AuthMode == "oauth" && policy.MCP != nil && policy.CLI == nil && policy.MCP.Transport == "streamable_http" && policy.MCP.URL == "https://mcp.tianyancha.com/mcp" && len(policy.MCP.EgressHosts) == 1 && policy.MCP.EgressHosts[0] == "mcp.tianyancha.com" && len(policy.MCP.Headers) == 0 && len(policy.MCP.Environment) == 0
}
