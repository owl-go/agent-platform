package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/webhook/domain"
)

type Repository interface {
	Claim(context.Context, time.Time, time.Duration) (domain.Delivery, bool, error)
	MarkDelivered(context.Context, string, time.Time) error
	MarkFailed(context.Context, string, string, time.Time, bool) error
}

type Deliverer interface {
	Deliver(context.Context, domain.Delivery) error
}

type Config struct {
	LeaseDuration time.Duration
	RetryBase     time.Duration
	RetryMaximum  time.Duration
	MaxAttempts   int
}

type Dispatcher struct {
	repository Repository
	deliverer  Deliverer
	config     Config
	now        func() time.Time
}

func NewDispatcher(repository Repository, deliverer Deliverer, config Config) (*Dispatcher, error) {
	if repository == nil || deliverer == nil {
		return nil, fmt.Errorf("Webhook Repository and Deliverer are required")
	}
	if config.LeaseDuration <= 0 || config.RetryBase <= 0 || config.RetryMaximum < config.RetryBase || config.MaxAttempts <= 0 {
		return nil, fmt.Errorf("Webhook Dispatcher configuration is invalid")
	}
	return &Dispatcher{repository: repository, deliverer: deliverer, config: config, now: time.Now}, nil
}

func (dispatcher *Dispatcher) ProcessNext(ctx context.Context) (bool, error) {
	now := dispatcher.now().UTC()
	delivery, found, err := dispatcher.repository.Claim(ctx, now, dispatcher.config.LeaseDuration)
	if err != nil || !found {
		return found, err
	}
	if err := delivery.Validate(); err != nil {
		markErr := dispatcher.repository.MarkFailed(context.WithoutCancel(ctx), delivery.ID, errorSummary(err), now, true)
		if markErr != nil {
			return true, fmt.Errorf("cancel invalid Webhook Delivery after %v: %w", err, markErr)
		}
		return true, nil
	}
	if err := dispatcher.deliverer.Deliver(ctx, delivery); err != nil {
		cancel := delivery.AttemptCount >= dispatcher.config.MaxAttempts
		next := now.Add(domain.RetryDelay(delivery.AttemptCount, dispatcher.config.RetryBase, dispatcher.config.RetryMaximum))
		if markErr := dispatcher.repository.MarkFailed(context.WithoutCancel(ctx), delivery.ID, errorSummary(err), next, cancel); markErr != nil {
			return true, fmt.Errorf("record Webhook Delivery failure after %v: %w", err, markErr)
		}
		return true, nil
	}
	if err := dispatcher.repository.MarkDelivered(ctx, delivery.ID, now); err != nil {
		return true, fmt.Errorf("mark Webhook Delivery delivered: %w", err)
	}
	return true, nil
}

func errorSummary(err error) string {
	value := strings.TrimSpace(err.Error())
	if len(value) > 500 {
		value = value[:500]
	}
	return value
}
