package workspace

import (
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestSessionWorkflowResourcesDeduplicateFrozenSelection(t *testing.T) {
	stages := []domain.ExecutionStageSnapshot{
		{Expert: &domain.ExpertSnapshot{ID: "expert-1", Name: "Researcher"}, Skills: []domain.SkillSnapshot{{ID: "skill-1", Name: "Search"}}, MCPServers: []domain.MCPServerSnapshot{{ID: "mcp-1", Name: "Docs"}}},
		{Expert: &domain.ExpertSnapshot{ID: "expert-2", Name: "Reviewer"}, Skills: []domain.SkillSnapshot{{ID: "skill-1", Name: "Search"}}, CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "cli-1", Name: "Git"}}},
	}
	name, resources := sessionWorkflowResources(stages)
	if name != "Researcher / Reviewer" {
		t.Fatalf("specialist name = %q", name)
	}
	if len(resources) != 5 {
		t.Fatalf("resource count = %d, want 5", len(resources))
	}
}

func TestSessionWorkflowFileDecisionsRequireEveryServerSource(t *testing.T) {
	sources := map[string]sessionWorkflowSource{
		"attachment:one": {view: &workspacev1.SessionWorkflowFile{Available: true}},
		"artifact:two":   {view: &workspacev1.SessionWorkflowFile{Available: false}},
	}
	if _, err := validateSessionWorkflowFileDecisions(sources, []*workspacev1.SessionWorkflowFileDecision{{SourceKey: "attachment:one", Destination: "workspace"}}); err == nil {
		t.Fatal("missing decision was accepted")
	}
	decisions, err := validateSessionWorkflowFileDecisions(sources, []*workspacev1.SessionWorkflowFileDecision{
		{SourceKey: "attachment:one", Destination: "workspace"},
		{SourceKey: "artifact:two", Destination: "exclude"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if decisions["attachment:one"] != "workspace" || decisions["artifact:two"] != "exclude" {
		t.Fatalf("decisions = %#v", decisions)
	}
	if _, err := validateSessionWorkflowFileDecisions(sources, []*workspacev1.SessionWorkflowFileDecision{
		{SourceKey: "attachment:one", Destination: "workspace"},
		{SourceKey: "artifact:two", Destination: "workspace"},
	}); err == nil {
		t.Fatal("unavailable source was accepted for Workspace copy")
	}
}

func TestSessionWorkflowFileNameCannotEscapeWorkspace(t *testing.T) {
	name := sessionWorkflowFileName("artifact:abc", "../../secret\n.txt")
	if name != "artifact-abc-secret-.txt" {
		t.Fatalf("safe name = %q", name)
	}
}
