package gormrepo

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/defaultresources"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"github.com/google/uuid"
)

func defaultResourceFixture(t *testing.T) (*Repository, string, defaultresources.Catalog) {
	t.Helper()
	db := conversationTestDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name,administrator,bootstrap_administrator) VALUES(?,?,?,?,?,true,true)", owner, owner, "fixture-admin", "fixture-admin@example.test", "Fixture Administrator").Error; err != nil {
		t.Fatal(err)
	}
	catalog, err := defaultresources.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) == 0 || len(catalog.Experts) == 0 || len(catalog.Connectors) == 0 {
		t.Fatal("starter catalog must include Skills, Experts and Connectors")
	}
	// Keep concurrency tests focused while the asset contract validates every package.
	catalog.Skills = catalog.Skills[:1]
	catalog.Experts = catalog.Experts[:1]
	catalog.Connectors = catalog.Connectors[:1]
	return New(db, nil), owner, catalog
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
	repo, _, _ := defaultResourceFixture(t)
	objects := memory.New()
	if err := repo.EnsureDefaultResources(context.Background(), objects); err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int64{"skills": 12, "experts": 8, "connector_package_publications": 20, "connector_installations": 0, "connector_authorizations": 0, "cli_connector_conformance": 0} {
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
	pkg, err := defaultresources.ParseConnector(catalog.Connectors[0])
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
