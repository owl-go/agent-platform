package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/workspacefs"
	"github.com/google/uuid"
)

type conversationFileSource struct {
	view       *workspacev1.ConversationFile
	attachment domain.Attachment
}

func (service *Service) conversationFileSources(ctx context.Context, owner string, scope domain.ConversationScope) (map[string]conversationFileSource, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	result := map[string]conversationFileSource{}
	addAttachments := func(items []domain.Attachment) {
		for _, item := range items {
			result["attachment:"+item.ID] = conversationFileSource{view: &workspacev1.ConversationFile{Kind: "attachment", Id: item.ID, Name: item.Name, Size: item.Size, Available: true}, attachment: item}
		}
	}
	addArtifact := func(item domain.Artifact) {
		available := item.ExpiresAt == nil || item.ExpiresAt.After(time.Now())
		view := &workspacev1.ConversationFile{Kind: "artifact", Id: item.ID, Name: item.Name, Path: item.Path, Size: item.Size, Available: available}
		if !available {
			view.UnavailableReason = "expired"
		}
		result["artifact:"+item.ID] = conversationFileSource{view: view, attachment: domain.Attachment{ID: item.ID, Name: item.Name, ObjectKey: item.ObjectKey, Size: item.Size, SHA256: item.SHA256}}
	}
	if scope.SessionID != "" {
		var after int64
		for {
			messages, err := service.workspace.Repository().ListMessages(ctx, owner, scope.SessionID, after, 200)
			if err != nil {
				return nil, err
			}
			for _, message := range messages {
				addAttachments(message.Attachments)
				for _, item := range message.Artifacts {
					addArtifact(item)
				}
			}
			if len(messages) < 200 {
				break
			}
			last := messages[len(messages)-1].ID
			if last <= after {
				return nil, fmt.Errorf("invalid message pagination")
			}
			after = last
		}
	} else {
		turns, err := service.workspace.Repository().ListRunTurns(ctx, owner, scope.WorkflowID, scope.RunID)
		if err != nil {
			return nil, err
		}
		ids := map[string]bool{}
		for _, turn := range turns {
			ids[turn.ID] = true
			addAttachments(turn.Attachments)
		}
		artifacts, err := service.workspace.Repository().ListArtifacts(ctx, owner, scope.WorkflowID)
		if err != nil {
			return nil, err
		}
		for _, artifact := range artifacts {
			if ids[artifact.RunID] {
				addArtifact(artifact)
			}
		}
	}
	return result, nil
}

func (service *Service) ListConversationFiles(ctx context.Context, request *workspacev1.ListConversationFilesRequest) (*workspacev1.ListConversationFilesResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	scope := domain.ConversationScope{SessionID: request.SessionId, WorkflowID: request.WorkflowId, RunID: request.RunId}
	sources, err := service.conversationFileSources(ctx, owner, scope)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListConversationFilesResponse{}
	for _, source := range sources {
		response.Items = append(response.Items, source.view)
	}
	if scope.WorkflowID != "" {
		workflow, err := service.workspace.Repository().GetWorkflow(ctx, owner, scope.WorkflowID, false)
		if err != nil {
			return nil, publicError(err)
		}
		entries, _, err := service.files.List(ctx, workflow.WorkspacePath, request.WorkspacePath)
		if err != nil {
			return nil, publicError(fmt.Errorf("%w: Workspace files unavailable: %v", domain.ErrInvalid, err))
		}
		for _, entry := range entries {
			kind := "workspace"
			if entry.Directory {
				kind = "directory"
			}
			response.Items = append(response.Items, &workspacev1.ConversationFile{Kind: kind, Name: entry.Name, Path: entry.Path, Size: entry.Size, Available: true})
		}
	}
	return response, nil
}

func (service *Service) resolveMessageAttachments(ctx context.Context, owner string, scope domain.ConversationScope, ids []string, refs []*workspacev1.FileReference) ([]domain.Attachment, func(), error) {
	created := []string{}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		for _, key := range created {
			_ = service.objects.Delete(cleanupCtx, key)
		}
	}
	attachments, err := service.resolveAttachments(ctx, owner, ids)
	if err != nil {
		return nil, cleanup, err
	}
	if len(refs) == 0 {
		return attachments, cleanup, nil
	}
	sources, err := service.conversationFileSources(ctx, owner, scope)
	if err != nil {
		return nil, cleanup, err
	}
	seen := map[string]bool{}
	for _, item := range attachments {
		seen["attachment:"+item.ID] = true
	}
	for _, ref := range refs {
		if ref == nil {
			return nil, cleanup, domain.ErrInvalid
		}
		key := ref.Kind + ":" + ref.Id
		if ref.Kind == "workspace" {
			key = "workspace:" + ref.Path
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(attachments) >= maxTurnAttachments {
			return nil, cleanup, fmt.Errorf("%w: at most ten files per message", domain.ErrInvalid)
		}
		var content []byte
		var name, contentType string
		if ref.Kind == "workspace" && scope.WorkflowID != "" {
			workflow, err := service.workspace.Repository().GetWorkflow(ctx, owner, scope.WorkflowID, false)
			if err != nil {
				return nil, cleanup, err
			}
			file, info, err := service.files.Open(ctx, workflow.WorkspacePath, ref.Path)
			if err != nil {
				return nil, cleanup, fmt.Errorf("%w: Workspace file unavailable", domain.ErrInvalid)
			}
			if info.Size() > workspacefs.UploadLimit {
				file.Close()
				return nil, cleanup, fmt.Errorf("%w: file exceeds 100 MiB", domain.ErrInvalid)
			}
			content, err = io.ReadAll(io.LimitReader(file, workspacefs.UploadLimit+1))
			after, statErr := file.Stat()
			closeErr := file.Close()
			if err != nil || statErr != nil || closeErr != nil {
				return nil, cleanup, fmt.Errorf("%w: cannot read Workspace file", domain.ErrInvalid)
			}
			if info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) || int64(len(content)) != info.Size() {
				return nil, cleanup, fmt.Errorf("%w: Workspace file changed; select it again", domain.ErrConflict)
			}
			name = filepath.Base(ref.Path)
		} else {
			source, ok := sources[key]
			if !ok || !source.view.Available {
				return nil, cleanup, fmt.Errorf("%w: file is unavailable in this conversation", domain.ErrInvalid)
			}
			if source.attachment.Size > workspacefs.UploadLimit {
				return nil, cleanup, fmt.Errorf("%w: file exceeds 100 MiB", domain.ErrInvalid)
			}
			reader, object, err := service.objects.Get(ctx, source.attachment.ObjectKey)
			if err != nil {
				return nil, cleanup, fmt.Errorf("%w: referenced file unavailable", domain.ErrInvalid)
			}
			content, err = io.ReadAll(io.LimitReader(reader, workspacefs.UploadLimit+1))
			closeErr := reader.Close()
			if err != nil || closeErr != nil {
				return nil, cleanup, fmt.Errorf("%w: cannot read referenced file", domain.ErrInvalid)
			}
			hash := sha256.Sum256(content)
			if int64(len(content)) != source.attachment.Size || hex.EncodeToString(hash[:]) != source.attachment.SHA256 {
				return nil, cleanup, fmt.Errorf("%w: referenced file checksum mismatch", domain.ErrInvalid)
			}
			name, contentType = source.attachment.Name, object.ContentType
		}
		if int64(len(content)) > workspacefs.UploadLimit {
			return nil, cleanup, fmt.Errorf("%w: file exceeds 100 MiB", domain.ErrInvalid)
		}
		if contentType == "" {
			contentType = http.DetectContentType(content)
		}
		id := uuid.NewString()
		objectKey := attachmentObjectKey(owner, id)
		hash := sha256.Sum256(content)
		object, err := service.objects.Put(ctx, objectKey, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: hex.EncodeToString(hash[:]), ContentType: contentType, Metadata: map[string]string{"name": name}})
		if err != nil {
			return nil, cleanup, err
		}
		created = append(created, objectKey)
		attachments = append(attachments, attachmentFromObject(id, object))
	}
	return attachments, cleanup, nil
}
