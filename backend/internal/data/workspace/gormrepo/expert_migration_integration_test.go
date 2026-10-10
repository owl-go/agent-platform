package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"agent-platform/backend/internal/infrastructure/gormdb"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Fixture construction uses the unchanged migration files from the old release.
func applyLegacyExpertSchema(t *testing.T, db *gorm.DB, last string) {
	t.Helper()
	root := "../../../infrastructure/gormdb/migrations"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE schema_migrations(name text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`).Error; err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() > last {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(string(contents)).Error; err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(contents)
		if err := db.Exec(`INSERT INTO schema_migrations(name,checksum) VALUES(?,?)`, entry.Name(), hex.EncodeToString(digest[:])).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestExpertUpgradeDeletesPrivateTeamsAndPreservesAuthoredTextAndHistory(t *testing.T) {
	db := conversationTestDatabase(t, "000074_optional_platform_default_validation_run.sql")
	user, admin, expert, privateTeam, platformTeam, session, selection := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(sql string, args ...any) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, owner := range []string{user, admin} {
		exec(`INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,?)`, owner, owner, owner, owner+"@example.test", owner, owner == admin)
	}
	exec(`INSERT INTO experts(id,owner_user_id,name,name_normalized,introduction,core_capability,operating_procedure,output_standard,cautions) VALUES(?,?,'Reviewer','reviewer','DISPLAY_ONLY','  **设计**  ',E'步骤\n原文','报告','谨慎')`, expert, user)
	for _, fixture := range []struct{ id, owner string }{{privateTeam, user}, {platformTeam, admin}} {
		exec(`INSERT INTO expert_teams(id,owner_user_id,name,capability_introduction) VALUES(?,?,'Legacy Team','Display')`, fixture.id, fixture.owner)
	}
	exec(`INSERT INTO sessions(id,owner_user_id,expert_team_id) VALUES(?,?,?)`, session, user, privateTeam)
	exec(`INSERT INTO conversation_selections(id,owner_user_id,session_id,snapshot) VALUES(?,?,?,jsonb_build_object('expert_team_id',?::text))`, selection, user, session, privateTeam)
	exec(`UPDATE sessions SET selection_id=? WHERE id=?`, selection, session)
	exec(`INSERT INTO session_messages(session_id,role,state,content,response_snapshot) VALUES(?,'assistant','completed','private history','{"schema_version":2,"marker":"unchanged"}')`, session)
	// Interrupt the destructive migration after reference updates, before deletion.
	// Its transaction must roll back references and data together, then allow rerun.
	exec(`CREATE FUNCTION block_team_fixture_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture interruption'; END $$`)
	exec(`CREATE TRIGGER interrupt_team_fixture BEFORE DELETE ON expert_teams FOR EACH ROW EXECUTE FUNCTION block_team_fixture_delete()`)
	if err := gormdb.Migrate(context.Background(), db); err == nil {
		t.Fatal("fixture interruption did not fail migration")
	}
	var interrupted sessionRecord
	if err := db.Where("id=?", session).Take(&interrupted).Error; err != nil {
		t.Fatal(err)
	}
	if interrupted.ExpertTeamID == nil || *interrupted.ExpertTeamID != privateTeam || interrupted.SelectionID == nil {
		t.Fatal("failed deletion left partially cleared references")
	}
	var retained int64
	if err := db.Table("expert_teams").Where("id=?", privateTeam).Count(&retained).Error; err != nil || retained != 1 {
		t.Fatal("failed migration deleted private definition", err)
	}
	exec(`DROP TRIGGER interrupt_team_fixture ON expert_teams`)
	exec(`DROP FUNCTION block_team_fixture_delete()`)
	if err := gormdb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var teams int64
	if err := db.Table("expert_teams").Where("id=?", privateTeam).Count(&teams).Error; err != nil {
		t.Fatal(err)
	}
	if teams != 0 {
		t.Fatal("ordinary User's Team was retained")
	}
	if err := db.Table("expert_teams").Where("id=?", platformTeam).Count(&teams).Error; err != nil || teams != 1 {
		t.Fatalf("Administrator Team lost: %v", err)
	}
	var row sessionRecord
	if err := db.Where("id=?", session).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ExpertTeamID != nil || row.SelectionID != nil {
		t.Fatal("mutable selection still references deleted Team")
	}
	if _, err := New(db, nil).ResolveConversationSelection(context.Background(), user, domain.ConversationScope{SessionID: session}, domain.ConversationSelectionInput{PreviousID: selection}); err == nil {
		t.Fatal("a stale selection restored the retired private Team")
	}
	var guidance string
	if err := db.Raw(`SELECT guidance FROM experts WHERE id=?`, expert).Scan(&guidance).Error; err != nil {
		t.Fatal(err)
	}
	if guidance != "# Core Capability\n\n  **设计**  \n\n# Operating Procedure\n\n步骤\n原文\n\n# Output Standard\n\n报告\n\n# Cautions\n\n谨慎" {
		t.Fatalf("converted guidance=%q", guidance)
	}
	var history string
	if err := db.Raw(`SELECT response_snapshot->>'marker' FROM session_messages WHERE session_id=?`, session).Scan(&history).Error; err != nil || history != "unchanged" {
		t.Fatalf("history changed: %q %v", history, err)
	}
	if err := gormdb.Migrate(context.Background(), db); err != nil {
		t.Fatalf("repeated migration: %v", err)
	}
}
