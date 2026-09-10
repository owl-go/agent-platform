package workspace

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (service *Service) uploadReferenceImage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 20*1024*1024+1024*1024)
	file, _, err := request.FormFile("image")
	if err != nil {
		http.Error(writer, "invalid Reference Image", http.StatusUnprocessableEntity)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 20*1024*1024+1))
	if err != nil || len(data) > 20*1024*1024 {
		http.Error(writer, "invalid Reference Image", http.StatusUnprocessableEntity)
		return
	}
	upload, err := service.aicreation.UploadReference(request.Context(), owner, data)
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "reference_image_invalid")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(writer).Encode(map[string]any{"id": upload.ID, "media_type": upload.Image.MediaType, "encoded_size": upload.Image.Size, "width": upload.Image.Width, "height": upload.Image.Height, "expires_at": upload.Image.ExpiresAt})
}

func (service *Service) deleteReferenceImage(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if request.Method != http.MethodDelete || len(parts) != 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "ai-creation" || parts[3] != "reference-images" || parts[4] == "" {
		http.NotFound(writer, request)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	if err := service.aicreation.DeleteReference(request.Context(), owner, parts[4]); err != nil {
		http.NotFound(writer, request)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (service *Service) downloadGeneratedImages(writer http.ResponseWriter, request *http.Request) {
	recordID, _, ok := imageGenerationPath(request.URL.Path, "download")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	record, err := service.aicreation.GetRecord(request.Context(), owner, recordID)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "application/zip")
	writer.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=image-generation-%s.zip", record.ID))
	writer.Header().Set("Cache-Control", "private, no-store")
	archive := zip.NewWriter(writer)
	defer archive.Close()
	for _, image := range record.Images {
		reader, _, err := service.objects.Get(request.Context(), image.ObjectKey)
		if err != nil {
			return
		}
		entry, err := archive.Create(fmt.Sprintf("generated-%d.%s", image.Position, imageExtension(image.MediaType)))
		if err == nil {
			_, err = io.Copy(entry, reader)
		}
		_ = reader.Close()
		if err != nil {
			return
		}
	}
}

func (service *Service) streamImageGeneration(writer http.ResponseWriter, request *http.Request) {
	recordID, _, ok := imageGenerationPath(request.URL.Path, "events")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	if _, err := service.aicreation.GetRecord(request.Context(), owner, recordID); err != nil {
		http.NotFound(writer, request)
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("X-Accel-Buffering", "no")
	last := int64(0)
	if parsed, err := strconv.ParseInt(request.Header.Get("Last-Event-ID"), 10, 64); err == nil {
		last = parsed
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		events, err := service.aicreation.ListEvents(request.Context(), owner, recordID, last)
		if err != nil {
			return
		}
		for _, event := range events {
			payload, _ := json.Marshal(map[string]any{"created_at": event.CreatedAt})
			writeSSE(writer, event.Sequence, event.Type, payload)
			flusher.Flush()
			last = event.Sequence
		}
		record, err := service.aicreation.GetRecord(request.Context(), owner, recordID)
		if err != nil {
			return
		}
		if record.State.Terminal() {
			return
		}
		select {
		case <-request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (service *Service) downloadGeneratedImage(writer http.ResponseWriter, request *http.Request) {
	recordID, positionValue, ok := imageGenerationPath(request.URL.Path, "images")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	record, err := service.aicreation.GetRecord(request.Context(), owner, recordID)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	position, err := strconv.Atoi(positionValue)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	for _, image := range record.Images {
		if image.Position != position {
			continue
		}
		reader, object, err := service.objects.Get(request.Context(), image.ObjectKey)
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		defer reader.Close()
		writer.Header().Set("Content-Type", image.MediaType)
		writer.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
		writer.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=generated-%d.%s", position, imageExtension(image.MediaType)))
		writer.Header().Set("Cache-Control", "private, no-store")
		if _, err := io.Copy(writer, reader); err != nil {
			return
		}
		return
	}
	http.NotFound(writer, request)
}

func imageGenerationPath(path, action string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if action == "images" && len(parts) == 7 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "ai-creation" && parts[3] == "image-generations" && parts[5] == "images" {
		return parts[4], parts[6], true
	}
	if action != "images" && len(parts) == 6 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "ai-creation" && parts[3] == "image-generations" && parts[5] == action {
		return parts[4], "", true
	}
	return "", "", false
}

func imageExtension(mediaType string) string {
	switch mediaType {
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}
