package gormrepo_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/artifact/domain"
	"agent-platform/backend/internal/data/artifact/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
)

func TestArtifactMetadataLifecycleWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("ARTIFACT_DATABASE_DSN")
	if dsn == "" {
		t.Skip("ARTIFACT_DATABASE_DSN is required for PostgreSQL integration")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{DSN: dsn, MaxOpenConnections: 2, MaxIdleConnections: 1, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var runID string
	if err := database.ORM().Raw(`SELECT id::text FROM runs ORDER BY created_at LIMIT 1`).Scan(&runID).Error; err != nil {
		t.Fatal(err)
	}
	if runID == "" {
		t.Skip("a Run fixture is required for Artifact integration")
	}
	now := time.Now().UTC()
	artifact, err := domain.New(uuid.NewString(), runID, "runtime_stdout", "integration/"+uuid.NewString()+"/stdout.log", 4, strings.Repeat("a", 64), "text/plain", map[string]string{"stream": "stdout"}, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	repository := gormrepo.New(database.ORM())
	if err := repository.Create(context.Background(), artifact); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.ORM().Exec(`DELETE FROM artifacts WHERE id = ?`, artifact.ID).Error })
	loaded, err := repository.Get(context.Background(), artifact.ID)
	if err != nil || loaded.RunID != runID || loaded.Metadata["stream"] != "stdout" {
		t.Fatalf("Get() = (%+v, %v)", loaded, err)
	}
	listed, err := repository.ListByRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range listed {
		found = found || candidate.ID == artifact.ID
	}
	if !found {
		t.Fatalf("Artifact %s missing from ListByRun()", artifact.ID)
	}
}
