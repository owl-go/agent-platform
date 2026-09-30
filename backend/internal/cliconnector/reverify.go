package cliconnector

import (
	"context"
	"fmt"
)

// ReverificationTarget is an installed CLI bundle missing Conformance for a
// candidate Runtime RepoDigest. DefinitionID identifies the existing evidence
// owner; the bundle and Runtime identities are checked again before recording.
type ReverificationTarget struct {
	DefinitionID    string
	BundleObjectKey string
	BundleSHA256    string
	Executable      string
	CPUMillis       int `gorm:"column:cpu_millis"`
	MemoryMiB       int `gorm:"column:memory_mib"`
	ChildProcesses  int `gorm:"column:child_processes"`
}

type ReverificationRepository interface {
	ListCLIReverificationTargets(context.Context, string) ([]ReverificationTarget, error)
	RecordCLIReverification(context.Context, string, string, string) error
}

type VerifiedBundleReader interface {
	GetVerified(context.Context, string, string) ([]byte, error)
}

// ReverifyCLIBundles tests exact immutable bundle bytes in the candidate
// Runtime before writing any evidence. A failed test never grants use of that
// bundle on the new image.
func ReverifyCLIBundles(ctx context.Context, repository ReverificationRepository, bundles VerifiedBundleReader, suite Conformance, digest string) (int, error) {
	if repository == nil || bundles == nil || suite == nil || !runtimeDigest.MatchString(digest) {
		return 0, fmt.Errorf("invalid CLI reverification dependencies or Runtime Digest")
	}
	targets, err := repository.ListCLIReverificationTargets(ctx, digest)
	if err != nil {
		return 0, fmt.Errorf("list CLI reverification targets: %w", err)
	}
	verified := 0
	for _, target := range targets {
		if target.DefinitionID == "" || target.BundleObjectKey == "" || !bundleDigest.MatchString(target.BundleSHA256) || target.Executable == "" {
			return verified, fmt.Errorf("installed CLI Connector has incomplete Conformance identity")
		}
		bundle, err := bundles.GetVerified(ctx, target.BundleObjectKey, target.BundleSHA256)
		if err != nil {
			return verified, fmt.Errorf("read immutable CLI bundle: %w", err)
		}
		if err := suite.Test(ctx, bundle, digest, Definition{Executable: target.Executable, CPUMillis: target.CPUMillis, MemoryMiB: target.MemoryMiB, ChildProcesses: target.ChildProcesses}); err != nil {
			return verified, fmt.Errorf("CLI bundle Conformance failed: %w", err)
		}
		if err := repository.RecordCLIReverification(ctx, target.DefinitionID, target.BundleSHA256, digest); err != nil {
			return verified, fmt.Errorf("record CLI Conformance evidence: %w", err)
		}
		verified++
	}
	return verified, nil
}
