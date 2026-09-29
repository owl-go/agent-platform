package workspace

import (
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestExecutionEvidenceProjectsToSessionSnapshotAndHistory(t *testing.T) {
	evidence := domain.Evidence{ID: "evidence-1", Kind: "knowledge", SourceID: "document-1", SourceName: "Guide", ContainerID: "base-1", State: "succeeded", Action: "retrieved indexed source", StagePosition: 2, Citation: &domain.EvidenceCitation{RevisionID: "revision-1", SourceLocation: "Manual/page 2", Relevance: .9}}
	message := domain.Message{ID: 1, Role: "assistant", State: "completed", CreatedAt: time.Now(), Evidence: []domain.Evidence{evidence}}

	snapshot := snapshotOf(message)
	response := messageResponse(message)
	if len(snapshot.Evidence) != 1 || snapshot.Evidence[0].Citation == nil || snapshot.Evidence[0].Citation.RevisionID != "revision-1" {
		t.Fatalf("snapshot Evidence = %#v", snapshot.Evidence)
	}
	if len(response.Evidence) != 1 || response.Evidence[0].Citation == nil || response.Evidence[0].Citation.RevisionId != "revision-1" || response.Evidence[0].StagePosition != 2 {
		t.Fatalf("message Evidence = %#v", response.Evidence)
	}
}

func TestExecutionEvidenceProjectsToRunHistory(t *testing.T) {
	run := domain.Run{ID: "run-1", WorkflowID: "workflow-1", State: "succeeded", QueuedAt: time.Now(), Evidence: []domain.Evidence{{ID: "evidence-1", Kind: "connector", SourceID: "crm", SourceName: "CRM", State: "failed", Action: "contact.search", StagePosition: 1}}}

	response := runResponse(run)
	if len(response.Evidence) != 1 || response.Evidence[0].SourceId != "crm" || response.Evidence[0].State != "failed" {
		t.Fatalf("Run Evidence = %#v", response.Evidence)
	}
}
