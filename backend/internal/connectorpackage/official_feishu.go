package connectorpackage

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// OfficialFeishuInput is populated only from the reviewed @larksuite/cli build
// result after its exact npm integrity and Runtime Conformance have passed.
type OfficialFeishuInput struct {
	Version        string
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
	metadata, _ := json.Marshal(Metadata{Source: "feishu", Version: input.Version, Type: TypeCLI, Name: "飞书", Description: "读取、检索并操作飞书文档、日历、消息及其他开放平台资源。", ExamplesZH: []string{"查询飞书文档", "发送飞书消息"}, ExamplesEN: []string{"Search Feishu documents", "Send a Feishu message"}, MinPlatformVersion: "1.0.0", AuthMode: "oauth"})
	manifest, _ := json.Marshal(CLIManifest{
		Runtime: ManagedRuntime{Kind: "node", Version: input.RuntimeVersion, Digest: input.RuntimeDigest}, Executable: "lark-cli", BundlePath: input.BundlePath, AuthenticationDriver: "feishu",
		Commands:    CLICommands{Init: LifecycleCommand{Argv: []string{"app", "status", "--output", "json"}}, Auth: LifecycleCommand{Argv: []string{"auth", "login", "--output", "json"}}, Status: LifecycleCommand{Argv: []string{"auth", "status", "--output", "json"}}, UnAuth: LifecycleCommand{Argv: []string{"auth", "logout", "--output", "json"}}},
		StatusMatch: StatusMatch{JSONPath: "$.authenticated", Equals: true}, Capabilities: input.Capabilities, AuthURLDomains: []string{"accounts.feishu.cn", "open.feishu.cn"}, EgressHosts: []string{"accounts.feishu.cn", "open.feishu.cn"}, TimeoutSeconds: 60,
		Limits: ResourceLimits{CPU: 1000, MemoryMiB: 1024, TimeoutSeconds: 900, Concurrency: 1, ChildProcesses: 64},
	})
	files := map[string][]byte{
		"connector-meta.json":    metadata,
		"icon.svg":               []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#3370ff" d="M4 4h16v16H4z"/></svg>`),
		"cli.json":               manifest,
		"cli-bundle.tgz":         input.Bundle,
		"skills/feishu/SKILL.md": []byte("---\nname: feishu\ndisplay_name: 飞书\ndescription: 使用经过审核的飞书能力\nversion: " + input.Version + "\nauthor: Agent Workspace\n---\n\n仅使用连接器公开的结构化能力访问飞书。\n"),
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
