package evidenceverifier

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"agent-platform/backend/internal/biz/runtimecatalog/application"
	"agent-platform/backend/internal/biz/runtimecatalog/domain"
	"agent-platform/backend/internal/objectstore"
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

const (
	maxArtifactSize = 64 << 20
	maxSummarySize  = 1 << 20
)

type Verifier struct {
	objects objectstore.Provider
}

func New(objects objectstore.Provider) *Verifier { return &Verifier{objects: objects} }

func (verifier *Verifier) Verify(ctx context.Context, key string, image domain.RuntimeImage) (application.VerifiedEvidence, error) {
	if verifier == nil || verifier.objects == nil {
		return application.VerifiedEvidence{}, fmt.Errorf("%w: Object Storage Provider is required", application.ErrEvidenceUnavailable)
	}
	reader, metadata, err := verifier.objects.Get(ctx, key)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotFound) || errors.Is(err, objectstore.ErrInvalidKey) {
			return application.VerifiedEvidence{}, fmt.Errorf("%w: evidence object does not exist", application.ErrInvalidEvidence)
		}
		return application.VerifiedEvidence{}, fmt.Errorf("%w: %w", application.ErrEvidenceUnavailable, err)
	}
	defer reader.Close()
	if metadata.Size <= 0 || metadata.Size > maxArtifactSize || !sha256Pattern.MatchString(metadata.SHA256) || metadata.ContentType != "application/x-tar" || metadata.Metadata["artifact-kind"] != "production-conformance" {
		return application.VerifiedEvidence{}, fmt.Errorf("%w: object metadata does not describe a Production Conformance archive", application.ErrInvalidEvidence)
	}

	digest := sha256.New()
	counted := &countingReader{reader: io.TeeReader(io.LimitReader(reader, maxArtifactSize+1), digest)}
	summary, err := readScenarioSummary(counted)
	if err != nil {
		return application.VerifiedEvidence{}, err
	}
	if _, err := io.Copy(io.Discard, counted); err != nil {
		return application.VerifiedEvidence{}, fmt.Errorf("%w: read evidence archive: %w", application.ErrEvidenceUnavailable, err)
	}
	actualSHA := hex.EncodeToString(digest.Sum(nil))
	if counted.count != metadata.Size || actualSHA != metadata.SHA256 {
		return application.VerifiedEvidence{}, fmt.Errorf("%w: object size or SHA-256 mismatch", application.ErrInvalidEvidence)
	}
	if err := validateSummary(summary, image); err != nil {
		return application.VerifiedEvidence{}, err
	}
	return application.VerifiedEvidence{Key: key, SHA256: actualSHA}, nil
}

type scenarioSummary struct {
	Runtime      string                    `json:"runtime"`
	Image        string                    `json:"image"`
	ReviewBranch string                    `json:"review_branch"`
	Scenarios    map[string]scenarioReport `json:"scenarios"`
	Snapshots    map[string]snapshotReport `json:"snapshots"`
}

type scenarioReport struct {
	Runtime struct {
		Name         string          `json:"name"`
		Version      string          `json:"version"`
		Capabilities map[string]bool `json:"capabilities"`
	} `json:"runtime"`
	Image     string `json:"image"`
	ErrorCode string `json:"error_code"`
	Error     string `json:"error"`
	Result    *struct {
		ExitCode *int `json:"exit_code"`
	} `json:"result"`
}

type snapshotReport struct {
	Action   string `json:"action"`
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

func readScenarioSummary(reader io.Reader) (scenarioSummary, error) {
	tarReader := tar.NewReader(reader)
	seen := false
	var summary scenarioSummary
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return scenarioSummary{}, fmt.Errorf("%w: malformed tar archive", application.ErrInvalidEvidence)
		}
		name := strings.TrimPrefix(header.Name, "./")
		if name != "scenario-summary.json" {
			continue
		}
		if seen || header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || header.Size <= 0 || header.Size > maxSummarySize {
			return scenarioSummary{}, fmt.Errorf("%w: invalid scenario-summary.json entry", application.ErrInvalidEvidence)
		}
		seen = true
		decoder := json.NewDecoder(io.LimitReader(tarReader, maxSummarySize))
		if err := decoder.Decode(&summary); err != nil {
			return scenarioSummary{}, fmt.Errorf("%w: malformed scenario summary", application.ErrInvalidEvidence)
		}
	}
	if !seen {
		return scenarioSummary{}, fmt.Errorf("%w: scenario-summary.json is missing", application.ErrInvalidEvidence)
	}
	return summary, nil
}

func validateSummary(summary scenarioSummary, image domain.RuntimeImage) error {
	if summary.Runtime != string(image.Runtime) || summary.Image != image.ImageDigest || strings.TrimSpace(summary.ReviewBranch) == "" {
		return fmt.Errorf("%w: Runtime, image Digest, or Review Branch does not match", application.ErrInvalidEvidence)
	}
	expectedErrors := map[string]string{
		"forced_kill": "", "recovery": "", "interrupt": "interrupted", "cancel": "interrupted", "timeout": "timed_out",
	}
	for name, expectedError := range expectedErrors {
		report, ok := summary.Scenarios[name]
		if !ok || report.Image != image.ImageDigest || report.Runtime.Name != string(image.Runtime) || report.Runtime.Version != image.CLIVersion || !equalCapabilities(report.Runtime.Capabilities, image.Capabilities) {
			return fmt.Errorf("%w: scenario %q does not certify the registered Runtime Image", application.ErrInvalidEvidence, name)
		}
		if name == "recovery" && report.ErrorCode != "" || expectedError != "" && report.ErrorCode != expectedError {
			return fmt.Errorf("%w: scenario %q has an unexpected result", application.ErrInvalidEvidence, name)
		}
		if name == "forced_kill" && (strings.TrimSpace(report.Error) == "" || report.ErrorCode == "") {
			return fmt.Errorf("%w: forced-kill scenario does not record the expected failure", application.ErrInvalidEvidence)
		}
		if name == "recovery" && (report.Result == nil || report.Result.ExitCode == nil || *report.Result.ExitCode != 0) {
			return fmt.Errorf("%w: recovery scenario did not complete successfully", application.ErrInvalidEvidence)
		}
	}
	minio, minioOK := summary.Snapshots["minio"]
	aliyun, aliyunOK := summary.Snapshots["aliyun_oss"]
	if !minioOK || !aliyunOK || !validSnapshot(minio, "minio") || !validSnapshot(aliyun, "aliyun_oss") || minio.Key != aliyun.Key || minio.Size != aliyun.Size || minio.SHA256 != aliyun.SHA256 {
		return fmt.Errorf("%w: MinIO and Aliyun OSS Snapshot evidence is incomplete", application.ErrInvalidEvidence)
	}
	return nil
}

func validSnapshot(snapshot snapshotReport, provider string) bool {
	return snapshot.Action == "restored" && snapshot.Provider == provider && strings.TrimSpace(snapshot.Key) != "" && snapshot.Size > 0 && sha256Pattern.MatchString(snapshot.SHA256)
}

func equalCapabilities(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for capability, enabled := range left {
		if right[capability] != enabled {
			return false
		}
	}
	return true
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.count += int64(count)
	return count, err
}
