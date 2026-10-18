package gormrepo

import (
	"context"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestExpertDefinitionRevisionsCannotBeRewrittenOrRemoved(t *testing.T) {
	db := conversationTestDatabase(t)
	owner := uuid.NewString()
	if err := db.Exec(`INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)`, owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	repository := New(db, nil)
	ctx := context.Background()
	expert, err := repository.CreateExpert(ctx, owner, domain.ExpertInput{Name: "Reviewer", Introduction: "Display", Guidance: "# Original\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpdateExpert(ctx, owner, expert.ID, domain.ExpertInput{Name: "Reviewer", Introduction: "Display", Guidance: "# Changed\n"}, expert.Version); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := db.Raw(`SELECT definition->>'guidance' FROM expert_definition_revisions WHERE expert_id=? AND version=1`, expert.ID).Scan(&original).Error; err != nil {
		t.Fatal(err)
	}
	if original != "# Original\n" {
		t.Fatalf("original guidance=%q", original)
	}
	if err := db.Exec(`UPDATE expert_definition_revisions SET definition='{}' WHERE expert_id=?`, expert.ID).Error; err == nil {
		t.Fatal("immutable revision could be rewritten")
	}
	if err := db.Exec(`DELETE FROM expert_definition_revisions WHERE expert_id=?`, expert.ID).Error; err == nil {
		t.Fatal("immutable revision could be removed")
	}
	if err := repository.DeleteExpert(ctx, owner, expert.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("expert_definition_revisions").Where("expert_id=?", expert.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("retained revisions=%d", count)
	}
}
