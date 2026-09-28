package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/productanalytics"
	"agent-platform/backend/internal/workspacefs"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type sessionWorkflowSource struct {
	view       *workspacev1.SessionWorkflowFile
	attachment domain.Attachment
}

func (service *Service) PreviewSessionWorkflowDraft(ctx context.Context, request *workspacev1.PreviewSessionWorkflowDraftRequest) (*workspacev1.SessionWorkflowDraft, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	draft, _, err := service.sessionWorkflowDraft(ctx, owner, request.SessionId, request.MessageId)
	if err != nil {
		return nil, publicError(err)
	}
	service.productAnalytics().WorkflowSaveStarted(ctx, owner, request.SessionId, 2+len(draft.Resources))
	return draft, nil
}

func (service *Service) CreateWorkflowFromSession(ctx context.Context, request *workspacev1.CreateWorkflowFromSessionRequest) (*workspacev1.CreateWorkflowFromSessionResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, ok := service.workspace.Repository().(application.SessionWorkflowRepository)
	if !ok {
		return nil, publicError(fmt.Errorf("%w: Session Workflow conversion is unavailable", domain.ErrInvalid))
	}
	links, err := repository.ListSessionWorkflowLinks(ctx, owner, request.SessionId)
	if err != nil {
		return nil, publicError(err)
	}
	for _, link := range links {
		if link.MessageID == request.MessageId {
			created, err := repository.CreateWorkflowFromSession(ctx, owner, link.WorkflowID, request.SessionId, request.MessageId, request.Name, request.Goal)
			if err != nil {
				return nil, publicError(err)
			}
			return sessionWorkflowCreationResponse(created), nil
		}
	}

	_, sources, err := service.sessionWorkflowDraft(ctx, owner, request.SessionId, request.MessageId)
	if err != nil {
		return nil, publicError(err)
	}
	decisions, err := validateSessionWorkflowFileDecisions(sources, request.Files)
	if err != nil {
		return nil, publicError(err)
	}
	workflowID := uuid.NewString()
	workspacePath := "workflows/" + owner + "/" + workflowID
	staged := false
	for key, destination := range decisions {
		if destination != "workspace" {
			continue
		}
		source := sources[key]
		content, err := service.readSessionWorkflowSource(ctx, source)
		if err != nil {
			_ = service.files.Clear(context.WithoutCancel(ctx), workspacePath)
			return nil, publicError(err)
		}
		path := "session-imports/" + sessionWorkflowFileName(key, source.attachment.Name)
		if _, err := service.files.Upload(ctx, workspacePath, path, content); err != nil {
			_ = service.files.Clear(context.WithoutCancel(ctx), workspacePath)
			return nil, publicError(fmt.Errorf("copy Session file to Workflow Workspace: %w", err))
		}
		staged = true
	}
	created, err := repository.CreateWorkflowFromSession(ctx, owner, workflowID, request.SessionId, request.MessageId, request.Name, request.Goal)
	if err != nil {
		if staged {
			_ = service.files.Clear(context.WithoutCancel(ctx), workspacePath)
		}
		return nil, publicError(err)
	}
	if created.Replayed && staged {
		_ = service.files.Clear(context.WithoutCancel(ctx), workspacePath)
	}
	hasConnector := false
	message, messageErr := service.workspace.Repository().GetMessage(ctx, owner, request.SessionId, request.MessageId)
	if messageErr == nil && message.ResponseSnapshot != nil {
		for _, stage := range message.ResponseSnapshot.Stages {
			hasConnector = hasConnector || len(stage.MCPServers) > 0 || len(stage.CLIConnectors) > 0
		}
	}
	if !created.Replayed {
		service.productAnalytics().WorkflowCreated(ctx, productanalytics.WorkflowCreatedObservation{OwnerID: owner, WorkflowID: created.Workflow.ID, Source: "session", HasConnector: hasConnector})
	}
	return sessionWorkflowCreationResponse(created), nil
}

func (service *Service) ListSessionWorkflowLinks(ctx context.Context, request *workspacev1.ListSessionWorkflowLinksRequest) (*workspacev1.ListSessionWorkflowLinksResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, ok := service.workspace.Repository().(application.SessionWorkflowRepository)
	if !ok {
		return nil, publicError(fmt.Errorf("%w: Session Workflow conversion is unavailable", domain.ErrInvalid))
	}
	items, err := repository.ListSessionWorkflowLinks(ctx, owner, request.SessionId)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListSessionWorkflowLinksResponse{}
	for _, item := range items {
		response.Items = append(response.Items, sessionWorkflowLinkResponse(item))
	}
	return response, nil
}

func (service *Service) sessionWorkflowDraft(ctx context.Context, owner, sessionID string, messageID int64) (*workspacev1.SessionWorkflowDraft, map[string]sessionWorkflowSource, error) {
	session, err := service.workspace.Repository().GetSession(ctx, owner, sessionID)
	if err != nil {
		return nil, nil, err
	}
	assistant, err := service.workspace.Repository().GetMessage(ctx, owner, sessionID, messageID)
	if err != nil {
		return nil, nil, err
	}
	if assistant.Role != "assistant" || assistant.State != "completed" || assistant.ResponseSnapshot == nil || len(assistant.ResponseSnapshot.Stages) == 0 {
		return nil, nil, fmt.Errorf("%w: select a successful assistant response", domain.ErrInvalid)
	}
	user, err := service.userMessageBefore(ctx, owner, sessionID, messageID)
	if err != nil {
		return nil, nil, err
	}
	draft := &workspacev1.SessionWorkflowDraft{SuggestedName: session.Title, SuggestedGoal: user.Content}
	draft.SpecialistName, draft.Resources = sessionWorkflowResources(assistant.ResponseSnapshot.Stages)
	sources := make(map[string]sessionWorkflowSource, len(user.Attachments)+len(assistant.Artifacts))
	for _, item := range user.Attachments {
		key := "attachment:" + item.ID
		source := sessionWorkflowSource{view: &workspacev1.SessionWorkflowFile{SourceKey: key, Kind: "attachment", Name: item.Name, Size: item.Size, Available: item.ObjectKey != "" && item.Size >= 0 && item.SHA256 != ""}, attachment: item}
		if !source.view.Available {
			source.view.UnavailableReason = stringPointer("source_unavailable")
		}
		sources[key] = source
	}
	for _, item := range assistant.Artifacts {
		key := "artifact:" + item.ID
		available := item.ObjectKey != "" && item.SHA256 != "" && (item.ExpiresAt == nil || item.ExpiresAt.After(time.Now()))
		source := sessionWorkflowSource{view: &workspacev1.SessionWorkflowFile{SourceKey: key, Kind: "artifact", Name: item.Name, Size: item.Size, Available: available}, attachment: domain.Attachment{ID: item.ID, Name: item.Name, ObjectKey: item.ObjectKey, Size: item.Size, SHA256: item.SHA256}}
		if !available {
			reason := "source_unavailable"
			if item.ExpiresAt != nil && !item.ExpiresAt.After(time.Now()) {
				reason = "expired"
			}
			source.view.UnavailableReason = &reason
		}
		sources[key] = source
	}
	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		draft.Files = append(draft.Files, sources[key].view)
	}
	if repository, ok := service.workspace.Repository().(application.SessionWorkflowRepository); ok {
		links, linkErr := repository.ListSessionWorkflowLinks(ctx, owner, sessionID)
		if linkErr != nil {
			return nil, nil, linkErr
		}
		for _, link := range links {
			if link.MessageID == messageID {
				draft.ExistingLink = sessionWorkflowLinkResponse(link)
				break
			}
		}
	}
	return draft, sources, nil
}

func (service *Service) userMessageBefore(ctx context.Context, owner, sessionID string, messageID int64) (domain.Message, error) {
	var result domain.Message
	var after int64
	for {
		items, err := service.workspace.Repository().ListMessages(ctx, owner, sessionID, after, 200)
		if err != nil {
			return result, err
		}
		for _, item := range items {
			if item.ID >= messageID {
				if result.ID == 0 {
					return result, fmt.Errorf("%w: successful response has no source user message", domain.ErrInvalid)
				}
				return result, nil
			}
			if item.Role == "user" {
				result = item
			}
		}
		if len(items) < 200 {
			break
		}
		last := items[len(items)-1].ID
		if last <= after {
			break
		}
		after = last
	}
	if result.ID == 0 {
		return result, fmt.Errorf("%w: successful response has no source user message", domain.ErrInvalid)
	}
	return result, nil
}

func sessionWorkflowResources(stages []domain.ExecutionStageSnapshot) (string, []*workspacev1.SessionWorkflowResource) {
	var specialists []string
	result := []*workspacev1.SessionWorkflowResource{}
	seen := map[string]bool{}
	add := func(kind, id, name string) {
		key := kind + ":" + id
		if id == "" || seen[key] {
			return
		}
		seen[key] = true
		result = append(result, &workspacev1.SessionWorkflowResource{Kind: kind, Id: id, Name: name})
	}
	for _, stage := range stages {
		if stage.Expert != nil {
			specialists = append(specialists, stage.Expert.Name)
			add("expert", stage.Expert.ID, stage.Expert.Name)
		}
		for _, item := range stage.Skills {
			add("skill", item.ID, item.Name)
		}
		for _, item := range stage.MCPServers {
			add("mcp", item.ID, item.Name)
		}
		for _, item := range stage.CLIConnectors {
			add("cli", item.ID, item.Name)
		}
	}
	if len(specialists) == 0 {
		return "默认执行配置", result
	}
	return strings.Join(specialists, " / "), result
}

func validateSessionWorkflowFileDecisions(sources map[string]sessionWorkflowSource, decisions []*workspacev1.SessionWorkflowFileDecision) (map[string]string, error) {
	result := make(map[string]string, len(decisions))
	for _, decision := range decisions {
		if decision == nil {
			return nil, fmt.Errorf("%w: file decision is required", domain.ErrInvalid)
		}
		source, exists := sources[decision.SourceKey]
		if !exists {
			return nil, fmt.Errorf("%w: file source is not part of this response", domain.ErrInvalid)
		}
		if _, duplicate := result[decision.SourceKey]; duplicate {
			return nil, fmt.Errorf("%w: file source decision is duplicated", domain.ErrInvalid)
		}
		if decision.Destination != "workspace" && decision.Destination != "exclude" {
			return nil, fmt.Errorf("%w: file destination must be workspace or exclude", domain.ErrInvalid)
		}
		if decision.Destination == "workspace" && !source.view.Available {
			return nil, fmt.Errorf("%w: unavailable file cannot be copied", domain.ErrInvalid)
		}
		result[decision.SourceKey] = decision.Destination
	}
	if len(result) != len(sources) {
		return nil, fmt.Errorf("%w: choose a destination for every source file", domain.ErrInvalid)
	}
	return result, nil
}

func (service *Service) readSessionWorkflowSource(ctx context.Context, source sessionWorkflowSource) ([]byte, error) {
	if !source.view.Available || source.attachment.Size > workspacefs.UploadLimit {
		return nil, fmt.Errorf("%w: referenced file is unavailable or exceeds 100 MiB", domain.ErrInvalid)
	}
	reader, _, err := service.objects.Get(ctx, source.attachment.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("%w: referenced file is unavailable", domain.ErrInvalid)
	}
	content, readErr := io.ReadAll(io.LimitReader(reader, workspacefs.UploadLimit+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || int64(len(content)) > workspacefs.UploadLimit {
		return nil, fmt.Errorf("%w: cannot read referenced file", domain.ErrInvalid)
	}
	hash := sha256.Sum256(content)
	if int64(len(content)) != source.attachment.Size || hex.EncodeToString(hash[:]) != source.attachment.SHA256 {
		return nil, fmt.Errorf("%w: referenced file checksum mismatch", domain.ErrInvalid)
	}
	return content, nil
}

func sessionWorkflowFileName(sourceKey, name string) string {
	prefix := strings.NewReplacer(":", "-", "/", "-", "\\", "-").Replace(sourceKey)
	base := filepath.Base(name)
	base = strings.Map(func(value rune) rune {
		if unicode.IsLetter(value) || unicode.IsDigit(value) || strings.ContainsRune("._- ", value) {
			return value
		}
		return '-'
	}, base)
	base = strings.Trim(strings.TrimSpace(base), ".")
	if base == "" {
		base = "file"
	}
	return prefix + "-" + base
}

func sessionWorkflowCreationResponse(item domain.SessionWorkflowCreation) *workspacev1.CreateWorkflowFromSessionResponse {
	return &workspacev1.CreateWorkflowFromSessionResponse{Workflow: workflowResponse(item.Workflow), ValidationRun: runResponse(item.Run), Link: sessionWorkflowLinkResponse(item.Link), Replayed: item.Replayed}
}

func sessionWorkflowLinkResponse(item domain.SessionWorkflowLink) *workspacev1.SessionWorkflowLink {
	return &workspacev1.SessionWorkflowLink{SessionId: item.SessionID, MessageId: item.MessageID, WorkflowId: item.WorkflowID, WorkflowName: item.WorkflowName, ValidationRunId: item.ValidationRunID, CreatedAt: timestamppb.New(item.CreatedAt)}
}

func stringPointer(value string) *string { return &value }
