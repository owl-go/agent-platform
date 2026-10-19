//go:build linux

package workspaceworker

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime/containerprocess"
	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/defaultresources"
	"agent-platform/backend/internal/objectstore/memory"
	"agent-platform/backend/internal/sandbox"
)

// Run this test binary in the local control container so Docker's bind paths
// use exactly the same shim and data volume as real Worker execution.
func TestLocalDockerConnectorExecution(t *testing.T) {
	image, root := os.Getenv("CLI_CONNECTOR_LOCAL_TEST_IMAGE"), os.Getenv("CLI_CONNECTOR_LOCAL_TEST_ROOT")
	if image == "" || root == "" {
		t.Skip("requires explicit local control-container Docker acceptance environment")
	}
	if os.Getuid() != 0 || !filepath.IsAbs(root) || root == "/" {
		t.Fatal("requires a scoped shared control directory")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(root, "acceptance-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	suite, err := cliconnector.NewDockerConformance(cliconnector.DockerConformanceConfig{Profile: "local", DockerCommand: "docker", Runtime: "runc", TempRoot: filepath.Join(directory, "verification"), RuntimeImages: map[string]string{image: image}, UID: 65532, GID: 65532, Timeout: 30 * time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("real-distributed-bundles", func(t *testing.T) {
		catalog, err := defaultresources.LoadContext(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, definition := range catalog.Connectors {
			if definition.Source != "feishu" && definition.Source != "dingtalk" && definition.Source != "wecom" {
				continue
			}
			found++
			t.Run(definition.Source, func(t *testing.T) {
				pkg, err := catalog.ParseConnector(definition)
				if candidate := os.Getenv("CLI_CONNECTOR_LOCAL_TEST_WECOM_PACKAGE"); definition.Source == "wecom" && candidate != "" {
					var archive []byte
					archive, err = os.ReadFile(candidate)
					if err == nil {
						pkg, err = connectorpackage.Parse(archive)
					}
					if err == nil && pkg.Metadata.Source != "wecom" {
						t.Fatal("candidate is not WeCom")
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := suite.Test(t.Context(), pkg.CLIBundle, image, cliconnector.Definition{Executable: pkg.CLI.Executable, CPUMillis: pkg.CLI.Limits.CPU, MemoryMiB: pkg.CLI.Limits.MemoryMiB, ChildProcesses: pkg.CLI.Limits.ChildProcesses}); err != nil {
					t.Fatal(err)
				}
			})
		}
		if found != 3 {
			t.Fatalf("found %d requested connectors", found)
		}
	})
	t.Run("runtime-client-broker-isolated-command", func(t *testing.T) {
		const canary = "fixture-credential-not-real-092183"
		bundle := localConnectorFixtureBundle(t, `#!/usr/bin/env node
if(process.argv.includes('--help')){console.log('fixture help');process.exit(0)}
console.log('fixture-ok '+process.env.FIXTURE_TOKEN);
`)
		digest := sha256.Sum256(bundle)
		hash := hex.EncodeToString(digest[:])
		key := "cli-connectors/fixture/" + hash + ".tgz"
		store, _ := cliconnector.NewArtifactStore(memory.New())
		if err := store.PutImmutable(t.Context(), key, bundle, hash); err != nil {
			t.Fatal(err)
		}
		bundles, workspace, credentials, scratch := filepath.Join(directory, "bundles"), filepath.Join(directory, "workspace"), filepath.Join(directory, "credentials"), filepath.Join(directory, "scratch")
		for _, path := range []string{bundles, workspace, credentials, scratch} {
			if err := os.MkdirAll(path, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chown(path, 65532, 65532); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.MaterializeVerified(t.Context(), key, hash, filepath.Join(bundles, "fixture")); err != nil {
			t.Fatal(err)
		}
		process, err := cliconnector.NewDockerContainerProcess(cliconnector.DockerContainerProcessConfig{Profile: "local", Image: image, Runtime: "runc", RunID: "local-connector-acceptance", BundleDirectory: bundles, WorkspaceDirectory: workspace, EgressNetwork: "none", Limits: sandbox.Limits{CPUs: 1, MemoryBytes: 256 << 20, PIDs: 64, TempBytes: 32 << 20}, UID: 65532, GID: 65532})
		if err != nil {
			t.Fatal(err)
		}
		definition := cliconnector.Definition{ID: "fixture", Name: "Fixture", Executable: "tool", State: cliconnector.StateAvailable, AuthenticationDriver: "none", BundleSHA256: hash, RuntimeDigests: []string{image}, Capabilities: []cliconnector.Capability{{ID: "identity", ArgvPrefix: []string{"identity", "whoami"}, Risk: cliconnector.RiskLow, Identities: []cliconnector.Identity{cliconnector.IdentityUser}, EgressHosts: []string{"example.com"}, Timeout: 10 * time.Second}}}
		broker, err := cliconnector.NewBroker(cliconnector.BrokerConfig{Definitions: []cliconnector.Definition{definition}, RuntimeDigest: image, Wrapper: cliconnector.Wrapper{Process: process}, ResolveEnvironment: func(context.Context, cliconnector.Definition, cliconnector.Capability, cliconnector.Identity) (map[string]string, error) {
			return map[string]string{"FIXTURE_TOKEN": canary}, nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		socket := filepath.Join(scratch, "broker", "cli-broker.sock")
		server, err := cliconnector.StartUnixBroker(t.Context(), broker, socket, 65532, 65532)
		if err != nil {
			t.Fatal(err)
		}
		defer server.Close()
		run, err := containerprocess.New(containerprocess.Config{Profile: "local", DockerCommand: "docker", Image: image, RuntimeCommand: "node", RunID: "local-connector-client", Runtime: "runc", UID: 65532, GID: 65532, Egress: sandbox.EgressNone, WorkspaceDirectory: workspace, ContainerWorkspace: "/workspace", CredentialDirectory: credentials, ScratchDirectory: scratch, CLIBrokerSocket: socket, DirectCommand: true, Limits: sandbox.Limits{CPUs: 1, MemoryBytes: 256 << 20, PIDs: 64, TempBytes: 32 << 20}})
		if err != nil {
			t.Fatal(err)
		}
		output := &localConnectorOutput{}
		result, err := run(t.Context(), processharness.Spec{Command: []string{"node", "/usr/local/bin/agent-cli", "--connector", "fixture", "--capability", "identity", "--identity", "user", "--", "identity", "whoami"}, Dir: workspace}, output)
		if err != nil || result.ExitCode != 0 || !strings.Contains(output.text, "fixture-ok [REDACTED]") || strings.Contains(output.text, canary) {
			t.Fatalf("broker round trip failed exit=%d error=%v output=%q", result.ExitCode, err, output.text)
		}
	})
}

type localConnectorOutput struct{ text string }

func (o *localConnectorOutput) Store(_ context.Context, item processharness.Output) error {
	data, err := io.ReadAll(item.Reader)
	o.text += string(data)
	return err
}
func localConnectorFixtureBundle(t *testing.T, script string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	if err := archive.WriteHeader(&tar.Header{Name: "node_modules/.bin/tool", Mode: 0755, Size: int64(len(script)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(script)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
