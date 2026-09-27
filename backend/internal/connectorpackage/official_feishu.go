package connectorpackage

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

//go:embed skills/feishu/SKILL.md
var officialFeishuSkillTemplate string

//go:embed skills/feishu/upstream-v1.0.93.tar.gz
var officialFeishuReferences []byte

const officialFeishuReferencesSHA256 = "a334fccef99e02628c631a8373fa5ed1fbe3ef8c459bc457d57b267cc29bc24b"

func OfficialFeishuSkill(version string) []byte {
	return []byte(strings.ReplaceAll(officialFeishuSkillTemplate, "{{VERSION}}", version))
}

// OfficialFeishuSkillResources provides the pinned official CLI references
// under the Connector Skill directory. They document command syntax; the
// reviewed manifest remains the sole execution policy.
func OfficialFeishuSkillResources(version string) (map[string][]byte, error) {
	if version != "1.0.93" {
		return nil, fmt.Errorf("Feishu Skill references are not reviewed for CLI version %q", version)
	}
	digest := sha256.Sum256(officialFeishuReferences)
	if fmt.Sprintf("%x", digest) != officialFeishuReferencesSHA256 {
		return nil, fmt.Errorf("Feishu Skill references have changed without review")
	}
	compressed, err := gzip.NewReader(bytes.NewReader(officialFeishuReferences))
	if err != nil {
		return nil, fmt.Errorf("open Feishu Skill references: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	files := map[string][]byte{"SKILL.md": OfficialFeishuSkill(version)}
	var total int64
	for {
		entry, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read Feishu Skill references: %w", err)
		}
		name := path.Clean(entry.Name)
		if entry.Typeflag != tar.TypeReg || name != entry.Name || !strings.HasPrefix(name, "references/") || strings.ContainsAny(name, "\\\x00\r\n") || entry.Size < 0 || entry.Size > 1<<20 {
			return nil, fmt.Errorf("Feishu Skill reference has an unsafe entry")
		}
		if _, exists := files[name]; exists {
			return nil, fmt.Errorf("Feishu Skill reference has a duplicate entry")
		}
		total += entry.Size
		if total > 10<<20 || len(files) > 600 {
			return nil, fmt.Errorf("Feishu Skill references exceed their bounds")
		}
		body, err := io.ReadAll(io.LimitReader(reader, entry.Size+1))
		if err != nil {
			return nil, fmt.Errorf("read Feishu Skill reference %q: %w", name, err)
		}
		if int64(len(body)) != entry.Size {
			return nil, fmt.Errorf("Feishu Skill reference %q has an invalid size", name)
		}
		files[name] = body
	}
	return files, nil
}

// OfficialFeishuInput is populated only from the reviewed @larksuite/cli build
// result after its exact npm integrity and Runtime Conformance have passed.
type OfficialFeishuInput struct {
	Version        string // Reviewed @larksuite/cli version and Skill reference version.
	PackageVersion string // Connector package revision; defaults to Version.
	Bundle         []byte
	BundlePath     string
	RuntimeVersion string
	RuntimeDigest  string
	Capabilities   []CLICapability
}

func BuildOfficialFeishuArchive(input OfficialFeishuInput) ([]byte, error) {
	if len(input.Bundle) == 0 || input.BundlePath == "" || input.RuntimeVersion == "" || input.RuntimeDigest == "" || len(input.Capabilities) == 0 {
		return nil, fmt.Errorf("official Feishu package requires the exact bundle, Runtime digest, and reviewed capabilities")
	}
	examplesZH, examplesEN := []string{}, []string{}
	for _, capability := range input.Capabilities {
		switch capability.ID {
		case "im_chat_search":
			examplesZH, examplesEN = append(examplesZH, "搜索飞书群聊"), append(examplesEN, "Search Feishu chats")
		case "im_messages_send":
			examplesZH, examplesEN = append(examplesZH, "发送飞书消息"), append(examplesEN, "Send a Feishu message")
		case "task_create":
			examplesZH, examplesEN = append(examplesZH, "创建飞书任务"), append(examplesEN, "Create a Feishu task")
		}
	}
	packageVersion := input.PackageVersion
	if packageVersion == "" {
		packageVersion = input.Version
	}
	metadata, _ := json.Marshal(Metadata{Source: "feishu", Version: packageVersion, Type: TypeCLI, Name: "飞书", Description: "使用已审核的 CLI 能力操作飞书资源。", ExamplesZH: examplesZH, ExamplesEN: examplesEN, MinPlatformVersion: "1.0.0", AuthMode: "oauth"})
	manifest, _ := json.Marshal(CLIManifest{
		Runtime: ManagedRuntime{Kind: "node", Version: input.RuntimeVersion, Digest: input.RuntimeDigest}, Executable: "lark-cli", BundlePath: input.BundlePath, AuthenticationDriver: "feishu",
		Commands:    CLICommands{Init: LifecycleCommand{Argv: []string{"app", "status", "--output", "json"}}, Auth: LifecycleCommand{Argv: []string{"auth", "login", "--output", "json"}}, Status: LifecycleCommand{Argv: []string{"auth", "status", "--output", "json"}}, UnAuth: LifecycleCommand{Argv: []string{"auth", "logout", "--output", "json"}}},
		StatusMatch: StatusMatch{JSONPath: "$.authenticated", Equals: true}, Capabilities: input.Capabilities, AuthURLDomains: []string{"accounts.feishu.cn", "open.feishu.cn"}, EgressHosts: []string{"accounts.feishu.cn", "open.feishu.cn"}, TimeoutSeconds: 60,
		Limits: ResourceLimits{CPU: 1000, MemoryMiB: 1024, TimeoutSeconds: 900, Concurrency: 1, ChildProcesses: 64},
	})
	resources, err := OfficialFeishuSkillResources(input.Version)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"connector-meta.json": metadata,
		"icon.svg":            []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#3370ff" d="M4 4h16v16H4z"/></svg>`),
		"cli.json":            manifest,
		"cli-bundle.tgz":      input.Bundle,
	}
	for name, body := range resources {
		files["skills/feishu/"+name] = body
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	pkg, err := Parse(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("validate official Feishu package: %w", err)
	}
	return pkg.NormalizedArchive, nil
}
