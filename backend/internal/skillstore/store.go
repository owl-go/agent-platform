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
	if len(archive) == 0 || len(archive) > maxArchiveSize {
		return "", "", fmt.Errorf("Skill archive must contain 1-50 MiB")
	}
	if err := validateArchive(archive); err != nil {
		return "", "", err
	}
	return store.put(ctx, ownerID, archive)
}

func (store *Store) InstallGit(ctx context.Context, ownerID, repositoryURL, ref string) (objectKey, digest, resolvedRef string, err error) {
	repositoryURL = strings.TrimSpace(repositoryURL)
	parsed, parseErr := url.Parse(repositoryURL)
	if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", "", fmt.Errorf("Skill Git URL must use HTTPS")
	}
	temporary, err := os.MkdirTemp("", "agent-workspace-skill-*")
	if err != nil {
		return "", "", "", err
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
		return "", "", "", fmt.Errorf("clone Skill: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if _, err := os.Stat(filepath.Join(temporary, "SKILL.md")); err != nil {
		return "", "", "", fmt.Errorf("Skill repository root must contain SKILL.md")
	}
	revision := exec.CommandContext(ctx, "git", "-C", temporary, "rev-parse", "HEAD")
	output, err := revision.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("resolve Skill revision: %w", err)
	}
	archive, err := zipDirectory(temporary)
	if err != nil {
		return "", "", "", err
	}
	key, hash, err := store.put(ctx, ownerID, archive)
	return key, hash, strings.TrimSpace(string(output)), err
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

func validateArchive(archive []byte) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("Skill archive is not a valid ZIP: %w", err)
	}
	found := false
	for _, file := range reader.File {
		name := filepath.ToSlash(filepath.Clean(file.Name))
		if name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("Skill archive contains an unsafe path")
		}
		if name == "SKILL.md" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("Skill archive root must contain SKILL.md")
	}
	return nil
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
