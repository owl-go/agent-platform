package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/workspacefs"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) uploadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 6 {
		http.NotFound(writer, request)
		return
	}
	_, workflow, err := service.workspaceForRequest(request.Context(), parts[3])
	if err != nil {
		writeAuthError(writer, http.StatusNotFound, "resource_not_found")
		return
	}
	if request.ContentLength > workspacefs.UploadLimit {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "upload_too_large")
		return
	}
	content, err := io.ReadAll(io.LimitReader(request.Body, workspacefs.UploadLimit+1))
	if err != nil || int64(len(content)) > workspacefs.UploadLimit {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "upload_too_large")
		return
	}
	entry, err := service.files.Upload(request.Context(), workflow.WorkspacePath, request.URL.Query().Get("path"), content)
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_workspace_upload")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(map[string]any{"path": entry.Path, "name": entry.Name, "directory": entry.Directory, "size": entry.Size, "modified_at": entry.ModifiedAt})
}

func (service *Service) downloadWorkspaceFile(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 6 {
		http.NotFound(writer, request)
		return
	}
	_, workflow, err := service.workspaceForRequest(request.Context(), parts[3])
	if err != nil {
		writeAuthError(writer, http.StatusNotFound, "resource_not_found")
		return
	}
	relative := request.URL.Query().Get("path")
	file, info, err := service.files.Open(request.Context(), workflow.WorkspacePath, relative)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer file.Close()
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(relative)))
	writer.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(writer, request, filepath.Base(relative), info.ModTime(), file)
}

func (service *Service) ListWorkspaceEntries(ctx context.Context, request *workspacev1.ListWorkspaceEntriesRequest) (*workspacev1.ListWorkspaceEntriesResponse, error) {
	owner, workflow, err := service.workspaceForRequest(ctx, request.WorkflowId)
	if err != nil {
		return nil, err
	}
	_ = owner
	relative := ""
	if request.Path != nil {
		relative = *request.Path
	}
	items, used, err := service.files.List(ctx, workflow.WorkspacePath, relative)
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	response := make([]*workspacev1.WorkspaceEntry, 0, len(items))
	for _, item := range items {
		response = append(response, workspaceEntryResponse(item))
	}
	return &workspacev1.ListWorkspaceEntriesResponse{Items: response, UsedBytes: used, LimitBytes: workspacefs.WorkspaceLimit}, nil
}

func (service *Service) CreateWorkspaceDirectory(ctx context.Context, request *workspacev1.CreateWorkspaceDirectoryRequest) (*workspacev1.WorkspaceEntry, error) {
	_, workflow, err := service.workspaceForRequest(ctx, request.WorkflowId)
	if err != nil {
		return nil, err
	}
	item, err := service.files.CreateDirectory(ctx, workflow.WorkspacePath, request.Path)
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	return workspaceEntryResponse(item), nil
}

func (service *Service) UploadWorkspaceFile(ctx context.Context, request *workspacev1.UploadWorkspaceFileRequest) (*workspacev1.WorkspaceEntry, error) {
	_, workflow, err := service.workspaceForRequest(ctx, request.WorkflowId)
	if err != nil {
		return nil, err
	}
	item, err := service.files.Upload(ctx, workflow.WorkspacePath, request.Path, request.Content)
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	return workspaceEntryResponse(item), nil
}

func (service *Service) GetWorkspaceFile(ctx context.Context, request *workspacev1.GetWorkspaceFileRequest) (*workspacev1.WorkspaceFile, error) {
	_, workflow, err := service.workspaceForRequest(ctx, request.WorkflowId)
	if err != nil {
		return nil, err
	}
	content, modified, err := service.files.Read(ctx, workflow.WorkspacePath, request.Path)
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	contentType := http.DetectContentType(content)
	return &workspacev1.WorkspaceFile{Path: request.Path, Content: content, ContentType: contentType, Size: int64(len(content)), ModifiedAt: timestamppb.New(modified)}, nil
}

func (service *Service) ClearWorkspace(ctx context.Context, request *workspacev1.ClearWorkspaceRequest) (*workspacev1.DeleteResponse, error) {
	_, workflow, err := service.workspaceForRequest(ctx, request.WorkflowId)
	if err != nil {
		return nil, err
	}
	if request.Confirmation != workflow.Name {
		return nil, publicError(fmt.Errorf("%w: enter the Workflow name to clear its Workspace", workspacedomain.ErrInvalid))
	}
	if err := service.files.Clear(ctx, workflow.WorkspacePath); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) CloneWorkspace(ctx context.Context, request *workspacev1.CloneWorkspaceRequest) (*workspacev1.Workflow, error) {
	owner, workflow, err := service.workspaceForRequest(ctx, request.WorkflowId)
	if err != nil {
		return nil, err
	}
	private := request.SshPrivateKey != nil && strings.TrimSpace(*request.SshPrivateKey) != ""
	source := workspacedomain.GitSource{URL: request.Url, Branch: request.Branch, PrivateSSH: private, CredentialConfigured: private}
	if err := workspacedomain.ValidateGitSource(source); err != nil {
		return nil, publicError(err)
	}
	var encryptedKey []byte
	if private {
		key := []byte(*request.SshPrivateKey)
		defer clear(key)
		encryptedKey, err = service.box.Encrypt(key, "workflow-git:"+owner)
		if err == nil {
			err = service.files.CloneSSH(ctx, workflow.WorkspacePath, request.Url, request.Branch, key)
		}
	} else {
		err = service.files.Clone(ctx, workflow.WorkspacePath, request.Url, request.Branch)
	}
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	updated, err := service.workspace.Repository().SetWorkflowGitSource(ctx, owner, request.WorkflowId, source, encryptedKey)
	if err != nil {
		_ = service.files.Clear(context.WithoutCancel(ctx), workflow.WorkspacePath)
		return nil, publicError(err)
	}
	return workflowResponse(updated), nil
}

func (service *Service) workspaceForRequest(ctx context.Context, workflowID string) (string, workspacedomain.Workflow, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return "", workspacedomain.Workflow{}, err
	}
	workflow, err := service.workspace.Repository().GetWorkflow(ctx, owner, workflowID, false)
	if err != nil {
		return "", workspacedomain.Workflow{}, publicError(err)
	}
	return owner, workflow, nil
}

func workspaceEntryResponse(item workspacedomain.WorkspaceEntry) *workspacev1.WorkspaceEntry {
	return &workspacev1.WorkspaceEntry{Path: item.Path, Name: item.Name, Directory: item.Directory, Size: item.Size, ModifiedAt: timestamppb.New(item.ModifiedAt)}
}
