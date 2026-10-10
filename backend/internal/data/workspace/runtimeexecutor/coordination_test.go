package runtimeexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func coordinatedFixture(t *testing.T) (*Executor, application.ExecutionJob, string) {
	executor, job, persistent := newTeamTestExecutor(t)
	model := job.Snapshot.ProviderModel
	model.ID = "model-1"
	job.Snapshot.SchemaVersion = 3
	job.Snapshot.RuntimeEngine = ""
	job.Snapshot.ProviderModel = domain.ProviderModelSnapshot{}
	job.Snapshot.ExpertTeam = nil
	job.Snapshot.TeamProfile = &domain.ExpertTeamProfileSnapshot{ID: "team-1", Name: "Team", LeadMemberID: "lead", Version: 1}
	job.Snapshot.Coordination = &domain.TeamCoordinationSnapshot{LeadMemberID: "lead"}
	job.Snapshot.Stages = []domain.ExecutionStageSnapshot{
		{Position: 1, TeamMemberID: "lead", TeamMemberName: "Lead", RuntimeEngine: domain.RuntimeCodex, ProviderModel: model, Expert: &domain.ExpertSnapshot{ID: "lead-expert", Name: "Lead", Guidance: "# Lead Guidance"}},
		{Position: 2, TeamMemberID: "reviewer", TeamMemberName: "Reviewer", RuntimeEngine: domain.RuntimeCodex, ProviderModel: model, Expert: &domain.ExpertSnapshot{ID: "review-expert", Name: "Reviewer", Guidance: "# Reviewer Guidance"}},
	}
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	return executor, job, persistent
}

func coordinationRuntimeResult(t *testing.T, sink agentruntime.EventSink, id, text string) agentruntime.Result {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"message": text})
	for i, event := range []agentruntime.Event{{Kind: agentruntime.EventRuntimeStarted, Payload: []byte(`{}`)}, {Kind: agentruntime.EventMessageCompleted, Payload: payload}, {Kind: agentruntime.EventRuntimeCompleted, Payload: []byte(`{}`)}} {
		event.RunID = id
		event.Sequence = int64(i + 1)
		event.OccurredAt = time.Now()
		if err := sink.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	return agentruntime.Result{FinalMessage: text}
}

func TestExecuteCoordinatedTeamDelegatesAndPublishesOnlyLeadResponse(t *testing.T) {
	executor, job, persistent := coordinatedFixture(t)
	var calls, ids []string
	leadCalls := 0
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
			calls = append(calls, request.Instruction)
			ids = append(ids, request.RunID)
			text := ""
			if strings.Contains(request.Instruction, "# Lead Guidance") {
				leadCalls++
				if leadCalls == 1 {
					text = `{"action":"delegate","tasks":[{"id":"review","member_id":"reviewer","instruction":"Check evidence","required":true}]}`
				} else {
					if !strings.Contains(request.Instruction, "Checked evidence") {
						t.Fatal("lead did not receive bounded member result")
					}
					text = `{"action":"complete","response":"Official reviewed answer"}`
				}
			} else {
				if !strings.Contains(request.Instruction, "Check evidence") {
					t.Fatal("member assignment absent")
				}
				if err := os.WriteFile(filepath.Join(request.WorkspacePath, "review.md"), []byte("Checked evidence"), 0600); err != nil {
					t.Fatal(err)
				}
				text = "Checked evidence"
			}
			return coordinationRuntimeResult(t, sink, request.RunID, text), nil
		}}, nil
	}
	progress := &recordingProgress{}
	result, err := executor.Execute(context.Background(), job, progress)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || result.FinalMessage != "Official reviewed answer" || len(result.ExpertStages) != 3 {
		t.Fatalf("coordinated result=%+v calls=%d", result, len(calls))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || id == job.ID {
			t.Fatal("actual invocations reused execution identity")
		}
		seen[id] = true
	}
	for _, event := range progress.events {
		if strings.Contains(string(event.Payload), `\"action\"`) || strings.Contains(string(event.Payload), `"action"`) {
			t.Fatal("coordination control output leaked into persisted activity", event.Type)
		}
	}
	if _, err := os.Stat(filepath.Join(persistent, "review.md")); !os.IsNotExist(err) {
		t.Fatal("member files promoted before whole response success")
	}
	commitSuccessfulResult(t, result)
	if data, err := os.ReadFile(filepath.Join(persistent, "review.md")); err != nil || string(data) != "Checked evidence" {
		t.Fatalf("successful staged merge=%q err=%v", data, err)
	}
}

func TestCoordinatedCancellationStopsQueuedMembersAndDiscardsAllFiles(t *testing.T) {
	executor, job, persistent := coordinatedFixture(t)
	started := make(chan struct{}, 4)
	var mutex sync.Mutex
	memberCalls := 0
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(ctx context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
			if strings.Contains(request.Instruction, "# Lead Guidance") {
				return coordinationRuntimeResult(t, sink, request.RunID, `{"action":"delegate","tasks":[{"id":"one","member_id":"reviewer","instruction":"One","required":true},{"id":"two","member_id":"reviewer","instruction":"Two","required":true},{"id":"three","member_id":"reviewer","instruction":"Three","required":true},{"id":"four","member_id":"reviewer","instruction":"Four","required":true}]}`), nil
			}
			mutex.Lock()
			memberCalls++
			mutex.Unlock()
			if err := os.WriteFile(filepath.Join(request.WorkspacePath, "uncommitted.txt"), []byte("Private pending work"), 0600); err != nil {
				t.Error(err)
			}
			started <- struct{}{}
			<-ctx.Done()
			return agentruntime.Result{}, ctx.Err()
		}}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type outcome struct {
		result application.ExecutionResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() { result, err := executor.Execute(ctx, job, &recordingProgress{}); done <- outcome{result, err} }()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("members did not start")
		}
	}
	cancel()
	select {
	case value := <-done:
		if !errors.Is(value.err, context.Canceled) || value.result.SuccessCommit != nil || value.result.FinalMessage != "" {
			t.Fatalf("cancelled result=%+v err=%v", value.result, value.err)
		}
	case <-time.After(time.Second):
		t.Fatal("members did not stop after cancellation")
	}
	if memberCalls != 3 {
		t.Fatal("queued member ran after cancellation")
	}
	if _, err := os.Stat(filepath.Join(persistent, "uncommitted.txt")); !os.IsNotExist(err) {
		t.Fatal("cancelled files promoted")
	}
	entries, err := os.ReadDir(filepath.Join(executor.config.Workspace.Root, ".team-attempts"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("attempt cleanup=%v %v", entries, err)
	}
}

func TestCoordinatedTeamLimitsParallelCallsAndIsolatesRepeatedMemberWorkspaces(t *testing.T) {
	executor, job, _ := coordinatedFixture(t)
	var mutex sync.Mutex
	active, peak, memberCalls, leadCalls := 0, 0, 0, 0
	paths := map[string]bool{}
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	finished := make(chan struct{})
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(ctx context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
			if strings.Contains(request.Instruction, "# Lead Guidance") {
				leadCalls++
				text := `{"action":"complete","response":"Official"}`
				if leadCalls == 1 {
					text = `{"action":"delegate","tasks":[{"id":"one","member_id":"reviewer","instruction":"One","required":true},{"id":"two","member_id":"reviewer","instruction":"Two","required":true},{"id":"three","member_id":"reviewer","instruction":"Three","required":true},{"id":"four","member_id":"reviewer","instruction":"Four","required":true}]}`
				}
				return coordinationRuntimeResult(t, sink, request.RunID, text), nil
			}
			mutex.Lock()
			active++
			memberCalls++
			if active > peak {
				peak = active
			}
			if paths[request.WorkspacePath] {
				t.Error("repeated member shared writable Workspace")
			}
			paths[request.WorkspacePath] = true
			mutex.Unlock()
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return agentruntime.Result{}, ctx.Err()
			}
			mutex.Lock()
			active--
			mutex.Unlock()
			return coordinationRuntimeResult(t, sink, request.RunID, "Done"), nil
		}}, nil
	}
	var result application.ExecutionResult
	var executeErr error
	go func() {
		defer close(finished)
		result, executeErr = executor.Execute(context.Background(), job, &recordingProgress{})
	}()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			<-finished
			t.Fatalf("only %d members started concurrently", i)
		}
	}
	select {
	case <-started:
		close(release)
		<-finished
		t.Fatal("more than three members executed concurrently")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	<-finished
	if executeErr != nil {
		t.Fatal(executeErr)
	}
	if peak != 3 || memberCalls != 4 || len(paths) != 4 || len(result.ExpertStages) != 6 {
		t.Fatalf("peak=%d calls=%d paths=%d invocations=%d", peak, memberCalls, len(paths), len(result.ExpertStages))
	}
	if result.SuccessCommit != nil {
		_ = result.SuccessCommit.Cleanup()
	}
}

func TestCoordinatedTeamReturnsFileConflictsToLeadForExplicitResolution(t *testing.T) {
	executor, job, persistent := coordinatedFixture(t)
	leadCalls := 0
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
			text := ""
			if strings.Contains(request.Instruction, "# Lead Guidance") {
				leadCalls++
				if leadCalls == 1 {
					text = `{"action":"delegate","tasks":[{"id":"one","member_id":"reviewer","instruction":"Write variant one","required":true},{"id":"two","member_id":"reviewer","instruction":"Write variant two","required":true}]}`
				} else {
					if !strings.Contains(request.Instruction, "shared.txt") || !strings.Contains(request.Instruction, "conflicts") {
						t.Fatal("file conflict absent from lead context")
					}
					text = `{"action":"complete","response":"Resolved answer","resolutions":[{"path":"shared.txt","source_task_id":"one"}]}`
				}
			} else {
				variant := "two"
				if strings.Contains(request.Instruction, "Write variant one") {
					variant = "one"
				}
				if _, err := os.Stat(filepath.Join(request.WorkspacePath, "shared.txt")); !os.IsNotExist(err) {
					t.Error("parallel member received another member's unmerged file")
				}
				if err := os.WriteFile(filepath.Join(request.WorkspacePath, "shared.txt"), []byte(variant), 0600); err != nil {
					t.Fatal(err)
				}
				text = "Variant " + variant
			}
			return coordinationRuntimeResult(t, sink, request.RunID, text), nil
		}}, nil
	}
	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if leadCalls != 2 || result.FinalMessage != "Resolved answer" {
		t.Fatalf("lead calls=%d result=%s", leadCalls, result.FinalMessage)
	}
	commitSuccessfulResult(t, result)
	body, err := os.ReadFile(filepath.Join(persistent, "shared.txt"))
	if err != nil || string(body) != "one" {
		t.Fatalf("selected merge=%q err=%v", body, err)
	}
}

func TestCoordinatedTeamEnforcesRequiredRepairAndInvocationLimits(t *testing.T) {
	for _, scenario := range []string{"required failure", "successful repair", "second repair forbidden", "invocation limit"} {
		t.Run(scenario, func(t *testing.T) {
			executor, job, _ := coordinatedFixture(t)
			leadCalls, memberCalls := 0, 0
			if scenario == "invocation limit" {
				job.Snapshot.Coordination.MaxInvocations = 2
			}
			executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
				return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
					if strings.Contains(request.Instruction, "# Lead Guidance") {
						leadCalls++
						text := `{"action":"complete","response":"Official"}`
						if leadCalls == 1 {
							text = `{"action":"delegate","tasks":[{"id":"original","member_id":"reviewer","instruction":"Review","required":true}]}`
						} else if leadCalls == 2 && scenario != "required failure" && scenario != "invocation limit" {
							text = `{"action":"delegate","tasks":[{"id":"repair","member_id":"reviewer","instruction":"Repair","required":false,"repair_of":"original"}]}`
						} else if leadCalls == 3 && scenario == "second repair forbidden" {
							text = `{"action":"delegate","tasks":[{"id":"another","member_id":"reviewer","instruction":"Try again","required":false,"repair_of":"original"}]}`
						}
						return coordinationRuntimeResult(t, sink, request.RunID, text), nil
					}
					memberCalls++
					if scenario != "invocation limit" && (memberCalls == 1 || scenario == "second repair forbidden") {
						publishRuntimeFailure(t, sink, request.RunID, "failed")
						return agentruntime.Result{}, errors.New("member failed")
					}
					return coordinationRuntimeResult(t, sink, request.RunID, "Repaired evidence"), nil
				}}, nil
			}
			result, err := executor.Execute(context.Background(), job, &recordingProgress{})
			if scenario == "successful repair" {
				if err != nil || result.FinalMessage != "Official" || memberCalls != 2 || leadCalls != 3 {
					t.Fatalf("repair result=%s member=%d lead=%d err=%v", result.FinalMessage, memberCalls, leadCalls, err)
				}
				_ = result.SuccessCommit.Cleanup()
			} else {
				if err == nil || result.SuccessCommit != nil {
					t.Fatalf("failed work promoted: %+v err=%v", result, err)
				}
			}
			if scenario == "second repair forbidden" && memberCalls != 2 {
				t.Fatalf("second repair executed: %d calls", memberCalls)
			}
			if scenario == "invocation limit" && leadCalls+memberCalls != 2 {
				t.Fatalf("invocation cap exceeded: lead=%d member=%d", leadCalls, memberCalls)
			}
		})
	}
}

func TestCoordinatedTeamRejectsControlOutsideFrozenContract(t *testing.T) {
	for _, action := range []string{`Delegate work in prose.`, `{"action":"delegate","tasks":[{"id":"one","member_id":"outsider","instruction":"Run","required":true}]}`, `{"action":"complete","response":"Official","runtime":"override"}`, `{"action":"delegate","action":"complete","response":"Ambiguous"}`} {
		t.Run(action, func(t *testing.T) {
			executor, job, _ := coordinatedFixture(t)
			calls := 0
			executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
				return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, sink agentruntime.EventSink) (agentruntime.Result, error) {
					calls++
					return coordinationRuntimeResult(t, sink, request.RunID, action), nil
				}}, nil
			}
			result, err := executor.Execute(context.Background(), job, &recordingProgress{})
			if err == nil || result.SuccessCommit != nil || calls != 1 {
				if result.SuccessCommit != nil {
					_ = result.SuccessCommit.Cleanup()
				}
				t.Fatalf("invalid control accepted calls=%d err=%v", calls, err)
			}
		})
	}
}
