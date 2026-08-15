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
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestProviderConformance(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	bucket := os.Getenv("MINIO_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("MINIO_ENDPOINT and MINIO_BUCKET are required for integration conformance")
	}
	ensureBucket(t, endpoint, bucket)
	runPrefix := "conformance/" + time.Now().UTC().Format("20060102T150405.000000000")
	factory := func(t *testing.T) objectstore.Provider {
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
	}
	conformance.Run(t, factory)
	t.Run("http behavior", func(t *testing.T) {
		conformance.RunHTTPBehavior(t, factory)
	})
}

func ensureBucket(t *testing.T, endpoint, bucket string) {
	t.Helper()
	if !strings.EqualFold(os.Getenv("MINIO_CREATE_BUCKET"), "true") {
		return
	}
	client, err := miniogo.New(endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), os.Getenv("MINIO_SESSION_TOKEN")),
		Secure: strings.EqualFold(os.Getenv("MINIO_SECURE"), "true"),
	})
	if err != nil {
		t.Fatalf("create MinIO setup client: %v", err)
	}
	exists, err := client.BucketExists(context.Background(), bucket)
	if err != nil {
		t.Fatalf("check MinIO bucket: %v", err)
	}
	if !exists {
		if err := client.MakeBucket(context.Background(), bucket, miniogo.MakeBucketOptions{}); err != nil {
			t.Fatalf("create MinIO bucket: %v", err)
		}
	}
}

func sanitize(value string) string {
	return strings.NewReplacer("/", "-", " ", "-").Replace(value)
}
