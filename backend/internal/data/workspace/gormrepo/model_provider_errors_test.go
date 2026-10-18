package gormrepo

import (
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestModelProviderSaveErrorClassifiesOnlyNameCollision(t *testing.T) {
	for _, test := range []struct {
		code, constraint string
		conflict         bool
	}{
		{"23505", "model_provider_connections_owner_user_id_name_key", true},
		{"23505", "model_provider_credential_versions_pkey", false},
		{"23503", "model_provider_connections_owner_user_id_name_key", false},
	} {
		t.Run(test.code+test.constraint, func(t *testing.T) {
			cause := &pgconn.PgError{Code: test.code, ConstraintName: test.constraint}
			err := modelProviderSaveError(cause)
			if errors.Is(err, domain.ErrProviderNameConflict) != test.conflict {
				t.Fatalf("classification = %v", err)
			}
			if !errors.Is(err, cause) {
				t.Fatal("storage cause was lost")
			}
		})
	}
}
