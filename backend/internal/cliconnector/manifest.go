package cliconnector

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ManifestSource string

const (
	ManifestSourceProfile ManifestSource = "profile"
	ManifestSourcePackage ManifestSource = "package"
)

type InputType string

const (
	InputString  InputType = "string"
	InputInteger InputType = "integer"
	InputBoolean InputType = "boolean"
)

type Idempotency string

const (
	IdempotencyRetrySafe   Idempotency = "read_retryable"
	IdempotencyKeyRequired Idempotency = "idempotency_key"
	IdempotencyUnknown     Idempotency = "non_idempotent"
)

type InputField struct {
	Name      string    `json:"name"`
	Type      InputType `json:"type"`
	Required  bool      `json:"required,omitempty"`
	Flag      string    `json:"flag,omitempty"`
	Sensitive bool      `json:"sensitive,omitempty"`
	Enum      []string  `json:"enum,omitempty"`
}

type InputSchema struct {
	Fields []InputField `json:"fields,omitempty"`
}

type AuthorizationDeclaration struct {
	Scheme      string   `json:"scheme"`
	Permissions []string `json:"permissions,omitempty"`
}

type Manifest struct {
	SchemaVersion string                   `json:"schema_version"`
	Authorization AuthorizationDeclaration `json:"authorization"`
	UsageGuide    string                   `json:"usage_guide"`
	Capabilities  []Capability             `json:"capabilities"`
}

type ManifestResolution struct {
	Source            ManifestSource
	ArtifactSHA256    string
	DefinitionVersion int64
	Reviewed          bool
	Conformant        bool
}

type ResolvedManifest struct {
	Manifest
	Source            ManifestSource `json:"source"`
	ArtifactSHA256    string         `json:"artifact_sha256"`
	DefinitionVersion int64          `json:"definition_version"`
}

var inputFieldName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var inputFlag = regexp.MustCompile(`^--[a-z][a-z0-9-]{0,63}$`)
var localeTag = regexp.MustCompile(`^[a-z]{2}(?:-[A-Z]{2})?$`)

func ResolveManifest(manifest Manifest, resolution ManifestResolution) (ResolvedManifest, error) {
	if resolution.Source != ManifestSourceProfile && resolution.Source != ManifestSourcePackage {
		return ResolvedManifest{}, errors.New("trusted Manifest source is required")
	}
	if !resolution.Reviewed || !resolution.Conformant {
		return ResolvedManifest{}, errors.New("Manifest review and Conformance are required")
	}
	if !bundleDigest.MatchString(resolution.ArtifactSHA256) || resolution.DefinitionVersion <= 0 {
		return ResolvedManifest{}, errors.New("exact Manifest artifact and Definition version are required")
	}
	if err := validateManifest(manifest); err != nil {
		return ResolvedManifest{}, err
	}
	return ResolvedManifest{Manifest: cloneManifest(manifest), Source: resolution.Source, ArtifactSHA256: resolution.ArtifactSHA256, DefinitionVersion: resolution.DefinitionVersion}, nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != "1" {
		return errors.New("unsupported Connector Manifest schema version")
	}
	if manifest.Authorization.Scheme != "none" && !policyToken.MatchString(manifest.Authorization.Scheme) {
		return errors.New("invalid Connector Authorization Scheme")
	}
	if strings.TrimSpace(manifest.UsageGuide) == "" || len(manifest.UsageGuide) > 32*1024 {
		return errors.New("Connector Manifest requires a bounded reviewed Usage Guide")
	}
	if len(manifest.Capabilities) == 0 {
		return errors.New("Connector Manifest requires a capability")
	}
	if err := ValidateAuthorizationReferences(manifest.Authorization.Scheme, manifest.Authorization.Permissions); err != nil {
		return err
	}
	definition := Definition{Executable: "manifest-validation", AuthenticationDriver: "none", Capabilities: manifest.Capabilities}
	if err := validateExecutionPolicy(definition); err != nil {
		return err
	}
	for _, capability := range manifest.Capabilities {
		if capability.Idempotency != IdempotencyRetrySafe && capability.Idempotency != IdempotencyKeyRequired && capability.Idempotency != IdempotencyUnknown {
			return fmt.Errorf("capability %q requires an idempotency policy", capability.ID)
		}
		if len(capability.DisplayName) == 0 || len(capability.OperationPhrase) == 0 {
			return fmt.Errorf("capability %q requires display semantics", capability.ID)
		}
		if err := validateLocalizedText(capability.DisplayName); err != nil {
			return fmt.Errorf("capability %q display name: %w", capability.ID, err)
		}
		if err := validateLocalizedText(capability.OperationPhrase); err != nil {
			return fmt.Errorf("capability %q operation phrase: %w", capability.ID, err)
		}
		if capability.Idempotency == IdempotencyKeyRequired {
			var hasRequiredKey bool
			for _, field := range capability.Input.Fields {
				hasRequiredKey = hasRequiredKey || (field.Name == "idempotency_key" && field.Required && field.Type == InputString)
			}
			if !hasRequiredKey {
				return fmt.Errorf("capability %q requires a required string idempotency_key input", capability.ID)
			}
		}
		if manifest.Authorization.Scheme == "none" && len(capability.Scopes) > 0 {
			return fmt.Errorf("capability %q cannot request Permissions without an Authorization Scheme", capability.ID)
		}
		for _, permission := range capability.Scopes {
			found := false
			for _, declared := range manifest.Authorization.Permissions {
				found = found || permission == declared
			}
			if !found {
				return fmt.Errorf("capability %q references undeclared Permission %q", capability.ID, permission)
			}
		}
	}
	return nil
}

func validateLocalizedText(values map[string]string) error {
	if len(values) == 0 || len(values) > 8 {
		return errors.New("localized text requires between one and eight entries")
	}
	for locale, value := range values {
		if !localeTag.MatchString(locale) || strings.TrimSpace(value) == "" || len(value) > 200 {
			return errors.New("invalid localized text")
		}
	}
	return nil
}

func validateInputSchema(schema InputSchema) error {
	seen := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		if !inputFieldName.MatchString(field.Name) {
			return errors.New("invalid Connector input field")
		}
		if _, exists := seen[field.Name]; exists {
			return errors.New("duplicate Connector input field")
		}
		seen[field.Name] = struct{}{}
		if field.Type != InputString && field.Type != InputInteger && field.Type != InputBoolean {
			return errors.New("unsupported Connector input type")
		}
		if field.Flag != "" && !inputFlag.MatchString(field.Flag) {
			return errors.New("unsafe Connector input flag")
		}
		if field.Type == InputBoolean && field.Flag == "" {
			return errors.New("boolean Connector input requires a flag")
		}
		for _, value := range field.Enum {
			if value == "" || strings.ContainsRune(value, '\x00') {
				return errors.New("invalid Connector input enum")
			}
		}
	}
	return nil
}

func renderStructuredArguments(capability Capability, input map[string]any) ([]string, error) {
	if err := validateInputSchema(capability.Input); err != nil {
		return nil, err
	}
	known := make(map[string]InputField, len(capability.Input.Fields))
	for _, field := range capability.Input.Fields {
		known[field.Name] = field
	}
	for name := range input {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("unknown Connector input field %q", name)
		}
	}
	arguments := append([]string(nil), capability.ArgvPrefix...)
	for _, field := range capability.Input.Fields {
		value, present := input[field.Name]
		if !present {
			if field.Required {
				return nil, fmt.Errorf("Connector input field %q is required", field.Name)
			}
			continue
		}
		rendered, include, err := renderInputValue(field, value)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		if field.Flag != "" {
			arguments = append(arguments, field.Flag)
		}
		if field.Type != InputBoolean {
			arguments = append(arguments, rendered)
		}
	}
	return arguments, nil
}

func renderInputValue(field InputField, value any) (string, bool, error) {
	var rendered string
	switch field.Type {
	case InputString:
		text, ok := value.(string)
		if !ok || strings.ContainsRune(text, '\x00') || len(text) > 64*1024 {
			return "", false, fmt.Errorf("Connector input field %q must be a valid string", field.Name)
		}
		rendered = text
	case InputInteger:
		number, ok := value.(float64)
		if !ok || number != float64(int64(number)) {
			return "", false, fmt.Errorf("Connector input field %q must be an integer", field.Name)
		}
		rendered = strconv.FormatInt(int64(number), 10)
	case InputBoolean:
		boolean, ok := value.(bool)
		if !ok {
			return "", false, fmt.Errorf("Connector input field %q must be a boolean", field.Name)
		}
		return "", boolean, nil
	}
	if len(field.Enum) > 0 {
		allowed := false
		for _, candidate := range field.Enum {
			allowed = allowed || rendered == candidate
		}
		if !allowed {
			return "", false, fmt.Errorf("Connector input field %q is outside its enum", field.Name)
		}
	}
	return rendered, true, nil
}

func cloneManifest(value Manifest) Manifest {
	result := value
	result.Authorization.Permissions = append([]string(nil), value.Authorization.Permissions...)
	result.Capabilities = append([]Capability(nil), value.Capabilities...)
	for index := range result.Capabilities {
		result.Capabilities[index] = cloneCapability(value.Capabilities[index])
	}
	return result
}

func cloneCapability(value Capability) Capability {
	result := value
	result.ArgvPrefix = append([]string(nil), value.ArgvPrefix...)
	result.Identities = append([]Identity(nil), value.Identities...)
	result.Scopes = append([]string(nil), value.Scopes...)
	result.EgressHosts = append([]string(nil), value.EgressHosts...)
	result.Input.Fields = append([]InputField(nil), value.Input.Fields...)
	for index := range result.Input.Fields {
		result.Input.Fields[index].Enum = append([]string(nil), value.Input.Fields[index].Enum...)
	}
	result.DisplayName = cloneStrings(value.DisplayName)
	result.OperationPhrase = cloneStrings(value.OperationPhrase)
	return result
}

func cloneStrings(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
