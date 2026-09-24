// Package anythingllm defines the platform-owned boundary to the AnythingLLM
// deployment. Runtime containers never receive this adapter or its credentials.
package anythingllm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("AnythingLLM provider unavailable")

type Citation struct {
	KnowledgeBaseID string
	CategoryID      *string
	DocumentID      string
	RevisionID      string
	SourceLocation  string
	Relevance       float32
	Text            string
	SHA256          string
}

type Retrieval struct {
	Citations []Citation
	NoHit     bool
}

// Adapter is intentionally narrow: provider IDs and credentials stay inside
// the trusted API/Worker process and are never part of a Workflow snapshot.
type Adapter interface {
	EnsureWorkspace(context.Context, string) error
	DeleteWorkspace(context.Context, string) error
	UpsertRevision(context.Context, string, string, string, []byte) error
	RemoveRevision(context.Context, string, string) error
	Query(context.Context, string, int64, string, int, int) (Retrieval, error)
}

type Client struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

type httpStatusError struct{ status int }

func (err httpStatusError) Error() string {
	return fmt.Sprintf("AnythingLLM returned HTTP %d", err.status)
}

func NewClient(endpoint, apiKey string, timeout time.Duration) (*Client, error) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" || strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("AnythingLLM endpoint and API key are required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{endpoint: endpoint, apiKey: apiKey, client: &http.Client{Timeout: timeout}}, nil
}

func workspaceSlug(id string) string {
	return "knowledge-" + strings.NewReplacer("-", "", "_", "").Replace(id)
}

func (client *Client) request(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, client.endpoint+endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("AnythingLLM request: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, httpStatusError{status: response.StatusCode}
	}
	return response, nil
}

func (client *Client) EnsureWorkspace(ctx context.Context, knowledgeBaseID string) error {
	slug := workspaceSlug(knowledgeBaseID)
	response, err := client.request(ctx, http.MethodGet, "/v1/workspace/"+url.PathEscape(slug), nil, "")
	if err == nil {
		response.Body.Close()
		return nil
	}
	var statusErr httpStatusError
	if !errors.As(err, &statusErr) || statusErr.status != http.StatusNotFound {
		return fmt.Errorf("read AnythingLLM workspace: %w", err)
	}
	payload, marshalErr := json.Marshal(map[string]string{"name": slug})
	if marshalErr != nil {
		return marshalErr
	}
	response, err = client.request(ctx, http.MethodPost, "/v1/workspace/new", bytes.NewReader(payload), "application/json")
	if err != nil {
		return fmt.Errorf("ensure AnythingLLM workspace: %w", err)
	}
	defer response.Body.Close()
	return nil
}

func (client *Client) DeleteWorkspace(ctx context.Context, knowledgeBaseID string) error {
	response, err := client.request(ctx, http.MethodDelete, "/v1/workspace/"+url.PathEscape(workspaceSlug(knowledgeBaseID)), nil, "")
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func (client *Client) UpsertRevision(ctx context.Context, knowledgeBaseID, revisionID, contentType string, content []byte) error {
	if err := client.EnsureWorkspace(ctx, knowledgeBaseID); err != nil {
		return err
	}
	slug := workspaceSlug(knowledgeBaseID)
	// An earlier attempt may have embedded only some parsed documents before
	// failing. Remove its membership and rebuild the whole immutable revision;
	// seeing one path alone cannot prove that every parsed part was indexed.
	if documents, err := client.workspaceDocuments(ctx, slug); err == nil {
		for _, document := range documents {
			if document.belongsToRevision(revisionID) {
				if err := client.RemoveRevision(ctx, knowledgeBaseID, revisionID); err != nil {
					return fmt.Errorf("replace partial AnythingLLM revision: %w", err)
				}
				break
			}
		}
	} else {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", revisionID+anythingLLMExtension(contentType))
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	_ = writer.WriteField("metadata", fmt.Sprintf(`{"title":%q,"docSource":%q}`, revisionID, "agent-platform"))
	if err := writer.Close(); err != nil {
		return err
	}
	response, err := client.request(ctx, http.MethodPost, "/v1/document/upload", &body, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var uploaded struct {
		Success   bool   `json:"success"`
		Error     string `json:"error"`
		Documents []struct {
			Location string `json:"location"`
		} `json:"documents"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&uploaded); err != nil {
		return fmt.Errorf("decode AnythingLLM upload: %w", err)
	}
	if !uploaded.Success || len(uploaded.Documents) == 0 {
		return fmt.Errorf("AnythingLLM did not accept a document for indexing")
	}
	locations := make([]string, 0, len(uploaded.Documents))
	for _, document := range uploaded.Documents {
		if document.Location == "" {
			return fmt.Errorf("AnythingLLM returned an empty document location")
		}
		locations = append(locations, document.Location)
	}
	payload, err := json.Marshal(map[string][]string{"adds": locations})
	if err != nil {
		return err
	}
	response, err = client.request(ctx, http.MethodPost, "/v1/workspace/"+url.PathEscape(slug)+"/update-embeddings", bytes.NewReader(payload), "application/json")
	if err != nil {
		return fmt.Errorf("embed AnythingLLM document: %w", err)
	}
	var indexed struct {
		Workspace json.RawMessage `json:"workspace"`
		Message   string          `json:"message"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&indexed)
	response.Body.Close()
	if decodeErr != nil || len(indexed.Workspace) == 0 || string(indexed.Workspace) == "null" || indexed.Message != "" {
		return fmt.Errorf("AnythingLLM did not confirm embedding the document")
	}
	// A successful HTTP response or upload alone is not indexing evidence.
	// Require the immutable revision's document to appear in this workspace.
	documents, err := client.workspaceDocuments(ctx, slug)
	if err != nil {
		return err
	}
	for _, location := range locations {
		found := false
		for _, document := range documents {
			if document.Location == location && document.belongsToRevision(revisionID) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("AnythingLLM embedded document is missing from the Knowledge Base workspace")
		}
	}
	return nil
}

type workspaceDocument struct {
	Location string `json:"docpath"`
	Filename string `json:"filename"`
	Metadata string `json:"metadata"`
}

func (document workspaceDocument) belongsToRevision(revisionID string) bool {
	if strings.Contains(document.Filename, revisionID) || strings.Contains(document.Location, revisionID) {
		return true
	}
	var metadata struct {
		Title string `json:"title"`
	}
	return json.Unmarshal([]byte(document.Metadata), &metadata) == nil && metadata.Title == revisionID
}

func (client *Client) workspaceDocuments(ctx context.Context, slug string) ([]workspaceDocument, error) {
	response, err := client.request(ctx, http.MethodGet, "/v1/workspace/"+url.PathEscape(slug), nil, "")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result struct {
		Workspace []struct {
			Documents []workspaceDocument `json:"documents"`
		} `json:"workspace"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode AnythingLLM workspace: %w", err)
	}
	if len(result.Workspace) != 1 {
		return nil, fmt.Errorf("AnythingLLM workspace response is unavailable")
	}
	return result.Workspace[0].Documents, nil
}

func anythingLLMExtension(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/pdf":
		return ".pdf"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "text/markdown":
		return ".md"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "text/html", "application/xhtml+xml":
		return ".html"
	default:
		return ".txt"
	}
}

func (client *Client) RemoveRevision(ctx context.Context, knowledgeBaseID, revisionID string) error {
	slug := workspaceSlug(knowledgeBaseID)
	documents, err := client.workspaceDocuments(ctx, slug)
	if err != nil {
		return err
	}
	var paths []string
	for _, document := range documents {
		if document.Location != "" && document.belongsToRevision(revisionID) {
			paths = append(paths, document.Location)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	payload, err := json.Marshal(map[string][]string{"deletes": paths})
	if err != nil {
		return err
	}
	response, err := client.request(ctx, http.MethodPost, "/v1/workspace/"+url.PathEscape(slug)+"/update-embeddings", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	response.Body.Close()
	remaining, err := client.workspaceDocuments(ctx, slug)
	if err != nil {
		return err
	}
	for _, document := range remaining {
		if document.belongsToRevision(revisionID) {
			return fmt.Errorf("AnythingLLM still contains the removed Knowledge revision")
		}
	}
	return nil
}

func (client *Client) Query(ctx context.Context, knowledgeBaseID string, generation int64, query string, limit int, tokens int) (Retrieval, error) {
	if generation <= 0 {
		return Retrieval{NoHit: true}, nil
	}
	payload, err := json.Marshal(map[string]any{"query": query, "topN": limit})
	if err != nil {
		return Retrieval{}, err
	}
	response, err := client.request(ctx, http.MethodPost, path.Join("/v1/workspace", workspaceSlug(knowledgeBaseID), "vector-search"), bytes.NewReader(payload), "application/json")
	if err != nil {
		return Retrieval{}, fmt.Errorf("query AnythingLLM: %w", err)
	}
	defer response.Body.Close()
	var result struct {
		Results []struct {
			Text     string  `json:"text"`
			Score    float32 `json:"score"`
			Distance float32 `json:"distance"`
			Metadata struct {
				URL         string `json:"url"`
				Title       string `json:"title"`
				ChunkSource string `json:"chunkSource"`
			} `json:"metadata"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&result); err != nil {
		return Retrieval{}, err
	}
	output := Retrieval{Citations: make([]Citation, 0, min(limit, len(result.Results)))}
	used := 0
	for _, item := range result.Results {
		text := strings.TrimSpace(item.Text)
		if text == "" || used >= tokens {
			continue
		}
		remaining := tokens - used
		if len([]rune(text)) > remaining*4 {
			text = string([]rune(text)[:remaining*4])
		}
		location := item.Metadata.ChunkSource
		if location == "" {
			location = item.Metadata.URL
		}
		output.Citations = append(output.Citations, Citation{KnowledgeBaseID: knowledgeBaseID, RevisionID: item.Metadata.Title, SourceLocation: location, Relevance: item.Score, Text: text})
		used += max(1, len([]rune(text))/4)
	}
	output.NoHit = len(output.Citations) == 0
	return output, nil
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
