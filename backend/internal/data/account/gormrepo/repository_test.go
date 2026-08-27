package gormrepo

import (
	"testing"

	"agent-platform/backend/internal/biz/account/domain"

	"github.com/google/uuid"
)

func TestNewUserModelAssignsDatabaseIdentity(t *testing.T) {
	model := newUserModel(domain.User{
		OIDCSubject: "subject", Username: "user", Email: "user@example.test",
		DisplayName: "User", Administrator: true,
	})

	if _, err := uuid.Parse(model.ID); err != nil {
		t.Fatalf("new User ID %q is not a UUID: %v", model.ID, err)
	}
	if !model.Administrator || model.OIDCSubject != "subject" || model.Version != 1 {
		t.Fatalf("new User mapping lost domain fields: %+v", model)
	}
}
