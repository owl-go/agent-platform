package platformconfig

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Exercise the actual installer output against the same strict loader used by API and Worker.
func TestFirstInstallationConfigurationValidatesWithExecutionDisabled(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	command := exec.Command("python3", "-c", `import json,sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / 'scripts'))
from installation import generate_config
values, config, _ = generate_config(Path(sys.argv[1]), {'host':'root@server.example.com','root':'/srv/agent-workspace','domain':'workspace.example.com','email':'admin@example.com'})
print(json.dumps({'values':values, 'config':config}))`, repo)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("generate installation fixture: %v", err)
	}
	var fixture struct {
		Values map[string]string `json:"values"`
		Config string            `json:"config"`
	}
	if err := json.Unmarshal(output, &fixture); err != nil {
		t.Fatal("installer fixture is not JSON")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	fixture.Values["REGISTRATION_SIGNING_KEY"] = base64.StdEncoding.EncodeToString(der)
	fixture.Values["DATABASE_URL"] = "postgres://agent_platform:fixture-only@postgres:5432/agent_platform?sslmode=disable"
	for name, value := range fixture.Values {
		t.Setenv(name, value)
	}
	path := filepath.Join(t.TempDir(), "platform.yaml")
	if err := os.WriteFile(path, []byte(fixture.Config), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(path)
	if err != nil {
		t.Fatalf("load generated configuration: %v", err)
	}
	if err := config.ValidateAPI(); err != nil {
		t.Fatalf("generated API configuration: %v", err)
	}
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("generated Worker configuration: %v", err)
	}
	if config.Worker.CLIBuilder.Enabled || config.MessageChannels.Enabled {
		t.Fatal("optional execution / channel capability enabled without evidence")
	}
	if len(config.Worker.Runtimes) != 5 {
		t.Fatal("installer must configure all five unavailable engines")
	}
	for name, engine := range config.Worker.Runtimes {
		if engine.Available || engine.NativeResume || engine.ImageDigest != "" {
			t.Fatalf("unverified engine %q was enabled or assigned a digest", name)
		}
	}
}
