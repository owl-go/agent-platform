package workspace

import (
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

func (service *Service) downloadSessionArtifact(writer http.ResponseWriter, request *http.Request) {
	service.downloadArtifact(writer, request, true)
}

func (service *Service) downloadWorkflowArtifact(writer http.ResponseWriter, request *http.Request) {
	service.downloadArtifact(writer, request, false)
}

func (service *Service) downloadArtifact(writer http.ResponseWriter, request *http.Request, session bool) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 7 || parts[0] != "api" || parts[1] != "v1" || parts[4] != "artifacts" || parts[6] != "download" {
		http.NotFound(writer, request)
		return
	}
	parentID, artifactID := parts[3], parts[5]
	var item workspacedomain.Artifact
	if session {
		if parts[2] != "sessions" {
			http.NotFound(writer, request)
			return
		}
		item, err = service.workspace.Repository().GetSessionArtifact(request.Context(), owner, parentID, artifactID)
	} else {
		if parts[2] != "workflows" {
			http.NotFound(writer, request)
			return
		}
		item, err = service.workspace.Repository().GetArtifact(request.Context(), owner, parentID, artifactID)
	}
	if err != nil || item.Kind != "file" || item.ObjectKey == "" || item.ExpiresAt == nil || item.ExpiresAt.Before(time.Now()) {
		http.NotFound(writer, request)
		return
	}
	reader, object, err := service.objects.Get(request.Context(), item.ObjectKey)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer reader.Close()
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	writer.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.Name}))
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(writer, io.LimitReader(reader, object.Size))
}
