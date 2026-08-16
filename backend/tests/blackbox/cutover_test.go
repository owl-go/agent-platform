package blackbox_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	platformv1 "agent-platform/backend/api/platform/v1"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const databaseEnvironment = "AGENT_PLATFORM_BLACKBOX_DATABASE_DSN"

func TestBuiltAPIAndWorkerCutover(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(databaseEnvironment))
	if dsn == "" {
		t.Skip(databaseEnvironment + " is not set; built-binary PostgreSQL black-box suite skipped")
	}

	root := moduleRoot(t)
	schema := fmt.Sprintf("blackbox_%d", time.Now().UnixNano())
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.ExecContext(context.Background(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`)
	})
	isolatedDSN := dsnWithSearchPath(dsn, schema)

	temporary := t.TempDir()
	apiBinary := buildBinary(t, root, temporary, "api", "./cmd/api")
	workerBinary := buildBinary(t, root, temporary, "worker", "./cmd/worker")
	apiAddress := freeAddress(t)
	workerAddress := freeAddress(t)
	configPath := filepath.Join(temporary, "platform.yaml")
	config := blackboxConfig(isolatedDSN, apiAddress, workerAddress)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	api := startProcess(t, apiBinary, "-config", configPath)
	worker := startProcess(t, workerBinary, "-config", configPath)
	waitForGeneratedReadiness(t, "http://"+apiAddress)
	waitForHTTPStatus(t, "http://"+workerAddress+"/readyz", http.StatusOK)

	assertEveryProtectedOperationIsMounted(t, root, "http://"+apiAddress)

	stopProcess(t, api)
	stopProcess(t, worker)
}

func assertEveryProtectedOperationIsMounted(t *testing.T, root, endpoint string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, "gen/openapi/agent-platform.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{"delete": true, "get": true, "patch": true, "post": true, "put": true}
	for path, operations := range document.Paths {
		if path == "/healthz" || path == "/readyz" {
			continue
		}
		for method := range operations {
			if !methods[method] {
				continue
			}
			t.Run(strings.ToUpper(method)+" "+path, func(t *testing.T) {
				request, err := http.NewRequest(strings.ToUpper(method), endpoint+concretePath(path), strings.NewReader(`{}`))
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set("Content-Type", "application/json")
				response, err := http.DefaultClient.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != http.StatusUnauthorized || string(body) != "{\"error\":\"authentication_required\"}\n" {
					t.Fatalf("response = (%d, %q), want stable 401 authentication error", response.StatusCode, body)
				}
				if response.Header.Get("Cache-Control") != "no-store" {
					t.Fatalf("Cache-Control = %q, want no-store", response.Header.Get("Cache-Control"))
				}
			})
		}
	}
}

func concretePath(path string) string {
	const identifier = "00000000-0000-4000-8000-000000000001"
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[index] = identifier
		}
	}
	return strings.Join(parts, "/")
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func buildBinary(t *testing.T, root, destination, name, pkg string) string {
	t.Helper()
	path := filepath.Join(destination, name)
	command := exec.Command("go", "build", "-o", path, pkg)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}

func startProcess(t *testing.T, binary string, arguments ...string) *exec.Cmd {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil || !command.ProcessState.Exited() {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
		}
	})
	return command
}

func stopProcess(t *testing.T, command *exec.Cmd) {
	t.Helper()
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("process did not stop cleanly: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("process did not stop within graceful shutdown deadline")
	}
}

func waitForGeneratedReadiness(t *testing.T, endpoint string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := kratoshttp.NewClient(ctx, kratoshttp.WithEndpoint(endpoint), kratoshttp.WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	health := platformv1.NewHealthServiceHTTPClient(client)
	for ctx.Err() == nil {
		response, err := health.Ready(ctx, &platformv1.ReadyRequest{})
		if err == nil && response.Status == "ready" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("generated client did not observe API readiness")
}

func waitForHTTPStatus(t *testing.T, url string, status int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == status {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s did not return %d", url, status)
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func dsnWithSearchPath(dsn, schema string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "search_path=" + schema
}

func blackboxConfig(dsn, apiAddress, workerAddress string) string {
	return fmt.Sprintf(`api:
  address: %q
  read_header_timeout: 2s
  idle_timeout: 30s
  shutdown_timeout: 5s
authentication:
  mode: deny_all
worker:
  management_address: %q
  shutdown_timeout: 5s
  reconcile_interval: 100ms
  max_attempts: 3
  execution_enabled: false
  id: blackbox
  poll_interval: 1s
  lease_duration: 30s
  renew_interval: 2s
  adapter_version: blackbox
  secret_store_root: /tmp/agent-platform-blackbox-secrets
  credential_temp_root: /tmp/agent-platform-blackbox-credentials
  sandbox_uid: 65532
  sandbox_gid: 65532
webhook:
  enabled: false
  poll_interval: 1s
  request_timeout: 1s
  lease_duration: 2s
  retry_base: 1s
  retry_maximum: 2s
  max_attempts: 1
  signing_secret: ""
  target_url: ""
retention:
  enabled: false
  sweep_interval: 1h
  batch_size: 100
  run_event_period: 168h
  artifact_period: 168h
  workspace_period: 168h
  audit_period: 168h
  idempotency_grace: 1h
database:
  dsn: %q
  max_open_connections: 10
  max_idle_connections: 2
  connection_max_idle: 1m
  connection_max_lifetime: 5m
object_store:
  provider: minio
  minio:
    endpoint: 127.0.0.1:9000
    access_key: blackbox
    secret_key: blackbox-secret
    bucket: blackbox
    secure: false
  aliyun_oss:
    endpoint: ""
    access_key_id: ""
    access_key_secret: ""
    bucket: ""
    prefix: ""
sandbox:
  runtime: runsc
  egress_network: blackbox
  resolver_config: /etc/resolv.conf
`, apiAddress, workerAddress, dsn)
}
