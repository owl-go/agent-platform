package conf_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-platform/backend/internal/conf"
)

func TestLoadRetainsStrictYAMLAndExpandsEnvironmentOnce(t *testing.T) {
	t.Setenv("DATABASE_VALUE", "${NESTED_VALUE}")
	t.Setenv("NESTED_VALUE", "must-not-expand")
	path := filepath.Join(t.TempDir(), "platform.yaml")
	contents := `
api:
  address: :8080
  read_header_timeout: 2s
  idle_timeout: 1m
  shutdown_timeout: 10s
authentication:
  mode: deny_all
worker: {}
database:
  dsn: ${DATABASE_VALUE}
  max_open_connections: 5
  max_idle_connections: 2
  connection_max_idle: 1m
  connection_max_lifetime: 5m
object_store:
  provider: minio
  minio: {}
  aliyun_oss: {}
sandbox:
  runtime: runsc
  egress_network: egress
  resolver_config: /etc/resolv.conf
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := conf.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Database.DSN != "${NESTED_VALUE}" {
		t.Fatalf("database DSN = %q, want exactly-once expansion", loaded.Database.DSN)
	}

	if err := os.WriteFile(path, []byte(contents+"unknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := conf.Load(path); err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("unknown YAML field error = %v", err)
	}
}
