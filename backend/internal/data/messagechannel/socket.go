package messagechannel

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"github.com/gorilla/websocket"
)

type channelSocket interface {
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	SetReadLimit(int64)
	Close() error
}
type channelSocketDialer func(context.Context, string, http.Header) (channelSocket, error)

func dialChannelSocket(ctx context.Context, target string, headers http.Header) (channelSocket, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "wss" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Host != "openws.work.weixin.qq.com" && u.Host != "bot-wss.yuanbao.tencent.com" && u.Host != "api.sgroup.qq.com") {
		return nil, providerError("provider_endpoint_invalid")
	}
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second, NetDialContext: dialPublicProvider}
	connection, response, err := dialer.DialContext(ctx, target, headers)
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
	if err != nil {
		return nil, providerError("provider_connection_failed")
	}
	connection.SetReadLimit(65536)
	return connection, nil
}

type socketSession struct {
	socket    channelSocket
	ctx       context.Context
	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan []byte
}

func newSocketSession(ctx context.Context, socket channelSocket) *socketSession {
	return &socketSession{socket: socket, ctx: ctx, pending: map[string]chan []byte{}}
}
func (s *socketSession) write(kind int, data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	if err := s.socket.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return providerError("provider_connection_failed")
	}
	if err := s.socket.WriteMessage(kind, data); err != nil {
		return providerError("provider_connection_failed")
	}
	return nil
}
func (s *socketSession) request(ctx context.Context, key string, kind int, data []byte) ([]byte, error) {
	response := make(chan []byte, 1)
	s.pendingMu.Lock()
	if _, exists := s.pending[key]; exists {
		s.pendingMu.Unlock()
		return nil, providerError("provider_request_in_flight")
	}
	s.pending[key] = response
	s.pendingMu.Unlock()
	defer func() { s.pendingMu.Lock(); delete(s.pending, key); s.pendingMu.Unlock() }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.write(kind, data); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case data := <-response:
		return data, nil
	}
}
func (s *socketSession) respond(key string, data []byte) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	if response := s.pending[key]; response != nil {
		select {
		case response <- append([]byte(nil), data...):
		default:
		}
	}
}
func (s *socketSession) heartbeat(kind int, interval time.Duration, payload func() []byte) {
	timer := time.NewTicker(interval)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
			if s.write(kind, payload()) != nil {
				s.socket.Close()
				return
			}
		}
	}
}

type socketBindings struct {
	mu     sync.Mutex
	active map[string]socketBinding
}
type socketBinding struct {
	version int64
	session *socketSession
}

func (b *socketBindings) add(s application.ChannelStored, session *socketSession) func() {
	b.mu.Lock()
	if b.active == nil {
		b.active = map[string]socketBinding{}
	}
	b.active[s.Channel.ID] = socketBinding{version: s.Channel.ConfigVersion, session: session}
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.active[s.Channel.ID].session == session {
			delete(b.active, s.Channel.ID)
		}
	}
}
func (b *socketBindings) get(s application.ChannelStored) *socketSession {
	b.mu.Lock()
	defer b.mu.Unlock()
	binding := b.active[s.Channel.ID]
	if binding.version != s.Channel.ConfigVersion || binding.session == nil || binding.session.ctx.Err() != nil {
		return nil
	}
	return binding.session
}
