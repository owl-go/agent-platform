package minio_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/internal/objectstore"
	"agent-platform/internal/objectstore/conformance"
	minioadapter "agent-platform/internal/objectstore/minio"
)

func TestProviderConformance(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	bucket := os.Getenv("MINIO_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("MINIO_ENDPOINT and MINIO_BUCKET are required for integration conformance")
	}
	runPrefix := "conformance/" + time.Now().UTC().Format("20060102T150405.000000000")
	conformance.Run(t, func(t *testing.T) objectstore.Provider {
		provider, err := minioadapter.New(minioadapter.Config{
			Endpoint:     endpoint,
			AccessKey:    os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey:    os.Getenv("MINIO_SECRET_KEY"),
			SessionToken: os.Getenv("MINIO_SESSION_TOKEN"),
			Bucket:       bucket,
			Secure:       strings.EqualFold(os.Getenv("MINIO_SECURE"), "true"),
			Prefix:       runPrefix + "/" + sanitize(t.Name()),
		})
		if err != nil {
			t.Fatalf("create MinIO provider: %v", err)
		}
		t.Cleanup(func() {
			_, _ = provider.DeleteExpired(context.Background(), objectstore.LifecycleQuery{Before: time.Now().Add(time.Hour)})
		})
		return provider
	})
}

func sanitize(value string) string {
	return strings.NewReplacer("/", "-", " ", "-").Replace(value)
}
