package messagechannel

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestWeChatReceiveOptionalSuccessCodes(t *testing.T) {
	for _, test := range []struct {
		name, codes string
		accepted    bool
	}{
		{"omitted", "", true},
		{"zero", `"ret":0,"errcode":0,`, true},
		{"ret_failure", `"ret":-4,`, false},
		{"session_expired", `"errcode":-14,`, false},
		{"string_code", `"ret":"0",`, false},
		{"null_code", `"errcode":null,`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cursor := &memoryCursor{value: "prior", afterSave: cancel}
			polls, received := 0, 0
			adapter := NewTransports(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				polls++
				if r.URL.Path != "/ilink/bot/getupdates" {
					t.Fatalf("unexpected endpoint: %s", r.URL.Path)
				}
				return jsonResponse(`{`+test.codes+`"get_updates_buf":"next","msgs":[{"message_id":1,"from_user_id":"alice","to_user_id":"bot","message_type":1,"message_state":2,"context_token":"protected","item_list":[{"type":1,"text_item":{"text":"verify example"}}]}]}`, http.StatusOK), nil
			}), TransportOptions{Cursor: cursor})["wechat"].StreamReceiver
			err := adapter.Connect(ctx, application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot"}}, application.ChannelCredentials{"bot_token": "token"}, func(_ context.Context, message domain.ChannelMessage) error {
				received++
				if message.MessageID != "1" || message.Reply["context_token"] != "protected" || message.Text != "verify example" {
					t.Fatal("lost incoming verification message or reply context")
				}
				return nil
			})
			if polls != 1 {
				t.Fatalf("poll count: %d", polls)
			}
			if test.accepted {
				if !errors.Is(err, context.Canceled) || received != 1 || cursor.saves != 1 || cursor.value != "next" {
					t.Fatalf("successful response rejected: received=%d saves=%d error=%v", received, cursor.saves, err)
				}
			} else if err == nil || received != 0 || cursor.saves != 0 {
				t.Fatalf("failed response accepted: received=%d saves=%d error=%v", received, cursor.saves, err)
			}
		})
	}
}

func TestWeChatReceiveIdleResponseWithoutSuccessCode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cursor := &memoryCursor{value: "prior", afterSave: cancel}
	adapter := NewTransports(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"msgs":[],"get_updates_buf":"next","sync_buf":"deprecated"}`, http.StatusOK), nil
	}), TransportOptions{Cursor: cursor})["wechat"].StreamReceiver
	err := adapter.Connect(ctx, application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot"}}, application.ChannelCredentials{"bot_token": "token"}, func(context.Context, domain.ChannelMessage) error {
		t.Fatal("idle poll created an incoming message")
		return nil
	})
	if !errors.Is(err, context.Canceled) || cursor.saves != 1 || cursor.value != "next" {
		t.Fatalf("idle success rejected: cursor=%+v error=%v", cursor, err)
	}
}

func TestWeChatSendOptionalSuccessCodes(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		sent       bool
	}{
		{"omitted", `{}`, 200, true},
		{"zero", `{"ret":0,"errcode":0}`, 200, true},
		{"ret_failure", `{"ret":-4}`, 200, false},
		{"session_expired", `{"errcode":-14}`, 200, false},
		{"string_code", `{"ret":"0"}`, 200, false},
		{"null_code", `{"ret":null}`, 200, false},
		{"null_response", `null`, 200, false},
		{"http_failure", `{}`, 401, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter := NewTransports(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/ilink/bot/sendmessage" {
					t.Fatalf("unexpected endpoint: %s", r.URL.Path)
				}
				return jsonResponse(test.body, test.status), nil
			}), TransportOptions{})["wechat"].Sender
			result := adapter.Send(context.Background(), application.ChannelStored{}, application.ChannelCredentials{"bot_token": "token"}, domain.ChannelMessage{SenderID: "alice", Reply: map[string]string{"context_token": "protected"}}, "answer", "delivery")
			if (result.State == "sent") != test.sent {
				t.Fatalf("unexpected send result: %+v", result)
			}
		})
	}
}
