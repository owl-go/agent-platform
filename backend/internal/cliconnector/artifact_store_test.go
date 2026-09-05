package cliconnector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
