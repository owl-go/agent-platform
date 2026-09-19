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
		return nil, fmt.Errorf("AnythingLLM returned HTTP %d", response.StatusCode)
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
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", revisionID+anythingLLMExtension(contentType))
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	_ = writer.WriteField("addToWorkspaces", workspaceSlug(knowledgeBaseID))
	_ = writer.WriteField("metadata", fmt.Sprintf(`{"title":%q,"docSource":%q}`, revisionID, "agent-platform"))
	if err := writer.Close(); err != nil {
		return err
	}
	response, err := client.request(ctx, http.MethodPost, "/v1/document/upload", &body, writer.FormDataContentType())
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
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
	// AnythingLLM's document identifiers are provider-generated. Rebuilds are
	// isolated by Knowledge Base workspace, so cleanup is handled by workspace
	// deletion or the next index generation until provider-side delete-by-source
	// is available in the configured deployment.
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
