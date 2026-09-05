package cliconnector

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	defaultBrokerRequestLimit = 1 << 20
	defaultBrokerOutputLimit  = 8 << 20
)

// BrokerCommand is the complete set of fields an untrusted Runtime may choose.
// Bundle paths, digests, policies, credentials, and approval state remain server-owned.
type BrokerCommand struct {
	ConnectorID string   `json:"connector_id"`
	Capability  string   `json:"capability"`
	Identity    Identity `json:"identity"`
	Target      string   `json:"target,omitempty"`
	Arguments   []string `json:"arguments"`
}

type BrokerResponse struct {
	StdoutBase64 string `json:"stdout_base64,omitempty"`
	StderrBase64 string `json:"stderr_base64,omitempty"`
	ExitCode     int    `json:"exit_code,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type EnvironmentResolver func(context.Context, Definition, Identity) (map[string]string, error)

type BrokerConfig struct {
	Definitions        []Definition
	RuntimeDigest      string
	Wrapper            Wrapper
	ResolveEnvironment EnvironmentResolver
	Approval           ApprovalCoordinator
	ApprovalContext    ApprovalContext
	ApprovalTimeout    time.Duration
	Now                func() time.Time
	GenerateNonce      func() (string, error)
	RequestLimit       int64
	OutputLimit        int
}

type ApprovalContext struct {
	OwnerID, ExecutionKind, ExecutionID, StageID string
}

// Broker serializes CLI commands for one Execution Stage and resolves all
// security-sensitive values from its frozen server-side configuration.
type Broker struct {
	definitions        map[string]Definition
	runtimeDigest      string
	wrapper            Wrapper
	resolveEnvironment EnvironmentResolver
	approval           ApprovalCoordinator
	approvalContext    ApprovalContext
	approvalTimeout    time.Duration
	now                func() time.Time
	generateNonce      func() (string, error)
	requestLimit       int64
	outputLimit        int
	mu                 sync.Mutex
}

func NewBroker(config BrokerConfig) (*Broker, error) {
	if !runtimeDigest.MatchString(config.RuntimeDigest) || config.Wrapper.Process == nil || len(config.Definitions) == 0 {
		return nil, errors.New("invalid CLI broker configuration")
	}
	definitions := make(map[string]Definition, len(config.Definitions))
	for _, definition := range config.Definitions {
		if !definitionID.MatchString(definition.ID) || definition.State != StateAvailable || !bundleDigest.MatchString(definition.BundleSHA256) || !containsRuntimeDigest(definition.RuntimeDigests, config.RuntimeDigest) {
			return nil, errors.New("CLI broker requires available frozen Definitions")
		}
		if err := validateExecutionPolicy(definition); err != nil {
			return nil, fmt.Errorf("invalid CLI broker Definition: %w", err)
		}
		if _, duplicate := definitions[definition.ID]; duplicate {
			return nil, errors.New("CLI broker Definitions must be unique")
		}
		definitions[definition.ID] = cloneDefinition(definition)
	}
	approvalTimeout := config.ApprovalTimeout
	if approvalTimeout <= 0 {
		approvalTimeout = 5 * time.Minute
	}
	if approvalTimeout > 15*time.Minute {
		return nil, errors.New("CLI approval timeout exceeds fifteen minutes")
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	generateNonce := config.GenerateNonce
	if generateNonce == nil {
		generateNonce = randomApprovalNonce
	}
	return &Broker{
		definitions: definitions, runtimeDigest: config.RuntimeDigest, wrapper: config.Wrapper,
		resolveEnvironment: config.ResolveEnvironment, approval: config.Approval,
		approvalContext: config.ApprovalContext, approvalTimeout: approvalTimeout,
		now: now, generateNonce: generateNonce, requestLimit: config.RequestLimit, outputLimit: config.OutputLimit,
	}, nil
}

func (broker *Broker) Handle(ctx context.Context, command BrokerCommand) BrokerResponse {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	definition, ok := broker.definitions[command.ConnectorID]
	if !ok {
		return brokerFailure("connector_unavailable", "CLI Connector is unavailable")
	}
	if err := validateBrokerCommand(command); err != nil {
		return brokerFailure("invalid_request", err.Error())
	}
	capability := findCapability(definition, command.Capability)
	if capability == nil {
		return brokerFailure("capability_unavailable", "CLI capability is unavailable")
	}
	request := Request{
		CapabilityID: command.Capability, RuntimeDigest: broker.runtimeDigest,
		BundleSHA256: definition.BundleSHA256, Target: command.Target,
		Identity: command.Identity, Argv: append([]string(nil), command.Arguments...),
	}
	wrapper := broker.wrapper
	wrapper.Now = broker.now
	environment := map[string]string{}
	if definition.AuthenticationDriver != "none" && broker.resolveEnvironment == nil {
		return brokerFailure("authorization_unavailable", "CLI authorization is unavailable")
	}
	if broker.resolveEnvironment != nil {
		resolved, err := broker.resolveEnvironment(ctx, definition, request.Identity)
		if err != nil {
			return brokerFailure("authorization_unavailable", "CLI authorization is unavailable")
		}
		environment = resolved
	}
	request.Environment = environment
	if capability.Risk == RiskHigh {
		if broker.approval == nil || !validApprovalContext(broker.approvalContext) {
			return brokerFailure("user_action_required", "CLI command requires user confirmation")
		}
		nonce, err := broker.generateNonce()
		if err != nil {
			return brokerFailure("user_action_unavailable", "CLI command confirmation is unavailable")
		}
		request.ApprovalNonce = nonce
		request.ApprovalExpiresAt = broker.now().UTC().Add(broker.approvalTimeout)
		digests := make(map[Identity]string, len(capability.Identities))
		for _, identity := range capability.Identities {
			candidate := request
			candidate.Identity = identity
			digests[identity] = CommandDigest(definition, candidate)
		}
		grant, err := broker.approval.Await(ctx, ApprovalRequest{
			OwnerID: broker.approvalContext.OwnerID, ExecutionKind: broker.approvalContext.ExecutionKind,
			ExecutionID: broker.approvalContext.ExecutionID, StageID: broker.approvalContext.StageID,
			ConnectorName: definition.Name, Operation: capability.ID, Target: command.Target,
			RedactedArguments: redactArguments(capability, command.Arguments),
			CommandDigest:     CommandDigest(definition, request), Nonce: nonce,
			Identity: command.Identity, AllowedIdentities: append([]Identity(nil), capability.Identities...),
			CommandDigests: digests, ExpiresAt: request.ApprovalExpiresAt,
		})
		switch {
		case errors.Is(err, ErrApprovalRejected):
			return brokerFailure("user_action_rejected", "CLI command confirmation was rejected")
		case errors.Is(err, ErrApprovalExpired):
			return brokerFailure("user_action_expired", "CLI command confirmation expired")
		case err != nil:
			return brokerFailure("user_action_unavailable", "CLI command confirmation is unavailable")
		}
		if grant.Nonce != nonce || !slices.Contains(capability.Identities, grant.Identity) || !grant.ExpiresAt.Equal(request.ApprovalExpiresAt) {
			return brokerFailure("user_action_unavailable", "CLI command confirmation did not match")
		}
		request.Identity = grant.Identity
		approvalConsumed := false
		defer func() {
			if !approvalConsumed {
				_ = broker.approval.Close(context.WithoutCancel(ctx), broker.approvalContext.OwnerID, nonce)
			}
		}()
		if broker.resolveEnvironment != nil {
			resolved, err := broker.resolveEnvironment(ctx, definition, request.Identity)
			if err != nil {
				return brokerFailure("authorization_unavailable", "CLI authorization is unavailable")
			}
			request.Environment = resolved
		}
		wrapper.ConsumeApproval = func(ctx context.Context, digest, approvalNonce string) error {
			err := broker.approval.Consume(ctx, broker.approvalContext.OwnerID, digest, approvalNonce)
			approvalConsumed = err == nil
			return err
		}
	}
	result, err := wrapper.Execute(ctx, definition, request)
	if err != nil {
		return brokerFailure("execution_rejected", "CLI command was rejected")
	}
	limit := broker.outputLimit
	if limit <= 0 {
		limit = defaultBrokerOutputLimit
	}
	if len(result.Stdout)+len(result.Stderr) > limit {
		return brokerFailure("output_limit", "CLI command output exceeded the limit")
	}
	return BrokerResponse{StdoutBase64: base64.StdEncoding.EncodeToString(result.Stdout), StderrBase64: base64.StdEncoding.EncodeToString(result.Stderr), ExitCode: result.ExitCode}
}

func randomApprovalNonce() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validApprovalContext(value ApprovalContext) bool {
	return value.OwnerID != "" && (value.ExecutionKind == "session" || value.ExecutionKind == "run") && value.ExecutionID != "" && value.StageID != ""
}

func redactArguments(capability *Capability, arguments []string) string {
	visible := strings.Join(capability.ArgvPrefix, " ")
	if len(arguments) > len(capability.ArgvPrefix) {
		visible += " [arguments redacted]"
	}
	return visible
}

func (broker *Broker) Serve(ctx context.Context, listener net.Listener) error {
	if listener == nil {
		return errors.New("CLI broker listener is required")
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
			if ctx.Err() != nil {
				connections.wait()
				return nil
			}
			connections.closeAll()
			connections.wait()
			return fmt.Errorf("accept CLI broker connection: %w", err)
		}
		if !connections.add(connection) {
			continue
		}
		go func() {
			defer connections.done(connection)
			broker.serveConnection(ctx, connection)
		}()
	}
}

type brokerConnections struct {
	mu      sync.Mutex
	active  map[net.Conn]struct{}
	closing bool
	wg      sync.WaitGroup
}

func newBrokerConnections() *brokerConnections {
	return &brokerConnections{active: make(map[net.Conn]struct{})}
}

func (connections *brokerConnections) add(connection net.Conn) bool {
	connections.mu.Lock()
	defer connections.mu.Unlock()
	if connections.closing {
		_ = connection.Close()
		return false
	}
	connections.active[connection] = struct{}{}
	connections.wg.Add(1)
	return true
}

func (connections *brokerConnections) done(connection net.Conn) {
	connections.mu.Lock()
	delete(connections.active, connection)
	connections.mu.Unlock()
	connections.wg.Done()
}

func (connections *brokerConnections) closeAll() {
	connections.mu.Lock()
	connections.closing = true
	for connection := range connections.active {
		_ = connection.Close()
	}
	connections.mu.Unlock()
}

func (connections *brokerConnections) wait() {
	connections.wg.Wait()
}

func (broker *Broker) serveConnection(ctx context.Context, connection net.Conn) {
	defer connection.Close()
	limit := broker.requestLimit
	if limit <= 0 {
		limit = defaultBrokerRequestLimit
	}
	reader := bufio.NewReader(io.LimitReader(connection, limit+1))
	line, err := reader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		_ = writeBrokerResponse(connection, brokerFailure("invalid_request", "unable to read CLI command"))
		return
	}
	if int64(len(line)) > limit {
		_ = writeBrokerResponse(connection, brokerFailure("invalid_request", "CLI command exceeds the request limit"))
		return
	}
	var command BrokerCommand
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(line)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&command); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		_ = writeBrokerResponse(connection, brokerFailure("invalid_request", "invalid CLI command"))
		return
	}
	_ = writeBrokerResponse(connection, broker.Handle(ctx, command))
}

func validateBrokerCommand(command BrokerCommand) error {
	if strings.TrimSpace(command.ConnectorID) == "" || strings.TrimSpace(command.Capability) == "" || len(command.Arguments) == 0 || len(command.Arguments) > 256 {
		return errors.New("CLI command is incomplete")
	}
	if len(command.Target) > 4096 {
		return errors.New("CLI command target is too long")
	}
	for _, argument := range command.Arguments {
		if len(argument) > 64*1024 || strings.ContainsRune(argument, '\x00') {
			return errors.New("CLI command contains an invalid argument")
		}
	}
	return nil
}

func findCapability(definition Definition, id string) *Capability {
	for index := range definition.Capabilities {
		if definition.Capabilities[index].ID == id {
			return &definition.Capabilities[index]
		}
	}
	return nil
}

func containsRuntimeDigest(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneDefinition(value Definition) Definition {
	result := value
	result.RuntimeDigests = append([]string(nil), value.RuntimeDigests...)
	result.SupportedArchitectures = append([]string(nil), value.SupportedArchitectures...)
	result.RecommendedSkillIDs = append([]string(nil), value.RecommendedSkillIDs...)
	result.Capabilities = append([]Capability(nil), value.Capabilities...)
	for index := range result.Capabilities {
		result.Capabilities[index].ArgvPrefix = append([]string(nil), value.Capabilities[index].ArgvPrefix...)
		result.Capabilities[index].Identities = append([]Identity(nil), value.Capabilities[index].Identities...)
		result.Capabilities[index].Scopes = append([]string(nil), value.Capabilities[index].Scopes...)
		result.Capabilities[index].EgressHosts = append([]string(nil), value.Capabilities[index].EgressHosts...)
	}
	return result
}

func brokerFailure(code, message string) BrokerResponse {
	return BrokerResponse{ErrorCode: code, ErrorMessage: message}
}

func writeBrokerResponse(writer io.Writer, response BrokerResponse) error {
	return json.NewEncoder(writer).Encode(response)
}
