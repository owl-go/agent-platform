package gormrepo

import (
	"context"
	"testing"

	"agent-platform/backend/internal/expertpackage"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestManagedDirectoryExportsRetainBilingualSourcePackage(t *testing.T) {
	repo, owner, catalog := defaultResourceFixture(t)
	ctx := context.Background()
	objects := memory.New()
	for i := 0; i < 2; i++ {
		if err := repo.ensureResources(ctx, objects, catalog); err != nil {
			t.Fatal(err)
		}
	}
	experts, err := repo.ListExperts(ctx, owner)
	if err != nil || len(experts) != 1 {
		t.Fatalf("inventory err=%v", err)
	}
	archive, err := repo.ImportedExpertPackage(ctx, owner, "expert", experts[0].ID, experts[0].Version)
	if err != nil || len(archive) == 0 {
		t.Fatalf("source archive missing: %v", err)
	}
	pkg, err := expertpackage.Parse(ctx, archive)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.Expert.Translations == nil || len(pkg.Manifest.Expert.Translations.English.StarterPrompts) != 3 {
		t.Fatal("bilingual source metadata lost")
	}
	original, err := catalog.Archive(catalog.Experts[0].Archive)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := expertpackage.Parse(ctx, original)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.SHA256 != expected.SHA256 {
		t.Fatal("source package changed during persistence")
	}
	if err := repo.db.Exec("UPDATE expert_package_imports SET content_sha256=repeat('f',64) WHERE package_id=?", pkg.Manifest.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ensureResources(ctx, objects, catalog); err == nil {
		t.Fatal("conflicting source archive accepted")
	}
	current, err := repo.GetExpert(ctx, owner, experts[0].ID)
	if err != nil || current.Version != experts[0].Version {
		t.Fatal("failed initialization changed the managed definition")
	}
}
