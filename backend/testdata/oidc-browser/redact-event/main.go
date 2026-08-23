package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/credentials"
)

type outputSink struct{}

func (outputSink) Publish(_ context.Context, event agentruntime.Event) error {
	_, err := os.Stdout.Write(event.Payload)
	return err
}

func main() {
	secret := os.Getenv("RUN_EVENT_SECRET_CANARY")
	if secret == "" {
		panic("RUN_EVENT_SECRET_CANARY is required")
	}
	payload, err := json.Marshal(map[string]string{
		"diff":       "safe change",
		"diagnostic": "credential=" + secret,
		"large_log":  strings.Repeat("x", 20_000),
	})
	if err != nil {
		panic("encode event fixture")
	}
	sink := agentruntime.NewRedactingEventSink(credentials.NewRedactor([]byte(secret)), outputSink{})
	if err := sink.Publish(context.Background(), agentruntime.Event{Payload: payload}); err != nil {
		panic(fmt.Sprintf("redact event fixture: %v", err))
	}
}
