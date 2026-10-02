package workspace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

const knowledgePreviewLimit = 10

type knowledgeSearchResult struct {
	DocumentID   string  `json:"document_id"`
	RevisionID   string  `json:"revision_id"`
	DocumentName string  `json:"document_name"`
	CategoryName string  `json:"category_name,omitempty"`
	Text         string  `json:"text"`
	Relevance    float32 `json:"relevance"`
}

func (service *Service) searchKnowledgeBase(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeAuthError(writer, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	owner, administrator, err := service.knowledgePrincipal(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	baseID, valid := knowledgeBaseActionPath(request.URL.Path, "search")
	if !valid {
		writeAuthError(writer, http.StatusNotFound, "resource_not_found")
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("q"))
	if query == "" || len([]rune(query)) > 500 {
		writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_knowledge_query")
		return
	}
	repository := service.workspace.Repository()
	generation, err := repository.ReadyKnowledgeSearchGeneration(request.Context(), owner, baseID, administrator)
	if err != nil {
		if errors.Is(err, workspacedomain.ErrNotFound) {
			writeAuthError(writer, http.StatusNotFound, "resource_not_found")
		} else {
			writeAuthError(writer, http.StatusInternalServerError, "knowledge_search_failed")
		}
		return
	}
	result := struct {
		IndexReady bool                    `json:"index_ready"`
		Items      []knowledgeSearchResult `json:"items"`
	}{IndexReady: generation > 0, Items: []knowledgeSearchResult{}}
	if generation == 0 {
		writeKnowledgeSearchJSON(writer, result)
		return
	}
	if service.knowledgeSearch == nil {
		writeAuthError(writer, http.StatusServiceUnavailable, "knowledge_search_unavailable")
		return
	}
	hits, err := service.knowledgeSearch.Search(request.Context(), owner, baseID, 0, query, knowledgePreviewLimit, 8000)
	if err != nil {
		writeAuthError(writer, http.StatusBadGateway, "knowledge_search_failed")
		return
	}
	for _, hit := range hits {
		result.Items = append(result.Items, knowledgeSearchResult{DocumentID: hit.Source.DocumentID, RevisionID: hit.Source.RevisionID, DocumentName: hit.Source.DocumentName, CategoryName: hit.Source.CategoryName, Text: truncateKnowledgePreview(hit.Text), Relevance: hit.Relevance})
	}
	writeKnowledgeSearchJSON(writer, result)
}

func truncateKnowledgePreview(text string) string {
	if runes := []rune(text); len(runes) > 1200 {
		return string(runes[:1200])
	}
	return text
}

func writeKnowledgeSearchJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(value)
}
