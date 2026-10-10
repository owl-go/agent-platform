package domain

import (
	"fmt"
	"regexp"
)

type ExpertConnectorDependency struct {
	Source  string `json:"source"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

var dependencySource = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,127}$`)

func ValidateExpertDependencies(items []ExpertConnectorDependency) error {
	if len(items) > 50 {
		return fmt.Errorf("%w: too many Connector dependencies", ErrInvalid)
	}
	seen := map[string]bool{}
	for _, item := range items {
		if !dependencySource.MatchString(item.Source) || (item.Kind != "mcp" && item.Kind != "cli") || len(item.Version) > 100 || seen[item.Source] {
			return fmt.Errorf("%w: invalid Connector dependency", ErrInvalid)
		}
		seen[item.Source] = true
	}
	return nil
}
