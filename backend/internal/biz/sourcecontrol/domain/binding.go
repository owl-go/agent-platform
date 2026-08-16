package domain

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrBindingNotFound         = errors.New("Repository Binding not found")
	ErrBindingNameExists       = errors.New("Repository Binding name already exists")
	ErrBindingConcurrentUpdate = errors.New("Repository Binding was modified concurrently")
	ErrInvalidBinding          = errors.New("invalid Repository Binding input")
)

var (
	branchPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)
	executablePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+/-]{0,199}$`)
	scpURLPattern     = regexp.MustCompile(`^([^@/:]+)@([^/:]+):([^\s]+)$`)
)

type QualityCommandKind string

const (
	QualityBuild  QualityCommandKind = "build"
	QualityFormat QualityCommandKind = "format"
	QualityLint   QualityCommandKind = "lint"
	QualityTest   QualityCommandKind = "test"
)

type QualityCommand struct {
	Name           string             `json:"name"`
	Kind           QualityCommandKind `json:"kind"`
	Executable     string             `json:"executable"`
	Arguments      []string           `json:"arguments"`
	TimeoutSeconds int                `json:"timeout_seconds"`
}

type ModelBudget struct {
	MaxInputTokens  int64  `json:"max_input_tokens"`
	MaxOutputTokens int64  `json:"max_output_tokens"`
	MaxCostAmount   string `json:"max_cost_amount"`
}

type EgressPolicy struct {
	Mode string `json:"mode"`
}

type ValidationReport struct {
	Valid     bool              `json:"valid"`
	Errors    map[string]string `json:"errors"`
	CheckedAt time.Time         `json:"checked_at"`
}

type RepositoryBinding struct {
	ID                        string
	OrganizationID            string
	TeamID                    string
	SourceControlProviderID   string
	Name                      string
	RepositorySSHURL          string
	RepositoryHost            string
	DefaultBranch             string
	SSHCredentialProfileID    string
	BuildCredentialProfileIDs []string
	GitAuthorName             string
	GitAuthorEmail            string
	AllowedRuntimeImageIDs    []string
	DefaultRuntimeImageID     string
	DefaultModelID            string
	ModelBudget               ModelBudget
	Instructions              string
	QualityCommands           []QualityCommand
	EgressPolicy              EgressPolicy
	ValidationReport          *ValidationReport
	ValidatedAt               *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	Version                   int64
}

type BindingRegistration struct {
	ID                        string
	OrganizationID            string
	TeamID                    string
	SourceControlProviderID   string
	Name                      string
	RepositorySSHURL          string
	DefaultBranch             string
	SSHCredentialProfileID    string
	BuildCredentialProfileIDs []string
	GitAuthorName             string
	GitAuthorEmail            string
	AllowedRuntimeImageIDs    []string
	DefaultRuntimeImageID     string
	DefaultModelID            string
	ModelBudget               ModelBudget
	Instructions              string
	QualityCommands           []QualityCommand
	EgressPolicy              EgressPolicy
	Now                       time.Time
}

type PersistedBinding struct {
	Registration     BindingRegistration
	ValidationReport *ValidationReport
	ValidatedAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

func RegisterBinding(registration BindingRegistration) (RepositoryBinding, error) {
	if strings.TrimSpace(registration.ID) == "" || strings.TrimSpace(registration.OrganizationID) == "" || strings.TrimSpace(registration.TeamID) == "" || strings.TrimSpace(registration.SourceControlProviderID) == "" {
		return RepositoryBinding{}, invalidBindingf("identity, Organization, Team, and Source Control Provider are required")
	}
	if strings.TrimSpace(registration.Name) == "" || len(registration.Name) > 200 {
		return RepositoryBinding{}, invalidBindingf("name is required and cannot exceed 200 characters")
	}
	host, err := repositoryHost(registration.RepositorySSHURL)
	if err != nil {
		return RepositoryBinding{}, err
	}
	if !validBranch(registration.DefaultBranch) {
		return RepositoryBinding{}, invalidBindingf("default branch is invalid")
	}
	if strings.TrimSpace(registration.SSHCredentialProfileID) == "" || strings.TrimSpace(registration.DefaultModelID) == "" {
		return RepositoryBinding{}, invalidBindingf("SSH Credential Profile and default Configured Model are required")
	}
	if strings.TrimSpace(registration.GitAuthorName) == "" || len(registration.GitAuthorName) > 200 {
		return RepositoryBinding{}, invalidBindingf("Git author name is required")
	}
	address, err := mail.ParseAddress(registration.GitAuthorEmail)
	if err != nil || address.Address != registration.GitAuthorEmail {
		return RepositoryBinding{}, invalidBindingf("Git author email must be a plain email address")
	}
	runtimes, err := normalizedIDs(registration.AllowedRuntimeImageIDs, "allowed Runtime Images")
	if err != nil || !contains(runtimes, registration.DefaultRuntimeImageID) {
		return RepositoryBinding{}, invalidBindingf("default Runtime Image must belong to the non-empty allowed Runtime Image set")
	}
	buildCredentials, err := normalizedIDsAllowEmpty(registration.BuildCredentialProfileIDs, "build Credential Profiles")
	if err != nil {
		return RepositoryBinding{}, err
	}
	if err := registration.ModelBudget.validate(); err != nil {
		return RepositoryBinding{}, err
	}
	commands, err := validateQualityCommands(registration.QualityCommands)
	if err != nil {
		return RepositoryBinding{}, err
	}
	if registration.EgressPolicy.Mode != "public" {
		return RepositoryBinding{}, invalidBindingf("Egress Policy mode must be public")
	}
	if registration.Now.IsZero() {
		return RepositoryBinding{}, invalidBindingf("registration time is required")
	}
	now := registration.Now.UTC()
	return RepositoryBinding{
		ID: registration.ID, OrganizationID: registration.OrganizationID, TeamID: registration.TeamID,
		SourceControlProviderID: registration.SourceControlProviderID, Name: strings.TrimSpace(registration.Name),
		RepositorySSHURL: registration.RepositorySSHURL, RepositoryHost: host, DefaultBranch: registration.DefaultBranch,
		SSHCredentialProfileID: registration.SSHCredentialProfileID, BuildCredentialProfileIDs: buildCredentials,
		GitAuthorName: strings.TrimSpace(registration.GitAuthorName), GitAuthorEmail: registration.GitAuthorEmail,
		AllowedRuntimeImageIDs: runtimes, DefaultRuntimeImageID: registration.DefaultRuntimeImageID,
		DefaultModelID: registration.DefaultModelID, ModelBudget: registration.ModelBudget,
		Instructions: registration.Instructions, QualityCommands: commands, EgressPolicy: registration.EgressPolicy,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func RestoreBinding(persisted PersistedBinding) (RepositoryBinding, error) {
	persisted.Registration.Now = persisted.CreatedAt
	binding, err := RegisterBinding(persisted.Registration)
	if err != nil {
		return RepositoryBinding{}, err
	}
	if persisted.Version <= 0 || persisted.UpdatedAt.IsZero() || persisted.UpdatedAt.Before(persisted.CreatedAt) {
		return RepositoryBinding{}, invalidBindingf("persisted timestamps and Version are invalid")
	}
	if (persisted.ValidationReport == nil) != (persisted.ValidatedAt == nil) {
		return RepositoryBinding{}, invalidBindingf("persisted Validation Report is inconsistent")
	}
	if persisted.ValidationReport != nil {
		if persisted.ValidationReport.CheckedAt.IsZero() || !persisted.ValidationReport.CheckedAt.Equal(*persisted.ValidatedAt) {
			return RepositoryBinding{}, invalidBindingf("persisted validation timestamp is inconsistent")
		}
		if persisted.ValidationReport.Valid && len(persisted.ValidationReport.Errors) != 0 || !persisted.ValidationReport.Valid && len(persisted.ValidationReport.Errors) == 0 {
			return RepositoryBinding{}, invalidBindingf("persisted Validation Report result is inconsistent")
		}
		report := *persisted.ValidationReport
		report.Errors = cloneErrors(report.Errors)
		binding.ValidationReport = &report
		validatedAt := persisted.ValidatedAt.UTC()
		binding.ValidatedAt = &validatedAt
	}
	binding.CreatedAt = persisted.CreatedAt.UTC()
	binding.UpdatedAt = persisted.UpdatedAt.UTC()
	binding.Version = persisted.Version
	return binding, nil
}

func (binding *RepositoryBinding) RecordValidation(report ValidationReport) error {
	if report.CheckedAt.IsZero() {
		return invalidBindingf("validation time is required")
	}
	if report.Valid && len(report.Errors) != 0 || !report.Valid && len(report.Errors) == 0 {
		return invalidBindingf("Validation Report result and errors are inconsistent")
	}
	checkedAt := report.CheckedAt.UTC()
	report.CheckedAt = checkedAt
	report.Errors = cloneErrors(report.Errors)
	binding.ValidationReport = &report
	binding.ValidatedAt = &checkedAt
	binding.UpdatedAt = checkedAt
	binding.Version++
	return nil
}

func (binding *RepositoryBinding) Reconfigure(registration BindingRegistration) error {
	registration.ID = binding.ID
	registration.OrganizationID = binding.OrganizationID
	registration.TeamID = binding.TeamID
	if registration.Now.IsZero() || registration.Now.Before(binding.UpdatedAt) {
		return invalidBindingf("reconfiguration time is required and cannot move backwards")
	}
	candidate, err := RegisterBinding(registration)
	if err != nil {
		return err
	}
	candidate.CreatedAt = binding.CreatedAt
	candidate.UpdatedAt = registration.Now.UTC()
	candidate.Version = binding.Version + 1
	candidate.ValidationReport = nil
	candidate.ValidatedAt = nil
	*binding = candidate
	return nil
}

func (budget ModelBudget) validate() error {
	if budget.MaxInputTokens <= 0 || budget.MaxOutputTokens <= 0 || !validPositiveDecimal(budget.MaxCostAmount) {
		return invalidBindingf("Model Budget requires positive input tokens, output tokens, and cost amount")
	}
	return nil
}

func validPositiveDecimal(value string) bool {
	if value == "" || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return false
	}
	var whole, fraction string
	whole, fraction, _ = strings.Cut(value, ".")
	if whole == "" || len(fraction) > 8 {
		return false
	}
	for _, part := range []string{whole, fraction} {
		for _, character := range part {
			if character < '0' || character > '9' {
				return false
			}
		}
	}
	return strings.Trim(value, "0.") != ""
}

func validateQualityCommands(input []QualityCommand) ([]QualityCommand, error) {
	if len(input) == 0 || len(input) > 20 {
		return nil, invalidBindingf("one to twenty quality commands are required")
	}
	commands := make([]QualityCommand, len(input))
	names := make(map[string]bool, len(input))
	for index, command := range input {
		command.Name = strings.TrimSpace(command.Name)
		if command.Name == "" || names[command.Name] || !executablePattern.MatchString(command.Executable) || command.TimeoutSeconds <= 0 || command.TimeoutSeconds > 3600 || len(command.Arguments) > 100 {
			return nil, invalidBindingf("quality command %d is invalid", index)
		}
		switch command.Kind {
		case QualityBuild, QualityFormat, QualityLint, QualityTest:
		default:
			return nil, invalidBindingf("quality command %d has unsupported kind", index)
		}
		for _, argument := range command.Arguments {
			if strings.ContainsRune(argument, 0) || len(argument) > 4096 {
				return nil, invalidBindingf("quality command %d has invalid argument", index)
			}
		}
		names[command.Name] = true
		command.Arguments = append([]string(nil), command.Arguments...)
		commands[index] = command
	}
	return commands, nil
}

func repositoryHost(value string) (string, error) {
	if match := scpURLPattern.FindStringSubmatch(value); len(match) == 4 && match[1] == "git" && !strings.HasPrefix(match[3], "/") {
		return strings.ToLower(match[2]), nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "ssh" || parsed.Hostname() == "" || parsed.User == nil || parsed.User.Username() != "git" || parsed.User.String() != "git" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path == "" || parsed.Path == "/" {
		return "", invalidBindingf("repository URL must use ssh://git@host/path or git@host:path")
	}
	return strings.ToLower(parsed.Hostname()), nil
}

func validBranch(value string) bool {
	return branchPattern.MatchString(value) && !strings.Contains(value, "..") && !strings.Contains(value, "//") && !strings.HasSuffix(value, ".") && !strings.HasSuffix(value, "/") && !strings.Contains(value, "@{")
}

func normalizedIDs(input []string, name string) ([]string, error) {
	if len(input) == 0 {
		return nil, invalidBindingf("%s cannot be empty", name)
	}
	return normalizedIDsAllowEmpty(input, name)
}

func normalizedIDsAllowEmpty(input []string, name string) ([]string, error) {
	seen := make(map[string]bool, len(input))
	result := make([]string, 0, len(input))
	for _, id := range input {
		if strings.TrimSpace(id) == "" || seen[id] {
			return nil, invalidBindingf("%s contains an empty or duplicate ID", name)
		}
		seen[id] = true
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func contains(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}

func cloneErrors(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func invalidBindingf(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidBinding, fmt.Sprintf(format, arguments...))
}
