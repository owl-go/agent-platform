package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
)

func TestWeComAccountRejectionKeepsOnlySafeDiagnostics(t *testing.T) {
	socket := newFakeSocket()
	socket.onWrite = func(data []byte) {
		var request wecomFrame
		_ = json.Unmarshal(data, &request)
		response, _ := json.Marshal(map[string]any{"headers": map[string]string{"req_id": request.Headers.ID}, "errcode": 400001, "errmsg": "private-secret-provider-detail"})
		socket.incoming <- response
	}
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	_, err := a.Identify(context.Background(), application.ChannelCredentials{"bot_id": "bot", "bot_secret": "private-secret"}, "")
	var failure *application.ChannelAccountFailure
	if !errors.As(err, &failure) || failure.Code != "wecom_authentication_rejected" || failure.ProviderCode != 400001 || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("lost safe authentication diagnostics: %v", err)
	}
	select {
	case <-socket.closed:
	default:
		t.Fatal("rejected authentication left a socket open")
	}
}

func TestWeComAuthenticationWaitsForMatchingReceipt(t *testing.T) {
	socket := newFakeSocket()
	socket.onWrite = func(data []byte) {
		var request wecomFrame
		_ = json.Unmarshal(data, &request)
		socket.incoming <- []byte(`{"cmd":"aibot_event_callback","headers":{"req_id":"event"},"body":{"event":{"eventtype":"enter_chat"}}}`)
		socket.incoming <- []byte(`{"headers":{"req_id":"unrelated"},"errcode":400001}`)
		response, _ := json.Marshal(map[string]any{"headers": map[string]string{"req_id": request.Headers.ID}, "errcode": 0})
		socket.incoming <- response
	}
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	identity, err := a.Identify(ctx, application.ChannelCredentials{"bot_id": "bot", "bot_secret": "secret"}, "")
	if err != nil || identity.ID != "bot" {
		t.Fatalf("non-authentication frame rejected a valid receipt: %v", err)
	}
}

type wecomFailingSocket struct {
	*fakeSocket
	readError error
}

func (s *wecomFailingSocket) ReadMessage() (int, []byte, error) {
	if s.readError != nil {
		return 0, nil, s.readError
	}
	return s.fakeSocket.ReadMessage()
}

type wecomTimeout struct{}

func (wecomTimeout) Error() string   { return "private-network-detail" }
func (wecomTimeout) Timeout() bool   { return true }
func (wecomTimeout) Temporary() bool { return true }

func TestWeComAccountFailureClassifiesAndCloses(t *testing.T) {
	for _, test := range []struct {
		name, response, code string
		dialError, readError error
	}{
		{"network", "", "wecom_connection_failed", errors.New("private-network-detail"), nil},
		{"timeout", "", "wecom_authentication_timeout", nil, wecomTimeout{}},
		{"closed", "", "wecom_connection_failed", nil, errors.New("private-provider-detail")},
		{"malformed", `not-json-private-provider-detail`, "wecom_authentication_invalid", nil, nil},
		{"missing code", `{"headers":{"req_id":%q}}`, "wecom_authentication_invalid", nil, nil},
		{"invalid code", `{"headers":{"req_id":%q},"errcode":"0"}`, "wecom_authentication_invalid", nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			socket := &wecomFailingSocket{fakeSocket: newFakeSocket(), readError: test.readError}
			socket.onWrite = func(data []byte) {
				var request wecomFrame
				_ = json.Unmarshal(data, &request)
				response := test.response
				if strings.Contains(response, "%q") {
					key, _ := json.Marshal(request.Headers.ID)
					response = strings.ReplaceAll(response, "%q", string(key))
				}
				socket.incoming <- []byte(response)
			}
			a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, test.dialError }}
			_, err := a.Identify(context.Background(), application.ChannelCredentials{"bot_id": "bot", "bot_secret": "private-secret"}, "")
			var failure *application.ChannelAccountFailure
			if !errors.As(err, &failure) || failure.Code != test.code || failure.ProviderCode != 0 || strings.Contains(err.Error(), "private-") {
				t.Fatalf("unsafe failure classification: %v", err)
			}
			if test.dialError == nil {
				select {
				case <-socket.closed:
				default:
					t.Fatal("failed authentication leaked a socket")
				}
			}
		})
	}
}

func TestWeComAuthenticationCannotUseCallbackAsReceiptAndIsBounded(t *testing.T) {
	socket := newFakeSocket()
	socket.onWrite = func(data []byte) {
		var request wecomFrame
		_ = json.Unmarshal(data, &request)
		response, _ := json.Marshal(map[string]any{"cmd": "aibot_event_callback", "headers": map[string]string{"req_id": request.Headers.ID}, "errcode": 0})
		go func() {
			for i := 0; i < 32; i++ {
				select {
				case socket.incoming <- response:
				case <-socket.closed:
					return
				}
			}
		}()
	}
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := a.Identify(ctx, application.ChannelCredentials{"bot_id": "bot", "bot_secret": "secret"}, "")
	var failure *application.ChannelAccountFailure
	if !errors.As(err, &failure) || failure.Code != "wecom_authentication_invalid" || ctx.Err() != nil {
		t.Fatalf("authentication accepted a callback or exceeded frame bound: %v", err)
	}
}

func TestWeComAuthenticationCancellationClosesSocket(t *testing.T) {
	socket := newFakeSocket()
	ctx, cancel := context.WithCancel(context.Background())
	socket.onWrite = func([]byte) { cancel() }
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	_, err := a.Identify(ctx, application.ChannelCredentials{"bot_id": "bot", "bot_secret": "secret"}, "")
	if err == nil {
		t.Fatal("cancelled authentication accepted")
	}
	select {
	case <-socket.closed:
	default:
		t.Fatal("cancelled authentication leaked a socket")
	}
}
