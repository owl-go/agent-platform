package workspace

import (
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
)

func TestKnowledgeDocumentActionPaths(t *testing.T) {
	baseID := uuid.NewString()
	categoryID := uuid.NewString()
	documentID := uuid.NewString()
	if value, ok := knowledgeBaseIDFromUploadPath("/api/v1/knowledge-bases/" + baseID + "/documents/upload"); !ok || value != baseID {
		t.Fatalf("upload path = %q, %t", value, ok)
	}
	if value, ok := knowledgeBaseIDFromDocumentActionPath("/api/v1/knowledge-bases/"+baseID+"/documents/import", "import"); !ok || value != baseID {
		t.Fatalf("import path = %q, %t", value, ok)
	}
	if actualBase, actualDocument, ok := knowledgeDocumentDownloadPath("/api/v1/knowledge-bases/" + baseID + "/documents/" + documentID + "/download"); !ok || actualBase != baseID || actualDocument != documentID {
		t.Fatalf("download path = %q/%q, %t", actualBase, actualDocument, ok)
	}
	if actualBase, actualDocument, ok := knowledgeDocumentActionPath("/api/v1/knowledge-bases/"+baseID+"/documents/"+documentID+"/retry", "retry"); !ok || actualBase != baseID || actualDocument != documentID {
		t.Fatalf("retry path = %q/%q, %t", actualBase, actualDocument, ok)
	}
	if actualBase, actualDocument, ok := knowledgeDocumentActionPath("/api/v1/knowledge-bases/"+baseID+"/documents/"+documentID, ""); !ok || actualBase != baseID || actualDocument != documentID {
		t.Fatalf("delete path = %q/%q, %t", actualBase, actualDocument, ok)
	}
	if actualBase, ok := knowledgeBaseActionPath("/api/v1/knowledge-bases/"+baseID+"/restore", "restore"); !ok || actualBase != baseID {
		t.Fatalf("base restore path = %q, %t", actualBase, ok)
	}
	if actualBase, actualCategory, ok := knowledgeCategoryActionPath("/api/v1/knowledge-bases/"+baseID+"/categories/"+categoryID+"/restore", "restore"); !ok || actualBase != baseID || actualCategory != categoryID {
		t.Fatalf("category restore path = %q/%q, %t", actualBase, actualCategory, ok)
	}
	if actualBase, actualDocument, ok := knowledgeDocumentActionPath("/api/v1/knowledge-bases/"+baseID+"/documents/"+documentID+"/restore", "restore"); !ok || actualBase != baseID || actualDocument != documentID {
		t.Fatalf("document restore path = %q/%q, %t", actualBase, actualDocument, ok)
	}
}

func TestValidateKnowledgeURLRejectsPrivateDestinations(t *testing.T) {
	for _, value := range []string{
		"file:///etc/passwd",
		"http://127.0.0.1/document",
		"http://10.0.0.1/document",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/document",
		"https://user:password@example.com/document",
	} {
		if err := validateKnowledgeURL(value); err == nil {
			t.Errorf("validateKnowledgeURL(%q) unexpectedly succeeded", value)
		}
	}
	if err := validateKnowledgeURL("https://example.com/reference"); err != nil {
		t.Fatalf("public URL rejected: %v", err)
	}
}

func TestValidateKnowledgeUploadRequiresMatchingType(t *testing.T) {
	header := &multipartFileHeader{filename: "guide.pdf", contentType: "application/pdf"}
	name, contentType, err := validateKnowledgeUpload(header.fileHeader(), []byte("%PDF-1.7\n"))
	if err != nil || name != "guide.pdf" || contentType != "application/pdf" {
		t.Fatalf("valid upload = %q, %q, %v", name, contentType, err)
	}
	header.contentType = "image/png"
	if _, _, err := validateKnowledgeUpload(header.fileHeader(), []byte("png")); err == nil {
		t.Fatal("mismatched MIME unexpectedly succeeded")
	}
}

func TestValidateKnowledgeUploadAcceptsTextContentTypeParameters(t *testing.T) {
	for _, test := range []struct {
		name        string
		contentType string
		content     string
	}{
		{name: "notes.md", contentType: "text/markdown", content: "# Notes\n\nText content."},
		{name: "notes.txt", contentType: "text/plain", content: "Plain text content."},
		{name: "page.html", contentType: "text/html", content: "<!doctype html><html><body>Page</body></html>"},
	} {
		header := &multipartFileHeader{filename: test.name, contentType: test.contentType}
		if _, _, err := validateKnowledgeUpload(header.fileHeader(), []byte(test.content)); err != nil {
			t.Errorf("validateKnowledgeUpload(%q) returned error: %v", test.name, err)
		}
	}
}

func TestPublicKnowledgeDocumentIncludesTimestamps(t *testing.T) {
	createdAt := time.Date(2026, time.January, 1, 2, 3, 4, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	item := workspacedomain.KnowledgeDocument{ID: "doc-1", KnowledgeBaseID: "base-1", Name: "guide.md", CreatedAt: createdAt, UpdatedAt: updatedAt}
	value := publicKnowledgeDocument(item)
	for key, expected := range map[string]time.Time{"created_at": createdAt, "updated_at": updatedAt} {
		actual, ok := value[key].(time.Time)
		if !ok || !actual.Equal(expected) {
			t.Errorf("publicKnowledgeDocument()[%s] = %#v, want %s", key, value[key], expected)
		}
	}
}

type multipartFileHeader struct {
	filename    string
	contentType string
}

func (header multipartFileHeader) fileHeader() *multipart.FileHeader {
	values := make(textproto.MIMEHeader)
	values.Set("Content-Type", header.contentType)
	return &multipart.FileHeader{Filename: header.filename, Header: values}
}
