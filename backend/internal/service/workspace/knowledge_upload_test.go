package workspace

import (
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/google/uuid"
)

func TestKnowledgeDocumentActionPaths(t *testing.T) {
	baseID := uuid.NewString()
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

type multipartFileHeader struct {
	filename    string
	contentType string
}

func (header multipartFileHeader) fileHeader() *multipart.FileHeader {
	values := make(textproto.MIMEHeader)
	values.Set("Content-Type", header.contentType)
	return &multipart.FileHeader{Filename: header.filename, Header: values}
}
