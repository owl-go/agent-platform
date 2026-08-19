package api

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type openAPIDocument struct {
	Paths      map[string]map[string]yaml.Node `yaml:"paths"`
	Components struct {
		Schemas map[string]struct {
			Properties map[string]yaml.Node `yaml:"properties"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

func TestOpenAPIPathsMatchRegisteredRoutes(t *testing.T) {
	document := loadOpenAPIDocument(t)
	want := []string{
		"GET /v1/agents",
		"GET /v1/agents/{agent_id}",
		"GET /v1/agents/{agent_id}/drafts",
		"GET /v1/agents/{agent_id}/drafts/{draft_id}",
		"GET /v1/agents/{agent_id}/memories",
		"GET /v1/agents/{agent_id}/releases",
		"GET /v1/agents/{agent_id}/releases/{release_id}",
		"GET /v1/approvals/{approval_id}",
		"GET /v1/audit-events",
		"GET /v1/artifacts/{artifact_id}/download",
		"GET /healthz",
		"GET /readyz",
		"GET /v1/me",
		"GET /v1/coding-tasks",
		"GET /v1/coding-tasks/{task_id}",
		"GET /v1/coding-tasks/{task_id}/memory-candidates",
		"GET /v1/coding-tasks/{task_id}/messages",
		"GET /v1/coding-tasks/{task_id}/session",
		"GET /v1/configured-models",
		"GET /v1/configured-models/{configured_model_id}",
		"GET /v1/credential-profiles",
		"GET /v1/credential-profiles/{credential_profile_id}",
		"GET /v1/repository-bindings",
		"GET /v1/repository-bindings/{repository_binding_id}",
		"GET /v1/runs",
		"GET /v1/runs/{run_id}",
		"GET /v1/runs/{run_id}/artifacts",
		"GET /v1/runs/{run_id}/approvals",
		"GET /v1/runs/{run_id}/events",
		"GET /v1/runtime-images",
		"GET /v1/runtime-images/{runtime_image_id}",
		"GET /v1/source-control-providers",
		"GET /v1/source-control-providers/{source_control_provider_id}",
		"PATCH /v1/configured-models/{configured_model_id}/status",
		"PATCH /v1/agent-memories/{memory_id}",
		"PATCH /v1/agents/{agent_id}",
		"PATCH /v1/agents/{agent_id}/drafts/{draft_id}",
		"PATCH /v1/agents/{agent_id}/drafts/{draft_id}/approval",
		"PATCH /v1/coding-tasks/{task_id}",
		"PATCH /v1/coding-tasks/{task_id}/session",
		"PATCH /v1/credential-profiles/{credential_profile_id}/status",
		"PATCH /v1/memory-candidates/{candidate_id}",
		"PATCH /v1/repository-bindings/{repository_binding_id}",
		"PATCH /v1/runtime-images/{runtime_image_id}/status",
		"PATCH /v1/source-control-providers/{source_control_provider_id}/status",
		"POST /v1/configured-models",
		"POST /v1/approvals/{approval_id}/decision",
		"POST /v1/agent-memories/{memory_id}/deletion",
		"POST /v1/agents",
		"POST /v1/agents/{agent_id}/drafts",
		"POST /v1/agents/{agent_id}/drafts/{draft_id}/approval",
		"POST /v1/agents/{agent_id}/drafts/{draft_id}/release",
		"POST /v1/agents/{agent_id}/drafts/{draft_id}/validation",
		"POST /v1/agents/{agent_id}/releases/{release_id}/block",
		"POST /v1/agents/{agent_id}/releases/{release_id}/deprecation",
		"POST /v1/coding-tasks",
		"POST /v1/coding-tasks/{task_id}/memory-candidates",
		"POST /v1/coding-tasks/{task_id}/runs",
		"POST /v1/credential-profiles",
		"POST /v1/repository-bindings",
		"POST /v1/repository-bindings/{repository_binding_id}/validation",
		"POST /v1/runs/{run_id}/cancel",
		"POST /v1/runs/{run_id}/approvals",
		"POST /v1/runs/{run_id}/interrupt",
		"POST /v1/runs/{run_id}/kill",
		"POST /v1/runs/{run_id}/resume",
		"POST /v1/runtime-images",
		"POST /v1/source-control-providers",
	}
	got := openAPIOperations(document)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("OpenAPI operations differ from HTTP routes\nwant:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}

}

func TestOpenAPICredentialResponsesAreSecretSafe(t *testing.T) {
	document := loadOpenAPIDocument(t)
	credential := document.Components.Schemas["v1CredentialProfile"]
	if _, exposed := credential.Properties["secret_ref"]; exposed {
		t.Fatal("CredentialProfile response schema must not expose secret_ref")
	}
	register := document.Components.Schemas["v1RegisterCredentialProfileRequest"]
	if _, accepted := register.Properties["secret_ref"]; !accepted {
		t.Fatal("RegisterCredentialProfile must accept the write-only secret_ref")
	}
}

func TestGeneratedOpenAPIMatchesFrozenV1Golden(t *testing.T) {
	want, err := os.ReadFile("../../../testdata/contracts/openapi-v1.yaml")
	if err != nil {
		t.Fatalf("read frozen v1 OpenAPI: %v", err)
	}
	got, err := os.ReadFile("../../../gen/openapi/agent-platform.openapi.json")
	if err != nil {
		t.Fatalf("read generated OpenAPI: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("generated OpenAPI differs from the frozen v1 golden; review every operation's parameters, headers, statuses, and JSON schema before updating the golden")
	}
}

func TestGeneratedOpenAPIModelsLegacyJSONShape(t *testing.T) {
	contents, err := os.ReadFile("../../../gen/openapi/agent-platform.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	schemas := document["components"].(map[string]any)["schemas"].(map[string]any)
	credential := schemas["v1CredentialProfile"].(map[string]any)["properties"].(map[string]any)
	if _, exposed := credential["secret_ref"]; exposed {
		t.Fatal("generated Credential Profile exposes secret_ref")
	}
	register := schemas["v1RegisterCredentialProfileRequest"].(map[string]any)["properties"].(map[string]any)
	if _, accepted := register["secret_ref"]; !accepted {
		t.Fatal("generated Credential Profile request omits secret_ref")
	}
	runtimeImage := schemas["v1RuntimeImage"].(map[string]any)["properties"].(map[string]any)
	version := runtimeImage["version"].(map[string]any)
	if version["type"] != "integer" || version["format"] != "int64" {
		t.Fatalf("Runtime Image Version schema = %#v, want an int64 JSON number", version)
	}
	if _, exposed := runtimeImage["organization_id"]; exposed {
		t.Fatal("Runtime Image response exposes its authorization scope")
	}
	if _, present := runtimeImage["conformance_evidence_key"]; !present {
		t.Fatal("Runtime Image response omits its logical Conformance evidence key")
	}
	if _, present := runtimeImage["conformance_evidence_sha256"]; !present {
		t.Fatal("Runtime Image response omits its immutable Conformance evidence SHA-256")
	}
	paths := document["paths"].(map[string]any)
	getRuntime := paths["/v1/runtime-images/{runtime_image_id}"].(map[string]any)["get"].(map[string]any)
	response := getRuntime["responses"].(map[string]any)["200"].(map[string]any)
	schema := response["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	if schema["$ref"] != "#/components/schemas/v1RuntimeImage" {
		t.Fatalf("Runtime Image response schema = %#v", schema)
	}
	listRuntime := paths["/v1/runtime-images"].(map[string]any)["get"].(map[string]any)
	parameters := listRuntime["parameters"].([]any)
	if !hasParameter(parameters, "query", "page_size", false) || !hasParameter(parameters, "query", "page_token", false) {
		t.Fatalf("Runtime Image list pagination parameters = %#v", parameters)
	}
	registerRuntime := paths["/v1/runtime-images"].(map[string]any)["post"].(map[string]any)
	if !hasParameter(registerRuntime["parameters"].([]any), "header", "Idempotency-Key", true) {
		t.Fatal("Runtime Image registration does not declare Idempotency-Key")
	}
	changeStatus := paths["/v1/runtime-images/{runtime_image_id}/status"].(map[string]any)["patch"].(map[string]any)
	statusParameters := changeStatus["parameters"].([]any)
	if !hasParameter(statusParameters, "header", "Idempotency-Key", true) || !hasParameter(statusParameters, "header", "If-Match", true) {
		t.Fatalf("Runtime Image status headers = %#v", statusParameters)
	}
}

func hasParameter(parameters []any, location, name string, required bool) bool {
	for _, raw := range parameters {
		parameter, ok := raw.(map[string]any)
		if ok && parameter["in"] == location && parameter["name"] == name && (parameter["required"] == true) == required {
			return true
		}
	}
	return false
}

func loadOpenAPIDocument(t *testing.T) openAPIDocument {
	t.Helper()
	contents, err := os.ReadFile("../../../testdata/contracts/openapi-v1.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI document: %v", err)
	}
	var document openAPIDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	return document
}

func openAPIOperations(document openAPIDocument) []string {
	methods := map[string]bool{
		"delete": true, "get": true, "head": true, "options": true,
		"patch": true, "post": true, "put": true, "trace": true,
	}
	operations := make([]string, 0)
	for path, item := range document.Paths {
		for method := range item {
			if methods[method] {
				operations = append(operations, strings.ToUpper(method)+" "+path)
			}
		}
	}
	sort.Strings(operations)
	return operations
}
