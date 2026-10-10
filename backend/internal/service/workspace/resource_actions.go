package workspace

import (
	"agent-platform/backend/internal/strictjson"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/resourceaction"
	"agent-platform/backend/internal/skillstore"
)

type resourceCreationActionRepository interface {
	GetResourceCreationAction(context.Context, string, string) (workspacedomain.ResourceCreationAction, resourceaction.Proposal, error)
	ClaimResourceCreationAction(context.Context, string, string) (workspacedomain.ResourceCreationAction, resourceaction.Proposal, error)
	CancelResourceCreationAction(context.Context, string, string) (workspacedomain.ResourceCreationAction, error)
	CompleteResourceCreationAction(context.Context, string, string, string, string, error) (workspacedomain.ResourceCreationAction, error)
}

func (service *Service) resourceCreationActions() (resourceCreationActionRepository, error) {
	repository, ok := service.workspace.Repository().(resourceCreationActionRepository)
	if !ok {
		return nil, fmt.Errorf("resource creation actions are unavailable")
	}
	return repository, nil
}

func (service *Service) decideResourceCreationAction(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if request.Method == http.MethodGet && len(parts) == 4 && parts[2] == "resource-creation-actions" {
		owner, err := service.owner(request.Context())
		if err != nil {
			writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
			return
		}
		repository, err := service.resourceCreationActions()
		if err != nil {
			writeResourceActionError(writer, err)
			return
		}
		action, proposal, err := repository.GetResourceCreationAction(request.Context(), owner, parts[3])
		if err != nil {
			writeResourceActionError(writer, err)
			return
		}
		if proposal.Kind == resourceaction.TeamKind {
			if _, err := service.administrator(request.Context()); err != nil {
				writeAuthError(writer, http.StatusForbidden, "administrator_required")
				return
			}
		}
		result := resourceCreationActionJSON(action)
		result["proposal"] = proposal
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(writer).Encode(result)
		return
	}
	if len(parts) != 5 || parts[2] != "resource-creation-actions" || parts[4] != "decision" {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	var input struct {
		Decision        string                   `json:"decision"`
		Proposal        *resourceaction.Proposal `json:"proposal,omitempty"`
		ExpectedVersion int64                    `json:"expected_version,omitempty"`
	}
	body, readErr := io.ReadAll(http.MaxBytesReader(writer, request.Body, 4*1024*1024))
	if readErr != nil || strictjson.Decode(body, &input) != nil {
		writeAuthError(writer, http.StatusBadRequest, "invalid_request_body")
		return
	}
	repository, err := service.resourceCreationActions()
	if err != nil {
		writeAuthError(writer, http.StatusServiceUnavailable, "resource_action_unavailable")
		return
	}
	actionID := parts[3]
	current, preview, err := repository.GetResourceCreationAction(request.Context(), owner, actionID)
	if err != nil {
		writeResourceActionError(writer, err)
		return
	}
	if preview.Kind == resourceaction.TeamKind {
		if _, err := service.administrator(request.Context()); err != nil {
			writeAuthError(writer, http.StatusForbidden, "administrator_required")
			return
		}
	}
	if input.Decision == "revise" {
		revisionRepo, ok := service.workspace.Repository().(interface {
			ReviseExpertCreationAction(context.Context, string, string, resourceaction.Proposal, int64) (workspacedomain.ResourceCreationAction, error)
		})
		if !ok || input.Proposal == nil {
			writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_resource_preview")
			return
		}
		action, err := revisionRepo.ReviseExpertCreationAction(request.Context(), owner, actionID, *input.Proposal, input.ExpectedVersion)
		if err != nil {
			writeResourceActionError(writer, err)
			return
		}
		writeResourceAction(writer, action)
		return
	}
	if input.Decision == "confirm" && (preview.Kind == resourceaction.ExpertKind || preview.Kind == resourceaction.TeamKind) {
		if current.State != "confirmed" {
			if preview.Expert != nil {
				if err := service.validateExpertInputAvailability(request.Context(), preview.Expert.Input()); err != nil {
					writeResourceActionError(writer, err)
					return
				}
			}
		}
		atomicRepo, ok := service.workspace.Repository().(interface {
			ConfirmExpertCreationAction(context.Context, string, string) (workspacedomain.ResourceCreationAction, error)
		})
		if !ok {
			writeAuthError(writer, http.StatusServiceUnavailable, "resource_action_unavailable")
			return
		}
		action, err := atomicRepo.ConfirmExpertCreationAction(request.Context(), owner, actionID)
		if err != nil {
			writeResourceActionError(writer, err)
			return
		}
		writeResourceAction(writer, action)
		return
	}

	if input.Decision == "cancel" {
		action, cancelErr := repository.CancelResourceCreationAction(request.Context(), owner, actionID)
		if cancelErr != nil {
			writeResourceActionError(writer, cancelErr)
			return
		}
		writeResourceAction(writer, action)
		return
	}
	if input.Decision != "confirm" {
		writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_resource_action_decision")
		return
	}
	action, proposal, err := repository.ClaimResourceCreationAction(request.Context(), owner, actionID)
	if err != nil {
		writeResourceActionError(writer, err)
		return
	}
	if action.State == "confirmed" {
		writeResourceAction(writer, action)
		return
	}
	resourceID, createErr := service.executeResourceCreationAction(request.Context(), owner, proposal)
	state := "confirmed"
	if createErr != nil {
		state = "failed"
	}
	completionErr := createErr
	if createErr != nil {
		// Persist only a stable, non-sensitive error code; provider and Git output
		// must never be exposed through the action API or session history.
		completionErr = errors.New(resourceActionErrorCode(createErr))
	}
	completed, completeErr := repository.CompleteResourceCreationAction(request.Context(), owner, actionID, state, resourceID, completionErr)
	if completeErr != nil {
		writeResourceActionError(writer, completeErr)
		return
	}
	if createErr != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(writer).Encode(resourceCreationActionJSON(completed))
		return
	}
	writeResourceAction(writer, completed)
}

func (service *Service) executeResourceCreationAction(ctx context.Context, owner string, proposal resourceaction.Proposal) (string, error) {
	switch proposal.Kind {
	case resourceaction.SkillKind:
		return service.executeSkillCreationAction(ctx, owner, *proposal.Skill)
	case resourceaction.ExpertKind:
		expertInput := proposal.Expert.Input()
		if err := service.validateExpertInputAvailability(ctx, expertInput); err != nil {
			return "", err
		}
		item, err := service.workspace.Repository().CreateExpert(ctx, owner, expertInput)
		return item.ID, err
	case resourceaction.TeamKind:
		if _, err := service.administrator(ctx); err != nil {
			return "", err
		}
		item, err := service.workspace.Repository().CreateExpertTeam(ctx, owner, proposal.Team.Input())
		return item.ID, err
	case resourceaction.ConnectorKind:
		input := proposal.Connector
		item, err := service.CreateConnectorPackage(ctx, &workspacev1.CreateConnectorPackageRequest{Source: input.Source, Version: input.Version, Type: "mcp", Name: input.Name, Description: input.Description, AuthMode: input.AuthMode, McpJson: input.MCPJSON, SkillName: input.SkillName, SkillMarkdown: input.SkillMarkdown})
		if err != nil {
			return "", err
		}
		return item.Id, nil
	default:
		return "", fmt.Errorf("%w: unsupported resource action", workspacedomain.ErrInvalid)
	}
}

func (service *Service) executeSkillCreationAction(ctx context.Context, owner string, proposal resourceaction.SkillProposal) (string, error) {
	var objectKey, digest string
	var gitURL, gitRef *string
	var name string
	var err error
	if proposal.Source == "git" {
		ref := strings.TrimSpace(proposal.GitRef)
		var resolved string
		var metadataName string
		var metadata skillstore.Metadata
		var installErr error
		objectKey, digest, resolved, metadata, installErr = service.skills.InstallGitWithMetadata(ctx, owner, proposal.GitURL, ref)
		err = installErr
		metadataName = metadata.DisplayName
		name = metadataName
		url := strings.TrimSpace(proposal.GitURL)
		gitURL, gitRef = &url, &resolved
	} else {
		archive, archiveErr := generatedSkillArchive(proposal)
		if archiveErr != nil {
			return "", archiveErr
		}
		var metadataName string
		var metadata skillstore.Metadata
		var installErr error
		objectKey, digest, metadata, installErr = service.skills.InstallUploadWithMetadata(ctx, owner, archive)
		err = installErr
		metadataName = metadata.DisplayName
		name = metadataName
	}
	if err != nil {
		return "", fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err)
	}
	if !strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(proposal.Name)) {
		_ = service.objects.Delete(context.WithoutCancel(ctx), objectKey)
		return "", fmt.Errorf("%w: generated Skill display name does not match the confirmed proposal", workspacedomain.ErrConflict)
	}
	item, err := service.workspace.Repository().CreateSkill(ctx, owner, workspacedomain.Skill{Name: name, Icon: "sparkles", Source: map[bool]string{true: "git", false: "upload"}[gitURL != nil], GitURL: gitURL, GitRef: gitRef, ObjectKey: objectKey, SHA256: digest})
	if err != nil {
		_ = service.objects.Delete(context.WithoutCancel(ctx), objectKey)
		return "", err
	}
	return item.ID, nil
}

func generatedSkillArchive(proposal resourceaction.SkillProposal) ([]byte, error) {
	document := strings.TrimSpace(proposal.Document)
	if !strings.HasPrefix(document, "---") {
		document = fmt.Sprintf("---\ndisplay_name: %s\n---\n\n%s\n", strconv.Quote(strings.TrimSpace(proposal.Name)), document)
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("SKILL.md")
	if err != nil {
		return nil, err
	}
	if _, err := entry.Write([]byte(document)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeResourceAction(writer http.ResponseWriter, action workspacedomain.ResourceCreationAction) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(resourceCreationActionJSON(action))
}

func resourceCreationActionJSON(action workspacedomain.ResourceCreationAction) map[string]any {
	result := map[string]any{"id": action.ID, "kind": action.Kind, "state": action.State, "name": action.Name, "description": action.Description, "resource_id": action.ResourceID, "expires_at": action.ExpiresAt, "version": action.Version}
	if action.Error != "" {
		result["error"] = action.Error
	}
	return result
}

func writeResourceActionError(writer http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "resource_action_failed"
	if errors.Is(err, workspacedomain.ErrNotFound) {
		status, code = http.StatusNotFound, "resource_action_not_found"
	} else if errors.Is(err, workspacedomain.ErrInvalid) {
		status, code = http.StatusUnprocessableEntity, "invalid_resource_preview"
	} else if errors.Is(err, workspacedomain.ErrConflict) {
		status, code = http.StatusConflict, "resource_action_conflict"
	}
	writeAuthError(writer, status, code)
}

func resourceActionErrorCode(err error) string {
	switch {
	case errors.Is(err, workspacedomain.ErrInvalid):
		return "invalid_input"
	case errors.Is(err, workspacedomain.ErrConflict):
		return "resource_conflict"
	default:
		return "resource_creation_failed"
	}
}
