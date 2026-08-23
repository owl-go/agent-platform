package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCodingTaskAndSessionLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	task, err := RegisterTask(TaskRegistration{ID: "task", OrganizationID: "org", TeamID: "team", AgentReleaseID: "release", CreatedBy: "user", Title: "Fix parser", RequestText: "Handle empty input", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := task.Activate(now); err != nil {
		t.Fatal(err)
	}
	if err := task.WaitForUser(now); err != nil {
		t.Fatal(err)
	}
	if err := task.Activate(now); err != nil {
		t.Fatal(err)
	}
	if err := task.Complete(now); err != nil || task.CompletedAt == nil {
		t.Fatalf("complete task: %v", err)
	}

	session, err := OpenSession(SessionRegistration{ID: "session", CodingTaskID: task.ID, RepositoryBindingID: "binding", TargetBranch: "main", ReviewBranch: "agent-platform/backend/task-123", WorkspaceVolume: "workspace-123", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for range MaximumSessionRuns {
		if err := session.AddRun(now); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.AddRun(now); !errors.Is(err, ErrRunLimitReached) {
		t.Fatalf("expected Run limit, got %v", err)
	}
}

func TestMemoryCandidateRequiresExplicitApproval(t *testing.T) {
	now := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	candidate, err := ProposeMemory("candidate", "agent", "task", "Run the parser tests before committing.", now)
	if err != nil {
		t.Fatal(err)
	}
	memory, err := candidate.Approve("memory", "user", now)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.State != MemoryCandidateApproved || !memory.Enabled || memory.Content != candidate.ProposedContent {
		t.Fatalf("unexpected approved memory: %#v %#v", candidate, memory)
	}
	if _, err := candidate.Approve("other", "user", now); err == nil {
		t.Fatal("approved candidate must not be approved twice")
	}
}

func TestSessionRejectsUnsafeReviewBranch(t *testing.T) {
	_, err := OpenSession(SessionRegistration{ID: "session", CodingTaskID: "task", RepositoryBindingID: "binding", TargetBranch: "main", ReviewBranch: "refs/heads/bad..branch", WorkspaceVolume: "workspace", Now: time.Now()})
	if err == nil {
		t.Fatal("expected unsafe branch rejection")
	}
}

func TestCodingTaskRejectsUntrustedIssueSnapshotURL(t *testing.T) {
	_, err := RegisterTask(TaskRegistration{
		ID: "task", OrganizationID: "org", TeamID: "team", AgentReleaseID: "release", CreatedBy: "user",
		Title: "Unsafe issue", RequestText: "Do not execute this URL.",
		IssueSnapshot: &IssueSnapshot{Title: "Unsafe issue", Body: "body", URL: "javascript:alert(1)"}, Now: time.Now(),
	})
	if err == nil {
		t.Fatal("expected non-HTTPS Issue Snapshot URL rejection")
	}
	if _, err := RegisterTask(TaskRegistration{
		ID: "task", OrganizationID: "org", TeamID: "team", AgentReleaseID: "release", CreatedBy: "user",
		Title: "Safe issue", RequestText: "Use the immutable snapshot.",
		IssueSnapshot: &IssueSnapshot{Title: "Safe issue", Body: "body", URL: "https://git.example.test/issues/42"}, Now: time.Now(),
	}); err != nil {
		t.Fatalf("expected HTTPS Issue Snapshot URL: %v", err)
	}
}
