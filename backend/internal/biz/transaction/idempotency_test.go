package transaction

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIdempotencyRequestAndResultValidation(t *testing.T) {
	now := time.Now().UTC()
	request := IdempotencyRequest{
		OrganizationID: "org", Key: "request-1", Operation: "runtime-image.register",
		RequestSHA256: strings.Repeat("a", 64), ExpiresAt: now.Add(time.Hour),
	}
	if err := request.Validate(now); err != nil {
		t.Fatal(err)
	}
	if err := (IdempotencyResult{Status: 201, Body: json.RawMessage(`{"id":"one"}`)}).Validate(); err != nil {
		t.Fatal(err)
	}
	request.RequestSHA256 = "bad"
	if err := request.Validate(now); err == nil {
		t.Fatal("invalid request hash accepted")
	}
	if err := (IdempotencyResult{Status: 0, Body: json.RawMessage(`nope`)}).Validate(); err == nil {
		t.Fatal("invalid result accepted")
	}
}
