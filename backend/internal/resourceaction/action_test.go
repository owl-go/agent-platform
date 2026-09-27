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

func TestParseConnectorProposal(t *testing.T) {
	content := `<platform-action>{"kind":"connector","user_message":"请确认","connector":{"source":"example-service","version":"1.0.0","name":"Example Service","description":"Query data","auth_mode":"none","mcp_json":"{\"transport\":\"streamable_http\",\"url\":\"https://example.com/mcp\",\"timeout_seconds\":30,\"egress_hosts\":[\"example.com\"]}","skill_name":"example-service","skill_markdown":"# Example Service\nQuery data."}}</platform-action>`
	proposal, visible, marked, err := Parse(content)
	if err != nil || !marked || proposal.Kind != ConnectorKind || visible != "请确认" {
		t.Fatalf("Parse() proposal=%+v visible=%q marked=%v err=%v", proposal, visible, marked, err)
	}
	if name, _ := proposal.NameAndDescription(); name != "Example Service" {
		t.Fatalf("NameAndDescription() name=%q", name)
	}
}

func TestParseRejectsIncompleteConnector(t *testing.T) {
	_, _, marked, err := Parse(`<platform-action>{"kind":"connector","connector":{"name":"Missing manifest"}}</platform-action>`)
	if !marked || err == nil {
		t.Fatalf("Parse() marked=%v err=%v, want validation error", marked, err)
	}
}
