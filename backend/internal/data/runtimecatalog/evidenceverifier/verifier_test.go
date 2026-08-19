package evidenceverifier

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/runtimecatalog/application"
	"agent-platform/backend/internal/biz/runtimecatalog/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestVerifierAcceptsMatchingProductionConformanceArchive(t *testing.T) {
	provider := memory.New()
	image := evidenceImage()
	archive := evidenceArchive(t, image, true, true)
	digest := sha256.Sum256(archive)
	key := "phase-0/codex/evidence.tar"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(archive), objectstore.PutOptions{
		Size: int64(len(archive)), SHA256: hex.EncodeToString(digest[:]), ContentType: "application/x-tar",
		Metadata: map[string]string{"artifact-kind": "production-conformance"},
	}); err != nil {
		t.Fatal(err)
	}

	verified, err := New(provider).Verify(context.Background(), key, image)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Key != key || verified.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("verified evidence = %+v", verified)
	}
}

func TestVerifierRejectsMissingOrMismatchedEvidence(t *testing.T) {
	provider := memory.New()
	image := evidenceImage()
	if _, err := New(provider).Verify(context.Background(), "missing/evidence.tar", image); !errors.Is(err, application.ErrInvalidEvidence) {
		t.Fatalf("missing evidence error = %v", err)
	}

	archive := evidenceArchive(t, image, true, true)
	digest := sha256.Sum256(archive)
	if _, err := provider.Put(context.Background(), "phase-0/codex/evidence.tar", bytes.NewReader(archive), objectstore.PutOptions{
		Size: int64(len(archive)), SHA256: hex.EncodeToString(digest[:]), ContentType: "application/x-tar",
		Metadata: map[string]string{"artifact-kind": "production-conformance"},
	}); err != nil {
		t.Fatal(err)
	}
	image.ImageDigest = "registry.example/codex@sha256:" + strings.Repeat("c", 64)
	if _, err := New(provider).Verify(context.Background(), "phase-0/codex/evidence.tar", image); !errors.Is(err, application.ErrInvalidEvidence) {
		t.Fatalf("mismatched evidence error = %v", err)
	}
}

func TestVerifierRejectsSuccessfulForcedKillReport(t *testing.T) {
	provider := memory.New()
	image := evidenceImage()
	archive := evidenceArchive(t, image, false, true)
	digest := sha256.Sum256(archive)
	key := "phase-0/codex/evidence.tar"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(archive), objectstore.PutOptions{
		Size: int64(len(archive)), SHA256: hex.EncodeToString(digest[:]), ContentType: "application/x-tar",
		Metadata: map[string]string{"artifact-kind": "production-conformance"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(provider).Verify(context.Background(), key, image); !errors.Is(err, application.ErrInvalidEvidence) {
		t.Fatalf("successful forced-kill evidence error = %v", err)
	}
}

func TestVerifierRejectsRecoveryWithoutExplicitExitCode(t *testing.T) {
	provider := memory.New()
	image := evidenceImage()
	archive := evidenceArchive(t, image, true, false)
	digest := sha256.Sum256(archive)
	key := "phase-0/codex/evidence.tar"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(archive), objectstore.PutOptions{
		Size: int64(len(archive)), SHA256: hex.EncodeToString(digest[:]), ContentType: "application/x-tar",
		Metadata: map[string]string{"artifact-kind": "production-conformance"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := New(provider).Verify(context.Background(), key, image); !errors.Is(err, application.ErrInvalidEvidence) {
		t.Fatalf("missing recovery result error = %v", err)
	}
}

func evidenceImage() domain.RuntimeImage {
	return domain.RuntimeImage{
		Runtime: domain.Codex, CLIVersion: "1.2.3", ImageDigest: "registry.example/codex@sha256:" + strings.Repeat("a", 64),
		Capabilities: map[string]bool{"streaming": true, "subagents": false},
	}
}

func evidenceArchive(t *testing.T, image domain.RuntimeImage, forcedKillFailed, recoverySucceeded bool) []byte {
	t.Helper()
	report := map[string]any{
		"runtime": map[string]any{"name": string(image.Runtime), "version": image.CLIVersion, "capabilities": image.Capabilities},
		"image":   image.ImageDigest,
	}
	forcedKill := report
	if forcedKillFailed {
		forcedKill = withFailure(report, "execution_failed", "container terminated")
	}
	recovery := report
	if recoverySucceeded {
		recovery = withResult(report, 0)
	}
	scenarios := map[string]any{
		"forced_kill": forcedKill,
		"recovery":    recovery,
		"interrupt":   withError(report, "interrupted"),
		"cancel":      withError(report, "interrupted"),
		"timeout":     withError(report, "timed_out"),
	}
	snapshotSHA := strings.Repeat("d", 64)
	summary, err := json.Marshal(map[string]any{
		"runtime": string(image.Runtime), "image": image.ImageDigest, "review_branch": "review/codex",
		"scenarios": scenarios, "snapshots": map[string]any{
			"minio":      map[string]any{"action": "restored", "provider": "minio", "key": "phase-0/workspace.tar", "size": 1024, "sha256": snapshotSHA},
			"aliyun_oss": map[string]any{"action": "restored", "provider": "aliyun_oss", "key": "phase-0/workspace.tar", "size": 1024, "sha256": snapshotSHA},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	if err := writer.WriteHeader(&tar.Header{Name: "scenario-summary.json", Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(summary)), ModTime: time.Unix(0, 0)}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(summary); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func withError(source map[string]any, code string) map[string]any {
	cloned := make(map[string]any, len(source)+1)
	for key, value := range source {
		cloned[key] = value
	}
	cloned["error_code"] = code
	return cloned
}

func withFailure(source map[string]any, code, message string) map[string]any {
	cloned := withError(source, code)
	cloned["error"] = message
	return cloned
}

func withResult(source map[string]any, exitCode int) map[string]any {
	cloned := make(map[string]any, len(source)+1)
	for key, value := range source {
		cloned[key] = value
	}
	cloned["result"] = map[string]int{"exit_code": exitCode}
	return cloned
}
