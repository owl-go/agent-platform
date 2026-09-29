package domain

import "time"

// SessionWorkflowLink records provenance only. The Workflow and Session keep
// independent histories after conversion.
type SessionWorkflowLink struct {
	SessionID       string
	MessageID       int64
	WorkflowID      string
	WorkflowName    string
	ValidationRunID string
	CreatedAt       time.Time
}

type SessionWorkflowCreation struct {
	Workflow Workflow
	Run      Run
	Link     SessionWorkflowLink
	Replayed bool
}
