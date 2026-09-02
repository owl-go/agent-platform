package domain

import "encoding/json"

// ExecutionSnapshot freezes every mutable selection required by a Workflow Run.
// Secret fields remain encrypted at rest and are decrypted only by the Worker.
type ExecutionSnapshot struct {
	WorkflowName                string                `json:"workflow_name"`
	Goal                        string                `json:"goal"`
	RuntimeEngine               RuntimeEngine         `json:"runtime_engine"`
	ProviderModel               ProviderModelSnapshot `json:"provider_model"`
	Personality                 string                `json:"personality"`
	PersonalityInstructions     string                `json:"personality_instructions"`
	Expert                      *ExpertSnapshot       `json:"expert,omitempty"`
	ExpertTeam                  *ExpertTeamSnapshot   `json:"expert_team,omitempty"`
	Environment                 []EnvironmentVariable `json:"environment"`
	EnvironmentSecretCiphertext []byte                `json:"environment_secret_ciphertext,omitempty"`
	MCPServers                  []MCPServerSnapshot   `json:"mcp_servers"`
	Skills                      []SkillSnapshot       `json:"skills"`
	GitSource                   *GitSource            `json:"git_source,omitempty"`
	GitSecretCiphertext         []byte                `json:"git_secret_ciphertext,omitempty"`
	WorkspacePath               string                `json:"workspace_path"`
}

type ProviderModelSnapshot struct {
	ID                string   `json:"id"`
	ConnectionID      string   `json:"connection_id"`
	ConnectionVersion int64    `json:"connection_version"`
	ConnectionName    string   `json:"connection_name"`
	ProviderType      string   `json:"provider_type"`
	ModelID           string   `json:"model_id"`
	Name              string   `json:"name"`
	Endpoint          string   `json:"endpoint"`
	Protocols         []string `json:"protocols"`
	// Populated only after the Worker resolves ConnectionID + ConnectionVersion.
	APIKeyCiphertext []byte `json:"-"`
}

type ExpertSnapshot struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	CapabilityIntroduction string   `json:"capability_introduction"`
	ExecutionInstruction   string   `json:"execution_instruction"`
	ExpertiseTags          []string `json:"expertise_tags"`
}

type ExpertTeamSnapshot struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	CapabilityIntroduction string                 `json:"capability_introduction"`
	ExpertiseTags          []string               `json:"expertise_tags"`
	Members                []ExpertMemberSnapshot `json:"members"`
}

type ExpertMemberSnapshot struct {
	ExpertSnapshot
	Position   int                 `json:"position"`
	MCPServers []MCPServerSnapshot `json:"mcp_servers"`
	Skills     []SkillSnapshot     `json:"skills"`
}

type MCPServerSnapshot struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Transport        string          `json:"transport"`
	Configuration    json.RawMessage `json:"configuration"`
	SecretCiphertext []byte          `json:"secret_ciphertext,omitempty"`
}

type SkillSnapshot struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ObjectKey string `json:"object_key"`
	SHA256    string `json:"sha256"`
}
