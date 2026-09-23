package workspace

import (
	"encoding/base64"
	"testing"
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
