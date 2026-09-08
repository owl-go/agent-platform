package cliconnector

import (
	"strings"
	"testing"
	"time"
)

func TestResolveManifestFreezesReviewedNoAuthorizationCapability(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "1",
		Authorization: AuthorizationDeclaration{Scheme: "none"},
		UsageGuide:    "Use search to find a document by title.",
		Capabilities: []Capability{{
			ID: "search", DisplayName: map[string]string{"en": "Search documents"}, OperationPhrase: map[string]string{"en": "Searched documents"},
			ArgvPrefix: []string{"documents", "search"}, Risk: RiskLow, Identities: []Identity{IdentityUser},
			EgressHosts: []string{"api.example.test"}, Timeout: time.Minute, Idempotency: IdempotencyRetrySafe,
			Input: InputSchema{Fields: []InputField{{Name: "query", Type: InputString, Required: true, Flag: "--query"}}},
		}},
	}
	resolved, err := ResolveManifest(manifest, ManifestResolution{
		Source: ManifestSourcePackage, ArtifactSHA256: strings.Repeat("a", 64), DefinitionVersion: 3, Reviewed: true, Conformant: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.SchemaVersion != "1" || resolved.DefinitionVersion != 3 || resolved.Source != ManifestSourcePackage || resolved.Capabilities[0].ID != "search" {
		t.Fatalf("resolved=%#v", resolved)
	}
}

func TestResolveManifestRejectsUnreviewedOrUnsafeDraft(t *testing.T) {
	manifest := Manifest{SchemaVersion: "1", Authorization: AuthorizationDeclaration{Scheme: "none"}, Capabilities: []Capability{{
		ID: "search", ArgvPrefix: []string{"search"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"api.example.test"}, Timeout: time.Minute,
		Input: InputSchema{Fields: []InputField{{Name: "query", Type: InputString, Required: true, Flag: "--query; rm"}}}, Idempotency: IdempotencyRetrySafe,
	}}}
	if _, err := ResolveManifest(manifest, ManifestResolution{Source: ManifestSourcePackage, ArtifactSHA256: strings.Repeat("a", 64), DefinitionVersion: 1, Reviewed: true, Conformant: true}); err == nil {
		t.Fatal("expected unsafe mapping to fail")
	}
	manifest.Capabilities[0].Input.Fields[0].Flag = "--query"
	if _, err := ResolveManifest(manifest, ManifestResolution{Source: ManifestSourcePackage, ArtifactSHA256: strings.Repeat("a", 64), DefinitionVersion: 1, Conformant: true}); err == nil {
		t.Fatal("expected unreviewed manifest to fail")
	}
}

func TestResolveManifestRequiresStableKeyForIdempotentWrites(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "1", Authorization: AuthorizationDeclaration{Scheme: "none"}, UsageGuide: "Create a record exactly once.",
		Capabilities: []Capability{{
			ID: "create", DisplayName: map[string]string{"en": "Create record"}, OperationPhrase: map[string]string{"en": "Created a record"},
			ArgvPrefix: []string{"records", "create"}, Risk: RiskHigh, Identities: []Identity{IdentityUser}, EgressHosts: []string{"api.example.test"}, Timeout: time.Minute,
			Idempotency: IdempotencyKeyRequired,
		}},
	}
	resolution := ManifestResolution{Source: ManifestSourcePackage, ArtifactSHA256: strings.Repeat("a", 64), DefinitionVersion: 1, Reviewed: true, Conformant: true}
	if _, err := ResolveManifest(manifest, resolution); err == nil || !strings.Contains(err.Error(), "idempotency_key") {
		t.Fatalf("error=%v", err)
	}
	manifest.Capabilities[0].Input = InputSchema{Fields: []InputField{{Name: "idempotency_key", Type: InputString, Required: true, Flag: "--idempotency-key"}}}
	if _, err := ResolveManifest(manifest, resolution); err != nil {
		t.Fatal(err)
	}
}

func TestResolveManifestRejectsPermissionOutsideTrustedSchemeCatalog(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: "1", Authorization: AuthorizationDeclaration{Scheme: "feishu", Permissions: []string{"contact:write"}}, UsageGuide: "Read contacts.",
		Capabilities: []Capability{{ID: "contacts.read", DisplayName: map[string]string{"en": "Read contacts"}, OperationPhrase: map[string]string{"en": "Read contacts"}, ArgvPrefix: []string{"contacts", "read"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, Scopes: []string{"contact:write"}, EgressHosts: []string{"open.feishu.cn"}, Timeout: time.Minute, Idempotency: IdempotencyRetrySafe}},
	}
	_, err := ResolveManifest(manifest, ManifestResolution{Source: ManifestSourcePackage, ArtifactSHA256: strings.Repeat("a", 64), DefinitionVersion: 1, Reviewed: true, Conformant: true})
	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("error=%v", err)
	}
}
