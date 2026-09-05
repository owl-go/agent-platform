package cliconnector

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
)

type leaseRecordingGate struct {
	events *[]string
}

func (gate leaseRecordingGate) Execute(ctx context.Context, container string, hosts []string, execute func(context.Context) (Result, error)) (Result, error) {
	*gate.events = append(*gate.events, "install:"+container+":"+hosts[0])
	result, err := execute(ctx)
	*gate.events = append(*gate.events, "remove")
	return result, err
}

func TestUnixEgressGateHoldsHostPolicyAroundExecution(t *testing.T) {
	var events []string
	serverConnection, clientConnection := net.Pipe()
	server := testHostEgressServer(leaseRecordingGate{events: &events})
	done := make(chan struct{})
	go func() {
		server.serveConnection(context.Background(), serverConnection)
		close(done)
	}()
	gate, err := NewUnixEgressGate(testUnixEgressConfig(), func(context.Context, string) (net.Conn, error) { return clientConnection, nil })
	if err != nil {
		t.Fatal(err)
	}
	result, err := gate.Execute(context.Background(), "agent-cli-test", []string{"open.feishu.cn"}, func(context.Context) (Result, error) {
		events = append(events, "execute")
		return Result{Stdout: []byte("ok")}, nil
	})
	<-done
	if err != nil || string(result.Stdout) != "ok" {
		t.Fatalf("result=%q err=%v", result.Stdout, err)
	}
	want := []string{"install:agent-cli-test:open.feishu.cn", "execute", "remove"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%#v want=%#v", events, want)
	}
}

func TestUnixEgressGateDisconnectsWithoutReleaseWhenContainerMayBeActive(t *testing.T) {
	serverConnection, clientConnection := net.Pipe()
	callbackErr := make(chan error, 1)
	server := testHostEgressServer(EgressGateFunc(func(ctx context.Context, _ string, _ []string, execute func(context.Context) (Result, error)) (Result, error) {
		_, err := execute(ctx)
		callbackErr <- err
		return Result{}, err
	}))
	go server.serveConnection(context.Background(), serverConnection)
	gate, err := NewUnixEgressGate(testUnixEgressConfig(), func(context.Context, string) (net.Conn, error) { return clientConnection, nil })
	if err != nil {
		t.Fatal(err)
	}
	_, err = gate.Execute(context.Background(), "agent-cli-test", []string{"example.com"}, func(context.Context) (Result, error) {
		return Result{}, ErrEgressSubjectActive
	})
	if !errors.Is(err, ErrEgressSubjectActive) {
		t.Fatalf("err=%v", err)
	}
	if err := <-callbackErr; !errors.Is(err, ErrEgressSubjectActive) {
		t.Fatalf("server callback err=%v", err)
	}
}

func TestHostEgressServerRejectsWorkerConfigurationDrift(t *testing.T) {
	serverConnection, clientConnection := net.Pipe()
	gateCalls := 0
	server := testHostEgressServer(EgressGateFunc(func(context.Context, string, []string, func(context.Context) (Result, error)) (Result, error) {
		gateCalls++
		return Result{}, nil
	}))
	go server.serveConnection(context.Background(), serverConnection)
	config := testUnixEgressConfig()
	config.NetworkCIDR = "172.31.0.0/24"
	gate, err := NewUnixEgressGate(config, func(context.Context, string) (net.Conn, error) { return clientConnection, nil })
	if err != nil {
		t.Fatal(err)
	}
	_, err = gate.Execute(context.Background(), "agent-cli-test", []string{"example.com"}, func(context.Context) (Result, error) {
		t.Fatal("CLI command must not run")
		return Result{}, nil
	})
	if err == nil || gateCalls != 0 {
		t.Fatalf("err=%v gate calls=%d", err, gateCalls)
	}
}

type EgressGateFunc func(context.Context, string, []string, func(context.Context) (Result, error)) (Result, error)

func (function EgressGateFunc) Execute(ctx context.Context, container string, hosts []string, execute func(context.Context) (Result, error)) (Result, error) {
	return function(ctx, container, hosts, execute)
}

func testUnixEgressConfig() UnixEgressConfig {
	return UnixEgressConfig{SocketPath: "/run/agent-platform/egress-controller.sock", EgressNetwork: "agent-public-egress", NetworkCIDR: "172.30.0.0/24", ResolverAddresses: []string{"1.1.1.1"}}
}

func testHostEgressServer(gate EgressGate) HostEgressServer {
	return HostEgressServer{Gate: gate, EgressNetwork: "agent-public-egress", NetworkCIDR: "172.30.0.0/24", ResolverAddresses: []string{"1.1.1.1"}}
}
