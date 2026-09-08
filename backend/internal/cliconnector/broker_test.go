package cliconnector

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBrokerExecutesOnlyServerOwnedLowRiskDefinition(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	definition.Executable = "mutated-after-construction"
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser})
	stdout, decodeErr := base64.StdEncoding.DecodeString(response.StdoutBase64)
	if decodeErr != nil || response.ErrorCode != "" || string(stdout) != "ok" || process.executable != "tool" {
		t.Fatalf("response=%#v executable=%q decode=%v", response, process.executable, decodeErr)
	}
}

func TestBrokerEmitsTypedOperationLifecycle(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.Capabilities[0].OperationPhrase = map[string]string{"zh-CN": "读取当前身份", "en": "Read current identity"}
	events := make([]OperationEvent, 0, 3)
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		GenerateOperationID: func() (string, error) { return "operation-1", nil },
		Events: func(_ context.Context, event OperationEvent) error {
			events = append(events, event)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "" || len(events) != 3 {
		t.Fatalf("response=%#v events=%#v", response, events)
	}
	if events[0].State != OperationRequested || events[1].State != OperationStarted || events[2].State != OperationSucceeded {
		t.Fatalf("event states=%#v", []OperationState{events[0].State, events[1].State, events[2].State})
	}
	for _, event := range events {
		if event.ContractVersion != 1 || event.OperationID != "operation-1" || event.ConnectorID != definition.ID || event.CapabilityID != "identity" || event.OperationPhrase["zh-CN"] != "读取当前身份" {
			t.Fatalf("event=%#v", event)
		}
	}
}

func TestBrokerAuditContainsOnlyStableMetadataAndInputDigest(t *testing.T) {
	definition := brokerDefinition(RiskLow)
	definition.ManifestVersion = "1"
	definition.Capabilities[0].Input = InputSchema{Fields: []InputField{{Name: "token", Type: InputString, Required: true, Flag: "--token", Sensitive: true}}}
	records := make([]AuditRecord, 0, 3)
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: &recordingProcess{}},
		ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "session", ExecutionID: "42", StageID: "session:42:stage:1"},
		Audit:           func(_ context.Context, record AuditRecord) error { records = append(records, record); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := "secret-audit-canary"
	response := broker.Handle(context.Background(), BrokerCommand{
		ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser, Target: "chat-1", Input: map[string]any{"token": secret},
	})
	if response.ErrorCode != "" || len(records) != 3 {
		t.Fatalf("response=%#v records=%#v", response, records)
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || records[0].UserID != "owner-1" || records[0].ManifestVersion != "1" || len(records[0].InputDigest) != 64 || records[2].Result != string(OperationSucceeded) {
		t.Fatalf("audit=%s", encoded)
	}
}

func TestBrokerStopsBeforeExecutionWhenAuditCannotBePersisted(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Audit: func(context.Context, AuditRecord) error { return errors.New("audit unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "event_unavailable" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
}

func TestBrokerStopsBeforeExecutionWhenOperationEventCannotBePublished(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Events: func(context.Context, OperationEvent) error { return errors.New("event store unavailable") },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "event_unavailable" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
}

func TestBrokerRendersStructuredInputFromFrozenCapability(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.Capabilities[0].Input = InputSchema{Fields: []InputField{
		{Name: "query", Type: InputString, Required: true, Flag: "--query"},
		{Name: "limit", Type: InputInteger, Flag: "--limit"},
	}}
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{
		ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser,
		Input: map[string]any{"query": "weekly plan", "limit": float64(5)},
	})
	if response.ErrorCode != "" || !slices.Equal(process.args, []string{"auth", "status", "--query", "weekly plan", "--limit", "5"}) {
		t.Fatalf("response=%#v arguments=%#v", response, process.args)
	}
}

func TestBrokerRejectsArgumentsForStructuredCapability(t *testing.T) {
	definition := brokerDefinition(RiskLow)
	definition.Capabilities[0].Input = InputSchema{Fields: []InputField{{Name: "query", Type: InputString, Required: true, Flag: "--query"}}}
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: &recordingProcess{}}})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{
		ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser,
		Arguments: []string{"auth", "status", "--query", "forged"}, Input: map[string]any{"query": "reviewed"},
	})
	if response.ErrorCode != "invalid_request" {
		t.Fatalf("response=%#v", response)
	}
}

func TestBrokerDescribesFrozenCapabilityWithoutExecution(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.ManifestVersion = "1"
	definition.UsageGuide = "Use identity to inspect the current account."
	definition.Capabilities[0].DisplayName = map[string]string{"en": "Inspect identity"}
	definition.Capabilities[0].OperationPhrase = map[string]string{"en": "Inspected identity"}
	definition.Capabilities[0].Input = InputSchema{Fields: []InputField{{Name: "verbose", Type: InputBoolean, Flag: "--verbose"}}}
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{Kind: BrokerCommandDescribe, ConnectorID: definition.ID})
	encoded, decodeErr := base64.StdEncoding.DecodeString(response.StdoutBase64)
	if decodeErr != nil || response.ErrorCode != "" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d decode=%v", response, process.starts, decodeErr)
	}
	var description ConnectorDescription
	if err := json.Unmarshal(encoded, &description); err != nil {
		t.Fatal(err)
	}
	if description.ManifestVersion != "1" || description.UsageGuide != definition.UsageGuide || len(description.Capabilities) != 1 || description.Capabilities[0].ID != "identity" {
		t.Fatalf("description=%#v", description)
	}
}

func TestBrokerRequiresUserActionBeforeHighRiskProcessStart(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskHigh)
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1"})
	if response.ErrorCode != "user_action_required" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
}

func TestBrokerWaitsForAndConsumesOneUseHighRiskApproval(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskHigh)
	definition.Capabilities[0].Input = InputSchema{Fields: []InputField{{Name: "value", Type: InputString, Required: true, Flag: "--value", Sensitive: true}}}
	now := time.Unix(100, 0).UTC()
	coordinator := &recordingApprovalCoordinator{}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Approval: coordinator, ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "run", ExecutionID: "run-1", StageID: "run-1:stage:1"},
		Now: func() time.Time { return now }, GenerateNonce: func() (string, error) { return "nonce-1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	command := BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1", Input: map[string]any{"value": "secret"}}
	response := broker.Handle(context.Background(), command)
	if response.ErrorCode != "" || process.starts != 1 || coordinator.consumed != 1 {
		t.Fatalf("response=%#v starts=%d consumed=%d", response, process.starts, coordinator.consumed)
	}
	if coordinator.request.OwnerID != "owner-1" || coordinator.request.RedactedArguments != "value=[redacted]" || coordinator.request.Nonce != "nonce-1" || !coordinator.request.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("approval request=%#v", coordinator.request)
	}
	if coordinator.digest != coordinator.request.CommandDigest || coordinator.nonce != "nonce-1" {
		t.Fatalf("consumed digest=%q nonce=%q", coordinator.digest, coordinator.nonce)
	}
}

func TestBrokerApprovalShowsStructuredInputAndRedactsSensitiveFields(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskHigh)
	definition.AuthenticationDriver = "feishu"
	definition.Capabilities[0].Scopes = []string{"message.send"}
	definition.Capabilities[0].Input = InputSchema{Fields: []InputField{
		{Name: "text", Type: InputString, Required: true, Flag: "--text"},
		{Name: "token", Type: InputString, Required: true, Flag: "--token", Sensitive: true},
	}}
	coordinator := &recordingApprovalCoordinator{}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Approval: coordinator, ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "session", ExecutionID: "42", StageID: "session:42:stage:1"},
		ResolveEnvironment: func(context.Context, Definition, Capability, Identity) (EnvironmentResolution, error) {
			return EnvironmentResolution{Environment: map[string]string{"TOKEN": "secret"}, AuthorizationID: "authorization-1", ExternalIdentityID: "ou_1", ExternalDisplayName: "Alice"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser, Target: "chat-1", Input: map[string]any{"text": "hello", "token": "secret-canary"}})
	if response.ErrorCode != "" || process.starts != 1 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
	if coordinator.request.RedactedArguments != `text="hello", token=[redacted]` || strings.Contains(coordinator.request.RedactedArguments, "secret-canary") {
		t.Fatalf("approval summary=%q", coordinator.request.RedactedArguments)
	}
	if coordinator.request.AuthorizationID != "authorization-1" || coordinator.request.ExternalIdentityID != "ou_1" || coordinator.request.ExternalDisplayName != "Alice" || coordinator.request.ManifestVersion != definition.ManifestVersion || coordinator.request.InputDigest == "" {
		t.Fatalf("approval identity and binding=%#v", coordinator.request)
	}
}

func TestBrokerRejectsAuthorizationIdentityChangeAfterApproval(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskHigh)
	definition.AuthenticationDriver = "feishu"
	definition.Capabilities[0].Scopes = []string{"message.send"}
	resolveCalls := 0
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Approval: &recordingApprovalCoordinator{}, ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "session", ExecutionID: "42", StageID: "session:42:stage:1"},
		ResolveEnvironment: func(context.Context, Definition, Capability, Identity) (EnvironmentResolution, error) {
			resolveCalls++
			return EnvironmentResolution{Environment: map[string]string{"TOKEN": "secret"}, AuthorizationID: "authorization-1", ExternalIdentityID: []string{"ou_alice", "ou_bob"}[resolveCalls-1]}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser, Target: "chat-1"})
	if response.ErrorCode != "user_action_unavailable" || resolveCalls != 2 || process.starts != 0 {
		t.Fatalf("response=%#v resolve calls=%d starts=%d", response, resolveCalls, process.starts)
	}
}

func TestBrokerRedactsCredentialsResolvedImmediatelyBeforeExecution(t *testing.T) {
	const token = "connector-token-canary"
	definition := brokerDefinition(RiskLow)
	definition.AuthenticationDriver = "feishu"
	definition.Capabilities[0].Scopes = []string{"identity.read"}
	redactor := &recordingDynamicRedactor{}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: credentialEchoProcess{value: token}}, Redactor: redactor,
		ResolveEnvironment: func(context.Context, Definition, Capability, Identity) (EnvironmentResolution, error) {
			return EnvironmentResolution{Environment: map[string]string{"TOKEN": token}, RedactValues: [][]byte{[]byte(token)}, AuthorizationID: "authorization-1", ExternalIdentityID: "ou_1"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	stdout, decodeErr := base64.StdEncoding.DecodeString(response.StdoutBase64)
	if decodeErr != nil || response.ErrorCode != "" || strings.Contains(string(stdout), token) || string(stdout) != "[REDACTED]" {
		t.Fatalf("response=%#v stdout=%q decode=%v", response, stdout, decodeErr)
	}
}

func TestBrokerReturnsStructuredApprovalExpiry(t *testing.T) {
	definition := brokerDefinition(RiskHigh)
	coordinator := &recordingApprovalCoordinator{awaitErr: ErrApprovalExpired}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: &recordingProcess{}},
		Approval: coordinator, ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "session", ExecutionID: "42", StageID: "session-1:42:1"},
		GenerateNonce: func() (string, error) { return "nonce-1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1"})
	if response.ErrorCode != "user_action_expired" {
		t.Fatalf("response=%#v", response)
	}
}

func TestBrokerBindsUserSelectedIdentityBeforeConsumption(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskHigh)
	definition.Capabilities[0].Identities = []Identity{IdentityUser, IdentityBot}
	coordinator := &recordingApprovalCoordinator{grantIdentity: IdentityBot}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Approval: coordinator, ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "run", ExecutionID: "run-1", StageID: "run-1:stage:1"},
		GenerateNonce: func() (string, error) { return "nonce-1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1"})
	if response.ErrorCode != "" || process.starts != 1 || coordinator.digest != coordinator.request.CommandDigests[IdentityBot] {
		t.Fatalf("response=%#v starts=%d digest=%q bot digest=%q", response, process.starts, coordinator.digest, coordinator.request.CommandDigests[IdentityBot])
	}
}

func TestBrokerClosesApprovalWhenConsumptionFails(t *testing.T) {
	definition := brokerDefinition(RiskHigh)
	coordinator := &recordingApprovalCoordinator{consumeErr: errors.New("stale approval")}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: &recordingProcess{}},
		Approval: coordinator, ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "run", ExecutionID: "run-1", StageID: "run-1:stage:1"},
		GenerateNonce: func() (string, error) { return "nonce-1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1"})
	if response.ErrorCode != "execution_rejected" || coordinator.closed != 1 {
		t.Fatalf("response=%#v closed=%d", response, coordinator.closed)
	}
}

func TestBrokerProtocolRejectsRuntimeSuppliedEnvironment(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	server, client := net.Pipe()
	done := make(chan struct{})
	go func() {
		broker.serveConnection(context.Background(), server)
		close(done)
	}()
	if _, err := client.Write([]byte(`{"connector_id":"connector-1","capability":"identity","identity":"user","arguments":["auth","status"],"environment":{"TOKEN":"forged"}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	var response BrokerResponse
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	<-done
	if response.ErrorCode != "invalid_request" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
}

func TestBrokerRejectsAuthenticatedConnectorWithoutCredentialResolver(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.AuthenticationDriver = "feishu"
	definition.Capabilities[0].Scopes = []string{"im:message:read"}
	definition.Capabilities[0].Scopes = []string{"identity.read"}
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "authorization_unavailable" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
}

func TestBrokerDoesNotAuthorizeCapabilityWithoutPermissions(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.AuthenticationDriver = "feishu"
	resolverCalls := 0
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		ResolveEnvironment: func(context.Context, Definition, Capability, Identity) (EnvironmentResolution, error) {
			resolverCalls++
			return EnvironmentResolution{}, errors.New("authorization should not be requested")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "" || resolverCalls != 0 || process.starts != 1 {
		t.Fatalf("response=%#v resolver calls=%d starts=%d", response, resolverCalls, process.starts)
	}
}

func TestBrokerResolvesCredentialsForReviewedCapability(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.AuthenticationDriver = "feishu"
	definition.Capabilities[0].Scopes = []string{"calendar:calendar:read"}
	called := false
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		ResolveEnvironment: func(_ context.Context, gotDefinition Definition, capability Capability, identity Identity) (EnvironmentResolution, error) {
			called = true
			if gotDefinition.ID != definition.ID || capability.ID != "identity" || len(capability.Scopes) != 1 || identity != IdentityUser {
				t.Fatalf("resolver input = %#v %#v %q", gotDefinition, capability, identity)
			}
			return EnvironmentResolution{Environment: map[string]string{"LARKSUITE_CLI_USER_ACCESS_TOKEN": "secret"}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "" || !called || process.starts != 1 {
		t.Fatalf("response=%#v called=%v starts=%d", response, called, process.starts)
	}
}

func TestBrokerPausesForAuthorizationAndResumesSameOperation(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.AuthenticationDriver = "feishu"
	definition.Capabilities[0].Scopes = []string{"im:message:read"}
	resolveCalls := 0
	actions := &recordingActionCoordinator{}
	events := make([]OperationEvent, 0, 4)
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		ApprovalContext:     ApprovalContext{OwnerID: "owner-1", ExecutionKind: "session", ExecutionID: "42", StageID: "session:42:stage:1"},
		GenerateOperationID: func() (string, error) { return "operation-1", nil }, Actions: actions,
		ResolveEnvironment: func(context.Context, Definition, Capability, Identity) (EnvironmentResolution, error) {
			resolveCalls++
			if resolveCalls == 1 {
				return EnvironmentResolution{}, NewRequirementError(ReasonAuthorizationRequired, "enablement-1", []string{"im:message:read"}, UserActionOpenURL, UserActionCheckStatus)
			}
			return EnvironmentResolution{Environment: map[string]string{"TOKEN": "secret"}, AuthorizationID: "authorization-1", ExternalIdentityID: "ou_1", ExternalDisplayName: "Alice"}, nil
		},
		Events: func(_ context.Context, event OperationEvent) error { events = append(events, event); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser, Target: "chat-1"})
	if response.ErrorCode != "" || resolveCalls != 2 || actions.calls != 1 || process.starts != 1 {
		t.Fatalf("response=%#v resolve=%d actions=%d starts=%d", response, resolveCalls, actions.calls, process.starts)
	}
	if actions.request.OperationID != "operation-1" || actions.request.EnablementID != "enablement-1" || actions.request.Identity != IdentityUser || actions.request.Reason != ReasonAuthorizationRequired || !slices.Equal(actions.request.Permissions, []string{"im:message:read"}) {
		t.Fatalf("action request=%#v", actions.request)
	}
	want := []OperationState{OperationRequested, OperationWaiting, OperationStarted, OperationSucceeded}
	got := make([]OperationState, 0, len(events))
	for _, event := range events {
		got = append(got, event.State)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("operation states=%#v", got)
	}
}

func TestBrokerFailsClosedForUnknownUserAction(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	definition.AuthenticationDriver = "feishu"
	definition.Capabilities[0].Scopes = []string{"im:message:read"}
	actions := &recordingActionCoordinator{}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "session", ExecutionID: "42", StageID: "session:42:stage:1"}, Actions: actions,
		ResolveEnvironment: func(context.Context, Definition, Capability, Identity) (EnvironmentResolution, error) {
			return EnvironmentResolution{}, NewRequirementError(ReasonAuthorizationRequired, "enablement-1", nil, UserAction("provider_magic"))
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "client_incompatible" || process.starts != 0 || actions.calls != 0 {
		t.Fatalf("response=%#v starts=%d action calls=%d", response, process.starts, actions.calls)
	}
}

func TestBrokerRetriesInterruptedReadOnce(t *testing.T) {
	process := &flakyProcess{failures: 1}
	definition := brokerDefinition(RiskLow)
	definition.Capabilities[0].Idempotency = IdempotencyRetrySafe
	events := make([]OperationEvent, 0, 3)
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Events: func(_ context.Context, event OperationEvent) error { events = append(events, event); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "" || process.starts != 2 || events[len(events)-1].State != OperationSucceeded {
		t.Fatalf("response=%#v starts=%d events=%#v", response, process.starts, events)
	}
}

func TestBrokerDoesNotRetryNonIdempotentOperationWithUnknownOutcome(t *testing.T) {
	process := &flakyProcess{failures: 1}
	definition := brokerDefinition(RiskLow)
	definition.Capabilities[0].Idempotency = IdempotencyUnknown
	events := make([]OperationEvent, 0, 3)
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process},
		Events: func(_ context.Context, event OperationEvent) error { events = append(events, event); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser})
	if response.ErrorCode != "outcome_unknown" || process.starts != 1 || events[len(events)-1].State != OperationOutcomeUnknown {
		t.Fatalf("response=%#v starts=%d events=%#v", response, process.starts, events)
	}
}

func TestBrokerRetriesIdempotencyKeyWriteWithoutReusingApproval(t *testing.T) {
	process := &flakyProcess{failures: 1}
	definition := brokerDefinition(RiskHigh)
	definition.Capabilities[0].Input = InputSchema{Fields: []InputField{{Name: "idempotency_key", Type: InputString, Required: true, Flag: "--idempotency-key"}}}
	definition.Capabilities[0].Idempotency = IdempotencyKeyRequired
	coordinator := &recordingApprovalCoordinator{}
	broker, err := NewBroker(BrokerConfig{
		Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}, Approval: coordinator,
		ApprovalContext: ApprovalContext{OwnerID: "owner-1", ExecutionKind: "session", ExecutionID: "42", StageID: "session:42:stage:1"},
		GenerateNonce:   func() (string, error) { return "nonce-1", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: definition.ID, Capability: "identity", Identity: IdentityUser, Target: "chat-1", Input: map[string]any{"idempotency_key": "operation-1"}})
	if response.ErrorCode != "" || process.starts != 2 || coordinator.consumed != 1 {
		t.Fatalf("response=%#v starts=%d approval consumptions=%d", response, process.starts, coordinator.consumed)
	}
}

func brokerDefinition(risk Risk) Definition {
	return Definition{
		ID: "connector-1", Name: "Tool", Executable: "tool", AuthenticationDriver: "none", State: StateAvailable,
		BundleSHA256: strings.Repeat("a", 64), RuntimeDigests: []string{"sha256:" + strings.Repeat("b", 64)},
		Capabilities: []Capability{{ID: "identity", ArgvPrefix: []string{"auth", "status"}, Risk: risk, Identities: []Identity{IdentityUser}, EgressHosts: []string{"example.test"}, Timeout: time.Minute}},
	}
}

type recordingApprovalCoordinator struct {
	request          ApprovalRequest
	awaitErr         error
	consumeErr       error
	grantIdentity    Identity
	consumed, closed int
	digest, nonce    string
}

type recordingActionCoordinator struct {
	request ActionRequest
	err     error
	calls   int
}

type flakyProcess struct {
	starts   int
	failures int
}

type credentialEchoProcess struct{ value string }

func (process credentialEchoProcess) Run(context.Context, ProcessRequest) (Result, error) {
	return Result{Stdout: []byte(process.value), Stderr: []byte(process.value)}, nil
}

type recordingDynamicRedactor struct{ patterns [][]byte }

func (redactor *recordingDynamicRedactor) AddPatterns(values ...[]byte) {
	redactor.patterns = append(redactor.patterns, values...)
}

func (redactor *recordingDynamicRedactor) Bytes(value []byte) []byte {
	result := append([]byte(nil), value...)
	for _, pattern := range redactor.patterns {
		result = []byte(strings.ReplaceAll(string(result), string(pattern), "[REDACTED]"))
	}
	return result
}

func (process *flakyProcess) Run(context.Context, ProcessRequest) (Result, error) {
	process.starts++
	if process.starts <= process.failures {
		return Result{}, errors.New("connection reset after request started")
	}
	return Result{Stdout: []byte("ok")}, nil
}

func (coordinator *recordingActionCoordinator) AwaitAction(_ context.Context, request ActionRequest) error {
	coordinator.calls++
	coordinator.request = request
	return coordinator.err
}

func (coordinator *recordingApprovalCoordinator) Await(_ context.Context, request ApprovalRequest) (ApprovalGrant, error) {
	coordinator.request = request
	if coordinator.awaitErr != nil {
		return ApprovalGrant{}, coordinator.awaitErr
	}
	identity := coordinator.grantIdentity
	if identity == "" {
		identity = request.Identity
	}
	return ApprovalGrant{Nonce: request.Nonce, Identity: identity, ExpiresAt: request.ExpiresAt}, nil
}

func (coordinator *recordingApprovalCoordinator) Consume(_ context.Context, _ string, digest, nonce string) error {
	coordinator.consumed++
	coordinator.digest, coordinator.nonce = digest, nonce
	if coordinator.consumeErr != nil {
		return coordinator.consumeErr
	}
	if digest == "" || nonce == "" {
		return errors.New("missing approval binding")
	}
	return nil
}

func (coordinator *recordingApprovalCoordinator) Close(context.Context, string, string) error {
	coordinator.closed++
	return nil
}
