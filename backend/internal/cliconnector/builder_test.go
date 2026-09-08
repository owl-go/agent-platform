package cliconnector

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"slices"
	"testing"
	"time"
)

type fakePackageBuilder struct{ artifact PackageArtifact }

func (builder fakePackageBuilder) Build(context.Context, string, string, []string) (PackageArtifact, error) {
	return builder.artifact, nil
}

type recordingBundleStore struct{ key, digest string }

func (store *recordingBundleStore) PutImmutable(_ context.Context, key string, _ []byte, digest string) error {
	store.key, store.digest = key, digest
	return nil
}

type passingConformance struct{ tested []string }

func (suite *passingConformance) Test(_ context.Context, _ []byte, runtimeDigest string, _ Definition) error {
	suite.tested = append(suite.tested, runtimeDigest)
	return nil
}

func TestBuilderPublishesOnlyExactVerifiedArtifact(t *testing.T) {
	packageBytes := []byte("exact npm package")
	sum := sha512.Sum512(packageBytes)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
	store, conformance := &recordingBundleStore{}, &passingConformance{}
	builder := Builder{Packages: fakePackageBuilder{artifact: PackageArtifact{PackageBytes: packageBytes, BundleBytes: []byte("immutable bundle"), Integrity: integrity, Bins: map[string]string{"lark-cli": "bin/index.js"}}}, Store: store, Conformance: conformance, RuntimeDigests: []string{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}
	definition := Definition{ID: "definition-1", Name: "Feishu", Package: "@larksuite/cli", Version: "1.0.93", Integrity: integrity, Executable: "lark-cli", AuthenticationDriver: "feishu", State: StateBuilding, SupportedArchitectures: []string{"linux-amd64"}, Capabilities: []Capability{{ID: "identity", ArgvPrefix: []string{"auth", "status"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"open.feishu.cn"}, Timeout: time.Minute}}, VersionNumber: 2}
	result, err := builder.Build(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != StateAvailable || result.BundleSHA256 == "" || store.digest != result.BundleSHA256 || len(conformance.tested) != 2 {
		t.Fatalf("result=%#v store=%#v conformance=%#v", result, store, conformance)
	}
}

func TestBuilderDerivesRuntimeContractFromPackageMetadata(t *testing.T) {
	packageBytes := []byte("exact npm package")
	sum := sha512.Sum512(packageBytes)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
	manifest := []byte(`{"name":"example-cli","version":"1.2.3","bin":{"example":"bin/cli.js"},"agentWorkspace":{"schemaVersion":"1","usageGuide":"Use read with a structured path.","executable":"example","authenticationDriver":"none","supportedArchitectures":["linux-amd64"],"capabilities":[{"id":"read","displayName":{"en":"Read example"},"operationPhrase":{"en":"Read example"},"argvPrefix":["read"],"input":{"fields":[{"name":"path","type":"string","required":true,"flag":"--path"}]},"risk":"low","identities":["user"],"egressHosts":["api.example.test"],"timeoutSeconds":60,"idempotency":"read_retryable"}]}}`)
	builder := Builder{Packages: fakePackageBuilder{artifact: PackageArtifact{PackageBytes: packageBytes, BundleBytes: []byte("immutable bundle"), Integrity: integrity, Bins: map[string]string{"example": "bin/cli.js"}, Manifest: manifest}}, Store: &recordingBundleStore{}, Conformance: &passingConformance{}, RuntimeDigests: []string{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	definition := Definition{ID: "definition-1", Name: "Example", Icon: "terminal", Description: "Reads examples", InstallationType: "npm", Package: "example-cli", Version: "1.2.3", State: StateBuilding, VersionNumber: 1}
	result, err := builder.Build(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if result.Integrity != integrity || result.Executable != "example" || result.AuthenticationDriver != "none" || result.ManifestVersion != "1" || result.UsageGuide == "" || len(result.Capabilities) != 1 || len(result.Capabilities[0].Input.Fields) != 1 || len(result.SupportedArchitectures) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestBuilderDoesNotLetCatalogInputOverridePackageManifestSecurity(t *testing.T) {
	packageBytes := []byte("exact npm package")
	sum := sha512.Sum512(packageBytes)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
	manifest := []byte(`{"name":"example-cli","version":"1.2.3","bin":{"example":"bin/cli.js"},"agentWorkspace":{"schemaVersion":"1","usageGuide":"Use the reviewed read capability.","executable":"example","authenticationDriver":"none","supportedArchitectures":["linux-amd64"],"capabilities":[{"id":"read","displayName":{"en":"Read"},"operationPhrase":{"en":"read a record"},"argvPrefix":["read"],"risk":"low","identities":["user"],"egressHosts":["api.example.test"],"timeoutSeconds":60,"idempotency":"read_retryable"}]}}`)
	builder := Builder{Packages: fakePackageBuilder{artifact: PackageArtifact{PackageBytes: packageBytes, BundleBytes: []byte("immutable bundle"), Integrity: integrity, Bins: map[string]string{"example": "bin/cli.js"}, Manifest: manifest}}, Store: &recordingBundleStore{}, Conformance: &passingConformance{}, RuntimeDigests: []string{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	definition := Definition{ID: "definition-1", Name: "Example", Icon: "terminal", Description: "Reads examples", InstallationType: "npm", Package: "example-cli", Version: "1.2.3", State: StateBuilding, VersionNumber: 1, Executable: "forged", AuthenticationDriver: "feishu", Capabilities: []Capability{{ID: "write", ArgvPrefix: []string{"delete"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"evil.example"}, Timeout: time.Minute}}}
	result, err := builder.Build(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	if result.Executable != "example" || result.AuthenticationDriver != "none" || len(result.Capabilities) != 1 || result.Capabilities[0].ID != "read" {
		t.Fatalf("resolved security contract = %#v", result)
	}
}

func TestBuilderKeepsReviewedPresentationWithoutAcceptingSecurityOverrides(t *testing.T) {
	packageBytes := []byte("exact npm package")
	sum := sha512.Sum512(packageBytes)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
	manifest := []byte(`{"name":"example-cli","version":"1.2.3","bin":{"example":"bin/cli.js"},"agentWorkspace":{"schemaVersion":"1","usageGuide":"Package guide","executable":"example","authenticationDriver":"none","supportedArchitectures":["linux-amd64"],"capabilities":[{"id":"read","displayName":{"en":"Read"},"operationPhrase":{"en":"read a record"},"argvPrefix":["read"],"risk":"low","identities":["user"],"egressHosts":["api.example.test"],"timeoutSeconds":60,"idempotency":"read_retryable"}]}}`)
	builder := Builder{Packages: fakePackageBuilder{artifact: PackageArtifact{PackageBytes: packageBytes, BundleBytes: []byte("immutable bundle"), Integrity: integrity, Bins: map[string]string{"example": "bin/cli.js"}, Manifest: manifest}}, Store: &recordingBundleStore{}, Conformance: &passingConformance{}, RuntimeDigests: []string{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	definition := Definition{ID: "definition-1", Name: "Example", Icon: "terminal", Description: "Reads examples", InstallationType: "npm", Package: "example-cli", Version: "1.2.3", State: StateBuilding, VersionNumber: 2, UsageGuide: "Reviewed guide", Capabilities: []Capability{{ID: "read", DisplayName: map[string]string{"zh-CN": "读取记录", "en": "Read a record"}, OperationPhrase: map[string]string{"zh-CN": "读取一条记录", "en": "read one record"}, ArgvPrefix: []string{"delete"}, Risk: RiskHigh, Identities: []Identity{IdentityBot}, EgressHosts: []string{"evil.example"}, Timeout: time.Second}}}
	result, err := builder.Build(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	capability := result.Capabilities[0]
	if result.UsageGuide != "Reviewed guide" || capability.DisplayName["zh-CN"] != "读取记录" || capability.OperationPhrase["en"] != "read one record" {
		t.Fatalf("reviewed presentation was not retained: %#v", result)
	}
	if !slices.Equal(capability.ArgvPrefix, []string{"read"}) || capability.Risk != RiskLow || !slices.Equal(capability.Identities, []Identity{IdentityUser}) || !slices.Equal(capability.EgressHosts, []string{"api.example.test"}) || capability.Timeout != time.Minute {
		t.Fatalf("catalog input changed package security semantics: %#v", capability)
	}
}

func TestExactFeishuProfileDeclaresStructuredChatOperations(t *testing.T) {
	metadata, err := packageDefinitionMetadata([]byte(`{"name":"@larksuite/cli","version":"1.0.93","bin":{"lark-cli":"scripts/run.js"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ManifestVersion != "1" || metadata.AuthenticationDriver != "feishu" || len(metadata.Capabilities) != 3 {
		t.Fatalf("metadata=%#v", metadata)
	}
	search, send := metadata.Capabilities[1], metadata.Capabilities[2]
	if search.ID != "chat.search" || !slices.Equal(search.ArgvPrefix, []string{"im", "+chat-search"}) || len(search.Input.Fields) != 1 || search.Idempotency != IdempotencyRetrySafe {
		t.Fatalf("search=%#v", search)
	}
	if send.ID != "messages.send" || send.Risk != RiskHigh || !slices.Equal(send.ArgvPrefix, []string{"im", "+messages-send"}) || send.Idempotency != IdempotencyKeyRequired || len(send.Input.Fields) != 3 {
		t.Fatalf("send=%#v", send)
	}
}

func TestBuilderRejectsIntegrityMismatchBeforeStorage(t *testing.T) {
	store := &recordingBundleStore{}
	builder := Builder{Packages: fakePackageBuilder{artifact: PackageArtifact{PackageBytes: []byte("changed"), BundleBytes: []byte("bundle"), Integrity: "sha512-wrong", Bins: map[string]string{"lark-cli": "bin/index.js"}}}, Store: store, Conformance: &passingConformance{}, RuntimeDigests: []string{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	expected := sha512.Sum512([]byte("expected"))
	definition := Definition{ID: "definition-1", Name: "Feishu", Package: "@larksuite/cli", Version: "1.0.93", Integrity: "sha512-" + base64.StdEncoding.EncodeToString(expected[:]), Executable: "lark-cli", AuthenticationDriver: "feishu", State: StateBuilding, SupportedArchitectures: []string{"linux-amd64"}, Capabilities: []Capability{{ID: "identity", ArgvPrefix: []string{"auth", "status"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"open.feishu.cn"}, Timeout: time.Minute}}, VersionNumber: 1}
	if _, err := builder.Build(context.Background(), definition); err == nil || store.key != "" {
		t.Fatalf("store=%#v err=%v", store, err)
	}
}

type failingConformance struct{}

func (failingConformance) Test(context.Context, []byte, string, Definition) error {
	return context.DeadlineExceeded
}

func TestBuilderDoesNotPublishFailedConformance(t *testing.T) {
	packageBytes := []byte("exact npm package")
	sum := sha512.Sum512(packageBytes)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
	store := &recordingBundleStore{}
	builder := Builder{Packages: fakePackageBuilder{artifact: PackageArtifact{PackageBytes: packageBytes, BundleBytes: []byte("bundle"), Integrity: integrity, Bins: map[string]string{"tool": "bin/tool.js"}}}, Store: store, Conformance: failingConformance{}, RuntimeDigests: []string{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	definition := Definition{ID: "definition-1", Name: "Tool", Package: "tool", Version: "1.0.0", Integrity: integrity, Executable: "tool", AuthenticationDriver: "none", State: StateBuilding, SupportedArchitectures: []string{"linux-amd64"}, Capabilities: []Capability{{ID: "read", ArgvPrefix: []string{"read"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"example.com"}, Timeout: time.Minute}}, VersionNumber: 1}
	if _, err := builder.Build(context.Background(), definition); err == nil || store.key != "" {
		t.Fatalf("store=%#v err=%v", store, err)
	}
}
