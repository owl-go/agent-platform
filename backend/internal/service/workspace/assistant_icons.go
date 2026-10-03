package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	aiapplicationdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	"agent-platform/backend/internal/objectstore"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

const (
	assistantIconMaxBytes  = 2 * 1024 * 1024
	assistantIconMaxPixels = 16 * 1024 * 1024
)

func (service *Service) handleAssistantIcon(writer http.ResponseWriter, request *http.Request, owner, assistantID string) {
	switch request.Method {
	case http.MethodGet:
		service.downloadAssistantIcon(writer, request, owner, assistantID)
	case http.MethodPost:
		service.uploadAssistantIcon(writer, request, owner, assistantID)
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (service *Service) handleAssistantWidgetIcon(writer http.ResponseWriter, request *http.Request, owner, assistantID string) {
	switch request.Method {
	case http.MethodGet:
		service.downloadAssistantImage(writer, request, owner, assistantID, true)
	case http.MethodPost:
		service.uploadAssistantImage(writer, request, owner, assistantID, true)
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (service *Service) uploadAssistantIcon(writer http.ResponseWriter, request *http.Request, owner, assistantID string) {
	service.uploadAssistantImage(writer, request, owner, assistantID, false)
}

func (service *Service) uploadAssistantImage(writer http.ResponseWriter, request *http.Request, owner, assistantID string, widget bool) {
	current, err := service.aiapplications.GetAssistant(request.Context(), owner, assistantID)
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	envelopeLimit := int64(assistantIconMaxBytes + 64*1024)
	if widget {
		envelopeLimit += 64 * 1024 // Configuration plus multipart headers; image limit remains 2 MiB.
	}
	request.Body = http.MaxBytesReader(writer, request.Body, envelopeLimit)
	if err := request.ParseMultipartForm(envelopeLimit); err != nil {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "assistant_icon_too_large")
		return
	}
	defer request.MultipartForm.RemoveAll()
	version, err := strconv.ParseInt(request.FormValue("version"), 10, 64)
	if err != nil || version <= 0 {
		writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_assistant_icon_version")
		return
	}
	if version != current.Version {
		service.writeAIResult(writer, nil, fmt.Errorf("%w: assistant version changed", aiapplicationdomain.ErrVersionConflict))
		return
	}
	if widget {
		var payload assistantPayload
		configuration := request.FormValue("configuration")
		if len(configuration) > 64*1024 || json.Unmarshal([]byte(configuration), &payload) != nil {
			writeAuthError(writer, http.StatusBadRequest, "invalid_assistant_configuration")
			return
		}
		candidate := assistantFromPayload(payload)
		candidate.ID, candidate.OwnerID, candidate.Version = current.ID, owner, current.Version
		if candidate.State == "" {
			candidate.State = current.State
		}
		if err := service.validateAssistantModel(request.Context(), owner, candidate.ProviderModelID); err != nil {
			service.writeAssistantModelError(writer, err)
			return
		}
		current = candidate
	}
	file, header, err := request.FormFile("icon")
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "assistant_icon_required")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, assistantIconMaxBytes+1))
	if err != nil || len(content) > assistantIconMaxBytes {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "assistant_icon_too_large")
		return
	}
	contentType, err := validateAssistantIcon(content)
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "assistant_icon_invalid")
		return
	}
	digest := sha256.Sum256(content)
	iconID := uuid.NewString()
	objectKey := assistantIconObjectKey(owner, iconID)
	stored, err := service.objects.Put(request.Context(), objectKey, bytes.NewReader(content), objectstore.PutOptions{
		Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]), ContentType: contentType,
		Metadata: map[string]string{"name": filepath.Base(header.Filename), "assistant_id": assistantID},
	})
	if err != nil {
		writeAuthError(writer, http.StatusInternalServerError, "assistant_icon_upload_failed")
		return
	}
	var updated aiapplicationdomain.SmartAssistant
	if widget {
		current.Share.WidgetIcon = stored.Key
		updated, err = service.saveAssistantConfiguration(request.Context(), owner, assistantID, current, version)
	} else {
		current.Icon = stored.Key
		updated, err = service.aiapplications.UpdateAssistant(request.Context(), owner, assistantID, current, version)
	}
	if err != nil {
		if updated.Version == 0 {
			_ = service.objects.Delete(context.WithoutCancel(request.Context()), stored.Key)
		}
		service.writeAIResult(writer, nil, err)
		return
	}
	service.writeAIResult(writer, updated, nil)
}

func (service *Service) downloadAssistantIcon(writer http.ResponseWriter, request *http.Request, owner, assistantID string) {
	service.downloadAssistantImage(writer, request, owner, assistantID, false)
}

func (service *Service) downloadAssistantImage(writer http.ResponseWriter, request *http.Request, owner, assistantID string, widget bool) {
	assistant, err := service.aiapplications.GetAssistant(request.Context(), owner, assistantID)
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	key := assistant.Icon
	if widget {
		key = assistant.Share.WidgetIcon
	}
	if !isAssistantIconObjectKey(key, owner) {
		http.NotFound(writer, request)
		return
	}
	reader, object, err := service.objects.Get(request.Context(), key)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer reader.Close()
	writer.Header().Set("Content-Type", object.ContentType)
	writer.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	writer.Header().Set("Content-Disposition", "inline")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(writer, io.LimitReader(reader, object.Size))
}

func validateAssistantIcon(content []byte) (string, error) {
	if len(content) == 0 || len(content) > assistantIconMaxBytes {
		return "", fmt.Errorf("assistant icon size is invalid")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("decode assistant icon: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 || int64(config.Width)*int64(config.Height) > assistantIconMaxPixels {
		return "", fmt.Errorf("assistant icon dimensions are invalid")
	}
	switch format {
	case "png", "jpeg", "gif", "webp":
		return "image/" + format, nil
	default:
		return "", fmt.Errorf("unsupported assistant icon format %q", format)
	}
}

func assistantIconObjectKey(owner, id string) string {
	return "ai-applications/assistant-icons/" + owner + "/" + id
}

func isAssistantIconObjectKey(key, owner string) bool {
	prefix := "ai-applications/assistant-icons/" + owner + "/"
	id := strings.TrimPrefix(key, prefix)
	if id == key || strings.Contains(id, "/") {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}
