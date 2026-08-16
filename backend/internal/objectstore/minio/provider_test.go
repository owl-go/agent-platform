package minio_test

import (
	"errors"
	"testing"

	"agent-platform/backend/internal/objectstore"
	minioadapter "agent-platform/backend/internal/objectstore/minio"
)

func TestNewRejectsUnsafePrefix(t *testing.T) {
	_, err := minioadapter.New(minioadapter.Config{
		Endpoint:  "localhost:9000",
		AccessKey: "access",
		SecretKey: "secret",
		Bucket:    "private-bucket",
		Prefix:    "../escape",
	})
	if !errors.Is(err, objectstore.ErrInvalidKey) {
		t.Fatalf("expected invalid prefix error, got %v", err)
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	_, err := minioadapter.New(minioadapter.Config{Endpoint: "localhost:9000", Bucket: "private-bucket"})
	if err == nil {
		t.Fatal("expected missing credentials to be rejected")
	}
}
