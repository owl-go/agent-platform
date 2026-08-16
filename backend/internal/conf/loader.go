package conf

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"agent-platform/backend/internal/platformconfig"

	kratosconfig "github.com/go-kratos/kratos/v3/config"
)

func Load(path string) (platformconfig.Config, error) {
	validated, err := platformconfig.Load(path)
	if err != nil {
		return platformconfig.Config{}, err
	}
	payload, err := json.Marshal(validated)
	if err != nil {
		return platformconfig.Config{}, fmt.Errorf("encode validated configuration: %w", err)
	}
	source := newImmutableSource(payload)
	loader := kratosconfig.New(
		kratosconfig.WithSource(source),
		kratosconfig.WithResolver(func(map[string]any) error { return nil }),
	)
	if err := loader.Load(); err != nil {
		return platformconfig.Config{}, fmt.Errorf("load Kratos configuration: %w", err)
	}
	defer loader.Close()
	var scanned platformconfig.Config
	if err := loader.Scan(&scanned); err != nil {
		return platformconfig.Config{}, fmt.Errorf("scan Kratos configuration: %w", err)
	}
	return scanned, nil
}

type immutableSource struct {
	payload []byte
	watcher *immutableWatcher
}

func newImmutableSource(payload []byte) *immutableSource {
	return &immutableSource{
		payload: append([]byte(nil), payload...),
		watcher: &immutableWatcher{done: make(chan struct{})},
	}
}

func (source *immutableSource) Load() ([]*kratosconfig.KeyValue, error) {
	return []*kratosconfig.KeyValue{{Key: "platform", Value: append([]byte(nil), source.payload...), Format: "json"}}, nil
}

func (source *immutableSource) Watch() (kratosconfig.Watcher, error) {
	return source.watcher, nil
}

type immutableWatcher struct {
	done chan struct{}
	once sync.Once
}

func (watcher *immutableWatcher) Next() ([]*kratosconfig.KeyValue, error) {
	<-watcher.done
	return nil, context.Canceled
}

func (watcher *immutableWatcher) Stop() error {
	watcher.once.Do(func() { close(watcher.done) })
	return nil
}
