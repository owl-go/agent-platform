package runtimeprocessor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/sandbox"
)

func ParsePlan(lease domain.Lease) (Plan, []CredentialBinding, error) {
	if lease.RunID == "" || lease.Token == "" || lease.RuntimeName == "" || lease.RuntimeCLIVersion == "" || lease.ImageDigest == "" || lease.WorkspaceVolume == "" {
		return Plan{}, nil, fmt.Errorf("claimed Run has an incomplete frozen Runtime binding")
	}
	var model struct {
		ModelID string `json:"model_id"`
	}
	if err := decodeStrict(lease.ModelBinding, &model); err != nil {
		return Plan{}, nil, fmt.Errorf("decode frozen Model Binding: %w", err)
	}
	if model.ModelID == "" {
		return Plan{}, nil, fmt.Errorf("frozen Model Binding has no model ID")
	}
	budget, err := parseModelBudget(lease.ModelBudget)
	if err != nil {
		return Plan{}, nil, fmt.Errorf("decode frozen Model Budget: %w", err)
	}
	var limits struct {
		TimeoutSeconds int64   `json:"timeout_seconds"`
		CPUs           float64 `json:"cpus"`
		MemoryBytes    int64   `json:"memory_bytes"`
		PIDs           int64   `json:"pids"`
		TempBytes      int64   `json:"temp_bytes"`
		Egress         string  `json:"egress"`
	}
	if err := decodeStrict(lease.ExecutionLimits, &limits); err != nil {
		return Plan{}, nil, fmt.Errorf("decode frozen Execution Limits: %w", err)
	}
	if limits.TimeoutSeconds <= 0 || limits.TimeoutSeconds > int64((24*time.Hour)/time.Second) || limits.CPUs <= 0 || limits.MemoryBytes <= 0 || limits.PIDs <= 0 || limits.TempBytes <= 0 {
		return Plan{}, nil, fmt.Errorf("frozen Execution Limits must contain positive bounded values")
	}
	egress := sandbox.EgressMode(limits.Egress)
	if egress != sandbox.EgressNone && egress != sandbox.EgressPublic {
		return Plan{}, nil, fmt.Errorf("frozen Execution Limits contain an invalid Egress Policy")
	}
	var bindings []CredentialBinding
	if err := decodeStrict(lease.CredentialBindings, &bindings); err != nil {
		return Plan{}, nil, fmt.Errorf("decode frozen Credential Bindings: %w", err)
	}
	if len(bindings) == 0 {
		return Plan{}, nil, fmt.Errorf("frozen Credential Bindings are empty")
	}
	for _, binding := range bindings {
		if binding.Ref == "" || binding.Purpose == "" {
			return Plan{}, nil, fmt.Errorf("frozen Credential Binding is incomplete")
		}
	}
	var rawCapabilities map[string]bool
	if err := decodeStrict(lease.Capabilities, &rawCapabilities); err != nil {
		return Plan{}, nil, fmt.Errorf("decode frozen Runtime Capabilities: %w", err)
	}
	capabilities := make(map[agentruntime.Capability]bool, len(rawCapabilities))
	for name, enabled := range rawCapabilities {
		capabilities[agentruntime.Capability(name)] = enabled
	}
	return Plan{
		ModelID: model.ModelID, Budget: budget, Timeout: time.Duration(limits.TimeoutSeconds) * time.Second,
		Egress: egress, Limits: sandbox.Limits{CPUs: limits.CPUs, MemoryBytes: limits.MemoryBytes, PIDs: limits.PIDs, TempBytes: limits.TempBytes},
		Capabilities: capabilities,
	}, bindings, nil
}

func decodeStrict(input json.RawMessage, target any) error {
	if len(input) == 0 {
		return fmt.Errorf("JSON value is missing")
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("multiple JSON values are not allowed")
	}
	return nil
}
