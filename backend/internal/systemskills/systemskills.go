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
	CreateSkillKey  = "system.create_skill"
	CreateExpertKey = "system.create_expert"
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
