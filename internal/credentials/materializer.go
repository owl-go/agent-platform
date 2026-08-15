package credentials

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Request struct {
	Ref       string
	Variables map[string]string
	Files     map[string][]byte
}

type Materializer struct {
	Root string
}

type Environment struct {
	directory string
	variables []string
	secrets   [][]byte
}

func (m Materializer) Create(request Request) (*Environment, error) {
	if request.Ref == "" {
		return nil, fmt.Errorf("credential environment ref is required")
	}
	directory, err := os.MkdirTemp(m.Root, "agent-run-credentials-*")
	if err != nil {
		return nil, fmt.Errorf("create credential directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		_ = os.RemoveAll(directory)
		return nil, fmt.Errorf("protect credential directory: %w", err)
	}

	environment := &Environment{directory: directory}
	variableNames := make([]string, 0, len(request.Variables))
	for name := range request.Variables {
		if !validEnvironmentName(name) {
			_ = environment.Cleanup()
			return nil, fmt.Errorf("credential variable name %q is invalid", name)
		}
		variableNames = append(variableNames, name)
	}
	sort.Strings(variableNames)
	for _, name := range variableNames {
		value := request.Variables[name]
		environment.variables = append(environment.variables, name+"="+value)
		environment.secrets = append(environment.secrets, []byte(value))
	}
	for name, contents := range request.Files {
		if !filepath.IsLocal(name) {
			_ = environment.Cleanup()
			return nil, fmt.Errorf("credential file path %q must be local", name)
		}
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			_ = environment.Cleanup()
			return nil, fmt.Errorf("create credential parent: %w", err)
		}
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			_ = environment.Cleanup()
			return nil, fmt.Errorf("write credential file: %w", err)
		}
		environment.secrets = append(environment.secrets, append([]byte(nil), contents...))
	}
	return environment, nil
}

func (e *Environment) Directory() string {
	return e.directory
}

func (e *Environment) Environ() []string {
	variables := make([]string, len(e.variables))
	copy(variables, e.variables)
	return variables
}

func (e *Environment) Redactor() *Redactor {
	return NewRedactor(e.secrets...)
}

func (e *Environment) Cleanup() error {
	err := os.RemoveAll(e.directory)
	for index := range e.variables {
		e.variables[index] = ""
	}
	e.variables = nil
	for _, secret := range e.secrets {
		clear(secret)
	}
	e.secrets = nil
	return err
}

func validEnvironmentName(name string) bool {
	if name == "" {
		return false
	}
	for index, value := range name {
		if value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || index > 0 && value >= '0' && value <= '9' {
			continue
		}
		return false
	}
	return true
}
