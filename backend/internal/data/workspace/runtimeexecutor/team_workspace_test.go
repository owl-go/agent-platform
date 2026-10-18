package runtimeexecutor

import (
	"agent-platform/backend/internal/credentials"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeTeamWorkspaceReplacesFilesAndDirectories(t *testing.T) {
	for _, directoryToFile := range []bool{true, false} {
		name := "file_to_directory"
		if directoryToFile {
			name = "directory_to_file"
		}
		t.Run(name, func(t *testing.T) {
			baseline, candidate := t.TempDir(), t.TempDir()
			write := func(root, path, content string) {
				t.Helper()
				target := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			oldPath, newPath := "item", "item/nested/file.txt"
			if directoryToFile {
				oldPath, newPath = newPath, oldPath
			}
			write(baseline, oldPath, "original")
			write(candidate, newPath, "replacement")
			conflicts, err := mergeTeamWorkspaces(context.Background(), baseline, []teamWorkspaceCandidate{{taskID: "one", workspace: candidate}})
			if err != nil || len(conflicts) != 0 {
				t.Fatalf("valid replacement rejected: %v %+v", err, conflicts)
			}
			data, err := os.ReadFile(filepath.Join(baseline, newPath))
			if err != nil || string(data) != "replacement" {
				t.Fatalf("replacement=%q err=%v", data, err)
			}
		})
	}
}

func TestMergeTeamWorkspaceRetainsCompetingDescendantChanges(t *testing.T) {
	baseline, replacement, descendant := t.TempDir(), t.TempDir(), t.TempDir()
	for _, root := range []string{baseline, descendant} {
		if err := os.Mkdir(filepath.Join(root, "item"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for root, content := range map[string]string{baseline: "original", descendant: "competing"} {
		if err := os.WriteFile(filepath.Join(root, "item/file.txt"), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(replacement, "item"), []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	conflicts, err := mergeTeamWorkspaces(t.Context(), baseline, []teamWorkspaceCandidate{{taskID: "replace", workspace: replacement}, {taskID: "descendant", workspace: descendant}})
	if err != nil || len(conflicts) != 2 {
		t.Fatalf("competing ancestor change bypassed conflict: %v %+v", err, conflicts)
	}
	data, err := os.ReadFile(filepath.Join(baseline, "item/file.txt"))
	if err != nil || string(data) != "original" {
		t.Fatalf("unresolved file was overwritten: %q %v", data, err)
	}
}

func TestTeamFileChangeSummaryBoundsAndRedactsPaths(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 21; index++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("%02d-private-path-token.txt", index)), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := summarizeTeamFileChanges(t.Context(), root, map[string]string{}, credentials.NewRedactor([]byte("private-path-token")))
	if err != nil || len(summary.Changes) != 20 || !summary.Truncated {
		t.Fatalf("summary was not bounded: %v %+v", err, summary)
	}
	if summary.Changes[0].Path != "00-[REDACTED].txt" || summary.Changes[0].Change != "added" {
		t.Fatalf("unsafe change summary: %+v", summary.Changes[0])
	}
}

func TestTeamFileChangeSummaryBoundsLongUTF8Paths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, strings.Repeat("界", 70)+".txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	summary, err := summarizeTeamFileChanges(t.Context(), root, map[string]string{}, credentials.NewRedactor())
	if err != nil || len(summary.Changes) != 1 || len(summary.Changes[0].Path) > 200 || !strings.HasSuffix(summary.Changes[0].Path, "…") {
		t.Fatalf("path was not bounded: %v %+v", err, summary)
	}
}
