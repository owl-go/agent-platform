package platformconfig

import (
	"strings"
	"testing"
)

func TestLocalProfileRequiresExplicitOptIn(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Sandbox.Runtime = "runc"
	config.Worker.Runtimes = map[string]RuntimeEngineConfig{"codex": {Available: true, ImageDigest: "sha256:" + strings.Repeat("a", 64), CLIVersion: "0.147.0"}}
	if config.ValidateWorker() == nil {
		t.Fatal("production accepted runc")
	}
	config.Sandbox.Profile = "local"
	if err := config.ValidateWorker(); err != nil {
		t.Fatal(err)
	}
	config.Worker.CLIBuilder.Enabled = true
	if config.ValidateWorker() == nil {
		t.Fatal("local CLI Builder accepted")
	}
	config.Worker.CLIBuilder.Enabled = false
	config.Worker.Runtimes["codex"] = RuntimeEngineConfig{Available: true, ImageDigest: "runtime:latest", CLIVersion: "0.147.0"}
	if config.ValidateWorker() == nil {
		t.Fatal("mutable local image accepted")
	}
	config.Sandbox.Profile = "production"
	config.Sandbox.Runtime = "runsc"
	config.Worker.Runtimes["codex"] = RuntimeEngineConfig{Available: true, ImageDigest: "sha256:" + strings.Repeat("a", 64), CLIVersion: "0.147.0"}
	if config.ValidateWorker() == nil {
		t.Fatal("local image escaped into production")
	}
}

func TestLocalAPIRejectsPublicBindAddress(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Sandbox.Profile, config.Sandbox.Runtime = "local", "runc"
	for _, address := range []string{":8080", "0.0.0.0:8080", "192.168.1.2:8080"} {
		config.API.Address = address
		if config.ValidateAPI() == nil {
			t.Fatalf("accepted local public bind %q", address)
		}
	}
	config.API.Address = "127.0.0.1:8080"
	if err := config.ValidateAPI(); err != nil {
		t.Fatal(err)
	}
}
