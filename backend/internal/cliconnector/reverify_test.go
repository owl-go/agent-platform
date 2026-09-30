package cliconnector

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recordingReverificationRepository struct {
	targets  []ReverificationTarget
	recorded []string
}

func (repository *recordingReverificationRepository) ListCLIReverificationTargets(context.Context, string) ([]ReverificationTarget, error) {
	return repository.targets, nil
}

func (repository *recordingReverificationRepository) RecordCLIReverification(_ context.Context, definitionID, bundleSHA256, runtimeDigest string) error {
	repository.recorded = append(repository.recorded, definitionID+":"+bundleSHA256+":"+runtimeDigest)
	return nil
}

type fixedBundleReader struct{ bundle []byte }

func (reader fixedBundleReader) GetVerified(context.Context, string, string) ([]byte, error) {
	return reader.bundle, nil
}

type recordingReverificationSuite struct {
	failedExecutable string
	called           []string
	definitions      []Definition
}

func (suite *recordingReverificationSuite) Test(_ context.Context, _ []byte, _ string, definition Definition) error {
	suite.called = append(suite.called, definition.Executable)
	suite.definitions = append(suite.definitions, definition)
	if definition.Executable == suite.failedExecutable {
		return errors.New("isolated CLI check failed")
	}
	return nil
}

func TestReverifyCLIBundlesRecordsOnlyPassedExactRuntime(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	repository := &recordingReverificationRepository{targets: []ReverificationTarget{
		{DefinitionID: "definition-1", BundleObjectKey: "cli-connectors/one/bundle.tgz", BundleSHA256: strings.Repeat("b", 64), Executable: "feishu", CPUMillis: 500, MemoryMiB: 256, ChildProcesses: 16},
		{DefinitionID: "definition-2", BundleObjectKey: "cli-connectors/two/bundle.tgz", BundleSHA256: strings.Repeat("c", 64), Executable: "dingtalk"},
	}}
	suite := &recordingReverificationSuite{failedExecutable: "dingtalk"}
	count, err := ReverifyCLIBundles(context.Background(), repository, fixedBundleReader{bundle: []byte("verified")}, suite, digest)
	if err == nil || !strings.Contains(err.Error(), "isolated CLI check failed") {
		t.Fatalf("expected failed conformance, got count=%d error=%v", count, err)
	}
	if len(suite.definitions) != 2 || suite.definitions[0].CPUMillis != 500 || suite.definitions[0].MemoryMiB != 256 || suite.definitions[0].ChildProcesses != 16 {
		t.Fatalf("resource limits lost: %#v", suite.definitions)
	}
	if count != 1 || len(repository.recorded) != 1 || repository.recorded[0] != "definition-1:"+strings.Repeat("b", 64)+":"+digest {
		t.Fatalf("recorded unverified evidence: count=%d records=%v", count, repository.recorded)
	}
}
