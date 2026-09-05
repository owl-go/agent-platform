package cliconnector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"agent-platform/backend/internal/objectstore"
)

const maxBundleSize = 256 << 20

var bundleDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ArtifactStore keeps published CLI bundles private and verifies their exact
// bytes again whenever a Worker prepares a Runtime mount.
type ArtifactStore struct {
	objects objectstore.Provider
}

func NewArtifactStore(objects objectstore.Provider) (*ArtifactStore, error) {
	if objects == nil {
		return nil, errors.New("CLI bundle Object Store is required")
	}
	return &ArtifactStore{objects: objects}, nil
}

func (store *ArtifactStore) PutImmutable(ctx context.Context, key string, bundle []byte, digest string) error {
	if err := validateBundleReference(key, digest); err != nil {
		return err
	}
	if len(bundle) == 0 || len(bundle) > maxBundleSize {
		return errors.New("CLI bundle must be non-empty and no larger than 256 MiB")
	}
	actual := sha256.Sum256(bundle)
	if digest != hex.EncodeToString(actual[:]) {
		return objectstore.ErrChecksumMismatch
	}
	if existing, err := store.objects.Stat(ctx, key); err == nil {
		if existing.SHA256 == digest && existing.Size == int64(len(bundle)) {
			stored, readErr := store.GetVerified(ctx, key, digest)
			if readErr == nil && bytes.Equal(stored, bundle) {
				return nil
			}
			if readErr != nil {
				return fmt.Errorf("verify existing immutable CLI bundle: %w", readErr)
			}
		}
		return errors.New("immutable CLI bundle key already contains different content")
	} else if !errors.Is(err, objectstore.ErrNotFound) {
		return fmt.Errorf("inspect CLI bundle: %w", err)
	}
	stored, err := store.objects.Put(ctx, key, bytes.NewReader(bundle), objectstore.PutOptions{Size: int64(len(bundle)), SHA256: digest, ContentType: "application/gzip", Metadata: map[string]string{"artifact-kind": "cli-connector-bundle"}})
	if err != nil {
		return err
	}
	if stored.Key != key || stored.Size != int64(len(bundle)) || stored.SHA256 != digest {
		return errors.New("Object Store returned mismatched CLI bundle metadata")
	}
	return nil
}

func (store *ArtifactStore) GetVerified(ctx context.Context, key, digest string) ([]byte, error) {
	if err := validateBundleReference(key, digest); err != nil {
		return nil, err
	}
	body, metadata, err := store.objects.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get CLI bundle: %w", err)
	}
	defer body.Close()
	if metadata.Key != key || metadata.SHA256 != digest || metadata.Size <= 0 || metadata.Size > maxBundleSize {
		return nil, errors.New("CLI bundle metadata does not match its frozen reference")
	}
	bundle, err := io.ReadAll(io.LimitReader(body, metadata.Size+1))
	if err != nil {
		return nil, fmt.Errorf("read CLI bundle: %w", err)
	}
	if int64(len(bundle)) != metadata.Size {
		return nil, errors.New("CLI bundle size does not match Object Store metadata")
	}
	actual := sha256.Sum256(bundle)
	if digest != hex.EncodeToString(actual[:]) {
		return nil, objectstore.ErrChecksumMismatch
	}
	return bundle, nil
}

func validateBundleReference(key, digest string) error {
	if !strings.HasPrefix(key, "cli-connectors/") || !strings.HasSuffix(key, ".tgz") {
		return errors.New("invalid CLI bundle Object Key")
	}
	if !bundleDigest.MatchString(digest) {
		return errors.New("invalid CLI bundle SHA-256")
	}
	return objectstore.ValidateKey(key)
}
