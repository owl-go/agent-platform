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

type qqGatewayRepository struct {
	application.MessageChannelRepository
	stored application.ChannelStored
	inbox  chan domain.ChannelMessage
}

func (r *qqGatewayRepository) ActiveMessageChannels(context.Context) ([]application.ChannelStored, error) {
	return []application.ChannelStored{r.stored}, nil
}
func (r *qqGatewayRepository) SetChannelHealth(context.Context, string, int64, string, string) error {
	return nil
}
func (r *qqGatewayRepository) ReceiveChannelMessage(_ context.Context, _ application.ChannelStored, m domain.ChannelMessage, _ []byte) (bool, error) {
	r.inbox <- m
	return true, nil
}

type qqGatewayCipher struct{}

func (qqGatewayCipher) Encrypt(b []byte, _ string) ([]byte, error) { return b, nil }
func (qqGatewayCipher) Decrypt(b []byte, _ string) ([]byte, error) { return b, nil }

// Exercise the production registry and connection supervisor, not a separately
// constructed adapter: a QR-bound QQ account must actually authenticate online.
func TestQQGatewayRegisteredConnectionReachesInbox(t *testing.T) {
	socket := newFakeSocket()
	socket.incoming <- []byte(`{"op":10,"d":{"heartbeat_interval":30000}}`)
	socket.onWrite = func(b []byte) {
		var f struct {
			Op   int `json:"op"`
			Data struct {
				Token   string `json:"token"`
				Intents int    `json:"intents"`
				Shard   []int  `json:"shard"`
			} `json:"d"`
		}
		if json.Unmarshal(b, &f) != nil {
			t.Error("invalid gateway write")
			return
		}
		if f.Op != 2 {
			return
		}
		if f.Data.Token != "QQBot protected-token" || f.Data.Intents != 1<<25 || len(f.Data.Shard) != 2 || f.Data.Shard[0] != 0 || f.Data.Shard[1] != 1 {
			t.Error("wrong native QQ identification")
		}
		socket.incoming <- []byte(`{"op":0,"t":"READY","s":1,"d":{"session_id":"session","user":{"id":"bot"}}}`)
		message, _ := json.Marshal(map[string]any{"op": 0, "t": "C2C_MESSAGE_CREATE", "s": 2, "d": map[string]any{"id": "original", "content": "question", "timestamp": time.Now().UTC().Format(time.RFC3339), "author": map[string]string{"user_openid": "alice"}}})
		socket.incoming <- message
	}
	registry := NewTransports(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/app/getAppAccessToken":
			return jsonResponse(`{"access_token":"protected-token"}`, 200), nil
		case "/users/@me":
			return jsonResponse(`{"id":"bot","username":"QQ bot"}`, 200), nil
		case "/gateway":
			return jsonResponse(`{"url":"wss://api.sgroup.qq.com/websocket"}`, 200), nil
		default:
			t.Errorf("unexpected provider endpoint: %s", r.URL.Path)
			return jsonResponse(`{}`, 404), nil
		}
	}), TransportOptions{dialSocket: func(_ context.Context, target string, _ http.Header) (channelSocket, error) {
		if target != "wss://api.sgroup.qq.com/websocket" {
			t.Error("wrong gateway")
		}
		return socket, nil
	}})
	credentials, _ := json.Marshal(application.ChannelCredentials{"app_id": "app", "app_secret": "protected-secret"})
	repo := &qqGatewayRepository{stored: application.ChannelStored{Channel: domain.MessageChannel{ID: "channel", Provider: "qqbot", AccountID: "bot", BindingID: "app", Enabled: true, Version: 1, ConfigVersion: 1, Audience: domain.ChannelAudience{AllowDirect: true, SenderIDs: []string{"alice"}}}, Ciphertext: credentials}, inbox: make(chan domain.ChannelMessage, 1)}
	app := application.NewMessageChannels(repo, qqGatewayCipher{}, registry, true, "")
	connections := application.NewChannelConnections(app, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := connections.ProcessNext(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-repo.inbox:
		if m.MessageID != "original" || m.SenderID != "alice" || m.ChatID != "alice" || m.Text != "question" {
			t.Fatal("lost QQ native message scope")
		}
	case <-time.After(time.Second):
		t.Fatal("QR-bound QQ bot never authenticated online or received its message")
	}
	cancel()
	select {
	case <-socket.closed:
	case <-time.After(time.Second):
		t.Fatal("QQ socket survived cancellation")
	}
}

func TestQQGatewayReadyHeartbeatAndMissedACK(t *testing.T) {
	socket := newFakeSocket()
	socket.incoming <- []byte(`{"op":10,"d":{"heartbeat_interval":100}}`)
	socket.onWrite = func(b []byte) {
		var f qqGatewayFrame
		_ = json.Unmarshal(b, &f)
		if f.Op == 2 {
			socket.incoming <- []byte(`{"op":0,"t":"READY","s":1,"d":{"session_id":"native-session","user":{"id":"bot"}}}`)
		}
	}
	a := qqGatewayTestAdapter(socket)
	resume := &qqGatewayResume{}
	resume.sequence.Store(-1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	health := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- a.gatewayConnection(ctx, application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot"}}, application.ChannelCredentials{"app_id": "app", "app_secret": "secret"}, func(context.Context, domain.ChannelMessage) error {
			t.Error("READY created an Inbox message")
			return nil
		}, func(_ context.Context, state string) error { health <- state; return nil }, resume)
	}()
	select {
	case state := <-health:
		if state != "connected" {
			t.Fatal("READY did not establish health")
		}
	case <-ctx.Done():
		t.Fatal("missing READY")
	}
	// Identify is first; heartbeat is next and must include the committed READY seq.
	<-socket.outgoing
	select {
	case b := <-socket.outgoing:
		var f qqGatewayFrame
		_ = json.Unmarshal(b, &f)
		if f.Op != 1 || string(f.Data) != "1" {
			t.Fatal("heartbeat lost committed sequence")
		}
	case <-ctx.Done():
		t.Fatal("no heartbeat")
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("missing ACK left QQ online indefinitely")
	}
}

func qqGatewayTestAdapter(socket channelSocket) *QQBot {
	return &QQBot{HTTP: NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/app/getAppAccessToken" {
			return jsonResponse(`{"access_token":"token"}`, 200), nil
		}
		return jsonResponse(`{"url":"wss://api.sgroup.qq.com/websocket"}`, 200), nil
	})), dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
}

func TestQQGatewayResumeAndInboxCommitBoundary(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "failure"}[fail], func(t *testing.T) {
			socket := newFakeSocket()
			socket.incoming <- []byte(`{"op":10,"d":{"heartbeat_interval":30000}}`)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			resume := &qqGatewayResume{session: "old-session"}
			resume.sequence.Store(4)
			socket.onWrite = func(b []byte) {
				var f qqGatewayFrame
				_ = json.Unmarshal(b, &f)
				var d struct {
					Session  string `json:"session_id"`
					Sequence int    `json:"seq"`
					Token    string `json:"token"`
				}
				_ = json.Unmarshal(f.Data, &d)
				if f.Op != 6 || d.Session != "old-session" || d.Sequence != 4 || d.Token != "QQBot token" {
					t.Error("resume lost session/commit cursor")
				}
				socket.incoming <- []byte(`{"op":0,"t":"RESUMED","s":5,"d":{}}`)
				socket.incoming <- []byte(`{"op":0,"t":"GROUP_AT_MESSAGE_CREATE","s":6,"d":{"id":"msg","group_openid":"group","content":"<@!bot> question","timestamp":"2026-10-04T00:00:00Z","author":{"member_openid":"alice"}}}`)
			}
			err := qqGatewayTestAdapter(socket).gatewayConnection(ctx, application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot"}}, application.ChannelCredentials{"app_id": "app", "app_secret": "secret"}, func(commit context.Context, m domain.ChannelMessage) error {
				if _, ok := commit.Deadline(); !ok || !m.Group || !m.Mentioned || m.Text != "question" || m.ChatID != "group" || m.SenderID != "alice" {
					t.Error("lost bounded native group scope")
				}
				if fail {
					return errors.New("storage unavailable")
				}
				cancel()
				return nil
			}, func(context.Context, string) error { return nil }, resume)
			if fail && (!errors.Is(err, errQQInbox) || resume.sequence.Load() != 5) {
				t.Fatal("uncommitted Inbox advanced Resume")
			}
			if !fail && resume.sequence.Load() != 6 {
				t.Fatal("committed Inbox lost Resume cursor")
			}
		})
	}
}

func TestQQGatewayRejectsInvalidFramesAndEndpoints(t *testing.T) {
	for _, target := range []string{"ws://api.sgroup.qq.com/websocket", "wss://api.sgroup.qq.com.evil.test/websocket", "wss://api.sgroup.qq.com:443/websocket", "wss://api.sgroup.qq.com/websocket?token=secret", "wss://user@api.sgroup.qq.com/websocket", "wss://api.sgroup.qq.com/other", "wss://127.0.0.1/websocket"} {
		if qqGatewayURL(target) {
			t.Fatal("unsafe gateway accepted")
		}
	}
	for name, frame := range map[string]string{"bad_json": "{", "bad_interval": `{"op":10,"d":{"heartbeat_interval":0}}`, "before_hello": `{"op":0,"t":"READY","s":1,"d":{}}`} {
		t.Run(name, func(t *testing.T) {
			socket := newFakeSocket()
			socket.incoming <- []byte(frame)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r := &qqGatewayResume{}
			r.sequence.Store(-1)
			err := qqGatewayTestAdapter(socket).gatewayConnection(ctx, application.ChannelStored{}, application.ChannelCredentials{"app_id": "app", "app_secret": "secret"}, func(context.Context, domain.ChannelMessage) error { t.Error("invalid frame reached Inbox"); return nil }, func(context.Context, string) error { t.Error("invalid handshake marked online"); return nil }, r)
			if err == nil || errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("invalid frame not rejected")
			}
		})
	}
}
