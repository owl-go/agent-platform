package aliyunoss_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/internal/objectstore"
	aliyunadapter "agent-platform/internal/objectstore/aliyunoss"
	"agent-platform/internal/objectstore/conformance"
)

func TestProviderConformance(t *testing.T) {
	endpoint := os.Getenv("ALIYUN_OSS_ENDPOINT")
	bucket := os.Getenv("ALIYUN_OSS_BUCKET")
	if endpoint == "" || bucket == "" {
		t.Skip("ALIYUN_OSS_ENDPOINT and ALIYUN_OSS_BUCKET are required for integration conformance")
	}
	runPrefix := "conformance/" + time.Now().UTC().Format("20060102T150405.000000000")
	conformance.Run(t, func(t *testing.T) objectstore.Provider {
		provider, err := aliyunadapter.New(aliyunadapter.Config{
			Endpoint:     endpoint,
			AccessKey:    os.Getenv("ALIYUN_OSS_ACCESS_KEY"),
			SecretKey:    os.Getenv("ALIYUN_OSS_SECRET_KEY"),
			SessionToken: os.Getenv("ALIYUN_OSS_SESSION_TOKEN"),
			Bucket:       bucket,
			Prefix:       runPrefix + "/" + sanitize(t.Name()),
		})
		if err != nil {
			t.Fatalf("create Aliyun OSS provider: %v", err)
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
