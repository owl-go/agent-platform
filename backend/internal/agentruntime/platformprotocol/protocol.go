package platformprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const Prefix = "agent-platform-event: "

var (
	commitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	branchPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)
)

type Event struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type WorkflowDelivery struct {
	ReviewBranch string   `json:"review_branch"`
	Commit       string   `json:"commit"`
	ChangedFiles []string `json:"changed_files"`
}

func EncodeApprovalRequest(riskReason, reviewBranch string) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"kind": "high_risk_change",
		"request": map[string]string{
			"risk_reason":   riskReason,
			"review_branch": reviewBranch,
		},
	})
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(Event{Kind: "approval.requested", Payload: payload})
	if err != nil {
		return nil, err
	}
	return append([]byte(Prefix), encoded...), nil
}

func EncodeWorkflowDelivered(reviewBranch, commit string, changedFiles []string) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{"review_branch": reviewBranch, "commit": commit, "changed_files": changedFiles})
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(Event{Kind: "workflow.delivered", Payload: payload})
	if err != nil {
		return nil, err
	}
	return append([]byte(Prefix), encoded...), nil
}

func Parse(line []byte) (Event, bool, error) {
	if !bytes.HasPrefix(line, []byte(Prefix)) {
		return Event{}, false, nil
	}
	var event Event
	if err := json.Unmarshal(bytes.TrimPrefix(line, []byte(Prefix)), &event); err != nil {
		return Event{}, true, fmt.Errorf("decode Agent Platform Runtime event: %w", err)
	}
	if (event.Kind != "approval.requested" && event.Kind != "workflow.delivered") || len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return Event{}, true, fmt.Errorf("unsupported Agent Platform Runtime event")
	}
	if event.Kind == "workflow.delivered" {
		if _, err := DecodeWorkflowDelivery(event.Payload); err != nil {
			return Event{}, true, err
		}
	}
	return event, true, nil
}

func DecodeWorkflowDelivery(value []byte) (WorkflowDelivery, error) {
	var payload WorkflowDelivery
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || !branchPattern.MatchString(payload.ReviewBranch) || strings.Contains(payload.ReviewBranch, "..") || !commitPattern.MatchString(payload.Commit) || len(payload.ChangedFiles) > 10_000 {
		return WorkflowDelivery{}, fmt.Errorf("invalid workflow delivery event")
	}
	for _, name := range payload.ChangedFiles {
		if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "../") || strings.ContainsRune(name, '\x00') {
			return WorkflowDelivery{}, fmt.Errorf("invalid workflow delivery path")
		}
	}
	return payload, nil
}
