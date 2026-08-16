package runtimeprocessor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/claude"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/codex"
	"agent-platform/backend/internal/agentruntime/containerprocess"
	"agent-platform/backend/internal/agentruntime/hermes"
	"agent-platform/backend/internal/agentruntime/openclaw"
	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/gitworkflow"
)

type ContainerFactoryConfig struct {
	AdapterVersion      string
	SandboxRuntime      string
	PublicEgressNetwork string
	ResolverConfigFile  string
	UID                 int
	GID                 int
	OutputSinkFactory   func(string, string) processharness.OutputSink
}

type ContainerFactory struct{ config ContainerFactoryConfig }

func NewContainerFactory(config ContainerFactoryConfig) (*ContainerFactory, error) {
	if config.AdapterVersion == "" || config.SandboxRuntime != "runsc" || config.UID <= 0 || config.GID <= 0 || config.OutputSinkFactory == nil {
		return nil, fmt.Errorf("Adapter version, runsc, non-root identity, and Output Sink are required")
	}
	return &ContainerFactory{config: config}, nil
}

func (factory *ContainerFactory) New(lease domain.Lease, plan Plan, environment *credentials.Environment) (agentruntime.Adapter, error) {
	if environment == nil {
		return nil, fmt.Errorf("credential environment is required")
	}
	if lease.AdapterVersion != factory.config.AdapterVersion {
		return nil, fmt.Errorf("frozen Adapter version does not match Worker build")
	}
	var qualityCommands []gitworkflow.QualityCommand
	if err := decodeStrict(lease.QualityCommands, &qualityCommands); err != nil {
		return nil, fmt.Errorf("decode frozen quality commands: %w", err)
	}
	workflowPlan, err := json.Marshal(gitworkflow.Plan{
		RunID: lease.RunID, RepositoryURL: lease.RepositorySSHURL,
		TargetBranch: lease.TargetBranch, ReviewBranch: lease.ReviewBranch,
		GitAuthorName: lease.GitAuthorName, GitAuthorEmail: lease.GitAuthorEmail,
		QualityCommands: qualityCommands,
	})
	if err != nil {
		return nil, fmt.Errorf("encode Git workflow plan: %w", err)
	}
	process, err := containerprocess.New(containerprocess.Config{
		Image: lease.ImageDigest, RuntimeCommand: lease.RuntimeName, RunID: lease.RunID,
		Runtime: factory.config.SandboxRuntime, WorkspaceVolume: lease.WorkspaceVolume,
		ContainerWorkspace: "/workspace", CredentialDirectory: environment.Directory(),
		PublicEgressNetwork: factory.config.PublicEgressNetwork, ResolverConfigFile: factory.config.ResolverConfigFile,
		Egress: plan.Egress, Limits: plan.Limits, UID: factory.config.UID, GID: factory.config.GID,
		WorkflowPlan: base64.RawURLEncoding.EncodeToString(workflowPlan),
	})
	if err != nil {
		return nil, err
	}
	outputSink := factory.config.OutputSinkFactory(lease.RunID, lease.AttemptID)
	if outputSink == nil {
		return nil, fmt.Errorf("Runtime Output Sink Factory returned nil")
	}
	config := cliadapter.Config{
		ExpectedVersion: lease.RuntimeCLIVersion, VerifiedCapabilities: plan.Capabilities,
		RunProcess: process, OutputSink: processharness.NewRedactingSink(environment.Redactor(), outputSink),
	}
	switch lease.RuntimeName {
	case "claude":
		return claude.New(config), nil
	case "codex":
		return codex.New(config), nil
	case "hermes":
		return hermes.New(config), nil
	case "openclaw":
		return openclaw.New(config), nil
	default:
		return nil, fmt.Errorf("unsupported Agent Runtime %q", lease.RuntimeName)
	}
}
