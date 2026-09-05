package cliconnector

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
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
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Arguments: []string{"auth", "status"}})
	stdout, decodeErr := base64.StdEncoding.DecodeString(response.StdoutBase64)
	if decodeErr != nil || response.ErrorCode != "" || string(stdout) != "ok" || process.executable != "tool" {
		t.Fatalf("response=%#v executable=%q decode=%v", response, process.executable, decodeErr)
	}
}

func TestBrokerRequiresUserActionBeforeHighRiskProcessStart(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskHigh)
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1", Arguments: []string{"auth", "status"}})
	if response.ErrorCode != "user_action_required" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
}

func TestBrokerWaitsForAndConsumesOneUseHighRiskApproval(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskHigh)
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
	command := BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1", Arguments: []string{"auth", "status", "secret"}}
	response := broker.Handle(context.Background(), command)
	if response.ErrorCode != "" || process.starts != 1 || coordinator.consumed != 1 {
		t.Fatalf("response=%#v starts=%d consumed=%d", response, process.starts, coordinator.consumed)
	}
	if coordinator.request.OwnerID != "owner-1" || coordinator.request.RedactedArguments != "auth status [arguments redacted]" || coordinator.request.Nonce != "nonce-1" || !coordinator.request.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("approval request=%#v", coordinator.request)
	}
	if coordinator.digest != coordinator.request.CommandDigest || coordinator.nonce != "nonce-1" {
		t.Fatalf("consumed digest=%q nonce=%q", coordinator.digest, coordinator.nonce)
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
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1", Arguments: []string{"auth", "status"}})
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
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1", Arguments: []string{"auth", "status"}})
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
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Target: "chat-1", Arguments: []string{"auth", "status"}})
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
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	response := broker.Handle(context.Background(), BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Arguments: []string{"auth", "status"}})
	if response.ErrorCode != "authorization_unavailable" || process.starts != 0 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
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
