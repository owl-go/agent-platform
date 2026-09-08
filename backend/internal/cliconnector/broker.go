package cliconnector

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	defaultBrokerRequestLimit = 1 << 20
	defaultBrokerOutputLimit  = 8 << 20
	BrokerCommandExecute      = "execute"
	BrokerCommandDescribe     = "describe"
)

type OperationState string

const (
	OperationRequested      OperationState = "requested"
	OperationWaiting        OperationState = "waiting"
	OperationStarted        OperationState = "started"
	OperationSucceeded      OperationState = "succeeded"
	OperationFailed         OperationState = "failed"
	OperationCancelled      OperationState = "cancelled"
	OperationTimedOut       OperationState = "timed_out"
	OperationOutcomeUnknown OperationState = "outcome_unknown"
)

// OperationEvent is the transport-neutral, user-visible Connector lifecycle.
// Process diagnostics remain in Runtime command events.
type OperationEvent struct {
	ContractVersion int               `json:"contract_version"`
	OperationID     string            `json:"operation_id"`
	ConnectorID     string            `json:"connector_id"`
	ConnectorName   string            `json:"connector_name"`
	CapabilityID    string            `json:"capability_id"`
	OperationPhrase map[string]string `json:"operation_phrase,omitempty"`
	State           OperationState    `json:"state"`
	ReasonCode      string            `json:"reason_code,omitempty"`
	Target          string            `json:"target,omitempty"`
	OwnerID         string            `json:"-"`
	ManifestVersion string            `json:"-"`
	Permissions     []string          `json:"-"`
	InputDigest     string            `json:"-"`
	Identity        Identity          `json:"-"`
	AuthorizationID string            `json:"-"`
}

type OperationEventSink func(context.Context, OperationEvent) error

type AuditRecord struct {
	ContractVersion   int
	OperationID       string
	UserID            string
	ConnectorID       string
	ManifestVersion   string
	CapabilityID      string
	Permissions       []string
	ExecutionIdentity Identity
	AuthorizationID   string
	Action            string
	Reason            string
	Result            string
	TargetSummary     string
	InputDigest       string
	OccurredAt        time.Time
}

type AuditSink func(context.Context, AuditRecord) error

// BrokerCommand is the complete set of fields an untrusted Runtime may choose.
// Bundle paths, digests, policies, credentials, and approval state remain server-owned.
type BrokerCommand struct {
	Kind        string         `json:"kind,omitempty"`
	ConnectorID string         `json:"connector_id"`
	Capability  string         `json:"capability"`
	Identity    Identity       `json:"identity"`
	Target      string         `json:"target,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	Arguments   []string       `json:"arguments,omitempty"`
}

type ConnectorDescription struct {
	ManifestVersion string       `json:"manifest_version"`
	UsageGuide      string       `json:"usage_guide"`
	Capabilities    []Capability `json:"capabilities"`
}

type BrokerResponse struct {
	StdoutBase64 string `json:"stdout_base64,omitempty"`
	StderrBase64 string `json:"stderr_base64,omitempty"`
	ExitCode     int    `json:"exit_code,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type EnvironmentResolution struct {
	Environment         map[string]string
	RedactValues        [][]byte
	AuthorizationID     string
	ExternalIdentityID  string
	ExternalDisplayName string
}

type DynamicSecretRedactor interface {
	AddPatterns(...[]byte)
	Bytes([]byte) []byte
}

type EnvironmentResolver func(context.Context, Definition, Capability, Identity) (EnvironmentResolution, error)

type BrokerConfig struct {
	Definitions         []Definition
	RuntimeDigest       string
	Wrapper             Wrapper
	ResolveEnvironment  EnvironmentResolver
	Approval            ApprovalCoordinator
	Actions             ActionCoordinator
	ApprovalContext     ApprovalContext
	ApprovalTimeout     time.Duration
	Now                 func() time.Time
	GenerateNonce       func() (string, error)
	GenerateOperationID func() (string, error)
	Events              OperationEventSink
	Audit               AuditSink
	Redactor            DynamicSecretRedactor
	RequestLimit        int64
	OutputLimit         int
}

type ApprovalContext struct {
	OwnerID, ExecutionKind, ExecutionID, StageID string
}

// Broker serializes CLI commands for one Execution Stage and resolves all
// security-sensitive values from its frozen server-side configuration.
type Broker struct {
	definitions         map[string]Definition
	runtimeDigest       string
	wrapper             Wrapper
	resolveEnvironment  EnvironmentResolver
	approval            ApprovalCoordinator
	actions             ActionCoordinator
	approvalContext     ApprovalContext
	approvalTimeout     time.Duration
	now                 func() time.Time
	generateNonce       func() (string, error)
	generateOperationID func() (string, error)
	events              OperationEventSink
	audit               AuditSink
	redactor            DynamicSecretRedactor
	requestLimit        int64
	outputLimit         int
	mu                  sync.Mutex
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
	generateOperationID := config.GenerateOperationID
	if generateOperationID == nil {
		generateOperationID = randomApprovalNonce
	}
	return &Broker{
		definitions: definitions, runtimeDigest: config.RuntimeDigest, wrapper: config.Wrapper,
		resolveEnvironment: config.ResolveEnvironment, approval: config.Approval, actions: config.Actions,
		approvalContext: config.ApprovalContext, approvalTimeout: approvalTimeout,
		now: now, generateNonce: generateNonce, generateOperationID: generateOperationID, events: config.Events, audit: config.Audit, redactor: config.Redactor,
		requestLimit: config.RequestLimit, outputLimit: config.OutputLimit,
	}, nil
}

func (broker *Broker) Handle(ctx context.Context, command BrokerCommand) BrokerResponse {
	broker.mu.Lock()
	defer broker.mu.Unlock()
	definition, ok := broker.definitions[command.ConnectorID]
	if !ok {
		return brokerFailure("connector_unavailable", "CLI Connector is unavailable")
	}
	if command.Kind == BrokerCommandDescribe {
		if command.Capability != "" || command.Identity != "" || command.Input != nil || len(command.Arguments) > 0 || command.Target != "" {
			return brokerFailure("invalid_request", "Connector description request contains execution fields")
		}
		description, err := json.Marshal(ConnectorDescription{
			ManifestVersion: definition.ManifestVersion,
			UsageGuide:      definition.UsageGuide,
			Capabilities:    cloneDefinition(definition).Capabilities,
		})
		if err != nil {
			return brokerFailure("description_unavailable", "Connector description is unavailable")
		}
		return BrokerResponse{StdoutBase64: base64.StdEncoding.EncodeToString(description)}
	}
	if err := validateBrokerCommand(command); err != nil {
		return brokerFailure("invalid_request", err.Error())
	}
	capability := findCapability(definition, command.Capability)
	if capability == nil {
		return brokerFailure("capability_unavailable", "CLI capability is unavailable")
	}
	if len(command.Arguments) > 0 {
		return brokerFailure("invalid_request", "CLI capability does not accept free-form arguments")
	}
	arguments, err := renderStructuredArguments(*capability, command.Input)
	if err != nil {
		return brokerFailure("invalid_request", err.Error())
	}
	operationID, err := broker.generateOperationID()
	if err != nil {
		return brokerFailure("event_unavailable", "Connector operation could not be recorded")
	}
	event := OperationEvent{
		ContractVersion: 1, OperationID: operationID, ConnectorID: definition.ID, ConnectorName: definition.Name,
		CapabilityID: capability.ID, OperationPhrase: maps.Clone(capability.OperationPhrase), State: OperationRequested, Target: command.Target,
		OwnerID: broker.approvalContext.OwnerID, ManifestVersion: definition.ManifestVersion,
		Permissions: append([]string(nil), capability.Scopes...), InputDigest: brokerInputDigest(command), Identity: command.Identity,
	}
	if err := broker.emitOperation(ctx, event); err != nil {
		return brokerFailure("event_unavailable", "Connector operation could not be recorded")
	}
	failOperation := func(code, message, reason string, state OperationState) BrokerResponse {
		event.State, event.ReasonCode = state, reason
		if err := broker.emitOperation(ctx, event); err != nil {
			return brokerFailure("event_unavailable", "Connector operation could not be recorded")
		}
		return brokerFailure(code, message)
	}
	request := Request{
		CapabilityID: command.Capability, RuntimeDigest: broker.runtimeDigest,
		BundleSHA256: definition.BundleSHA256, Target: command.Target,
		Identity: command.Identity, Argv: arguments,
	}
	outputLimit := broker.outputLimit
	if outputLimit <= 0 {
		outputLimit = defaultBrokerOutputLimit
	}
	request.OutputLimit = outputLimit
	wrapper := broker.wrapper
	wrapper.Now = broker.now
	resolution := EnvironmentResolution{Environment: map[string]string{}}
	protected := len(capability.Scopes) > 0
	if protected && definition.AuthenticationDriver != "none" && broker.resolveEnvironment == nil {
		return failOperation("authorization_unavailable", "CLI authorization is unavailable", "scheme_unavailable", OperationFailed)
	}
	if protected && broker.resolveEnvironment != nil {
		resolved, err := broker.resolveEnvironment(ctx, definition, *capability, request.Identity)
		if err != nil {
			var requirement *RequirementError
			if !errors.As(err, &requirement) || broker.actions == nil || !validApprovalContext(broker.approvalContext) {
				return failOperation("authorization_unavailable", "CLI authorization is unavailable", "authorization_required", OperationFailed)
			}
			event.State, event.ReasonCode = OperationWaiting, string(requirement.Reason)
			if err := broker.emitOperation(ctx, event); err != nil {
				return brokerFailure("event_unavailable", "Connector operation could not be recorded")
			}
			actionRequest := ActionRequest{
				OwnerID: broker.approvalContext.OwnerID, ExecutionKind: broker.approvalContext.ExecutionKind,
				ExecutionID: broker.approvalContext.ExecutionID, StageID: broker.approvalContext.StageID,
				OperationID: operationID, ConnectorID: definition.ID, ConnectorName: definition.Name,
				AuthorizationScheme: definition.AuthenticationDriver,
				Identity:            request.Identity,
				EnablementID:        requirement.EnablementID, CapabilityID: capability.ID,
				OperationPhrase: maps.Clone(capability.OperationPhrase), Reason: requirement.Reason,
				Permissions: append([]string(nil), requirement.Permissions...), Actions: append([]UserAction(nil), requirement.Actions...),
				ExpiresAt: broker.now().UTC().Add(broker.approvalTimeout),
			}
			if err := ValidateActionRequest(actionRequest); err != nil {
				return failOperation("client_incompatible", "Connector action cannot be handled by this platform version", "unsupported_action", OperationFailed)
			}
			actionErr := broker.actions.AwaitAction(ctx, actionRequest)
			switch {
			case errors.Is(actionErr, ErrActionRejected):
				return failOperation("user_action_rejected", "Connector action was rejected", string(ReasonCancelledByUser), OperationCancelled)
			case errors.Is(actionErr, ErrActionExpired):
				return failOperation("user_action_expired", "Connector action expired", string(ReasonUserActionExpired), OperationTimedOut)
			case actionErr != nil:
				return failOperation("user_action_unavailable", "Connector action is unavailable", string(ReasonProviderUnavailable), OperationFailed)
			}
			resolved, err = broker.resolveEnvironment(ctx, definition, *capability, request.Identity)
			if err != nil {
				return failOperation("authorization_unavailable", "CLI authorization is unavailable", "authorization_required", OperationFailed)
			}
		}
		resolution = resolved
		broker.registerSecrets(resolved.RedactValues)
		request.AuthorizationID = resolved.AuthorizationID
		request.ExternalIdentityID = resolved.ExternalIdentityID
		event.AuthorizationID = resolved.AuthorizationID
	}
	request.Environment = resolution.Environment
	if capability.Risk == RiskHigh {
		if broker.approval == nil || !validApprovalContext(broker.approvalContext) {
			return failOperation("user_action_required", "CLI command requires user confirmation", "approval_required", OperationFailed)
		}
		nonce, err := broker.generateNonce()
		if err != nil {
			return failOperation("user_action_unavailable", "CLI command confirmation is unavailable", "approval_unavailable", OperationFailed)
		}
		request.ApprovalNonce = nonce
		request.ApprovalExpiresAt = broker.now().UTC().Add(broker.approvalTimeout)
		digests := make(map[Identity]string, len(capability.Identities))
		for _, identity := range capability.Identities {
			candidate := request
			candidate.Identity = identity
			digests[identity] = CommandDigest(definition, candidate)
		}
		event.State, event.ReasonCode = OperationWaiting, "approval_required"
		if err := broker.emitOperation(ctx, event); err != nil {
			return brokerFailure("event_unavailable", "Connector operation could not be recorded")
		}
		grant, err := broker.approval.Await(ctx, ApprovalRequest{
			OwnerID: broker.approvalContext.OwnerID, ExecutionKind: broker.approvalContext.ExecutionKind,
			ExecutionID: broker.approvalContext.ExecutionID, StageID: broker.approvalContext.StageID,
			ConnectorName: definition.Name, Operation: capability.ID, Target: command.Target,
			RedactedArguments: approvalInputSummary(capability, command.Input, arguments),
			CommandDigest:     CommandDigest(definition, request), Nonce: nonce,
			OperationID: operationID, ManifestVersion: definition.ManifestVersion, InputDigest: event.InputDigest,
			AuthorizationID: resolution.AuthorizationID, ExternalIdentityID: resolution.ExternalIdentityID, ExternalDisplayName: resolution.ExternalDisplayName,
			Identity: command.Identity, AllowedIdentities: append([]Identity(nil), capability.Identities...),
			CommandDigests: digests, ExpiresAt: request.ApprovalExpiresAt,
		})
		switch {
		case errors.Is(err, ErrApprovalRejected):
			return failOperation("user_action_rejected", "CLI command confirmation was rejected", "cancelled_by_user", OperationCancelled)
		case errors.Is(err, ErrApprovalExpired):
			return failOperation("user_action_expired", "CLI command confirmation expired", "user_action_expired", OperationTimedOut)
		case err != nil:
			return failOperation("user_action_unavailable", "CLI command confirmation is unavailable", "approval_unavailable", OperationFailed)
		}
		if grant.Nonce != nonce || (protected && grant.Identity != request.Identity) || !slices.Contains(capability.Identities, grant.Identity) || !grant.ExpiresAt.Equal(request.ApprovalExpiresAt) {
			return failOperation("user_action_unavailable", "CLI command confirmation did not match", "approval_mismatch", OperationFailed)
		}
		request.Identity = grant.Identity
		approvalConsumed := false
		defer func() {
			if !approvalConsumed {
				_ = broker.approval.Close(context.WithoutCancel(ctx), broker.approvalContext.OwnerID, nonce)
			}
		}()
		if protected && broker.resolveEnvironment != nil {
			resolved, err := broker.resolveEnvironment(ctx, definition, *capability, request.Identity)
			if err != nil {
				return failOperation("authorization_unavailable", "CLI authorization is unavailable", "authorization_required", OperationFailed)
			}
			if resolved.AuthorizationID != request.AuthorizationID || resolved.ExternalIdentityID != request.ExternalIdentityID {
				return failOperation("user_action_unavailable", "CLI command confirmation did not match the current authorization", "approval_mismatch", OperationFailed)
			}
			request.Environment = resolved.Environment
			broker.registerSecrets(resolved.RedactValues)
			resolution = resolved
			event.AuthorizationID = resolved.AuthorizationID
		}
		wrapper.ConsumeApproval = func(ctx context.Context, digest, approvalNonce string) error {
			err := broker.approval.Consume(ctx, broker.approvalContext.OwnerID, digest, approvalNonce)
			approvalConsumed = err == nil
			return err
		}
	}
	event.State, event.ReasonCode = OperationStarted, ""
	if err := broker.emitOperation(ctx, event); err != nil {
		return brokerFailure("event_unavailable", "Connector operation could not be recorded")
	}
	result, err := wrapper.Execute(ctx, definition, request)
	if err != nil {
		var executionError *ExecutionError
		if !errors.As(err, &executionError) {
			return failOperation("execution_rejected", "CLI command was rejected", "execution_rejected", OperationFailed)
		}
		if errors.Is(err, ErrOutputLimit) {
			return failOperation("output_limit", "CLI command output exceeded the limit", "output_limit", OperationFailed)
		}
		if (capability.Risk == RiskLow && capability.Idempotency == IdempotencyRetrySafe) || capability.Idempotency == IdempotencyKeyRequired {
			retryWrapper := wrapper
			if capability.Risk == RiskHigh {
				// The same in-memory operation already consumed its one-use approval.
				retryWrapper.ConsumeApproval = func(context.Context, string, string) error { return nil }
			}
			result, err = retryWrapper.Execute(ctx, definition, request)
		}
		if err != nil {
			if capability.Idempotency == IdempotencyUnknown || capability.Idempotency == IdempotencyKeyRequired {
				return failOperation("outcome_unknown", "Connector operation outcome is unknown", "transport_outcome_unknown", OperationOutcomeUnknown)
			}
			return failOperation("execution_failed", "Connector operation failed", "transport_failed", OperationFailed)
		}
	}
	if len(result.Stdout)+len(result.Stderr) > outputLimit {
		return failOperation("output_limit", "CLI command output exceeded the limit", "output_limit", OperationFailed)
	}
	event.State = OperationSucceeded
	if result.ExitCode != 0 {
		event.State, event.ReasonCode = OperationFailed, "process_failed"
	}
	if err := broker.emitOperation(ctx, event); err != nil {
		return brokerFailure("event_unavailable", "Connector operation result could not be recorded")
	}
	if broker.redactor != nil {
		result.Stdout = broker.redactor.Bytes(result.Stdout)
		result.Stderr = broker.redactor.Bytes(result.Stderr)
	}
	return BrokerResponse{StdoutBase64: base64.StdEncoding.EncodeToString(result.Stdout), StderrBase64: base64.StdEncoding.EncodeToString(result.Stderr), ExitCode: result.ExitCode}
}

func (broker *Broker) registerSecrets(values [][]byte) {
	if broker.redactor != nil && len(values) > 0 {
		broker.redactor.AddPatterns(values...)
	}
}

func (broker *Broker) emitOperation(ctx context.Context, event OperationEvent) error {
	if broker.events != nil {
		if err := broker.events(ctx, event); err != nil {
			return err
		}
	}
	if broker.audit == nil {
		return nil
	}
	result := ""
	if event.State == OperationSucceeded || event.State == OperationFailed || event.State == OperationCancelled || event.State == OperationTimedOut || event.State == OperationOutcomeUnknown {
		result = string(event.State)
	}
	return broker.audit(ctx, AuditRecord{
		ContractVersion: 1, OperationID: event.OperationID, UserID: event.OwnerID, ConnectorID: event.ConnectorID, ManifestVersion: event.ManifestVersion,
		CapabilityID: event.CapabilityID, Permissions: append([]string(nil), event.Permissions...), ExecutionIdentity: event.Identity,
		AuthorizationID: event.AuthorizationID,
		Action:          "operation." + string(event.State), Reason: event.ReasonCode, Result: result,
		TargetSummary: truncateAuditValue(event.Target, 256), InputDigest: event.InputDigest, OccurredAt: broker.now().UTC(),
	})
}

func brokerInputDigest(command BrokerCommand) string {
	encoded, err := json.Marshal(struct {
		Input     map[string]any `json:"input,omitempty"`
		Arguments []string       `json:"arguments,omitempty"`
	}{Input: command.Input, Arguments: command.Arguments})
	if err != nil {
		encoded = []byte("invalid")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func truncateAuditValue(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
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

func approvalInputSummary(capability *Capability, input map[string]any, arguments []string) string {
	if len(capability.Input.Fields) == 0 {
		return redactArguments(capability, arguments)
	}
	parts := make([]string, 0, len(capability.Input.Fields))
	for _, field := range capability.Input.Fields {
		value, exists := input[field.Name]
		if !exists {
			continue
		}
		display := "[redacted]"
		if !field.Sensitive {
			encoded, err := json.Marshal(value)
			if err == nil {
				display = truncateAuditValue(string(encoded), 256)
			}
		}
		parts = append(parts, field.Name+"="+display)
	}
	return strings.Join(parts, ", ")
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
	if command.Kind != "" && command.Kind != BrokerCommandExecute {
		return errors.New("unsupported Connector command kind")
	}
	if strings.TrimSpace(command.ConnectorID) == "" || strings.TrimSpace(command.Capability) == "" || len(command.Arguments) > 256 {
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
	result.RecommendedSkills = append([]RecommendedSkill(nil), value.RecommendedSkills...)
	result.Capabilities = append([]Capability(nil), value.Capabilities...)
	for index := range result.Capabilities {
		result.Capabilities[index] = cloneCapability(value.Capabilities[index])
	}
	return result
}

func brokerFailure(code, message string) BrokerResponse {
	return BrokerResponse{ErrorCode: code, ErrorMessage: message}
}

func writeBrokerResponse(writer io.Writer, response BrokerResponse) error {
	return json.NewEncoder(writer).Encode(response)
}
