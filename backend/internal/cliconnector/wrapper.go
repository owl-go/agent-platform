package cliconnector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

type State string

const (
	StateDraft     State = "draft"
	StateBuilding  State = "building"
	StateTesting   State = "testing"
	StateAvailable State = "available"
	StateFailed    State = "failed"
	StateDisabled  State = "disabled"
)

type Risk string

const (
	RiskLow  Risk = "low"
	RiskHigh Risk = "high"
)

type Identity string

const (
	IdentityUser Identity = "user"
	IdentityBot  Identity = "bot"
)

type Capability struct {
	ID          string        `json:"id"`
	ArgvPrefix  []string      `json:"argv_prefix"`
	Risk        Risk          `json:"risk"`
	Identities  []Identity    `json:"identities"`
	Scopes      []string      `json:"scopes"`
	EgressHosts []string      `json:"egress_hosts"`
	Timeout     time.Duration `json:"timeout"`
}

type Definition struct {
	ID                   string
	Name                 string
	Package              string
	Version              string
	Integrity            string
	Executable           string
	AuthenticationDriver string
	State                State
	BundleSHA256         string
	RuntimeDigests       []string
	Capabilities         []Capability
	VersionNumber        int64
	FailureReason        string
	CreatedByUserID      string
}

type Enablement struct {
	ID, OwnerID, DefinitionID string
	State                     string
	ActionURL                 string
	ActionExpiresAt           *time.Time
	Version                   int64
}

var exactVersion = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
var npmPackage = regexp.MustCompile(`^(?:@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)
var policyToken = regexp.MustCompile(`^[A-Za-z0-9._:/-]+$`)
var egressHost = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

func (definition Definition) Validate() error {
	if strings.TrimSpace(definition.Name) == "" || !npmPackage.MatchString(definition.Package) || !exactVersion.MatchString(definition.Version) {
		return errors.New("CLI Connector requires a name, valid npm package, and exact version")
	}
	if definition.Executable == "" || strings.ContainsAny(definition.Executable, `/\\`) {
		return errors.New("CLI executable must be selected from package bin metadata")
	}
	if definition.AuthenticationDriver != "feishu" && definition.AuthenticationDriver != "none" {
		return errors.New("unsupported built-in authentication driver")
	}
	if definition.Integrity == "" {
		return errors.New("npm integrity is required")
	}
	if len(definition.Capabilities) == 0 {
		return errors.New("at least one reviewed CLI capability is required")
	}
	seen := map[string]struct{}{}
	for _, capability := range definition.Capabilities {
		if capability.ID == "" || len(capability.ArgvPrefix) == 0 || capability.Timeout <= 0 || capability.Timeout > 15*time.Minute {
			return errors.New("invalid CLI capability")
		}
		if _, ok := seen[capability.ID]; ok {
			return errors.New("duplicate CLI capability")
		}
		seen[capability.ID] = struct{}{}
		for _, arg := range capability.ArgvPrefix {
			if arg == "" || strings.ContainsAny(arg, "\x00\r\n") {
				return errors.New("unsafe CLI argument pattern")
			}
		}
		if capability.Risk != RiskLow && capability.Risk != RiskHigh {
			return errors.New("unsupported CLI capability risk")
		}
		if len(capability.Identities) == 0 {
			return errors.New("CLI capability requires an execution identity")
		}
		for _, identity := range capability.Identities {
			if identity != IdentityUser && identity != IdentityBot {
				return errors.New("unsupported CLI execution identity")
			}
		}
		for _, scope := range capability.Scopes {
			if !policyToken.MatchString(scope) {
				return errors.New("unsafe CLI scope")
			}
		}
		if len(capability.EgressHosts) == 0 {
			return errors.New("CLI capability requires explicit Egress hosts")
		}
		for _, host := range capability.EgressHosts {
			if !egressHost.MatchString(host) || strings.Contains(host, "..") {
				return errors.New("unsafe CLI Egress host")
			}
		}
	}
	return nil
}

type Request struct {
	CapabilityID, RuntimeDigest, BundleSHA256 string
	Target                                    string
	Identity                                  Identity
	Argv                                      []string
	Environment                               map[string]string
	ApprovalNonce                             string
	ApprovalExpiresAt                         time.Time
}
type Result struct {
	Stdout, Stderr []byte
	ExitCode       int
}
type Process interface {
	Run(context.Context, string, []string, map[string]string) (Result, error)
}
type Wrapper struct {
	Process         Process
	ConsumeApproval func(context.Context, string, string) error
	Revalidate      func(context.Context, Definition, Request) error
	Now             func() time.Time
}

func (wrapper Wrapper) Execute(ctx context.Context, definition Definition, request Request) (Result, error) {
	if wrapper.Process == nil {
		return Result{}, errors.New("CLI process port is required")
	}
	if definition.State != StateAvailable || definition.BundleSHA256 == "" || request.BundleSHA256 != definition.BundleSHA256 || !slices.Contains(definition.RuntimeDigests, request.RuntimeDigest) {
		return Result{}, errors.New("CLI bundle and Runtime combination is unavailable")
	}
	var capability *Capability
	for index := range definition.Capabilities {
		if definition.Capabilities[index].ID == request.CapabilityID {
			capability = &definition.Capabilities[index]
			break
		}
	}
	if capability == nil || !slices.Contains(capability.Identities, request.Identity) || !hasPrefix(request.Argv, capability.ArgvPrefix) {
		return Result{}, errors.New("CLI command is outside the reviewed capability")
	}
	if wrapper.Revalidate != nil {
		if err := wrapper.Revalidate(ctx, definition, request); err != nil {
			return Result{}, fmt.Errorf("revalidate CLI command: %w", err)
		}
	}
	if capability.Risk == RiskHigh {
		now := time.Now().UTC()
		if wrapper.Now != nil {
			now = wrapper.Now().UTC()
		}
		if wrapper.ConsumeApproval == nil || request.ApprovalNonce == "" || strings.TrimSpace(request.Target) == "" || request.ApprovalExpiresAt.IsZero() {
			return Result{}, errors.New("high-risk CLI command requires one-use approval")
		}
		if !now.Before(request.ApprovalExpiresAt) {
			return Result{}, errors.New("CLI command approval expired")
		}
		if err := wrapper.ConsumeApproval(ctx, CommandDigest(definition, request), request.ApprovalNonce); err != nil {
			return Result{}, fmt.Errorf("consume CLI approval: %w", err)
		}
	}
	timeout := capability.Timeout
	if timeout > 15*time.Minute {
		timeout = 15 * time.Minute
	}
	execution, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return wrapper.Process.Run(execution, definition.Executable, append([]string(nil), request.Argv...), cloneEnvironment(request.Environment))
}

func CommandDigest(definition Definition, request Request) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{definition.ID, fmt.Sprint(definition.VersionNumber), definition.Executable, strings.Join(request.Argv, "\x00"), request.Target, request.CapabilityID, string(request.Identity), request.BundleSHA256, request.RuntimeDigest, request.ApprovalExpiresAt.UTC().Format(time.RFC3339Nano)}, "\x1f")))
	return hex.EncodeToString(sum[:])
}
func hasPrefix(value, prefix []string) bool {
	return len(value) >= len(prefix) && slices.Equal(value[:len(prefix)], prefix)
}
func cloneEnvironment(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
