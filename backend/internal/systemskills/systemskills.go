package systemskills

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	CreateSkillKey     = "system.create_skill"
	CreateExpertKey    = "system.create_expert"
	CreateConnectorKey = "system.create_connector"
)

type Definition struct {
	Key         string
	Name        string
	Description string
	Document    string
}

var definitions = []Definition{
	{Key: CreateSkillKey, Name: "Create Skill", Description: "Create and install a reusable Skill through a confirmed platform action.", Document: `---
display_name: Create Skill
---
# Create Skill

Help the User design a reusable Skill through conversation. Clarify the goal, trigger, inputs, outputs, workflow, constraints, examples, and whether the User wants to generate a new package or install an explicitly supplied Git URL or ZIP.

Never claim that a Skill was installed until the platform confirms the User's action. After the requirements are complete, include exactly one machine-readable proposal marker and ask the User to confirm it. The marker JSON must use kind "skill" and include skill.name, skill.source ("generated" or "git"), skill.document for generated Skills, or skill.git_url for Git Skills. Example: <platform-action>{"kind":"skill","user_message":"预览已准备好，请确认。","skill":{"name":"Example","description":"...","source":"generated","document":"---\ndisplay_name: Example\n---\n# Example\n..."}}</platform-action>. Do not invent platform ownership, resource IDs, or hidden files.
`},
	{Key: CreateExpertKey, Name: "Create Expert", Description: "Create a reusable Expert through a confirmed platform action.", Document: `---
display_name: Create Expert
---
# Create Expert

Help the User create an Expert through conversation. Collect a name, Introduction, Core Capability, Operating Procedure, Output Standard, optional Cautions, and any Skills or Connectors the User explicitly wants to bind.

Do not infer resource bindings from the current conversation. Keep asking until all required fields are complete, then include exactly one machine-readable proposal marker and ask the User to confirm it. The marker JSON must use kind "expert" and include expert.name, introduction, core_capability, operating_procedure, output_standard, and only explicitly selected skill_ids, mcp_server_ids, or cli_connector_definition_ids. Example: <platform-action>{"kind":"expert","user_message":"预览已准备好，请确认。","expert":{"name":"Example","introduction":"...","core_capability":"...","operating_procedure":"...","output_standard":"...","skill_ids":[]}}</platform-action>. Never claim that an Expert was created until the platform confirms the User's action.
`},
	{Key: CreateConnectorKey, Name: "Create Connector", Description: "Create and install a private MCP Connector Package with a companion SKILL.md through a confirmed platform action.", Document: `---
display_name: Create Connector
---
# Create Connector

Help the User create a Connector in this Agent Workspace. Collect the real integration endpoint or exact-version package, intended operations, authentication mode, and concise instructions. Never invent an endpoint, package version, credentials, or working capabilities. Read the provider's official documentation when necessary.

For a private MCP Connector, prepare a Connector Package with a source slug (lowercase letters, digits, hyphens), semantic version, display name, description, auth_mode (none or oauth), a valid mcp.json, and a companion SKILL.md. The mcp.json must declare either streamable_http with a real HTTPS URL, timeout_seconds and egress_hosts, or stdio with runner npx/uvx, package, exact package_version, timeout_seconds and egress_hosts. Do not embed secrets in URLs, headers, or instructions. The SKILL.md should tell the runtime when to use the Connector and its exact supported workflow. Check that all required information is known before proposing creation.

When the MCP package is ready, include exactly one marker and ask the User to confirm. Use kind "connector" and fields connector.source, version, name, description, auth_mode, mcp_json (a JSON string), skill_name and skill_markdown. Example: <platform-action>{"kind":"connector","user_message":"连接器预览已准备好，请确认创建。","connector":{"source":"example-service","version":"1.0.0","name":"Example Service","description":"Query Example Service","auth_mode":"none","mcp_json":"{\"transport\":\"streamable_http\",\"url\":\"https://mcp.example.com/tools\",\"timeout_seconds\":30,\"egress_hosts\":[\"mcp.example.com\"]}","skill_name":"example-service","skill_markdown":"# Example Service\nUse this Connector to query Example Service."}}</platform-action>. The platform validates the package and installs it for the current User after confirmation; never claim success before the platform confirms it.

A CLI Connector needs an actual immutable executable bundle, exact Runtime RepoDigest, reviewed command and capability policy, and Conformance evidence. This guided conversation action cannot synthesize or publish those artifacts. If the User asks for a CLI Connector, explain what is missing and help prepare a validated CLI package for the existing upload and review flow; do not emit a connector action marker for an unverified CLI.
`},
}

func Definitions() []Definition { return append([]Definition(nil), definitions...) }

func DefinitionByKey(key string) (Definition, bool) {
	for _, definition := range definitions {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

func Archive(key string) ([]byte, string, error) {
	definition, ok := DefinitionByKey(strings.TrimSpace(key))
	if !ok {
		return nil, "", fmt.Errorf("unknown system Skill %q", key)
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("SKILL.md")
	if err != nil {
		return nil, "", err
	}
	if _, err := entry.Write([]byte(definition.Document)); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(buffer.Bytes())
	return buffer.Bytes(), hex.EncodeToString(sum[:]), nil
}
