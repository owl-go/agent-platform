package defaultresources

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeResourceFixture(t *testing.T, root, name, body string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func directoryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeResourceFixture(t, root, "resources.json", `{"version":"1.0.0"}`)
	writeResourceFixture(t, root, "skills/example/resource.json", `{"key":"local.skill.example","version":"1.0.0","icon":"sparkles"}`)
	writeResourceFixture(t, root, "skills/example/SKILL.md", "---\nname: example\ndisplay_name: Example Skill\ndescription: Use for the example fixture.\n---\n\n# Example\n\nRead the supplied input and explain the result.\n")
	writeResourceFixture(t, root, "experts/example/.plugin/plugin.json", `{"schema_version":1,"id":"local.expert.example","version":"1.0.0","kind":"expert","expert":{"name":"Example Expert","icon":"sparkles","icon_background":"sage","introduction":"Fixture","guidance_file":"agents/expert.md","skill_keys":["local.skill.example"]}}`)
	writeResourceFixture(t, root, "experts/example/agents/expert.md", "# Fixture\n")
	for _, name := range []string{"connector-meta.json", "icon.svg", "mcp.json", "skills/ai-hive/SKILL.md"} {
		data, err := os.ReadFile(filepath.Join("../../../connectors/ai-hive/package", name))
		if err != nil {
			t.Fatal(err)
		}
		writeResourceFixture(t, root, "connectors/ai-hive/package/"+name, string(data))
	}
	return root
}

func TestDirectoryDiscoveryDoesNotLoadBuildScriptsAndFreezesBytes(t *testing.T) {
	root := directoryFixture(t)
	writeResourceFixture(t, root, "scripts/connectors/ai-hive/connector-meta.json", `{"source":"duplicate"}`)
	writeResourceFixture(t, root, "connectors/ai-hive/build.py", "not part of the package")
	catalog, err := LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Connectors) != 1 || len(catalog.Skills) != 1 || len(catalog.Experts) != 1 {
		t.Fatal("did not discover exactly one resource of each kind")
	}
	pkg, err := catalog.ParseConnector(catalog.Connectors[0])
	if err != nil || len(pkg.Skills) != 1 {
		t.Fatal("directory did not use the real Connector Package validator", err)
	}
	before, err := catalog.Archive(catalog.Skills[0].Archive)
	if err != nil {
		t.Fatal(err)
	}
	writeResourceFixture(t, root, "skills/example/SKILL.md", "changed after load")
	after, err := catalog.Archive(catalog.Skills[0].Archive)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("loaded catalog reread mutable files", err)
	}
	after[0] ^= 1
	again, _ := catalog.Archive(catalog.Skills[0].Archive)
	if !bytes.Equal(before, again) {
		t.Fatal("caller could mutate frozen package bytes")
	}
}

func TestDirectoryRejectsInvalidDefinitionsBeforeInstallation(t *testing.T) {
	for _, problem := range []string{"duplicate key", "duplicate name", "unknown Skill reference", "missing Skill document", "unknown JSON field", "incomplete Expert", "reserved Skill key", "reserved Skill name", "invalid version", "source alias", "both Connector modes"} {
		t.Run(problem, func(t *testing.T) {
			root := directoryFixture(t)
			switch problem {
			case "duplicate key", "duplicate name":
				meta, _ := os.ReadFile(filepath.Join(root, "skills/example/resource.json"))
				if problem == "duplicate name" {
					meta = bytes.ReplaceAll(meta, []byte("local.skill.example"), []byte("local.skill.other"))
				}
				body, _ := os.ReadFile(filepath.Join(root, "skills/example/SKILL.md"))
				writeResourceFixture(t, root, "skills/other/resource.json", string(meta))
				writeResourceFixture(t, root, "skills/other/SKILL.md", string(body))
			case "unknown Skill reference", "incomplete Expert", "unknown JSON field":
				file := filepath.Join(root, "experts/example/.plugin/plugin.json")
				body, _ := os.ReadFile(file)
				if problem == "unknown Skill reference" {
					body = bytes.ReplaceAll(body, []byte("local.skill.example"), []byte("missing"))
				}
				if problem == "incomplete Expert" {
					body = bytes.ReplaceAll(body, []byte(`"introduction":"Fixture"`), []byte(`"introduction":""`))
				}
				if problem == "unknown JSON field" {
					body = bytes.Replace(body, []byte("{"), []byte(`{"credential":"forbidden",`), 1)
				}
				writeResourceFixture(t, root, "experts/example/.plugin/plugin.json", string(body))
			case "missing Skill document":
				os.Remove(filepath.Join(root, "skills/example/SKILL.md"))
			case "reserved Skill key":
				writeResourceFixture(t, root, "skills/example/resource.json", `{"key":"system.create_skill","version":"1.0.0"}`)
			case "reserved Skill name":
				file := filepath.Join(root, "skills/example/SKILL.md")
				body, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				writeResourceFixture(t, root, "skills/example/SKILL.md", strings.ReplaceAll(string(body), "Example Skill", "Create Skill"))
			case "invalid version":
				writeResourceFixture(t, root, "skills/example/resource.json", `{"key":"local.skill.example","version":"latest"}`)
			case "source alias":
				if err := os.Rename(filepath.Join(root, "connectors/ai-hive"), filepath.Join(root, "connectors/alias")); err != nil {
					t.Fatal(err)
				}
			case "both Connector modes":
				writeResourceFixture(t, root, "connectors/ai-hive/package/cli.json", `{}`)
			}
			if _, err := LoadDirectory(context.Background(), root); err == nil {
				t.Fatal("invalid directory was accepted")
			}
		})
	}
}

func TestDirectoryRejectsSymlinksIncludingMetadataAndPackageRoots(t *testing.T) {
	for _, name := range []string{"resources.json", "skills/example/resource.json", "experts/example/.plugin/plugin.json", "connectors/ai-hive/package", "connectors/ai-hive/package/mcp.json", "skills"} {
		t.Run(name, func(t *testing.T) {
			root := directoryFixture(t)
			target := filepath.Join(root, filepath.FromSlash(name))
			if err := os.Rename(target, target+".real"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target+".real", target); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadDirectory(context.Background(), root); err == nil {
				t.Fatal("in-root symlink accepted")
			}
		})
	}
}

func TestExplicitDirectoryNeverFallsBackAndCancellationIsPropagated(t *testing.T) {
	t.Setenv(RootEnvironment, filepath.Join(t.TempDir(), "missing"))
	if _, err := Load(); err == nil {
		t.Fatal("invalid explicit root fell back to repository resources")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LoadDirectory(ctx, directoryFixture(t)); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation not propagated", err)
	}
}

func TestExpertMayReferenceBuiltInCreationSkill(t *testing.T) {
	root := directoryFixture(t)
	file := filepath.Join(root, "experts/example/.plugin/plugin.json")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	writeResourceFixture(t, root, "experts/example/.plugin/plugin.json", strings.ReplaceAll(string(body), "local.skill.example", "system.create_skill"))
	if _, err := LoadDirectory(context.Background(), root); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryDigestIgnoresGitArchiveWriteBitsButRetainsExecutability(t *testing.T) {
	root := directoryFixture(t)
	file := filepath.Join(root, "skills/example/script.sh")
	writeResourceFixture(t, root, "skills/example/script.sh", "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(file, 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o775); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "skills/example/SKILL.md"), 0o664); err != nil {
		t.Fatal(err)
	}
	after, err := LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if before.Skills[0].SHA256 != after.Skills[0].SHA256 {
		t.Fatal("Git archive write bits changed the immutable Skill identity")
	}
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	nonExecutable, err := LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if nonExecutable.Skills[0].SHA256 == before.Skills[0].SHA256 {
		t.Fatal("script executability was discarded")
	}
}

func TestDirectoryDiscoversNeutralExpertAndTeamPackages(t *testing.T) {
	root := directoryFixture(t)
	writeResourceFixture(t, root, "experts/portable/.plugin/plugin.json", `{"schema_version":1,"id":"example.portable","version":"1.0.0","kind":"expert","expert":{"name":"Portable","introduction":"Review evidence","guidance_file":"agents/reviewer.md"}}`)
	writeResourceFixture(t, root, "experts/portable/agents/reviewer.md", "# Review\n\nPreserve whitespace.\n")
	writeResourceFixture(t, root, "experts/team/.plugin/plugin.json", `{"schema_version":1,"id":"example.team","version":"1.0.0","kind":"expert_team","team":{"name":"Portable Team","introduction":"Review evidence","core_capability":"Review and synthesize","lead_member_id":"lead","members":[{"id":"lead","name":"Lead","expert":{"name":"Lead","introduction":"Coordinate","guidance_file":"agents/lead.md"}},{"id":"reviewer","name":"Reviewer","expert":{"name":"Reviewer","introduction":"Review","guidance_file":"agents/reviewer.md"}}]}}`)
	writeResourceFixture(t, root, "experts/team/agents/lead.md", "# Coordinate\n")
	writeResourceFixture(t, root, "experts/team/agents/reviewer.md", "# Review\n")
	catalog, err := LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Experts) != 2 || len(catalog.Teams) != 1 {
		t.Fatalf("packages not discovered: Experts=%d Teams=%d", len(catalog.Experts), len(catalog.Teams))
	}
	writeResourceFixture(t, root, "experts/portable/agents/reviewer.md", "changed after load")
	if catalog.Experts[1].Guidance != "# Review\n\nPreserve whitespace.\n" {
		t.Fatal("guidance was not frozen")
	}
	if catalog.Teams[0].Definition.LeadMemberID != "lead" || len(catalog.Teams[0].Definition.Members) != 2 {
		t.Fatal("team lead or owned roster lost")
	}
}
