package cliconnector

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
