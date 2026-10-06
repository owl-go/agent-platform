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
	writeResourceFixture(t, root, "experts/example/expert.json", `{"key":"local.expert.example","version":"1.0.0","name":"Example Expert","icon":"sparkles","icon_background":"sage","introduction":"Fixture","core_capability":"Fixture","operating_procedure":"Fixture","output_standard":"Fixture","skill_keys":["local.skill.example"]}`)
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
				file := filepath.Join(root, "experts/example/expert.json")
				body, _ := os.ReadFile(file)
				if problem == "unknown Skill reference" {
					body = bytes.ReplaceAll(body, []byte("local.skill.example"), []byte("missing"))
				}
				if problem == "incomplete Expert" {
					body = bytes.ReplaceAll(body, []byte(`"core_capability":"Fixture"`), []byte(`"core_capability":""`))
				}
				if problem == "unknown JSON field" {
					body = bytes.Replace(body, []byte("{"), []byte(`{"credential":"forbidden",`), 1)
				}
				writeResourceFixture(t, root, "experts/example/expert.json", string(body))
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
	for _, name := range []string{"resources.json", "skills/example/resource.json", "experts/example/expert.json", "connectors/ai-hive/package", "connectors/ai-hive/package/mcp.json", "skills"} {
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
	file := filepath.Join(root, "experts/example/expert.json")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	writeResourceFixture(t, root, "experts/example/expert.json", strings.ReplaceAll(string(body), "local.skill.example", "system.create_skill"))
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
