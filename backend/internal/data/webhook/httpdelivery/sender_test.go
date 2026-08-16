package httpdelivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/webhook/domain"
)

type clientStub struct {
	request *http.Request
	status  int
}

func (client *clientStub) Do(request *http.Request) (*http.Response, error) {
	client.request = request
	return &http.Response{StatusCode: client.status, Body: io.NopCloser(strings.NewReader("ok"))}, nil
}

func TestSenderSignsExactPayloadAndMetadata(t *testing.T) {
	client := &clientStub{status: http.StatusNoContent}
	secret := []byte("0123456789abcdef0123456789abcdef")
	sender, err := New(client, secret)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_786_838_400, 0).UTC()
	sender.now = func() time.Time { return now }
	payload := json.RawMessage(`{"run_id":"run-1","state":"completed"}`)
	delivery := domain.Delivery{
		ID: "delivery-1", OrganizationID: "organization-1", EventType: "run.completed",
		Payload: payload, TargetURL: "https://hooks.example.test/agent-platform",
	}
	if err := sender.Deliver(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(client.request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(payload) {
		t.Fatalf("body = %q, want exact payload %q", body, payload)
	}
	timestamp := client.request.Header.Get("X-Agent-Platform-Timestamp")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp + "." + string(payload)))
	wantSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := client.request.Header.Get("X-Agent-Platform-Signature"); got != wantSignature {
		t.Fatalf("signature = %q, want %q", got, wantSignature)
	}
	if client.request.Header.Get("X-Agent-Platform-Delivery") != delivery.ID || client.request.Header.Get("X-Agent-Platform-Event") != delivery.EventType {
		t.Fatal("Webhook metadata headers are missing")
	}
}

func TestSenderRejectsNonSuccessStatusWithoutReflectingBody(t *testing.T) {
	client := &clientStub{status: http.StatusBadGateway}
	sender, err := New(client, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	err = sender.Deliver(context.Background(), domain.Delivery{
		ID: "delivery", OrganizationID: "organization", EventType: "run.failed",
		Payload: json.RawMessage(`{"secret":"must-not-be-reflected"}`), TargetURL: "https://hooks.example.test",
	})
	if err == nil || strings.Contains(err.Error(), "must-not-be-reflected") {
		t.Fatalf("unexpected delivery error: %v", err)
	}
}
