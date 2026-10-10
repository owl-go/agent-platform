package gormdb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestOpenRejectsInvalidConfigurationBeforeConnecting(t *testing.T) {
	for _, config := range []Config{
		{},
		{DSN: "postgres://database/platform"},
		{DSN: "postgres://database/platform", MaxOpenConnections: 1, MaxIdleConnections: 2, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute},
	} {
		if _, err := Open(context.Background(), config); err == nil {
			t.Fatalf("Open accepted invalid configuration: %+v", config)
		}
	}
}

func TestDatabaseTranslationRetainsPostgresCause(t *testing.T) {
	for _, test := range []struct {
		code       string
		translated error
	}{
		{"23505", gorm.ErrDuplicatedKey},
		{"23503", gorm.ErrForeignKeyViolated},
		{"42703", gorm.ErrInvalidField},
		{"23514", gorm.ErrCheckConstraintViolated},
		{"XX000", nil},
	} {
		t.Run(test.code, func(t *testing.T) {
			cause := &pgconn.PgError{Code: test.code, ConstraintName: "fixture_constraint"}
			db := &gorm.DB{Config: &gorm.Config{TranslateError: true, Dialector: newPostgresDialector("")}}
			db.AddError(cause)
			var original *pgconn.PgError
			if !errors.As(db.Error, &original) || original != cause {
				t.Fatalf("PostgreSQL cause was lost: %v", db.Error)
			}
			if test.translated != nil && !errors.Is(db.Error, test.translated) {
				t.Fatalf("GORM error category was lost: %v", db.Error)
			}
		})
	}
}

func TestDatabaseTranslationLeavesUnmappedErrorsUnchanged(t *testing.T) {
	dialector := newPostgresDialector("").(gorm.ErrorTranslator)
	for _, cause := range []error{nil, gorm.ErrRecordNotFound, errors.New("connection unavailable")} {
		if got := dialector.Translate(cause); got != cause {
			t.Fatalf("unmapped error changed: %v", got)
		}
	}
}

func TestDatabaseTranslationPreservesPostgresInterfaces(t *testing.T) {
	dialector := newPostgresDialector("")
	if _, ok := dialector.(gorm.SavePointerDialectorInterface); !ok {
		t.Fatal("PostgreSQL savepoint support was lost")
	}
	if _, ok := dialector.(interface{ Apply(*gorm.Config) error }); !ok {
		t.Fatal("PostgreSQL initialization defaults were lost")
	}
}
