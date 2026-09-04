package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestAttachmentNameRejectsPathsAndControlCharacters(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
		valid bool
	}{
		{value: "photo.png", want: "photo.png", valid: true},
		{value: "../notes.txt", valid: false},
		{value: "", valid: false},
		{value: "bad\nname.txt", valid: false},
	} {
		got, err := attachmentName(test.value)
		if test.valid && (err != nil || got != test.want) {
			t.Fatalf("attachmentName(%q) = %q, %v", test.value, got, err)
		}
		if !test.valid && err == nil {
			t.Fatalf("attachmentName(%q) unexpectedly succeeded", test.value)
		}
	}
}

func TestDownloadAttachmentStreamsAuthenticatedContent(t *testing.T) {
	provider := memory.New()
	content := []byte("image bytes")
	digest := sha256.Sum256(content)
	key := "attachments/owner-1/11111111-1111-4111-8111-111111111111"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(content), objectstore.PutOptions{
		Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]), ContentType: "image/jpeg", Metadata: map[string]string{"name": "photo.jpeg"},
	}); err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, objects: provider}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/attachments/11111111-1111-4111-8111-111111111111/download", nil)
	request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: "owner-1"}))
	response := httptest.NewRecorder()

	service.downloadAttachment(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := response.Header().Get("Content-Disposition"); got != `attachment; filename=photo.jpeg` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if !bytes.Equal(response.Body.Bytes(), content) {
		t.Fatalf("body = %q", response.Body.Bytes())
	}
}
