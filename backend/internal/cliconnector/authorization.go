package cliconnector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrAuthorizationPending = errors.New("authorization pending")
	ErrAuthorizationDenied  = errors.New("authorization denied")
	ErrAuthorizationExpired = errors.New("authorization expired")
)

type SetupChallenge struct {
	ActionURL    string
	ExpiresAt    time.Time
	Continuation []byte
}

type SetupResult struct {
	CredentialSlots map[string]string
	DisplayName     string
	ManagementURL   string
}

type AuthorizationChallenge struct {
	ActionURL    string
	ExpiresAt    time.Time
	Permissions  []string
	Continuation []byte
}

type AuthorizationResult struct {
	ExternalIdentityID  string
	ExternalDisplayName string
	Permissions         []string
	CredentialSlots     map[string]string
	ExpiresAt           time.Time
}

// AuthorizationAdapter is trusted platform code. Connector packages may only
// reference its registered Scheme identifier.
type AuthorizationAdapter interface {
	BeginSetup(context.Context) (SetupChallenge, error)
	PollSetup(context.Context, []byte) (SetupResult, error)
	BeginAuthorization(context.Context, map[string]string, []string) (AuthorizationChallenge, error)
	PollAuthorization(context.Context, map[string]string, []byte) (AuthorizationResult, error)
}

type AuthorizationRegistry struct {
	mu       sync.RWMutex
	adapters map[string]AuthorizationAdapter
}

// trustedAuthorizationSchemes is platform-owned policy. Connector packages may
// reference these logical Permissions but cannot add provider scopes themselves.
var trustedAuthorizationSchemes = map[string]map[string]string{
	"feishu": {
		"im:chat:readonly":        "im:chat:readonly",
		"im:message:send_as_user": "im:message:send_as_user",
	},
}

func ValidateAuthorizationReferences(scheme string, permissions []string) error {
	if scheme == "none" {
		if len(permissions) != 0 {
			return errors.New("Authorization-free Connector cannot declare Permissions")
		}
		return nil
	}
	catalog, found := trustedAuthorizationSchemes[scheme]
	if !found {
		return fmt.Errorf("Connector Authorization Scheme %q is not registered", scheme)
	}
	for _, permission := range permissions {
		if _, found := catalog[permission]; !found {
			return fmt.Errorf("Connector Permission %q is not registered for Scheme %q", permission, scheme)
		}
	}
	return nil
}

func NewAuthorizationRegistry() *AuthorizationRegistry {
	return &AuthorizationRegistry{adapters: map[string]AuthorizationAdapter{}}
}

func (registry *AuthorizationRegistry) Register(scheme string, adapter AuthorizationAdapter) error {
	if registry == nil || adapter == nil || ValidateAuthorizationReferences(scheme, nil) != nil {
		return errors.New("invalid Connector Authorization Adapter")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.adapters[scheme]; exists {
		return fmt.Errorf("Connector Authorization Scheme %q is already registered", scheme)
	}
	registry.adapters[scheme] = adapter
	return nil
}

func (registry *AuthorizationRegistry) Resolve(scheme string) (AuthorizationAdapter, error) {
	if registry == nil {
		return nil, errors.New("Connector Authorization registry is unavailable")
	}
	registry.mu.RLock()
	adapter := registry.adapters[scheme]
	registry.mu.RUnlock()
	if adapter == nil {
		return nil, fmt.Errorf("Connector Authorization Scheme %q is unavailable", scheme)
	}
	return adapter, nil
}
