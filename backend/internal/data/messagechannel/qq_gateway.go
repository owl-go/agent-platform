package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"github.com/gorilla/websocket"
)

type qqGatewayFrame struct {
	Op       int             `json:"op"`
	Type     string          `json:"t"`
	Sequence *int64          `json:"s"`
	Data     json.RawMessage `json:"d"`
}
type qqGatewayResume struct {
	session  string
	sequence atomic.Int64
}

func (a *QQBot) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink) error {
	return a.ConnectWithHealth(ctx, s, c, sink, func(context.Context, string) error { return nil })
}

// Resume lives only within this supervised connection: removal, disable and
// configuration replacement discard it. Sequence advances after Inbox commit.
func (a *QQBot) ConnectWithHealth(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink, health application.ChannelConnectionHealthSink) error {
	resume := &qqGatewayResume{}
	resume.sequence.Store(-1)
	for attempt := 0; attempt < 6; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := health(ctx, "connecting"); err != nil {
			return err
		}
		err := a.gatewayConnection(ctx, s, c, sink, health, resume)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var closeErr *websocket.CloseError
		if errors.As(err, &closeErr) {
			switch closeErr.Code {
			case 4004, 4013, 4014, 4914, 4915:
				return providerError("provider_authentication_failed")
			case 4007, 4009:
				resume.session = ""
				resume.sequence.Store(-1)
			}
		}
		// Inbox/health failures must return immediately, preserving committed state
		// and allowing the supervisor to expose a bounded safe failure.
		if errors.Is(err, errQQInbox) {
			return providerError("channel_inbox_unavailable")
		}
		timer := time.NewTimer(time.Second * time.Duration(1<<attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return providerError("provider_connection_failed")
}

var errQQInbox = errors.New("qq_inbox_unavailable")

func qqGatewayURL(target string) bool {
	u, err := url.Parse(target)
	return err == nil && u.Scheme == "wss" && u.Host == "api.sgroup.qq.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Path == "/websocket" || u.Path == "/websocket/")
}

func (a *QQBot) gatewayConnection(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink, health application.ChannelConnectionHealthSink, resume *qqGatewayResume) error {
	setup, cancelSetup := context.WithTimeout(ctx, 20*time.Second)
	defer cancelSetup()
	token, err := a.token(setup, c)
	if err != nil {
		return err
	}
	result, status, _, err := a.request(setup, http.MethodGet, "https://api.sgroup.qq.com/gateway", "QQBot "+token, nil)
	target := rawString(result["url"])
	if err != nil || status != http.StatusOK || !qqGatewayURL(target) {
		return providerError("provider_endpoint_invalid")
	}
	socket, err := a.dial(setup, target, http.Header{"User-Agent": []string{"AgentWorkspace/1.0"}})
	if err != nil {
		return err
	}
	defer socket.Close()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(child, func() { _ = socket.Close() })
	defer stop()
	socket.SetReadLimit(65536)
	session := newSocketSession(child, socket)
	write := func(op int, data any) error {
		b, err := json.Marshal(map[string]any{"op": op, "d": data})
		if err != nil {
			return providerError("provider_payload_invalid")
		}
		return session.write(websocket.TextMessage, b)
	}
	// HELLO + READY/RESUMED must complete within a fixed budget even if the
	// provider keeps sending unrelated frames. Cancellation closes blocked reads.
	handshake := time.AfterFunc(20*time.Second, func() { _ = socket.Close() })
	defer handshake.Stop()
	var waiting atomic.Bool
	hello, ready := false, false
	for child.Err() == nil {
		_, data, err := socket.ReadMessage()
		if err != nil {
			return err
		}
		var f qqGatewayFrame
		if len(data) > 65536 || json.Unmarshal(data, &f) != nil {
			return providerError("provider_payload_invalid")
		}
		switch f.Op {
		case 10:
			var d struct {
				Interval int64 `json:"heartbeat_interval"`
			}
			if hello || json.Unmarshal(f.Data, &d) != nil || d.Interval < 100 || d.Interval > 300000 {
				return providerError("provider_payload_invalid")
			}
			hello = true
			if resume.session != "" && resume.sequence.Load() >= 0 {
				err = write(6, map[string]any{"token": "QQBot " + token, "session_id": resume.session, "seq": resume.sequence.Load()})
			} else {
				err = write(2, map[string]any{"token": "QQBot " + token, "intents": 1 << 25, "shard": []int{0, 1}, "properties": map[string]string{"$os": "linux", "$browser": "agent-workspace", "$device": "agent-workspace"}})
			}
			if err != nil {
				return err
			}
			go func(interval time.Duration) {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				for {
					select {
					case <-child.Done():
						return
					case <-ticker.C:
						if waiting.Swap(true) {
							_ = socket.Close()
							return
						}
						var seq any
						if n := resume.sequence.Load(); n >= 0 {
							seq = n
						}
						if write(1, seq) != nil {
							_ = socket.Close()
							return
						}
					}
				}
			}(time.Duration(d.Interval) * time.Millisecond * 4 / 5)
		case 11:
			waiting.Store(false)
		case 1:
			var seq any
			if n := resume.sequence.Load(); n >= 0 {
				seq = n
			}
			if err := write(1, seq); err != nil {
				return err
			}
		case 7:
			return providerError("provider_reconnect_requested")
		case 9:
			var resumable bool
			if json.Unmarshal(f.Data, &resumable) != nil {
				return providerError("provider_payload_invalid")
			}
			if !resumable {
				resume.session = ""
				resume.sequence.Store(-1)
			}
			return providerError("provider_session_invalid")
		case 0:
			if !hello || f.Sequence == nil || *f.Sequence < 0 {
				return providerError("provider_payload_invalid")
			}
			if f.Type == "READY" {
				var d struct {
					Session string `json:"session_id"`
					User    struct {
						ID string `json:"id"`
					} `json:"user"`
				}
				if ready || json.Unmarshal(f.Data, &d) != nil || d.Session == "" || len(d.Session) > 512 || d.User.ID != s.Channel.AccountID {
					return providerError("provider_identity_changed")
				}
				resume.session = d.Session
				ready = true
			} else if f.Type == "RESUMED" {
				if ready || resume.session == "" {
					return providerError("provider_payload_invalid")
				}
				ready = true
			} else {
				if !ready {
					return providerError("provider_payload_invalid")
				}
				messages, err := normalizeQQ(s, f.Type, f.Data)
				if err != nil {
					return err
				}
				for _, m := range messages {
					commit, cancelCommit := context.WithTimeout(child, 10*time.Second)
					err = sink(commit, m)
					cancelCommit()
					if err != nil {
						return errQQInbox
					}
				}
			}
			if *f.Sequence > resume.sequence.Load() {
				resume.sequence.Store(*f.Sequence)
			}
			if f.Type == "READY" || f.Type == "RESUMED" {
				handshake.Stop()
				if err := health(child, "connected"); err != nil {
					return errQQInbox
				}
			}
		}
	}
	return child.Err()
}
