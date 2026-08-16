package gormdb

import (
	"context"
	"testing"
	"time"
)

func TestOpenRejectsInvalidConfigurationBeforeConnecting(t *testing.T) {
	for _, config := range []Config{
		{},
		{DSN: "postgres://database/platform"},
		{DSN: "postgres://database/platform", MaxOpenConnections: 1, MaxIdleConnections: 2, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute},
	} {
		if _, err := Open(context.Background(), config); err == nil {
			t.Fatalf("Open accepted invalid configuration: %+v", config)
		}
	}
}
