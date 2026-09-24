package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/anythingllm"
	"github.com/google/uuid"
)

const knowledgePreviewLimit = 10

var knowledgeRevisionPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

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
	baseID, valid := parseKnowledgeUUID(request.PathValue("knowledge_base_id"))
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
	if strings.TrimSpace(service.config.AnythingLLM.Endpoint) == "" {
		writeAuthError(writer, http.StatusServiceUnavailable, "knowledge_search_unavailable")
		return
	}
	provider, err := anythingllm.NewClient(service.config.AnythingLLM.Endpoint, service.config.AnythingLLM.APIKey, service.config.AnythingLLM.Timeout.Value())
	if err != nil {
		writeAuthError(writer, http.StatusServiceUnavailable, "knowledge_search_unavailable")
		return
	}
	retrieved, err := provider.Query(request.Context(), baseID, generation, query, 30, 8000)
	if err != nil {
		writeAuthError(writer, http.StatusBadGateway, "knowledge_search_failed")
		return
	}
	result.Items, err = verifiedKnowledgeResults(request.Context(), retrieved.Citations, func(ctx context.Context, revisionID string) (workspacedomain.KnowledgeSearchSource, error) {
		return repository.ResolveKnowledgeSearchSource(ctx, owner, baseID, revisionID, administrator)
	})
	if err != nil {
		writeAuthError(writer, http.StatusInternalServerError, "knowledge_search_failed")
		return
	}
	writeKnowledgeSearchJSON(writer, result)
}

func verifiedKnowledgeResults(ctx context.Context, citations []anythingllm.Citation, resolve func(context.Context, string) (workspacedomain.KnowledgeSearchSource, error)) ([]knowledgeSearchResult, error) {
	results := make([]knowledgeSearchResult, 0, knowledgePreviewLimit)
	for _, citation := range citations {
		if len(results) >= knowledgePreviewLimit {
			break
		}
		revisionID := knowledgeRevisionPattern.FindString(citation.RevisionID)
		if revisionID == "" {
			revisionID = knowledgeRevisionPattern.FindString(citation.SourceLocation)
		}
		if _, err := uuid.Parse(revisionID); err != nil {
			continue
		}
		source, err := resolve(ctx, revisionID)
		if errors.Is(err, workspacedomain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		text := strings.TrimSpace(citation.Text)
		if text == "" {
			continue
		}
		if runes := []rune(text); len(runes) > 1200 {
			text = string(runes[:1200])
		}
		results = append(results, knowledgeSearchResult{DocumentID: source.DocumentID, RevisionID: source.RevisionID, DocumentName: source.DocumentName, CategoryName: source.CategoryName, Text: text, Relevance: citation.Relevance})
	}
	return results, nil
}

func writeKnowledgeSearchJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(value)
}
