package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrRuntimeUnavailable = errors.New("sandbox runtime unavailable")
	ErrIsolationDrift     = errors.New("sandbox isolation policy drift")
	imageDigestPattern    = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
	identifierPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
)

type EgressMode string

const (
	EgressNone   EgressMode = "none"
	EgressPublic EgressMode = "public"
)

type Limits struct {
	CPUs        float64
	MemoryBytes int64
	PIDs        int64
	TempBytes   int64
}

type CreateSpec struct {
	RunID                string
	Image                string
	Command              []string
	WorkspaceVolume      string
	CredentialDirectory  string
	NonSecretEnvironment []string
	Egress               EgressMode
	Limits               Limits
}

type Container struct {
	ID    string
	RunID string
}

type Inspection struct {
	ID             string
	RunID          string
	State          string
	Runtime        string
	User           string
	NetworkMode    string
	ReadOnlyRootfs bool
	Managed        bool
	Egress         EgressMode
	CreatedAt      time.Time
	CapDrop        []string
	SecurityOpt    []string
	MemoryBytes    int64
	NanoCPUs       int64
	PIDs           int64
	Tmpfs          map[string]string
	Mounts         []Mount
}

type Mount struct {
	Type        string
	Source      string
	Destination string
	ReadWrite   bool
}

type DockerConfig struct {
	Runtime             string
	PublicEgressNetwork string
	ResolverConfigFile  string
	CredentialRoot      string
	UID                 int
	GID                 int
}

type Runner interface {
	Create(ctx context.Context, spec CreateSpec) (Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string, gracePeriod time.Duration) error
	Destroy(ctx context.Context, id string) error
	Inspect(ctx context.Context, id string) (Inspection, error)
	Reconcile(ctx context.Context, activeRunIDs map[string]struct{}, createdBefore time.Time) ([]string, error)
}

type Executor interface {
	Run(ctx context.Context, args ...string) (string, error)
}

type CommandError struct {
	ExitCode int
	Stderr   string
	Cause    error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("docker command exited with code %d: %s", e.ExitCode, strings.TrimSpace(e.Stderr))
}

func (e *CommandError) Unwrap() error {
	return e.Cause
}

type DockerRunner struct {
	executor Executor
	config   DockerConfig
}

var _ Runner = (*DockerRunner)(nil)

func NewDockerRunner(executor Executor, config DockerConfig) *DockerRunner {
	return &DockerRunner{executor: executor, config: config}
}

func (r *DockerRunner) Create(ctx context.Context, spec CreateSpec) (Container, error) {
	if err := r.validateCreateSpec(spec); err != nil {
		return Container{}, err
	}
	if err := r.requireRuntime(ctx); err != nil {
		return Container{}, err
	}
	network := "none"
	if spec.Egress == EgressPublic {
		network = r.config.PublicEgressNetwork
	}
	args := []string{
		"create",
		"--runtime", r.config.Runtime,
		"--user", fmt.Sprintf("%d:%d", r.config.UID, r.config.GID),
		"--read-only",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--network", network,
		"--memory", strconv.FormatInt(spec.Limits.MemoryBytes, 10),
		"--pids-limit", strconv.FormatInt(spec.Limits.PIDs, 10),
		"--cpus", strconv.FormatFloat(spec.Limits.CPUs, 'f', -1, 64),
		"--mount", "type=volume,src=" + spec.WorkspaceVolume + ",dst=/workspace,readonly=false",
		"--mount", "type=bind,src=" + spec.CredentialDirectory + ",dst=/run/agent-credentials,readonly=true",
		"--label", "agent-platform.managed=true",
		"--label", "agent-platform.run-id=" + spec.RunID,
		"--label", "agent-platform.egress=" + string(spec.Egress),
		"--init",
	}
	if spec.Egress == EgressPublic {
		args = append(args, "--mount", "type=bind,src="+r.config.ResolverConfigFile+",dst=/etc/resolv.conf,readonly=true")
	}
	args = append(args, "--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size="+strconv.FormatInt(spec.Limits.TempBytes, 10))
	for _, variable := range spec.NonSecretEnvironment {
		args = append(args, "--env", variable)
	}
	args = append(args, spec.Image)
	args = append(args, spec.Command...)
	output, err := r.executor.Run(ctx, args...)
	if err != nil {
		return Container{}, fmt.Errorf("create sandbox container: %w", err)
	}
	id := strings.TrimSpace(output)
	if id == "" {
		return Container{}, fmt.Errorf("create sandbox container: Docker returned an empty container ID")
	}
	return Container{ID: id, RunID: spec.RunID}, nil
}

func (r *DockerRunner) Start(ctx context.Context, id string) error {
	if err := validateContainerID(id); err != nil {
		return err
	}
	inspection, err := r.Inspect(ctx, id)
	if err != nil {
		return err
	}
	if err := r.validateInspection(inspection); err != nil {
		return err
	}
	if _, err := r.executor.Run(ctx, "start", id); err != nil {
		return fmt.Errorf("start sandbox container: %w", err)
	}
	return nil
}

func (r *DockerRunner) Inspect(ctx context.Context, id string) (Inspection, error) {
	if err := validateContainerID(id); err != nil {
		return Inspection{}, err
	}
	output, err := r.executor.Run(ctx, "inspect", id)
	if err != nil {
		return Inspection{}, fmt.Errorf("inspect sandbox container: %w", err)
	}
	var values []struct {
		ID      string `json:"Id"`
		Created string `json:"Created"`
		Config  struct {
			User   string            `json:"User"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		HostConfig struct {
			Runtime        string            `json:"Runtime"`
			ReadonlyRootfs bool              `json:"ReadonlyRootfs"`
			NetworkMode    string            `json:"NetworkMode"`
			CapDrop        []string          `json:"CapDrop"`
			SecurityOpt    []string          `json:"SecurityOpt"`
			Memory         int64             `json:"Memory"`
			NanoCPUs       int64             `json:"NanoCpus"`
			PidsLimit      int64             `json:"PidsLimit"`
			Tmpfs          map[string]string `json:"Tmpfs"`
		} `json:"HostConfig"`
		State struct {
			Status string `json:"Status"`
		} `json:"State"`
		Mounts []struct {
			Type        string `json:"Type"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			RW          bool   `json:"RW"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal([]byte(output), &values); err != nil {
		return Inspection{}, fmt.Errorf("decode sandbox inspection: %w", err)
	}
	if len(values) != 1 {
		return Inspection{}, fmt.Errorf("decode sandbox inspection: got %d records, want 1", len(values))
	}
	createdAt, err := time.Parse(time.RFC3339Nano, values[0].Created)
	if err != nil {
		return Inspection{}, fmt.Errorf("decode sandbox creation time: %w", err)
	}
	inspection := Inspection{
		ID:             values[0].ID,
		RunID:          values[0].Config.Labels["agent-platform.run-id"],
		State:          values[0].State.Status,
		Runtime:        values[0].HostConfig.Runtime,
		User:           values[0].Config.User,
		NetworkMode:    values[0].HostConfig.NetworkMode,
		ReadOnlyRootfs: values[0].HostConfig.ReadonlyRootfs,
		Managed:        values[0].Config.Labels["agent-platform.managed"] == "true",
		Egress:         EgressMode(values[0].Config.Labels["agent-platform.egress"]),
		CreatedAt:      createdAt,
		CapDrop:        values[0].HostConfig.CapDrop,
		SecurityOpt:    values[0].HostConfig.SecurityOpt,
		MemoryBytes:    values[0].HostConfig.Memory,
		NanoCPUs:       values[0].HostConfig.NanoCPUs,
		PIDs:           values[0].HostConfig.PidsLimit,
		Tmpfs:          values[0].HostConfig.Tmpfs,
	}
	for _, mount := range values[0].Mounts {
		inspection.Mounts = append(inspection.Mounts, Mount{
			Type: mount.Type, Source: mount.Source, Destination: mount.Destination, ReadWrite: mount.RW,
		})
	}
	return inspection, nil
}

func (r *DockerRunner) Reconcile(ctx context.Context, activeRunIDs map[string]struct{}, createdBefore time.Time) ([]string, error) {
	if createdBefore.IsZero() {
		return nil, fmt.Errorf("sandbox reconcile cutoff is required")
	}
	output, err := r.executor.Run(ctx, "ps", "--all", "--filter", "label=agent-platform.managed=true", "--format", "{{.ID}}")
	if err != nil {
		return nil, fmt.Errorf("list managed sandbox containers: %w", err)
	}
	destroyed := make([]string, 0)
	for _, id := range strings.Fields(output) {
		inspection, err := r.Inspect(ctx, id)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return destroyed, err
		}
		if !inspection.Managed || !inspection.CreatedAt.Before(createdBefore) {
			continue
		}
		if _, active := activeRunIDs[inspection.RunID]; active {
			continue
		}
		if err := r.Destroy(ctx, id); err != nil {
			return destroyed, err
		}
		destroyed = append(destroyed, id)
	}
	return destroyed, nil
}

func (r *DockerRunner) Stop(ctx context.Context, id string, gracePeriod time.Duration) error {
	if err := validateContainerID(id); err != nil {
		return err
	}
	if gracePeriod <= 0 {
		gracePeriod = 5 * time.Second
	}
	seconds := int64(math.Ceil(gracePeriod.Seconds()))
	if _, err := r.executor.Run(ctx, "stop", "--time", strconv.FormatInt(seconds, 10), id); err != nil && !isNotFound(err) {
		return fmt.Errorf("stop sandbox container: %w", err)
	}
	return nil
}

func (r *DockerRunner) Destroy(ctx context.Context, id string) error {
	if err := validateContainerID(id); err != nil {
		return err
	}
	if _, err := r.executor.Run(ctx, "rm", "--force", "--volumes", id); err != nil && !isNotFound(err) {
		return fmt.Errorf("destroy sandbox container: %w", err)
	}
	return nil
}

func (r *DockerRunner) requireRuntime(ctx context.Context) error {
	output, err := r.executor.Run(ctx, "info", "--format", "{{json .Runtimes}}")
	if err != nil {
		return fmt.Errorf("%w: inspect Docker runtimes: %v", ErrRuntimeUnavailable, err)
	}
	runtimes := make(map[string]json.RawMessage)
	if err := json.Unmarshal([]byte(output), &runtimes); err != nil {
		return fmt.Errorf("%w: decode Docker runtimes: %v", ErrRuntimeUnavailable, err)
	}
	if _, ok := runtimes[r.config.Runtime]; !ok {
		return fmt.Errorf("%w: %s", ErrRuntimeUnavailable, r.config.Runtime)
	}
	return nil
}

func (r *DockerRunner) validateCreateSpec(spec CreateSpec) error {
	if r.executor == nil {
		return fmt.Errorf("Docker executor is required")
	}
	if r.config.Runtime == "" || r.config.Runtime != "runsc" {
		return fmt.Errorf("sandbox runtime must be runsc")
	}
	if r.config.UID <= 0 || r.config.GID <= 0 {
		return fmt.Errorf("sandbox UID and GID must be non-root")
	}
	if !identifierPattern.MatchString(spec.RunID) {
		return fmt.Errorf("invalid run ID %q", spec.RunID)
	}
	if !imageDigestPattern.MatchString(spec.Image) {
		return fmt.Errorf("runtime image must use an immutable sha256 digest")
	}
	if len(spec.Command) == 0 {
		return fmt.Errorf("sandbox command is required")
	}
	if !identifierPattern.MatchString(spec.WorkspaceVolume) {
		return fmt.Errorf("invalid workspace volume %q", spec.WorkspaceVolume)
	}
	if err := withinRoot(r.config.CredentialRoot, spec.CredentialDirectory); err != nil {
		return err
	}
	if spec.Limits.CPUs <= 0 || spec.Limits.MemoryBytes <= 0 || spec.Limits.PIDs <= 0 || spec.Limits.TempBytes <= 0 {
		return fmt.Errorf("CPU, memory, PID, and temporary disk limits must be positive")
	}
	switch spec.Egress {
	case EgressNone:
	case EgressPublic:
		if r.config.PublicEgressNetwork == "" {
			return fmt.Errorf("public egress network is not configured")
		}
		if !filepath.IsAbs(r.config.ResolverConfigFile) || strings.Contains(r.config.ResolverConfigFile, ",") {
			return fmt.Errorf("public egress resolver config must be an absolute path without commas")
		}
	default:
		return fmt.Errorf("unsupported egress policy %q", spec.Egress)
	}
	for _, variable := range spec.NonSecretEnvironment {
		name, _, found := strings.Cut(variable, "=")
		if strings.ContainsRune(variable, '\x00') || !found || !validEnvironmentName(name) {
			return fmt.Errorf("invalid sandbox environment variable")
		}
	}
	return nil
}

func (r *DockerRunner) validateInspection(inspection Inspection) error {
	expectedUser := fmt.Sprintf("%d:%d", r.config.UID, r.config.GID)
	expectedNetwork := "none"
	if inspection.Egress == EgressPublic {
		expectedNetwork = r.config.PublicEgressNetwork
	}
	validNetwork := (inspection.Egress == EgressNone || inspection.Egress == EgressPublic) && inspection.NetworkMode == expectedNetwork
	baseIsolation := inspection.Managed && inspection.Runtime == r.config.Runtime && inspection.ReadOnlyRootfs && inspection.User == expectedUser && validNetwork
	resourceIsolation := inspection.MemoryBytes > 0 && inspection.NanoCPUs > 0 && inspection.PIDs > 0
	securityIsolation := containsFold(inspection.CapDrop, "ALL") && containsPrefixFold(inspection.SecurityOpt, "no-new-privileges")
	tmpIsolation := containsAll(inspection.Tmpfs["/tmp"], "noexec", "nosuid", "nodev", "size=")
	if !baseIsolation || !resourceIsolation || !securityIsolation || !tmpIsolation || !r.validMounts(inspection.Mounts, inspection.Egress) {
		return fmt.Errorf("%w: runtime=%q readonly=%v user=%q network=%q managed=%v", ErrIsolationDrift, inspection.Runtime, inspection.ReadOnlyRootfs, inspection.User, inspection.NetworkMode, inspection.Managed)
	}
	return nil
}

func validEnvironmentName(name string) bool {
	for index, value := range name {
		if value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || index > 0 && value >= '0' && value <= '9' {
			continue
		}
		return false
	}
	return name != ""
}

func (r *DockerRunner) validMounts(mounts []Mount, egress EgressMode) bool {
	workspace := false
	credentials := false
	resolver := egress == EgressNone
	for _, mount := range mounts {
		switch mount.Destination {
		case "/workspace":
			workspace = mount.Type == "volume" && mount.ReadWrite
		case "/run/agent-credentials":
			credentials = mount.Type == "bind" && !mount.ReadWrite && withinRoot(r.config.CredentialRoot, mount.Source) == nil
		case "/etc/resolv.conf":
			resolver = egress == EgressPublic && mount.Type == "bind" && !mount.ReadWrite && filepath.Clean(mount.Source) == filepath.Clean(r.config.ResolverConfigFile)
		default:
			return false
		}
	}
	expectedMounts := 2
	if egress == EgressPublic {
		expectedMounts = 3
	}
	return workspace && credentials && resolver && len(mounts) == expectedMounts
}

func containsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(value, expected) {
			return true
		}
	}
	return false
}

func containsPrefixFold(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}

func withinRoot(root, candidate string) error {
	if root == "" || !filepath.IsAbs(root) || !filepath.IsAbs(candidate) {
		return fmt.Errorf("credential root and directory must be absolute")
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("credential directory must be a child of the configured root")
	}
	return nil
}

func validateContainerID(id string) error {
	if !identifierPattern.MatchString(id) {
		return fmt.Errorf("invalid container ID %q", id)
	}
	return nil
}

func isNotFound(err error) bool {
	var commandErr *CommandError
	return errors.As(err, &commandErr) && strings.Contains(strings.ToLower(commandErr.Stderr), "no such container")
}
