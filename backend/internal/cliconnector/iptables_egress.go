package cliconnector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const cliEgressChainPrefix = "AGENT-CLI-"

// ErrEgressSubjectActive keeps the restrictive policy installed when the
// caller cannot prove that the container was stopped or removed.
var ErrEgressSubjectActive = errors.New("CLI Egress subject may still be active")

var dockerNetworkName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type EgressCommandRunner func(context.Context, string, ...string) ([]byte, error)
type HostResolver func(context.Context, string) ([]netip.Addr, error)
type EgressChainName func() (string, error)

type IPTablesEgressConfig struct {
	DockerCommand     string
	IPTablesCommand   string
	EgressNetwork     string
	NetworkCIDR       string
	ResolverAddresses []string
	Run               EgressCommandRunner
	Resolve           HostResolver
	ChainName         EgressChainName
}

// IPTablesEgressGate serializes CLI commands so two temporary policies can
// never widen one container's access by overlapping.
type IPTablesEgressGate struct {
	dockerCommand     string
	iptablesCommand   string
	egressNetwork     string
	network           netip.Prefix
	resolverAddresses []netip.Addr
	run               EgressCommandRunner
	resolve           HostResolver
	chainName         EgressChainName
	mu                sync.Mutex
}

func NewIPTablesEgressGate(config IPTablesEgressConfig) (*IPTablesEgressGate, error) {
	if config.DockerCommand == "" {
		config.DockerCommand = "docker"
	}
	if config.IPTablesCommand == "" {
		config.IPTablesCommand = "iptables"
	}
	network, err := netip.ParsePrefix(config.NetworkCIDR)
	if err != nil || !network.Addr().Is4() || network.Bits() < 16 || network.Bits() > 30 || !network.Addr().IsPrivate() {
		return nil, errors.New("CLI Egress requires an explicit private IPv4 Docker subnet")
	}
	if !dockerNetworkName.MatchString(config.EgressNetwork) || len(config.ResolverAddresses) == 0 {
		return nil, errors.New("CLI Egress network and resolver addresses are required")
	}
	resolvers := make([]netip.Addr, 0, len(config.ResolverAddresses))
	for _, value := range config.ResolverAddresses {
		address, parseErr := netip.ParseAddr(value)
		if parseErr != nil || !isPublicIPv4(address) {
			return nil, errors.New("CLI Egress resolver addresses must be public IPv4")
		}
		resolvers = append(resolvers, address)
	}
	if config.Run == nil {
		config.Run = runEgressCommand
	}
	if config.Resolve == nil {
		config.Resolve = resolveHostIPv4
	}
	if config.ChainName == nil {
		config.ChainName = randomEgressChainName
	}
	return &IPTablesEgressGate{
		dockerCommand: config.DockerCommand, iptablesCommand: config.IPTablesCommand,
		egressNetwork: config.EgressNetwork, network: network.Masked(), resolverAddresses: resolvers,
		run: config.Run, resolve: config.Resolve, chainName: config.ChainName,
	}, nil
}

func (gate *IPTablesEgressGate) Execute(ctx context.Context, container string, hosts []string, execute func(context.Context) (Result, error)) (result Result, returnErr error) {
	if execute == nil || !containerName.MatchString(container) || len(hosts) == 0 {
		return Result{}, errors.New("invalid CLI Egress request")
	}
	gate.mu.Lock()
	defer gate.mu.Unlock()

	destinations, err := gate.resolveDestinations(ctx, hosts)
	if err != nil {
		return Result{}, err
	}
	containerAddress, err := gate.inspectContainerAddress(ctx, container)
	if err != nil {
		return Result{}, err
	}
	chain, err := gate.chainName()
	if err != nil || !validEgressChainName(chain) {
		return Result{}, errors.New("create CLI Egress policy identity")
	}

	created := false
	defer func() {
		if !created || errors.Is(returnErr, ErrEgressSubjectActive) {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		returnErr = errors.Join(returnErr, gate.removePolicy(cleanupCtx, chain, containerAddress))
	}()
	created, err = gate.installPolicy(ctx, chain, containerAddress, destinations)
	if err != nil {
		return Result{}, err
	}
	return execute(ctx)
}

func (gate *IPTablesEgressGate) resolveDestinations(ctx context.Context, hosts []string) ([]netip.Addr, error) {
	unique := make(map[netip.Addr]struct{})
	for _, host := range hosts {
		if !egressHost.MatchString(host) || strings.Contains(host, "..") {
			return nil, errors.New("invalid CLI Egress host")
		}
		addresses, err := gate.resolve(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, fmt.Errorf("resolve CLI Egress host %q", host)
		}
		for _, address := range addresses {
			if !isPublicIPv4(address) {
				return nil, fmt.Errorf("CLI Egress host %q resolved outside public IPv4", host)
			}
			unique[address] = struct{}{}
		}
	}
	result := make([]netip.Addr, 0, len(unique))
	for address := range unique {
		result = append(result, address)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Less(result[j]) })
	return result, nil
}

func (gate *IPTablesEgressGate) inspectContainerAddress(ctx context.Context, container string) (netip.Addr, error) {
	template := fmt.Sprintf(`{{with index .NetworkSettings.Networks %q}}{{.IPAddress}}{{end}}`, gate.egressNetwork)
	output, err := gate.run(ctx, gate.dockerCommand, "inspect", "--format", template, container)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("inspect CLI Runtime container network: %w", err)
	}
	address, err := netip.ParseAddr(strings.TrimSpace(string(output)))
	if err != nil || !address.Is4() || !gate.network.Contains(address) {
		return netip.Addr{}, errors.New("CLI Runtime container is outside the configured Egress subnet")
	}
	return address, nil
}

func (gate *IPTablesEgressGate) installPolicy(ctx context.Context, chain string, source netip.Addr, destinations []netip.Addr) (bool, error) {
	if err := gate.command(ctx, "-N", chain); err != nil {
		return false, err
	}
	for _, resolver := range gate.resolverAddresses {
		for _, protocol := range []string{"udp", "tcp"} {
			if err := gate.command(ctx, "-A", chain, "-d", resolver.String()+"/32", "-p", protocol, "--dport", "53", "-j", "RETURN"); err != nil {
				return true, err
			}
		}
	}
	for _, destination := range destinations {
		if err := gate.command(ctx, "-A", chain, "-d", destination.String()+"/32", "-p", "tcp", "--dport", "443", "-j", "RETURN"); err != nil {
			return true, err
		}
	}
	if err := gate.command(ctx, "-A", chain, "-j", "REJECT"); err != nil {
		return true, err
	}
	if err := gate.command(ctx, "-I", "DOCKER-USER", "1", "-s", source.String()+"/32", "-j", chain); err != nil {
		return true, err
	}
	return true, nil
}

func (gate *IPTablesEgressGate) removePolicy(ctx context.Context, chain string, source netip.Addr) error {
	var result error
	for _, arguments := range [][]string{
		{"-D", "DOCKER-USER", "-s", source.String() + "/32", "-j", chain},
		{"-F", chain},
		{"-X", chain},
	} {
		if err := gate.command(ctx, arguments...); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (gate *IPTablesEgressGate) command(ctx context.Context, arguments ...string) error {
	commandArguments := append([]string{"--wait", "5"}, arguments...)
	output, err := gate.run(ctx, gate.iptablesCommand, commandArguments...)
	if err != nil {
		return fmt.Errorf("apply CLI Egress policy: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runEgressCommand(ctx context.Context, command string, arguments ...string) ([]byte, error) {
	return exec.CommandContext(ctx, command, arguments...).CombinedOutput()
}

func resolveHostIPv4(ctx context.Context, host string) ([]netip.Addr, error) {
	values, err := net.DefaultResolver.LookupNetIP(ctx, "ip4", host)
	if err != nil {
		return nil, err
	}
	return values, nil
}

func NewPublicDNSResolver(values []string) (HostResolver, error) {
	addresses := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		address, err := netip.ParseAddr(value)
		if err != nil || !isPublicIPv4(address) {
			return nil, errors.New("CLI Egress DNS resolvers must be public IPv4 addresses")
		}
		addresses = append(addresses, address)
	}
	if len(addresses) == 0 {
		return nil, errors.New("CLI Egress requires at least one DNS resolver")
	}
	return func(ctx context.Context, host string) ([]netip.Addr, error) {
		var result error
		for _, address := range addresses {
			resolverAddress := net.JoinHostPort(address.String(), "53")
			resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, resolverAddress)
			}}
			resolved, err := resolver.LookupNetIP(ctx, "ip4", host)
			if err == nil && len(resolved) > 0 {
				return resolved, nil
			}
			result = errors.Join(result, err)
		}
		return nil, result
	}, nil
}

func randomEgressChainName() (string, error) {
	var value [6]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return cliEgressChainPrefix + hex.EncodeToString(value[:]), nil
}

func validEgressChainName(value string) bool {
	if !strings.HasPrefix(value, cliEgressChainPrefix) || len(value) > 28 {
		return false
	}
	for _, character := range strings.TrimPrefix(value, cliEgressChainPrefix) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return len(value) > len(cliEgressChainPrefix)
}

func isPublicIPv4(address netip.Addr) bool {
	if !address.IsValid() || !address.Is4() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range nonPublicIPv4Prefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

var nonPublicIPv4Prefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
}
