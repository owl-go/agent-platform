package application

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

type cursorTestRepository struct{ data []byte }

func (r *cursorTestRepository) LoadChannelReceiveCursor(context.Context, ChannelStored) ([]byte, error) {
	return r.data, nil
}
func (r *cursorTestRepository) StoreChannelReceiveCursor(_ context.Context, _ ChannelStored, data []byte) error {
	r.data = append([]byte(nil), data...)
	return nil
}
func TestReceiveCursorEncryptedAndBoundToAccountVersion(t *testing.T) {
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	repo := &cursorTestRepository{}
	cursor := NewChannelReceiveCursor(repo, box)
	s := ChannelStored{Channel: domain.MessageChannel{ID: "channel", OwnerID: "owner", ConfigVersion: 1}}
	ctx := context.Background()
	if err := cursor.Save(ctx, s, "protected-provider-cursor"); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(repo.data, []byte("protected-provider-cursor")) {
		t.Fatal("plaintext persisted")
	}
	value, err := cursor.Load(ctx, s)
	if err != nil || value != "protected-provider-cursor" {
		t.Fatal("cursor recovery")
	}
	changed := s
	changed.Channel.ID = "other"
	if _, err := cursor.Load(ctx, changed); err == nil {
		t.Fatal("cursor crossed channels")
	}
	changed = s
	changed.Channel.ConfigVersion++
	if _, err := cursor.Load(ctx, changed); err == nil {
		t.Fatal("cursor crossed credential versions")
	}
	if _, err := box.Decrypt(repo.data, ChannelCredentialAAD(s.Channel)); err == nil {
		t.Fatal("cursor mistaken for credentials")
	}
	if err := cursor.Save(ctx, s, strings.Repeat("x", 16385)); err == nil {
		t.Fatal("unbounded cursor")
	}
}
