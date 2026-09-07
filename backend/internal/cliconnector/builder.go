package cliconnector

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

// PackageArtifact is produced inside the isolated package-build environment.
type PackageArtifact struct {
	PackageBytes []byte
	BundleBytes  []byte
	Integrity    string
	Bins         map[string]string
	Manifest     []byte
}

type PackageBuilder interface {
	Build(context.Context, string, string, []string) (PackageArtifact, error)
}

type BundleStore interface {
	PutImmutable(context.Context, string, []byte, string) error
}

type SourceStore interface {
	GetSourceVerified(context.Context, string, string) ([]byte, error)
}

type UploadPackageBuilder interface {
	Build(context.Context, []byte) (PackageArtifact, error)
}

type Conformance interface {
	Test(context.Context, []byte, string, Definition) error
}

type Builder struct {
	Packages       PackageBuilder
	Uploads        UploadPackageBuilder
	Store          BundleStore
	Sources        SourceStore
	Conformance    Conformance
	RuntimeDigests []string
}

type BuildResult struct {
	State                  State
	BundleObjectKey        string
	BundleSHA256           string
	RuntimeDigests         []string
	Package                string
	Version                string
	Integrity              string
	Executable             string
	AuthenticationDriver   string
	Capabilities           []Capability
	SupportedArchitectures []string
}

var runtimeDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var definitionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (builder Builder) Build(ctx context.Context, definition Definition) (BuildResult, error) {
	if builder.Packages == nil || builder.Store == nil || builder.Conformance == nil {
		return BuildResult{}, errors.New("CLI builder ports are required")
	}
	if definition.State != StateBuilding {
		return BuildResult{}, errors.New("CLI definition must be building")
	}
	if err := definition.ValidateDraft(); err != nil {
		return BuildResult{}, fmt.Errorf("validate CLI definition: %w", err)
	}
	if !definitionID.MatchString(definition.ID) || definition.VersionNumber <= 0 {
		return BuildResult{}, errors.New("unsafe CLI definition ID")
	}
	if len(builder.RuntimeDigests) == 0 {
		return BuildResult{}, errors.New("at least one pinned Runtime digest is required")
	}
	for index, digest := range builder.RuntimeDigests {
		if !runtimeDigest.MatchString(digest) || slices.Contains(builder.RuntimeDigests[:index], digest) {
			return BuildResult{}, errors.New("Runtime digests must be unique pinned sha256 values")
		}
	}

	var artifact PackageArtifact
	var err error
	if definition.InstallationType == "upload" {
		if builder.Uploads == nil || builder.Sources == nil {
			return BuildResult{}, errors.New("CLI ZIP builder ports are required")
		}
		var source []byte
		source, err = builder.Sources.GetSourceVerified(ctx, definition.SourceObjectKey, definition.SourceSHA256)
		if err == nil {
			artifact, err = builder.Uploads.Build(ctx, source)
		}
	} else {
		architectures := definition.SupportedArchitectures
		if len(architectures) == 0 {
			architectures = []string{"linux-amd64"}
		}
		artifact, err = builder.Packages.Build(ctx, definition.Package, definition.Version, slices.Clone(architectures))
	}
	if err != nil {
		return BuildResult{}, fmt.Errorf("build exact CLI package: %w", err)
	}
	resolved, err := resolvePackageDefinition(definition, artifact)
	if err != nil {
		return BuildResult{}, err
	}
	for _, digest := range builder.RuntimeDigests {
		if err := builder.Conformance.Test(ctx, artifact.BundleBytes, digest, resolved); err != nil {
			return BuildResult{}, fmt.Errorf("CLI conformance on %s: %w", digest, err)
		}
	}

	bundleSum := sha256.Sum256(artifact.BundleBytes)
	bundleDigest := hex.EncodeToString(bundleSum[:])
	objectKey := path.Join("cli-connectors", definition.ID, fmt.Sprintf("v%d", definition.VersionNumber), bundleDigest+".tgz")
	if err := builder.Store.PutImmutable(ctx, objectKey, artifact.BundleBytes, bundleDigest); err != nil {
		return BuildResult{}, fmt.Errorf("store immutable CLI bundle: %w", err)
	}
	return BuildResult{State: StateAvailable, BundleObjectKey: objectKey, BundleSHA256: bundleDigest, RuntimeDigests: slices.Clone(builder.RuntimeDigests), Package: resolved.Package, Version: resolved.Version, Integrity: resolved.Integrity, Executable: resolved.Executable, AuthenticationDriver: resolved.AuthenticationDriver, Capabilities: slices.Clone(resolved.Capabilities), SupportedArchitectures: slices.Clone(resolved.SupportedArchitectures)}, nil
}

func resolvePackageDefinition(definition Definition, artifact PackageArtifact) (Definition, error) {
	if len(artifact.PackageBytes) == 0 || len(artifact.BundleBytes) == 0 {
		return Definition{}, errors.New("CLI package builder returned an empty artifact")
	}
	if definition.Integrity != "" && artifact.Integrity != definition.Integrity {
		return Definition{}, errors.New("CLI package integrity differs from the reviewed definition")
	}
	encoded, ok := strings.CutPrefix(artifact.Integrity, "sha512-")
	if !ok {
		return Definition{}, errors.New("CLI package integrity must use sha512 SRI")
	}
	expected, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(expected) != sha512.Size {
		return Definition{}, errors.New("invalid CLI package sha512 integrity")
	}
	actual := sha512.Sum512(artifact.PackageBytes)
	if subtle.ConstantTimeCompare(expected, actual[:]) != 1 {
		return Definition{}, errors.New("CLI package integrity verification failed")
	}
	metadata := PackageDefinitionMetadata{Name: definition.Package, Version: definition.Version}
	if len(artifact.Manifest) > 0 {
		metadata, err = packageDefinitionMetadata(artifact.Manifest)
		if err != nil {
			return Definition{}, err
		}
	} else if definition.InstallationType == "upload" {
		return Definition{}, errors.New("CLI ZIP package has no readable package.json metadata")
	}
	if definition.InstallationType == "upload" {
		definition.Package, definition.Version = metadata.Name, metadata.Version
	} else if metadata.Name != "" && (metadata.Name != definition.Package || metadata.Version != definition.Version) {
		return Definition{}, errors.New("CLI package manifest differs from the requested exact package")
	}
	definition.Integrity = artifact.Integrity
	if definition.Executable == "" {
		definition.Executable = metadata.Executable
		if definition.Executable == "" && len(artifact.Bins) == 1 {
			for name := range artifact.Bins {
				definition.Executable = name
			}
		}
	}
	if definition.AuthenticationDriver == "" {
		definition.AuthenticationDriver = metadata.AuthenticationDriver
	}
	if definition.AuthenticationDriver == "" {
		definition.AuthenticationDriver = "none"
	}
	if len(definition.Capabilities) == 0 {
		definition.Capabilities = metadata.Capabilities
	}
	if len(definition.SupportedArchitectures) == 0 {
		definition.SupportedArchitectures = metadata.SupportedArchitectures
	}
	if len(definition.SupportedArchitectures) == 0 {
		definition.SupportedArchitectures = []string{"linux-amd64"}
	}
	if err := definition.Validate(); err != nil {
		return Definition{}, fmt.Errorf("validate CLI package metadata: %w", err)
	}
	binPath, ok := artifact.Bins[definition.Executable]
	if !ok || binPath == "" || strings.ContainsAny(binPath, "\\\x00\r\n") || path.IsAbs(binPath) || path.Clean(binPath) != binPath || strings.HasPrefix(binPath, "../") {
		return Definition{}, errors.New("CLI executable is not a safe package bin entry")
	}
	return definition, nil
}
