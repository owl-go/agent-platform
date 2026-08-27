package skillstore

import (
	"context"
	"testing"
)

func TestInstallGitRejectsCredentialsAndURLMetadataBeforeClone(t *testing.T) {
	store := &Store{}
	for _, repositoryURL := range []string{
		"https://user:secret@example.test/skill.git",
		"https://example.test/skill.git?token=secret",
		"https://example.test/skill.git#main",
		"http://example.test/skill.git",
	} {
		if _, _, _, err := store.InstallGit(context.Background(), "owner", repositoryURL, "main"); err == nil {
			t.Fatalf("InstallGit accepted unsafe URL %q", repositoryURL)
		}
	}
}
