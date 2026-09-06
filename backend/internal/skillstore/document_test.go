package skillstore

import (
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestDocumentReadsExactInstalledMarkdownAndVerifiesPackage(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	body := "---\nname: example\ndescription: example\n---\n# Instructions\n\nLiteral <script> content.\n"
	if _, err = file.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	provider := memory.New()
	sum := sha256.Sum256(archive.Bytes())
	digest := hex.EncodeToString(sum[:])
	if _, err = provider.Put(context.Background(), "skills/test.zip", bytes.NewReader(archive.Bytes()), objectstore.PutOptions{Size: int64(archive.Len()), SHA256: digest}); err != nil {
		t.Fatal(err)
	}
	store := &Store{objects: provider}
	got, err := store.Document(context.Background(), "skills/test.zip", digest)
	if err != nil || got != body {
		t.Fatalf("document = %q, %v", got, err)
	}
	if _, err = store.Document(context.Background(), "skills/test.zip", strings.Repeat("0", 64)); err == nil {
		t.Fatal("accepted tampered package")
	}
}
