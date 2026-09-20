package domain

import (
	"fmt"
	"strings"
	"time"
)

type KnowledgeState string

const (
	KnowledgeReady    KnowledgeState = "ready"
	KnowledgeFailed   KnowledgeState = "failed"
	KnowledgeDisabled KnowledgeState = "disabled"
)

type KnowledgeBase struct {
	ID          string         `json:"id"`
	OwnerID     string         `json:"owner_id,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	State       KnowledgeState `json:"state"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Version     int64          `json:"version"`
}

func (base KnowledgeBase) Validate() error {
	if strings.TrimSpace(base.Name) == "" || len([]rune(base.Name)) > 100 {
		return fmt.Errorf("%w: knowledge base name is required", ErrInvalid)
	}
	if base.State != "" && base.State != KnowledgeReady && base.State != KnowledgeFailed && base.State != KnowledgeDisabled {
		return fmt.Errorf("%w: unsupported knowledge base state", ErrInvalid)
	}
	return nil
}

type KnowledgeDocument struct {
	ID              string         `json:"id"`
	KnowledgeBaseID string         `json:"knowledge_base_id"`
	Name            string         `json:"name"`
	Content         string         `json:"content,omitempty"`
	ContentSHA256   string         `json:"content_sha256"`
	State           KnowledgeState `json:"state"`
	FailureReason   string         `json:"failure_reason,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Version         int64          `json:"version"`
}

func (document KnowledgeDocument) Validate() error {
	if strings.TrimSpace(document.Name) == "" || len([]rune(document.Name)) > 200 {
		return fmt.Errorf("%w: document name is required", ErrInvalid)
	}
	if strings.TrimSpace(document.Content) == "" || len([]rune(document.Content)) > 2_000_000 {
		return fmt.Errorf("%w: document content is required or too large", ErrInvalid)
	}
	return nil
}

type KnowledgeChunk struct {
	ID         string  `json:"id"`
	DocumentID string  `json:"document_id"`
	Position   int     `json:"position"`
	Text       string  `json:"text"`
	Score      float32 `json:"score,omitempty"`
}
