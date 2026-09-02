package gormrepo

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"agent-platform/backend/internal/biz/workspace/application"
)

func TestValidateRunInputBoundsTextAndJSON(t *testing.T) {
	tooLong := strings.Repeat("x", 100_001)
	if err := validateRunInput(&tooLong, nil); err == nil {
		t.Fatal("validateRunInput accepted text above 100000 bytes")
	}
	valid := strings.Repeat("x", 100_000)
	if err := validateRunInput(&valid, map[string]any{"ok": true}); err != nil {
		t.Fatalf("validateRunInput rejected valid input: %v", err)
	}
	if err := validateRunInput(nil, map[string]any{"payload": strings.Repeat("x", 1_048_576)}); err == nil {
		t.Fatal("validateRunInput accepted JSON above 1 MiB")
	}
}

func TestFileArtifactRecordsDoNotTurnFinalTextIntoAFile(t *testing.T) {
	job := application.ExecutionJob{ID: "run-1", OwnerID: "owner-1", WorkflowID: "workflow-1"}
	if records := fileArtifactRecords(job, nil, time.Now()); len(records) != 0 {
		t.Fatalf("text-only Run produced %d Artifact records", len(records))
	}

	expiresAt := time.Now().Add(90 * 24 * time.Hour)
	records := fileArtifactRecords(job, []application.ExecutionArtifact{{Name: "report.md", Path: "report.md", ObjectKey: "artifacts/report", Size: 12, SHA256: strings.Repeat("a", 64), ExpiresAt: expiresAt}}, time.Now())
	if len(records) != 1 || records[0].Kind != "file" || records[0].Name != "report.md" {
		t.Fatalf("file Artifact records = %#v", records)
	}
}

func TestSummarizeRunConversationsUsesLatestTurnProjection(t *testing.T) {
	oldTime := time.Date(2026, 8, 30, 12, 36, 18, 0, time.UTC)
	otherTime := oldTime.Add(24 * time.Hour)
	latestTime := oldTime.Add(72 * time.Hour)
	latestStarted := latestTime.Add(time.Second)
	latestEnded := latestStarted.Add(15 * time.Second)
	rows := []runRecord{
		{ID: "conversation-1", ConversationID: "conversation-1", TurnNumber: 1, Trigger: "api", State: "succeeded", QueuedAt: oldTime},
		{ID: "turn-2", ConversationID: "conversation-1", TurnNumber: 2, Trigger: "manual", State: "failed", QueuedAt: latestTime, StartedAt: &latestStarted, EndedAt: &latestEnded},
		{ID: "conversation-2", ConversationID: "conversation-2", TurnNumber: 1, Trigger: "scheduled", State: "succeeded", QueuedAt: otherTime},
	}

	summaries := summarizeRunConversations(rows)
	if len(summaries) != 2 {
		t.Fatalf("got %d Run Conversation summaries", len(summaries))
	}
	latest := summaries[0]
	if latest.ID != "conversation-1" || latest.Trigger != "api" || latest.State != "failed" || latest.TurnNumber != 2 || !latest.QueuedAt.Equal(latestTime) {
		t.Fatalf("latest Run Conversation summary = %#v", latest)
	}
	if latest.StartedAt == nil || !latest.StartedAt.Equal(latestStarted) || latest.EndedAt == nil || !latest.EndedAt.Equal(latestEnded) {
		t.Fatalf("latest Run Conversation timestamps = %#v / %#v", latest.StartedAt, latest.EndedAt)
	}
}

func TestWorkflowRunInstructionIncludesPriorConversation(t *testing.T) {
	firstInput, _ := json.Marshal(map[string]any{"text": "先检查代码", "json": nil})
	firstResult, _ := json.Marshal(map[string]any{"text": "发现两个问题", "json": nil})
	current := "继续修复第一个问题"
	instruction := workflowRunInstruction("完成代码审查", []runRecord{{Input: firstInput, FinalResult: firstResult}}, &current, nil)

	for _, expected := range []string{"Workflow goal:\n完成代码审查", "user: 先检查代码", "assistant: 发现两个问题", "Current user message:\n继续修复第一个问题"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("instruction %q does not contain %q", instruction, expected)
		}
	}
}

func TestSessionInstructionUsesOnlyCurrentMessageWhenNativeResumeIsActive(t *testing.T) {
	recent := []messageRecord{{Role: "user", Content: "old question"}, {Role: "assistant", Content: "old answer"}}
	got := sessionInstruction("old summary", recent, "new question", true)
	if got != "Current user message:\nnew question" {
		t.Fatalf("instruction = %q", got)
	}
}

func TestBoundedSummaryPreservesUTF8(t *testing.T) {
	value := strings.Repeat("你", 10_000)
	bounded := boundedSummary(value)
	if !utf8.ValidString(bounded) {
		t.Fatal("boundedSummary split a UTF-8 rune")
	}
	if len(bounded) > 20_000 {
		t.Fatalf("bounded summary is %d bytes", len(bounded))
	}
}
