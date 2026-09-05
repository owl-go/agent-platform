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
}

type PackageBuilder interface {
	Build(context.Context, string, string, []string) (PackageArtifact, error)
}

type BundleStore interface {
	PutImmutable(context.Context, string, []byte, string) error
}

type Conformance interface {
	Test(context.Context, []byte, string, Definition) error
}

type Builder struct {
	Packages       PackageBuilder
	Store          BundleStore
	Conformance    Conformance
	RuntimeDigests []string
}

type BuildResult struct {
	State           State
	BundleObjectKey string
	BundleSHA256    string
	RuntimeDigests  []string
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
	if err := definition.Validate(); err != nil {
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

	artifact, err := builder.Packages.Build(ctx, definition.Package, definition.Version, slices.Clone(definition.SupportedArchitectures))
	if err != nil {
		return BuildResult{}, fmt.Errorf("build exact CLI package: %w", err)
	}
	if err := verifyPackageArtifact(definition, artifact); err != nil {
		return BuildResult{}, err
	}
	for _, digest := range builder.RuntimeDigests {
		if err := builder.Conformance.Test(ctx, artifact.BundleBytes, digest, definition); err != nil {
			return BuildResult{}, fmt.Errorf("CLI conformance on %s: %w", digest, err)
		}
	}

	bundleSum := sha256.Sum256(artifact.BundleBytes)
	bundleDigest := hex.EncodeToString(bundleSum[:])
	objectKey := path.Join("cli-connectors", definition.ID, fmt.Sprintf("v%d", definition.VersionNumber), bundleDigest+".tgz")
	if err := builder.Store.PutImmutable(ctx, objectKey, artifact.BundleBytes, bundleDigest); err != nil {
		return BuildResult{}, fmt.Errorf("store immutable CLI bundle: %w", err)
	}
	return BuildResult{State: StateAvailable, BundleObjectKey: objectKey, BundleSHA256: bundleDigest, RuntimeDigests: slices.Clone(builder.RuntimeDigests)}, nil
}

func verifyPackageArtifact(definition Definition, artifact PackageArtifact) error {
	if len(artifact.PackageBytes) == 0 || len(artifact.BundleBytes) == 0 {
		return errors.New("CLI package builder returned an empty artifact")
	}
	if artifact.Integrity != definition.Integrity {
		return errors.New("CLI package integrity differs from the reviewed definition")
	}
	encoded, ok := strings.CutPrefix(definition.Integrity, "sha512-")
	if !ok {
		return errors.New("CLI package integrity must use sha512 SRI")
	}
	expected, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(expected) != sha512.Size {
		return errors.New("invalid CLI package sha512 integrity")
	}
	actual := sha512.Sum512(artifact.PackageBytes)
	if subtle.ConstantTimeCompare(expected, actual[:]) != 1 {
		return errors.New("CLI package integrity verification failed")
	}
	binPath, ok := artifact.Bins[definition.Executable]
	if !ok || binPath == "" || strings.ContainsAny(binPath, "\\\x00\r\n") || path.IsAbs(binPath) || path.Clean(binPath) != binPath || strings.HasPrefix(binPath, "../") {
		return errors.New("CLI executable is not a safe package bin entry")
	}
	return nil
}
