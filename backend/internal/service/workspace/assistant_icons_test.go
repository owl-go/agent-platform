package workspace

import (
	aiapp "agent-platform/backend/internal/biz/aiapplication/application"
	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	creditsapp "agent-platform/backend/internal/biz/credits/application"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidateAssistantIconAcceptsDecodedPNG(t *testing.T) {
	content, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	contentType, err := validateAssistantIcon(content)
	if err != nil {
		t.Fatalf("validateAssistantIcon() error = %v", err)
	}
	if contentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", contentType)
	}
}

func TestValidateAssistantIconRejectsNonImageBytes(t *testing.T) {
	if _, err := validateAssistantIcon([]byte("not an image")); err == nil {
		t.Fatal("validateAssistantIcon() error = nil, want invalid image")
	}
}

func TestAssistantIconObjectKeyIsOwnerScoped(t *testing.T) {
	const owner = "user-1"
	const id = "2a748f68-95a5-41df-8a53-488e335c223c"
	key := assistantIconObjectKey(owner, id)
	if key != "ai-applications/assistant-icons/user-1/2a748f68-95a5-41df-8a53-488e335c223c" {
		t.Fatalf("key = %q", key)
	}
	if !isAssistantIconObjectKey(key, owner) {
		t.Fatal("owner-scoped key was not accepted")
	}
	if isAssistantIconObjectKey(key, "user-2") {
		t.Fatal("key was accepted for another owner")
	}
}

type widgetIconRepository struct {
	*publicStreamRepository
	updateErr, validationErr error
}

func (r *widgetIconRepository) UpdateAssistant(_ context.Context, owner, id string, candidate aiappdomain.SmartAssistant, version int64) (aiappdomain.SmartAssistant, error) {
	if r.updateErr != nil {
		return aiappdomain.SmartAssistant{}, r.updateErr
	}
	if owner != r.assistant.OwnerID || id != r.assistant.ID || version != r.assistant.Version {
		return aiappdomain.SmartAssistant{}, aiappdomain.ErrVersionConflict
	}
	candidate.Version = version + 1
	candidate.LastValidatedAt, candidate.ValidatedVersion = nil, 0
	r.assistant = candidate
	return candidate, nil
}
func (r *widgetIconRepository) RecordPublicationValidation(_ context.Context, _, _ string, version int64, at time.Time) (aiappdomain.SmartAssistant, error) {
	if r.validationErr != nil {
		return aiappdomain.SmartAssistant{}, r.validationErr
	}
	r.assistant.ValidatedVersion, r.assistant.LastValidatedAt = version, &at
	return r.assistant, nil
}

func (r *widgetIconRepository) PublicationStats(context.Context, string, string, time.Time) (aiappdomain.PublicationStats, error) {
	return aiappdomain.PublicationStats{}, nil
}

type widgetIconStore struct {
	objectstore.Provider
	written, deleted []string
}

func (p *widgetIconStore) Put(ctx context.Context, key string, reader io.Reader, options objectstore.PutOptions) (objectstore.Object, error) {
	p.written = append(p.written, key)
	return p.Provider.Put(ctx, key, reader, options)
}
func (p *widgetIconStore) Delete(ctx context.Context, key string) error {
	p.deleted = append(p.deleted, key)
	return p.Provider.Delete(ctx, key)
}
func widgetIconService(t *testing.T) (*Service, *widgetIconRepository, *widgetIconStore) {
	t.Helper()
	service, repository, _ := publicStreamService(t)
	repository.assistant.Share.TokenHash = publicHash(publicTestToken)
	r := &widgetIconRepository{publicStreamRepository: repository}
	application, err := aiapp.New(r)
	if err != nil {
		t.Fatal(err)
	}
	service.aiapplications = application
	credits, err := creditsapp.New(publicationCreditRepository{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.credits = credits
	store := &widgetIconStore{Provider: memory.New()}
	service.objects = store
	return service, r, store
}
func widgetIconRequest(t *testing.T, assistant aiappdomain.SmartAssistant, content []byte, version int64) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("icon", "chat.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatal(err)
	}
	configuration, _ := json.Marshal(assistant)
	form.WriteField("configuration", string(configuration))
	form.WriteField("version", strconv.FormatInt(version, 10))
	form.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ai-apps/assistants/assistant/widget-icon", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	return request
}
func TestWidgetIconUploadSavesEmbeddingWithoutChangingAvatar(t *testing.T) {
	service, r, store := widgetIconService(t)
	content, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	candidate := r.assistant
	candidate.Share.EmbedType, candidate.Share.WidgetDefaultOpen = "floating", true
	writer := httptest.NewRecorder()
	service.handleAssistantWidgetIcon(writer, widgetIconRequest(t, candidate, content, 4), "owner", "assistant")
	if writer.Code != 200 || r.assistant.Icon != "sparkles" || r.assistant.Share.EmbedType != "floating" || !r.assistant.Share.WidgetDefaultOpen || r.assistant.Version != 5 || r.assistant.ValidatedVersion != 5 || len(store.written) != 1 || len(store.deleted) != 0 {
		t.Fatalf("upload=%d %s assistant=%#v writes=%v deleted=%v", writer.Code, writer.Body.String(), r.assistant, store.written, store.deleted)
	}
	for _, public := range []bool{false, true} {
		response := httptest.NewRecorder()
		if public {
			service.publicAssistantHandler(response, httptest.NewRequest(http.MethodGet, "/api/v1/public/assistants/"+publicTestToken+"/widget-icon", nil))
		} else {
			service.handleAssistantWidgetIcon(response, httptest.NewRequest(http.MethodGet, "/", nil), "owner", "assistant")
		}
		if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), content) || response.Header().Get("Content-Type") != "image/png" || strings.Contains(response.Body.String(), store.written[0]) {
			t.Fatalf("icon=%d %s", response.Code, response.Body.String())
		}
	}
	r.assistant.Share.WidgetIcon = assistantIconObjectKey("another-owner", "2a748f68-95a5-41df-8a53-488e335c223c")
	denied := httptest.NewRecorder()
	service.publicAssistantHandler(denied, httptest.NewRequest(http.MethodGet, "/api/v1/public/assistants/"+publicTestToken+"/widget-icon", nil))
	if denied.Code != 404 {
		t.Fatal("public icon accepted another owner's object")
	}
}
func TestWidgetIconUploadRejectsInvalidRequestsAndCleansFailedWrites(t *testing.T) {
	for _, test := range []struct {
		name                    string
		owner                   string
		version                 int64
		content                 []byte
		updateErr               error
		status, writes, deletes int
	}{
		{"wrong owner", "another-owner", 4, []byte("invalid"), nil, 404, 0, 0},
		{"stale version", "owner", 3, []byte("invalid"), nil, 412, 0, 0},
		{"invalid image", "owner", 4, []byte("<svg></svg>"), nil, 422, 0, 0},
		{"oversize image", "owner", 4, make([]byte, assistantIconMaxBytes+1), nil, 413, 0, 0},
		{"failed update", "owner", 4, nil, aiappdomain.ErrVersionConflict, 412, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, r, store := widgetIconService(t)
			r.updateErr = test.updateErr
			content := test.content
			if content == nil {
				content, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
			}
			writer := httptest.NewRecorder()
			service.handleAssistantWidgetIcon(writer, widgetIconRequest(t, r.assistant, content, test.version), test.owner, "assistant")
			if writer.Code != test.status || len(store.written) != test.writes || len(store.deleted) != test.deletes || r.assistant.Version != 4 || r.assistant.Share.WidgetIcon != "" {
				t.Fatalf("status=%d writes=%v deletes=%v assistant=%#v", writer.Code, store.written, store.deleted, r.assistant)
			}
		})
	}
}
func TestWidgetIconKeepsSavedObjectIfPublicationRecordingFails(t *testing.T) {
	service, r, store := widgetIconService(t)
	r.validationErr = errors.New("publication unavailable")
	content, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	writer := httptest.NewRecorder()
	service.handleAssistantWidgetIcon(writer, widgetIconRequest(t, r.assistant, content, 4), "owner", "assistant")
	if writer.Code != 500 || r.assistant.Version != 5 || r.assistant.ValidatedVersion != 0 || len(store.deleted) != 0 {
		t.Fatal("failed publication deleted the saved image or published stale configuration")
	}
	if _, err := store.Stat(context.Background(), r.assistant.Share.WidgetIcon); err != nil {
		t.Fatal(err)
	}
}
