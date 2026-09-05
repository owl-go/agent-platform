package cliconnector

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func TestIPTablesEgressGateLimitsCommandToResolvedHTTPSAndCleansUp(t *testing.T) {
	var commands [][]string
	policyInstalled := false
	gate, err := NewIPTablesEgressGate(IPTablesEgressConfig{
		EgressNetwork: "agent-public-egress", NetworkCIDR: "172.30.0.0/24", ResolverAddresses: []string{"1.1.1.1"},
		Resolve: func(_ context.Context, host string) ([]netip.Addr, error) {
			if host != "open.feishu.cn" {
				t.Fatalf("resolved host = %q", host)
			}
			return []netip.Addr{netip.MustParseAddr("203.0.114.20"), netip.MustParseAddr("93.184.216.34")}, nil
		},
		ChainName: func() (string, error) { return "AGENT-CLI-abcdef123456", nil },
		Run: func(_ context.Context, command string, arguments ...string) ([]byte, error) {
			call := append([]string{command}, arguments...)
			commands = append(commands, call)
			if command == "docker" {
				return []byte("172.30.0.8\n"), nil
			}
			if reflect.DeepEqual(arguments, []string{"-I", "DOCKER-USER", "1", "-s", "172.30.0.8/32", "-j", "AGENT-CLI-abcdef123456"}) {
				policyInstalled = true
			}
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	executed := false
	result, err := gate.Execute(context.Background(), "agent-runtime-1", []string{"open.feishu.cn"}, func(context.Context) (Result, error) {
		if !policyInstalled {
			t.Fatal("CLI command ran before Egress policy installation")
		}
		executed = true
		return Result{Stdout: []byte("ok")}, nil
	})
	if err != nil || !executed || string(result.Stdout) != "ok" {
		t.Fatalf("result=%q executed=%v err=%v", result.Stdout, executed, err)
	}

	want := [][]string{
		{"docker", "inspect", "--format", `{{with index .NetworkSettings.Networks "agent-public-egress"}}{{.IPAddress}}{{end}}`, "agent-runtime-1"},
		{"iptables", "-N", "AGENT-CLI-abcdef123456"},
		{"iptables", "-A", "AGENT-CLI-abcdef123456", "-d", "1.1.1.1/32", "-p", "udp", "--dport", "53", "-j", "RETURN"},
		{"iptables", "-A", "AGENT-CLI-abcdef123456", "-d", "1.1.1.1/32", "-p", "tcp", "--dport", "53", "-j", "RETURN"},
		{"iptables", "-A", "AGENT-CLI-abcdef123456", "-d", "93.184.216.34/32", "-p", "tcp", "--dport", "443", "-j", "RETURN"},
		{"iptables", "-A", "AGENT-CLI-abcdef123456", "-d", "203.0.114.20/32", "-p", "tcp", "--dport", "443", "-j", "RETURN"},
		{"iptables", "-A", "AGENT-CLI-abcdef123456", "-j", "REJECT"},
		{"iptables", "-I", "DOCKER-USER", "1", "-s", "172.30.0.8/32", "-j", "AGENT-CLI-abcdef123456"},
		{"iptables", "-D", "DOCKER-USER", "-s", "172.30.0.8/32", "-j", "AGENT-CLI-abcdef123456"},
		{"iptables", "-F", "AGENT-CLI-abcdef123456"},
		{"iptables", "-X", "AGENT-CLI-abcdef123456"},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestIPTablesEgressGateRejectsPrivateResolutionBeforeContainerInspection(t *testing.T) {
	calls := 0
	gate, err := NewIPTablesEgressGate(IPTablesEgressConfig{
		EgressNetwork: "public", NetworkCIDR: "172.30.0.0/24", ResolverAddresses: []string{"1.1.1.1"},
		Resolve: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
		},
		Run: func(context.Context, string, ...string) ([]byte, error) { calls++; return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = gate.Execute(context.Background(), "runtime-1", []string{"example.com"}, func(context.Context) (Result, error) {
		t.Fatal("CLI command must not run")
		return Result{}, nil
	})
	if err == nil || calls != 0 || !strings.Contains(err.Error(), "outside public IPv4") {
		t.Fatalf("err=%v command calls=%d", err, calls)
	}
}

func TestIPTablesEgressGateRejectsContainerOutsideConfiguredSubnet(t *testing.T) {
	iptablesCalls := 0
	gate, err := NewIPTablesEgressGate(IPTablesEgressConfig{
		EgressNetwork: "public", NetworkCIDR: "172.30.0.0/24", ResolverAddresses: []string{"1.1.1.1"},
		Resolve: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		},
		Run: func(_ context.Context, command string, _ ...string) ([]byte, error) {
			if command == "docker" {
				return []byte("172.31.0.8"), nil
			}
			iptablesCalls++
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = gate.Execute(context.Background(), "runtime-1", []string{"example.com"}, func(context.Context) (Result, error) {
		t.Fatal("CLI command must not run")
		return Result{}, nil
	})
	if err == nil || iptablesCalls != 0 {
		t.Fatalf("err=%v iptables calls=%d", err, iptablesCalls)
	}
}

func TestIPTablesEgressGateCleansRulesWhenCommandFails(t *testing.T) {
	var cleanup []string
	gate, err := NewIPTablesEgressGate(IPTablesEgressConfig{
		EgressNetwork: "public", NetworkCIDR: "172.30.0.0/24", ResolverAddresses: []string{"1.1.1.1"},
		Resolve: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		},
		ChainName: func() (string, error) { return "AGENT-CLI-0123456789ab", nil },
		Run: func(_ context.Context, command string, arguments ...string) ([]byte, error) {
			if command == "docker" {
				return []byte("172.30.0.8"), nil
			}
			if len(arguments) > 0 && (arguments[0] == "-D" || arguments[0] == "-F" || arguments[0] == "-X") {
				cleanup = append(cleanup, arguments[0])
			}
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("CLI failed")
	_, err = gate.Execute(context.Background(), "runtime-1", []string{"example.com"}, func(context.Context) (Result, error) { return Result{}, wantErr })
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(cleanup, []string{"-D", "-F", "-X"}) {
		t.Fatalf("err=%v cleanup=%v", err, cleanup)
	}
}

func TestIPTablesEgressGateDoesNotRemoveChainItFailedToCreate(t *testing.T) {
	var cleanupCalls int
	gate, err := NewIPTablesEgressGate(IPTablesEgressConfig{
		EgressNetwork: "public", NetworkCIDR: "172.30.0.0/24", ResolverAddresses: []string{"1.1.1.1"},
		Resolve: func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
		},
		ChainName: func() (string, error) { return "AGENT-CLI-0123456789ab", nil },
		Run: func(_ context.Context, command string, arguments ...string) ([]byte, error) {
			if command == "docker" {
				return []byte("172.30.0.8"), nil
			}
			if reflect.DeepEqual(arguments, []string{"-N", "AGENT-CLI-0123456789ab"}) {
				return nil, errors.New("chain already exists")
			}
			cleanupCalls++
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = gate.Execute(context.Background(), "runtime-1", []string{"example.com"}, func(context.Context) (Result, error) {
		t.Fatal("CLI command must not run")
		return Result{}, nil
	})
	if err == nil || cleanupCalls != 0 {
		t.Fatalf("err=%v cleanup calls=%d", err, cleanupCalls)
	}
}

func TestPublicIPv4RejectsNonPublicRanges(t *testing.T) {
	for _, value := range []string{"0.0.0.1", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.1.1", "172.16.0.1", "192.0.2.1", "192.168.1.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1", "240.0.0.1", "::1"} {
		if isPublicIPv4(netip.MustParseAddr(value)) {
			t.Fatalf("address %s unexpectedly accepted", value)
		}
	}
	if !isPublicIPv4(netip.MustParseAddr("93.184.216.34")) {
		t.Fatal("public IPv4 address was rejected")
	}
}
