package promptoptimizer

import (
	"bufio"
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
	connections ConnectionResolver
	client      *http.Client
}

type ConnectionResolver interface {
	ResolvePromptOptimization(context.Context) (openaiimages.Connection, error)
}

func New(connections ConnectionResolver, client *http.Client) (*Provider, error) {
	if connections == nil {
		return nil, fmt.Errorf("Prompt Optimization connection resolver is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Provider{connections: connections, client: client}, nil
}

func (provider *Provider) Optimize(ctx context.Context, request application.OptimizationRequest) (application.OptimizationResult, error) {
	connection, err := provider.connections.ResolvePromptOptimization(ctx)
	if err != nil {
		return application.OptimizationResult{}, err
	}
	defer clear(connection.APIKey)
	instruction := request.Candidate.Instruction
	var path string
	var payload any
	switch request.Candidate.Protocol {
	case "openai_responses":
		path = "responses"
		payload = map[string]any{
			"model": request.Candidate.ModelID, "instructions": instruction, "stream": true,
			"input": []any{map[string]any{"role": "user", "content": []any{map[string]string{"type": "input_text", "text": request.Prompt}}}},
		}
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
	httpRequest.Header.Set("Accept", "text/event-stream, application/json")
	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return application.OptimizationResult{}, fmt.Errorf("call Prompt Optimization model: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return application.OptimizationResult{}, fmt.Errorf("Prompt Optimization returned status %d", response.StatusCode)
	}
	if strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		return decodeResponsesStream(response.Body)
	}
	return decodeResponsesJSON(response.Body)
}

type responsesPayload struct {
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

func decodeResponsesJSON(reader io.Reader) (application.OptimizationResult, error) {
	var raw responsesPayload
	if err := json.NewDecoder(io.LimitReader(reader, 2*1024*1024)).Decode(&raw); err != nil {
		return application.OptimizationResult{}, err
	}
	return optimizationResult(raw, ""), nil
}

func decodeResponsesStream(reader io.Reader) (application.OptimizationResult, error) {
	limited := &io.LimitedReader{R: reader, N: 2*1024*1024 + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	text := ""
	var completed responsesPayload
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event struct {
			Type     string           `json:"type"`
			Delta    string           `json:"delta"`
			Text     string           `json:"text"`
			Response responsesPayload `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return application.OptimizationResult{}, fmt.Errorf("decode Prompt Optimization stream: %w", err)
		}
		switch event.Type {
		case "response.output_text.delta":
			text += event.Delta
		case "response.output_text.done":
			if text == "" {
				text = event.Text
			}
		case "response.completed":
			completed = event.Response
		}
	}
	if err := scanner.Err(); err != nil {
		return application.OptimizationResult{}, fmt.Errorf("read Prompt Optimization stream: %w", err)
	}
	if limited.N <= 0 {
		return application.OptimizationResult{}, fmt.Errorf("Prompt Optimization response exceeds 2 MiB")
	}
	return optimizationResult(completed, text), nil
}

func optimizationResult(raw responsesPayload, streamedText string) application.OptimizationResult {
	text := streamedText
	if text == "" {
		for _, output := range raw.Output {
			for _, content := range output.Content {
				if content.Type == "output_text" {
					text += content.Text
				}
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
	return application.OptimizationResult{Prompt: strings.TrimSpace(text), InputTokens: input, OutputTokens: output}
}
