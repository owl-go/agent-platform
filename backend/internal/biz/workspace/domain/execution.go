package domain

import (
	"encoding/json"
	"fmt"
)

// ExecutionSnapshot freezes every mutable selection required by a Workflow Run.
// Secret fields remain encrypted at rest and are decrypted only by the Worker.
type ExecutionSnapshot struct {
	SchemaVersion               int                      `json:"schema_version,omitempty"`
	Stages                      []ExecutionStageSnapshot `json:"stages,omitempty"`
	WorkflowName                string                   `json:"workflow_name"`
	Goal                        string                   `json:"goal"`
	RuntimeEngine               RuntimeEngine            `json:"runtime_engine"`
	ProviderModel               ProviderModelSnapshot    `json:"provider_model"`
	Personality                 string                   `json:"personality"`
	PersonalityInstructions     string                   `json:"personality_instructions"`
	Expert                      *ExpertSnapshot          `json:"expert,omitempty"`
	ExpertTeam                  *ExpertTeamSnapshot      `json:"expert_team,omitempty"`
	Environment                 []EnvironmentVariable    `json:"environment"`
	EnvironmentSecretCiphertext []byte                   `json:"environment_secret_ciphertext,omitempty"`
	MCPServers                  []MCPServerSnapshot      `json:"mcp_servers"`
	Skills                      []SkillSnapshot          `json:"skills"`
	CLIConnectors               []CLIConnectorSnapshot   `json:"cli_connectors"`
	GitSource                   *GitSource               `json:"git_source,omitempty"`
	GitSecretCiphertext         []byte                   `json:"git_secret_ciphertext,omitempty"`
	WorkspacePath               string                   `json:"workspace_path"`
}

// ExecutionStageSnapshot is the complete immutable identity of one model invocation.
type ExecutionStageSnapshot struct {
	Position         int                    `json:"position"`
	Expert           *ExpertSnapshot        `json:"expert,omitempty"`
	RuntimeEngine    RuntimeEngine          `json:"runtime_engine"`
	ProviderModel    ProviderModelSnapshot  `json:"provider_model"`
	ModelProtocol    string                 `json:"model_protocol,omitempty"`
	CreditRate       *CreditRateSnapshot    `json:"credit_rate,omitempty"`
	MCPServers       []MCPServerSnapshot    `json:"mcp_servers"`
	Skills           []SkillSnapshot        `json:"skills"`
	CLIConnectors    []CLIConnectorSnapshot `json:"cli_connectors"`
	TeamMemberID     string                 `json:"team_member_id,omitempty"`
	TeamMemberName   string                 `json:"team_member_name,omitempty"`
	TeamMemberLabels []string               `json:"team_member_labels,omitempty"`
}

type CreditRateSnapshot struct {
	RevisionID             string `json:"revision_id"`
	InputMultiplierMicros  int64  `json:"input_multiplier_micros"`
	OutputMultiplierMicros int64  `json:"output_multiplier_micros"`
	FallbackHundredths     int64  `json:"fallback_hundredths"`
}

// ModelProtocolForRuntime returns the protocol the selected Runtime will use.
func ModelProtocolForRuntime(runtime RuntimeEngine, protocols []string) (string, error) {
	has := func(want string) bool {
		for _, protocol := range protocols {
			if protocol == want {
				return true
			}
		}
		return false
	}
	switch runtime {
	case RuntimeClaude:
		if has("anthropic_messages") {
			return "anthropic_messages", nil
		}
	case RuntimeCodex:
		if has("openai_responses") {
			return "openai_responses", nil
		}
	case RuntimePI:
		for _, protocol := range []string{"openai_responses", "openai_chat", "anthropic_messages", "gemini"} {
			if has(protocol) {
				return protocol, nil
			}
		}
	case RuntimeHermes, RuntimeOpenClaw:
		if len(protocols) > 0 {
			return protocols[0], nil
		}
	}
	return "", fmt.Errorf("%w: no supported Model API Protocol for %s", ErrInvalid, runtime)
}

// OrderedStages hides snapshot schema compatibility from execution callers.
func (snapshot ExecutionSnapshot) OrderedStages() ([]ExecutionStageSnapshot, error) {
	if len(snapshot.Stages) > 0 {
		if snapshot.SchemaVersion != 2 || snapshot.RuntimeEngine != "" || snapshot.ProviderModel.ID != "" || snapshot.Expert != nil || snapshot.ExpertTeam != nil {
			return nil, fmt.Errorf("%w: Execution Snapshot mixes stage and legacy schemas", ErrInvalid)
		}
		stages := append([]ExecutionStageSnapshot(nil), snapshot.Stages...)
		for index, stage := range stages {
			if stage.Position != index+1 || stage.ProviderModel.ID == "" {
				return nil, fmt.Errorf("%w: invalid Execution Stage Snapshot at position %d", ErrInvalid, index+1)
			}
			if _, err := ParseRuntime(string(stage.RuntimeEngine)); err != nil {
				return nil, err
			}
		}
		return stages, nil
	}

	if snapshot.SchemaVersion != 0 || snapshot.RuntimeEngine == "" || snapshot.ProviderModel.ID == "" && snapshot.ProviderModel.ModelID == "" {
		return nil, fmt.Errorf("%w: incomplete legacy Execution Snapshot", ErrInvalid)
	}
	stageFor := func(position int, expert *ExpertSnapshot, servers []MCPServerSnapshot, skills []SkillSnapshot, connectors []CLIConnectorSnapshot) ExecutionStageSnapshot {
		return ExecutionStageSnapshot{Position: position, Expert: expert, RuntimeEngine: snapshot.RuntimeEngine, ProviderModel: snapshot.ProviderModel, MCPServers: servers, Skills: skills, CLIConnectors: connectors}
	}
	if snapshot.ExpertTeam != nil {
		stages := make([]ExecutionStageSnapshot, 0, len(snapshot.ExpertTeam.Members))
		for index := range snapshot.ExpertTeam.Members {
			member := &snapshot.ExpertTeam.Members[index]
			expert := member.ExpertSnapshot
			stages = append(stages, stageFor(index+1, &expert, member.MCPServers, member.Skills, member.CLIConnectors))
		}
		return stages, nil
	}
	if snapshot.Expert != nil {
		return []ExecutionStageSnapshot{stageFor(1, snapshot.Expert, snapshot.MCPServers, snapshot.Skills, snapshot.CLIConnectors)}, nil
	}
	return []ExecutionStageSnapshot{stageFor(1, nil, snapshot.MCPServers, snapshot.Skills, snapshot.CLIConnectors)}, nil
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
	Compatibility     string   `json:"compatibility"`
	// Populated only after the Worker resolves ConnectionID + ConnectionVersion.
	APIKeyCiphertext  []byte `json:"-"`
	CredentialOwnerID string `json:"-"`
}

type ExpertSnapshot struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	Icon                   string   `json:"icon,omitempty"`
	IconBackground         string   `json:"icon_background,omitempty"`
	Introduction           string   `json:"introduction,omitempty"`
	CoreCapability         string   `json:"core_capability,omitempty"`
	OperatingProcedure     string   `json:"operating_procedure,omitempty"`
	OutputStandard         string   `json:"output_standard,omitempty"`
	Cautions               string   `json:"cautions,omitempty"`
	CapabilityIntroduction string   `json:"capability_introduction"`
	ExecutionInstruction   string   `json:"execution_instruction"`
	ExpertiseTags          []string `json:"expertise_tags"`
	Version                int64    `json:"version"`
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
	Position      int                    `json:"position"`
	MemberID      string                 `json:"member_id,omitempty"`
	MemberName    string                 `json:"member_name,omitempty"`
	Labels        []string               `json:"labels,omitempty"`
	MCPServers    []MCPServerSnapshot    `json:"mcp_servers"`
	Skills        []SkillSnapshot        `json:"skills"`
	CLIConnectors []CLIConnectorSnapshot `json:"cli_connectors"`
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

// CLIConnectorSnapshot freezes the exact verified artifact and policy used by a Run.
type CLIConnectorSnapshot struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Executable           string          `json:"executable"`
	AuthenticationDriver string          `json:"authentication_driver"`
	BundleObjectKey      string          `json:"bundle_object_key"`
	BundleSHA256         string          `json:"bundle_sha256"`
	RuntimeDigests       []string        `json:"runtime_digests"`
	Capabilities         json.RawMessage `json:"capabilities"`
	Version              int64           `json:"version"`
}
