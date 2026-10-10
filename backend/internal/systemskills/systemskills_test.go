package systemskills

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"regexp"
	"testing"

	"agent-platform/backend/internal/resourceaction"
)

func TestDefinitionsAreStablePlatformSkills(t *testing.T) {
	definitions := Definitions()
	if len(definitions) != 3 {
		t.Fatalf("Definitions() returned %d entries, want 3", len(definitions))
	}
	for _, definition := range definitions {
		if definition.Name != "Create Skill" && definition.Name != "Create Expert" && definition.Name != "Create Connector" {
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

func TestCreateConnectorExampleIsAValidAction(t *testing.T) {
	definition, ok := DefinitionByKey(CreateConnectorKey)
	if !ok {
		t.Fatal("Create Connector Skill is missing")
	}
	proposal, _, marked, err := resourceaction.Parse(definition.Document)
	if err != nil || !marked || proposal.Kind != resourceaction.ConnectorKind {
		t.Fatalf("example proposal is invalid: marked=%v proposal=%+v err=%v", marked, proposal, err)
	}
}

func TestCreateExpertExamplesUseCurrentProfileContract(t *testing.T) {
	definition, ok := DefinitionByKey(CreateExpertKey)
	if !ok {
		t.Fatal("Create Expert Skill is missing")
	}
	markers := regexp.MustCompile(`(?s)<platform-action>(\{.*?\})</platform-action>`).FindAllString(definition.Document, -1)
	if len(markers) != 2 {
		t.Fatalf("want Expert and Team examples, got %d", len(markers))
	}
	for index, marker := range markers {
		proposal, _, marked, err := resourceaction.Parse(marker)
		if err != nil || !marked {
			t.Fatalf("example %d invalid: %v", index, err)
		}
		if index == 0 && proposal.Kind != resourceaction.ExpertKind || index == 1 && proposal.Kind != resourceaction.TeamKind {
			t.Fatalf("unexpected kind %q", proposal.Kind)
		}
	}
}
