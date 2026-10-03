package application

import (
	"context"
	"sync"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type channelConnection struct {
	version int64
	cancel  context.CancelFunc
	done    chan struct{}
	retryAt time.Time
}

// ChannelConnections supervises transport lifetimes separately from executions.
// The Repository's process ownership lock prevents duplicate Worker processes.
type ChannelConnections struct {
	channels *MessageChannels
	limit    int
	mu       sync.Mutex
	active   map[string]*channelConnection
}

func NewChannelConnections(channels *MessageChannels, limit int) *ChannelConnections {
	if limit == 0 {
		limit = 64
	}
	return &ChannelConnections{channels: channels, limit: limit, active: map[string]*channelConnection{}}
}
func (m *ChannelConnections) ProcessNext(ctx context.Context) (bool, error) {
	if !m.channels.enabled {
		return false, nil
	}
	items, err := m.channels.repository.ActiveMessageChannels(ctx)
	if err != nil {
		m.mu.Lock()
		for _, connection := range m.active {
			connection.cancel()
		}
		m.mu.Unlock()
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	live := map[string]bool{}
	for _, stored := range items {
		transport := m.channels.transports[stored.Channel.Provider]
		receiver := transport.StreamReceiver
		if transport.receiver() == nil || receiver == nil {
			continue
		}
		id := stored.Channel.ID
		live[id] = true
		if current := m.active[id]; current != nil {
			if current.version != stored.Channel.Version {
				current.cancel()
				select {
				case <-current.done:
					delete(m.active, id)
				default:
					continue
				}
			} else {
				select {
				case <-current.done:
					if time.Now().Before(current.retryAt) {
						continue
					}
					delete(m.active, id)
				default:
					continue
				}
			}
		}
		if len(m.active) >= m.limit {
			_ = m.channels.repository.SetChannelHealth(ctx, id, stored.Channel.ConfigVersion, "disconnected", "connection_capacity")
			continue
		}
		child, cancel := context.WithCancel(ctx)
		connection := &channelConnection{version: stored.Channel.Version, cancel: cancel, done: make(chan struct{}), retryAt: time.Now().Add(30 * time.Second)}
		m.active[id] = connection
		go func(stored ChannelStored, receiver ChannelStreamReceiver) {
			defer func() { connection.retryAt = time.Now().Add(30 * time.Second); close(connection.done) }()
			defer cancel()
			c, err := m.channels.credentials(stored)
			if err == nil {
				err = receiver.Configure(child, stored, c, "")
			}
			if err == nil {
				_ = m.channels.repository.SetChannelHealth(child, stored.Channel.ID, stored.Channel.ConfigVersion, "connecting", "")
				err = receiver.Connect(child, stored, c, func(receiveCtx context.Context, message domain.ChannelMessage) error {
					if child.Err() != nil {
						return child.Err()
					}
					if err := m.channels.Receive(receiveCtx, stored, message); err != nil {
						return err
					}
					return m.channels.repository.SetChannelHealth(receiveCtx, stored.Channel.ID, stored.Channel.ConfigVersion, "connected", "")
				})
			}
			if child.Err() == nil {
				_ = m.channels.repository.SetChannelHealth(child, stored.Channel.ID, stored.Channel.ConfigVersion, "disconnected", "provider_connection_failed")
			}
		}(stored, receiver)
	}
	for id, current := range m.active {
		if !live[id] {
			current.cancel()
			select {
			case <-current.done:
				delete(m.active, id)
			default:
			}
		}
	}
	return false, nil
}
