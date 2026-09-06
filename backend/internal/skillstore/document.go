package skillstore

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"unicode/utf8"
)

func (store *Store) Document(ctx context.Context, key, digest string) (string, error) {
	reader, object, err := store.objects.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("read Skill package: %w", err)
	}
	defer reader.Close()
	if object.Size > maxArchiveSize {
		return "", fmt.Errorf("Skill package exceeds size limit")
	}
	content, err := io.ReadAll(io.LimitReader(reader, maxArchiveSize+1))
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	if len(content) > maxArchiveSize || hex.EncodeToString(hash[:]) != digest {
		return "", fmt.Errorf("Skill package checksum mismatch")
	}
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", err
	}
	for _, file := range archive.File {
		if file.Name != "SKILL.md" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return "", err
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, maxArchiveSize+1))
		closeErr := stream.Close()
		if readErr != nil {
			return "", readErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if len(body) > maxArchiveSize || !utf8.Valid(body) {
			return "", fmt.Errorf("Skill document is not bounded UTF-8 text")
		}
		return string(body), nil
	}
	return "", fmt.Errorf("Skill package has no SKILL.md")
}
