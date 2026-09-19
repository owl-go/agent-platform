package workspace

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

func (service *Service) restoreKnowledgeBase(writer http.ResponseWriter, request *http.Request) {
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
	baseID, ok := knowledgeBaseActionPath(request.URL.Path, "restore")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if err := service.workspace.Repository().RestoreKnowledgeBase(request.Context(), owner, baseID, administrator); err != nil {
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	writeKnowledgeLifecycleAccepted(writer)
}

func (service *Service) restoreKnowledgeCategory(writer http.ResponseWriter, request *http.Request) {
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
	baseID, categoryID, ok := knowledgeCategoryActionPath(request.URL.Path, "restore")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if err := service.workspace.Repository().RestoreKnowledgeCategory(request.Context(), owner, baseID, categoryID, administrator); err != nil {
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	writeKnowledgeLifecycleAccepted(writer)
}

func (service *Service) mutateKnowledgeDocument(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodDelete {
		writer.Header().Set("Allow", http.MethodDelete)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, administrator, err := service.knowledgePrincipal(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	baseID, documentID, ok := knowledgeDocumentActionPath(request.URL.Path, "")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if err := service.workspace.Repository().DeleteKnowledgeDocument(request.Context(), owner, baseID, documentID, administrator); err != nil {
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	writeKnowledgeLifecycleAccepted(writer)
}

func (service *Service) restoreKnowledgeDocument(writer http.ResponseWriter, request *http.Request) {
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
	baseID, documentID, ok := knowledgeDocumentActionPath(request.URL.Path, "restore")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if err := service.workspace.Repository().RestoreKnowledgeDocument(request.Context(), owner, baseID, documentID, administrator); err != nil {
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	writeKnowledgeLifecycleAccepted(writer)
}

func writeKnowledgeLifecycleAccepted(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(map[string]any{"accepted": true})
}

func knowledgeBaseActionPath(value, action string) (string, bool) {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "knowledge-bases" || parts[4] != action {
		return "", false
	}
	return parseKnowledgeUUID(parts[3])
}

func knowledgeCategoryActionPath(value, action string) (string, string, bool) {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) != 7 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "knowledge-bases" || parts[4] != "categories" || parts[6] != action {
		return "", "", false
	}
	baseID, ok := parseKnowledgeUUID(parts[3])
	if !ok {
		return "", "", false
	}
	categoryID, ok := parseKnowledgeUUID(parts[5])
	return baseID, categoryID, ok
}

func parseKnowledgeUUID(value string) (string, bool) {
	if _, err := uuid.Parse(value); err != nil {
		return "", false
	}
	return value, true
}
