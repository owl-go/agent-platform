package defaultresources

import (
	"context"
	"encoding/json"
	"testing"

	"agent-platform/backend/internal/expertpackage"
	"agent-platform/backend/internal/resourceaction"
	"agent-platform/backend/internal/skillstore"
)

func TestShippedCatalogPackagesAndBindings(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) < 9 || len(catalog.Experts) < 8 || len(catalog.Connectors) < 20 {
		t.Fatalf("unexpected starter inventory: Skills=%d Experts=%d Connectors=%d", len(catalog.Skills), len(catalog.Experts), len(catalog.Connectors))
	}
	for _, s := range catalog.Skills {
		archive, err := catalog.Archive(s.Archive)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err = skillstore.ValidateUpload(context.Background(), archive); err != nil {
			t.Fatalf("Skill %s: %v", s.Key, err)
		}
	}
	cli := 0
	for _, c := range catalog.Connectors {
		pkg, err := catalog.ParseConnector(c)
		if err != nil {
			t.Fatal(err)
		}
		if pkg.CLI != nil {
			cli++
			if len(pkg.CLIBundle) == 0 {
				t.Fatalf("CLI %s has no immutable bundle", c.Source)
			}
		}
	}
	if cli < 11 {
		t.Fatalf("CLI package count=%d", cli)
	}
}

func TestShippedExpertsRoundTripIntoCurrentCreationProposals(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, definition := range catalog.Experts {
		t.Run(definition.Key, func(t *testing.T) {
			archive, err := catalog.Archive(definition.Archive)
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := expertpackage.Parse(context.Background(), archive)
			if err != nil {
				t.Fatal(err)
			}
			if pkg.Manifest.Version != "1.3.0" || len(pkg.Expert.StarterPrompts) != 3 || pkg.Expert.Guidance != definition.Guidance {
				t.Fatal("shipped profile did not retain its version, common tasks and Markdown guidance")
			}
			if pkg.Manifest.Expert.Translations == nil || len(pkg.Manifest.Expert.Translations.English.StarterPrompts) != 3 {
				t.Fatal("shipped profile lacks bilingual display content")
			}
			body, err := json.Marshal(map[string]any{"kind": "expert", "expert": map[string]any{
				"name": pkg.Expert.Name, "introduction": pkg.Expert.Introduction,
				"guidance": pkg.Expert.Guidance, "starter_prompts": pkg.Expert.StarterPrompts,
				"skill_ids": []string{}, "mcp_server_ids": []string{}, "cli_connector_definition_ids": []string{},
			}})
			if err != nil {
				t.Fatal(err)
			}
			proposal, _, marked, err := resourceaction.Parse("<platform-action>" + string(body) + "</platform-action>")
			if err != nil || !marked || proposal.Kind != resourceaction.ExpertKind {
				t.Fatalf("catalog Expert cannot be proposed through the current creation contract: %v", err)
			}
		})
	}
}

func TestArchiveRejectsOutsideCatalog(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../manifest.json", "manifest.json", "/etc/passwd", "skills/../../manifest.json"} {
		if _, err := catalog.Archive(name); err == nil {
			t.Fatalf("unsafe archive accepted: %s", name)
		}
	}
}

func TestArchiveChecksumMismatchFails(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.verifyArchive(catalog.Skills[0].Archive, "invalid"); err == nil {
		t.Fatal("mismatched immutable archive accepted")
	}
}
