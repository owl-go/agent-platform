package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestWeComPairingHandshakePrecedesNativeUserIDAndClosesOnRecognition(t *testing.T) {
	socket := newFakeSocket()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	s := application.ChannelStored{Channel: domain.MessageChannel{Provider: "wecom", AccountID: "bot", BindingID: "bot"}}
	health := make(chan string, 1)
	messages := make(chan domain.ChannelMessage, 1)
	done := make(chan error, 1)
	go func() {
		done <- a.ConnectWithHealth(ctx, s, application.ChannelCredentials{"bot_id": "bot", "bot_secret": "test-secret"}, func(commit context.Context, m domain.ChannelMessage) error {
			if _, ok := commit.Deadline(); !ok {
				t.Error("unbounded pairing callback")
			}
			messages <- m
			cancel()
			return nil
		}, func(_ context.Context, state string) error { health <- state; return nil })
	}()
	var auth wecomFrame
	select {
	case data := <-socket.outgoing:
		if json.Unmarshal(data, &auth) != nil || auth.Cmd != "aibot_subscribe" {
			t.Fatal("missing subscription")
		}
	case <-ctx.Done():
		t.Fatal("authentication did not start")
	}
	select {
	case <-health:
		t.Fatal("pairing became ready before authentication")
	default:
	}
	socket.incoming <- []byte(`{"headers":{"req_id":"` + auth.Headers.ID + `"},"errcode":0}`)
	select {
	case state := <-health:
		if state != "connected" {
			t.Fatal(state)
		}
	case <-ctx.Done():
		t.Fatal("authenticated pairing never became ready")
	}
	// A callback belonging to another bot cannot supply the paired sender.
	socket.incoming <- []byte(`{"cmd":"aibot_msg_callback","body":{"msgid":"foreign","aibotid":"other","chattype":"single","msgtype":"text","from":{"userid":"other-user"},"text":{"content":"pair code"}}}`)
	socket.incoming <- []byte(`{"cmd":"aibot_msg_callback","body":{"msgid":"paired","aibotid":"bot","chattype":"single","msgtype":"text","from":{"userid":"native-user"},"text":{"content":"pair code"}}}`)
	select {
	case m := <-messages:
		if m.SenderID != "native-user" || m.ChatID != "native-user" || m.MessageID != "paired" || m.Text != "pair code" || m.Group {
			t.Fatal("pairing lost authenticated native User ID", m)
		}
	case <-time.After(time.Second):
		t.Fatal("no pairing candidate")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recognition did not release receiver")
	}
	select {
	case <-socket.closed:
	default:
		t.Fatal("temporary socket remained open")
	}
	if a.bindings.get(s) != nil {
		t.Fatal("temporary pairing registered an outbound sender")
	}
	select {
	case <-socket.outgoing:
		t.Fatal("pairing sent a reply")
	default:
	}
	if _, ok := NewTransports(nil)["wecom"].StreamReceiver.(application.ChannelStreamHealthReceiver); !ok {
		t.Fatal("WeCom pairing receiver not registered")
	}
}

func TestWeComPairingAuthenticationAndHealthFailureCloseSocket(t *testing.T) {
	for _, rejected := range []bool{false, true} {
		t.Run(map[bool]string{false: "health-sink", true: "rejected-auth"}[rejected], func(t *testing.T) {
			socket := newFakeSocket()
			socket.onWrite = func(data []byte) {
				var auth wecomFrame
				_ = json.Unmarshal(data, &auth)
				code := 0
				if rejected {
					code = 400001
				}
				receipt, _ := json.Marshal(map[string]any{"headers": map[string]string{"req_id": auth.Headers.ID}, "errcode": code})
				socket.incoming <- receipt
			}
			a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			calls := 0
			sentinel := errors.New("health sink failed")
			err := a.ConnectWithHealth(ctx, application.ChannelStored{}, application.ChannelCredentials{"bot_id": "bot", "bot_secret": "test-secret"}, func(context.Context, domain.ChannelMessage) error {
				t.Fatal("failed receiver published candidate")
				return nil
			}, func(context.Context, string) error { calls++; return sentinel })
			if rejected && (err == nil || calls != 0) || !rejected && (!errors.Is(err, sentinel) || calls != 1) {
				t.Fatal("incorrect handshake/health failure", err, calls)
			}
			select {
			case <-socket.closed:
			default:
				t.Fatal("failed pairing left socket open")
			}
		})
	}
}
