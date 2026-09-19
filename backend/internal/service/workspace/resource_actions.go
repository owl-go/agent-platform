package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096)).Decode(&input); err != nil {
		writeAuthError(writer, http.StatusBadRequest, "invalid_request_body")
		return
	}
	repository, err := service.resourceCreationActions()
	if err != nil {
		writeAuthError(writer, http.StatusServiceUnavailable, "resource_action_unavailable")
		return
	}
	actionID := parts[3]
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
		input := proposal.Expert
		expertInput := workspacedomain.ExpertInput{Name: input.Name, Icon: input.Icon, IconBackground: input.IconBackground, Introduction: input.Introduction, CoreCapability: input.CoreCapability, OperatingProcedure: input.OperatingProcedure, OutputStandard: input.OutputStandard, Cautions: input.Cautions, SkillIDs: append([]string(nil), input.SkillIDs...), MCPServerIDs: append([]string(nil), input.MCPServerIDs...), CLIConnectorDefinitionIDs: append([]string(nil), input.CLIConnectorDefinitionIDs...)}
		if err := service.validateExpertInputAvailability(ctx, expertInput); err != nil {
			return "", err
		}
		item, err := service.workspace.Repository().CreateExpert(ctx, owner, expertInput)
		return item.ID, err
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
