package messagechannel

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestWeChatTypingUsesIncomingParticipantAndEphemeralTicket(t *testing.T) {
	var states []int
	configCalls := 0
	transport := NewTransports(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer bot-secret" {
			t.Fatal("typing lost account authorization")
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if rawString(body["ilink_user_id"]) != "participant" {
			t.Fatal("typing targeted QR scanner instead of incoming participant")
		}
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			configCalls++
			if rawString(body["context_token"]) != "incoming-context" {
				t.Fatal("typing lost reply context")
			}
			return jsonResponse(`{"ret":0,"typing_ticket":"ephemeral-ticket"}`, 200), nil
		case "/ilink/bot/sendtyping":
			if rawString(body["typing_ticket"]) != "ephemeral-ticket" {
				t.Fatal("wrong typing ticket")
			}
			states = append(states, int(rawNumber(body["status"])))
			return jsonResponse(`{}`, 200), nil
		default:
			t.Fatalf("typing invoked another endpoint: %s", r.URL.Path)
		}
		return nil, nil
	}))["wechat"]
	if transport.Typing == nil {
		t.Fatal("WeChat typing transport not registered")
	}
	credentials := application.ChannelCredentials{"bot_token": "bot-secret", "user_id": "qr-scanner"}
	message := domain.ChannelMessage{SenderID: "participant", Reply: map[string]string{"context_token": "incoming-context"}}
	session, err := transport.Typing.BeginTyping(context.Background(), application.ChannelStored{}, credentials, message)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func(context.Context) error{session.Refresh, session.Refresh, session.Stop} {
		if err = operation(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if configCalls != 1 || !reflect.DeepEqual(states, []int{1, 1, 2}) {
		t.Fatalf("typing lifecycle: config=%d states=%v", configCalls, states)
	}
	if credentials["typing_ticket"] != "" || message.Reply["typing_ticket"] != "" {
		t.Fatal("ticket escaped transient session")
	}
}

func TestWeChatTypingRejectsUnavailableOrMalformedTicket(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"provider_rejection", `{"ret":-4,"typing_ticket":"ticket"}`, 200},
		{"missing_ticket", `{"ret":0}`, 200},
		{"malformed_code", `{"ret":"0","typing_ticket":"ticket"}`, 200},
		{"malformed_ticket", `{"ret":0,"typing_ticket":1}`, 200},
		{"http_rejection", `{"typing_ticket":"ticket"}`, 401},
	} {
		t.Run(test.name, func(t *testing.T) {
			typing := NewTransports(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/ilink/bot/getconfig" {
					t.Fatal("unavailable ticket emitted typing")
				}
				return jsonResponse(test.body, test.status), nil
			}))["wechat"].Typing
			session, err := typing.BeginTyping(context.Background(), application.ChannelStored{}, application.ChannelCredentials{"bot_token": "secret"}, domain.ChannelMessage{SenderID: "participant", Reply: map[string]string{"context_token": "context"}})
			if err == nil || session != nil {
				t.Fatal("invalid ticket accepted")
			}
		})
	}
}
func TestWeChatTypingRejectsMissingReplyOrGroupWithoutRequest(t *testing.T) {
	typing := NewTransports(roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid typing issued network request")
		return nil, nil
	}))["wechat"].Typing
	for _, message := range []domain.ChannelMessage{{SenderID: "user"}, {Group: true, SenderID: "user", Reply: map[string]string{"context_token": "token"}}} {
		if _, err := typing.BeginTyping(context.Background(), application.ChannelStored{}, nil, message); err == nil {
			t.Fatal("invalid audience accepted")
		}
	}
}
