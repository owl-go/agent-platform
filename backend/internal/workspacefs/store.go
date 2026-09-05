package workspacefs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

const (
	WorkspaceLimit = int64(1 << 30)
	UploadLimit    = int64(100 << 20)
	PreviewLimit   = int64(1 << 20)
)

type Store struct {
	root       string
	knownHosts string
}

func New(root, knownHosts string) (*Store, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if !filepath.IsAbs(root) || root == string(filepath.Separator) {
		return nil, fmt.Errorf("Workspace root must be an absolute, non-root path")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create Workspace root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve Workspace root: %w", err)
	}
	return &Store{root: resolvedRoot, knownHosts: strings.TrimSpace(knownHosts)}, nil
}

func (store *Store) List(_ context.Context, workspacePath, relative string) ([]domain.WorkspaceEntry, int64, error) {
	directory, err := store.resolve(workspacePath, relative, true)
	if err != nil {
		return nil, 0, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, 0, fmt.Errorf("list Workspace: %w", err)
	}
	items := make([]domain.WorkspaceEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, 0, fmt.Errorf("Workspace symlinks are not allowed")
		}
		info, err := entry.Info()
		if err != nil {
			return nil, 0, fmt.Errorf("inspect Workspace entry: %w", err)
		}
		child := filepath.ToSlash(filepath.Join(relative, entry.Name()))
		items = append(items, domain.WorkspaceEntry{Path: strings.TrimPrefix(child, "./"), Name: entry.Name(), Directory: entry.IsDir(), Size: info.Size(), ModifiedAt: info.ModTime()})
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Directory != items[right].Directory {
			return items[left].Directory
		}
		return strings.ToLower(items[left].Name) < strings.ToLower(items[right].Name)
	})
	used, err := directorySize(store.workspaceRoot(workspacePath))
	return items, used, err
}

func (store *Store) CreateDirectory(_ context.Context, workspacePath, relative string) (domain.WorkspaceEntry, error) {
	path, err := store.resolve(workspacePath, relative, false)
	if err != nil {
		return domain.WorkspaceEntry{}, err
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return domain.WorkspaceEntry{}, fmt.Errorf("create Workspace directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.WorkspaceEntry{}, err
	}
	return domain.WorkspaceEntry{Path: relative, Name: filepath.Base(path), Directory: true, ModifiedAt: info.ModTime()}, nil
}

func (store *Store) Upload(_ context.Context, workspacePath, relative string, content []byte) (domain.WorkspaceEntry, error) {
	if int64(len(content)) > UploadLimit {
		return domain.WorkspaceEntry{}, fmt.Errorf("Workspace upload exceeds 100 MiB")
	}
	target, err := store.resolve(workspacePath, relative, false)
	if err != nil {
		return domain.WorkspaceEntry{}, err
	}
	used, err := directorySize(store.workspaceRoot(workspacePath))
	if err != nil {
		return domain.WorkspaceEntry{}, err
	}
	var replaced int64
	if info, err := os.Stat(target); err == nil {
		if info.IsDir() {
			return domain.WorkspaceEntry{}, fmt.Errorf("Workspace path is a directory")
		}
		replaced = info.Size()
	} else if !errors.Is(err, os.ErrNotExist) {
		return domain.WorkspaceEntry{}, err
	}
	if used-replaced+int64(len(content)) > WorkspaceLimit {
		return domain.WorkspaceEntry{}, fmt.Errorf("Workspace exceeds 1 GiB")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return domain.WorkspaceEntry{}, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return domain.WorkspaceEntry{}, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return domain.WorkspaceEntry{}, err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return domain.WorkspaceEntry{}, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return domain.WorkspaceEntry{}, err
	}
	if err := temporary.Close(); err != nil {
		return domain.WorkspaceEntry{}, err
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return domain.WorkspaceEntry{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return domain.WorkspaceEntry{}, err
	}
	return domain.WorkspaceEntry{Path: relative, Name: filepath.Base(target), Size: info.Size(), ModifiedAt: info.ModTime()}, nil
}

func (store *Store) Read(_ context.Context, workspacePath, relative string) ([]byte, time.Time, error) {
	path, err := store.resolve(workspacePath, relative, false)
	if err != nil {
		return nil, time.Time{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	if info.IsDir() || info.Size() > PreviewLimit {
		return nil, time.Time{}, fmt.Errorf("only files no larger than 1 MiB can be previewed")
	}
	content, err := os.ReadFile(path)
	return content, info.ModTime(), err
}

func (store *Store) Open(_ context.Context, workspacePath, relative string) (*os.File, os.FileInfo, error) {
	path, err := store.resolve(workspacePath, relative, false)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, fmt.Errorf("Workspace path is not a regular file")
	}
	return file, info, nil
}

func (store *Store) Clear(_ context.Context, workspacePath string) error {
	root := store.workspaceRoot(workspacePath)
	if err := store.validateRoot(root); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clear Workspace: %w", err)
	}
	return os.MkdirAll(root, 0o700)
}

type GitCloneOptions struct {
	RepositoryURL string
	Branch        string
	Username      string
	Password      []byte
	PrivateKey    []byte
	Config        []domain.GitConfigEntry
	SSHConfig     string
}

func (store *Store) Clone(ctx context.Context, workspacePath string, options GitCloneOptions) error {
	if err := domain.ValidateGitConfig(options.Config); err != nil {
		return err
	}
	if err := domain.ValidateSSHConfig(options.SSHConfig); err != nil {
		return err
	}
	var sshHome, sshConfigPath, keyPath string
	var askPassPath string
	if len(options.PrivateKey) > 0 {
		var cleanup func()
		var err error
		sshHome, sshConfigPath, keyPath, cleanup, err = store.writeSSHFiles(options.PrivateKey, options.SSHConfig)
		if err != nil {
			return err
		}
		defer cleanup()
	}
	if len(options.Password) > 0 {
		path, cleanup, err := writeAskPass()
		if err != nil {
			return err
		}
		defer cleanup()
		askPassPath = path
	}
	return store.clone(ctx, workspacePath, options, sshHome, sshConfigPath, keyPath, askPassPath)
}

func (store *Store) writeSSHFiles(privateKey []byte, config string) (string, string, string, func(), error) {
	if len(privateKey) == 0 || len(privateKey) > 64*1024 {
		return "", "", "", func() {}, fmt.Errorf("private SSH key must contain 1-65536 bytes")
	}
	if !filepath.IsAbs(store.knownHosts) {
		return "", "", "", func() {}, fmt.Errorf("known_hosts must be configured as an absolute path")
	}
	knownHosts, err := os.Stat(store.knownHosts)
	if err != nil || !knownHosts.Mode().IsRegular() || knownHosts.Size() == 0 {
		return "", "", "", func() {}, fmt.Errorf("configured known_hosts is unavailable")
	}
	identity, err := domain.SSHConfigIdentityFile(config)
	if err != nil {
		return "", "", "", func() {}, err
	}
	home, err := os.MkdirTemp("", "agent-workspace-git-ssh-*")
	if err != nil {
		return "", "", "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(home) }
	sshDirectory := filepath.Join(home, ".ssh")
	if err := os.Mkdir(sshDirectory, 0o700); err != nil {
		cleanup()
		return "", "", "", func() {}, err
	}
	configPath := filepath.Join(sshDirectory, "config")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		cleanup()
		return "", "", "", func() {}, err
	}
	keyPath := filepath.Join(sshDirectory, identity)
	if err := os.WriteFile(keyPath, privateKey, 0o600); err != nil {
		cleanup()
		return "", "", "", func() {}, err
	}
	return home, configPath, keyPath, cleanup, nil
}

func writeAskPass() (string, func(), error) {
	file, err := os.CreateTemp("", "agent-workspace-git-askpass-*")
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	const script = "#!/bin/sh\ncase \"$1\" in *Username*) printf '%s\\n' \"$AGENT_GIT_USERNAME\" ;; *) printf '%s\\n' \"$AGENT_GIT_PASSWORD\" ;; esac\n"
	if err := file.Chmod(0o700); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, err
	}
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

func (store *Store) clone(ctx context.Context, workspacePath string, options GitCloneOptions, sshHome, sshConfigPath, keyPath, askPassPath string) error {
	root := store.workspaceRoot(workspacePath)
	if err := store.validateRoot(root); err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("Git clone requires an empty Workspace")
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
		return err
	}
	args := make([]string, 0, 8+len(options.Config)*2)
	for _, entry := range options.Config {
		args = append(args, "-c", strings.TrimSpace(entry.Key)+"="+entry.Value)
	}
	args = append(args, "clone", "--depth", "1", "--branch", options.Branch, "--", options.RepositoryURL, root)
	command := exec.CommandContext(ctx, "git", args...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if askPassPath != "" {
		command.Env = append(command.Env, "GIT_ASKPASS="+askPassPath, "AGENT_GIT_USERNAME="+options.Username, "AGENT_GIT_PASSWORD="+string(options.Password))
	}
	if keyPath != "" {
		sshCommand := strings.Join([]string{
			"ssh", "-F", shellQuote(sshConfigPath),
			"-o", shellQuote("BatchMode=yes"),
			"-o", shellQuote("IdentitiesOnly=yes"),
			"-o", shellQuote("StrictHostKeyChecking=yes"),
			"-o", shellQuote("UserKnownHostsFile=" + store.knownHosts),
			"-i", shellQuote(keyPath),
		}, " ")
		command.Env = replaceEnvironment(command.Env, "HOME", sshHome)
		command.Env = replaceEnvironment(command.Env, "GIT_SSH_COMMAND", sshCommand)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(root)
		return fmt.Errorf("clone Git repository: %w: %s", err, strings.TrimSpace(string(output)))
	}
	used, err := directorySize(root)
	if err != nil {
		return err
	}
	if used > WorkspaceLimit {
		_ = os.RemoveAll(root)
		return fmt.Errorf("cloned repository exceeds 1 GiB")
	}
	for _, entry := range options.Config {
		configCommand := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", "--", strings.TrimSpace(entry.Key), entry.Value)
		if output, err := configCommand.CombinedOutput(); err != nil {
			_ = os.RemoveAll(root)
			return fmt.Errorf("configure Git repository: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, prefix+value)
}

func (store *Store) resolve(workspacePath, relative string, allowRoot bool) (string, error) {
	root := store.workspaceRoot(workspacePath)
	if err := store.validateRoot(root); err != nil {
		return "", err
	}
	if strings.TrimSpace(relative) == "" && allowRoot {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return "", err
		}
		return root, nil
	}
	clean, err := domain.ValidateWorkspacePath(relative)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(clean))
	if target == root || !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", fmt.Errorf("Workspace path escapes its root")
	}
	if err := rejectSymlinkPath(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func rejectSymlinkPath(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Workspace path escapes its root")
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Workspace symlinks are not allowed")
		}
	}
	return nil
}

func (store *Store) workspaceRoot(workspacePath string) string {
	return filepath.Join(store.root, filepath.FromSlash(workspacePath))
}

func (store *Store) validateRoot(root string) error {
	root = filepath.Clean(root)
	if root == store.root || !strings.HasPrefix(root, store.root+string(filepath.Separator)) {
		return fmt.Errorf("invalid Workspace root")
	}
	return nil
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}
