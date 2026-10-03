package application

import "time"

type ChannelLimits struct {
	MaxPendingMessages         int
	MaxSenderMessagesPerMinute int
	MaxTextBytes               int
	MaxSendAttempts            int
	SendInterval               time.Duration
}

func (l ChannelLimits) Effective() ChannelLimits {
	if l.MaxPendingMessages == 0 {
		l.MaxPendingMessages = 1000
	}
	if l.MaxSenderMessagesPerMinute == 0 {
		l.MaxSenderMessagesPerMinute = 10
	}
	if l.MaxTextBytes == 0 {
		l.MaxTextBytes = 10_000
	}
	if l.MaxSendAttempts == 0 {
		l.MaxSendAttempts = 8
	}
	if l.SendInterval == 0 {
		l.SendInterval = 3 * time.Second
	}
	return l
}
