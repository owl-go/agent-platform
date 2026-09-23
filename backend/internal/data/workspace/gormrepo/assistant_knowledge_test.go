package gormrepo

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type captureSQLLogger struct {
	sql string
}

func (capture *captureSQLLogger) LogMode(logger.LogLevel) logger.Interface { return capture }
func (*captureSQLLogger) Info(context.Context, string, ...any)             {}
func (*captureSQLLogger) Warn(context.Context, string, ...any)             {}
func (*captureSQLLogger) Error(context.Context, string, ...any)            {}
func (capture *captureSQLLogger) Trace(_ context.Context, _ time.Time, statement func() (string, int64), _ error) {
	capture.sql, _ = statement()
}

func TestAssistantKnowledgeContextUsesCurrentKnowledgeBaseSchema(t *testing.T) {
	capture := &captureSQLLogger{}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable", PreferSimpleProtocol: true}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
		Logger:               capture,
	})
	if err != nil {
		t.Fatal(err)
	}

	assistantKnowledgeContext(db, "session-1", "owner-1", "product policy")

	if strings.Contains(capture.sql, "b.state") {
		t.Fatalf("knowledge query references removed knowledge_bases.state column: %s", capture.sql)
	}
	for _, predicate := range []string{"d.deleted_at IS NULL", "b.deleted_at IS NULL"} {
		if !strings.Contains(capture.sql, predicate) {
			t.Fatalf("knowledge query does not exclude deleted records with %q: %s", predicate, capture.sql)
		}
	}
}
