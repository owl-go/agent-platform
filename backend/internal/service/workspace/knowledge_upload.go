package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/workspacefs"

	"github.com/google/uuid"
)

var knowledgeContentTypes = map[string][]string{
	".pdf":  {"application/pdf"},
	".doc":  {"application/msword", "application/octet-stream"},
	".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/zip"},
	".xls":  {"application/vnd.ms-excel", "application/octet-stream"},
	".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/zip"},
	".md":   {"text/markdown", "text/plain"},
	".txt":  {"text/plain"},
	".png":  {"image/png"},
	".jpg":  {"image/jpeg"},
	".jpeg": {"image/jpeg"},
	".webp": {"image/webp", "application/octet-stream"},
	".html": {"text/html", "application/xhtml+xml"},
}

func (service *Service) uploadKnowledgeDocument(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, administrator, err := service.knowledgePrincipal(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	knowledgeBaseID, ok := knowledgeBaseIDFromUploadPath(request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, workspacefs.UploadLimit+1<<20)
	if err := request.ParseMultipartForm(workspacefs.UploadLimit); err != nil {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "knowledge_document_too_large")
		return
	}
	file, header, err := request.FormFile("file")
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "knowledge_document_required")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, workspacefs.UploadLimit+1))
	if err != nil || int64(len(content)) > workspacefs.UploadLimit {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "knowledge_document_too_large")
		return
	}
	name, contentType, err := validateKnowledgeUpload(header, content)
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "unsupported_knowledge_document")
		return
	}
	var categoryID *string
	if value := strings.TrimSpace(request.FormValue("category_id")); value != "" {
		if _, err := uuid.Parse(value); err != nil {
			writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_knowledge_category")
			return
		}
		categoryID = &value
	}
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	objectID := uuid.NewString()
	objectKey := fmt.Sprintf("knowledge/%s/%s/%s", owner, knowledgeBaseID, objectID)
	stored, err := service.objects.Put(request.Context(), objectKey, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: contentType, Metadata: map[string]string{"name": name}})
	if err != nil {
		writeAuthError(writer, http.StatusInternalServerError, "knowledge_document_upload_failed")
		return
	}
	item, err := service.workspace.Repository().CreateKnowledgeDocument(request.Context(), owner, administrator, workspacedomain.KnowledgeDocumentInput{KnowledgeBaseID: knowledgeBaseID, CategoryID: categoryID, Name: name, SourceType: workspacedomain.KnowledgeUpload, NormalizedSource: sha, ObjectKey: stored.Key, SHA256: stored.SHA256, Size: stored.Size, ContentType: stored.ContentType})
	if err != nil {
		_ = service.objects.Delete(context.WithoutCancel(request.Context()), objectKey)
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(publicKnowledgeDocument(item))
}

type knowledgeURLImport struct {
	URL        string  `json:"url"`
	CategoryID *string `json:"category_id,omitempty"`
}

func (service *Service) importKnowledgeDocument(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	owner, administrator, err := service.knowledgePrincipal(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	knowledgeBaseID, ok := knowledgeBaseIDFromDocumentActionPath(request.URL.Path, "import")
	if !ok {
		http.NotFound(writer, request)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	var input knowledgeURLImport
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil || validateKnowledgeURL(input.URL) != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "unsafe_knowledge_url")
		return
	}
	if input.CategoryID != nil {
		if _, err := uuid.Parse(strings.TrimSpace(*input.CategoryID)); err != nil {
			writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_knowledge_category")
			return
		}
	}
	parsed, _ := url.Parse(strings.TrimSpace(input.URL))
	client := publicKnowledgeURLClient()
	fetchRequest, err := http.NewRequestWithContext(request.Context(), http.MethodGet, parsed.String(), nil)
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "unsafe_knowledge_url")
		return
	}
	response, err := client.Do(fetchRequest)
	if err != nil {
		writeAuthError(writer, http.StatusBadGateway, "knowledge_url_fetch_failed")
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		writeAuthError(writer, http.StatusBadGateway, "knowledge_url_fetch_failed")
		return
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, workspacefs.UploadLimit+1))
	if err != nil || int64(len(content)) > workspacefs.UploadLimit || len(content) == 0 {
		writeAuthError(writer, http.StatusRequestEntityTooLarge, "knowledge_document_too_large")
		return
	}
	name := filepath.Base(parsed.Path)
	if name == "." || name == "/" || name == "" {
		name = "web-page.html"
	} else if filepath.Ext(name) == "" {
		name += ".html"
	}
	name, err = attachmentName(name)
	if err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "invalid_knowledge_document")
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if contentType == "" {
		contentType = "text/html"
	}
	if _, _, err := validateKnowledgeURLSnapshot(name, contentType, content); err != nil {
		writeAuthError(writer, http.StatusUnprocessableEntity, "unsupported_knowledge_document")
		return
	}
	item, err := service.persistKnowledgeDocument(request.Context(), owner, administrator, knowledgeBaseID, input.CategoryID, name, workspacedomain.KnowledgeURL, input.URL, content, contentType)
	if err != nil {
		writeAuthError(writer, publicStatus(err), publicReason(err))
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(writer).Encode(publicKnowledgeDocument(item))
}

func (service *Service) persistKnowledgeDocument(ctx context.Context, owner string, administrator bool, knowledgeBaseID string, categoryID *string, name string, sourceType workspacedomain.KnowledgeSourceType, sourceURI string, content []byte, contentType string) (workspacedomain.KnowledgeDocument, error) {
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	objectKey := fmt.Sprintf("knowledge/%s/%s/%s", owner, knowledgeBaseID, uuid.NewString())
	stored, err := service.objects.Put(ctx, objectKey, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: contentType, Metadata: map[string]string{"name": name, "source_url": sourceURI}})
	if err != nil {
		return workspacedomain.KnowledgeDocument{}, err
	}
	item, err := service.workspace.Repository().CreateKnowledgeDocument(ctx, owner, administrator, workspacedomain.KnowledgeDocumentInput{KnowledgeBaseID: knowledgeBaseID, CategoryID: categoryID, Name: name, SourceType: sourceType, SourceURI: optionalString(sourceURI), NormalizedSource: sourceURI, ObjectKey: stored.Key, SHA256: stored.SHA256, Size: stored.Size, ContentType: stored.ContentType})
	if err != nil {
		_ = service.objects.Delete(context.WithoutCancel(ctx), objectKey)
		return workspacedomain.KnowledgeDocument{}, err
	}
	return item, nil
}

func validateKnowledgeURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("invalid public Knowledge URL")
	}
	if address, err := netip.ParseAddr(parsed.Hostname()); err == nil && !isPublicKnowledgeAddress(address) {
		return fmt.Errorf("Knowledge URL resolves to a private address")
	}
	return nil
}

func isPublicKnowledgeAddress(address netip.Addr) bool {
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() && !address.IsLinkLocalUnicast() && !address.IsUnspecified() && !address.IsMulticast()
}

func publicKnowledgeURLClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(addresses) == 0 {
			return nil, fmt.Errorf("resolve public Knowledge URL host: %w", err)
		}
		for _, value := range addresses {
			if !isPublicKnowledgeAddress(value) {
				return nil, fmt.Errorf("Knowledge URL host resolved to a private address")
			}
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
	}
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: transport, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many Knowledge URL redirects")
		}
		return validateKnowledgeURL(request.URL.String())
	}}
}

func validateKnowledgeURLSnapshot(name, contentType string, content []byte) (string, string, error) {
	if filepath.Ext(name) == "" {
		name += ".html"
	}
	allowed, ok := knowledgeContentTypes[strings.ToLower(filepath.Ext(name))]
	if !ok || len(content) == 0 {
		return "", "", fmt.Errorf("unsupported Knowledge URL snapshot")
	}
	for _, value := range allowed {
		if contentType == value {
			return name, contentType, nil
		}
	}
	return "", "", fmt.Errorf("Knowledge URL MIME does not match extension")
}

func knowledgeBaseIDFromUploadPath(value string) (string, bool) {
	return knowledgeBaseIDFromDocumentActionPath(value, "upload")
}

func knowledgeBaseIDFromDocumentActionPath(value, action string) (string, bool) {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) != 6 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "knowledge-bases" || parts[4] != "documents" || parts[5] != action {
		return "", false
	}
	if _, err := uuid.Parse(parts[3]); err != nil {
		return "", false
	}
	return parts[3], true
}

func validateKnowledgeUpload(header *multipart.FileHeader, content []byte) (string, string, error) {
	name, err := attachmentName(header.Filename)
	if err != nil {
		return "", "", err
	}
	extension := strings.ToLower(filepath.Ext(name))
	allowed, ok := knowledgeContentTypes[extension]
	if !ok || len(content) == 0 {
		return "", "", fmt.Errorf("unsupported Knowledge Document format")
	}
	contentType := normalizeKnowledgeContentType(header.Header.Get("Content-Type"))
	detectedType := normalizeKnowledgeContentType(http.DetectContentType(content))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detectedType
	}
	declaredAllowed := false
	detectedAllowed := false
	for _, value := range allowed {
		if contentType == value {
			declaredAllowed = true
		}
		if detectedType == value {
			detectedAllowed = true
		}
	}
	if declaredAllowed && detectedAllowed {
		return name, contentType, nil
	}
	return "", "", fmt.Errorf("Knowledge Document MIME does not match extension")
}

func normalizeKnowledgeContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
}

func publicKnowledgeDocument(item workspacedomain.KnowledgeDocument) map[string]any {
	value := map[string]any{"id": item.ID, "knowledge_base_id": item.KnowledgeBaseID, "name": item.Name, "source_type": item.SourceType, "state": item.State, "deleted": item.DeletedAt != nil, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt, "version": item.Version}
	if item.CategoryID != nil {
		value["category_id"] = *item.CategoryID
	}
	if item.LatestRevision != nil {
		value["latest_revision"] = map[string]any{"id": item.LatestRevision.ID, "revision": item.LatestRevision.Revision, "sha256": item.LatestRevision.SHA256, "size": item.LatestRevision.Size, "content_type": item.LatestRevision.ContentType, "state": item.LatestRevision.State}
	}
	return value
}

func publicStatus(err error) int {
	if errors.Is(err, workspacedomain.ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, workspacedomain.ErrConflict) {
		return http.StatusPreconditionFailed
	}
	if errors.Is(err, workspacedomain.ErrInvalid) {
		return http.StatusUnprocessableEntity
	}
	if strings.Contains(publicReason(err), "not_found") {
		return http.StatusNotFound
	}
	return http.StatusUnprocessableEntity
}

func publicReason(err error) string {
	switch {
	case err == nil:
		return ""
	case strings.Contains(err.Error(), "not found"):
		return "resource_not_found"
	default:
		return "invalid_knowledge_document"
	}
}
