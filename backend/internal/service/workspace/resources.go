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
	for _, item := range items {
		response = append(response, expertResponse(item))
	}
	return &workspacev1.ListExpertsResponse{Items: response}, nil
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
	item, err := service.workspace.Repository().CreateExpert(ctx, owner, input)
	if err != nil {
		return nil, publicError(err)
	}
	return expertResponse(item), nil
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
	item, err := service.workspace.Repository().UpdateExpert(ctx, owner, request.ExpertId, input, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return expertResponse(item), nil
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
	for _, name := range []string{"claude", "codex", "hermes", "openclaw"} {
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
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListModelProviderConnections(ctx, owner)
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
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
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
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	existing, err := service.findProviderConnection(ctx, owner, request.ConnectionId)
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
		ciphertext, err = service.box.Encrypt([]byte(replacement), "model-provider:"+owner)
		if err != nil {
			return nil, publicError(err)
		}
	} else {
		existingCiphertext, loadErr := service.workspace.Repository().GetModelProviderAPIKey(ctx, owner, request.ConnectionId)
		if loadErr != nil {
			return nil, publicError(loadErr)
		}
		plaintext, decryptErr := service.box.Decrypt(existingCiphertext, "model-provider:"+owner)
		if decryptErr != nil {
			return nil, publicError(decryptErr)
		}
		defer clear(plaintext)
		apiKey = string(plaintext)
	}
	connection := workspacedomain.ModelProviderConnection{Name: request.Name, ProviderType: existing.ProviderType, Endpoint: request.Endpoint, Protocols: append([]string(nil), request.Protocols...), VerificationStatus: "unverified", CustomEndpoint: customProviderEndpoint(existing.ProviderType, request.Endpoint)}
	catalog, discoveryErr := service.workspace.DiscoverProviderModels(ctx, connection, apiKey)
	models := applyCatalogResult(&connection, catalog, discoveryErr)
	item, err := service.workspace.Repository().UpdateModelProviderConnection(ctx, owner, request.ConnectionId, connection, ciphertext, models, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return modelProviderConnectionResponse(item), nil
}

func (service *Service) DeleteModelProviderConnection(ctx context.Context, request *workspacev1.DeleteModelProviderConnectionRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.workspace.Repository().DeleteModelProviderConnection(ctx, owner, request.ConnectionId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) RefreshProviderModels(ctx context.Context, request *workspacev1.RefreshProviderModelsRequest) (*workspacev1.ModelProviderConnection, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	connection, err := service.findProviderConnection(ctx, owner, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	ciphertext, err := service.workspace.Repository().GetModelProviderAPIKey(ctx, owner, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	plaintext, err := service.box.Decrypt(ciphertext, "model-provider:"+owner)
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
	updated, err := service.workspace.Repository().ReplaceProviderModels(ctx, owner, request.ConnectionId, models, status, syncError)
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
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	connection, err := service.findProviderConnection(ctx, owner, request.ConnectionId)
	if err != nil {
		return nil, publicError(err)
	}
	displayName := ""
	if request.DisplayName != nil {
		displayName = *request.DisplayName
	}
	if err := workspacedomain.ValidateProviderModel(request.ModelId, displayName); err != nil {
		return nil, publicError(err)
	}
	model := workspacedomain.ProviderModel{ModelID: request.ModelId, DisplayName: displayName, Available: true, ManuallyAdded: true, Compatibility: workspacedomain.CompatibilityForProtocols(connection.Protocols)}
	item, err := service.workspace.Repository().CreateProviderModel(ctx, owner, request.ConnectionId, model)
	if err != nil {
		return nil, publicError(err)
	}
	return providerModelResponse(item), nil
}

func (service *Service) ListMCPServers(ctx context.Context, _ *workspacev1.ListMCPServersRequest) (*workspacev1.ListMCPServersResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	items, err := service.workspace.Repository().ListMCPServers(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	response := make([]*workspacev1.MCPServer, 0, len(items))
	for _, item := range items {
		response = append(response, mcpResponse(item))
	}
	return &workspacev1.ListMCPServersResponse{Items: response}, nil
}

func (service *Service) CreateMCPServer(ctx context.Context, request *workspacev1.CreateMCPServerRequest) (*workspacev1.MCPServer, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, submittedSecrets, err := service.mcpInput(request.McpServer)
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

func (service *Service) UpdateMCPServer(ctx context.Context, request *workspacev1.UpdateMCPServerRequest) (*workspacev1.MCPServer, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	input, submittedSecrets, err := service.mcpInput(request.McpServer)
	if err != nil {
		return nil, publicError(err)
	}
	existingCiphertext, err := service.workspace.Repository().GetMCPSecret(ctx, owner, request.McpServerId)
	if err != nil {
		return nil, publicError(err)
	}
	secrets, err := service.encryptEnvironmentSecrets(owner, "mcp-server:", input.Environment, submittedSecrets, existingCiphertext)
	if err != nil {
		return nil, publicError(err)
	}
	item, err := service.workspace.Repository().UpdateMCPServer(ctx, owner, request.McpServerId, input, secrets, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return mcpResponse(item), nil
}

func (service *Service) TestMCPServer(ctx context.Context, request *workspacev1.TestMCPServerRequest) (*workspacev1.MCPServer, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	item, err := service.workspace.Repository().RequestMCPTest(ctx, owner, request.McpServerId)
	if err != nil {
		return nil, publicError(err)
	}
	return mcpResponse(item), nil
}

func (service *Service) DeleteMCPServer(ctx context.Context, request *workspacev1.DeleteMCPServerRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.workspace.Repository().DeleteMCPServer(ctx, owner, request.McpServerId); err != nil {
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
	if err := service.workspace.Repository().DeleteSkill(ctx, owner, request.SkillId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func expertInput(input *workspacev1.ExpertInput) (workspacedomain.ExpertInput, error) {
	if input == nil {
		return workspacedomain.ExpertInput{}, fmt.Errorf("%w: Expert input is required", workspacedomain.ErrInvalid)
	}
	return workspacedomain.ExpertInput{Name: input.Name, Description: input.Description, MCPServerIDs: append([]string(nil), input.McpServerIds...), SkillIDs: append([]string(nil), input.SkillIds...)}, nil
}

func expertResponse(item workspacedomain.Expert) *workspacev1.Expert {
	return &workspacev1.Expert{Id: item.ID, Name: item.Name, Description: item.Description, McpServerIds: item.MCPServerIDs, SkillIds: item.SkillIDs, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
}

func settingsResponse(item workspacedomain.Settings) *workspacev1.PersonalSettings {
	response := &workspacev1.PersonalSettings{Personality: item.Personality, PersonalityInstructions: item.PersonalityInstructions, DefaultRuntimeEngine: string(item.DefaultRuntimeEngine), Language: item.Language, Timezone: item.Timezone, Version: item.Version}
	for _, runtime := range []workspacedomain.RuntimeEngine{workspacedomain.RuntimeClaude, workspacedomain.RuntimeCodex, workspacedomain.RuntimeHermes, workspacedomain.RuntimeOpenClaw} {
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

func (service *Service) findProviderConnection(ctx context.Context, owner, connectionID string) (workspacedomain.ModelProviderConnection, error) {
	items, err := service.workspace.Repository().ListModelProviderConnections(ctx, owner)
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

func (service *Service) mcpInput(input *workspacev1.MCPServerInput) (workspacedomain.MCPServer, map[string]string, error) {
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

func mcpResponse(item workspacedomain.MCPServer) *workspacev1.MCPServer {
	response := &workspacev1.MCPServer{Id: item.ID, Name: item.Name, Transport: item.Transport, Url: item.URL, Runner: item.Runner, Package: item.Package, PackageVersion: item.PackageVersion, Arguments: item.Arguments, Tested: item.TestedAt != nil && item.TestError == "", TestPending: item.TestRequestedAt != nil, CreatedAt: timestamppb.New(item.CreatedAt), UpdatedAt: timestamppb.New(item.UpdatedAt), Version: item.Version}
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
