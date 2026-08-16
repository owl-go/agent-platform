package aliyunoss_test

import (
	"errors"
	"testing"

	"agent-platform/backend/internal/objectstore"
	aliyunadapter "agent-platform/backend/internal/objectstore/aliyunoss"
)

func TestNewRejectsUnsafePrefix(t *testing.T) {
	_, err := aliyunadapter.New(aliyunadapter.Config{
		Endpoint:  "https://oss-cn-hangzhou.aliyuncs.com",
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
	_, err := aliyunadapter.New(aliyunadapter.Config{Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "private-bucket"})
	if err == nil {
		t.Fatal("expected missing credentials to be rejected")
	}
}
