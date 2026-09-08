package promptoptimizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"agent-platform/backend/internal/biz/aicreation/application"
	"agent-platform/backend/internal/data/aicreation/openaiimages"
)

type Provider struct {
	connections openaiimages.ConnectionResolver
	client      *http.Client
}

func New(connections openaiimages.ConnectionResolver, client *http.Client) (*Provider, error) {
	if connections == nil {
		return nil, fmt.Errorf("Prompt Optimization connection resolver is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Provider{connections: connections, client: client}, nil
}

func (provider *Provider) Optimize(ctx context.Context, request application.OptimizationRequest) (application.OptimizationResult, error) {
	connection, err := provider.connections.Resolve(ctx, request.Candidate.ConnectionID, request.Candidate.ConnectionVersion)
	if err != nil {
		return application.OptimizationResult{}, err
	}
	defer clear(connection.APIKey)
	instruction := "Expand the user's image-generation prompt with concrete visual composition, subject, lighting, materials, camera, and style details. Preserve intent and return only the improved prompt, at most 10000 Unicode characters. Reply in " + request.Locale + "."
	var path string
	var payload any
	switch request.Candidate.Protocol {
	case "openai_responses":
		path = "responses"
		payload = map[string]any{"model": request.Candidate.ModelID, "instructions": instruction, "input": request.Prompt}
	case "openai_chat_completions":
		path = "chat/completions"
		payload = map[string]any{"model": request.Candidate.ModelID, "messages": []map[string]string{{"role": "system", "content": instruction}, {"role": "user", "content": request.Prompt}}}
	default:
		return application.OptimizationResult{}, fmt.Errorf("unsupported Prompt Optimization protocol")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return application.OptimizationResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(connection.Endpoint, "/")+"/"+path, bytes.NewReader(body))
	if err != nil {
		return application.OptimizationResult{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+string(connection.APIKey))
	defer httpRequest.Header.Del("Authorization")
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return application.OptimizationResult{}, fmt.Errorf("call Prompt Optimization model: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return application.OptimizationResult{}, fmt.Errorf("Prompt Optimization returned status %d", response.StatusCode)
	}
	var raw struct {
		Output []struct {
			Content []struct{ Type, Text string } `json:"content"`
		} `json:"output"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			InputTokens      int64 `json:"input_tokens"`
			OutputTokens     int64 `json:"output_tokens"`
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&raw); err != nil {
		return application.OptimizationResult{}, err
	}
	text := ""
	for _, output := range raw.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" {
				text += content.Text
			}
		}
	}
	if text == "" && len(raw.Choices) > 0 {
		text = raw.Choices[0].Message.Content
	}
	input, output := raw.Usage.InputTokens, raw.Usage.OutputTokens
	if input == 0 {
		input = raw.Usage.PromptTokens
	}
	if output == 0 {
		output = raw.Usage.CompletionTokens
	}
	return application.OptimizationResult{Prompt: strings.TrimSpace(text), InputTokens: input, OutputTokens: output}, nil
}
