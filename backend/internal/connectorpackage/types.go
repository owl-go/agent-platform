package connectorpackage

type Type string

const (
	TypeMCP Type = "mcp"
	TypeCLI Type = "cli"
)

type Package struct {
	Metadata          Metadata
	MCP               *MCPManifest
	CLI               *CLIManifest
	CLIBundle         []byte
	CLIBundleSHA256   string
	Skills            []Skill
	SHA256            string
	NormalizedArchive []byte
}

type Metadata struct {
	Source             string   `json:"source"`
	Version            string   `json:"version"`
	Type               Type     `json:"type"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	ExamplesZH         []string `json:"examples_zh"`
	ExamplesEN         []string `json:"examples_en"`
	MinPlatformVersion string   `json:"minPlatformVersion"`
	MaxPlatformVersion string   `json:"maxPlatformVersion,omitempty"`
	AuthMode           string   `json:"auth_mode"`
}

type Skill struct {
	Directory   string
	Name        string
	DisplayName string
	Description string
	Version     string
	Author      string
}

type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ResourceLimits struct {
	CPU            int `json:"cpu_millis"`
	MemoryMiB      int `json:"memory_mib"`
	TimeoutSeconds int `json:"timeout_seconds"`
	Concurrency    int `json:"concurrency"`
	ChildProcesses int `json:"child_processes"`
}

type MCPManifest struct {
	Transport      string                `json:"transport"`
	URL            string                `json:"url,omitempty"`
	Runner         string                `json:"runner,omitempty"`
	Package        string                `json:"package,omitempty"`
	PackageVersion string                `json:"package_version,omitempty"`
	Arguments      []string              `json:"arguments,omitempty"`
	Environment    []EnvironmentVariable `json:"environment,omitempty"`
	Headers        map[string]string     `json:"headers,omitempty"`
	WorkingDir     string                `json:"working_directory,omitempty"`
	DisabledTools  []string              `json:"disabled_tools,omitempty"`
	EgressHosts    []string              `json:"egress_hosts"`
	TimeoutSeconds int                   `json:"timeout_seconds"`
	Limits         ResourceLimits        `json:"resource_limits,omitempty"`
}

type ManagedRuntime struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type LifecycleCommand struct {
	Argv           []string `json:"argv"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

type CLICommands struct {
	Init   LifecycleCommand `json:"init"`
	Auth   LifecycleCommand `json:"auth"`
	Status LifecycleCommand `json:"status"`
	UnAuth LifecycleCommand `json:"unAuth"`
}

type StatusMatch struct {
	JSONPath string `json:"json_path"`
	Equals   any    `json:"equals"`
}

type CLIManifest struct {
	Runtime              ManagedRuntime        `json:"runtime"`
	Executable           string                `json:"executable"`
	BundlePath           string                `json:"bundle_path,omitempty"`
	AuthenticationDriver string                `json:"authentication_driver,omitempty"`
	Commands             CLICommands           `json:"commands"`
	StatusMatch          StatusMatch           `json:"status_match"`
	Capabilities         []CLICapability       `json:"capabilities,omitempty"`
	AuthURLDomains       []string              `json:"auth_url_domains,omitempty"`
	Environment          []EnvironmentVariable `json:"environment,omitempty"`
	EgressHosts          []string              `json:"egress_hosts"`
	TimeoutSeconds       int                   `json:"timeout_seconds"`
	Limits               ResourceLimits        `json:"resource_limits,omitempty"`
}

type CLICapability struct {
	ID             string   `json:"id"`
	ArgvPrefix     []string `json:"argv_prefix"`
	Risk           string   `json:"risk"`
	Identities     []string `json:"identities"`
	Scopes         []string `json:"scopes,omitempty"`
	EgressHosts    []string `json:"egress_hosts"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}
