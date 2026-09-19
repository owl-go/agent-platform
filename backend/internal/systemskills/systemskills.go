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

Never claim that a Skill was installed until the platform confirms the User's action. Produce a structured creation proposal for the platform and ask the User to confirm it. Do not invent platform ownership, resource IDs, or hidden files.
`},
	{Key: CreateExpertKey, Name: "Create Expert", Description: "Create a reusable Expert through a confirmed platform action.", Document: `---
display_name: Create Expert
---
# Create Expert

Help the User create an Expert through conversation. Collect a name, Introduction, Core Capability, Operating Procedure, Output Standard, optional Cautions, and any Skills or Connectors the User explicitly wants to bind.

Do not infer resource bindings from the current conversation. Keep asking until all required fields are complete, then produce a structured creation proposal and ask the User to confirm it. Never claim that an Expert was created until the platform confirms the User's action.
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
