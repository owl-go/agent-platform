package skillstore

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"agent-platform/backend/internal/objectstore"

	"github.com/google/uuid"
)

const maxArchiveSize = 50 << 20

type Store struct{ objects objectstore.Provider }

func New(objects objectstore.Provider) (*Store, error) {
	if objects == nil {
		return nil, fmt.Errorf("Skill Object Store is required")
	}
	return &Store{objects: objects}, nil
}

func (store *Store) InstallUpload(ctx context.Context, ownerID string, archive []byte) (objectKey, digest string, err error) {
	objectKey, digest, _, err = store.InstallUploadWithMetadata(ctx, ownerID, archive)
	return objectKey, digest, err
}

func (store *Store) InstallUploadWithMetadata(ctx context.Context, ownerID string, archive []byte) (objectKey, digest string, metadata Metadata, err error) {
	normalized, _, metadata, err := ValidateUpload(ctx, archive)
	if err != nil {
		return "", "", Metadata{}, err
	}
	objectKey, digest, err = store.put(ctx, ownerID, normalized)
	return objectKey, digest, metadata, err
}

// ValidateUpload applies the same archive and metadata checks used by uploads
// without assigning an owner or writing an Object Key.
func ValidateUpload(ctx context.Context, archive []byte) ([]byte, string, Metadata, error) {
	if len(archive) == 0 || len(archive) > maxArchiveSize {
		return nil, "", Metadata{}, fmt.Errorf("Skill archive must contain 1-50 MiB")
	}
	normalized, err := normalizeArchive(ctx, archive)
	if err != nil {
		return nil, "", Metadata{}, err
	}
	metadata, err := metadataFromArchive(normalized)
	if err != nil {
		return nil, "", Metadata{}, err
	}
	sum := sha256.Sum256(normalized)
	return normalized, hex.EncodeToString(sum[:]), metadata, nil
}

func (store *Store) InstallGit(ctx context.Context, ownerID, repositoryURL, ref string) (objectKey, digest, resolvedRef string, err error) {
	objectKey, digest, resolvedRef, _, err = store.InstallGitWithMetadata(ctx, ownerID, repositoryURL, ref)
	return objectKey, digest, resolvedRef, err
}

func (store *Store) InstallGitWithMetadata(ctx context.Context, ownerID, repositoryURL, ref string) (objectKey, digest, resolvedRef string, metadata Metadata, err error) {
	repositoryURL = strings.TrimSpace(repositoryURL)
	parsed, parseErr := url.Parse(repositoryURL)
	if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", "", Metadata{}, fmt.Errorf("Skill Git URL must use HTTPS")
	}
	temporary, err := os.MkdirTemp("", "agent-workspace-skill-*")
	if err != nil {
		return "", "", "", Metadata{}, err
	}
	defer os.RemoveAll(temporary)
	args := []string{"clone", "--depth", "1"}
	if strings.TrimSpace(ref) != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, "--", repositoryURL, temporary)
	command := exec.CommandContext(ctx, "git", args...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := command.CombinedOutput(); err != nil {
		return "", "", "", Metadata{}, fmt.Errorf("clone Skill: %w: %s", err, strings.TrimSpace(string(output)))
	}
	documentFile, err := os.Open(filepath.Join(temporary, "SKILL.md"))
	if err != nil {
		return "", "", "", Metadata{}, fmt.Errorf("Skill repository root must contain SKILL.md")
	}
	document, readErr := io.ReadAll(io.LimitReader(documentFile, maxArchiveSize+1))
	closeErr := documentFile.Close()
	if readErr != nil {
		return "", "", "", Metadata{}, fmt.Errorf("read SKILL.md: %w", readErr)
	}
	if closeErr != nil {
		return "", "", "", Metadata{}, closeErr
	}
	if len(document) > maxArchiveSize {
		return "", "", "", Metadata{}, fmt.Errorf("SKILL.md exceeds size limit")
	}
	metadata, err = ParseMetadata(string(document))
	if err != nil {
		return "", "", "", Metadata{}, err
	}
	revision := exec.CommandContext(ctx, "git", "-C", temporary, "rev-parse", "HEAD")
	output, err := revision.Output()
	if err != nil {
		return "", "", "", Metadata{}, fmt.Errorf("resolve Skill revision: %w", err)
	}
	archive, err := zipDirectory(temporary)
	if err != nil {
		return "", "", "", Metadata{}, err
	}
	key, hash, err := store.put(ctx, ownerID, archive)
	return key, hash, strings.TrimSpace(string(output)), metadata, err
}

func (store *Store) put(ctx context.Context, ownerID string, archive []byte) (string, string, error) {
	digestValue := sha256.Sum256(archive)
	digest := hex.EncodeToString(digestValue[:])
	key := "skills/" + ownerID + "/" + uuid.NewString() + ".zip"
	if _, err := store.objects.Put(ctx, key, bytes.NewReader(archive), objectstore.PutOptions{Size: int64(len(archive)), SHA256: digest, ContentType: "application/zip"}); err != nil {
		return "", "", fmt.Errorf("store Skill archive: %w", err)
	}
	return key, digest, nil
}

func zipDirectory(root string) ([]byte, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == filepath.Join(root, ".git") {
			return filepath.SkipDir
		}
		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: filepath.ToSlash(relative), Method: zip.Deflate})
		if err != nil {
			return nil, err
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		_, copyErr := io.Copy(entry, io.LimitReader(file, maxArchiveSize+1))
		closeErr := file.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if buffer.Len() > maxArchiveSize {
			return nil, fmt.Errorf("Skill archive exceeds 50 MiB")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
