package workspace

import (
	"context"
	"fmt"
	"strconv"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func (service *Service) conversationRepository() (application.ConversationRepository, error) {
	repository, ok := service.workspace.Repository().(application.ConversationRepository)
	if !ok {
		return nil, fmt.Errorf("conversation selection repository is unavailable")
	}
	return repository, nil
}

func (service *Service) GetConversationSelection(ctx context.Context, request *workspacev1.GetConversationSelectionRequest) (*workspacev1.ConversationSelection, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.conversationRepository()
	if err != nil {
		return nil, publicError(err)
	}
	selection, err := repository.GetConversationSelection(ctx, owner, domain.ConversationScope{SessionID: request.SessionId, WorkflowID: request.WorkflowId, RunID: request.RunId})
	if err != nil {
		return nil, publicError(err)
	}
	return conversationSelectionResponse(selection), nil
}

func (service *Service) ResolveConversationSelection(ctx context.Context, request *workspacev1.ResolveConversationSelectionRequest) (*workspacev1.ConversationSelection, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.conversationRepository()
	if err != nil {
		return nil, publicError(err)
	}
	selection, err := repository.ResolveConversationSelection(ctx, owner, domain.ConversationScope{SessionID: request.SessionId, WorkflowID: request.WorkflowId, RunID: request.RunId}, domain.ConversationSelectionInput{PreviousID: request.PreviousId, ChangeExpert: request.ChangeExpert, ExpertID: request.ExpertId, ExpertTeamID: request.ExpertTeamId, SkillIDs: request.SkillIds, MCPServerIDs: request.McpServerIds, CLIConnectorIDs: request.CliConnectorIds, DisabledConnectors: request.DisabledConnectors, RefreshIDs: request.RefreshIds})
	if err != nil {
		return nil, publicError(err)
	}
	return conversationSelectionResponse(selection), nil
}

func conversationSelectionResponse(item domain.ConversationSelection) *workspacev1.ConversationSelection {
	response := &workspacev1.ConversationSelection{Id: item.ID, ExpertId: item.ExpertID, ExpertTeamId: item.ExpertTeamID, Name: item.Name, Icon: item.Icon, IconBackground: item.IconBackground, DisabledConnectors: item.DisabledConnectors, MemberCount: int32(len(item.Defaults))}
	for _, skill := range item.Skills {
		response.Skills = append(response.Skills, &workspacev1.SelectedResource{Id: skill.ID, Name: skill.Name, Revision: skill.SHA256})
	}
	for _, server := range item.MCPServers {
		response.McpServers = append(response.McpServers, &workspacev1.SelectedResource{Id: server.ID, Name: server.Name, Icon: server.Icon})
	}
	for _, connector := range item.CLIConnectors {
		response.CliConnectors = append(response.CliConnectors, &workspacev1.SelectedResource{Id: connector.ID, Name: connector.Name, Icon: connector.Icon, Revision: strconv.FormatInt(connector.Version, 10)})
	}
	seen := map[string]bool{}
	for _, stage := range item.Defaults {
		for _, skill := range stage.Skills {
			key := "skill:" + skill.ID
			if !seen[key] {
				seen[key] = true
				response.InheritedSkills = append(response.InheritedSkills, &workspacev1.SelectedResource{Id: skill.ID, Name: skill.Name, Revision: skill.SHA256})
			}
		}
		for _, server := range stage.MCPServers {
			key := "mcp:" + server.ID
			if !seen[key] {
				seen[key] = true
				response.InheritedMcpServers = append(response.InheritedMcpServers, &workspacev1.SelectedResource{Id: server.ID, Name: server.Name, Icon: server.Icon})
			}
		}
		for _, connector := range stage.CLIConnectors {
			key := "cli:" + connector.ID
			if !seen[key] {
				seen[key] = true
				response.InheritedCliConnectors = append(response.InheritedCliConnectors, &workspacev1.SelectedResource{Id: connector.ID, Name: connector.Name, Icon: connector.Icon, Revision: strconv.FormatInt(connector.Version, 10)})
			}
		}
	}
	return response
}

func (service *Service) GetSkillDocument(ctx context.Context, request *workspacev1.GetSkillDocumentRequest) (*workspacev1.SkillDocument, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	skills, err := service.workspace.Repository().ListSkills(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	for _, skill := range skills {
		if skill.ID == request.SkillId {
			content, err := service.skills.Document(ctx, skill.ObjectKey, skill.SHA256)
			if err != nil {
				return nil, publicError(err)
			}
			return &workspacev1.SkillDocument{Skill: skillResponse(skill), Content: content}, nil
		}
	}
	return nil, publicError(domain.ErrNotFound)
}
