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
