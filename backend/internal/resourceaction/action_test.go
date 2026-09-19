package resourceaction

import "testing"

func TestParseValidatedProposalStripsMarker(t *testing.T) {
	content := "已准备预览。\n<platform-action>{\"kind\":\"expert\",\"user_message\":\"请确认\",\"expert\":{\"name\":\"架构专家\",\"introduction\":\"简介\",\"core_capability\":\"能力\",\"operating_procedure\":\"流程\",\"output_standard\":\"规范\"}}</platform-action>"
	proposal, visible, marked, err := Parse(content)
	if err != nil || !marked {
		t.Fatalf("Parse() error=%v marked=%v", err, marked)
	}
	if proposal.Kind != ExpertKind || proposal.Expert.Name != "架构专家" || visible != "已准备预览。" {
		t.Fatalf("proposal=%+v visible=%q", proposal, visible)
	}
}

func TestParseRejectsIncompleteGeneratedSkill(t *testing.T) {
	_, _, marked, err := Parse(`<platform-action>{"kind":"skill","skill":{"name":"Missing document","source":"generated"}}</platform-action>`)
	if !marked || err == nil {
		t.Fatalf("Parse() marked=%v err=%v, want validation error", marked, err)
	}
}
