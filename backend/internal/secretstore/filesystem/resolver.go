package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"agent-platform/backend/internal/credentials"
)

const (
	maxSecretFiles = 64
	maxSecretBytes = 4 * 1024 * 1024
)

type Resolver struct {
	root         string
	resolvedRoot string
}

var _ credentials.Resolver = (*Resolver)(nil)

func New(root string) (*Resolver, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("Secret Store root must be absolute")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect Secret Store root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("Secret Store root must be a non-symlink directory inaccessible to group and other users")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve Secret Store root: %w", err)
	}
	return &Resolver{root: filepath.Clean(root), resolvedRoot: filepath.Clean(resolvedRoot)}, nil
}

func (resolver *Resolver) Resolve(ctx context.Context, runID string, bindings []credentials.Binding) (credentials.Request, error) {
	if strings.TrimSpace(runID) == "" || len(bindings) == 0 {
		return credentials.Request{}, fmt.Errorf("Run ID and Credential Bindings are required")
	}
	request := credentials.Request{Ref: runID, Variables: make(map[string]string), Files: make(map[string][]byte)}
	fileCount, byteCount := 0, 0
	for _, binding := range bindings {
		if err := ctx.Err(); err != nil {
			return credentials.Request{}, err
		}
		if !validPurpose(binding.Purpose) {
			return credentials.Request{}, fmt.Errorf("unsupported Credential Binding purpose %q", binding.Purpose)
		}
		bundle, err := resolver.bundlePath(binding.Ref)
		if err != nil {
			return credentials.Request{}, err
		}
		err = filepath.WalkDir(bundle, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 || !entry.IsDir() && !entry.Type().IsRegular() {
				return fmt.Errorf("Secret bundle contains a symlink or special file")
			}
			if entry.IsDir() {
				if info.Mode().Perm()&0o077 != 0 {
					return fmt.Errorf("Secret bundle directory %q is accessible to group or other users", path)
				}
				return nil
			}
			if info.Mode().Perm()&0o077 != 0 {
				return fmt.Errorf("Secret bundle file %q is accessible to group or other users", path)
			}
			fileCount++
			byteCount += int(info.Size())
			if fileCount > maxSecretFiles || byteCount > maxSecretBytes || info.Size() <= 0 || info.Size() > maxSecretBytes {
				return fmt.Errorf("Secret bundle exceeds file or byte limits")
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(bundle, path)
			if err != nil {
				return err
			}
			parts := strings.Split(filepath.ToSlash(relative), "/")
			switch {
			case len(parts) == 2 && parts[0] == "env":
				name := parts[1]
				if _, exists := request.Variables[name]; exists {
					return fmt.Errorf("Credential Bindings define duplicate environment variable %q", name)
				}
				request.Variables[name] = string(contents)
			case len(parts) >= 2 && parts[0] == "files":
				name := strings.Join(parts[1:], "/")
				if _, exists := request.Files[name]; exists {
					return fmt.Errorf("Credential Bindings define duplicate file %q", name)
				}
				request.Files[name] = contents
			default:
				return fmt.Errorf("Secret bundle entries must be under env/ or files/")
			}
			return nil
		})
		if err != nil {
			return credentials.Request{}, fmt.Errorf("load Secret bundle %q: %w", binding.Ref, err)
		}
	}
	if len(request.Variables) == 0 && len(request.Files) == 0 {
		return credentials.Request{}, fmt.Errorf("Credential Bindings resolved to no Secret material")
	}
	return request, nil
}

func (resolver *Resolver) bundlePath(reference string) (string, error) {
	parsed, err := url.Parse(reference)
	if err != nil || parsed.Scheme != "secret" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return "", fmt.Errorf("invalid Secret Ref")
	}
	local := strings.TrimPrefix(parsed.Host+parsed.Path, "/")
	if local == "" || !filepath.IsLocal(filepath.FromSlash(local)) {
		return "", fmt.Errorf("Secret Ref must identify a local bundle")
	}
	path := filepath.Join(resolver.root, filepath.FromSlash(local))
	if !strings.HasPrefix(path+string(os.PathSeparator), resolver.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("Secret Ref escapes configured root")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("Secret Ref does not exist")
		}
		return "", err
	}
	expectedResolved := filepath.Join(resolver.resolvedRoot, filepath.FromSlash(local))
	if filepath.Clean(resolved) != filepath.Clean(expectedResolved) {
		return "", fmt.Errorf("Secret Ref traverses a symlink")
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("Secret Ref does not exist")
		}
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Secret Ref must resolve to a non-symlink directory")
	}
	return path, nil
}

func validPurpose(value string) bool {
	switch value {
	case "model", "git_ssh", "build", "object_storage":
		return true
	default:
		return false
	}
}
