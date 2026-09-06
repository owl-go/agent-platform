package domain

import (
	"reflect"
	"testing"
)

func TestConversationSelectionAppliesExplicitOverridesPerMember(t *testing.T) {
	selection := ConversationSelection{Defaults: []ExecutionStageSnapshot{
		{Expert: &ExpertSnapshot{ID: "expert-a"}, TeamMemberID: "member-a", Skills: []SkillSnapshot{{ID: "shared", SHA256: "old"}, {ID: "private-a"}}, MCPServers: []MCPServerSnapshot{{ID: "disabled"}}},
		{Expert: &ExpertSnapshot{ID: "expert-b"}, TeamMemberID: "member-b", Skills: []SkillSnapshot{{ID: "private-b"}}},
	}, Skills: []SkillSnapshot{{ID: "shared", SHA256: "new"}}, MCPServers: []MCPServerSnapshot{{ID: "direct"}}, DisabledConnectors: []string{"mcp:disabled"}}
	configuration := ExecutionStageSnapshot{RuntimeEngine: RuntimeCodex, ProviderModel: ProviderModelSnapshot{ID: "frozen-model"}, CreditRate: &CreditRateSnapshot{RevisionID: "rate"}, Skills: []SkillSnapshot{{ID: "obsolete"}}}
	stages := selection.Apply(configuration)
	if len(stages) != 2 || stages[0].Skills[0].SHA256 != "new" || stages[1].Skills[0].ID != "private-b" || stages[1].Skills[1].ID != "shared" {
		t.Fatalf("incorrect member resources: %#v", stages)
	}
	for _, stage := range stages {
		if stage.RuntimeEngine != RuntimeCodex || stage.ProviderModel.ID != "frozen-model" || len(stage.MCPServers) != 1 || stage.MCPServers[0].ID != "direct" {
			t.Fatalf("lost configuration or exclusion: %#v", stage)
		}
	}
	if selection.Defaults[0].Skills[0].SHA256 != "old" {
		t.Fatal("mutated frozen expert defaults")
	}
	selection.Skills = nil
	next := selection.Apply(configuration)
	if next[0].Skills[0].SHA256 != "old" || len(next[1].Skills) != 1 {
		t.Fatal("one-turn Skill did not revert to inherited version")
	}
	if stages[0].SelectionKey == next[0].SelectionKey || stages[0].SelectionKey == stages[1].SelectionKey {
		t.Fatal("resource or member changes reused state identity")
	}
	if !reflect.DeepEqual(next, selection.Apply(configuration)) {
		t.Fatal("same selection is not stable")
	}
	selection.Defaults = nil
	selection.MCPServers = nil
	cleared := selection.Apply(configuration)
	if len(cleared) != 1 || cleared[0].Expert != nil || len(cleared[0].Skills) != 0 {
		t.Fatal("removed specialist leaked through execution configuration")
	}
}

func TestConversationScopeRejectsAmbiguousOrIncompleteScopes(t *testing.T) {
	for _, scope := range []ConversationScope{{}, {WorkflowID: "w"}, {RunID: "r"}, {SessionID: "s", RunID: "r", WorkflowID: "w"}} {
		if scope.Validate() == nil {
			t.Fatalf("accepted %#v", scope)
		}
	}
	for _, scope := range []ConversationScope{{SessionID: "s"}, {WorkflowID: "w", RunID: "r"}} {
		if err := scope.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
