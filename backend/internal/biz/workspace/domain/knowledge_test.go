package domain

import (
	"errors"
	"testing"
)

func TestKnowledgeBaseInputValidation(t *testing.T) {
	tests := []struct {
		name          string
		input         KnowledgeBaseInput
		administrator bool
		wantErr       bool
	}{
		{name: "user private", input: KnowledgeBaseInput{Name: "Product Docs", Visibility: KnowledgePrivate}, wantErr: false},
		{name: "user public rejected", input: KnowledgeBaseInput{Name: "Public", Visibility: KnowledgePublic}, wantErr: true},
		{name: "admin public", input: KnowledgeBaseInput{Name: "Platform Docs", Visibility: KnowledgePublic, Platform: true}, administrator: true, wantErr: false},
		{name: "platform requires administrator", input: KnowledgeBaseInput{Name: "Platform Docs", Visibility: KnowledgePrivate, Platform: true}, wantErr: true},
		{name: "missing name", input: KnowledgeBaseInput{Visibility: KnowledgePrivate}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate(tt.administrator)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestKnowledgeSourceAndStateValidation(t *testing.T) {
	if err := ValidateKnowledgeSource(KnowledgeUpload, ""); err != nil {
		t.Fatalf("upload source should be accepted: %v", err)
	}
	if err := ValidateKnowledgeSource(KnowledgeURL, "https://example.com/docs"); err != nil {
		t.Fatalf("public URL should be accepted: %v", err)
	}
	for _, value := range []string{"ftp://example.com/docs", "https://user:pass@example.com/docs", ""} {
		if err := ValidateKnowledgeSource(KnowledgeURL, value); err == nil {
			t.Fatalf("URL %q should be rejected", value)
		}
	}
	for _, state := range []KnowledgeDocumentState{KnowledgeAccepted, KnowledgeProcessing, KnowledgeReady, KnowledgeFailed, KnowledgeBlocked} {
		if err := ValidateKnowledgeDocumentState(state); err != nil {
			t.Fatalf("state %q should be accepted: %v", state, err)
		}
	}
	if err := ValidateKnowledgeDocumentState("deleted"); err == nil {
		t.Fatal("unknown state should be rejected")
	}
}

func TestWorkflowInputRejectsDuplicateKnowledgeBases(t *testing.T) {
	input := WorkflowInput{Name: "Docs", Goal: "Answer", KnowledgeBaseIDs: []string{"kb-1", "kb-1"}}
	if err := input.Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate Knowledge Bases should be rejected, got %v", err)
	}
}
