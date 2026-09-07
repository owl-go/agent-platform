package domain

import (
	"path"
	"strings"
)

// FinalArtifactPaths returns changed files explicitly named by the final response.
// A basename selects a nested file only when that basename is unambiguous.
func FinalArtifactPaths(finalText string, changedPaths []string) map[string]struct{} {
	selected := make(map[string]struct{})
	basenameCounts := make(map[string]int, len(changedPaths))
	for _, value := range changedPaths {
		basenameCounts[path.Base(value)]++
	}
	for _, value := range changedPaths {
		if value == "" || value == "." {
			continue
		}
		if strings.Contains(finalText, value) {
			selected[value] = struct{}{}
			continue
		}
		base := path.Base(value)
		if basenameCounts[base] == 1 && strings.Contains(finalText, base) {
			selected[value] = struct{}{}
		}
	}
	return selected
}
