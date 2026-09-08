package cliconnector

import (
	"context"
	"testing"
)

type authorizationAdapterStub struct{}

func (authorizationAdapterStub) BeginSetup(context.Context) (SetupChallenge, error) {
	return SetupChallenge{}, nil
}
func (authorizationAdapterStub) PollSetup(context.Context, []byte) (SetupResult, error) {
	return SetupResult{}, nil
}
func (authorizationAdapterStub) BeginAuthorization(context.Context, map[string]string, []string) (AuthorizationChallenge, error) {
	return AuthorizationChallenge{}, nil
}
func (authorizationAdapterStub) PollAuthorization(context.Context, map[string]string, []byte) (AuthorizationResult, error) {
	return AuthorizationResult{}, nil
}

func TestAuthorizationRegistryRejectsUnknownAndDuplicateSchemes(t *testing.T) {
	registry := NewAuthorizationRegistry()
	adapter := authorizationAdapterStub{}
	if err := registry.Register("feishu", adapter); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve("feishu"); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("feishu", adapter); err == nil {
		t.Fatal("duplicate Authorization Scheme was accepted")
	}
	if _, err := registry.Resolve("unknown"); err == nil {
		t.Fatal("unknown Authorization Scheme was accepted")
	}
}
