package gormrepo

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/defaultresources"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"github.com/google/uuid"
)

func defaultResourceRepositoryFixture(t *testing.T) (*Repository, string) {
	t.Helper()
	db := conversationTestDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name,administrator,bootstrap_administrator) VALUES(?,?,?,?,?,true,true)", owner, owner, "fixture-admin", "fixture-admin@example.test", "Fixture Administrator").Error; err != nil {
		t.Fatal(err)
	}
	return New(db, nil), owner
}

func defaultResourceFixture(t *testing.T) (*Repository, string, defaultresources.Catalog) {
	t.Helper()
	repo, owner := defaultResourceRepositoryFixture(t)
	source := os.Getenv(defaultresources.RootEnvironment)
	if source == "" {
		var err error
		source, err = os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := os.Stat(filepath.Join(source, "resources.json")); err == nil {
				break
			}
			parent := filepath.Dir(source)
			if parent == source {
				t.Fatal("resource fixture root missing")
			}
			source = parent
		}
	}
	// These repository tests exercise initialization transactions and bindings.
	// Load their real, small directory packages instead of repeatedly assembling
	// every CLI bundle before discarding all but these three definitions.
	root := t.TempDir()
	marker, err := os.ReadFile(filepath.Join(source, "resources.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources.json"), marker, 0644); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"skills/travel-planning", "experts/travel-planning", "connectors/ai-hive"} {
		if err := os.CopyFS(filepath.Join(root, directory), os.DirFS(filepath.Join(source, directory))); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := defaultresources.LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 1 || len(catalog.Experts) != 1 || len(catalog.Connectors) != 1 {
		t.Fatal("transaction fixture must contain exactly one bound Expert, Skill and MCP package")
	}
	return repo, owner, catalog
}

func TestDefaultResourcesConcurrentInstallAndUpgrade(t *testing.T) {
	repo, owner, catalog := defaultResourceFixture(t)
	objects := memory.New()
	ctx := context.Background()
	results := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- repo.ensureResources(ctx, objects, catalog) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	for table, want := range map[string]int64{"skills": 4, "experts": 1, "connector_package_publications": 1, "default_resource_seeds": 6} {
		var n int64
		if err := repo.db.Table(table).Count(&n).Error; err != nil || n != want {
			t.Fatalf("%s count=%d want=%d: %v", table, n, want, err)
		}
	}
	var before expertRecord
	if err := repo.db.Where("system_key = ?", catalog.Experts[0].Key).Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	var skill skillRecord
	if err := repo.db.Where("system_key = ?", catalog.Skills[0].Key).Take(&skill).Error; err != nil {
		t.Fatal(err)
	}
	catalog.Experts[0].Version = "1.0.1"
	catalog.Experts[0].Introduction += " Updated fixture."
	if err := repo.ensureResources(ctx, objects, catalog); err != nil {
		t.Fatal(err)
	}
	var after expertRecord
	if err := repo.db.Where("system_key = ?", catalog.Experts[0].Key).Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.OwnerID != owner || after.Version != before.Version+1 || after.Introduction == before.Introduction {
		t.Fatal("upgrade must retain identity and update the managed definition exactly once")
	}
	if err := repo.ensureResources(ctx, objects, catalog); err != nil {
		t.Fatal(err)
	}
	var repeated expertRecord
	repo.db.Where("id = ?", after.ID).Take(&repeated)
	if repeated.Version != after.Version {
		t.Fatal("repeat startup changed Expert version")
	}
	catalog.Experts[0].Introduction += " Unversioned change."
	if err := repo.ensureResources(ctx, objects, catalog); err == nil {
		t.Fatal("changed content at the same catalog version must fail")
	}
}

func TestDefaultResourcesPreserveExistingAndPrivateResources(t *testing.T) {
	repo, owner, catalog := defaultResourceFixture(t)
	ctx := context.Background()
	objects := memory.New()
	privateOwner := uuid.NewString()
	if err := repo.db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", privateOwner, privateOwner, privateOwner, "private@example.test", "Private User").Error; err != nil {
		t.Fatal(err)
	}
	original := catalog.Experts[0]
	collision := expertRecord{ID: uuid.NewString(), OwnerID: owner, Name: original.Name, NameNormalized: normalizeResourceName(original.Name), Icon: "sparkles", Introduction: "Administrator custom introduction", SkillIDs: []byte("[]"), MCPServerIDs: []byte("[]"), CLIConnectorDefinitionIDs: []byte("[]"), ExpertiseTags: []byte("[]"), TagProjectionStatus: "idle", Version: 1}
	if err := repo.db.Omit("SystemKey").Create(&collision).Error; err != nil {
		t.Fatal(err)
	}
	private := collision
	private.ID = uuid.NewString()
	private.OwnerID = privateOwner
	private.Introduction = "Private introduction"
	if err := repo.db.Omit("SystemKey").Create(&private).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ensureResources(ctx, objects, catalog); err != nil {
		t.Fatal(err)
	}
	catalog.Experts[0].Version = "2.0.0"
	catalog.Experts[0].Introduction = "Replacement default"
	if err := repo.ensureResources(ctx, objects, catalog); err != nil {
		t.Fatal(err)
	}
	for _, want := range []expertRecord{collision, private} {
		var actual expertRecord
		if err := repo.db.Where("id = ?", want.ID).Take(&actual).Error; err != nil {
			t.Fatal(err)
		}
		if actual.Introduction != want.Introduction || actual.Version != want.Version || actual.SystemManaged {
			t.Fatal("initializer overwrote an existing or private Expert")
		}
	}
	var n int64
	repo.db.Model(&expertRecord{}).Count(&n)
	if n != 2 {
		t.Fatalf("existing-name collision created duplicate Expert: %d", n)
	}
}

type failingDefaultObjects struct{ objectstore.Provider }

func (failingDefaultObjects) Put(context.Context, string, io.Reader, objectstore.PutOptions) (objectstore.Object, error) {
	return objectstore.Object{}, errors.New("fixture object failure")
}

func TestDefaultResourceFailureRollsBackCatalog(t *testing.T) {
	repo, _, catalog := defaultResourceFixture(t)
	if err := repo.ensureResources(context.Background(), failingDefaultObjects{memory.New()}, catalog); err == nil {
		t.Fatal("Object Store failure must fail initialization")
	}
	for _, table := range []string{"skills", "experts", "connector_package_publications", "default_resource_seeds"} {
		var n int64
		repo.db.Table(table).Count(&n)
		if n != 0 {
			t.Fatalf("failed initializer persisted %s: %d", table, n)
		}
	}
	if err := repo.ensureResources(context.Background(), memory.New(), catalog); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
}

func TestDefaultCatalogFreshInstallDoesNotAuthorizeOrVerifyCLI(t *testing.T) {
	repo, _ := defaultResourceRepositoryFixture(t)
	catalog, err := defaultresources.Load()
	if err != nil {
		t.Fatal(err)
	}
	objects := memory.New()
	// Exercise the full installer with the already validated catalog. Public
	// directory discovery and repeated startup are covered by the directory test.
	if err := repo.ensureResources(context.Background(), objects, catalog); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int64{"skills": int64(len(catalog.Skills) + 3), "experts": int64(len(catalog.Experts)), "connector_package_publications": int64(len(catalog.Connectors)), "connector_installations": 0, "connector_authorizations": 0, "cli_connector_conformance": 0} {
		var n int64
		if err := repo.db.Table(table).Count(&n).Error; err != nil || n != want {
			t.Fatalf("%s count=%d want=%d: %v", table, n, want, err)
		}
	}
	var n int64
	if err := repo.db.Table("connector_package_publications p").Joins("JOIN connector_revisions r ON r.id = p.active_revision_id").Where("r.mode = 'cli' AND p.state = 'available'").Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("unverified CLI published available: %d, %v", n, err)
	}
	var first expertRecord
	repo.db.Where("system_key = ?", "default.expert.travel-planning").Take(&first)
	if len(first.SkillIDs) == 0 {
		t.Fatal("default Expert lost Skill binding")
	}
}

func TestDefaultConnectorDoesNotPublishPrivateVersionCollision(t *testing.T) {
	repo, _, catalog := defaultResourceFixture(t)
	ctx := context.Background()
	definition := catalog.Connectors[0]
	private, err := repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: definition.Source, Version: definition.Version, Mode: domain.ConnectorModeMCP, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/private/fixture.zip", RuntimePolicy: []byte(`{"auth_mode":"none","metadata":{"name":"Private fixture"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = repo.ensureResources(ctx, memory.New(), catalog); err != nil {
			t.Fatal(err)
		}
	}
	var publications int64
	repo.db.Model(&connectorPublicationRecord{}).Where("package_source = ?", definition.Source).Count(&publications)
	if publications != 0 {
		t.Fatal("private package collision was promoted to the platform catalog")
	}
	retained, err := repo.GetConnectorRevision(ctx, private.ID)
	if err != nil || retained.PackageSHA256 != private.PackageSHA256 {
		t.Fatalf("private revision changed: %v", err)
	}
}

func TestDefaultSkillMetadataUpgradeRequiresVersionAndRetainsBinding(t *testing.T) {
	repo, _, catalog := defaultResourceFixture(t)
	objects := memory.New()
	ctx := context.Background()
	if err := repo.ensureResources(ctx, objects, catalog); err != nil {
		t.Fatal(err)
	}
	var before skillRecord
	if err := repo.db.Where("system_key = ?", catalog.Skills[0].Key).Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	catalog.Skills[0].Icon = "file-text"
	if err := repo.ensureResources(ctx, objects, catalog); err == nil {
		t.Fatal("unversioned Skill metadata change accepted")
	}
	catalog.Skills[0].Version = "1.0.1"
	for i := 0; i < 2; i++ {
		if err := repo.ensureResources(ctx, objects, catalog); err != nil {
			t.Fatal(err)
		}
	}
	var after skillRecord
	if err := repo.db.Where("id = ?", before.ID).Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Version != before.Version+1 || after.Icon != "file-text" || after.ObjectKey != before.ObjectKey {
		t.Fatal("metadata upgrade changed identity/object or incremented version more than once")
	}
	var expert expertRecord
	if err := repo.db.Where("system_key = ?", catalog.Experts[0].Key).Take(&expert).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(expert.SkillIDs), before.ID) {
		t.Fatal("Skill upgrade lost Expert binding")
	}
}

func TestDefaultResourcesDoNotTakeOverExistingSkillOrPublication(t *testing.T) {
	repo, owner, catalog := defaultResourceFixture(t)
	ctx := context.Background()
	custom := skillRecord{ID: uuid.NewString(), OwnerID: owner, Name: catalog.Skills[0].Name, NameNormalized: normalizeResourceName(catalog.Skills[0].Name), Icon: "file-text", Source: "upload", ObjectKey: "skills/private/fixture.zip", SHA256: strings.Repeat("b", 64), Version: 1}
	if err := repo.db.Omit("SystemKey").Create(&custom).Error; err != nil {
		t.Fatal(err)
	}
	pkg, err := catalog.ParseConnector(catalog.Connectors[0])
	if err != nil {
		t.Fatal(err)
	}
	policy, err := pkg.RuntimePolicy()
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: pkg.Metadata.Source, Version: pkg.Metadata.Version, Mode: domain.ConnectorMode(pkg.Metadata.Type), PackageSHA256: pkg.SHA256, ObjectKey: pkg.ObjectKey(), RuntimePolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	publication := connectorPublicationRecord{PackageSource: pkg.Metadata.Source, ActiveRevisionID: revision.ID, State: string(domain.ConnectorPublicationDisabled), AdministratorID: owner, Version: 7}
	if err := repo.db.Create(&publication).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.ensureResources(ctx, memory.New(), catalog); err != nil {
			t.Fatal(err)
		}
	}
	var retained skillRecord
	if err := repo.db.Where("id = ?", custom.ID).Take(&retained).Error; err != nil {
		t.Fatal(err)
	}
	if retained.SystemManaged || retained.SHA256 != custom.SHA256 || retained.Version != 1 {
		t.Fatal("existing Skill was taken over")
	}
	var current connectorPublicationRecord
	if err := repo.db.Where("package_source = ?", publication.PackageSource).Take(&current).Error; err != nil {
		t.Fatal(err)
	}
	if current.ActiveRevisionID != publication.ActiveRevisionID || current.State != publication.State || current.Version != 7 {
		t.Fatal("existing publication changed")
	}
	var expert expertRecord
	if err := repo.db.Where("system_key = ?", catalog.Experts[0].Key).Take(&expert).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(expert.SkillIDs), custom.ID) {
		t.Fatal("default Expert did not bind retained Skill")
	}
	if err := repo.db.Where("id = ?", custom.ID).Delete(&skillRecord{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ensureResources(ctx, memory.New(), catalog); err == nil {
		t.Fatal("missing retained Skill produced a dangling binding")
	}
}

// Install a directory catalog, add entries without a central manifest edit,
// and verify discovery, stable bindings, repeat startup and prior publication.
func TestLocalResourceDirectoryDiscoveryAndPriorScriptPublication(t *testing.T) {
	repo, owner, catalog := defaultResourceFixture(t)
	root := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	extract := func(directory string, archive []byte) {
		t.Helper()
		reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range reader.File {
			content, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(content)
			content.Close()
			if err != nil {
				t.Fatal(err)
			}
			write(directory+"/"+entry.Name, data)
		}
	}
	write("resources.json", []byte(`{"version":"1.0.0"}`))
	archive, err := catalog.Archive(catalog.Skills[0].Archive)
	if err != nil {
		t.Fatal(err)
	}
	extract("skills/travel-planning", archive)
	write("skills/travel-planning/resource.json", []byte(`{"key":"default.skill.travel-planning","version":"1.0.0","icon":"compass"}`))
	expert, err := json.Marshal(catalog.Experts[0])
	if err != nil {
		t.Fatal(err)
	}
	write("experts/travel-planning/expert.json", expert)
	pkg, err := catalog.ParseConnector(catalog.Connectors[0])
	if err != nil {
		t.Fatal(err)
	}
	extract("connectors/ai-hive/package", pkg.NormalizedArchive)
	// A previous publisher already created this exact Revision/Publication.
	policy, err := pkg.RuntimePolicy()
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repo.CreateConnectorRevision(context.Background(), domain.ConnectorRevision{PackageSource: pkg.Metadata.Source, Version: pkg.Metadata.Version, Mode: domain.ConnectorModeMCP, PackageSHA256: pkg.SHA256, ObjectKey: pkg.ObjectKey(), RuntimePolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	original := connectorPublicationRecord{PackageSource: pkg.Metadata.Source, ActiveRevisionID: revision.ID, State: string(domain.ConnectorPublicationDisabled), AdministratorID: owner, Version: 7}
	if err := repo.db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv(defaultresources.RootEnvironment, root)
	objects := memory.New()
	if err := repo.EnsureDefaultResources(context.Background(), objects); err != nil {
		t.Fatal(err)
	}
	var before expertRecord
	if err := repo.db.Where("system_key = ?", catalog.Experts[0].Key).Take(&before).Error; err != nil {
		t.Fatal(err)
	}
	// Neither resources.json nor the old script directory needs an inventory edit.
	write("skills/directory-only/resource.json", []byte(`{"key":"local.skill.directory-only","version":"1.0.0"}`))
	write("skills/directory-only/SKILL.md", []byte("---\nname: directory-only\ndisplay_name: Directory Only\ndescription: Explain the supplied fixture.\n---\n\n# Fixture\nExplain the result.\n"))
	added := catalog.Experts[0]
	added.Key = "local.expert.directory-only"
	added.Name = "Directory Only Expert"
	added.SkillKeys = []string{"local.skill.directory-only"}
	addedData, err := json.Marshal(added)
	if err != nil {
		t.Fatal(err)
	}
	write("experts/directory-only/expert.json", addedData)
	// Script source files are outside discovery, even with the same source.
	write("scripts/connectors/ai-hive/connector-meta.json", []byte(`{"source":"ai-hive","version":"different"}`))
	for i := 0; i < 2; i++ {
		if err := repo.EnsureDefaultResources(context.Background(), objects); err != nil {
			t.Fatal(err)
		}
	}
	for table, want := range map[string]int64{"skills": 5, "experts": 2, "connector_package_publications": 1, "connector_revisions": 1, "connector_installations": 0, "connector_authorizations": 0} {
		var count int64
		if err := repo.db.Table(table).Count(&count).Error; err != nil || count != want {
			t.Fatalf("%s count=%d want=%d: %v", table, count, want, err)
		}
	}
	var publication connectorPublicationRecord
	if err := repo.db.Where("package_source = ?", original.PackageSource).Take(&publication).Error; err != nil {
		t.Fatal(err)
	}
	if publication.ActiveRevisionID != original.ActiveRevisionID || publication.State != original.State || publication.Version != original.Version {
		t.Fatal("directory import changed the prior script publication")
	}
	var after expertRecord
	if err := repo.db.Where("id = ?", before.ID).Take(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.Version != before.Version || !bytes.Equal(after.SkillIDs, before.SkillIDs) {
		t.Fatal("repeat discovery changed existing Expert identity or binding")
	}
	var newSkill skillRecord
	if err := repo.db.Where("system_key = ?", "local.skill.directory-only").Take(&newSkill).Error; err != nil {
		t.Fatal(err)
	}
	var newExpert expertRecord
	if err := repo.db.Where("system_key = ?", added.Key).Take(&newExpert).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(newExpert.SkillIDs), newSkill.ID) {
		t.Fatal("new Expert binding was not resolved by stable Skill key")
	}
}
