package cliconnector

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnixBrokerServesThroughLongWorkspacePath(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp("/tmp", "broker-long-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	scratch := filepath.Join(root, strings.Repeat("workspace", 12), ".runtime-containers", strings.Repeat("a", 32), "scratch")
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(scratch, "broker", "cli-broker.sock")
	server, err := StartUnixBroker(context.Background(), broker, path, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o660 {
		t.Fatalf("socket permissions: info=%v err=%v", info, err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil || info.Mode().Perm() != 0o550 {
		t.Fatalf("directory permissions: info=%v err=%v", info, err)
	}
	if _, err := os.Lstat(server.listener.Addr().String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary bind alias remains: %v", err)
	}
	// A short alias represents the Runtime's independent mount destination.
	alias := filepath.Join(root, "mounted")
	if err := os.Symlink(filepath.Dir(path), alias); err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("unix", filepath.Join(alias, "cli-broker.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := json.NewEncoder(connection).Encode(BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Arguments: []string{"auth", "status"}}); err != nil {
		t.Fatal(err)
	}
	var response BrokerResponse
	if err := json.NewDecoder(connection).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.ErrorCode != "" || process.starts != 1 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("broker directory remains: %v", err)
	}
}

func TestUnixBrokerServesAndRemovesProtectedSocket(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp("/tmp", "cli-broker-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	path := filepath.Join(root, "broker", "cli-broker.sock")
	server, err := StartUnixBroker(context.Background(), broker, path, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.Stat(filepath.Dir(path))
	if err != nil || directory.Mode().Perm() != 0o550 {
		t.Fatalf("broker directory mode=%v err=%v", directory.Mode().Perm(), err)
	}
	connection, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(connection).Encode(BrokerCommand{ConnectorID: "connector-1", Capability: "identity", Identity: IdentityUser, Arguments: []string{"auth", "status"}}); err != nil {
		t.Fatal(err)
	}
	var response BrokerResponse
	if err := json.NewDecoder(connection).Decode(&response); err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if response.ErrorCode != "" || process.starts != 1 {
		t.Fatalf("response=%#v starts=%d", response, process.starts)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("broker socket remains: %v", err)
	}
}

func TestUnixBrokerCloseTerminatesIncompleteConnection(t *testing.T) {
	process := &recordingProcess{}
	definition := brokerDefinition(RiskLow)
	broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: process}})
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp("/tmp", "cli-broker-cancel-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	path := filepath.Join(root, "broker", "cli-broker.sock")
	server, err := StartUnixBroker(context.Background(), broker, path, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Write([]byte(`{"connector_id":"connector-1"`)); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 1)
	if _, err := connection.Read(buffer); err == nil {
		t.Fatal("incomplete broker connection remained open")
	}
	_ = connection.Close()
}
