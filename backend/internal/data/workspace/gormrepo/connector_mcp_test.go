package gormrepo

import (
	"encoding/json"
	"testing"
)

func TestConnectorMCPConfigurationProjectsRevisionPolicy(t *testing.T) {
	policy := []byte(`{"auth_mode":"oauth","mcp":{"transport":"streamable_http","url":"https://mcp.example.test","arguments":["--safe"],"environment":[{"name":"REGION","value":"${REGION}"}]}}`)
	configuration, authMode, err := connectorMCPConfiguration(policy)
	if err != nil {
		t.Fatal(err)
	}
	if authMode != "oauth" || configuration.Transport != "streamable_http" || configuration.URL != "https://mcp.example.test" {
		t.Fatalf("projection = %#v, auth mode = %q", configuration, authMode)
	}
	if len(configuration.Arguments) != 1 || configuration.Arguments[0] != "--safe" || len(configuration.Environment) != 2 || configuration.Environment[1].Name != "MCP_BEARER_TOKEN" || !configuration.Environment[1].Secret {
		t.Fatalf("configuration = %#v", configuration)
	}
	encoded, err := json.Marshal(configuration)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("encoded configuration = %s, error = %v", encoded, err)
	}
}

func TestConnectorMCPConfigurationRejectsMissingPolicy(t *testing.T) {
	if _, _, err := connectorMCPConfiguration([]byte(`{"auth_mode":"none"}`)); err == nil {
		t.Fatal("expected missing MCP policy to be rejected")
	}
}
