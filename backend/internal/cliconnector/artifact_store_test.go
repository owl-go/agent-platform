package cliconnector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestArtifactStoreRoundTripsVerifiedBundle(t *testing.T) {
	store, err := NewArtifactStore(memory.New())
	if err != nil {
		t.Fatal(err)
	}
	bundle := []byte("immutable CLI bundle")
	digest := sha256.Sum256(bundle)
	expected := hex.EncodeToString(digest[:])
	key := "cli-connectors/definition-1/v2/" + expected + ".tgz"
	if err := store.PutImmutable(context.Background(), key, bundle, expected); err != nil {
		t.Fatal(err)
	}
	actual, err := store.GetVerified(context.Background(), key, expected)
	if err != nil || string(actual) != string(bundle) {
		t.Fatalf("bundle=%q err=%v", actual, err)
	}
	// Repeating the same content-addressed write is safe for Worker retries.
	if err := store.PutImmutable(context.Background(), key, bundle, expected); err != nil {
		t.Fatalf("idempotent put: %v", err)
	}
}

func TestArtifactStoreMaterializesVerifiedReadOnlyBundle(t *testing.T) {
	store, err := NewArtifactStore(memory.New())
	if err != nil {
		t.Fatal(err)
	}
	bundle := testBundle(t, map[string]string{"node_modules/.bin/tool": "#!/usr/bin/env node\n"})
	sum := sha256.Sum256(bundle)
	digest := hex.EncodeToString(sum[:])
	key := "cli-connectors/definition-1/v1/" + digest + ".tgz"
	if err := store.PutImmutable(context.Background(), key, bundle, digest); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "connector")
	if err := store.MaterializeVerified(context.Background(), key, digest, destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(destination, "node_modules", ".bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o222 != 0 || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("materialized CLI mode = %o", info.Mode().Perm())
	}
	if err := store.MaterializeVerified(context.Background(), key, digest, destination); err == nil {
		t.Fatal("expected an existing destination to be rejected")
	}
}

func TestArtifactStoreRejectsDigestMismatch(t *testing.T) {
	store, err := NewArtifactStore(memory.New())
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("expected"))
	err = store.PutImmutable(context.Background(), "cli-connectors/definition-1/v1/bundle.tgz", []byte("changed"), hex.EncodeToString(digest[:]))
	if !errors.Is(err, objectstore.ErrChecksumMismatch) {
		t.Fatalf("error = %v", err)
	}
}

func TestArtifactStoreRejectsNonConnectorKey(t *testing.T) {
	store, err := NewArtifactStore(memory.New())
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("bundle"))
	if err := store.PutImmutable(context.Background(), "skills/definition-1.tgz", []byte("bundle"), hex.EncodeToString(digest[:])); err == nil {
		t.Fatal("expected an invalid key error")
	}
}
