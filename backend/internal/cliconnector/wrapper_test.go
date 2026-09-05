package cliconnector

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingProcess struct {
	starts     int
	executable string
	args       []string
}

func (process *recordingProcess) Run(_ context.Context, executable string, args []string, _ map[string]string) (Result, error) {
	process.starts++
	process.executable = executable
	process.args = append([]string(nil), args...)
	return Result{Stdout: []byte("ok")}, nil
}

func TestWrapperStartsOnlyAnExactLowRiskCapability(t *testing.T) {
	process := &recordingProcess{}
	wrapper := Wrapper{Process: process, Now: func() time.Time { return time.Unix(100, 0) }}
	definition := Definition{State: StateAvailable, Executable: "lark", BundleSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RuntimeDigests: []string{"sha256:runtime"}, Capabilities: []Capability{{ID: "whoami", ArgvPrefix: []string{"user", "whoami"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, Timeout: time.Minute}}}
	result, err := wrapper.Execute(context.Background(), definition, Request{CapabilityID: "whoami", RuntimeDigest: "sha256:runtime", BundleSHA256: definition.BundleSHA256, Identity: IdentityUser, Argv: []string{"user", "whoami"}})
	if err != nil || string(result.Stdout) != "ok" || process.starts != 1 {
		t.Fatalf("result=%q starts=%d err=%v", result.Stdout, process.starts, err)
	}
	if process.executable != "lark" {
		t.Fatalf("executable = %q", process.executable)
	}
}

func TestWrapperRejectsChangedHighRiskCommandBeforeProcessStart(t *testing.T) {
	process := &recordingProcess{}
	wrapper := Wrapper{Process: process, ConsumeApproval: func(_ context.Context, digest, nonce string) error { return errors.New("approval does not match") }}
	definition := Definition{State: StateAvailable, Executable: "lark", BundleSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RuntimeDigests: []string{"sha256:runtime"}, Capabilities: []Capability{{ID: "send", ArgvPrefix: []string{"message", "send"}, Risk: RiskHigh, Identities: []Identity{IdentityUser}, Timeout: time.Minute}}}
	_, err := wrapper.Execute(context.Background(), definition, Request{CapabilityID: "send", RuntimeDigest: "sha256:runtime", BundleSHA256: definition.BundleSHA256, Identity: IdentityUser, Argv: []string{"message", "send", "--target", "changed"}, ApprovalNonce: "once"})
	if err == nil || process.starts != 0 {
		t.Fatalf("starts=%d err=%v", process.starts, err)
	}
}

func TestDefinitionRejectsIncompleteOrUnsafeCapabilityPolicy(t *testing.T) {
	base := Definition{Name: "Feishu", Package: "@larksuite/cli", Version: "1.0.93", Integrity: "sha512-test", Executable: "lark-cli", AuthenticationDriver: "feishu", Capabilities: []Capability{{ID: "identity", ArgvPrefix: []string{"auth", "status"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"open.feishu.cn"}, Timeout: time.Minute}}}
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{name: "unknown risk", mutate: func(value *Definition) { value.Capabilities[0].Risk = "medium" }},
		{name: "no identity", mutate: func(value *Definition) { value.Capabilities[0].Identities = nil }},
		{name: "unknown identity", mutate: func(value *Definition) { value.Capabilities[0].Identities = []Identity{"administrator"} }},
		{name: "wildcard egress", mutate: func(value *Definition) { value.Capabilities[0].EgressHosts = []string{"*.feishu.cn"} }},
		{name: "unsafe scope", mutate: func(value *Definition) { value.Capabilities[0].Scopes = []string{"contact read\nwrite"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := base
			value.Capabilities = append([]Capability(nil), base.Capabilities...)
			test.mutate(&value)
			if err := value.Validate(); err == nil {
				t.Fatal("expected invalid capability policy")
			}
		})
	}
}
