package gormrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agent-platform/backend/internal/defaultresources"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestDefaultExpertPackagesInitializeAndUpgradeBothTypesAtomically(t *testing.T) {
	repo, owner := defaultResourceRepositoryFixture(t)
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, directory := range []string{"skills", "connectors", "experts"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write("resources.json", `{"version":"1.0.0"}`)
	write("experts/example/.plugin/plugin.json", `{"schema_version":1,"id":"example.expert","version":"1.0.0","kind":"expert","expert":{"name":"Directory Expert","introduction":"Review evidence","guidance_file":"agents/expert.md","bundled_skills":["skills/review"]}}`)
	write("experts/example/agents/expert.md", "# Original Expert\n")
	write("experts/example/skills/review/SKILL.md", "---\ndisplay_name: Directory Review\n---\n# Review\nReview evidence.\n")
	manifest := `{"schema_version":1,"id":"example.team","version":"1.0.0","kind":"expert_team","team":{"name":"Directory Team","introduction":"Review evidence","core_capability":"Review and synthesize","lead_member_id":"lead","members":[{"id":"lead","name":"Lead","expert":{"name":"Lead","introduction":"Coordinate","guidance_file":"agents/lead.md"}},{"id":"reviewer","name":"Reviewer","expert":{"name":"Reviewer","introduction":"Review","guidance_file":"agents/reviewer.md"}}]}}`
	write("experts/team/.plugin/plugin.json", manifest)
	write("experts/team/agents/lead.md", "# Original Lead\n")
	write("experts/team/agents/reviewer.md", "# Review\n")
	t.Setenv(defaultresources.RootEnvironment, root)
	objects := memory.New()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- repo.EnsureDefaultResources(context.Background(), objects) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	experts, err := repo.ListExperts(context.Background(), owner)
	if err != nil || len(experts) != 1 {
		t.Fatalf("Experts=%d err=%v", len(experts), err)
	}
	if len(experts[0].BundledSkills) != 1 {
		t.Fatal("directory bundled Skill was not installed")
	}
	if _, err := objects.Stat(context.Background(), experts[0].BundledSkills[0].ObjectKey); err != nil {
		t.Fatal(err)
	}
	teams, err := repo.ListExpertTeams(context.Background(), owner)
	if err != nil || len(teams) != 1 {
		t.Fatalf("Teams=%d err=%v", len(teams), err)
	}
	team := teams[0]
	if !team.Immutable || !team.Platform || team.LeadMemberID != "lead" || team.Members[0].Expert.Guidance != "# Original Lead\n" {
		t.Fatalf("invalid Platform Team: %+v", team)
	}
	if err := repo.DeleteExpertTeam(context.Background(), owner, team.ID); err == nil {
		t.Fatal("directory original was mutable")
	}
	write("experts/team/.plugin/plugin.json", strings.Replace(manifest, "1.0.0", "1.1.0", 1))
	write("experts/team/agents/lead.md", "# Upgraded Lead\n")
	if err := repo.EnsureDefaultResources(context.Background(), objects); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.GetExpertTeam(context.Background(), owner, team.ID)
	if err != nil || updated.Version != team.Version+1 || updated.Members[0].Expert.Guidance != "# Upgraded Lead\n" {
		t.Fatalf("managed upgrade lost identity: %+v err=%v", updated, err)
	}
	var count int64
	if err := repo.db.Table("expert_team_definition_revisions").Where("team_id=?", team.ID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("Team revisions=%d err=%v", count, err)
	}
	write("experts/team/agents/lead.md", "# Unversioned mutation\n")
	if err := repo.EnsureDefaultResources(context.Background(), objects); err == nil {
		t.Fatal("same-version content overwrite accepted")
	}
	current, err := repo.GetExpertTeam(context.Background(), owner, team.ID)
	if err != nil || current.Members[0].Expert.Guidance != "# Upgraded Lead\n" {
		t.Fatal("failed initialization changed the Team")
	}
	if err := os.RemoveAll(filepath.Join(root, "experts/team")); err != nil {
		t.Fatal(err)
	}
	if err := repo.EnsureDefaultResources(context.Background(), objects); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetExpertTeam(context.Background(), owner, team.ID); err != nil {
		t.Fatal("directory removal deleted initialized Team", err)
	}
}
