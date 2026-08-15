package credentials_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"agent-platform/internal/credentials"
)

func TestMaterializerCreatesPrivateCredentialFilesAndCleansUp(t *testing.T) {
	materializer := credentials.Materializer{Root: t.TempDir()}
	environment, err := materializer.Create(credentials.Request{
		Ref: "run-1",
		Files: map[string][]byte{
			"git/id_ed25519": []byte("private-key-value"),
		},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	credentialPath := filepath.Join(environment.Directory(), "git", "id_ed25519")
	info, err := os.Stat(credentialPath)
	if err != nil {
		t.Fatalf("stat credential: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("credential mode: got %o, want 600", got)
	}
	directoryInfo, err := os.Stat(environment.Directory())
	if err != nil {
		t.Fatalf("stat credential directory: %v", err)
	}
	if got := directoryInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("credential directory mode: got %o, want 700", got)
	}

	if err := environment.Cleanup(); err != nil {
		t.Fatalf("cleanup environment: %v", err)
	}
	if _, err := os.Stat(environment.Directory()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credential directory still exists: %v", err)
	}
	if err := environment.Cleanup(); err != nil {
		t.Fatalf("repeat cleanup: %v", err)
	}
}

func TestMaterializerRejectsCredentialPathTraversal(t *testing.T) {
	root := t.TempDir()
	materializer := credentials.Materializer{Root: root}

	_, err := materializer.Create(credentials.Request{
		Ref: "run-1",
		Files: map[string][]byte{
			"../escaped": []byte("secret"),
		},
	})
	if err == nil {
		t.Fatal("expected path traversal to be rejected")
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("read materializer root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("failed materialization left temporary entries: %v", entries)
	}
}

func TestMaterializerRejectsInvalidEnvironmentVariableName(t *testing.T) {
	materializer := credentials.Materializer{Root: t.TempDir()}

	_, err := materializer.Create(credentials.Request{
		Ref:       "run-1",
		Variables: map[string]string{"BAD-NAME": "secret"},
	})
	if err == nil {
		t.Fatal("expected invalid environment variable name to be rejected")
	}
}

func TestEnvironmentReturnsIsolatedVariables(t *testing.T) {
	materializer := credentials.Materializer{Root: t.TempDir()}
	environment, err := materializer.Create(credentials.Request{
		Ref: "run-1",
		Variables: map[string]string{
			"MODEL_API_KEY": "model-secret",
		},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	defer environment.Cleanup()

	variables := environment.Environ()
	if !slices.Contains(variables, "MODEL_API_KEY=model-secret") {
		t.Fatalf("environment variables: %v", variables)
	}
	variables[0] = "MODEL_API_KEY=changed"
	if slices.Contains(environment.Environ(), "MODEL_API_KEY=changed") {
		t.Fatal("caller changed the stored credential environment")
	}
}

func TestEnvironmentRedactorCoversSelectedVariablesAndFiles(t *testing.T) {
	materializer := credentials.Materializer{Root: t.TempDir()}
	environment, err := materializer.Create(credentials.Request{
		Ref: "run-1",
		Variables: map[string]string{
			"MODEL_API_KEY": "model-secret",
		},
		Files: map[string][]byte{
			"git/id_ed25519": []byte("private-key-value"),
		},
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	defer environment.Cleanup()

	got := string(environment.Redactor().Bytes([]byte("model-secret private-key-value unrelated")))
	want := "[REDACTED] [REDACTED] unrelated"
	if got != want {
		t.Fatalf("redacted credentials = %q, want %q", got, want)
	}
}

func TestEnvironmentRedactorDoesNotIncludeAnotherRunCredentials(t *testing.T) {
	materializer := credentials.Materializer{Root: t.TempDir()}
	first, err := materializer.Create(credentials.Request{Ref: "run-1", Variables: map[string]string{"TOKEN": "first-secret"}})
	if err != nil {
		t.Fatalf("create first environment: %v", err)
	}
	defer first.Cleanup()
	second, err := materializer.Create(credentials.Request{Ref: "run-2", Variables: map[string]string{"TOKEN": "second-secret"}})
	if err != nil {
		t.Fatalf("create second environment: %v", err)
	}
	defer second.Cleanup()

	got := string(first.Redactor().Bytes([]byte("first-secret second-secret")))
	if got != "[REDACTED] second-secret" {
		t.Fatalf("first run redaction crossed credential environments: %q", got)
	}
}
