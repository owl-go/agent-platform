package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrRuntimeImageNotFound = errors.New("Runtime Image not found")
	ErrConcurrentUpdate     = errors.New("Runtime Image was modified concurrently")
	ErrImageDigestExists    = errors.New("Runtime Image digest is already registered")
	ErrInvalidRuntimeImage  = errors.New("invalid Runtime Image input")
	imageDigestPattern      = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
)

type Runtime string

const (
	Claude   Runtime = "claude"
	Codex    Runtime = "codex"
	Hermes   Runtime = "hermes"
	OpenClaw Runtime = "openclaw"
)

type Status string

const (
	Experimental Status = "experimental"
	Production   Status = "production"
	Blocked      Status = "blocked"
	Deprecated   Status = "deprecated"
)

var knownCapabilities = map[string]struct{}{
	"native_resume": {}, "streaming": {}, "structured_final": {}, "subagents": {}, "usage": {},
}

type RuntimeImage struct {
	ID             string
	Runtime        Runtime
	CLIVersion     string
	AdapterVersion string
	ImageDigest    string
	Capabilities   map[string]bool
	Status         Status
	BlockedReason  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

type Registration struct {
	ID             string
	Runtime        Runtime
	CLIVersion     string
	AdapterVersion string
	ImageDigest    string
	Capabilities   map[string]bool
	Now            time.Time
}

func Register(registration Registration) (RuntimeImage, error) {
	if strings.TrimSpace(registration.ID) == "" || strings.TrimSpace(registration.CLIVersion) == "" || strings.TrimSpace(registration.AdapterVersion) == "" {
		return RuntimeImage{}, invalidf("Runtime Image ID, CLI version, and Adapter version are required")
	}
	if err := validateRuntime(registration.Runtime); err != nil {
		return RuntimeImage{}, err
	}
	if !imageDigestPattern.MatchString(registration.ImageDigest) {
		return RuntimeImage{}, invalidf("Runtime Image must use an immutable sha256 repository digest")
	}
	capabilities, err := cloneCapabilities(registration.Capabilities)
	if err != nil {
		return RuntimeImage{}, err
	}
	now := registration.Now.UTC()
	if now.IsZero() {
		return RuntimeImage{}, invalidf("registration time is required")
	}
	return RuntimeImage{
		ID: registration.ID, Runtime: registration.Runtime, CLIVersion: registration.CLIVersion,
		AdapterVersion: registration.AdapterVersion, ImageDigest: registration.ImageDigest,
		Capabilities: capabilities, Status: Experimental, CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func Restore(id, runtimeValue, cliVersion, adapterVersion, imageDigest string, capabilities map[string]bool, statusValue, blockedReason string, createdAt, updatedAt time.Time, version int64) (RuntimeImage, error) {
	runtime := Runtime(runtimeValue)
	if err := validateRuntime(runtime); err != nil {
		return RuntimeImage{}, err
	}
	status, err := ParseStatus(statusValue)
	if err != nil {
		return RuntimeImage{}, err
	}
	cloned, err := cloneCapabilities(capabilities)
	if err != nil {
		return RuntimeImage{}, err
	}
	if id == "" || cliVersion == "" || adapterVersion == "" || !imageDigestPattern.MatchString(imageDigest) || version <= 0 || createdAt.IsZero() || updatedAt.IsZero() {
		return RuntimeImage{}, invalidf("invalid persisted Runtime Image")
	}
	if status == Blocked && strings.TrimSpace(blockedReason) == "" || status != Blocked && blockedReason != "" {
		return RuntimeImage{}, invalidf("invalid persisted Runtime Image block reason")
	}
	return RuntimeImage{
		ID: id, Runtime: runtime, CLIVersion: cliVersion, AdapterVersion: adapterVersion,
		ImageDigest: imageDigest, Capabilities: cloned, Status: status, BlockedReason: blockedReason,
		CreatedAt: createdAt, UpdatedAt: updatedAt, Version: version,
	}, nil
}

func (image *RuntimeImage) ChangeStatus(status Status, reason string, now time.Time) error {
	if _, err := ParseStatus(string(status)); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if status == Blocked && reason == "" {
		return invalidf("blocked Runtime Image requires a reason")
	}
	if status != Blocked && reason != "" {
		return invalidf("block reason is only valid for blocked Runtime Images")
	}
	if image.Status == Deprecated && status != Deprecated {
		return invalidf("deprecated Runtime Image cannot be reactivated")
	}
	if image.Status == status && image.BlockedReason == reason {
		return nil
	}
	if now.IsZero() {
		return invalidf("status change time is required")
	}
	image.Status = status
	image.BlockedReason = reason
	image.UpdatedAt = now.UTC()
	image.Version++
	return nil
}

func ParseStatus(value string) (Status, error) {
	status := Status(value)
	switch status {
	case Experimental, Production, Blocked, Deprecated:
		return status, nil
	default:
		return "", invalidf("unknown Runtime Image status %q", value)
	}
}

func validateRuntime(runtime Runtime) error {
	switch runtime {
	case Claude, Codex, Hermes, OpenClaw:
		return nil
	default:
		return invalidf("unknown Agent Runtime %q", runtime)
	}
}

func cloneCapabilities(capabilities map[string]bool) (map[string]bool, error) {
	cloned := make(map[string]bool, len(capabilities))
	for capability, enabled := range capabilities {
		if _, known := knownCapabilities[capability]; !known {
			return nil, invalidf("unknown Runtime Capability %q", capability)
		}
		cloned[capability] = enabled
	}
	return cloned, nil
}

func invalidf(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRuntimeImage, fmt.Sprintf(format, arguments...))
}
