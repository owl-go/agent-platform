package httpdelivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"agent-platform/backend/internal/biz/webhook/domain"
)

type Client interface {
	Do(*http.Request) (*http.Response, error)
}

type Sender struct {
	client Client
	secret []byte
	now    func() time.Time
}

func New(client Client, signingSecret []byte) (*Sender, error) {
	if client == nil || len(signingSecret) < 32 {
		return nil, fmt.Errorf("Webhook HTTP Client and a signing Secret of at least 32 bytes are required")
	}
	return &Sender{client: client, secret: append([]byte(nil), signingSecret...), now: time.Now}, nil
}

func (sender *Sender) Deliver(ctx context.Context, delivery domain.Delivery) error {
	if err := delivery.Validate(); err != nil {
		return err
	}
	timestamp := strconv.FormatInt(sender.now().UTC().Unix(), 10)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.TargetURL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return fmt.Errorf("create Webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Agent-Platform-Webhook/1.0")
	request.Header.Set("X-Agent-Platform-Delivery", delivery.ID)
	request.Header.Set("X-Agent-Platform-Event", delivery.EventType)
	request.Header.Set("X-Agent-Platform-Timestamp", timestamp)
	request.Header.Set("X-Agent-Platform-Signature", "sha256="+signature(sender.secret, timestamp, delivery.Payload))

	response, err := sender.client.Do(request)
	if err != nil {
		return fmt.Errorf("send Webhook request: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Webhook endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}

func signature(secret []byte, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
