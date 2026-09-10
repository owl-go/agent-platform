package openaiimages

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"agent-platform/backend/internal/biz/aicreation/application"
	"agent-platform/backend/internal/biz/aicreation/domain"
)

const bailianGenerationPath = "/api/v1/services/aigc/multimodal-generation/generation"

func isBailianEndpoint(endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	return err == nil && strings.TrimRight(parsed.Path, "/") == bailianGenerationPath
}

func (provider *Provider) createBailian(ctx context.Context, connection Connection, request application.ProviderRequest) (application.ProviderResult, error) {
	content := make([]map[string]string, 0, len(request.Inputs)+1)
	for index, input := range request.Inputs {
		dataURL, err := provider.referenceDataURL(ctx, input, index)
		if err != nil {
			return application.ProviderResult{}, err
		}
		content = append(content, map[string]string{"image": dataURL})
	}
	content = append(content, map[string]string{"text": request.Prompt})
	body, err := json.Marshal(map[string]any{
		"model":      request.ModelID,
		"input":      map[string]any{"messages": []any{map[string]any{"role": "user", "content": content}}},
		"parameters": map[string]any{"size": strings.ReplaceAll(request.Size, "x", "*"), "n": request.Count, "prompt_extend": true, "watermark": false},
	})
	if err != nil {
		return application.ProviderResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, connection.Endpoint, bytes.NewReader(body))
	if err != nil {
		return application.ProviderResult{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+string(connection.APIKey))
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := provider.client.Do(httpRequest)
	httpRequest.Header.Del("Authorization")
	if err != nil {
		return application.ProviderResult{}, &application.ProviderFailure{Code: "image_outcome_unknown", OutcomeUnknown: true, Cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return application.ProviderResult{}, bailianFailure(response.StatusCode)
	}
	var payload struct {
		Code   string `json:"code"`
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Image string `json:"image"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return application.ProviderResult{}, &application.ProviderFailure{Code: "image_outcome_unknown", OutcomeUnknown: true, Cause: err}
	}
	if payload.Code != "" {
		return application.ProviderResult{}, &application.ProviderFailure{Code: "image_provider_rejected", Cause: fmt.Errorf("Bailian returned error code %s", payload.Code)}
	}
	urls := make([]string, 0, request.Count)
	for _, choice := range payload.Output.Choices {
		for _, item := range choice.Message.Content {
			if item.Image != "" {
				urls = append(urls, item.Image)
			}
		}
	}
	if len(urls) == 0 || len(urls) > request.Count {
		return application.ProviderResult{}, &application.ProviderFailure{Code: "image_output_invalid", Cause: fmt.Errorf("Bailian returned an invalid result count")}
	}
	result := application.ProviderResult{Images: make([][]byte, 0, len(urls))}
	for _, imageURL := range urls {
		image, err := provider.downloadBailianImage(ctx, imageURL)
		if err != nil {
			return application.ProviderResult{}, err
		}
		result.Images = append(result.Images, image)
	}
	return result, nil
}

func (provider *Provider) referenceDataURL(ctx context.Context, input domain.ReferenceImage, index int) (string, error) {
	reader, err := provider.sources.Get(ctx, input.ObjectKey)
	if err != nil {
		return "", fmt.Errorf("open Reference Image %d: %w", index+1, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, 20*1024*1024+1))
	closeErr := reader.Close()
	if readErr != nil {
		return "", fmt.Errorf("read Reference Image %d: %w", index+1, readErr)
	}
	if len(data) > 20*1024*1024 {
		return "", fmt.Errorf("read Reference Image %d: image exceeds 20 MiB", index+1)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close Reference Image %d: %w", index+1, closeErr)
	}
	return "data:" + input.MediaType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (provider *Provider) downloadBailianImage(ctx context.Context, value string) ([]byte, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || (parsed.Hostname() != "aliyuncs.com" && !strings.HasSuffix(parsed.Hostname(), ".aliyuncs.com")) || parsed.User != nil {
		return nil, &application.ProviderFailure{Code: "image_output_invalid", Cause: fmt.Errorf("Bailian returned an untrusted image URL")}
	}
	return provider.downloadImageURL(ctx, value)
}

func bailianFailure(status int) error {
	code := "image_provider_rejected"
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		code = "image_provider_authentication_failed"
	case http.StatusTooManyRequests:
		code = "image_provider_rate_limited"
	default:
		if status >= 500 {
			code = "image_provider_unavailable"
		}
	}
	return &application.ProviderFailure{Code: code, Cause: fmt.Errorf("Bailian API returned status %d", status)}
}
