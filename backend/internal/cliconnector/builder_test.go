package cliconnector

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
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
