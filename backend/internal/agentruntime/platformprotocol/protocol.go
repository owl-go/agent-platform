package platformprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const Prefix = "agent-platform-event: "

type Event struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
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

func Parse(line []byte) (Event, bool, error) {
	if !bytes.HasPrefix(line, []byte(Prefix)) {
		return Event{}, false, nil
	}
	var event Event
	if err := json.Unmarshal(bytes.TrimPrefix(line, []byte(Prefix)), &event); err != nil {
		return Event{}, true, fmt.Errorf("decode Agent Platform Runtime event: %w", err)
	}
	if event.Kind != "approval.requested" || len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return Event{}, true, fmt.Errorf("unsupported Agent Platform Runtime event")
	}
	return event, true, nil
}
