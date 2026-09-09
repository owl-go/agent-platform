package openaiimages_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/aicreation/application"
	"agent-platform/backend/internal/biz/aicreation/domain"
	"agent-platform/backend/internal/data/aicreation/openaiimages"
)

func TestProviderGeneratesImagesWithJSON(t *testing.T) {
	want := []byte("generated-image")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/images/generations" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("request = %s %s, authorization = %q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "gpt-image-1" || body["prompt"] != "a red kite" || body["output_format"] != "png" || body["n"] != float64(2) {
			t.Fatalf("body = %#v", body)
		}
		if _, ok := body["response_format"]; ok {
			t.Fatalf("GPT Image request contains unsupported response_format: %#v", body)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(want)}}})
	}))
	defer server.Close()

	provider := mustProvider(t, server.URL+"/v1", nil)
	result, err := provider.Create(context.Background(), application.ProviderRequest{
		ModelRevisionID: "revision-1", ModelID: "gpt-image-1", Prompt: "a red kite",
		Size: "1024x1024", Quality: "high", Format: "png", Background: "opaque", Count: 2,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(result.Images) != 1 || !bytes.Equal(result.Images[0], want) {
		t.Fatalf("Images = %q", result.Images)
	}
}

func TestProviderEditsWithOrderedMultipartImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/images/edits" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		files := request.MultipartForm.File["image[]"]
		if len(files) != 2 || files[0].Filename != "reference-1.png" || files[1].Filename != "reference-2.png" {
			t.Fatalf("files = %#v", files)
		}
		if request.MultipartForm.Value["response_format"] != nil {
			t.Fatalf("GPT Image edit contains unsupported response_format")
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString([]byte("edited"))}}})
	}))
	defer server.Close()

	provider := mustProvider(t, server.URL, map[string][]byte{"first": []byte("one"), "second": []byte("two")})
	_, err := provider.Create(context.Background(), application.ProviderRequest{
		ModelRevisionID: "revision-1", ModelID: "gpt-image-1", Prompt: "combine",
		Inputs: []domain.ReferenceImage{{Position: 1, ObjectKey: "first", MediaType: "image/png"}, {Position: 2, ObjectKey: "second", MediaType: "image/png"}},
		Size:   "1024x1024", Quality: "high", Format: "png", Background: "opaque", Count: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestProviderUsesBailianNativeGenerationEndpoint(t *testing.T) {
	want := []byte("bailian-image")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation":
			if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer secret" {
				t.Fatalf("request = %s, authorization = %q", request.Method, request.Header.Get("Authorization"))
			}
			var body struct {
				Model string `json:"model"`
				Input struct {
					Messages []struct {
						Content []map[string]string `json:"content"`
					} `json:"messages"`
				} `json:"input"`
				Parameters struct {
					Size string `json:"size"`
					N    int    `json:"n"`
				} `json:"parameters"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			content := body.Input.Messages[0].Content
			if body.Model != "qwen-image-3.0-pro" || body.Parameters.Size != "1024*1024" || body.Parameters.N != 1 || !strings.HasPrefix(content[0]["image"], "data:image/png;base64,") || content[1]["text"] != "make it warmer" {
				t.Fatalf("body = %#v", body)
			}
			return jsonResponse(http.StatusOK, `{"output":{"choices":[{"message":{"content":[{"image":"https://result.aliyuncs.com/result.png"}]}}]}}`), nil
		case "https://result.aliyuncs.com/result.png":
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(want))}, nil
		default:
			t.Fatalf("unexpected URL %q", request.URL.String())
			return nil, nil
		}
	})}
	provider, err := openaiimages.New(
		resolver{connection: openaiimages.Connection{Endpoint: "https://workspace.cn-beijing.maas.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation", APIKey: []byte("secret")}},
		sources{objects: map[string][]byte{"reference": []byte("source-image")}}, client,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Create(context.Background(), application.ProviderRequest{
		ModelRevisionID: "revision-1", ModelID: "qwen-image-3.0-pro", Prompt: "make it warmer",
		Inputs: []domain.ReferenceImage{{Position: 1, ObjectKey: "reference", MediaType: "image/png"}},
		Size:   "1024x1024", Quality: "auto", Format: "png", Background: "opaque", Count: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(result.Images) != 1 || !bytes.Equal(result.Images[0], want) {
		t.Fatalf("Images = %q", result.Images)
	}
}

func mustProvider(t *testing.T, endpoint string, objects map[string][]byte) *openaiimages.Provider {
	t.Helper()
	provider, err := openaiimages.New(
		resolver{connection: openaiimages.Connection{Endpoint: endpoint, APIKey: []byte("secret")}},
		sources{objects: objects}, http.DefaultClient,
	)
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

type resolver struct{ connection openaiimages.Connection }

func (resolver resolver) ResolveImageModel(context.Context, string) (openaiimages.Connection, error) {
	return resolver.connection, nil
}

type sources struct{ objects map[string][]byte }

func (source sources) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(source.objects[key]))), nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}
