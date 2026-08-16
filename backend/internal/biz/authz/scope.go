package authz

import "strings"

// ReadScope is the database predicate an authenticated Principal may use for
// scope-sensitive resource lookups. It contains no transport or persistence
// details and is safe to share with bounded-context repository ports.
type ReadScope struct {
	OrganizationID string
	TeamIDs        []string
	AllTeams       bool
}

func (scope ReadScope) Valid() bool {
	if strings.TrimSpace(scope.OrganizationID) == "" {
		return false
	}
	if scope.AllTeams {
		return true
	}
	for _, teamID := range scope.TeamIDs {
		if strings.TrimSpace(teamID) != "" {
			return true
		}
	}
	return false
}
