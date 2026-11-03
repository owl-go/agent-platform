package gormrepo

import (
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/defaultresources"
	"agent-platform/backend/internal/objectstore/memory"
	"gorm.io/gorm"
)

func TestManagedCLIUpgradeRetainsAutomaticEnablementAndExplicitDisable(t *testing.T) {
	r, owner := defaultResourceRepositoryFixture(t)
	catalog, err := defaultresources.Load()
	if err != nil {
		t.Fatal(err)
	}
	var definition defaultresources.Connector
	for _, candidate := range catalog.Connectors {
		if candidate.Source == "github" {
			definition = candidate
		}
	}
	if definition.Source == "" {
		t.Fatal("GitHub fixture missing")
	}
	image := "example/runtime@sha256:" + strings.Repeat("b", 64)
	if err := r.ConfigureConnectorExecution("production", "runsc", []string{image}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, state   string
		pending, want bool
	}{
		{"enabled", "available", false, true},
		{"awaiting verification", "disabled", true, true},
		{"administrator disabled", "disabled", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if err := r.db.Exec("DELETE FROM default_resource_seeds; DELETE FROM connector_package_publications; DELETE FROM production_cli_connector_conformance").Error; err != nil {
				t.Fatal(err)
			}
			old, err := r.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "github", Version: "0.0.9", Mode: domain.ConnectorModeCLI, PackageSHA256: strings.Repeat("c", 64), ObjectKey: "connectors/old.zip", RuntimePolicy: []byte(`{"cli":{"executable":"gh"}}`)})
			if err != nil {
				t.Fatal(err)
			}
			if err := r.db.Create(&connectorPublicationRecord{PackageSource: "github", ActiveRevisionID: old.ID, State: tc.state, AdministratorID: owner, Version: 7, LocalActivationPending: tc.pending}).Error; err != nil {
				t.Fatal(err)
			}
			if err := saveSeed(r.db, "connector", "github", old.ID, "0.0.9", strings.Repeat("c", 64), true); err != nil {
				t.Fatal(err)
			}
			objects := memory.New()
			if err := r.db.Transaction(func(tx *gorm.DB) error { return seedConnector(ctx, tx, objects, owner, catalog, definition) }); err != nil {
				t.Fatal(err)
			}
			var publication connectorPublicationRecord
			if err := r.db.Where("package_source=?", "github").Take(&publication).Error; err != nil {
				t.Fatal(err)
			}
			if publication.ActiveRevisionID == old.ID || publication.State != "disabled" || publication.LocalActivationPending != tc.want {
				t.Fatalf("incorrect upgrade state: %#v", publication)
			}
			store, _ := cliconnector.NewArtifactStore(objects)
			if _, err := r.VerifyNextProductionConnector(ctx, store, &localConnectorSuite{}); err != nil {
				t.Fatal(err)
			}
			if err := r.db.Where("package_source=?", "github").Take(&publication).Error; err != nil {
				t.Fatal(err)
			}
			if (publication.State == "available") != tc.want {
				t.Fatalf("verification changed explicit enablement: %s", publication.State)
			}
		})
	}
}
