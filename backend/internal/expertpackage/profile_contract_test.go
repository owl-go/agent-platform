package expertpackage

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestImportedZIPUsesCurrentExpertProfileContract(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(map[string]any, map[string]string)
		valid  bool
	}{
		{name: "current Markdown profile", valid: true},
		{name: "missing capability description", change: func(profile map[string]any, _ map[string]string) { delete(profile, "introduction") }},
		{name: "missing guidance reference", change: func(profile map[string]any, _ map[string]string) { delete(profile, "guidance_file") }},
		{name: "blank guidance", change: func(_ map[string]any, files map[string]string) { files["agents/reviewer.md"] = " \n " }},
		{name: "legacy form instructions", change: func(profile map[string]any, _ map[string]string) { profile["operating_procedure"] = "Review" }},
		{name: "automatic tags", change: func(profile map[string]any, _ map[string]string) { profile["expertise_tags"] = []string{"review"} }},
		{name: "category", change: func(profile map[string]any, _ map[string]string) { profile["category"] = "review" }},
		{name: "credentials", change: func(profile map[string]any, _ map[string]string) { profile["api_key"] = "sample" }},
		{name: "execution settings", change: func(profile map[string]any, _ map[string]string) { profile["runtime_engine"] = "example" }},
		{name: "four common tasks", change: func(profile map[string]any, _ map[string]string) {
			profile["starter_prompts"] = []string{"One", "Two", "Three", "Four"}
		}},
		{name: "blank common task", change: func(profile map[string]any, _ map[string]string) { profile["starter_prompts"] = []string{" "} }},
		{name: "unreferenced instruction", change: func(_ map[string]any, files map[string]string) { files["agents/hidden.md"] = "hidden" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := map[string]any{"name": "Reviewer", "introduction": "Review evidence", "guidance_file": "agents/reviewer.md", "starter_prompts": []string{"Review this proposal", "Check evidence", "Find risks"}}
			files := map[string]string{"agents/reviewer.md": "# Review\nCheck supplied evidence and report findings."}
			if test.change != nil {
				test.change(profile, files)
			}
			metadata, err := json.Marshal(map[string]any{"schema_version": 1, "id": "example.reviewer", "version": "1.0.0", "kind": "expert", "expert": profile})
			if err != nil {
				t.Fatal(err)
			}
			files[".plugin/plugin.json"] = string(metadata)
			var buffer bytes.Buffer
			writer := zip.NewWriter(&buffer)
			for name, content := range files {
				file, err := writer.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.Write([]byte(content)); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			pkg, err := Parse(context.Background(), buffer.Bytes())
			if test.valid {
				if err != nil || len(pkg.Expert.StarterPrompts) != 3 || pkg.Expert.Guidance != files["agents/reviewer.md"] {
					t.Fatalf("profile not preserved: %v", err)
				}
			} else if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("want invalid package, got %v", err)
			}
		})
	}
}
