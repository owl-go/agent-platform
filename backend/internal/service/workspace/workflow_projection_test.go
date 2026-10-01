package workspace

import (
	"testing"
	"time"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

func TestWorkflowResponseIncludesOperationalSummary(t *testing.T) {
	lastRunAt := time.Date(2026, time.September, 29, 8, 0, 0, 0, time.UTC)
	nextRunAt := lastRunAt.Add(time.Hour)
	item := workspacedomain.Workflow{
		ID:                      "workflow-1",
		Name:                    "Daily report",
		Goal:                    "Summarize progress",
		CreatedAt:               lastRunAt.Add(-24 * time.Hour),
		UpdatedAt:               lastRunAt,
		Version:                 3,
		NextScheduledAt:         &nextRunAt,
		UpcomingScheduleTimes:   []time.Time{nextRunAt, nextRunAt.Add(24 * time.Hour)},
		LastRunID:               "run-2",
		LastRunState:            "failed",
		LastRunAt:               &lastRunAt,
		RunCount30Days:          5,
		SucceededRunCount30Days: 4,
		NeedsAttention:          true,
	}

	response := workflowResponse(item)
	if response.GetLastRunId() != "run-2" || response.GetLastRunState() != "failed" || response.GetRunCount_30D() != 5 || response.GetSucceededRunCount_30D() != 4 || !response.GetNeedsAttention() {
		t.Fatalf("operational summary = %#v", response)
	}
	if response.NextScheduledAt == nil || !response.NextScheduledAt.AsTime().Equal(nextRunAt) || len(response.UpcomingScheduleTimes) != 2 {
		t.Fatalf("schedule projection = %#v", response)
	}
}
