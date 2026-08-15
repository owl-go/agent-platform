package conformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"agent-platform/internal/objectstore"
)

type Factory func(t *testing.T) objectstore.Provider

var conformanceHTTPClient = &http.Client{Timeout: 10 * time.Second}

func Run(t *testing.T, factory Factory) {
	t.Helper()

	t.Run("put get and metadata", func(t *testing.T) {
		provider := factory(t)
		contents := []byte("artifact contents")
		created, err := provider.Put(context.Background(), "runs/run-1/artifact.txt", bytes.NewReader(contents), objectstore.PutOptions{
			Size:        int64(len(contents)),
			SHA256:      checksum(contents),
			ContentType: "text/plain",
			Metadata:    map[string]string{"run-id": "run-1"},
		})
		if err != nil {
			t.Fatalf("put object: %v", err)
		}
		if created.Key != "runs/run-1/artifact.txt" || created.SHA256 != checksum(contents) {
			t.Fatalf("created metadata: %+v", created)
		}

		reader, metadata, err := provider.Get(context.Background(), created.Key)
		if err != nil {
			t.Fatalf("get object: %v", err)
		}
		defer reader.Close()
		got, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read object: %v", err)
		}
		if !bytes.Equal(got, contents) || metadata.ContentType != "text/plain" || metadata.Metadata["run-id"] != "run-1" {
			t.Fatalf("retrieved object = %q, metadata = %+v", got, metadata)
		}

		stat, err := provider.Stat(context.Background(), created.Key)
		if err != nil {
			t.Fatalf("stat object: %v", err)
		}
		if stat.Size != int64(len(contents)) || stat.SHA256 != checksum(contents) {
			t.Fatalf("stat metadata: %+v", stat)
		}
	})

	t.Run("large object", func(t *testing.T) {
		provider := factory(t)
		contents := bytes.Repeat([]byte("m"), 12*1024*1024)
		_, err := provider.Put(context.Background(), "runs/run-1/large.bin", bytes.NewReader(contents), objectstore.PutOptions{
			Size:   int64(len(contents)),
			SHA256: checksum(contents),
		})
		if err != nil {
			t.Fatalf("put large object: %v", err)
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		provider := factory(t)
		_, err := provider.Put(context.Background(), "runs/run-1/bad.bin", strings.NewReader("wrong"), objectstore.PutOptions{
			Size:   5,
			SHA256: strings.Repeat("0", 64),
		})
		if !errors.Is(err, objectstore.ErrChecksumMismatch) {
			t.Fatalf("expected checksum mismatch, got %v", err)
		}
		if _, err := provider.Stat(context.Background(), "runs/run-1/bad.bin"); !errors.Is(err, objectstore.ErrNotFound) {
			t.Fatalf("checksum failure persisted object: %v", err)
		}
	})

	t.Run("cancelled upload", func(t *testing.T) {
		provider := factory(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		contents := []byte("cancelled")
		_, err := provider.Put(ctx, "runs/run-1/cancelled.bin", bytes.NewReader(contents), objectstore.PutOptions{
			Size:   int64(len(contents)),
			SHA256: checksum(contents),
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
		if _, err := provider.Stat(context.Background(), "runs/run-1/cancelled.bin"); !errors.Is(err, objectstore.ErrNotFound) {
			t.Fatalf("cancelled upload persisted object: %v", err)
		}
	})

	t.Run("not found and delete", func(t *testing.T) {
		provider := factory(t)
		if _, _, err := provider.Get(context.Background(), "missing"); !errors.Is(err, objectstore.ErrNotFound) {
			t.Fatalf("get missing object: %v", err)
		}
		put(t, provider, "delete-me", []byte("value"))
		if err := provider.Delete(context.Background(), "delete-me"); err != nil {
			t.Fatalf("delete object: %v", err)
		}
		if _, err := provider.Stat(context.Background(), "delete-me"); !errors.Is(err, objectstore.ErrNotFound) {
			t.Fatalf("deleted object still exists: %v", err)
		}
	})

	t.Run("presigned download", func(t *testing.T) {
		provider := factory(t)
		put(t, provider, "signed", []byte("value"))
		link, err := provider.PresignGet(context.Background(), "signed", 5*time.Minute)
		if err != nil {
			t.Fatalf("presign object: %v", err)
		}
		if link.URL == "" || link.ExpiresAt.IsZero() {
			t.Fatalf("invalid signed URL: %+v", link)
		}
		if _, err := provider.PresignGet(context.Background(), "signed", 0); !errors.Is(err, objectstore.ErrInvalidExpiry) {
			t.Fatalf("zero expiry: %v", err)
		}
	})

	t.Run("lifecycle cleanup", func(t *testing.T) {
		provider := factory(t)
		put(t, provider, "expired/one", []byte("one"))
		put(t, provider, "keep/two", []byte("two"))
		deleted, err := provider.DeleteExpired(context.Background(), objectstore.LifecycleQuery{
			Prefix: "expired/",
			Before: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("delete expired objects: %v", err)
		}
		if deleted != 1 {
			t.Fatalf("deleted count = %d, want 1", deleted)
		}
		if _, err := provider.Stat(context.Background(), "keep/two"); err != nil {
			t.Fatalf("cleanup deleted object outside prefix: %v", err)
		}
	})
}

// RunHTTPBehavior verifies properties that an in-memory provider cannot model.
func RunHTTPBehavior(t *testing.T, factory Factory) {
	t.Helper()
	provider := factory(t)
	contents := []byte("private artifact")
	put(t, provider, "http/private.txt", contents)

	signed, err := provider.PresignGet(context.Background(), "http/private.txt", time.Second)
	if err != nil {
		t.Fatalf("presign object: %v", err)
	}
	assertHTTPBody(t, signed.URL, contents)

	unsigned, err := url.Parse(signed.URL)
	if err != nil {
		t.Fatalf("parse signed URL: %v", err)
	}
	unsigned.RawQuery = ""
	response, err := conformanceHTTPClient.Get(unsigned.String()) // #nosec G107 -- conformance target is explicitly configured.
	if err != nil {
		t.Fatalf("request unsigned URL: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t.Fatalf("private object was accessible without signature: status %d", response.StatusCode)
	}

	time.Sleep(1500 * time.Millisecond)
	response, err = conformanceHTTPClient.Get(signed.URL) // #nosec G107 -- conformance target is explicitly configured.
	if err != nil {
		t.Fatalf("request expired URL: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		t.Fatalf("expired signed URL remained accessible: status %d", response.StatusCode)
	}
}

func assertHTTPBody(t *testing.T, target string, want []byte) {
	t.Helper()
	response, err := conformanceHTTPClient.Get(target) // #nosec G107 -- conformance target is explicitly configured.
	if err != nil {
		t.Fatalf("request signed URL: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("signed URL status = %d", response.StatusCode)
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read signed URL response: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("signed URL body = %q, want %q", got, want)
	}
}

func put(t *testing.T, provider objectstore.Provider, key string, contents []byte) {
	t.Helper()
	_, err := provider.Put(context.Background(), key, bytes.NewReader(contents), objectstore.PutOptions{
		Size:   int64(len(contents)),
		SHA256: checksum(contents),
	})
	if err != nil {
		t.Fatalf("put fixture %q: %v", key, err)
	}
}

func checksum(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
