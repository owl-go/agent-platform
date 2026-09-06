package workspace

import (
	"context"
	"errors"
	"net/http"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func gitSourceError(code string) error {
	return kratoserrors.New(http.StatusUnprocessableEntity, code, code)
}

func gitCloneError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return gitSourceError("git_connection_failed")
	}
	// Return only recognized categories: Git output can contain credentials and private paths.
	message := strings.ToLower(err.Error())
	for _, category := range []struct {
		code     string
		patterns []string
	}{
		{"git_workspace_not_empty", []string{"git clone requires an empty workspace"}},
		{"git_server_unavailable", []string{"known_hosts must be configured", "configured known_hosts is unavailable", "executable file not found"}},
		{"git_ssh_host_untrusted", []string{"host key verification failed", "remote host identification has changed"}},
		{"git_ssh_key_invalid", []string{"invalid format", "error in libcrypto", "incorrect passphrase", "private ssh key must contain"}},
		{"git_authentication_failed", []string{"permission denied (", "authentication failed", "could not read username", "terminal prompts disabled"}},
		{"git_branch_not_found", []string{"remote branch", "couldn't find remote ref"}},
		{"git_repository_unavailable", []string{"repository not found", "does not appear to be a git repository", "not found"}},
		{"git_connection_failed", []string{"could not resolve", "couldn't resolve", "connection refused", "connection timed out", "network is unreachable", "connection reset"}},
		{"git_repository_too_large", []string{"cloned repository exceeds 1 gib"}},
	} {
		for _, pattern := range category.patterns {
			if strings.Contains(message, pattern) {
				return gitSourceError(category.code)
			}
		}
	}
	return gitSourceError("git_clone_failed")
}
