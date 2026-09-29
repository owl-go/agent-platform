package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const moduleInternal = "agent-platform/backend/internal/"

func TestBizDoesNotDependOnTransportOrPersistence(t *testing.T) {
	root := repositoryRoot(t)
	walkGoFiles(t, filepath.Join(root, "internal", "biz"), func(path string, file *ast.File) {
		for _, imported := range imports(file) {
			if strings.HasPrefix(imported, moduleInternal+"data/") ||
				strings.HasPrefix(imported, moduleInternal+"service/") ||
				strings.Contains(imported, "go-kratos") || strings.Contains(imported, "gorm.io/") {
				t.Errorf("%s: Biz imports transport or persistence package %q", relative(root, path), imported)
			}
		}
	})
}

func TestBoundedContextBizDoesNotImportAnotherContext(t *testing.T) {
	root := repositoryRoot(t)
	bizRoot := filepath.Join(root, "internal", "biz")
	walkGoFiles(t, bizRoot, func(path string, file *ast.File) {
		relativePath := relative(bizRoot, path)
		contextName := strings.Split(relativePath, string(filepath.Separator))[0]
		if contextName == "workflow" || contextName == "transaction" || contextName == "authz" {
			return
		}
		for _, imported := range imports(file) {
			const prefix = moduleInternal + "biz/"
			if !strings.HasPrefix(imported, prefix) {
				continue
			}
			importedContext := strings.Split(strings.TrimPrefix(imported, prefix), "/")[0]
			if importedContext != contextName && importedContext != "authz" && importedContext != "workflow" && importedContext != "transaction" {
				t.Errorf("%s: %s Biz imports %s Biz", relative(root, path), contextName, importedContext)
			}
		}
	})
}

func TestContextRepositoriesDoNotWriteForeignTables(t *testing.T) {
	root := repositoryRoot(t)
	owned := map[string][]string{
		"agentlifecycle": {"agents", "agent_drafts", "agent_release_approvals", "agent_releases"},
		"approval":       {"approvals"},
		"artifact":       {"artifacts"},
		"audit":          {"audit_events"},
		"collaboration":  {"coding_tasks", "sessions", "session_messages", "memory_candidates", "agent_memories"},
		"execution":      {"runs", "run_attempts", "run_leases", "workspace_write_leases", "run_events"},
		"modelcatalog":   {"credential_profiles", "configured_models"},
		"runtimecatalog": {"runtime_images"},
		"sourcecontrol":  {"source_control_providers", "repository_bindings"},
		"webhook":        {"webhook_deliveries"},
	}
	allTables := make(map[string]string)
	for contextName, tables := range owned {
		for _, table := range tables {
			allTables[table] = contextName
		}
	}
	for contextName := range owned {
		directory := filepath.Join(root, "internal", "data", contextName, "gormrepo")
		walkSourceFiles(t, directory, func(path string, source string) {
			lower := strings.ToLower(source)
			for table, owner := range allTables {
				if owner == contextName || !strings.Contains(lower, table) {
					continue
				}
				for _, writeMarker := range []string{"table(\"" + table, "insert into " + table, "update " + table, "delete from " + table} {
					if strings.Contains(lower, writeMarker) {
						t.Errorf("%s: %s Data writes %s-owned table %s", relative(root, path), contextName, owner, table)
					}
				}
			}
		})
	}
}

func TestProtoServicesDoNotBridgeThroughHTTPHandlers(t *testing.T) {
	root := repositoryRoot(t)
	directory := filepath.Join(root, "internal", "service", "api")
	walkSourceFiles(t, directory, func(path, source string) {
		for _, forbidden := range []string{"http.ResponseWriter", "*http.Request", "decodeWriteRequest", "handledResponse"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s: Proto Service contains legacy HTTP bridge %q", relative(root, path), forbidden)
			}
		}
	})
}

func TestWorkerCapabilitiesSupportWorkspaceOwnershipNormalization(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "..", "deploy", "platform", "compose.execution.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	configuration := string(contents)
	for _, capability := range []string{"CHOWN", "DAC_OVERRIDE", "FOWNER"} {
		if !strings.Contains(configuration, "      - "+capability) {
			t.Errorf("Worker execution overlay is missing %s", capability)
		}
	}
}

func TestWorkerHasDedicatedProviderEgressNetwork(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "..", "deploy", "platform", "compose.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	configuration := string(contents)
	workerStart := strings.Index(configuration, "  worker:\n")
	networksStart := strings.Index(configuration, "\nnetworks:\n")
	if workerStart < 0 || networksStart <= workerStart {
		t.Fatal("compose worker or top-level networks block is missing")
	}
	workerBlock := configuration[workerStart:networksStart]
	if !strings.Contains(workerBlock, "      - provider-egress") {
		t.Error("Worker must have a dedicated network for AI Creation provider calls")
	}
	topLevelNetworks := configuration[networksStart:]
	if !strings.Contains(topLevelNetworks, "  provider-egress:\n  edge:") {
		t.Error("compose provider-egress network must provide external egress without joining the edge network")
	}
}

func TestCaddyAllowsLongRunningAPIResponses(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "..", "deploy", "platform", "Caddyfile")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	configuration := string(contents)
	apiStart := strings.Index(configuration, "    handle /api/* {")
	identityStart := strings.Index(configuration, "    handle /identity/* {")
	if apiStart < 0 || identityStart <= apiStart {
		t.Fatal("Caddyfile API proxy block is missing")
	}
	if apiBlock := configuration[apiStart:identityStart]; !strings.Contains(apiBlock, "response_header_timeout 5m") {
		t.Error("Caddyfile API proxy must allow long-running Image Model verification responses")
	}
}

func TestPlatformDeploymentRecreatesAndVerifiesCaddy(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "..", "scripts", "deploy-platform.sh")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	for _, required := range []string{
		`--force-recreate caddy`,
		`caddy_container="${CADDY_CONTAINER:-agent-platform-caddy-1}"`,
		`"$caddy_container")" = "$release_dir/deploy/platform"`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("deploy-platform.sh does not enforce %q", required)
		}
	}
}

func TestPlatformDeploymentBuildsPinsAndSmokesUnifiedRuntime(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "..", "scripts", "deploy-platform.sh")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	for _, required := range []string{
		`--file deploy/runtimes/unified/Dockerfile`,
		`docker push "$runtime_tag"`,
		`RUNTIME_IMAGE_REF="$runtime_digest" CLI_BUILDER_IMAGE_REF="$builder_digest" scripts/conformance/runtime-image-smoke.sh`,
		`RUNTIME_IMAGE={digest}`,
		`expected zero or five legacy Runtime image references`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("deploy-platform.sh does not enforce %q", required)
		}
	}
}

func TestPlatformDeploymentReverifiesCLIConnectorsBeforeCutover(t *testing.T) {
	path := filepath.Join(repositoryRoot(t), "..", "scripts", "deploy-platform.sh")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	verify := strings.Index(script, "-reverify-cli-connectors")
	cutover := strings.Index(script, "stage \"Activate source, migrate, and replace services\"")
	if verify < 0 || cutover <= verify {
		t.Fatal("CLI bundle Conformance must run before service cutover")
	}
	if !strings.Contains(script, `candidate_env_file="${env_file}.candidate-${release_id}"`) || !strings.Contains(script, `candidate_config_file="${config_file}.candidate-${release_id}"`) {
		t.Fatal("candidate Runtime configuration must not replace live configuration before Conformance passes")
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func walkGoFiles(t *testing.T, root string, visit func(string, *ast.File)) {
	t.Helper()
	walkSourceFiles(t, root, func(path string, source string) {
		file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		visit(path, file)
	})
}

func walkSourceFiles(t *testing.T, root string, visit func(string, string)) {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visit(path, string(contents))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func imports(file *ast.File) []string {
	values := make([]string, 0, len(file.Imports))
	for _, item := range file.Imports {
		value, err := strconv.Unquote(item.Path.Value)
		if err == nil {
			values = append(values, value)
		}
	}
	return values
}

func relative(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(value)
}
