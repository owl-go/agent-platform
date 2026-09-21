package connectorpackage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var (
	sourcePattern       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	semverPattern       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	digestPattern       = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	identifierPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	markdownLinkPattern = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
)

func validateSkillResources(directory string, body []byte, files map[string][]byte) error {
	for _, match := range markdownLinkPattern.FindAllSubmatch(body, -1) {
		target := strings.TrimSpace(string(match[1]))
		if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") {
			continue
		}
		target = strings.SplitN(target, "#", 2)[0]
		if path.IsAbs(target) {
			return fmt.Errorf("%s references a resource outside its directory", directory)
		}
		cleaned := path.Clean(path.Join(directory, target))
		if cleaned != directory && !strings.HasPrefix(cleaned, directory+"/") {
			return fmt.Errorf("%s references a resource outside its directory", directory)
		}
		if _, ok := files[cleaned]; !ok {
			return fmt.Errorf("%s references a missing resource %s", directory, target)
		}
	}
	return nil
}

func decodeStrict[T any](name string, body []byte, target *T) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s contains multiple JSON values", name)
	}
	return nil
}

func validateMetadata(meta Metadata) error {
	if !sourcePattern.MatchString(meta.Source) {
		return fmt.Errorf("connector-meta.json source must use lower-case letters, digits, and hyphens")
	}
	if !semverPattern.MatchString(meta.Version) || !semverPattern.MatchString(meta.MinPlatformVersion) {
		return fmt.Errorf("connector-meta.json requires semantic version and minPlatformVersion")
	}
	if meta.MaxPlatformVersion != "" && !semverPattern.MatchString(meta.MaxPlatformVersion) {
		return fmt.Errorf("connector-meta.json maxPlatformVersion must be a semantic version")
	}
	if meta.Type != TypeMCP && meta.Type != TypeCLI {
		return fmt.Errorf("connector-meta.json type must be mcp or cli")
	}
	if strings.TrimSpace(meta.Name) == "" || strings.TrimSpace(meta.Description) == "" {
		return fmt.Errorf("connector-meta.json name and description are required")
	}
	if len(meta.ExamplesZH) == 0 || len(meta.ExamplesEN) == 0 {
		return fmt.Errorf("connector-meta.json requires Chinese and English examples")
	}
	if meta.AuthMode != "oauth" && meta.AuthMode != "none" && meta.AuthMode != "cli" {
		return fmt.Errorf("connector-meta.json auth_mode must be oauth, cli, or none")
	}
	return nil
}

func validateMCP(manifest MCPManifest) error {
	if manifest.TimeoutSeconds < 1 || manifest.TimeoutSeconds > 3600 {
		return fmt.Errorf("mcp.json timeout_seconds must be between 1 and 3600")
	}
	if err := validateHosts("mcp.json egress_hosts", manifest.EgressHosts); err != nil {
		return err
	}
	switch manifest.Transport {
	case "streamable_http":
		parsed, err := url.Parse(manifest.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("mcp.json remote URL must use HTTPS")
		}
		for key := range parsed.Query() {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key") || strings.Contains(lower, "authorization") {
				return fmt.Errorf("mcp.json remote URL must not contain credentials")
			}
		}
		if manifest.Runner != "" || manifest.Package != "" || manifest.PackageVersion != "" {
			return fmt.Errorf("mcp.json remote transport cannot declare a local runner")
		}
	case "stdio":
		if manifest.Runner != "npx" && manifest.Runner != "uvx" {
			return fmt.Errorf("mcp.json stdio runner must be npx or uvx")
		}
		if strings.TrimSpace(manifest.Package) == "" || !semverPattern.MatchString(manifest.PackageVersion) {
			return fmt.Errorf("mcp.json stdio requires a package and exact semantic version")
		}
		if manifest.URL != "" {
			return fmt.Errorf("mcp.json stdio cannot declare a URL")
		}
	default:
		return fmt.Errorf("mcp.json transport is unsupported")
	}
	for key, value := range manifest.Headers {
		lower := strings.ToLower(key)
		if (strings.Contains(lower, "authorization") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key")) && !variableReference(value) {
			return fmt.Errorf("mcp.json contains a literal secret in header %s", key)
		}
	}
	if err := validateLimits(manifest.Limits); err != nil {
		return fmt.Errorf("mcp.json %w", err)
	}
	return validateEnvironment("mcp.json", manifest.Environment)
}

func validateCLI(manifest CLIManifest) error {
	if manifest.Runtime.Kind != "node" && manifest.Runtime.Kind != "python" {
		return fmt.Errorf("cli.json runtime kind must be node or python")
	}
	if !semverPattern.MatchString(manifest.Runtime.Version) || !digestPattern.MatchString(manifest.Runtime.Digest) {
		return fmt.Errorf("cli.json runtime requires an exact version and sha256 digest")
	}
	if !identifierPattern.MatchString(manifest.Executable) {
		return fmt.Errorf("cli.json executable is invalid")
	}
	if manifest.BundlePath != "" {
		cleaned := path.Clean(strings.TrimPrefix(manifest.BundlePath, "./"))
		if cleaned == "." || path.IsAbs(manifest.BundlePath) || strings.HasPrefix(cleaned, "../") || strings.ContainsAny(manifest.BundlePath, "\\\x00\r\n") {
			return fmt.Errorf("cli.json bundle_path is unsafe")
		}
	}
	commands := map[string]LifecycleCommand{"init": manifest.Commands.Init, "auth": manifest.Commands.Auth, "status": manifest.Commands.Status, "unAuth": manifest.Commands.UnAuth}
	for name, command := range commands {
		if len(command.Argv) == 0 {
			return fmt.Errorf("cli.json command %s requires argv", name)
		}
		for _, argument := range command.Argv {
			if strings.ContainsRune(argument, 0) || strings.ContainsAny(argument, "\r\n") {
				return fmt.Errorf("cli.json command %s contains an unsafe argument", name)
			}
		}
	}
	if manifest.StatusMatch.JSONPath == "" || manifest.StatusMatch.Equals == nil {
		return fmt.Errorf("cli.json status_match is required")
	}
	if manifest.TimeoutSeconds < 1 || manifest.TimeoutSeconds > 3600 {
		return fmt.Errorf("cli.json timeout_seconds must be between 1 and 3600")
	}
	if err := validateHosts("cli.json egress_hosts", manifest.EgressHosts); err != nil {
		return err
	}
	if err := validateHosts("cli.json auth_url_domains", manifest.AuthURLDomains); err != nil && len(manifest.AuthURLDomains) > 0 {
		return err
	}
	if err := validateLimits(manifest.Limits); err != nil {
		return fmt.Errorf("cli.json %w", err)
	}
	seenCapabilities := map[string]struct{}{}
	for _, capability := range manifest.Capabilities {
		if capability.ID == "" || len(capability.ArgvPrefix) == 0 || capability.TimeoutSeconds < 1 || capability.TimeoutSeconds > 900 {
			return fmt.Errorf("cli.json capability %s is invalid", capability.ID)
		}
		if _, duplicate := seenCapabilities[capability.ID]; duplicate {
			return fmt.Errorf("cli.json capability %s is duplicated", capability.ID)
		}
		seenCapabilities[capability.ID] = struct{}{}
		if capability.Risk != "low" && capability.Risk != "high" {
			return fmt.Errorf("cli.json capability %s has unsupported risk", capability.ID)
		}
		if len(capability.Identities) == 0 || len(capability.EgressHosts) == 0 {
			return fmt.Errorf("cli.json capability %s requires identities and egress_hosts", capability.ID)
		}
		for _, identity := range capability.Identities {
			if identity != "user" && identity != "bot" {
				return fmt.Errorf("cli.json capability %s has unsupported identity", capability.ID)
			}
		}
		for _, argument := range capability.ArgvPrefix {
			if argument == "" || strings.ContainsAny(argument, "\x00\r\n") {
				return fmt.Errorf("cli.json capability %s contains an unsafe argv prefix", capability.ID)
			}
		}
		if err := validateHosts("cli.json capability egress_hosts", capability.EgressHosts); err != nil {
			return err
		}
	}
	return validateEnvironment("cli.json", manifest.Environment)
}

func validateLimits(limits ResourceLimits) error {
	if limits.CPU < 0 || limits.CPU > 16000 || limits.MemoryMiB < 0 || limits.MemoryMiB > 65536 || limits.Concurrency < 0 || limits.Concurrency > 256 || limits.ChildProcesses < 0 || limits.ChildProcesses > 1024 || limits.TimeoutSeconds < 0 || limits.TimeoutSeconds > 3600 {
		return fmt.Errorf("resource_limits are outside the allowed range")
	}
	return nil
}

func validateHosts(field string, hosts []string) error {
	if len(hosts) == 0 {
		return fmt.Errorf("%s must declare at least one host", field)
	}
	seen := map[string]struct{}{}
	for _, host := range hosts {
		if host != strings.ToLower(host) || net.ParseIP(host) != nil || strings.ContainsAny(host, "/:@* ") || !strings.Contains(host, ".") {
			return fmt.Errorf("%s contains an invalid domain", field)
		}
		if _, duplicate := seen[host]; duplicate {
			return fmt.Errorf("%s contains a duplicate domain", field)
		}
		seen[host] = struct{}{}
	}
	return nil
}

func validateEnvironment(file string, values []EnvironmentVariable) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		if !identifierPattern.MatchString(value.Name) {
			return fmt.Errorf("%s environment contains an invalid name", file)
		}
		if _, duplicate := seen[value.Name]; duplicate {
			return fmt.Errorf("%s environment contains a duplicate name", file)
		}
		seen[value.Name] = struct{}{}
		if !variableReference(value.Value) {
			return fmt.Errorf("%s environment %s must reference a platform variable", file, value.Name)
		}
	}
	return nil
}

func variableReference(value string) bool {
	return strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") && identifierPattern.MatchString(strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}"))
}

func parseSkill(directory string, body []byte) (Skill, error) {
	if len(body) == 0 || len(body) > 1<<20 || !utf8.Valid(body) {
		return Skill{}, fmt.Errorf("%s must be bounded UTF-8 text", path.Join(directory, "SKILL.md"))
	}
	normalized := strings.ReplaceAll(string(body), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return Skill{}, fmt.Errorf("%s must start with YAML frontmatter", path.Join(directory, "SKILL.md"))
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return Skill{}, fmt.Errorf("%s has unterminated YAML frontmatter", path.Join(directory, "SKILL.md"))
	}
	var values struct {
		Name        string `yaml:"name"`
		DisplayName string `yaml:"display_name"`
		Description string `yaml:"description"`
		Version     string `yaml:"version"`
		Author      string `yaml:"author"`
	}
	if err := yaml.Unmarshal([]byte(normalized[4:4+end]), &values); err != nil {
		return Skill{}, fmt.Errorf("parse %s frontmatter: %w", path.Join(directory, "SKILL.md"), err)
	}
	if !sourcePattern.MatchString(values.Name) || strings.TrimSpace(values.DisplayName) == "" || strings.TrimSpace(values.Description) == "" || !semverPattern.MatchString(values.Version) || strings.TrimSpace(values.Author) == "" {
		return Skill{}, fmt.Errorf("%s requires name, display_name, description, semantic version, and author", path.Join(directory, "SKILL.md"))
	}
	if strings.TrimSpace(normalized[4+end+5:]) == "" {
		return Skill{}, fmt.Errorf("%s requires instructions", path.Join(directory, "SKILL.md"))
	}
	return Skill{Directory: directory, Name: values.Name, DisplayName: strings.TrimSpace(values.DisplayName), Description: strings.TrimSpace(values.Description), Version: values.Version, Author: strings.TrimSpace(values.Author)}, nil
}
