package gormrepo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/secretcrypto"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestChannelProtectionPreservesIndependentResourceQueries(t *testing.T) {
	for _, test := range []struct {
		name      string
		query     func(*Repository) error
		contains  string
		forbidden string
	}{
		{"enablements", func(r *Repository) error {
			_, err := r.ListCLIConnectorEnablements(context.Background(), "owner")
			return err
		}, `FROM "cli_connector_enablements"`, `SELECT "id" FROM "cli_connector_definitions" WHERE (owner_user_id`},
		{"experts", func(r *Repository) error { _, err := r.ListExperts(context.Background(), "owner"); return err }, `FROM "experts"`, `"users"."deleted_at"`},
		{"skills", func(r *Repository) error { _, err := r.ListSkills(context.Background(), "owner"); return err }, `FROM "skills"`, `"users"."deleted_at"`},
		{"mcp", func(r *Repository) error { _, err := r.ListMCPServers(context.Background(), "owner"); return err }, `FROM "connector_installations"`, `"cli_connector_definitions"`},
		{"cli", func(r *Repository) error {
			_, err := r.ListCLIConnectorDefinitions(context.Background(), false)
			return err
		}, "FROM cli_connector_conformance c", `"c"."deleted_at"`},
		{"catalog", func(r *Repository) error {
			_, err := r.ListConnectorPublications(context.Background(), false)
			return err
		}, `FROM "connector_package_publications"`, `"cli_connector_definitions"`},
		{"connectors", func(r *Repository) error {
			_, err := r.ListConnectorInstallations(context.Background(), "owner")
			return err
		}, "FROM connector_installations AS installation", `"installation"."deleted_at"`},
		{"approvals", func(r *Repository) error {
			_, err := r.ListCommandApprovals(context.Background(), "owner", time.Now())
			return err
		}, `FROM "cli_command_approvals"`, `"cli_connector_definitions"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			capture := &captureSQLLogger{}
			db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable", PreferSimpleProtocol: true}), &gorm.Config{DisableAutomaticPing: true, DryRun: true, SkipDefaultTransaction: true, Logger: capture})
			if err != nil {
				t.Fatal(err)
			}
			r := New(db, nil)
			cipher := &secretcrypto.Box{}
			r.ConfigureChannelProtection(cipher, true, application.ChannelLimits{MaxPendingMessages: 7})
			// This real call creates the Definition subquery used by the settings page.
			if _, err := r.ListCLIConnectorEnablements(context.Background(), "owner"); err != nil {
				t.Fatal(err)
			}
			// Scan still logs its built SQL in DryRun but cannot return database rows.
			if err := test.query(r); err != nil && !errors.Is(err, gorm.ErrDryRunModeUnsupported) {
				t.Fatal(err)
			}
			if !strings.Contains(capture.sql, test.contains) || strings.Contains(capture.sql, test.forbidden) {
				t.Fatalf("resource query inherited another model or projection: %s", capture.sql)
			}
			configured := r.db.WithContext(context.Background())
			if value, ok := configured.Get(channelProtectionKey); !ok || value != cipher {
				t.Fatal("channel cipher was lost")
			}
			if value, ok := configured.Get(channelEnabledKey); !ok || value != true {
				t.Fatal("channel enablement was lost")
			}
			if channelLimits(configured).MaxPendingMessages != 7 {
				t.Fatal("channel limits were lost")
			}
		})
	}
}
