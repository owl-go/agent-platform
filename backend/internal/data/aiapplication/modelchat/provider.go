package modelchat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/application"
)

type Provider struct{ client *http.Client }

func New() *Provider { return &Provider{client: &http.Client{Timeout: 3 * time.Minute}} }

func (provider *Provider) Generate(ctx context.Context, input application.ChatRequest, onDelta func(string) error) (application.ChatResult, error) {
	endpoint, payload, err := buildRequest(input)
	if err != nil {
		return application.ChatResult{}, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return application.ChatResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return application.ChatResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream, application/json")
	if input.Protocol == "anthropic_messages" {
		request.Header.Set("x-api-key", string(input.APIKey))
		request.Header.Set("anthropic-version", "2023-06-01")
	} else if input.Protocol == "gemini" {
		request.Header.Set("x-goog-api-key", string(input.APIKey))
	} else {
		request.Header.Set("Authorization", "Bearer "+string(input.APIKey))
	}
	response, err := provider.client.Do(request)
	if err != nil {
		return application.ChatResult{}, fmt.Errorf("call Assistant model: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return application.ChatResult{}, fmt.Errorf("Assistant model returned status %d", response.StatusCode)
	}
	if strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		return decodeStream(response.Body, input.Protocol, onDelta)
	}
	if input.Stream {
		return application.ChatResult{}, fmt.Errorf("Assistant model did not provide a streaming response")
	}
	return decodeJSON(response.Body, input.Protocol, onDelta)
}

func buildRequest(input application.ChatRequest) (string, any, error) {
	base := strings.TrimRight(strings.TrimSpace(input.Endpoint), "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed == nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
		return "", nil, fmt.Errorf("invalid Assistant model endpoint")
	}
	if strings.TrimSpace(input.ModelID) == "" || len(input.Messages) == 0 {
		return "", nil, fmt.Errorf("Assistant model and messages are required")
	}
	if len(input.APIKey) == 0 {
		return "", nil, fmt.Errorf("Assistant model credential is unavailable")
	}
	switch input.Protocol {
	case "openai_chat":
		payload := map[string]any{"model": input.ModelID, "messages": input.Messages, "stream": input.Stream}
		if input.Stream {
			payload["stream_options"] = map[string]bool{"include_usage": true}
		}
		return base + "/chat/completions", payload, nil
	case "openai_responses":
		return base + "/responses", map[string]any{"model": input.ModelID, "input": input.Messages, "stream": input.Stream}, nil
	case "anthropic_messages":
		var system string
		messages := make([]application.ChatMessage, 0, len(input.Messages))
		for _, message := range input.Messages {
			if message.Role == "system" {
				system += message.Content + "\n"
			} else {
				messages = append(messages, message)
			}
		}
		return base + "/messages", map[string]any{"model": input.ModelID, "system": strings.TrimSpace(system), "messages": messages, "max_tokens": 2048, "stream": input.Stream}, nil
	case "gemini":
		var system string
		contents := make([]map[string]any, 0, len(input.Messages))
		for _, message := range input.Messages {
			if message.Role == "system" {
				system += message.Content + "\n"
				continue
			}
			role := "user"
			if message.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, map[string]any{"role": role, "parts": []map[string]string{{"text": message.Content}}})
		}
		path := "/models/" + url.PathEscape(input.ModelID) + ":generateContent"
		if input.Stream {
			path = "/models/" + url.PathEscape(input.ModelID) + ":streamGenerateContent?alt=sse"
		}
		return base + path, map[string]any{"systemInstruction": map[string]any{"parts": []map[string]string{{"text": strings.TrimSpace(system)}}}, "contents": contents}, nil
	default:
		return "", nil, fmt.Errorf("unsupported Assistant model protocol %q", input.Protocol)
	}
}

type modelEvent struct {
	Type    string          `json:"type"`
	Delta   json.RawMessage `json:"delta"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	ContentBlock struct {
		Text string `json:"text"`
	} `json:"content_block"`
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	Response *struct {
		Usage *tokenUsage `json:"usage"`
	} `json:"response"`
	Message *struct {
		Usage *tokenUsage `json:"usage"`
	} `json:"message"`
	Usage         *tokenUsage `json:"usage"`
	UsageMetadata *struct {
		PromptTokenCount     int64 `json:"promptTokenCount"`
		CandidatesTokenCount int64 `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

type tokenUsage struct {
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

func decodeJSON(reader io.Reader, protocol string, onDelta func(string) error) (application.ChatResult, error) {
	var event modelEvent
	if err := json.NewDecoder(io.LimitReader(reader, 4*1024*1024)).Decode(&event); err != nil {
		return application.ChatResult{}, fmt.Errorf("decode Assistant model response: %w", err)
	}
	text := ""
	switch protocol {
	case "openai_chat":
		if len(event.Choices) > 0 {
			text = event.Choices[0].Message.Content
		}
	case "openai_responses":
		for _, item := range event.Output {
			for _, part := range item.Content {
				if part.Type == "output_text" {
					text += part.Text
				}
			}
		}
	case "anthropic_messages":
		for _, part := range event.Content {
			if part.Type == "text" {
				text += part.Text
			}
		}
	case "gemini":
		for _, candidate := range event.Candidates {
			for _, part := range candidate.Content.Parts {
				text += part.Text
			}
		}
	}
	if text == "" {
		return application.ChatResult{}, fmt.Errorf("Assistant model returned no text")
	}
	if onDelta != nil {
		if err := onDelta(text); err != nil {
			return application.ChatResult{}, err
		}
	}
	result := application.ChatResult{Text: text}
	applyUsage(&result, event)
	return result, nil
}

func decodeStream(reader io.Reader, protocol string, onDelta func(string) error) (application.ChatResult, error) {
	limited := &io.LimitedReader{R: reader, N: 4*1024*1024 + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	result := application.ChatResult{}
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event modelEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return result, fmt.Errorf("decode Assistant model stream: %w", err)
		}
		chunk := ""
		switch protocol {
		case "openai_chat":
			if len(event.Choices) > 0 {
				chunk = event.Choices[0].Delta.Content
			}
		case "openai_responses":
			if event.Type == "response.output_text.delta" {
				_ = json.Unmarshal(event.Delta, &chunk)
			}
		case "anthropic_messages":
			if event.Type == "content_block_delta" {
				var delta struct {
					Text string `json:"text"`
				}
				_ = json.Unmarshal(event.Delta, &delta)
				chunk = delta.Text
			}
			if event.Type == "content_block_start" {
				chunk = event.ContentBlock.Text
			}
		case "gemini":
			for _, candidate := range event.Candidates {
				for _, part := range candidate.Content.Parts {
					chunk += part.Text
				}
			}
		}
		if chunk != "" {
			result.Text += chunk
			if len(result.Text) > 256*1024 {
				return result, fmt.Errorf("Assistant model output exceeds limit")
			}
			if onDelta != nil {
				if err := onDelta(chunk); err != nil {
					return result, err
				}
			}
		}
		applyUsage(&result, event)
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("read Assistant model stream: %w", err)
	}
	if limited.N <= 0 {
		return result, fmt.Errorf("Assistant model stream exceeds limit")
	}
	if result.Text == "" {
		return result, fmt.Errorf("Assistant model returned no text")
	}
	return result, nil
}

func applyUsage(result *application.ChatResult, event modelEvent) {
	usage := event.Usage
	if event.Response != nil && event.Response.Usage != nil {
		usage = event.Response.Usage
	}
	if event.Message != nil && event.Message.Usage != nil {
		usage = event.Message.Usage
	}
	if usage != nil {
		input := usage.InputTokens
		if input == 0 {
			input = usage.PromptTokens
		}
		output := usage.OutputTokens
		if output == 0 {
			output = usage.CompletionTokens
		}
		if input > 0 {
			result.InputTokens = input
		}
		if output > 0 {
			result.OutputTokens = output
		}
		result.UsageKnown = result.InputTokens > 0 || result.OutputTokens > 0
	}
	if event.UsageMetadata != nil {
		result.InputTokens, result.OutputTokens = event.UsageMetadata.PromptTokenCount, event.UsageMetadata.CandidatesTokenCount
		result.UsageKnown = result.InputTokens > 0 || result.OutputTokens > 0
	}
}
