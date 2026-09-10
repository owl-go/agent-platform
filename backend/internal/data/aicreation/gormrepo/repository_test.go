package gormrepo

import (
	"reflect"
	"testing"

	"agent-platform/backend/internal/biz/aicreation/domain"
)

func TestCurrentImageModelRevisionGuard(t *testing.T) {
	tests := []struct {
		name           string
		model          domain.ImageModelRevision
		revisionExists bool
		wantCondition  string
		wantArguments  []any
	}{
		{
			name:           "existing revised model verifies its current revision",
			model:          domain.ImageModelRevision{RevisionID: "revision-2", PredecessorID: "revision-1"},
			revisionExists: true,
			wantCondition:  "current_revision_id = ?",
			wantArguments:  []any{"revision-2"},
		},
		{
			name:           "new revision replaces its predecessor",
			model:          domain.ImageModelRevision{RevisionID: "revision-2", PredecessorID: "revision-1"},
			revisionExists: false,
			wantCondition:  "current_revision_id = ?",
			wantArguments:  []any{"revision-1"},
		},
		{
			name:           "initial revision permits an empty pointer",
			model:          domain.ImageModelRevision{RevisionID: "revision-1"},
			revisionExists: false,
			wantCondition:  "current_revision_id IS NULL OR current_revision_id = ?",
			wantArguments:  []any{"revision-1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			condition, arguments := currentImageModelRevisionGuard(test.model, test.revisionExists)
			if condition != test.wantCondition || !reflect.DeepEqual(arguments, test.wantArguments) {
				t.Fatalf("guard = %q %v, want %q %v", condition, arguments, test.wantCondition, test.wantArguments)
			}
		})
	}
}
