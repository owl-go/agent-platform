package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/workspacefs"

	"google.golang.org/protobuf/types/known/timestamppb"
)

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

func (service *Service) ConfigureWorkflowGitSource(ctx context.Context, request *workspacev1.ConfigureWorkflowGitSourceRequest) (*workspacev1.Workflow, error) {
	owner, workflow, err := service.workspaceForRequest(ctx, request.WorkflowId)
	if err != nil {
		return nil, err
	}
	config := make([]workspacedomain.GitConfigEntry, 0, len(request.Config))
	for _, entry := range request.Config {
		config = append(config, workspacedomain.GitConfigEntry{Key: entry.Key, Value: entry.Value})
	}
	username := optionalString(request.GetUsername())
	source := workspacedomain.GitSource{URL: request.Url, Branch: request.Branch, Authentication: request.Authentication, Username: username, Config: config, SSHConfig: request.SshConfig, CredentialConfigured: request.Authentication != "none"}
	if err := workspacedomain.ValidateGitSource(source); err != nil {
		return nil, publicError(err)
	}
	password := []byte(request.GetPassword())
	privateKey := []byte(request.GetSshPrivateKey())
	defer clear(password)
	defer clear(privateKey)
	if source.Authentication == "basic" && len(password) == 0 {
		return nil, publicError(fmt.Errorf("%w: Git password is required", workspacedomain.ErrInvalid))
	}
	if source.Authentication == "ssh" && len(privateKey) == 0 {
		return nil, publicError(fmt.Errorf("%w: private SSH key is required", workspacedomain.ErrInvalid))
	}
	secretPayload, err := json.Marshal(map[string]string{"password": string(password), "ssh_private_key": string(privateKey)})
	if err != nil {
		return nil, publicError(err)
	}
	defer clear(secretPayload)
	var encryptedSecret []byte
	if source.CredentialConfigured {
		encryptedSecret, err = service.box.Encrypt(secretPayload, "workflow-git:"+owner)
		if err != nil {
			return nil, publicError(err)
		}
	}
	err = service.files.Clone(ctx, workflow.WorkspacePath, workspacefs.GitCloneOptions{RepositoryURL: request.Url, Branch: request.Branch, Username: request.GetUsername(), Password: password, PrivateKey: privateKey, Config: config, SSHConfig: request.SshConfig})
	if err != nil {
		return nil, publicError(fmt.Errorf("%w: %v", workspacedomain.ErrInvalid, err))
	}
	updated, err := service.workspace.Repository().SetWorkflowGitSource(ctx, owner, request.WorkflowId, source, encryptedSecret)
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
