package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/workspacefs"

	"github.com/google/uuid"
)

const maxTurnAttachments = 10

func (service *Service) uploadAttachment(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	name, err := attachmentName(request.URL.Query().Get("name"))
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_attachment_name")
		return
	}
	if request.ContentLength > workspacefs.UploadLimit {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "attachment_too_large")
		return
	}
	content, err := io.ReadAll(io.LimitReader(request.Body, workspacefs.UploadLimit+1))
	if err != nil || int64(len(content)) > workspacefs.UploadLimit {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "attachment_too_large")
		return
	}
	digest := sha256.Sum256(content)
	id := uuid.NewString()
	contentType := strings.TrimSpace(request.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(content)
	}
	key := attachmentObjectKey(owner, id)
	stored, err := service.objects.Put(request.Context(), key, bytes.NewReader(content), objectstore.PutOptions{
		Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]), ContentType: contentType,
		Metadata: map[string]string{"name": name},
	})
	if err != nil {
		writeAuthError(writer, http.StatusInternalServerError, "attachment_upload_failed")
		return
	}
	attachment := attachmentFromObject(id, stored)
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(publicAttachment(attachment))
}

func (service *Service) downloadAttachment(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "attachments" || parts[4] != "download" {
		http.NotFound(writer, request)
		return
	}
	id := parts[3]
	if _, err := uuid.Parse(id); err != nil {
		http.NotFound(writer, request)
		return
	}
	reader, object, err := service.objects.Get(request.Context(), attachmentObjectKey(owner, id))
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer reader.Close()
	contentType := strings.TrimSpace(object.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	writer.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": object.Metadata["name"]}))
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(writer, io.LimitReader(reader, object.Size))
}

func (service *Service) resolveAttachments(ctx context.Context, owner string, ids []string) ([]workspacedomain.Attachment, error) {
	if len(ids) > maxTurnAttachments {
		return nil, fmt.Errorf("%w: at most %d attachments are allowed per message", workspacedomain.ErrInvalid, maxTurnAttachments)
	}
	result := make([]workspacedomain.Attachment, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return nil, fmt.Errorf("%w: invalid attachment", workspacedomain.ErrInvalid)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		object, err := service.objects.Stat(ctx, attachmentObjectKey(owner, id))
		if err != nil {
			return nil, fmt.Errorf("%w: attachment not found", workspacedomain.ErrInvalid)
		}
		result = append(result, attachmentFromObject(id, object))
	}
	return result, nil
}

func attachmentObjectKey(owner, id string) string { return "attachments/" + owner + "/" + id }

func attachmentFromObject(id string, object objectstore.Object) workspacedomain.Attachment {
	name, _ := attachmentName(object.Metadata["name"])
	return workspacedomain.Attachment{ID: id, Name: name, ContentType: object.ContentType, ObjectKey: object.Key, Size: object.Size, SHA256: object.SHA256, Image: strings.HasPrefix(strings.ToLower(object.ContentType), "image/")}
}

func publicAttachment(value workspacedomain.Attachment) map[string]any {
	return map[string]any{"id": value.ID, "name": value.Name, "content_type": value.ContentType, "size": value.Size, "sha256": value.SHA256, "image": value.Image}
}

func attachmentName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." || filepath.Base(value) != value || strings.Contains(value, "\\") || len([]rune(value)) > 255 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("invalid attachment name")
	}
	return value, nil
}
