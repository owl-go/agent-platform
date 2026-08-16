package conformanceartifact

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveAndRestoreDirectory(t *testing.T) {
	source := t.TempDir()
	if err := os.Mkdir(filepath.Join(source, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "scripts", "test.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("scripts/test.sh", filepath.Join(source, "test-link")); err != nil {
		t.Fatal(err)
	}

	var archive bytes.Buffer
	metadata, err := Archive(source, &archive)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Size != int64(archive.Len()) || len(metadata.SHA256) != 64 || metadata.Files != 2 {
		t.Fatalf("metadata = %+v", metadata)
	}

	target := filepath.Join(t.TempDir(), "restored")
	if err := Restore(bytes.NewReader(archive.Bytes()), target); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(target, "scripts", "test.sh"))
	if err != nil || string(contents) != "#!/bin/sh\necho ok\n" {
		t.Fatalf("restored file = %q, err = %v", contents, err)
	}
	info, err := os.Stat(filepath.Join(target, "scripts", "test.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("restored mode = %v, err = %v", info.Mode(), err)
	}
	link, err := os.Readlink(filepath.Join(target, "test-link"))
	if err != nil || link != "scripts/test.sh" {
		t.Fatalf("restored link = %q, err = %v", link, err)
	}
}

func TestArchiveRejectsSymlinkOutsideSource(t *testing.T) {
	source := t.TempDir()
	if err := os.Symlink("../secret", filepath.Join(source, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := Archive(source, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escaping symlink rejection, got %v", err)
	}
}

func TestRestoreRejectsTraversalAndNonEmptyTarget(t *testing.T) {
	malicious := func(name, link string, typeflag byte) []byte {
		var buffer bytes.Buffer
		writer := tar.NewWriter(&buffer)
		if err := writer.WriteHeader(&tar.Header{Name: name, Linkname: link, Typeflag: typeflag, Mode: 0o644}); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	for name, archive := range map[string][]byte{
		"parent traversal": malicious("../escape", "", tar.TypeReg),
		"absolute path":    malicious("/escape", "", tar.TypeReg),
		"escaping symlink": malicious("link", "../escape", tar.TypeSymlink),
	} {
		t.Run(name, func(t *testing.T) {
			if err := Restore(bytes.NewReader(archive), filepath.Join(t.TempDir(), "target")); err == nil {
				t.Fatal("expected malicious archive rejection")
			}
		})
	}

	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "existing"), []byte("value"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(bytes.NewReader(malicious("file", "", tar.TypeReg)), target); err == nil {
		t.Fatal("expected non-empty target rejection")
	}
}
