package openaiimages

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"agent-platform/backend/internal/biz/aicreation/application"
)

const maxResponseBytes = 110 * 1024 * 1024

type Connection struct {
	Endpoint string
	APIKey   []byte
}

type ImageModelConnectionResolver interface {
	ResolveImageModel(context.Context, string) (Connection, error)
}

type ImageSources interface {
	Get(context.Context, string) (io.ReadCloser, error)
}

type Provider struct {
	connections ImageModelConnectionResolver
	sources     ImageSources
	client      *http.Client
	imageClient *http.Client
}

func New(connections ImageModelConnectionResolver, sources ImageSources, client *http.Client) (*Provider, error) {
	if connections == nil || sources == nil {
		return nil, fmt.Errorf("Image Provider connection resolver and image sources are required")
	}
	imageClient := client
	if client == nil {
		client = http.DefaultClient
		imageClient = newPublicImageClient()
	}
	return &Provider{connections: connections, sources: sources, client: client, imageClient: imageClient}, nil
}

var _ application.ImageProvider = (*Provider)(nil)

func (provider *Provider) Create(ctx context.Context, request application.ProviderRequest) (application.ProviderResult, error) {
	connection, err := provider.connections.ResolveImageModel(ctx, request.ModelRevisionID)
	if err != nil {
		return application.ProviderResult{}, fmt.Errorf("resolve Image Model connection: %w", err)
	}
	defer clear(connection.APIKey)
	if isBailianEndpoint(connection.Endpoint) {
		return provider.createBailian(ctx, connection, request)
	}
	var httpRequest *http.Request
	if len(request.Inputs) == 0 {
		httpRequest, err = provider.generationRequest(ctx, connection.Endpoint, request)
	} else {
		httpRequest, err = provider.editRequest(ctx, connection.Endpoint, request)
	}
	if err != nil {
		return application.ProviderResult{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+string(connection.APIKey))
	defer httpRequest.Header.Del("Authorization")
	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return application.ProviderResult{}, &application.ProviderFailure{Code: "image_outcome_unknown", OutcomeUnknown: true, Cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var providerError struct {
			Error struct {
				Code string `json:"code"`
				Type string `json:"type"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&providerError)
		code := "image_provider_rejected"
		if providerError.Error.Code == "content_policy_violation" || providerError.Error.Type == "image_generation_user_error" {
			code = "image_safety_rejected"
		}
		switch response.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			code = "image_provider_authentication_failed"
		case http.StatusTooManyRequests:
			code = "image_provider_rate_limited"
		default:
			if response.StatusCode >= 500 {
				code = "image_provider_unavailable"
			}
		}
		return application.ProviderResult{}, &application.ProviderFailure{Code: code, Cause: fmt.Errorf("Images API returned status %d", response.StatusCode)}
	}
	var payload struct {
		Data []struct {
			Base64 string `json:"b64_json"`
			URL    string `json:"url"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(&payload); err != nil {
		return application.ProviderResult{}, &application.ProviderFailure{Code: "image_outcome_unknown", OutcomeUnknown: true, Cause: err}
	}
	if len(payload.Data) == 0 || len(payload.Data) > request.Count {
		return application.ProviderResult{}, &application.ProviderFailure{Code: "image_output_invalid", Cause: fmt.Errorf("Images API returned an invalid result count")}
	}
	result := application.ProviderResult{Images: make([][]byte, 0, len(payload.Data))}
	for _, item := range payload.Data {
		if (item.Base64 == "") == (item.URL == "") {
			return application.ProviderResult{}, &application.ProviderFailure{Code: "image_output_invalid", Cause: fmt.Errorf("Images API returned an ambiguous image result")}
		}
		if item.URL != "" {
			downloaded, err := provider.downloadImageURL(ctx, item.URL)
			if err != nil {
				return application.ProviderResult{}, err
			}
			result.Images = append(result.Images, downloaded)
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(item.Base64)
		if err != nil || len(decoded) > 25*1024*1024 {
			return application.ProviderResult{}, &application.ProviderFailure{Code: "image_output_invalid", Cause: fmt.Errorf("decode Images API image: invalid Base64 output")}
		}
		result.Images = append(result.Images, decoded)
	}
	return result, nil
}

func (provider *Provider) generationRequest(ctx context.Context, endpoint string, request application.ProviderRequest) (*http.Request, error) {
	body, err := json.Marshal(map[string]any{
		"model": request.ModelID, "prompt": request.Prompt, "size": request.Size, "quality": request.Quality,
		"output_format": request.Format, "background": request.Background, "n": request.Count,
	})
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, joinEndpoint(endpoint, "images/generations"), bytes.NewReader(body))
	if err == nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	return httpRequest, err
}

func (provider *Provider) editRequest(ctx context.Context, endpoint string, request application.ProviderRequest) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"model": request.ModelID, "prompt": request.Prompt, "size": request.Size, "quality": request.Quality,
		"output_format": request.Format, "background": request.Background, "n": strconv.Itoa(request.Count),
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, err
		}
	}
	for index, input := range request.Inputs {
		reader, err := provider.sources.Get(ctx, input.ObjectKey)
		if err != nil {
			return nil, fmt.Errorf("open Reference Image %d: %w", index+1, err)
		}
		part, err := writer.CreateFormFile("image[]", fmt.Sprintf("reference-%d.%s", index+1, extension(input.MediaType)))
		if err == nil {
			_, err = io.Copy(part, io.LimitReader(reader, 20*1024*1024+1))
		}
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("encode Reference Image %d: %w", index+1, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close Reference Image %d: %w", index+1, closeErr)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, joinEndpoint(endpoint, "images/edits"), &body)
	if err == nil {
		httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	}
	return httpRequest, err
}

func joinEndpoint(endpoint, suffix string) string {
	return strings.TrimRight(endpoint, "/") + "/" + suffix
}

func extension(mediaType string) string {
	switch mediaType {
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}
