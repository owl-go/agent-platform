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
	"unicode/utf8"

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
	instruction := strings.TrimSpace(request.Candidate.Instruction) + "\n\n" + outputContractInstruction
	var path string
	var payload any
	switch request.Candidate.Protocol {
	case "openai_chat":
		path = "chat/completions"
		payload = map[string]any{
			"model": request.Candidate.ModelID, "stream": true,
			"stream_options": map[string]bool{"include_usage": true},
			"messages": []any{
				map[string]string{"role": "system", "content": instruction},
				map[string]string{"role": "user", "content": request.Prompt},
			},
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
		return decodeChatStream(response.Body, request.Locale)
	}
	return decodeChatJSON(response.Body, request.Locale)
}

const outputContractInstruction = "只输出最终图片生成提示词正文，不输出解释、标题、编号、引号、Markdown、代码、文件名、多个版本、英文翻译、后续建议或问题。保留用户输入语言；中文输入只输出中文，英文输入只输出英文。"

type chatPayload struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
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

func decodeChatJSON(reader io.Reader, locale string) (application.OptimizationResult, error) {
	var raw chatPayload
	if err := json.NewDecoder(io.LimitReader(reader, 2*1024*1024)).Decode(&raw); err != nil {
		return application.OptimizationResult{}, err
	}
	return chatResult(raw, "", locale), nil
}

func decodeChatStream(reader io.Reader, locale string) (application.OptimizationResult, error) {
	limited := &io.LimitedReader{R: reader, N: 2*1024*1024 + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	text := ""
	var completed chatPayload
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event chatPayload
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return application.OptimizationResult{}, fmt.Errorf("decode Prompt Optimization stream: %w", err)
		}
		if len(event.Choices) > 0 {
			text += event.Choices[0].Delta.Content
		}
		if event.Usage.InputTokens != 0 || event.Usage.OutputTokens != 0 || event.Usage.PromptTokens != 0 || event.Usage.CompletionTokens != 0 {
			completed = event
		}
	}
	if err := scanner.Err(); err != nil {
		return application.OptimizationResult{}, fmt.Errorf("read Prompt Optimization stream: %w", err)
	}
	if limited.N <= 0 {
		return application.OptimizationResult{}, fmt.Errorf("Prompt Optimization response exceeds 2 MiB")
	}
	return chatResult(completed, text, locale), nil
}

func chatResult(raw chatPayload, streamedText, locale string) application.OptimizationResult {
	text := streamedText
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
	return application.OptimizationResult{Prompt: sanitizePrompt(text, locale), InputTokens: input, OutputTokens: output}
}

func sanitizePrompt(text, locale string) string {
	text = stripCodeFence(text)
	text = strings.TrimSpace(strings.Trim(text, "\"'"))
	lines := strings.Split(text, "\n")
	selected := make([]string, 0, len(lines))
	wantChinese := strings.HasPrefix(strings.ToLower(locale), "zh")
	variant := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "如果") || strings.HasPrefix(trimmed, "如需") || strings.HasPrefix(trimmed, "if you need") {
			break
		}
		if isChineseVariantLabel(trimmed) {
			variant = "zh"
			if !wantChinese {
				continue
			}
			trimmed = stripVariantPrefix(trimmed)
		} else if isEnglishVariantLabel(trimmed) {
			variant = "en"
			if wantChinese {
				continue
			}
			trimmed = stripVariantPrefix(trimmed)
		} else if variant != "" && ((variant == "zh") != wantChinese) {
			continue
		}
		trimmed = cleanPromptLine(trimmed)
		if trimmed != "" {
			selected = append(selected, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(selected, "\n"))
}

func stripCodeFence(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func isChineseVariantLabel(line string) bool {
	line = stripListMarker(line)
	return strings.HasPrefix(line, "中文版") || strings.HasPrefix(line, "中文版本")
}

func isEnglishVariantLabel(line string) bool {
	line = stripListMarker(line)
	return strings.HasPrefix(line, "英文版") || strings.HasPrefix(line, "英文版本") || strings.HasPrefix(strings.ToLower(line), "english version")
}

func cleanPromptLine(line string) string {
	line = stripListMarker(strings.TrimSpace(line))
	for _, prefix := range []string{"提示词：", "提示词:", "优化后的提示词：", "优化后的提示词:", "最终提示词：", "最终提示词:", "prompt:", "final prompt:"} {
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(prefix)) {
			line = strings.TrimSpace(line[len(prefix):])
			break
		}
	}
	line = strings.TrimSpace(strings.Trim(line, "`\"'"))
	line = strings.TrimPrefix(line, "**")
	line = strings.TrimSuffix(line, "**")
	return strings.TrimSpace(line)
}

func stripListMarker(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"- ", "• ", "* "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	for index, runeValue := range line {
		if runeValue < '0' || runeValue > '9' {
			if index > 0 && (strings.HasPrefix(line[index:], ". ") || strings.HasPrefix(line[index:], ") ")) {
				return strings.TrimSpace(line[index+2:])
			}
			break
		}
	}
	return line
}

func stripVariantPrefix(line string) string {
	if index := strings.IndexAny(line, ":："); index >= 0 {
		_, size := utf8.DecodeRuneInString(line[index:])
		return line[index+size:]
	}
	return line
}
