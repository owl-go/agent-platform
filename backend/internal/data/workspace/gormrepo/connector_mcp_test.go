package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestConnectorMCPConfigurationProjectsRevisionPolicy(t *testing.T) {
	policy := []byte(`{"auth_mode":"oauth","mcp":{"transport":"streamable_http","url":"https://mcp.example.test","arguments":["--safe"],"environment":[{"name":"REGION","value":"${REGION}"}]}}`)
	configuration, authMode, err := connectorMCPConfiguration(policy)
	if err != nil {
		t.Fatal(err)
	}
	if authMode != "oauth" || configuration.Transport != "streamable_http" || configuration.URL != "https://mcp.example.test" {
		t.Fatalf("projection = %#v, auth mode = %q", configuration, authMode)
	}
	if len(configuration.Arguments) != 1 || configuration.Arguments[0] != "--safe" || len(configuration.Environment) != 2 || configuration.Environment[1].Name != "MCP_BEARER_TOKEN" || !configuration.Environment[1].Secret {
		t.Fatalf("configuration = %#v", configuration)
	}
	encoded, err := json.Marshal(configuration)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("encoded configuration = %s, error = %v", encoded, err)
	}
}

func TestConnectorMCPConfigurationRejectsMissingPolicy(t *testing.T) {
	if _, _, err := connectorMCPConfiguration([]byte(`{"auth_mode":"none"}`)); err == nil {
		t.Fatal("expected missing MCP policy to be rejected")
	}
}

func TestConnectorMCPCatalogAndSnapshotUsePackageDisplayName(t *testing.T) {
	db := conversationTestDatabase(t)
	repo := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ source, declared, want string }{
		{"mindbye", "明白律师", "明白律师"},
		{"pkulaw", "北大法宝", "北大法宝"},
		{"caoliao", "草料二维码", "草料二维码"},
		{"notion", "Notion CLI", "Notion"},
		{"custom-search", "私人知识检索", "私人知识检索"},
		{"legacy-mcp", "", "legacy-mcp"},
	} {
		t.Run(test.source, func(t *testing.T) {
			policy, err := json.Marshal(map[string]any{"auth_mode": "none", "metadata": map[string]string{"name": test.declared}, "mcp": map[string]any{"transport": "streamable_http", "url": "https://mcp.example.test"}})
			if err != nil {
				t.Fatal(err)
			}
			revision, err := repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: test.source, Version: "1.0.0", Mode: domain.ConnectorModeMCP, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/" + test.source + "/package.zip", RuntimePolicy: policy})
			if err != nil {
				t.Fatal(err)
			}
			installation, err := repo.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: test.source, ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := connectorMCPServerSnapshot(db, owner, installation.ID)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Name != test.want {
				t.Errorf("snapshot name = %q, want %q", snapshot.Name, test.want)
			}
			for _, state := range []domain.ConnectorInstallationState{domain.ConnectorInstallationActive, domain.ConnectorInstallationDisabled} {
				if err := db.Model(&connectorInstallationRecord{}).Where("id = ?", installation.ID).Update("state", state).Error; err != nil {
					t.Fatal(err)
				}
				items, err := repo.ListMCPServers(ctx, owner)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, item := range items {
					if item.ID != installation.ID {
						continue
					}
					found = true
					if item.Name != test.want {
						t.Errorf("%s catalog name = %q, want %q", state, item.Name, test.want)
					}
					if (item.TestedAt != nil) != (state == domain.ConnectorInstallationActive) {
						t.Errorf("availability changed for %s", state)
					}
				}
				if !found {
					t.Fatal("installation missing from MCP catalog")
				}
			}
		})
	}
}

func TestMCPAuthorizationSnapshotPreservesGrantAADAndExcludesRefresh(t *testing.T) {
	db := conversationTestDatabase(t)
	repo := New(db, nil)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	revision, err := repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: "kling-ai", Version: "0.1.0", Mode: domain.ConnectorModeMCP, PackageSHA256: strings.Repeat("a", 64), ObjectKey: "connectors/kling-ai/package.zip", RuntimePolicy: []byte(`{"auth_mode":"oauth","mcp":{"transport":"streamable_http","url":"https://klingai.com/mcp","egress_hosts":["klingai.com"],"timeout_seconds":120}}`)})
	if err != nil {
		t.Fatal(err)
	}
	installation, err := repo.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "kling-ai", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connectorMCPServerSnapshot(db, owner, installation.ID); err == nil {
		t.Fatal("unauthorized snapshot accepted")
	}
	aad := "connector-authorization:" + owner + ":" + installation.ID + ":"
	_, err = repo.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "user", CredentialCiphertext: []byte("access-only"), CredentialAAD: aad, CredentialFormat: "json", RefreshCredentialCiphertext: []byte("platform-only-refresh"), RefreshCredentialAAD: aad + ":refresh"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := connectorMCPServerSnapshot(db, owner, installation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SecretAAD != aad || string(snapshot.SecretCiphertext) != "access-only" || strings.Contains(string(snapshot.Configuration), "refresh") {
		t.Fatal("authorization AAD or refresh boundary lost")
	}
	if _, err := connectorMCPServerSnapshot(db, uuid.NewString(), installation.ID); err == nil {
		t.Fatal("another owner obtained snapshot")
	}
}
