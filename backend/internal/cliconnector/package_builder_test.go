package cliconnector

import (
	"context"
	"testing"
)

type recordingBuildEnvironment struct {
	request PackageBuildRequest
}

func (environment *recordingBuildEnvironment) Build(_ context.Context, request PackageBuildRequest) (PackageArtifact, error) {
	environment.request = request
	return PackageArtifact{PackageBytes: []byte("package"), BundleBytes: []byte("bundle")}, nil
}

func TestIsolatedPackageBuilderPassesOnlyStructuredExactInput(t *testing.T) {
	environment := &recordingBuildEnvironment{}
	builder, err := NewIsolatedPackageBuilder(environment)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Build(context.Background(), "@larksuite/cli", "1.0.93", []string{"linux-amd64"}); err != nil {
		t.Fatal(err)
	}
	if environment.request.Package != "@larksuite/cli" || environment.request.ExactVersion != "1.0.93" || len(environment.request.Architectures) != 1 {
		t.Fatalf("request = %#v", environment.request)
	}
}

func TestIsolatedPackageBuilderRejectsInstallSyntax(t *testing.T) {
	environment := &recordingBuildEnvironment{}
	builder, err := NewIsolatedPackageBuilder(environment)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Build(context.Background(), "@larksuite/cli && curl example.com", "latest", []string{"linux-amd64"}); err == nil {
		t.Fatal("expected unsafe package input to be rejected")
	}
	if environment.request.Package != "" {
		t.Fatal("unsafe input reached the build environment")
	}
}
