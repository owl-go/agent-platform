package systemskills

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestDefinitionsAreStablePlatformSkills(t *testing.T) {
	definitions := Definitions()
	if len(definitions) != 2 {
		t.Fatalf("Definitions() returned %d entries, want 2", len(definitions))
	}
	for _, definition := range definitions {
		if definition.Name != "Create Skill" && definition.Name != "Create Expert" {
			t.Fatalf("unexpected default Skill name %q", definition.Name)
		}
		archiveBytes, digest, err := Archive(definition.Key)
		if err != nil {
			t.Fatalf("Archive(%q): %v", definition.Key, err)
		}
		sum := sha256.Sum256(archiveBytes)
		if digest != hex.EncodeToString(sum[:]) {
			t.Fatalf("Archive(%q) returned an invalid digest", definition.Key)
		}
		reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
		if err != nil || len(reader.File) != 1 || reader.File[0].Name != "SKILL.md" {
			t.Fatalf("Archive(%q) does not contain exactly SKILL.md: %v", definition.Key, err)
		}
		file, err := reader.File[0].Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil || len(content) == 0 {
			t.Fatalf("Archive(%q) contains an empty SKILL.md: %v", definition.Key, err)
		}
	}
}
