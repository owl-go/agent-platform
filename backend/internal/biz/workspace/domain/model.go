package domain

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("resource not found")
	ErrConflict = errors.New("resource conflicts with current state")
	ErrInvalid  = errors.New("resource is invalid")
)

type RuntimeEngine string

const (
	RuntimeClaude   RuntimeEngine = "claude"
	RuntimeCodex    RuntimeEngine = "codex"
	RuntimeHermes   RuntimeEngine = "hermes"
	RuntimeOpenClaw RuntimeEngine = "openclaw"
)

func ParseRuntime(value string) (RuntimeEngine, error) {
	runtime := RuntimeEngine(strings.TrimSpace(value))
	switch runtime {
	case RuntimeClaude, RuntimeCodex, RuntimeHermes, RuntimeOpenClaw:
		return runtime, nil
	default:
		return "", fmt.Errorf("%w: unsupported Runtime Engine", ErrInvalid)
	}
}

type Session struct {
	ID                     string
	OwnerID                string
	Title                  string
	ExpertID               *string
	CurrentProviderModelID *string
	ArchivedAt             *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
}

type Message struct {
	ID               int64
	SessionID        string
	Role             string
	State            string
	Content          string
	Error            string
	ProgressStage    string
	ElapsedMS        int64
	CreatedAt        time.Time
	ResponseSnapshot *ResponseSnapshot
}

type ResponseSnapshot struct {
	ProviderModelID   string        `json:"provider_model_id"`
	ConnectionID      string        `json:"connection_id"`
	ConnectionName    string        `json:"connection_name"`
	ProviderType      string        `json:"provider_type"`
	ModelID           string        `json:"model_id"`
	ModelName         string        `json:"model_name"`
	Endpoint          string        `json:"endpoint"`
	Protocols         []string      `json:"protocols"`
	RuntimeEngine     RuntimeEngine `json:"runtime_engine"`
	Compatibility     string        `json:"compatibility"`
	ConnectionVersion int64         `json:"connection_version"`
}

type MCPServer struct {
	ID              string
	OwnerID         string
	Name            string
	Transport       string
	URL             *string
	Runner          *string
	Package         *string
	PackageVersion  *string
	Arguments       []string
	Environment     []EnvironmentVariable
	TestRequestedAt *time.Time
	TestedAt        *time.Time
	TestError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Version         int64
}

func (server MCPServer) Validate() error {
	if name := strings.TrimSpace(server.Name); name == "" || len(name) > 100 {
		return fmt.Errorf("%w: MCP Server name must contain 1-100 characters", ErrInvalid)
	}
	switch server.Transport {
	case "streamable_http":
		if server.URL == nil {
			return fmt.Errorf("%w: Streamable HTTP URL is required", ErrInvalid)
		}
		parsed, err := url.Parse(strings.TrimSpace(*server.URL))
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("%w: MCP Server URL must use HTTPS", ErrInvalid)
		}
		if len(server.Environment) > 1 || len(server.Environment) == 1 && (server.Environment[0].Name != "MCP_BEARER_TOKEN" || !server.Environment[0].Secret) {
			return fmt.Errorf("%w: HTTP MCP authentication must use one optional Secret MCP_BEARER_TOKEN", ErrInvalid)
		}
	case "stdio":
		if server.Runner == nil || server.Package == nil || server.PackageVersion == nil {
			return fmt.Errorf("%w: stdio runner, package, and fixed version are required", ErrInvalid)
		}
		if *server.Runner != "npx" && *server.Runner != "uvx" {
			return fmt.Errorf("%w: stdio runner must be npx or uvx", ErrInvalid)
		}
		if strings.TrimSpace(*server.Package) == "" || strings.TrimSpace(*server.PackageVersion) == "" || strings.EqualFold(strings.TrimSpace(*server.PackageVersion), "latest") {
			return fmt.Errorf("%w: MCP package requires a fixed version", ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: unsupported MCP transport", ErrInvalid)
	}
	if len(server.Arguments) > 100 {
		return fmt.Errorf("%w: too many MCP arguments", ErrInvalid)
	}
	for _, argument := range server.Arguments {
		if strings.ContainsRune(argument, '\x00') || len(argument) > 4_096 {
			return fmt.Errorf("%w: invalid MCP argument", ErrInvalid)
		}
	}
	return ValidateEnvironment(server.Environment)
}

type Skill struct {
	ID        string
	OwnerID   string
	Name      string
	Source    string
	GitURL    *string
	GitRef    *string
	ObjectKey string
	SHA256    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int64
}

type Artifact struct {
	ID          string
	RunID       string
	Kind        string
	Name        string
	Path        string
	Size        int64
	SHA256      string
	TextPreview string
	ObjectKey   string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}

type RunEvent struct {
	Sequence   int64
	Type       string
	Payload    []byte
	OccurredAt time.Time
}

type WorkspaceEntry struct {
	Path       string
	Name       string
	Directory  bool
	Size       int64
	ModifiedAt time.Time
}

type EnvironmentVariable struct {
	Name       string
	Value      string
	Secret     bool
	Configured bool
}

var environmentName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)

func ValidateEnvironment(values []EnvironmentVariable) error {
	if len(values) > 100 {
		return fmt.Errorf("%w: at most 100 environment variables are allowed", ErrInvalid)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !environmentName.MatchString(value.Name) {
			return fmt.Errorf("%w: invalid environment variable %q", ErrInvalid, value.Name)
		}
		if _, exists := seen[value.Name]; exists {
			return fmt.Errorf("%w: duplicate environment variable %q", ErrInvalid, value.Name)
		}
		if reservedEnvironmentName(value.Name) {
			return fmt.Errorf("%w: environment variable %q is reserved by the Runtime sandbox", ErrInvalid, value.Name)
		}
		seen[value.Name] = struct{}{}
		if len(value.Value) > 64*1024 {
			return fmt.Errorf("%w: environment value is too large", ErrInvalid)
		}
	}
	return nil
}

func reservedEnvironmentName(name string) bool {
	switch name {
	case "HOME", "PATH", "SHELL", "ENV", "BASH_ENV", "PROMPT_COMMAND", "CODEX_HOME", "OPENCLAW_CONFIG_PATH", "GIT_SSH", "GIT_SSH_COMMAND", "NODE_OPTIONS", "PYTHONHOME", "PYTHONPATH":
		return true
	}
	for _, prefix := range []string{"AGENT_PLATFORM_", "AGENT_WORKSPACE_", "GIT_CONFIG_", "LD_", "DYLD_"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

type Schedule struct {
	Enabled   bool
	Frequency string
	Hour      int32
	Minute    int32
	Weekday   int32
	Timezone  string
}

func (schedule Schedule) Validate() error {
	if !schedule.Enabled {
		return nil
	}
	if schedule.Frequency != "hourly" && schedule.Frequency != "daily" && schedule.Frequency != "weekly" {
		return fmt.Errorf("%w: schedule frequency must be hourly, daily, or weekly", ErrInvalid)
	}
	if schedule.Hour < 0 || schedule.Hour > 23 || schedule.Minute < 0 || schedule.Minute > 59 || schedule.Weekday < 0 || schedule.Weekday > 6 {
		return fmt.Errorf("%w: invalid schedule time", ErrInvalid)
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return fmt.Errorf("%w: invalid schedule timezone", ErrInvalid)
	}
	return nil
}

type GitSource struct {
	URL                  string
	Branch               string
	PrivateSSH           bool
	CredentialConfigured bool
}

func ValidateGitSource(source GitSource) error {
	if strings.TrimSpace(source.URL) == "" {
		return fmt.Errorf("%w: Git URL is required", ErrInvalid)
	}
	if strings.ContainsAny(source.Branch, "\x00\r\n") || strings.TrimSpace(source.Branch) == "" || len(source.Branch) > 255 {
		return fmt.Errorf("%w: invalid Git branch", ErrInvalid)
	}
	if source.PrivateSSH {
		if !strings.HasPrefix(source.URL, "ssh://") && !regexp.MustCompile(`^[^@\s]+@[^:\s]+:.+$`).MatchString(source.URL) {
			return fmt.Errorf("%w: private Git source requires an SSH URL", ErrInvalid)
		}
		return nil
	}
	parsed, err := url.Parse(source.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("%w: public Git source requires an HTTPS URL", ErrInvalid)
	}
	return nil
}

type WorkflowInput struct {
	Name            string
	Goal            string
	ExpertID        *string
	ProviderModelID *string
	RuntimeEngine   *RuntimeEngine
	Environment     []EnvironmentVariable
	Schedule        *Schedule
}

func (input WorkflowInput) Validate() error {
	if name := strings.TrimSpace(input.Name); len(name) < 1 || len(name) > 100 {
		return fmt.Errorf("%w: Workflow name must contain 1-100 characters", ErrInvalid)
	}
	if goal := strings.TrimSpace(input.Goal); len(goal) < 1 || len(goal) > 100_000 {
		return fmt.Errorf("%w: Workflow goal must contain 1-100000 characters", ErrInvalid)
	}
	if err := ValidateEnvironment(input.Environment); err != nil {
		return err
	}
	if input.Schedule != nil {
		return input.Schedule.Validate()
	}
	return nil
}

type Workflow struct {
	ID                      string
	OwnerID                 string
	Name                    string
	Goal                    string
	ExpertID                *string
	ProviderModelID         *string
	RuntimeEngine           *RuntimeEngine
	Environment             []EnvironmentVariable
	Schedule                *Schedule
	GitSource               *GitSource
	APICredentialConfigured bool
	WorkspacePath           string
	DeletedAt               *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
	Version                 int64
}

type ExpertInput struct {
	Name         string
	Description  string
	MCPServerIDs []string
	SkillIDs     []string
}

func (input ExpertInput) Validate() error {
	if name := strings.TrimSpace(input.Name); len(name) < 1 || len(name) > 100 {
		return fmt.Errorf("%w: Expert name must contain 1-100 characters", ErrInvalid)
	}
	if len(input.Description) > 2_000 || len(input.MCPServerIDs) > 50 || len(input.SkillIDs) > 50 {
		return fmt.Errorf("%w: Expert configuration exceeds limits", ErrInvalid)
	}
	return nil
}

type Expert struct {
	ID           string
	OwnerID      string
	Name         string
	Description  string
	MCPServerIDs []string
	SkillIDs     []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int64
}

type ModelProviderPreset struct {
	ProviderType     string
	DisplayName      string
	OfficialEndpoint string
	Protocols        []string
	DynamicDiscovery bool
}

type ModelProviderConnection struct {
	ID                 string
	OwnerID            string
	Name               string
	ProviderType       string
	Endpoint           string
	Protocols          []string
	HasAPIKey          bool
	VerificationStatus string
	VerificationError  string
	CustomEndpoint     bool
	LastSyncedAt       *time.Time
	LastSyncError      string
	Models             []ProviderModel
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Version            int64
}

type ProviderModel struct {
	ID            string
	ConnectionID  string
	ModelID       string
	DisplayName   string
	ModelType     string
	Available     bool
	ManuallyAdded bool
	Compatibility []RuntimeModelCompatibility
}

type RuntimeModelCompatibility struct {
	RuntimeEngine RuntimeEngine
	Status        string
	Reason        string
}

var providerPresets = []ModelProviderPreset{
	{ProviderType: "openai", DisplayName: "OpenAI", OfficialEndpoint: "https://api.openai.com/v1", Protocols: []string{"openai_responses", "openai_chat"}, DynamicDiscovery: true},
	{ProviderType: "anthropic", DisplayName: "Anthropic", OfficialEndpoint: "https://api.anthropic.com", Protocols: []string{"anthropic_messages"}, DynamicDiscovery: true},
	{ProviderType: "google_gemini", DisplayName: "Google Gemini", OfficialEndpoint: "https://generativelanguage.googleapis.com/v1beta", Protocols: []string{"gemini"}, DynamicDiscovery: true},
	{ProviderType: "xai", DisplayName: "xAI", OfficialEndpoint: "https://api.x.ai/v1", Protocols: []string{"openai_responses", "openai_chat"}, DynamicDiscovery: true},
	{ProviderType: "deepseek", DisplayName: "DeepSeek", OfficialEndpoint: "https://api.deepseek.com", Protocols: []string{"openai_chat"}, DynamicDiscovery: true},
	{ProviderType: "alibaba_bailian", DisplayName: "阿里云百炼", OfficialEndpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1", Protocols: []string{"openai_responses", "openai_chat", "anthropic_messages"}},
	{ProviderType: "volcengine_ark", DisplayName: "火山方舟", OfficialEndpoint: "https://ark.cn-beijing.volces.com/api/v3", Protocols: []string{"openai_chat"}},
	{ProviderType: "moonshot", DisplayName: "Moonshot", OfficialEndpoint: "https://api.moonshot.cn/v1", Protocols: []string{"openai_chat"}, DynamicDiscovery: true},
	{ProviderType: "zhipu", DisplayName: "智谱 GLM", OfficialEndpoint: "https://open.bigmodel.cn/api/paas/v4", Protocols: []string{"openai_chat", "anthropic_messages"}},
	{ProviderType: "minimax", DisplayName: "MiniMax", OfficialEndpoint: "https://api.minimax.io/v1", Protocols: []string{"openai_chat"}},
	{ProviderType: "custom_openai", DisplayName: "OpenAI Compatible", Protocols: []string{"openai_responses", "openai_chat", "anthropic_messages"}, DynamicDiscovery: true},
}

func ModelProviderPresets() []ModelProviderPreset {
	result := make([]ModelProviderPreset, len(providerPresets))
	copy(result, providerPresets)
	return result
}

func ValidateModelProviderConnection(name, providerType, endpoint string, protocols []string, apiKey string, apiKeyRequired bool) error {
	if value := strings.TrimSpace(name); value == "" || len(value) > 100 {
		return fmt.Errorf("%w: Provider Connection name must contain 1-100 characters", ErrInvalid)
	}
	knownProvider := false
	for _, preset := range providerPresets {
		knownProvider = knownProvider || preset.ProviderType == providerType
	}
	if !knownProvider {
		return fmt.Errorf("%w: unsupported model provider", ErrInvalid)
	}
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	scheme := ""
	if parsed != nil {
		scheme = strings.ToLower(parsed.Scheme)
	}
	if err != nil || parsed == nil || (scheme != "http" && scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%w: Model Endpoint must be an absolute HTTP or HTTPS URL", ErrInvalid)
	}
	if len(protocols) == 0 || len(protocols) > 4 {
		return fmt.Errorf("%w: at least one Model API Protocol is required", ErrInvalid)
	}
	seen := map[string]struct{}{}
	for _, protocol := range protocols {
		switch protocol {
		case "openai_responses", "openai_chat", "anthropic_messages", "gemini":
		default:
			return fmt.Errorf("%w: unsupported Model API Protocol", ErrInvalid)
		}
		if _, duplicate := seen[protocol]; duplicate {
			return fmt.Errorf("%w: duplicate Model API Protocol", ErrInvalid)
		}
		seen[protocol] = struct{}{}
	}
	if apiKeyRequired && strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("%w: API Key is required", ErrInvalid)
	}
	if len(apiKey) > 64*1024 {
		return fmt.Errorf("%w: API Key is too large", ErrInvalid)
	}
	return nil
}

func ValidateProviderModel(modelID, displayName, modelType string) error {
	if value := strings.TrimSpace(modelID); value == "" || len(value) > 500 || strings.ContainsAny(value, "\x00\r\n") {
		return fmt.Errorf("%w: invalid provider model identifier", ErrInvalid)
	}
	if len(displayName) > 500 {
		return fmt.Errorf("%w: provider model display name is too large", ErrInvalid)
	}
	switch modelType {
	case "agent", "text", "embedding", "image", "audio", "unknown":
		return nil
	default:
		return fmt.Errorf("%w: unsupported provider model type", ErrInvalid)
	}
}

func CompatibilityForProtocols(protocols []string) []RuntimeModelCompatibility {
	has := func(value string) bool {
		for _, protocol := range protocols {
			if protocol == value {
				return true
			}
		}
		return false
	}
	items := make([]RuntimeModelCompatibility, 0, 4)
	for _, runtime := range []RuntimeEngine{RuntimeClaude, RuntimeCodex, RuntimeHermes, RuntimeOpenClaw} {
		item := RuntimeModelCompatibility{RuntimeEngine: runtime, Status: "unverified", Reason: "This model and Runtime image have not passed conformance"}
		if runtime == RuntimeCodex && !has("openai_responses") {
			item.Status = "incompatible"
			item.Reason = "Codex requires the OpenAI Responses protocol"
		}
		if runtime == RuntimeClaude && !has("anthropic_messages") {
			item.Status = "incompatible"
			item.Reason = "Claude Code requires the Anthropic Messages protocol"
		}
		items = append(items, item)
	}
	return items
}

type Settings struct {
	Personality             string
	PersonalityInstructions string
	RuntimeModelDefaults    map[RuntimeEngine]string
	DefaultRuntimeEngine    RuntimeEngine
	Language                string
	Timezone                string
	Version                 int64
}

func (settings Settings) Validate() error {
	switch settings.Personality {
	case "gentle_professional", "direct_efficient", "lively_friendly":
	case "custom":
		if strings.TrimSpace(settings.PersonalityInstructions) == "" {
			return fmt.Errorf("%w: custom personality instructions are required", ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: unsupported personality", ErrInvalid)
	}
	if len(settings.PersonalityInstructions) > 4_000 {
		return fmt.Errorf("%w: personality instructions are too large", ErrInvalid)
	}
	if settings.Language != "zh-CN" && settings.Language != "en-US" {
		return fmt.Errorf("%w: unsupported language", ErrInvalid)
	}
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		return fmt.Errorf("%w: invalid timezone", ErrInvalid)
	}
	for runtime, modelID := range settings.RuntimeModelDefaults {
		if _, err := ParseRuntime(string(runtime)); err != nil {
			return err
		}
		if strings.TrimSpace(modelID) == "" {
			return fmt.Errorf("%w: Runtime default Provider Model is required", ErrInvalid)
		}
	}
	_, err := ParseRuntime(string(settings.DefaultRuntimeEngine))
	return err
}

type Run struct {
	ID               string
	OwnerID          string
	WorkflowID       string
	WorkflowName     string
	Trigger          string
	State            string
	TextInput        *string
	JSONInput        map[string]any
	FinalText        *string
	FinalJSON        map[string]any
	Error            string
	WorkflowSnapshot map[string]any
	QueuedAt         time.Time
	StartedAt        *time.Time
	EndedAt          *time.Time
}

func ValidateWorkspacePath(value string) (string, error) {
	if strings.ContainsRune(value, '\x00') || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return "", fmt.Errorf("%w: invalid Workspace path", ErrInvalid)
	}
	clean := path.Clean(strings.TrimSpace(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || len(clean) > 1_024 {
		return "", fmt.Errorf("%w: invalid Workspace path", ErrInvalid)
	}
	return clean, nil
}
