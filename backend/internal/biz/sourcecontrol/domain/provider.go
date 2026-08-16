package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	ErrProviderNotFound = errors.New("Source Control Provider not found")
	ErrConcurrentUpdate = errors.New("Source Control Provider was modified concurrently")
	ErrNameExists       = errors.New("Source Control Provider name already exists")
	ErrInvalidProvider  = errors.New("invalid Source Control Provider input")
)

type Kind string

const (
	GitHubCom         Kind = "github_com"
	GitLabSelfManaged Kind = "gitlab_self_managed"
)

type Provider struct {
	ID             string
	OrganizationID string
	Name           string
	Kind           Kind
	BaseURL        string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

type Registration struct {
	ID             string
	OrganizationID string
	Name           string
	Kind           Kind
	BaseURL        string
	Now            time.Time
}

func Register(registration Registration) (Provider, error) {
	if strings.TrimSpace(registration.ID) == "" || strings.TrimSpace(registration.OrganizationID) == "" || strings.TrimSpace(registration.Name) == "" {
		return Provider{}, invalidf("Source Control Provider ID, Organization ID, and name are required")
	}
	baseURL, err := validateBaseURL(registration.Kind, registration.BaseURL)
	if err != nil {
		return Provider{}, err
	}
	if registration.Now.IsZero() {
		return Provider{}, invalidf("registration time is required")
	}
	now := registration.Now.UTC()
	return Provider{
		ID: registration.ID, OrganizationID: registration.OrganizationID, Name: strings.TrimSpace(registration.Name),
		Kind: registration.Kind, BaseURL: baseURL, Enabled: true, CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func Restore(id, organizationID, name, kindValue, baseURL string, enabled bool, createdAt, updatedAt time.Time, version int64) (Provider, error) {
	kind := Kind(kindValue)
	normalized, err := validateBaseURL(kind, baseURL)
	if err != nil {
		return Provider{}, err
	}
	if id == "" || organizationID == "" || name == "" || createdAt.IsZero() || updatedAt.IsZero() || version <= 0 {
		return Provider{}, invalidf("invalid persisted Source Control Provider")
	}
	return Provider{
		ID: id, OrganizationID: organizationID, Name: name, Kind: kind, BaseURL: normalized,
		Enabled: enabled, CreatedAt: createdAt, UpdatedAt: updatedAt, Version: version,
	}, nil
}

func (provider *Provider) SetEnabled(enabled bool, now time.Time) error {
	if provider.Enabled == enabled {
		return nil
	}
	if now.IsZero() {
		return invalidf("status change time is required")
	}
	provider.Enabled = enabled
	provider.UpdatedAt = now.UTC()
	provider.Version++
	return nil
}

func validateBaseURL(kind Kind, value string) (string, error) {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", invalidf("Source Control Provider Base URL must be an absolute HTTPS origin")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = ""
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	switch kind {
	case GitHubCom:
		if !strings.EqualFold(parsed.Host, "github.com") || parsed.Path != "" {
			return "", invalidf("GitHub.com Provider Base URL must be https://github.com")
		}
		parsed.Host = "github.com"
	case GitLabSelfManaged:
		if strings.EqualFold(parsed.Host, "github.com") {
			return "", invalidf("self-managed GitLab Provider cannot use github.com")
		}
	default:
		return "", invalidf("unknown Source Control Provider kind %q", kind)
	}
	return strings.TrimSuffix(parsed.String(), "/"), nil
}

func invalidf(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidProvider, fmt.Sprintf(format, arguments...))
}
