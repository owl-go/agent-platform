package application

import (
	"context"
	"encoding/json"
)

// RuntimeApprovalGate is the cross-context seam used when a paused Runtime
// needs a human decision before it may continue a risky operation.
type RuntimeApprovalGate interface {
	AwaitDecision(context.Context, RuntimeApprovalRequest) error
}

type RuntimeApprovalRequest struct {
	RunID     string
	AttemptID string
	Sequence  int64
	Kind      string
	Request   json.RawMessage
}
