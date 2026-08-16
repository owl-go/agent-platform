package domain

import (
	"fmt"
	"time"
)

type Policy struct {
	BatchSize        int
	RunEventPeriod   time.Duration
	ArtifactPeriod   time.Duration
	WorkspacePeriod  time.Duration
	AuditPeriod      time.Duration
	IdempotencyGrace time.Duration
}

func (policy Policy) Validate() error {
	if policy.BatchSize <= 0 || policy.BatchSize > 10_000 {
		return fmt.Errorf("Retention batch size must be between 1 and 10000")
	}
	if policy.RunEventPeriod <= 0 || policy.ArtifactPeriod <= 0 || policy.WorkspacePeriod <= 0 || policy.AuditPeriod <= 0 || policy.IdempotencyGrace < 0 {
		return fmt.Errorf("Retention periods must be positive and Idempotency grace cannot be negative")
	}
	if policy.AuditPeriod < policy.RunEventPeriod {
		return fmt.Errorf("Audit retention cannot be shorter than Run Event retention")
	}
	return nil
}

type Artifact struct {
	ID        string
	ObjectKey string
}

type Workspace struct {
	SessionID string
	Volume    string
}

type Result struct {
	Artifacts      int64
	RunEvents      int64
	AuditEvents    int64
	IdempotencyKey int64
	Workspaces     int64
}
