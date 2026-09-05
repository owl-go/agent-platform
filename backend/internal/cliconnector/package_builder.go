package cliconnector

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// PackageBuildEnvironment is the only bridge to the credential-free build
// sandbox. It deliberately exposes structured values rather than shell text.
type PackageBuildEnvironment interface {
	Build(context.Context, PackageBuildRequest) (PackageArtifact, error)
}

type PackageBuildRequest struct {
	Package       string
	ExactVersion  string
	Architectures []string
}

type IsolatedPackageBuilder struct {
	environment PackageBuildEnvironment
}

func NewIsolatedPackageBuilder(environment PackageBuildEnvironment) (*IsolatedPackageBuilder, error) {
	if environment == nil {
		return nil, errors.New("isolated package build environment is required")
	}
	return &IsolatedPackageBuilder{environment: environment}, nil
}

func (builder *IsolatedPackageBuilder) Build(ctx context.Context, packageName, version string, architectures []string) (PackageArtifact, error) {
	if !npmPackage.MatchString(packageName) || !exactVersion.MatchString(version) {
		return PackageArtifact{}, errors.New("package build requires a valid npm package and exact version")
	}
	if len(architectures) == 0 {
		return PackageArtifact{}, errors.New("package build requires at least one architecture")
	}
	seen := make(map[string]struct{}, len(architectures))
	for _, architecture := range architectures {
		if architecture != "linux-amd64" && architecture != "linux-arm64" {
			return PackageArtifact{}, errors.New("package build contains an unsupported architecture")
		}
		if _, duplicate := seen[architecture]; duplicate {
			return PackageArtifact{}, errors.New("package build contains a duplicate architecture")
		}
		seen[architecture] = struct{}{}
	}
	artifact, err := builder.environment.Build(ctx, PackageBuildRequest{Package: packageName, ExactVersion: version, Architectures: slices.Clone(architectures)})
	if err != nil {
		return PackageArtifact{}, fmt.Errorf("isolated package build: %w", err)
	}
	if len(artifact.PackageBytes) == 0 || len(artifact.BundleBytes) == 0 || len(artifact.BundleBytes) > maxBundleSize {
		return PackageArtifact{}, errors.New("isolated package build returned an invalid artifact size")
	}
	return artifact, nil
}

var _ PackageBuilder = (*IsolatedPackageBuilder)(nil)
