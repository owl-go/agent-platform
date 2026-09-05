package cliconnector

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const egressControlRequestLimit = 64 << 10

type egressAcquire struct {
	Container         string   `json:"container"`
	Hosts             []string `json:"hosts"`
	EgressNetwork     string   `json:"egress_network"`
	NetworkCIDR       string   `json:"network_cidr"`
	ResolverAddresses []string `json:"resolver_addresses"`
}

type egressRelease struct {
	Release bool `json:"release"`
}

type egressControlResponse struct {
	State string `json:"state,omitempty"`
	Error string `json:"error,omitempty"`
}

// HostEgressServer owns host-network policy. Its protocol exposes only an
// acquire/release lease, never raw iptables arguments.
type HostEgressServer struct {
	Gate              EgressGate
	EgressNetwork     string
	NetworkCIDR       string
	ResolverAddresses []string
}

func (server HostEgressServer) Serve(ctx context.Context, listener net.Listener) error {
	if server.Gate == nil || listener == nil || !validRemoteEgressConfig(server.EgressNetwork, server.NetworkCIDR, server.ResolverAddresses) {
		return errors.New("host CLI Egress server requires a Gate and listener")
	}
	connections := newBrokerConnections()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
		connections.closeAll()
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			connections.closeAll()
			connections.wait()
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept host CLI Egress connection: %w", err)
		}
		if !connections.add(connection) {
			continue
		}
		go func() {
			defer connections.done(connection)
			server.serveConnection(ctx, connection)
		}()
	}
}

func (server HostEgressServer) serveConnection(ctx context.Context, connection net.Conn) {
	defer connection.Close()
	decoder := json.NewDecoder(bufio.NewReader(io.LimitReader(connection, egressControlRequestLimit)))
	decoder.DisallowUnknownFields()
	var request egressAcquire
	if err := decoder.Decode(&request); err != nil {
		_ = json.NewEncoder(connection).Encode(egressControlResponse{Error: "invalid acquire request"})
		return
	}
	if request.EgressNetwork != server.EgressNetwork || request.NetworkCIDR != server.NetworkCIDR || !slices.Equal(request.ResolverAddresses, server.ResolverAddresses) {
		_ = json.NewEncoder(connection).Encode(egressControlResponse{Error: "Egress configuration drift"})
		return
	}
	ready := false
	_, err := server.Gate.Execute(ctx, request.Container, request.Hosts, func(context.Context) (Result, error) {
		if err := json.NewEncoder(connection).Encode(egressControlResponse{State: "ready"}); err != nil {
			return Result{}, ErrEgressSubjectActive
		}
		ready = true
		var release egressRelease
		if err := decoder.Decode(&release); err != nil || !release.Release {
			return Result{}, ErrEgressSubjectActive
		}
		return Result{}, nil
	})
	if err != nil {
		if !ready {
			_ = json.NewEncoder(connection).Encode(egressControlResponse{Error: "Egress policy rejected"})
		}
		return
	}
	_ = json.NewEncoder(connection).Encode(egressControlResponse{State: "released"})
}

type EgressDialer func(context.Context, string) (net.Conn, error)

type UnixEgressGate struct {
	config UnixEgressConfig
	dial   EgressDialer
}

type UnixEgressConfig struct {
	SocketPath        string
	EgressNetwork     string
	NetworkCIDR       string
	ResolverAddresses []string
}

func NewUnixEgressGate(config UnixEgressConfig, dial EgressDialer) (*UnixEgressGate, error) {
	if !filepath.IsAbs(config.SocketPath) || filepath.Base(config.SocketPath) != "egress-controller.sock" || strings.ContainsAny(config.SocketPath, "\x00\r\n") || !validRemoteEgressConfig(config.EgressNetwork, config.NetworkCIDR, config.ResolverAddresses) {
		return nil, errors.New("invalid host CLI Egress controller socket")
	}
	if dial == nil {
		dialer := &net.Dialer{}
		dial = func(ctx context.Context, path string) (net.Conn, error) { return dialer.DialContext(ctx, "unix", path) }
	}
	config.ResolverAddresses = append([]string(nil), config.ResolverAddresses...)
	return &UnixEgressGate{config: config, dial: dial}, nil
}

func validRemoteEgressConfig(network, cidr string, resolvers []string) bool {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !dockerNetworkName.MatchString(network) || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() < 16 || prefix.Bits() > 30 || len(resolvers) == 0 {
		return false
	}
	for _, value := range resolvers {
		address, err := netip.ParseAddr(value)
		if err != nil || !isPublicIPv4(address) {
			return false
		}
	}
	return true
}

func (gate *UnixEgressGate) Execute(ctx context.Context, container string, hosts []string, execute func(context.Context) (Result, error)) (Result, error) {
	if execute == nil {
		return Result{}, errors.New("CLI Egress execution callback is required")
	}
	connection, err := gate.dial(ctx, gate.config.SocketPath)
	if err != nil {
		return Result{}, fmt.Errorf("connect host CLI Egress controller: %w", err)
	}
	defer connection.Close()
	encoder, decoder := json.NewEncoder(connection), json.NewDecoder(connection)
	if err := encoder.Encode(egressAcquire{
		Container: container, Hosts: append([]string(nil), hosts...),
		EgressNetwork: gate.config.EgressNetwork, NetworkCIDR: gate.config.NetworkCIDR,
		ResolverAddresses: append([]string(nil), gate.config.ResolverAddresses...),
	}); err != nil {
		return Result{}, fmt.Errorf("acquire host CLI Egress policy: %w", err)
	}
	var response egressControlResponse
	if err := decoder.Decode(&response); err != nil || response.State != "ready" {
		return Result{}, errors.New("host CLI Egress policy was rejected")
	}
	result, executeErr := execute(ctx)
	if errors.Is(executeErr, ErrEgressSubjectActive) {
		return result, executeErr
	}
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	if err := encoder.Encode(egressRelease{Release: true}); err != nil {
		return result, errors.Join(executeErr, fmt.Errorf("release host CLI Egress policy: %w", err))
	}
	response = egressControlResponse{}
	if err := decoder.Decode(&response); err != nil || response.State != "released" {
		return result, errors.Join(executeErr, errors.New("host CLI Egress policy release was not confirmed"))
	}
	return result, executeErr
}
