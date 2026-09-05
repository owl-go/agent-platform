package domain

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
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
	RuntimePI       RuntimeEngine = "pi"
)

func ParseRuntime(value string) (RuntimeEngine, error) {
	runtime := RuntimeEngine(strings.TrimSpace(value))
	switch runtime {
	case RuntimeClaude, RuntimeCodex, RuntimeHermes, RuntimeOpenClaw, RuntimePI:
		return runtime, nil
	default:
		return "", fmt.Errorf("%w: unsupported Runtime Engine", ErrInvalid)
	}
}

type Session struct {
	ID           string
	OwnerID      string
	Title        string
	ExpertID     *string
	ExpertTeamID *string
	ArchivedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int64
}

type Message struct {
	ID                int64
	SessionID         string
	Role              string
	State             string
	Content           string
	Error             string
	ProgressStage     string
	ElapsedMS         int64
	CreatedAt         time.Time
	ResponseSnapshot  *ResponseSnapshot
	Attachments       []Attachment
	ExpertStages      []ExpertStage
	CreditConsumption *CreditConsumption
	Activities        []ExecutionActivity
}

// ExecutionActivity is a redacted, user-visible summary rather than raw Runtime output.
type ExecutionActivity struct {
	Type   string `json:"type"`
	Detail string `json:"detail"`
}

// Attachment is an immutable reference to a user upload frozen onto one turn.
type Attachment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	ObjectKey   string `json:"object_key"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Image       bool   `json:"image"`
}

type ResponseSnapshot struct {
	SchemaVersion     int                      `json:"schema_version,omitempty"`
	Stages            []ExecutionStageSnapshot `json:"stages,omitempty"`
	ProviderModelID   string                   `json:"provider_model_id"`
	ConnectionID      string                   `json:"connection_id"`
	ConnectionName    string                   `json:"connection_name"`
	ProviderType      string                   `json:"provider_type"`
	ModelID           string                   `json:"model_id"`
	ModelName         string                   `json:"model_name"`
	Endpoint          string                   `json:"endpoint"`
	Protocols         []string                 `json:"protocols"`
	RuntimeEngine     RuntimeEngine            `json:"runtime_engine"`
	Compatibility     string                   `json:"compatibility"`
	ConnectionVersion int64                    `json:"connection_version"`
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
	case "HOME", "PATH", "SHELL", "ENV", "BASH_ENV", "PROMPT_COMMAND", "CODEX_HOME", "OPENCLAW_CONFIG_PATH", "PI_CODING_AGENT_DIR", "PI_CODING_AGENT_SESSION_DIR", "GIT_SSH", "GIT_SSH_COMMAND", "NODE_OPTIONS", "PYTHONHOME", "PYTHONPATH":
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
	Authentication       string
	Username             *string
	Config               []GitConfigEntry
	SSHConfig            string
	CredentialConfigured bool
}

type GitConfigEntry struct {
	Key   string
	Value string
}

func ValidateGitSource(source GitSource) error {
	if strings.TrimSpace(source.URL) == "" {
		return fmt.Errorf("%w: Git URL is required", ErrInvalid)
	}
	if strings.ContainsAny(source.Branch, "\x00\r\n") || strings.TrimSpace(source.Branch) == "" || len(source.Branch) > 255 {
		return fmt.Errorf("%w: invalid Git branch", ErrInvalid)
	}
	if source.Authentication != "none" && source.Authentication != "basic" && source.Authentication != "ssh" {
		return fmt.Errorf("%w: Git authentication must be none, basic, or ssh", ErrInvalid)
	}
	if source.Authentication == "ssh" {
		if !validSSHRepositoryURL(source.URL, source.SSHConfig) {
			return fmt.Errorf("%w: private Git source requires an SSH URL", ErrInvalid)
		}
		if err := ValidateSSHConfig(source.SSHConfig); err != nil {
			return err
		}
	} else {
		if strings.TrimSpace(source.SSHConfig) != "" {
			return fmt.Errorf("%w: SSH config requires SSH authentication", ErrInvalid)
		}
		parsed, err := url.Parse(source.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("%w: Git source requires an HTTPS URL without embedded credentials", ErrInvalid)
		}
		if source.Authentication == "basic" && (source.Username == nil || strings.TrimSpace(*source.Username) == "") {
			return fmt.Errorf("%w: Git username is required for account authentication", ErrInvalid)
		}
	}
	return ValidateGitConfig(source.Config)
}

func validSSHRepositoryURL(value, config string) bool {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "ssh://") {
		parsed, err := url.Parse(value)
		return err == nil && parsed.Scheme == "ssh" && parsed.Host != "" && parsed.Path != ""
	}
	match := regexp.MustCompile(`^(?:([^@\s/:]+)@)?([^:\s/]+):(.+)$`).FindStringSubmatch(value)
	return match != nil && strings.TrimSpace(match[3]) != "" && (match[1] != "" || strings.EqualFold(match[2], sshConfigHost(config)))
}

func sshConfigHost(config string) string {
	for _, raw := range strings.Split(config, "\n") {
		key, value, ok := sshConfigDirective(strings.TrimSpace(strings.TrimSuffix(raw, "\r")))
		if ok && key == "host" {
			return value
		}
	}
	return ""
}

func ValidateSSHConfig(config string) error {
	_, err := SSHConfigIdentityFile(config)
	return err
}

func SSHConfigIdentityFile(config string) (string, error) {
	if len(config) > 16*1024 || strings.ContainsRune(config, '\x00') {
		return "", fmt.Errorf("%w: SSH config must contain at most 16384 bytes", ErrInvalid)
	}
	identity := "id_workflow"
	seen := make(map[string]bool)
	hasHost := false
	for number, raw := range strings.Split(config, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := sshConfigDirective(line)
		if !ok || seen[key] {
			return "", fmt.Errorf("%w: invalid or repeated SSH config directive on line %d", ErrInvalid, number+1)
		}
		seen[key] = true
		switch key {
		case "host":
			hasHost = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$`).MatchString(value)
			if !hasHost {
				return "", fmt.Errorf("%w: SSH Host must be one exact alias", ErrInvalid)
			}
		case "hostname":
			if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,254}$`).MatchString(value) {
				return "", fmt.Errorf("%w: invalid SSH HostName", ErrInvalid)
			}
		case "user":
			if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`).MatchString(value) {
				return "", fmt.Errorf("%w: invalid SSH User", ErrInvalid)
			}
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil || port < 1 || port > 65535 {
				return "", fmt.Errorf("%w: invalid SSH Port", ErrInvalid)
			}
		case "identityfile":
			match := regexp.MustCompile(`^~/.ssh/([A-Za-z0-9][A-Za-z0-9._-]{0,127})$`).FindStringSubmatch(value)
			if match == nil {
				return "", fmt.Errorf("%w: SSH IdentityFile must be a file under ~/.ssh", ErrInvalid)
			}
			identity = match[1]
		case "identitiesonly":
			if !strings.EqualFold(value, "yes") {
				return "", fmt.Errorf("%w: SSH IdentitiesOnly must be yes", ErrInvalid)
			}
		case "serveraliveinterval":
			seconds, err := strconv.Atoi(value)
			if err != nil || seconds < 0 || seconds > 3600 {
				return "", fmt.Errorf("%w: invalid SSH ServerAliveInterval", ErrInvalid)
			}
		case "serveralivecountmax":
			count, err := strconv.Atoi(value)
			if err != nil || count < 0 || count > 100 {
				return "", fmt.Errorf("%w: invalid SSH ServerAliveCountMax", ErrInvalid)
			}
		default:
			return "", fmt.Errorf("%w: unsupported SSH config directive %q", ErrInvalid, key)
		}
	}
	if strings.TrimSpace(config) != "" && !hasHost {
		return "", fmt.Errorf("%w: SSH config requires one Host directive", ErrInvalid)
	}
	return identity, nil
}

func sshConfigDirective(line string) (string, string, bool) {
	separator := strings.IndexAny(line, " \t=")
	if separator <= 0 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(line[:separator]))
	value := strings.TrimSpace(strings.TrimLeft(line[separator:], " \t="))
	return key, value, value != ""
}

func ValidateGitConfig(config []GitConfigEntry) error {
	allowedConfig := map[string]bool{"user.name": true, "user.email": true, "core.autocrlf": true, "core.filemode": true, "pull.rebase": true, "init.defaultbranch": true}
	for _, entry := range config {
		key := strings.ToLower(strings.TrimSpace(entry.Key))
		if !allowedConfig[key] || strings.TrimSpace(entry.Value) == "" || strings.ContainsAny(entry.Value, "\x00\r\n") || len(entry.Value) > 512 {
			return fmt.Errorf("%w: unsupported or invalid Git config %q", ErrInvalid, entry.Key)
		}
	}
	return nil
}

type WorkflowInput struct {
	Name            string
	Goal            string
	ExpertID        *string
	ExpertTeamID    *string
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
	if input.ExpertID != nil && input.ExpertTeamID != nil {
		return fmt.Errorf("%w: choose either an Expert or an Expert Team", ErrInvalid)
	}
	if input.ProviderModelID != nil || input.RuntimeEngine != nil {
		return fmt.Errorf("%w: Workflow model and Runtime overrides are no longer supported", ErrInvalid)
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
	ExpertTeamID            *string
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
	Name                   string
	Icon                   string
	IconBackground         string
	Introduction           string
	CoreCapability         string
	OperatingProcedure     string
	OutputStandard         string
	Cautions               string
	CapabilityIntroduction string
	ExecutionInstruction   string
	ProviderModelID        string
	RuntimeEngine          RuntimeEngine
	ExpertiseTags          []string
	MCPServerIDs           []string
	SkillIDs               []string
}

func (input ExpertInput) Validate() error {
	if name := strings.TrimSpace(input.Name); len(name) < 1 || len(name) > 100 {
		return fmt.Errorf("%w: Expert name must contain 1-100 characters", ErrInvalid)
	}
	structured := strings.TrimSpace(input.Introduction) != "" || strings.TrimSpace(input.CoreCapability) != "" || strings.TrimSpace(input.OperatingProcedure) != "" || strings.TrimSpace(input.OutputStandard) != "" || strings.TrimSpace(input.Cautions) != ""
	if structured {
		for _, field := range []struct {
			name  string
			value string
			max   int
		}{
			{name: "Introduction", value: input.Introduction, max: 2_000},
			{name: "Core Capability", value: input.CoreCapability, max: 20_000},
			{name: "Operating Procedure", value: input.OperatingProcedure, max: 20_000},
			{name: "Output Standard", value: input.OutputStandard, max: 20_000},
		} {
			length := len(strings.TrimSpace(field.value))
			if length < 1 || length > field.max {
				return fmt.Errorf("%w: %s must contain 1-%d characters", ErrInvalid, field.name, field.max)
			}
		}
		if len(strings.TrimSpace(input.Cautions)) > 20_000 {
			return fmt.Errorf("%w: Cautions must contain at most 20000 characters", ErrInvalid)
		}
	} else {
		if introduction := strings.TrimSpace(input.CapabilityIntroduction); len(introduction) < 1 || len(introduction) > 2_000 {
			return fmt.Errorf("%w: Capability Introduction must contain 1-2000 characters", ErrInvalid)
		}
		if instruction := strings.TrimSpace(input.ExecutionInstruction); len(instruction) < 1 || len(instruction) > 20_000 {
			return fmt.Errorf("%w: Execution Instruction must contain 1-20000 characters", ErrInvalid)
		}
	}
	if !structured {
		if strings.TrimSpace(input.ProviderModelID) == "" {
			return fmt.Errorf("%w: Expert Provider Model is required", ErrInvalid)
		}
		if _, err := ParseRuntime(string(input.RuntimeEngine)); err != nil {
			return fmt.Errorf("%w: Expert Runtime Engine is required", ErrInvalid)
		}
	}
	if err := ValidateExpertiseTags(input.ExpertiseTags); err != nil {
		return err
	}
	if len(input.MCPServerIDs) > 50 || len(input.SkillIDs) > 50 {
		return fmt.Errorf("%w: Expert configuration exceeds limits", ErrInvalid)
	}
	return nil
}

type Expert struct {
	ID                     string
	OwnerID                string
	Name                   string
	Icon                   string
	IconBackground         string
	Introduction           string
	CoreCapability         string
	OperatingProcedure     string
	OutputStandard         string
	Cautions               string
	CapabilityIntroduction string
	ExecutionInstruction   string
	ProviderModelID        string
	RuntimeEngine          RuntimeEngine
	ExpertiseTags          []string
	MCPServerIDs           []string
	SkillIDs               []string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
	TagProjectionStatus    string
	TagProjectionError     string
}

func (expert Expert) Available() bool {
	if strings.TrimSpace(expert.Introduction) != "" || strings.TrimSpace(expert.CoreCapability) != "" || strings.TrimSpace(expert.OperatingProcedure) != "" || strings.TrimSpace(expert.OutputStandard) != "" {
		return strings.TrimSpace(expert.Introduction) != "" && strings.TrimSpace(expert.CoreCapability) != "" && strings.TrimSpace(expert.OperatingProcedure) != "" && strings.TrimSpace(expert.OutputStandard) != ""
	}
	if strings.TrimSpace(expert.ExecutionInstruction) == "" || strings.TrimSpace(expert.ProviderModelID) == "" {
		return false
	}
	_, err := ParseRuntime(string(expert.RuntimeEngine))
	return err == nil
}

func ValidateExpertiseTags(tags []string) error {
	if len(tags) > 10 {
		return fmt.Errorf("%w: at most ten Expertise Tags are allowed", ErrInvalid)
	}
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" || len([]rune(trimmed)) > 20 {
			return fmt.Errorf("%w: Expertise Tags must contain 1-20 characters", ErrInvalid)
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: duplicate Expertise Tag", ErrInvalid)
		}
		seen[key] = struct{}{}
	}
	return nil
}

type ExpertTeamInput struct {
	Name                   string
	Icon                   string
	IconBackground         string
	Introduction           string
	CoreCapability         string
	Members                []ExpertTeamMemberInput
	CapabilityIntroduction string
	ExpertiseTags          []string
	ExpertIDs              []string
}

type ExpertTeamMemberInput struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	ExpertID string   `json:"expert_id"`
	Labels   []string `json:"labels"`
}

type ExpertTeamMember struct {
	ID       string
	Name     string
	Expert   Expert
	Labels   []string
	Position int
}

var teamMemberID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

func (input ExpertTeamInput) Validate() error {
	if name := strings.TrimSpace(input.Name); len(name) < 1 || len(name) > 100 {
		return fmt.Errorf("%w: Expert Team name must contain 1-100 characters", ErrInvalid)
	}
	structured := strings.TrimSpace(input.Introduction) != "" || strings.TrimSpace(input.CoreCapability) != "" || len(input.Members) > 0
	introduction := input.CapabilityIntroduction
	if structured {
		introduction = input.Introduction
	}
	if introduction := strings.TrimSpace(introduction); len(introduction) < 1 || len(introduction) > 2_000 {
		return fmt.Errorf("%w: Capability Introduction must contain 1-2000 characters", ErrInvalid)
	}
	if structured {
		if capability := strings.TrimSpace(input.CoreCapability); len(capability) < 1 || len(capability) > 20_000 {
			return fmt.Errorf("%w: Expert Team Core Capability must contain 1-20000 characters", ErrInvalid)
		}
		if len(input.Members) < 2 || len(input.Members) > 10 {
			return fmt.Errorf("%w: Expert Team must contain 2-10 Members", ErrInvalid)
		}
		ids, names := map[string]struct{}{}, map[string]struct{}{}
		for _, member := range input.Members {
			id, name, expertID := strings.TrimSpace(member.ID), strings.TrimSpace(member.Name), strings.TrimSpace(member.ExpertID)
			if !teamMemberID.MatchString(id) || expertID == "" || len([]rune(name)) < 1 || len([]rune(name)) > 100 {
				return fmt.Errorf("%w: Team Member ID, name, and Expert are required", ErrInvalid)
			}
			if _, exists := ids[id]; exists {
				return fmt.Errorf("%w: duplicate Team Member ID", ErrInvalid)
			}
			if _, exists := names[strings.ToLower(name)]; exists {
				return fmt.Errorf("%w: duplicate Team Member name", ErrInvalid)
			}
			ids[id], names[strings.ToLower(name)] = struct{}{}, struct{}{}
			if len(member.Labels) > 5 {
				return fmt.Errorf("%w: at most five Member Labels are allowed", ErrInvalid)
			}
			for _, label := range member.Labels {
				if length := len([]rune(strings.TrimSpace(label))); length < 1 || length > 20 {
					return fmt.Errorf("%w: Member Labels must contain 1-20 characters", ErrInvalid)
				}
			}
		}
		return nil
	}
	if err := ValidateExpertiseTags(input.ExpertiseTags); err != nil {
		return err
	}
	if len(input.ExpertIDs) < 2 || len(input.ExpertIDs) > 10 {
		return fmt.Errorf("%w: Expert Team must contain 2-10 Experts", ErrInvalid)
	}
	seen := make(map[string]struct{}, len(input.ExpertIDs))
	for _, expertID := range input.ExpertIDs {
		if strings.TrimSpace(expertID) == "" {
			return fmt.Errorf("%w: Expert Team member is required", ErrInvalid)
		}
		if _, exists := seen[expertID]; exists {
			return fmt.Errorf("%w: duplicate Expert Team member", ErrInvalid)
		}
		seen[expertID] = struct{}{}
	}
	return nil
}

type ExpertTeam struct {
	ID                     string
	OwnerID                string
	Name                   string
	Icon                   string
	IconBackground         string
	Introduction           string
	CoreCapability         string
	Members                []ExpertTeamMember
	CapabilityIntroduction string
	ExpertiseTags          []string
	Experts                []Expert
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
}

func (team ExpertTeam) Available() bool {
	if len(team.Members) > 0 {
		if len(team.Members) < 2 || len(team.Members) > 10 {
			return false
		}
		for _, member := range team.Members {
			if !member.Expert.Available() {
				return false
			}
		}
		return true
	}
	if len(team.Experts) < 2 || len(team.Experts) > 10 {
		return false
	}
	for _, expert := range team.Experts {
		if !expert.Available() {
			return false
		}
	}
	return true
}

type ModelProviderPreset struct {
	ProviderType     string
	DisplayName      string
	OfficialEndpoint string
	Protocols        []string
}

type ModelProviderConnection struct {
	ID                 string
	CredentialOwnerID  string
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
	{ProviderType: "openai", DisplayName: "OpenAI", OfficialEndpoint: "https://api.openai.com/v1", Protocols: []string{"openai_responses", "openai_chat"}},
	{ProviderType: "anthropic", DisplayName: "Anthropic", OfficialEndpoint: "https://api.anthropic.com", Protocols: []string{"anthropic_messages"}},
	{ProviderType: "google_gemini", DisplayName: "Google Gemini", OfficialEndpoint: "https://generativelanguage.googleapis.com/v1beta", Protocols: []string{"gemini"}},
	{ProviderType: "xai", DisplayName: "xAI", OfficialEndpoint: "https://api.x.ai/v1", Protocols: []string{"openai_responses", "openai_chat"}},
	{ProviderType: "deepseek", DisplayName: "DeepSeek", OfficialEndpoint: "https://api.deepseek.com", Protocols: []string{"openai_chat"}},
	{ProviderType: "alibaba_bailian", DisplayName: "阿里云百炼", OfficialEndpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1", Protocols: []string{"openai_responses", "openai_chat", "anthropic_messages"}},
	{ProviderType: "volcengine_ark", DisplayName: "火山方舟", OfficialEndpoint: "https://ark.cn-beijing.volces.com/api/v3", Protocols: []string{"openai_chat"}},
	{ProviderType: "moonshot", DisplayName: "Moonshot", OfficialEndpoint: "https://api.moonshot.cn/v1", Protocols: []string{"openai_chat"}},
	{ProviderType: "zhipu", DisplayName: "智谱 GLM", OfficialEndpoint: "https://open.bigmodel.cn/api/paas/v4", Protocols: []string{"openai_chat", "anthropic_messages"}},
	{ProviderType: "minimax", DisplayName: "MiniMax", OfficialEndpoint: "https://api.minimax.io/v1", Protocols: []string{"openai_chat"}},
	{ProviderType: "custom_openai", DisplayName: "OpenAI Compatible", Protocols: []string{"openai_responses", "openai_chat", "anthropic_messages"}},
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

func ValidateProviderModel(modelID, displayName string) error {
	if value := strings.TrimSpace(modelID); value == "" || len(value) > 500 || strings.ContainsAny(value, "\x00\r\n") {
		return fmt.Errorf("%w: invalid provider model identifier", ErrInvalid)
	}
	if len(displayName) > 500 {
		return fmt.Errorf("%w: provider model display name is too large", ErrInvalid)
	}
	return nil
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
	items := make([]RuntimeModelCompatibility, 0, 5)
	for _, runtime := range []RuntimeEngine{RuntimeClaude, RuntimeCodex, RuntimeHermes, RuntimeOpenClaw, RuntimePI} {
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
	ID                string
	ConversationID    string
	TurnNumber        int
	OwnerID           string
	WorkflowID        string
	WorkflowName      string
	Trigger           string
	State             string
	TextInput         *string
	JSONInput         map[string]any
	Attachments       []Attachment
	ExpertStages      []ExpertStage
	FinalText         *string
	FinalJSON         map[string]any
	Error             string
	WorkflowSnapshot  map[string]any
	QueuedAt          time.Time
	StartedAt         *time.Time
	EndedAt           *time.Time
	CreditConsumption *CreditConsumption
}

type ExpertStage struct {
	ExpertID          string                  `json:"expert_id"`
	ExpertName        string                  `json:"expert_name"`
	ProviderModelID   string                  `json:"provider_model_id"`
	ProviderModelName string                  `json:"provider_model_name"`
	RuntimeEngine     RuntimeEngine           `json:"runtime_engine"`
	Position          int                     `json:"position"`
	Total             int                     `json:"total"`
	State             string                  `json:"state"`
	ElapsedMS         int64                   `json:"elapsed_ms"`
	FinalText         string                  `json:"final_text,omitempty"`
	Error             string                  `json:"error,omitempty"`
	StartedAt         time.Time               `json:"started_at,omitempty"`
	EndedAt           time.Time               `json:"ended_at,omitempty"`
	CreditConsumption *CreditStageConsumption `json:"credit_consumption,omitempty"`
}

// CreditConsumption is the safe, owner-visible accounting result for a turn.
type CreditConsumption struct {
	TotalHundredths int64                    `json:"total_hundredths"`
	Stages          []CreditStageConsumption `json:"stages"`
}

type CreditStageConsumption struct {
	StagePosition          int    `json:"stage_position"`
	ProviderModel          string `json:"provider_model"`
	RuntimeEngine          string `json:"runtime_engine"`
	InputTokens            int64  `json:"input_tokens,omitempty"`
	OutputTokens           int64  `json:"output_tokens,omitempty"`
	UsageReported          bool   `json:"usage_reported"`
	InputMultiplierMicros  int64  `json:"input_multiplier_micros"`
	OutputMultiplierMicros int64  `json:"output_multiplier_micros"`
	FallbackHundredths     int64  `json:"fallback_hundredths"`
	AmountHundredths       int64  `json:"amount_hundredths"`
	Estimated              bool   `json:"estimated"`
	RateRevisionID         string `json:"rate_revision_id"`
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
