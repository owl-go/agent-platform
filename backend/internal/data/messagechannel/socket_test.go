package messagechannel

import (
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"testing"
	"time"
)

type fakeSocket struct {
	incoming chan []byte
	outgoing chan []byte
	closed   chan struct{}
	once     sync.Once
	onWrite  func([]byte)
}

func newFakeSocket() *fakeSocket {
	return &fakeSocket{incoming: make(chan []byte, 8), outgoing: make(chan []byte, 8), closed: make(chan struct{})}
}
func (s *fakeSocket) ReadMessage() (int, []byte, error) {
	select {
	case d := <-s.incoming:
		return websocket.BinaryMessage, d, nil
	case <-s.closed:
		return 0, nil, errors.New("closed")
	}
}
func (s *fakeSocket) WriteMessage(_ int, d []byte) error {
	if s.onWrite != nil {
		s.onWrite(d)
	}
	s.outgoing <- append([]byte(nil), d...)
	return nil
}
func (s *fakeSocket) SetReadDeadline(time.Time) error  { return nil }
func (s *fakeSocket) SetWriteDeadline(time.Time) error { return nil }
func (s *fakeSocket) SetReadLimit(int64)               {}
func (s *fakeSocket) Close() error                     { s.once.Do(func() { close(s.closed) }); return nil }
func TestWeComAuthenticatesAndCorrelatesOriginalChatReceipt(t *testing.T) {
	socket := newFakeSocket()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	s := application.ChannelStored{Channel: domain.MessageChannel{ID: "channel", AccountID: "bot", ConfigVersion: 1}}
	received := make(chan struct{}, 1)
	socket.onWrite = func(data []byte) {
		var frame wecomFrame
		_ = json.Unmarshal(data, &frame)
		switch frame.Cmd {
		case "aibot_subscribe":
			var v map[string]string
			_ = json.Unmarshal(frame.Body, &v)
			if v["bot_id"] != "bot" || v["secret"] != "protected" {
				t.Error("bad WeCom handshake")
			}
			socket.incoming <- []byte(`{"headers":{"req_id":"` + frame.Headers.ID + `"},"errcode":0}`)
			socket.incoming <- []byte(`{"cmd":"aibot_msg_callback","body":{"msgid":"original","aibotid":"bot","chattype":"group","chatid":"group","msgtype":"text","from":{"userid":"alice"},"text":{"content":"question"}}}`)
		case "aibot_send_msg":
			var v map[string]any
			_ = json.Unmarshal(frame.Body, &v)
			if v["chatid"] != "group" {
				t.Error("changed WeCom reply target")
			}
			socket.incoming <- []byte(`{"headers":{"req_id":"unrelated"},"errcode":400}`)
			socket.incoming <- []byte(`{"headers":{"req_id":"` + frame.Headers.ID + `"},"errcode":0}`)
		}
	}
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	done := make(chan error, 1)
	go func() {
		done <- a.Connect(ctx, s, application.ChannelCredentials{"bot_id": "bot", "bot_secret": "protected"}, func(commit context.Context, m domain.ChannelMessage) error {
			if _, ok := commit.Deadline(); !ok {
				t.Error("unbounded commit")
			}
			if !m.Group || !m.Mentioned || m.MessageID != "original" {
				t.Error("lost native message")
			}
			received <- struct{}{}
			return nil
		})
	}()
	select {
	case <-received:
	case <-ctx.Done():
		t.Fatal("connection did not receive")
	}
	result := a.Send(ctx, s, nil, domain.ChannelMessage{ChatID: "group", SenderID: "alice", Group: true}, "answer", "delivery-key")
	if result.State != "sent" || result.MessageID != "delivery-key" {
		t.Fatal(result)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close socket")
	}
	if result := a.Send(context.Background(), s, nil, domain.ChannelMessage{}, "answer", "new-key"); result.State != "retry_wait" {
		t.Fatal("dead connection accepted send")
	}
}
func TestYuanbaoACKFollowsDurableInboxCommit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "failure"}[fail], func(t *testing.T) {
			socket := newFakeSocket()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			committed := false
			ack := false
			socket.onWrite = func(data []byte) {
				frame, err := decodeYuanbaoFrame(data)
				if err != nil {
					t.Error(err)
					return
				}
				if frame.cmd == "auth-bind" {
					fields, _ := readWire(frame.body)
					info, _ := readWire(fields.data(2))
					if info.text(1) != "bot" || info.text(3) != "token" {
						t.Error("Yuanbao auth mismatch")
					}
					socket.incoming <- encodeYuanbaoFrame(yuanbaoFrame{kind: 1, cmd: frame.cmd, id: frame.id, body: []byte{8, 0}})
					socket.incoming <- encodeYuanbaoFrame(yuanbaoFrame{kind: 2, cmd: "push", id: "push-id", ack: true, body: []byte(`{"callback_command":"C2C.CallbackAfterSendMsg","from_account":"alice","to_account":"bot","msg_id":"original","msg_time":123,"msg_body":[{"msg_type":"TIMTextElem","msg_content":{"text":"question"}}]}`)})
				}
				if frame.kind == 3 {
					if !committed {
						t.Error("ACK before commit")
					}
					ack = true
					cancel()
				}
			}
			a := &Yuanbao{HTTP: NewHTTP(roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(`{"code":0,"data":{"bot_id":"bot","token":"token","duration":3600}}`, 200), nil
			})), dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
			err := a.Connect(ctx, application.ChannelStored{Channel: domain.MessageChannel{ID: "channel", AccountID: "bot", ConfigVersion: 1}}, application.ChannelCredentials{"app_key": "key", "app_secret": "secret"}, func(commit context.Context, m domain.ChannelMessage) error {
				if _, ok := commit.Deadline(); !ok {
					t.Fatal("unbounded Inbox")
				}
				if m.MessageID != "original" || m.SenderID != "alice" {
					t.Fatal("Yuanbao identity")
				}
				if fail {
					return errors.New("database failure")
				}
				committed = true
				return nil
			})
			if err == nil || ack == fail || committed == fail {
				t.Fatalf("unsafe ACK: committed=%v ack=%v error=%v", committed, ack, err)
			}
		})
	}
}
func TestYuanbaoSendUsesStableNativeGroupReplyAndReceipt(t *testing.T) {
	socket := newFakeSocket()
	session := newSocketSession(context.Background(), socket)
	s := application.ChannelStored{Channel: domain.MessageChannel{ID: "channel", AccountID: "bot", ConfigVersion: 1}}
	a := &Yuanbao{}
	remove := a.bindings.add(s, session)
	defer remove()
	socket.onWrite = func(data []byte) {
		frame, _ := decodeYuanbaoFrame(data)
		fields, _ := readWire(frame.body)
		if frame.cmd != "send_group_message" || frame.module != "yuanbao_openclaw_proxy" || fields.text(2) != "group" || fields.text(4) != "alice" || fields.text(7) != "original" || frame.id != "stablekey" {
			t.Error("native group reply lost target or key")
		}
		session.respond("unrelated", encodeYuanbaoFrame(yuanbaoFrame{kind: 1, cmd: frame.cmd, id: "other", status: 400}))
		session.respond(frame.id, encodeYuanbaoFrame(yuanbaoFrame{kind: 1, cmd: frame.cmd, id: frame.id, body: []byte{8, 0}}))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := a.Send(ctx, s, nil, domain.ChannelMessage{Group: true, SenderID: "alice", ChatID: "group", MessageID: "original"}, "answer", "stable-key")
	if r.State != "sent" || r.MessageID != "stablekey" {
		t.Fatal(r)
	}
}
