package workspace

import (
	"context"
	"fmt"
	"strings"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type resourceDeletionRepository interface {
	GetMCPServerDeletionImpact(context.Context, string, string) (workspacedomain.ResourceDeletionImpact, error)
	DeleteMCPServerConfirmed(context.Context, string, string, string) error
	GetSkillDeletionImpact(context.Context, string, string) (workspacedomain.ResourceDeletionImpact, error)
	DeleteSkillConfirmed(context.Context, string, string, string) error
}

func (service *Service) resourceDeletion() (resourceDeletionRepository, error) {
	repository, ok := service.workspace.Repository().(resourceDeletionRepository)
	if !ok {
		return nil, fmt.Errorf("resource deletion preview is unavailable")
	}
	return repository, nil
}

func (service *Service) ListExperts(ctx context.Context, _ *workspacev1.ListExpertsRequest) (*workspacev1.ListExpertsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListExperts(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.Expert, 0, len(items))
	availability, err := service.expertAvailability(ctx, items)
	if err != nil {
		return nil, publicError(err)
	}
	for _, item := range items {
		response = append(response, expertResponse(item, availability[item.ID]))
	}
	return &workspacev1.ListExpertsResponse{Items: response}, nil
}

func (service *Service) GetExpert(ctx context.Context, request *workspacev1.GetExpertRequest) (*workspacev1.Expert, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetExpert(ctx, owner, request.ExpertId)
	if err != nil {
		return nil, publicError(err)
	}
	availability, err := service.expertAvailability(ctx, []workspacedomain.Expert{item})
	if err != nil {
		return nil, publicError(err)
	}
	return expertResponse(item, availability[item.ID]), nil
}

func (service *Service) CreateExpert(ctx context.Context, request *workspacev1.CreateExpertRequest) (*workspacev1.Expert, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, err := expertInput(request.Expert)
	if err != nil {
		return nil, publicError(err)
	}
	if err := service.validateExpertInputAvailability(ctx, input); err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().CreateExpert(ctx, owner, input)
	if err != nil {
		return nil, publicError(err)
	}
	availability, err := service.expertAvailability(ctx, []workspacedomain.Expert{item})
	if err != nil {
		return nil, publicError(err)
	}
	return expertResponse(item, availability[item.ID]), nil
}

func (service *Service) UpdateExpert(ctx context.Context, request *workspacev1.UpdateExpertRequest) (*workspacev1.Expert, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, err := expertInput(request.Expert)
	if err != nil {
		return nil, publicError(err)
	}
	if err := service.validateExpertInputAvailability(ctx, input); err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().UpdateExpert(ctx, owner, request.ExpertId, input, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	availability, err := service.expertAvailability(ctx, []workspacedomain.Expert{item})
	if err != nil {
		return nil, publicError(err)
	}
	return expertResponse(item, availability[item.ID]), nil
}

func (service *Service) DeleteExpert(ctx context.Context, request *workspacev1.DeleteExpertRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.workspace.Repository().DeleteExpert(ctx, owner, request.ExpertId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) ListExpertTeams(ctx context.Context, _ *workspacev1.ListExpertTeamsRequest) (*workspacev1.ListExpertTeamsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListExpertTeams(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.ExpertTeam, 0, len(items))
	availability, err := service.teamExpertAvailability(ctx, items)
	if err != nil {
		return nil, publicError(err)
	}
	for _, item := range items {
		response = append(response, expertTeamResponse(item, availability))
	}
	return &workspacev1.ListExpertTeamsResponse{Items: response}, nil
}

func (service *Service) GetExpertTeam(ctx context.Context, request *workspacev1.GetExpertTeamRequest) (*workspacev1.ExpertTeam, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetExpertTeam(ctx, owner, request.ExpertTeamId)
	if err != nil {
		return nil, publicError(err)
	}
	availability, err := service.teamExpertAvailability(ctx, []workspacedomain.ExpertTeam{item})
	if err != nil {
		return nil, publicError(err)
	}
	return expertTeamResponse(item, availability), nil
}

func (service *Service) CreateExpertTeam(ctx context.Context, request *workspacev1.CreateExpertTeamRequest) (*workspacev1.ExpertTeam, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, err := expertTeamInput(request.ExpertTeam)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().CreateExpertTeam(ctx, owner, input)
	if err != nil {
		return nil, publicError(err)
	}
	availability, err := service.teamExpertAvailability(ctx, []workspacedomain.ExpertTeam{item})
	if err != nil {
		return nil, publicError(err)
	}
	return expertTeamResponse(item, availability), nil
}

func (service *Service) UpdateExpertTeam(ctx context.Context, request *workspacev1.UpdateExpertTeamRequest) (*workspacev1.ExpertTeam, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, err := expertTeamInput(request.ExpertTeam)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().UpdateExpertTeam(ctx, owner, request.ExpertTeamId, input, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	availability, err := service.teamExpertAvailability(ctx, []workspacedomain.ExpertTeam{item})
	if err != nil {
		return nil, publicError(err)
	}
	return expertTeamResponse(item, availability), nil
}

func (service *Service) DeleteExpertTeam(ctx context.Context, request *workspacev1.DeleteExpertTeamRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.workspace.Repository().DeleteExpertTeam(ctx, owner, request.ExpertTeamId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) GetSettings(ctx context.Context, _ *workspacev1.GetSettingsRequest) (*workspacev1.PersonalSettings, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().GetSettings(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	return settingsResponse(item), nil
}

func (service *Service) UpdateSettings(ctx context.Context, request *workspacev1.UpdateSettingsRequest) (*workspacev1.PersonalSettings, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	runtime, err := workspacedomain.ParseRuntime(request.DefaultRuntimeEngine)
	if err != nil {
		return nil, publicError(err)
	}
	defaults := make(map[workspacedomain.RuntimeEngine]string, len(request.RuntimeModelDefaults))
	for _, item := range request.RuntimeModelDefaults {
		if item == nil {
			continue
		}
		parsed, err := workspacedomain.ParseRuntime(item.RuntimeEngine)
		if err != nil {
			return nil, publicError(err)
		}
		defaults[parsed] = item.ProviderModelId
	}
	settings := workspacedomain.Settings{Personality: request.Personality, PersonalityInstructions: request.PersonalityInstructions, RuntimeModelDefaults: defaults, DefaultRuntimeEngine: runtime, Language: request.Language, Timezone: request.Timezone}
	item, err := service.workspace.Repository().UpdateSettings(ctx, owner, settings, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return settingsResponse(item), nil
}

func (service *Service) ListRuntimeEngines(ctx context.Context, _ *workspacev1.ListRuntimeEnginesRequest) (*workspacev1.ListRuntimeEnginesResponse, error) {
	if _, err := service.owner(ctx); err != nil {
		return nil, err
	}
	response := &workspacev1.ListRuntimeEnginesResponse{}
	for _, name := range []string{"claude", "codex", "hermes", "openclaw", "pi"} {
		configured, exists := service.config.Worker.Runtimes[name]
		response.Items = append(response.Items, &workspacev1.RuntimeEngineStatus{
			Name: name, Available: exists && configured.Available,
			NativeResume: exists && configured.Available && configured.NativeResume,
			CliVersion:   configured.CLIVersion,
		})
	}
	return response, nil
}

func (service *Service) ListModelProviderPresets(ctx context.Context, _ *workspacev1.ListModelProviderPresetsRequest) (*workspacev1.ListModelProviderPresetsResponse, error) {
	if _, err := service.owner(ctx); err != nil {
		return nil, err
	}
	response := &workspacev1.ListModelProviderPresetsResponse{}
	for _, preset := range workspacedomain.ModelProviderPresets() {
		response.Items = append(response.Items, &workspacev1.ModelProviderPreset{ProviderType: preset.ProviderType, DisplayName: preset.DisplayName, OfficialEndpoint: preset.OfficialEndpoint, Protocols: preset.Protocols})
	}
	return response, nil
}

func (service *Service) ListModelProviderConnections(ctx context.Context, _ *workspacev1.ListModelProviderConnectionsRequest) (*workspacev1.ListModelProviderConnectionsResponse, error) {
	if _, err := service.owner(ctx); err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListModelProviderConnections(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.ModelProviderConnection, 0, len(items))
	for _, item := range items {
		response = append(response, modelProviderConnectionResponse(item))
	}
	return &workspacev1.ListModelProviderConnectionsResponse{Items: response}, nil
}

func (service *Service) CreateModelProviderConnection(ctx context.Context, request *workspacev1.CreateModelProviderConnectionRequest) (*workspacev1.ModelProviderConnection, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	owner := administrator.UserID
	if err := workspacedomain.ValidateModelProviderConnection(request.Name, request.ProviderType, request.Endpoint, request.Protocols, request.ApiKey, true); err != nil {
		return nil, publicError(err)
	}
	connection := workspacedomain.ModelProviderConnection{Name: request.Name, ProviderType: request.ProviderType, Endpoint: request.Endpoint, Protocols: append([]string(nil), request.Protocols...), VerificationStatus: "unverified", CustomEndpoint: customProviderEndpoint(request.ProviderType, request.Endpoint)}
	catalog, discoveryErr := service.workspace.DiscoverProviderModels(ctx, connection, request.ApiKey)
	models := applyCatalogResult(&connection, catalog, discoveryErr)
	ciphertext, err := service.box.Encrypt([]byte(request.ApiKey), "model-provider:"+owner)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().CreateModelProviderConnection(ctx, owner, connection, ciphertext, models)
	if err != nil {
		return nil, publicError(err)
	}
	return modelProviderConnectionResponse(item), nil
}

func (service *Service) UpdateModelProviderConnection(ctx context.Context, request *workspacev1.UpdateModelProviderConnectionRequest) (*workspacev1.ModelProviderConnection, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	existing, err := service.findProviderConnection(ctx, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	replacement := ""
	if request.ReplacementApiKey != nil {
		replacement = *request.ReplacementApiKey
	}
	if err := workspacedomain.ValidateModelProviderConnection(request.Name, existing.ProviderType, request.Endpoint, request.Protocols, replacement, false); err != nil {
		return nil, publicError(err)
	}
	var ciphertext []byte
	apiKey := replacement
	if request.ReplacementApiKey != nil {
		ciphertext, err = service.box.Encrypt([]byte(replacement), "model-provider:"+existing.CredentialOwnerID)
		if err != nil {
			return nil, publicError(err)
		}
	} else {
		existingCiphertext, loadErr := service.workspace.Repository().GetModelProviderAPIKey(ctx, existing.CredentialOwnerID, request.ConnectionId)
		if loadErr != nil {
			return nil, publicError(loadErr)
		}
		plaintext, decryptErr := service.box.Decrypt(existingCiphertext, "model-provider:"+existing.CredentialOwnerID)
		if decryptErr != nil {
			return nil, publicError(decryptErr)
		}
		defer clear(plaintext)
		apiKey = string(plaintext)
	}
	connection := workspacedomain.ModelProviderConnection{Name: request.Name, ProviderType: existing.ProviderType, Endpoint: request.Endpoint, Protocols: append([]string(nil), request.Protocols...), VerificationStatus: "unverified", CustomEndpoint: customProviderEndpoint(existing.ProviderType, request.Endpoint)}
	catalog, discoveryErr := service.workspace.DiscoverProviderModels(ctx, connection, apiKey)
	models := applyCatalogResult(&connection, catalog, discoveryErr)
	item, err := service.workspace.Repository().UpdateModelProviderConnection(ctx, existing.CredentialOwnerID, request.ConnectionId, connection, ciphertext, models, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return modelProviderConnectionResponse(item), nil
}

func (service *Service) DeleteModelProviderConnection(ctx context.Context, request *workspacev1.DeleteModelProviderConnectionRequest) (*workspacev1.DeleteResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	existing, err := service.findProviderConnection(ctx, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	if err := service.workspace.Repository().DeleteModelProviderConnection(ctx, existing.CredentialOwnerID, request.ConnectionId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) RefreshProviderModels(ctx context.Context, request *workspacev1.RefreshProviderModelsRequest) (*workspacev1.ModelProviderConnection, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	connection, err := service.findProviderConnection(ctx, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	ciphertext, err := service.workspace.Repository().GetModelProviderAPIKey(ctx, connection.CredentialOwnerID, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	plaintext, err := service.box.Decrypt(ciphertext, "model-provider:"+connection.CredentialOwnerID)
	if err != nil {
		return nil, publicError(err)
	}
	defer clear(plaintext)
	catalog, discoveryErr := service.workspace.DiscoverProviderModels(ctx, connection, string(plaintext))
	models, status, syncError := catalog.Models, "unverified", ""
	if catalog.Source == "provider" {
		status = "verified"
	}
	if discoveryErr != nil {
		models = nil
		syncError = discoveryErr.Error()
	}
	updated, err := service.workspace.Repository().ReplaceProviderModels(ctx, connection.CredentialOwnerID, request.ConnectionId, models, status, syncError)
	if err != nil {
		return nil, publicError(err)
	}
	return modelProviderConnectionResponse(updated), nil
}

func applyCatalogResult(connection *workspacedomain.ModelProviderConnection, result workspaceapplication.ModelCatalogResult, discoveryErr error) []workspacedomain.ProviderModel {
	if discoveryErr != nil {
		connection.VerificationError = discoveryErr.Error()
		return nil
	}
	if result.Source == "provider" {
		connection.VerificationStatus = "verified"
	}
	return result.Models
}

func (service *Service) CreateProviderModel(ctx context.Context, request *workspacev1.CreateProviderModelRequest) (*workspacev1.ProviderModel, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	connection, err := service.findProviderConnection(ctx, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	if err := workspacedomain.ValidateProviderModel(request.ModelId, request.ModelId); err != nil {
		return nil, publicError(err)
	}
	model := workspacedomain.ProviderModel{ModelID: request.ModelId, DisplayName: request.ModelId, Available: true, ManuallyAdded: true, Compatibility: workspacedomain.CompatibilityForProtocols(connection.Protocols)}
	item, err := service.workspace.Repository().CreateProviderModel(ctx, request.ConnectionId, model)
	if err != nil {
		return nil, publicError(err)
	}
	return providerModelResponse(item), nil
}

func (service *Service) ListMCPConnectors(ctx context.Context, _ *workspacev1.ListMCPConnectorsRequest) (*workspacev1.ListMCPConnectorsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListMCPServers(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.MCPConnector, 0, len(items))
	for _, item := range items {
		response = append(response, mcpResponse(item))
	}
	return &workspacev1.ListMCPConnectorsResponse{Items: response}, nil
}

func (service *Service) CreateMCPConnector(ctx context.Context, request *workspacev1.CreateMCPConnectorRequest) (*workspacev1.MCPConnector, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, submittedSecrets, err := service.mcpInput(request.McpConnector)
	if err != nil {
		return nil, publicError(err)
	}
	secrets, err := service.encryptEnvironmentSecrets(owner, "mcp-server:", input.Environment, submittedSecrets, nil)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().CreateMCPServer(ctx, owner, input, secrets)
	if err != nil {
		return nil, publicError(err)
	}
	return mcpResponse(item), nil
}

func (service *Service) UpdateMCPConnector(ctx context.Context, request *workspacev1.UpdateMCPConnectorRequest) (*workspacev1.MCPConnector, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, submittedSecrets, err := service.mcpInput(request.McpConnector)
	if err != nil {
		return nil, publicError(err)
	}
	existingCiphertext, err := service.workspace.Repository().GetMCPSecret(ctx, owner, request.McpConnectorId)
	if err != nil {
		return nil, publicError(err)
	}
	secrets, err := service.encryptEnvironmentSecrets(owner, "mcp-server:", input.Environment, submittedSecrets, existingCiphertext)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().UpdateMCPServer(ctx, owner, request.McpConnectorId, input, secrets, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return mcpResponse(item), nil
}

func (service *Service) TestMCPConnector(ctx context.Context, request *workspacev1.TestMCPConnectorRequest) (*workspacev1.MCPConnector, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().RequestMCPTest(ctx, owner, request.McpConnectorId)
	if err != nil {
		return nil, publicError(err)
	}
	return mcpResponse(item), nil
}

func (service *Service) GetMCPConnectorDeletionImpact(ctx context.Context, request *workspacev1.GetMCPConnectorDeletionImpactRequest) (*workspacev1.ResourceDeletionImpact, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.resourceDeletion()
	if err != nil {
		return nil, publicError(err)
	}
	impact, err := repository.GetMCPServerDeletionImpact(ctx, owner, request.McpConnectorId)
	if err != nil {
		return nil, publicError(err)
	}
	return deletionImpactResponse(impact), nil
}

func (service *Service) DeleteMCPConnector(ctx context.Context, request *workspacev1.DeleteMCPConnectorRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.resourceDeletion()
	if err != nil {
		return nil, publicError(err)
	}
	if err := repository.DeleteMCPServerConfirmed(ctx, owner, request.McpConnectorId, request.ConfirmationToken); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) ListSkills(ctx context.Context, _ *workspacev1.ListSkillsRequest) (*workspacev1.ListSkillsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListSkills(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.Skill, 0, len(items))
	for _, item := range items {
		response = append(response, skillResponse(item))
	}
	return &workspacev1.ListSkillsResponse{Items: response}, nil
}

func (service *Service) CreateSkill(ctx context.Context, request *workspacev1.CreateSkillRequest) (*workspacev1.Skill, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len(name) > 100 {
		return nil, publicError(fmt.Errorf("%w: invalid Skill name", workspacedomain.ErrInvalid))
	}
	var objectKey, digest string
	var gitRef *string
	switch request.Source {
	case "upload":
		objectKey, digest, err = service.skills.InstallUpload(ctx, owner, request.Archive)
	case "git":
		if request.GitUrl == nil {
			err = fmt.Errorf("Git URL is required")
			break
		}
		ref := ""
		if request.GitRef != nil {
			ref = *request.GitRef
		}
		var resolved string
		objectKey, digest, resolved, err = service.skills.InstallGit(ctx, owner, *request.GitUrl, ref)
		if err == nil {
			gitRef = &resolved
		}
	default:
		err = fmt.Errorf("Skill source must be git or upload")
	}
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	item, err := service.workspace.Repository().CreateSkill(ctx, owner, workspacedomain.Skill{Name: name, Source: request.Source, GitURL: request.GitUrl, GitRef: gitRef, ObjectKey: objectKey, SHA256: digest})
	if err != nil {
		return nil, publicError(err)
	}
	return skillResponse(item), nil
}

func (service *Service) UpdateSkill(ctx context.Context, request *workspacev1.UpdateSkillRequest) (*workspacev1.Skill, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListSkills(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	var current *workspacedomain.Skill
	for index := range items {
		if items[index].ID == request.SkillId {
			current = &items[index]
			break
		}
	}
	if current == nil {
		return nil, publicError(workspacedomain.ErrNotFound)
	}
	var objectKey, digest string
	var resolvedRef *string
	if current.Source == "git" {
		ref := ""
		if request.GitRef != nil {
			ref = *request.GitRef
		}
		var resolved string
		objectKey, digest, resolved, err = service.skills.InstallGit(ctx, owner, *current.GitURL, ref)
		resolvedRef = &resolved
	} else {
		objectKey, digest, err = service.skills.InstallUpload(ctx, owner, request.Archive)
	}
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	item, err := service.workspace.Repository().UpdateSkill(ctx, owner, request.SkillId, resolvedRef, objectKey, digest, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return skillResponse(item), nil
}

func (service *Service) DeleteSkill(ctx context.Context, request *workspacev1.DeleteSkillRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.resourceDeletion()
	if err != nil {
		return nil, publicError(err)
	}
	if err := repository.DeleteSkillConfirmed(ctx, owner, request.SkillId, request.ConfirmationToken); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) GetSkillDeletionImpact(ctx context.Context, request *workspacev1.GetSkillDeletionImpactRequest) (*workspacev1.ResourceDeletionImpact, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.resourceDeletion()
	if err != nil {
		return nil, publicError(err)
	}
	impact, err := repository.GetSkillDeletionImpact(ctx, owner, request.SkillId)
	if err != nil {
		return nil, publicError(err)
	}
	return deletionImpactResponse(impact), nil
}

func deletionImpactResponse(impact workspacedomain.ResourceDeletionImpact) *workspacev1.ResourceDeletionImpact {
	experts := make([]*workspacev1.AffectedExpert, 0, len(impact.AffectedExperts))
	for _, expert := range impact.AffectedExperts {
		experts = append(experts, &workspacev1.AffectedExpert{Id: expert.ID, Name: expert.Name, Version: expert.Version})
	}
	return &workspacev1.ResourceDeletionImpact{AffectedExperts: experts, ConfirmationToken: impact.ConfirmationToken}
}

func expertInput(input *workspacev1.ExpertInput) (workspacedomain.ExpertInput, error) {
	if input == nil {
		return workspacedomain.ExpertInput{}, fmt.Errorf("%w: Expert input is required", workspacedomain.ErrInvalid)
	}
	return workspacedomain.ExpertInput{Name: input.Name, Icon: input.Icon, IconBackground: input.IconBackground, Introduction: input.Introduction, CoreCapability: input.CoreCapability, OperatingProcedure: input.OperatingProcedure, OutputStandard: input.OutputStandard, Cautions: input.Cautions, MCPServerIDs: append([]string(nil), input.McpServerIds...), SkillIDs: append([]string(nil), input.SkillIds...), CLIConnectorDefinitionIDs: append([]string(nil), input.CliConnectorDefinitionIds...)}, nil
}

func expertResponse(item workspacedomain.Expert, status expertAvailabilityStatus) *workspacev1.Expert {
	response := &workspacev1.Expert{Id: item.ID, Name: item.Name, Icon: item.Icon, IconBackground: item.IconBackground, Introduction: item.Introduction, CoreCapability: item.CoreCapability, OperatingProcedure: item.OperatingProcedure, OutputStandard: item.OutputStandard, Cautions: item.Cautions, ExpertiseTags: item.ExpertiseTags, McpServerIds: item.MCPServerIDs, SkillIds: item.SkillIDs, CliConnectorDefinitionIds: item.CLIConnectorDefinitionIDs, Complete: status.Complete, Available: status.Available, Compatibility: status.Compatibility, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version, TagProjectionStatus: item.TagProjectionStatus}
	if item.TagProjectionError != "" {
		response.TagProjectionError = &item.TagProjectionError
	}
	if status.Reason != "" {
		response.AvailabilityReason = &status.Reason
	}
	return response
}

func expertTeamInput(input *workspacev1.ExpertTeamInput) (workspacedomain.ExpertTeamInput, error) {
	if input == nil {
		return workspacedomain.ExpertTeamInput{}, fmt.Errorf("%w: Expert Team input is required", workspacedomain.ErrInvalid)
	}
	members := make([]workspacedomain.ExpertTeamMemberInput, 0, len(input.Members))
	for _, member := range input.Members {
		members = append(members, workspacedomain.ExpertTeamMemberInput{ID: member.Id, Name: member.Name, ExpertID: member.ExpertId, Labels: append([]string(nil), member.Labels...)})
	}
	return workspacedomain.ExpertTeamInput{Name: input.Name, Icon: input.Icon, IconBackground: input.IconBackground, Introduction: input.Introduction, CoreCapability: input.CoreCapability, Members: members}, nil
}

func expertTeamResponse(item workspacedomain.ExpertTeam, availability map[string]expertAvailabilityStatus) *workspacev1.ExpertTeam {
	experts := make([]*workspacev1.Expert, 0, len(item.Experts))
	available := len(item.Experts) >= 2
	for _, expert := range item.Experts {
		status := availability[expert.ID]
		available = available && status.Available
		experts = append(experts, expertResponse(expert, status))
	}
	members := make([]*workspacev1.ExpertTeamMember, 0, len(item.Members))
	if len(item.Members) > 0 {
		available = len(item.Members) >= 2
	}
	for _, member := range item.Members {
		status := availability[member.Expert.ID]
		available = available && status.Available
		members = append(members, &workspacev1.ExpertTeamMember{Id: member.ID, Name: member.Name, Expert: expertResponse(member.Expert, status), Labels: member.Labels, Position: int32(member.Position)})
	}
	return &workspacev1.ExpertTeam{Id: item.ID, Name: item.Name, Icon: item.Icon, IconBackground: item.IconBackground, Introduction: item.Introduction, CoreCapability: item.CoreCapability, Members: members, CapabilityIntroduction: item.CapabilityIntroduction, ExpertiseTags: item.ExpertiseTags, Experts: experts, Available: available, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
}

func (service *Service) teamExpertAvailability(ctx context.Context, teams []workspacedomain.ExpertTeam) (map[string]expertAvailabilityStatus, error) {
	var experts []workspacedomain.Expert
	for _, team := range teams {
		experts = append(experts, team.Experts...)
		for _, member := range team.Members {
			experts = append(experts, member.Expert)
		}
	}
	return service.expertAvailability(ctx, experts)
}

type expertAvailabilityStatus struct {
	Complete      bool
	Available     bool
	Compatibility string
	ModelName     string
	Reason        string
}

func (service *Service) expertAvailability(ctx context.Context, experts []workspacedomain.Expert) (map[string]expertAvailabilityStatus, error) {
	models := make(map[string]workspacedomain.ProviderModel)
	needsLegacyCatalog := false
	for _, expert := range experts {
		if strings.TrimSpace(expert.CoreCapability) == "" && expert.Available() {
			needsLegacyCatalog = true
			break
		}
	}
	if needsLegacyCatalog {
		connections, err := service.workspace.Repository().ListModelProviderConnections(ctx)
		if err != nil {
			return nil, err
		}
		for _, connection := range connections {
			for _, model := range connection.Models {
				models[model.ID] = model
			}
		}
	}
	result := make(map[string]expertAvailabilityStatus, len(experts))
	for _, expert := range experts {
		status := expertAvailabilityStatus{Complete: expert.Available(), Compatibility: "unavailable"}
		if !status.Complete {
			status.Reason = "Expert guidance is incomplete"
			result[expert.ID] = status
			continue
		}
		if strings.TrimSpace(expert.CoreCapability) != "" {
			status.Available = true
			status.Compatibility = "verified"
			result[expert.ID] = status
			continue
		}
		runtimeConfig, runtimeAvailable := service.config.Worker.Runtimes[string(expert.RuntimeEngine)]
		model, modelExists := models[expert.ProviderModelID]
		status.ModelName = model.DisplayName
		for _, item := range model.Compatibility {
			if item.RuntimeEngine == expert.RuntimeEngine {
				status.Compatibility = item.Status
			}
		}
		status.Available = runtimeAvailable && runtimeConfig.Available && modelExists && model.Available && status.Compatibility != "incompatible" && status.Compatibility != "unavailable"
		if !status.Available {
			status.Reason = "Expert model, connection, Runtime Engine, or compatibility is unavailable"
		}
		result[expert.ID] = status
	}
	return result, nil
}

func settingsResponse(item workspacedomain.Settings) *workspacev1.PersonalSettings {
	response := &workspacev1.PersonalSettings{Personality: item.Personality, PersonalityInstructions: item.PersonalityInstructions, DefaultRuntimeEngine: string(item.DefaultRuntimeEngine), Language: item.Language, Timezone: item.Timezone, Version: item.Version}
	for _, runtime := range []workspacedomain.RuntimeEngine{workspacedomain.RuntimeClaude, workspacedomain.RuntimeCodex, workspacedomain.RuntimeHermes, workspacedomain.RuntimeOpenClaw, workspacedomain.RuntimePI} {
		if modelID := item.RuntimeModelDefaults[runtime]; modelID != "" {
			response.RuntimeModelDefaults = append(response.RuntimeModelDefaults, &workspacev1.RuntimeModelDefault{RuntimeEngine: string(runtime), ProviderModelId: modelID})
		}
	}
	return response
}

func modelProviderConnectionResponse(item workspacedomain.ModelProviderConnection) *workspacev1.ModelProviderConnection {
	response := &workspacev1.ModelProviderConnection{Id: item.ID, Name: item.Name, ProviderType: item.ProviderType, Endpoint: item.Endpoint, Protocols: item.Protocols, ApiKeyConfigured: item.HasAPIKey, VerificationStatus: item.VerificationStatus, CustomEndpoint: item.CustomEndpoint, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
	if item.VerificationError != "" {
		response.VerificationError = &item.VerificationError
	}
	if item.LastSyncedAt != nil {
		response.LastSyncedAt = timestamppb.New(*item.LastSyncedAt)
	}
	if item.LastSyncError != "" {
		response.LastSyncError = &item.LastSyncError
	}
	for _, model := range item.Models {
		response.Models = append(response.Models, providerModelResponse(model))
	}
	return response
}

func providerModelResponse(item workspacedomain.ProviderModel) *workspacev1.ProviderModel {
	response := &workspacev1.ProviderModel{Id: item.ID, ConnectionId: item.ConnectionID, ModelId: item.ModelID, DisplayName: item.DisplayName, Available: item.Available, ManuallyAdded: item.ManuallyAdded}
	for _, value := range item.Compatibility {
		compatibility := &workspacev1.RuntimeModelCompatibility{RuntimeEngine: string(value.RuntimeEngine), Status: value.Status}
		if value.Reason != "" {
			compatibility.Reason = &value.Reason
		}
		response.Compatibility = append(response.Compatibility, compatibility)
	}
	return response
}

func (service *Service) findProviderConnection(ctx context.Context, connectionID string) (workspacedomain.ModelProviderConnection, error) {
	items, err := service.workspace.Repository().ListModelProviderConnections(ctx)
	if err != nil {
		return workspacedomain.ModelProviderConnection{}, err
	}
	for _, item := range items {
		if item.ID == connectionID {
			return item, nil
		}
	}
	return workspacedomain.ModelProviderConnection{}, workspacedomain.ErrNotFound
}

func customProviderEndpoint(providerType, endpoint string) bool {
	for _, preset := range workspacedomain.ModelProviderPresets() {
		if preset.ProviderType == providerType {
			return strings.TrimRight(strings.TrimSpace(endpoint), "/") != strings.TrimRight(preset.OfficialEndpoint, "/")
		}
	}
	return true
}

func (service *Service) mcpInput(input *workspacev1.MCPConnectorInput) (workspacedomain.MCPServer, map[string]string, error) {
	if input == nil {
		return workspacedomain.MCPServer{}, nil, fmt.Errorf("%w: MCP input is required", workspacedomain.ErrInvalid)
	}
	item := workspacedomain.MCPServer{Name: input.Name, Transport: input.Transport, URL: input.Url, Runner: input.Runner, Package: input.Package, PackageVersion: input.PackageVersion, Arguments: append([]string(nil), input.Arguments...)}
	secretValues := make(map[string]string)
	for _, value := range input.Environment {
		if value == nil {
			continue
		}
		environment := workspacedomain.EnvironmentVariable{Name: value.Name, Secret: value.Secret, Configured: value.Configured || value.Value != nil}
		if value.Value != nil {
			if value.Secret {
				secretValues[value.Name] = *value.Value
			} else {
				environment.Value = *value.Value
			}
		}
		item.Environment = append(item.Environment, environment)
	}
	if err := item.Validate(); err != nil {
		return workspacedomain.MCPServer{}, nil, err
	}
	return item, secretValues, nil
}

func mcpResponse(item workspacedomain.MCPServer) *workspacev1.MCPConnector {
	response := &workspacev1.MCPConnector{Id: item.ID, Name: item.Name, Transport: item.Transport, Url: item.URL, Runner: item.Runner, Package: item.Package, PackageVersion: item.PackageVersion, Arguments: item.Arguments, Tested: item.TestedAt != nil && item.TestError == "", TestPending: item.TestRequestedAt != nil, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
	if item.TestError != "" {
		response.TestError = &item.TestError
	}
	for _, value := range item.Environment {
		entry := &workspacev1.EnvironmentVariable{Name: value.Name, Secret: value.Secret, Configured: value.Configured}
		if !value.Secret && value.Value != "" {
			entry.Value = &value.Value
		}
		response.Environment = append(response.Environment, entry)
	}
	return response
}

func skillResponse(item workspacedomain.Skill) *workspacev1.Skill {
	return &workspacev1.Skill{Id: item.ID, Name: item.Name, Source: item.Source, GitUrl: item.GitURL, GitRef: item.GitRef, Sha256: item.SHA256, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
}
