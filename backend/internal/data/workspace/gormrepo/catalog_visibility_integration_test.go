package gormrepo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
)

func TestCatalogVisibilityIncludesPlatformAndOwnResources(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	repository := New(db, nil)
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	administrator, owner, other := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,true)", administrator, administrator, administrator, administrator+"@example.test", administrator)
	for _, id := range []string{owner, other} {
		exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id)
	}

	platformSkill, ownSkill, privateSkill := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, item := range []struct{ id, resourceOwner, name string }{{platformSkill, administrator, "Platform Skill"}, {ownSkill, owner, "Own Skill"}, {privateSkill, other, "Private Skill"}} {
		exec("INSERT INTO skills(id,owner_user_id,name,source,object_key,sha256) VALUES(?, ?, ?, 'upload', ?, ?)", item.id, item.resourceOwner, item.name, "skills/"+item.id, strings.Repeat("a", 64))
	}
	configuration := `{"url":"https://mcp.example.test","environment":[]}`
	platformMCP, ownMCP, privateMCP := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, item := range []struct{ id, resourceOwner, name string }{{platformMCP, administrator, "Platform MCP"}, {ownMCP, owner, "Own MCP"}, {privateMCP, other, "Private MCP"}} {
		exec("INSERT INTO mcp_servers(id,owner_user_id,name,transport,configuration,tested_at) VALUES(?, ?, ?, 'streamable_http', ?::jsonb, ?)", item.id, item.resourceOwner, item.name, configuration, time.Now().UTC())
	}
	platformExpert, ownExpert, privateExpert := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, item := range []struct{ id, resourceOwner, name string }{{platformExpert, administrator, "Platform Expert"}, {ownExpert, owner, "Own Expert"}, {privateExpert, other, "Private Expert"}} {
		exec("INSERT INTO experts(id,owner_user_id,name,introduction,core_capability,operating_procedure,output_standard) VALUES(?, ?, ?, 'Intro', 'Capability', 'Procedure', 'Standard')", item.id, item.resourceOwner, item.name)
	}

	experts, err := repository.ListExperts(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(experts) != 2 || !catalogContainsExpert(experts, platformExpert, true) || !catalogContainsExpert(experts, ownExpert, false) || catalogContainsExpert(experts, privateExpert, false) {
		t.Fatalf("Expert catalog = %#v", experts)
	}
	skills, err := repository.ListSkills(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 || !catalogContainsSkill(skills, platformSkill, true) || !catalogContainsSkill(skills, ownSkill, false) || catalogContainsSkill(skills, privateSkill, false) {
		t.Fatalf("Skill catalog = %#v", skills)
	}
	servers, err := repository.ListMCPServers(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 || !catalogContainsMCP(servers, platformMCP, true) || !catalogContainsMCP(servers, ownMCP, false) || catalogContainsMCP(servers, privateMCP, false) {
		t.Fatalf("MCP catalog = %#v", servers)
	}

	created, err := repository.CreateExpert(ctx, owner, domain.ExpertInput{Name: "Uses platform resources", Introduction: "Intro", CoreCapability: "Capability", OperatingProcedure: "Procedure", OutputStandard: "Standard", SkillIDs: []string{platformSkill}, MCPServerIDs: []string{platformMCP}})
	if err != nil {
		t.Fatal(err)
	}
	var row expertRecord
	if err = db.Where("id = ?", created.ID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := loadExpertMemberSnapshot(db, owner, row, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Skills) != 1 || snapshot.Skills[0].ID != platformSkill || len(snapshot.MCPServers) != 1 || snapshot.MCPServers[0].ID != platformMCP || snapshot.MCPServers[0].SecretOwnerID != administrator {
		encoded, _ := json.Marshal(snapshot)
		t.Fatalf("platform resource snapshot = %s", encoded)
	}
	impact, err := repository.GetSkillDeletionImpact(ctx, administrator, platformSkill)
	if err != nil {
		t.Fatal(err)
	}
	if len(impact.AffectedExperts) != 1 || impact.AffectedExperts[0].ID != created.ID {
		t.Fatalf("platform Skill deletion impact = %#v", impact)
	}
}

func catalogContainsExpert(items []domain.Expert, id string, platform bool) bool {
	for _, item := range items {
		if item.ID == id {
			return item.Platform == platform
		}
	}
	return false
}

func catalogContainsSkill(items []domain.Skill, id string, platform bool) bool {
	for _, item := range items {
		if item.ID == id {
			return item.Platform == platform
		}
	}
	return false
}

func catalogContainsMCP(items []domain.MCPServer, id string, platform bool) bool {
	for _, item := range items {
		if item.ID == id {
			return item.Platform == platform
		}
	}
	return false
}
