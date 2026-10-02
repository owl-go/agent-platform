// Package ragflow implements trusted ingestion and retrieval through RAGFlow's
// versioned HTTP API. Product identity and authorization never come from RAGFlow.
package ragflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/knowledgebase/retrieval"
	"github.com/google/uuid"
)

type Config struct {
	Endpoint, APIKey, DeploymentID, EmbeddingModel string
	RequestTimeout, ParseTimeout, PollInterval     time.Duration
}

type Mapping struct{ BaseID, RevisionID, DatasetID, DocumentID string }

// Store keeps provider identity outside product snapshots. GenerationMappings
// must return an immutable, platform-owned revision set, never an entire dataset.
type Store interface {
	GetMapping(context.Context, string, string) (Mapping, error)
	SaveMapping(context.Context, string, Mapping) error
	GenerationMappings(context.Context, string, string, int64) ([]Mapping, error)
	DeleteMapping(context.Context, string, string) error
	CleanupMappings(context.Context, string) ([]Mapping, error)
}

var ErrMappingNotFound = errors.New("RAGFlow mapping not found")

// APIError retains only the stable numeric category, never a provider message.
type APIError struct{ Code int }

func (e *APIError) Error() string { return fmt.Sprintf("RAGFlow API error %d", e.Code) }

type Client struct {
	config Config
	http   *http.Client
	store  Store
}

func New(config Config, store Store, transport http.RoundTripper) (*Client, error) {
	u, err := url.Parse(config.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return nil, fmt.Errorf("RAGFlow requires HTTPS or loopback HTTP")
	}
	if strings.TrimSpace(config.APIKey) == "" || strings.ContainsAny(config.APIKey, "\r\n") || config.DeploymentID == "" || config.EmbeddingModel == "" || store == nil {
		return nil, fmt.Errorf("RAGFlow configuration and mapping store are required")
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.ParseTimeout <= 0 {
		config.ParseTimeout = 5 * time.Minute
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 2 * time.Second
	}
	if transport == nil {
		transport = http.DefaultTransport
	}
	config.Endpoint = strings.TrimRight(config.Endpoint, "/")
	return &Client{config: config, store: store, http: &http.Client{Transport: transport, Timeout: config.RequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) request(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.config.Endpoint+"/api/v1"+path, body)
	if err != nil {
		return fmt.Errorf("construct RAGFlow request")
	}
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("ngrok-skip-browser-warning", "true")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(req)
	// Transport errors and provider messages can contain secrets and URLs.
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("RAGFlow transport unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("RAGFlow HTTP status %d", response.StatusCode)
	}
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if err != nil || len(data) > 8*1024*1024 || json.Unmarshal(data, &envelope) != nil || envelope.Code == nil {
		return fmt.Errorf("RAGFlow invalid response")
	}
	if *envelope.Code != 0 {
		return &APIError{Code: *envelope.Code}
	}
	if out != nil && json.Unmarshal(envelope.Data, out) != nil {
		return fmt.Errorf("RAGFlow invalid data")
	}
	return nil
}

func (c *Client) json(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode RAGFlow request")
		}
		reader = bytes.NewReader(data)
	}
	return c.request(ctx, method, path, reader, "application/json", out)
}

type dataset struct{ ID, Name string }
type document struct {
	ID, Name, Run string
	Progress      float64
	ChunkCount    int `json:"chunk_count"`
}

func (c *Client) dataset(ctx context.Context, baseID string) (string, error) {
	digest := sha256.Sum256([]byte(c.config.DeploymentID + ":" + baseID))
	name := "aw-" + hex.EncodeToString(digest[:])
	var datasets []dataset
	if err := c.json(ctx, "GET", "/datasets?name="+url.QueryEscape(name)+"&page_size=100", nil, &datasets); err != nil {
		// v0.24 returns permission code 108 for an absent exact-name lookup.
		// This name is derived only from our deployment/base namespace. Dataset
		// creation still enforces tenant ownership and uniqueness in RAGFlow.
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Code != 108 {
			return "", err
		}
	}
	for _, d := range datasets {
		if d.Name == name && validID(d.ID) {
			return d.ID, nil
		}
	}
	var created dataset
	err := c.json(ctx, "POST", "/datasets", map[string]any{"name": name, "permission": "me", "embedding_model": c.config.EmbeddingModel, "chunk_method": "naive", "parser_config": map[string]any{"chunk_token_num": 512, "layout_recognize": "DeepDOC", "raptor": map[string]any{"use_raptor": false}}}, &created)
	if err != nil {
		return "", err
	}
	if !validID(created.ID) {
		return "", fmt.Errorf("RAGFlow invalid dataset identity")
	}
	return created.ID, nil
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func (c *Client) documents(ctx context.Context, datasetID, filter string) ([]document, error) {
	var result struct {
		Docs []document `json:"docs"`
	}
	err := c.json(ctx, "GET", "/datasets/"+datasetID+"/documents?page_size=100&"+filter, nil, &result)
	return result.Docs, err
}

func extension(contentType string) (string, error) {
	switch strings.Split(contentType, ";")[0] {
	case "text/plain":
		return ".txt", nil
	case "text/markdown":
		return ".md", nil
	case "text/html":
		return ".html", nil
	case "application/pdf":
		return ".pdf", nil
	case "application/msword":
		return ".doc", nil
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx", nil
	case "application/vnd.ms-excel":
		return ".xls", nil
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx", nil
	case "image/png":
		return ".png", nil
	case "image/jpeg":
		return ".jpg", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", fmt.Errorf("RAGFlow unsupported source type")
	}
}

func (c *Client) UpsertRevision(ctx context.Context, baseID, revisionID, contentType string, content []byte) error {
	if _, err := uuid.Parse(baseID); err != nil {
		return fmt.Errorf("invalid Knowledge Base identity")
	}
	if _, err := uuid.Parse(revisionID); err != nil {
		return fmt.Errorf("invalid Document Revision identity")
	}
	if len(content) == 0 || len(content) > 100*1024*1024 {
		return fmt.Errorf("RAGFlow source size is invalid")
	}
	ext, err := extension(contentType)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.ParseTimeout)
	defer cancel()
	mapping, err := c.store.GetMapping(ctx, c.config.DeploymentID, revisionID)
	if errors.Is(err, ErrMappingNotFound) {
		datasetID, err := c.dataset(ctx, baseID)
		if err != nil {
			return err
		}
		name := revisionID + ext
		docs, err := c.documents(ctx, datasetID, "name="+url.QueryEscape(name))
		if err != nil {
			// The exact-name lookup returns 102 when this revision is not uploaded.
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Code != 102 {
				return err
			}
		}
		docID := ""
		for _, d := range docs {
			if d.Name == name {
				docID = d.ID
				break
			}
		}
		if docID == "" {
			var upload bytes.Buffer
			writer := multipart.NewWriter(&upload)
			part, err := writer.CreateFormFile("file", name)
			if err != nil {
				return err
			}
			if _, err = part.Write(content); err != nil {
				return err
			}
			if err = writer.Close(); err != nil {
				return err
			}
			var uploaded []document
			if err = c.request(ctx, "POST", "/datasets/"+datasetID+"/documents", &upload, writer.FormDataContentType(), &uploaded); err != nil {
				return err
			}
			if len(uploaded) != 1 {
				return fmt.Errorf("RAGFlow upload did not return one document")
			}
			docID = uploaded[0].ID
		}
		if !validID(docID) {
			return fmt.Errorf("RAGFlow invalid document identity")
		}
		mapping = Mapping{BaseID: baseID, RevisionID: revisionID, DatasetID: datasetID, DocumentID: docID}
		if err = c.store.SaveMapping(ctx, c.config.DeploymentID, mapping); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if mapping.BaseID != baseID || !validID(mapping.DatasetID) || !validID(mapping.DocumentID) {
		return fmt.Errorf("RAGFlow mapping mismatch")
	}
	started := false
	for {
		docs, err := c.documents(ctx, mapping.DatasetID, "id="+url.QueryEscape(mapping.DocumentID))
		if err != nil {
			return err
		}
		if len(docs) != 1 || docs[0].ID != mapping.DocumentID {
			return fmt.Errorf("RAGFlow indexed document is missing")
		}
		doc := docs[0]
		switch doc.Run {
		case "DONE", "3":
			if doc.ChunkCount <= 0 {
				return fmt.Errorf("RAGFlow parsing produced no searchable chunks")
			}
			var indexed struct {
				Chunks []struct {
					DocumentID string `json:"document_id"`
					Content    string `json:"content"`
					Available  bool   `json:"available"`
				} `json:"chunks"`
			}
			if err := c.json(ctx, "GET", "/datasets/"+mapping.DatasetID+"/documents/"+mapping.DocumentID+"/chunks?page_size=1", nil, &indexed); err != nil {
				return err
			}
			if len(indexed.Chunks) > 0 {
				chunk := indexed.Chunks[0]
				if chunk.DocumentID != mapping.DocumentID || strings.TrimSpace(chunk.Content) == "" || !chunk.Available {
					return fmt.Errorf("RAGFlow indexed chunk is not a usable source")
				}
				return nil
			}
			// DONE can precede Elasticsearch's searchable refresh. Wait for an
			// actual indexed chunk instead of publishing a misleading Ready flag.
		case "FAIL", "4", "CANCEL", "2":
			if started {
				return fmt.Errorf("RAGFlow document parsing failed")
			}
		}
		if !started && (doc.Run == "UNSTART" || doc.Run == "0" || doc.Run == "FAIL" || doc.Run == "4" || doc.Run == "CANCEL" || doc.Run == "2") {
			if err = c.json(ctx, "POST", "/datasets/"+mapping.DatasetID+"/chunks", map[string]any{"document_ids": []string{mapping.DocumentID}}, nil); err != nil {
				return err
			}
			started = true
		}
		timer := time.NewTimer(c.config.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// Previous revisions are retained for frozen generations and thirty-day restore.
func (c *Client) RetainsRevisions() bool { return true }

func (c *Client) RemoveRevision(ctx context.Context, baseID, revisionID string) error {
	mapping, err := c.store.GetMapping(ctx, c.config.DeploymentID, revisionID)
	if errors.Is(err, ErrMappingNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if mapping.BaseID != baseID || !validID(mapping.DatasetID) || !validID(mapping.DocumentID) {
		return fmt.Errorf("RAGFlow mapping mismatch")
	}
	docs, err := c.documents(ctx, mapping.DatasetID, "id="+url.QueryEscape(mapping.DocumentID))
	if err != nil {
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Code != 102 {
			return err
		}
		// Explicit cleanup: the known provider document or dataset is already gone.
		return c.store.DeleteMapping(ctx, c.config.DeploymentID, revisionID)
	}
	if len(docs) == 0 {
		return c.store.DeleteMapping(ctx, c.config.DeploymentID, revisionID)
	}
	if err = c.json(ctx, "DELETE", "/datasets/"+mapping.DatasetID+"/documents", map[string]any{"ids": []string{mapping.DocumentID}}, nil); err != nil {
		return err
	}
	return c.store.DeleteMapping(ctx, c.config.DeploymentID, revisionID)
}

func (c *Client) Query(ctx context.Context, baseID string, generation int64, question string, limit, tokenLimit int) (retrieval.Result, error) {
	mappings, err := c.store.GenerationMappings(ctx, c.config.DeploymentID, baseID, generation)
	if err != nil {
		return retrieval.Result{}, err
	}
	if len(mappings) == 0 {
		// A valid generation whose sources are all deleted is an indexed no-hit.
		return retrieval.Result{Citations: []retrieval.Citation{}}, nil
	}
	ids := make([]string, 0, len(mappings))
	revisions := make(map[string]string, len(mappings))
	datasetID := mappings[0].DatasetID
	for _, m := range mappings {
		if m.BaseID != baseID || m.DatasetID != datasetID || !validID(m.DatasetID) || !validID(m.DocumentID) {
			return retrieval.Result{}, fmt.Errorf("RAGFlow generation mapping mismatch")
		}
		ids = append(ids, m.DocumentID)
		revisions[m.DocumentID] = m.RevisionID
	}
	if limit <= 0 || tokenLimit <= 0 {
		return retrieval.Result{}, nil
	}
	limit = min(limit, 100)
	var result struct {
		Chunks []struct {
			DocumentID string  `json:"document_id"`
			DatasetID  string  `json:"dataset_id"`
			Content    string  `json:"content"`
			Similarity float32 `json:"similarity"`
		} `json:"chunks"`
	}
	err = c.json(ctx, "POST", "/retrieval", map[string]any{"question": question, "dataset_ids": []string{datasetID}, "document_ids": ids, "page_size": limit, "top_k": 1024, "similarity_threshold": 0.2, "vector_similarity_weight": 0.3, "highlight": false}, &result)
	if err != nil {
		return retrieval.Result{}, err
	}
	out := retrieval.Result{Citations: []retrieval.Citation{}}
	remaining := tokenLimit // Conservative Unicode character budget, including CJK.
	for _, chunk := range result.Chunks {
		revision, ok := revisions[chunk.DocumentID]
		if !ok || (chunk.DatasetID != "" && chunk.DatasetID != datasetID) {
			continue
		}
		if math.IsNaN(float64(chunk.Similarity)) || math.IsInf(float64(chunk.Similarity), 0) {
			continue
		}
		text := []rune(strings.TrimSpace(chunk.Content))
		if len(text) == 0 {
			continue
		}
		if len(text) > remaining {
			text = text[:remaining]
		}
		remaining -= len(text)
		out.Citations = append(out.Citations, retrieval.Citation{RevisionID: revision, Text: string(text), Relevance: max(0, min(1, chunk.Similarity))})
		if remaining == 0 || len(out.Citations) == limit {
			break
		}
	}
	return out, nil
}

func (c *Client) Cleanup(ctx context.Context) (bool, error) {
	mappings, err := c.store.CleanupMappings(ctx, c.config.DeploymentID)
	if err != nil {
		return false, err
	}
	for _, m := range mappings {
		if err = c.RemoveRevision(ctx, m.BaseID, m.RevisionID); err != nil {
			return true, err
		}
	}
	return len(mappings) > 0, nil
}
