package defaultresources

import (
	"context"
	"testing"

	"agent-platform/backend/internal/skillstore"
)

func TestShippedCatalogPackagesAndBindings(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 9 || len(catalog.Experts) != 8 || len(catalog.Connectors) != 20 {
		t.Fatalf("unexpected starter inventory: Skills=%d Experts=%d Connectors=%d", len(catalog.Skills), len(catalog.Experts), len(catalog.Connectors))
	}
	for _, s := range catalog.Skills {
		archive, err := Archive(s.Archive)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err = skillstore.ValidateUpload(context.Background(), archive); err != nil {
			t.Fatalf("Skill %s: %v", s.Key, err)
		}
	}
	cli := 0
	for _, c := range catalog.Connectors {
		pkg, err := ParseConnector(c)
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
	if cli != 11 {
		t.Fatalf("CLI package count=%d", cli)
	}
}

func TestArchiveRejectsOutsideCatalog(t *testing.T) {
	for _, name := range []string{"../manifest.json", "manifest.json", "/etc/passwd", "skills/../../manifest.json"} {
		if _, err := Archive(name); err == nil {
			t.Fatalf("unsafe archive accepted: %s", name)
		}
	}
}

func TestArchiveChecksumMismatchFails(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(catalog.Skills[0].Archive, "invalid"); err == nil {
		t.Fatal("mismatched immutable archive accepted")
	}
}
