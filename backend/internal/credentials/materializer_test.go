package credentials_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"agent-platform/backend/internal/credentials"
)

func TestMaterializerCreatesPrivateCredentialFilesAndCleansUp(t *testing.T) {
	materializer := credentials.Materializer{
		Root:  t.TempDir(),
		Owner: &credentials.Owner{UID: os.Getuid(), GID: os.Getgid()},
	}
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
	credentialStat := info.Sys().(*syscall.Stat_t)
	if int(credentialStat.Uid) != os.Getuid() || int(credentialStat.Gid) != os.Getgid() {
		t.Fatalf("credential owner = %d:%d, want %d:%d", credentialStat.Uid, credentialStat.Gid, os.Getuid(), os.Getgid())
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

func TestMaterializerReplacesWarmContainerCredentialDirectory(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, ".runtime-containers", "slot", "credentials")
	materializer := credentials.Materializer{Root: root}
	first, err := materializer.CreateAt(credentials.Request{Ref: "run-1", Variables: map[string]string{"TOKEN": "first"}}, directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Cleanup(); err != nil {
		t.Fatal(err)
	}
	second, err := materializer.CreateAt(credentials.Request{Ref: "run-2", Variables: map[string]string{"TOKEN": "second"}}, directory)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Cleanup()
	contents, err := os.ReadFile(filepath.Join(directory, "env", "TOKEN"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "second" {
		t.Fatalf("warm credential value = %q", contents)
	}
	if _, err := materializer.CreateAt(credentials.Request{Ref: "escape"}, filepath.Join(root, "..", "escape")); err == nil {
		t.Fatal("CreateAt accepted a credential directory outside its root")
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
	credentialValue, err := os.ReadFile(filepath.Join(environment.Directory(), "env", "MODEL_API_KEY"))
	if err != nil {
		t.Fatalf("read materialized environment credential: %v", err)
	}
	if string(credentialValue) != "model-secret" {
		t.Fatalf("materialized environment credential = %q", credentialValue)
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
