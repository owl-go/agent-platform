package domain

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type State string

const (
	StatePending    State = "pending"
	StateDelivering State = "delivering"
	StateDelivered  State = "delivered"
	StateFailed     State = "failed"
	StateCancelled  State = "cancelled"
)

type Delivery struct {
	ID             string
	OrganizationID string
	EventType      string
	Payload        json.RawMessage
	TargetURL      string
	State          State
	AttemptCount   int
	NextAttemptAt  time.Time
	LockedUntil    *time.Time
	CreatedAt      time.Time
}

func (delivery Delivery) Validate() error {
	if strings.TrimSpace(delivery.ID) == "" || strings.TrimSpace(delivery.OrganizationID) == "" {
		return fmt.Errorf("Webhook Delivery identity is required")
	}
	if strings.TrimSpace(delivery.EventType) == "" || !json.Valid(delivery.Payload) {
		return fmt.Errorf("Webhook Delivery event type and JSON payload are required")
	}
	target, err := url.ParseRequestURI(delivery.TargetURL)
	if err != nil || target.Scheme != "https" || target.Host == "" || target.User != nil {
		return fmt.Errorf("Webhook Delivery target must be an HTTPS URL without user info")
	}
	if delivery.AttemptCount < 0 {
		return fmt.Errorf("Webhook Delivery attempt count cannot be negative")
	}
	return nil
}

func RetryDelay(attempt int, base, maximum time.Duration) time.Duration {
	if attempt <= 1 {
		return base
	}
	delay := base
	for index := 1; index < attempt; index++ {
		if delay >= maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}
