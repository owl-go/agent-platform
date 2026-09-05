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
	wrapper := Wrapper{Process: process, ConsumeApproval: func(_ context.Context, digest, nonce string) error { return errors.New("approval does not match") }, Now: func() time.Time { return time.Unix(100, 0) }}
	definition := Definition{State: StateAvailable, Executable: "lark", BundleSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RuntimeDigests: []string{"sha256:runtime"}, Capabilities: []Capability{{ID: "send", ArgvPrefix: []string{"message", "send"}, Risk: RiskHigh, Identities: []Identity{IdentityUser}, Timeout: time.Minute}}}
	_, err := wrapper.Execute(context.Background(), definition, Request{CapabilityID: "send", RuntimeDigest: "sha256:runtime", BundleSHA256: definition.BundleSHA256, Identity: IdentityUser, Argv: []string{"message", "send", "--target", "changed"}, Target: "chat-1", ApprovalNonce: "once", ApprovalExpiresAt: time.Unix(200, 0)})
	if err == nil || process.starts != 0 {
		t.Fatalf("starts=%d err=%v", process.starts, err)
	}
}

func TestHighRiskCommandDigestBindsTargetAndExpiry(t *testing.T) {
	definition := Definition{ID: "connector-1", VersionNumber: 3, Executable: "lark-cli"}
	request := Request{CapabilityID: "send", Identity: IdentityUser, Argv: []string{"message", "send"}, Target: "chat-1", BundleSHA256: "bundle", RuntimeDigest: "runtime", ApprovalExpiresAt: time.Unix(200, 0)}
	first := CommandDigest(definition, request)
	request.Target = "chat-2"
	if first == CommandDigest(definition, request) {
		t.Fatal("target was not bound to command digest")
	}
	request.Target = "chat-1"
	request.ApprovalExpiresAt = time.Unix(201, 0)
	if first == CommandDigest(definition, request) {
		t.Fatal("approval expiry was not bound to command digest")
	}
}

func TestWrapperRejectsExpiredApprovalBeforeConsumption(t *testing.T) {
	process := &recordingProcess{}
	consumed := false
	wrapper := Wrapper{Process: process, ConsumeApproval: func(context.Context, string, string) error { consumed = true; return nil }, Now: func() time.Time { return time.Unix(200, 0) }}
	definition := Definition{State: StateAvailable, Executable: "lark", BundleSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", RuntimeDigests: []string{"sha256:runtime"}, Capabilities: []Capability{{ID: "send", ArgvPrefix: []string{"message", "send"}, Risk: RiskHigh, Identities: []Identity{IdentityUser}, Timeout: time.Minute}}}
	_, err := wrapper.Execute(context.Background(), definition, Request{CapabilityID: "send", RuntimeDigest: "sha256:runtime", BundleSHA256: definition.BundleSHA256, Identity: IdentityUser, Argv: []string{"message", "send"}, Target: "chat-1", ApprovalNonce: "once", ApprovalExpiresAt: time.Unix(199, 0)})
	if err == nil || consumed || process.starts != 0 {
		t.Fatalf("consumed=%v starts=%d err=%v", consumed, process.starts, err)
	}
}

func TestDefinitionRejectsIncompleteOrUnsafeCapabilityPolicy(t *testing.T) {
	base := Definition{Name: "Feishu", Package: "@larksuite/cli", Version: "1.0.93", Integrity: "sha512-test", Executable: "lark-cli", AuthenticationDriver: "feishu", SupportedArchitectures: []string{"linux-amd64"}, Capabilities: []Capability{{ID: "identity", ArgvPrefix: []string{"auth", "status"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"open.feishu.cn"}, Timeout: time.Minute}}}
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
