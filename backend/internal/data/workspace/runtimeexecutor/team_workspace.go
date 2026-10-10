package runtimeexecutor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/workspacefs"
)

type teamWorkspaceCandidate struct{ taskID, workspace string }

// mergeTeamWorkspaces compares every candidate with the same staged baseline.
// Completion order is never a conflict-resolution rule.
func mergeTeamWorkspaces(ctx context.Context, workspace string, candidates []teamWorkspaceCandidate) (map[string][]teamWorkspaceCandidate, error) {

	baseline, err := teamWorkspaceManifest(workspace)
	if err != nil {
		return nil, err
	}
	type proposal struct {
		digest string
		source string
		taskID string
	}
	changes := map[string][]proposal{}
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		manifest, err := teamWorkspaceManifest(candidate.workspace)
		if err != nil {
			return nil, err
		}
		for path, digest := range manifest {
			if baseline[path] != digest {
				changes[path] = append(changes[path], proposal{digest, candidate.workspace, candidate.taskID})
			}
		}
		for path := range baseline {
			if _, exists := manifest[path]; !exists {
				changes[path] = append(changes[path], proposal{"", candidate.workspace, candidate.taskID})
			}
		}
	}
	paths := make([]string, 0, len(changes))
	for path := range changes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	conflicts := map[string]bool{}
	for index, path := range paths {
		values := changes[path]
		for _, value := range values[1:] {
			if value.digest != values[0].digest {
				conflicts[path] = true
			}
		}
		for _, other := range paths[index+1:] {
			if !strings.HasPrefix(other, path+"/") {
				continue
			}
			for _, first := range values {
				for _, second := range changes[other] {
					if first.taskID != second.taskID {
						conflicts[path] = true
						conflicts[other] = true
					}
				}
			}
		}
	}
	pending := map[string][]teamWorkspaceCandidate{}
	for path := range conflicts {
		for _, value := range changes[path] {
			pending[path] = append(pending[path], teamWorkspaceCandidate{taskID: value.taskID, workspace: value.source})
		}
	}

	for _, path := range paths {
		if !conflicts[path] && changes[path][0].digest == "" {
			if err := os.Remove(filepath.Join(workspace, filepath.FromSlash(path))); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if conflicts[path] {
			continue
		}
		chosen := changes[path][0]
		if chosen.digest == "" {
			continue
		}
		source := filepath.Join(chosen.source, filepath.FromSlash(path))
		target := filepath.Join(workspace, filepath.FromSlash(path))
		if err := copyTeamFile(ctx, source, target); err != nil {
			return nil, err
		}
	}
	used, err := executionWorkspaceSize(workspace)
	if err != nil {
		return nil, err
	}
	if used > workspacefs.WorkspaceLimit {
		return nil, fmt.Errorf("merged Team Workspace exceeds quota")
	}
	return pending, nil
}

func teamWorkspaceManifest(root string) (map[string]string, error) {
	result, err := workspaceManifest(root)
	if err != nil {
		return nil, err
	}
	for path, digest := range result {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		result[path] = fmt.Sprintf("%s:%o", digest, info.Mode().Perm()&0111)
	}
	return result, nil
}

type teamContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader teamContextReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(data)
}

func copyTeamFile(ctx context.Context, source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Team Workspace contains an unsupported file type")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	if existing, err := os.Lstat(target); err == nil && !existing.Mode().IsRegular() {
		return fmt.Errorf("Team Workspace path conflicts with a directory")
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.CreateTemp(filepath.Dir(target), ".team-merge-")
	if err != nil {
		return err
	}
	temporary := output.Name()
	defer os.Remove(temporary)
	_, copyErr := io.Copy(output, teamContextReader{ctx, input})
	closeErr := output.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if info.Mode().Perm()&0111 != 0 {
		mode = 0700
	}
	if err := os.Chmod(temporary, mode); err != nil {
		return err
	}
	return os.Rename(temporary, target)
}

func teamConflictFacts(pending map[string][]teamWorkspaceCandidate, resolved map[string]domain.TeamWorkspaceConflict) []domain.TeamWorkspaceConflict {
	paths := make([]string, 0, len(pending)+len(resolved))
	for path := range pending {
		paths = append(paths, path)
	}
	for path := range resolved {
		if _, exists := pending[path]; !exists {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	facts := make([]domain.TeamWorkspaceConflict, 0, len(paths))
	for _, path := range paths {
		if fact, ok := resolved[path]; ok {
			facts = append(facts, fact)
			continue
		}
		ids := map[string]bool{}
		for _, candidate := range pending[path] {
			ids[candidate.taskID] = true
		}
		taskIDs := make([]string, 0, len(ids))
		for id := range ids {
			taskIDs = append(taskIDs, id)
		}
		sort.Strings(taskIDs)
		facts = append(facts, domain.TeamWorkspaceConflict{Path: path, TaskIDs: taskIDs, State: "unresolved"})
	}
	return facts
}

func resolveTeamFiles(ctx context.Context, workspace string, choices []domain.TeamFileResolution, pending map[string][]teamWorkspaceCandidate, resolved map[string]domain.TeamWorkspaceConflict) error {
	// Validate every choice before applying any part of the round.
	selected := map[string]string{}
	for _, choice := range choices {
		candidates, exists := pending[choice.Path]
		if !exists {
			return fmt.Errorf("file resolution references no pending conflict")
		}
		if choice.SourceTaskID == "$lead" {
			selected[choice.Path] = workspace
			continue
		}
		for _, candidate := range candidates {
			if candidate.taskID == choice.SourceTaskID {
				selected[choice.Path] = candidate.workspace
				break
			}
		}
		if selected[choice.Path] == "" {
			return fmt.Errorf("file resolution does not select a contributing task")
		}
	}
	for _, choice := range choices {
		if err := ctx.Err(); err != nil {
			return err
		}
		source := selected[choice.Path]
		if source != workspace {
			input := filepath.Join(source, filepath.FromSlash(choice.Path))
			target := filepath.Join(workspace, filepath.FromSlash(choice.Path))
			info, err := os.Lstat(input)
			switch {
			case errors.Is(err, os.ErrNotExist):
				if err := os.RemoveAll(target); err != nil {
					return err
				}
			case err != nil:
				return err
			case info.IsDir():
				if err := os.RemoveAll(target); err != nil {
					return err
				}
				if err := os.MkdirAll(target, 0700); err != nil {
					return err
				}
			case info.Mode().IsRegular():
				if existing, err := os.Lstat(target); err == nil && existing.IsDir() {
					if err := os.RemoveAll(target); err != nil {
						return err
					}
				}
				if err := copyTeamFile(ctx, input, target); err != nil {
					return err
				}
			default:
				return fmt.Errorf("file resolution source is unsafe")
			}
		}
		fact := teamConflictFacts(map[string][]teamWorkspaceCandidate{choice.Path: pending[choice.Path]}, nil)[0]
		fact.State = "resolved"
		fact.ResolutionSourceTaskID = choice.SourceTaskID
		resolved[choice.Path] = fact
		delete(pending, choice.Path)
	}
	return nil
}

func publishTeamConflicts(ctx context.Context, progress application.ProgressRecorder, job application.ExecutionJob, result *application.ExecutionResult, pending map[string][]teamWorkspaceCandidate, resolved map[string]domain.TeamWorkspaceConflict) error {
	facts := teamConflictFacts(pending, resolved)
	for index := range result.ExpertStages {
		stage := &result.ExpertStages[index]
		if stage.Role != "member" {
			continue
		}
		var related []domain.TeamWorkspaceConflict
		for _, fact := range facts {
			for _, id := range fact.TaskIDs {
				if id == stage.TaskID {
					related = append(related, fact)
					break
				}
			}
		}
		if len(related) == 0 {
			continue
		}
		stage.WorkspaceConflicts = related
		if err := recordExpertStage(ctx, progress, job, *stage); err != nil {
			return err
		}
	}
	return nil
}
