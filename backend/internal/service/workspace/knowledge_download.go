package workspace

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
)

func (service *Service) retryKnowledgeDocument(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, administrator, err := service.knowledgePrincipal(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	baseID, documentID, ok := knowledgeDocumentActionPath(request.URL.Path, "retry")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if err := service.workspace.Repository().RetryKnowledgeDocument(request.Context(), owner, baseID, documentID, administrator); err != nil {
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(map[string]any{"accepted": true})
}

func (service *Service) downloadKnowledgeDocument(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, administrator, err := service.knowledgePrincipal(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	baseID, documentID, ok := knowledgeDocumentDownloadPath(request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	documents, err := service.workspace.Repository().ListKnowledgeDocuments(request.Context(), owner, baseID, administrator)
	if err != nil {
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	var document *workspacedomain.KnowledgeDocument
	for index := range documents {
		if documents[index].ID == documentID {
			document = &documents[index]
			break
		}
	}
	if document == nil || document.LatestRevision == nil || document.LatestRevision.ObjectKey == "" {
		http.NotFound(writer, request)
		return
	}
	reader, object, err := service.objects.Get(request.Context(), document.LatestRevision.ObjectKey)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer reader.Close()
	writer.Header().Set("Content-Type", object.ContentType)
	writer.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	writer.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(document.Name, `"`, "")+`"`)
	writer.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(writer, reader)
}

func knowledgeDocumentDownloadPath(value string) (string, string, bool) {
	return knowledgeDocumentActionPath(value, "download")
}

func knowledgeDocumentActionPath(value, action string) (string, string, bool) {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if action == "" {
		if len(parts) != 6 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "knowledge-bases" || parts[4] != "documents" {
			return "", "", false
		}
		baseID, baseOK := parseKnowledgeUUID(parts[3])
		documentID, documentOK := parseKnowledgeUUID(parts[5])
		return baseID, documentID, baseOK && documentOK
	}
	if len(parts) != 7 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "knowledge-bases" || parts[4] != "documents" || parts[6] != action {
		return "", "", false
	}
	if _, err := uuid.Parse(parts[3]); err != nil {
		return "", "", false
	}
	if _, err := uuid.Parse(parts[5]); err != nil {
		return "", "", false
	}
	return parts[3], parts[5], true
}
