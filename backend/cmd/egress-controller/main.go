package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"agent-platform/backend/internal/cliconnector"
)

const defaultSocketPath = "/run/agent-platform/egress-controller.sock"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return errors.New("CLI Egress controller requires Linux root with host NET_ADMIN")
	}
	socketPath := environment("AGENT_EGRESS_CONTROLLER_SOCKET", defaultSocketPath)
	if !filepath.IsAbs(socketPath) || filepath.Base(socketPath) != "egress-controller.sock" || filepath.Dir(socketPath) == "/" || strings.ContainsAny(socketPath, "\x00\r\n") {
		return errors.New("invalid CLI Egress controller socket path")
	}
	resolverAddresses := strings.Fields(environment("AGENT_DNS_SERVERS", "223.5.5.5 1.1.1.1"))
	resolver, err := cliconnector.NewPublicDNSResolver(resolverAddresses)
	if err != nil {
		return err
	}
	gate, err := cliconnector.NewIPTablesEgressGate(cliconnector.IPTablesEgressConfig{
		DockerCommand: "docker", IPTablesCommand: "iptables",
		EgressNetwork:     environment("AGENT_EGRESS_NETWORK", "agent-public-egress"),
		NetworkCIDR:       environment("AGENT_EGRESS_SUBNET", "172.30.0.0/24"),
		ResolverAddresses: resolverAddresses,
		Resolve:           resolver,
	})
	if err != nil {
		return err
	}
	listener, err := listen(socketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}()
	return (cliconnector.HostEgressServer{
		Gate: gate, EgressNetwork: environment("AGENT_EGRESS_NETWORK", "agent-public-egress"),
		NetworkCIDR:       environment("AGENT_EGRESS_SUBNET", "172.30.0.0/24"),
		ResolverAddresses: resolverAddresses,
	}).Serve(ctx, listener)
}

func listen(socketPath string) (*net.UnixListener, error) {
	directory := filepath.Dir(socketPath)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create CLI Egress controller directory: %w", err)
	}
	if info, err := os.Lstat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("reserved CLI Egress controller path is not a socket")
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, fmt.Errorf("remove stale CLI Egress controller socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect CLI Egress controller socket: %w", err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("listen on CLI Egress controller socket: %w", err)
	}
	if err := os.Chmod(socketPath, 0o660); err != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, fmt.Errorf("protect CLI Egress controller socket: %w", err)
	}
	return listener, nil
}

func environment(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
