package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDraftMustValidateBeforeRelease(t *testing.T) {
	now := time.Now().UTC()
	draft, err := CreateDraft(DraftRegistration{ID: "draft", AgentID: "agent", Revision: 1, Configuration: validConfiguration(), ReleaseRisk: ReleaseRiskLow, CreatedBy: "builder", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(ReleaseRegistration{ID: "release", ReleaseNumber: 1, Draft: draft, ReleasedBy: "builder", Now: now}); !errors.Is(err, ErrDraftNotReady) {
		t.Fatalf("unvalidated Publish error = %v", err)
	}
	if err := draft.StartValidation(now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := draft.FinishValidation(ValidationReport{Valid: true, Errors: map[string]string{}, CheckedAt: now.Add(2 * time.Second)}, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	release, err := Publish(ReleaseRegistration{ID: "release", ReleaseNumber: 1, Draft: draft, ReleasedBy: "builder", Now: now.Add(3 * time.Second)})
	if err != nil || release.Status != ReleaseStatusReleased || release.Configuration.RuntimeImageID != "runtime" {
		t.Fatalf("Publish() = (%+v, %v)", release, err)
	}
}

func TestHighRiskReleaseRequiresDifferentApprover(t *testing.T) {
	now := time.Now().UTC()
	draft, err := CreateDraft(DraftRegistration{ID: "draft", AgentID: "agent", Revision: 1, Configuration: validConfiguration(), ReleaseRisk: ReleaseRiskHigh, CreatedBy: "builder", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	_ = draft.StartValidation(now.Add(time.Second))
	_ = draft.FinishValidation(ValidationReport{Valid: true, Errors: map[string]string{}, CheckedAt: now.Add(2 * time.Second)}, now.Add(2*time.Second))
	for _, approval := range []*RiskApproval{
		nil,
		{ID: "approval", DraftID: draft.ID, DraftVersion: draft.Version, RequestedBy: "builder", ApprovedBy: "builder", ApprovedAt: now},
	} {
		if _, err := Publish(ReleaseRegistration{ID: "release", ReleaseNumber: 1, Draft: draft, ReleasedBy: "builder", Approval: approval, Now: now.Add(3 * time.Second)}); !errors.Is(err, ErrApprovalRequired) {
			t.Fatalf("Publish approval error = %v", err)
		}
	}
	approval := &RiskApproval{ID: "approval", DraftID: draft.ID, DraftVersion: draft.Version, RequestedBy: "builder", ApprovedBy: "reviewer", ApprovedAt: now}
	if _, err := Publish(ReleaseRegistration{ID: "release", ReleaseNumber: 1, Draft: draft, ReleasedBy: "builder", Approval: approval, Now: now.Add(3 * time.Second)}); err != nil {
		t.Fatal(err)
	}
}

func TestEditingDraftClearsValidation(t *testing.T) {
	now := time.Now().UTC()
	draft, _ := CreateDraft(DraftRegistration{ID: "draft", AgentID: "agent", Revision: 1, Configuration: validConfiguration(), ReleaseRisk: ReleaseRiskLow, CreatedBy: "builder", Now: now})
	_ = draft.StartValidation(now.Add(time.Second))
	_ = draft.FinishValidation(ValidationReport{Valid: true, Errors: map[string]string{}, CheckedAt: now.Add(2 * time.Second)}, now.Add(2*time.Second))
	configuration := validConfiguration()
	configuration.Instructions = "updated"
	if err := draft.Edit(configuration, ReleaseRiskLow, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if draft.State != DraftStateDraft || draft.ValidationReport != nil {
		t.Fatalf("edited Draft retained validation: %+v", draft)
	}
}

func TestEditingHighRiskDraftInvalidatesApproval(t *testing.T) {
	now := time.Now().UTC()
	draft, _ := CreateDraft(DraftRegistration{ID: "draft", AgentID: "agent", Revision: 1, Configuration: validConfiguration(), ReleaseRisk: ReleaseRiskHigh, CreatedBy: "builder", Now: now})
	_ = draft.StartValidation(now.Add(time.Second))
	_ = draft.FinishValidation(ValidationReport{Valid: true, Errors: map[string]string{}, CheckedAt: now.Add(2 * time.Second)}, now.Add(2*time.Second))
	approval := &RiskApproval{ID: "approval", DraftID: draft.ID, DraftVersion: draft.Version, RequestedBy: "builder", ApprovedBy: "reviewer", ApprovedAt: now.Add(3 * time.Second)}
	configuration := validConfiguration()
	configuration.Instructions = "materially changed after approval"
	_ = draft.Edit(configuration, ReleaseRiskHigh, now.Add(4*time.Second))
	_ = draft.StartValidation(now.Add(5 * time.Second))
	_ = draft.FinishValidation(ValidationReport{Valid: true, Errors: map[string]string{}, CheckedAt: now.Add(6 * time.Second)}, now.Add(6*time.Second))
	if _, err := Publish(ReleaseRegistration{ID: "release", ReleaseNumber: 1, Draft: draft, ReleasedBy: "builder", Approval: approval, Now: now.Add(7 * time.Second)}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("Publish with stale Approval error = %v", err)
	}
}

func validConfiguration() Configuration {
	return Configuration{
		Instructions: "Implement repository changes.", RepositoryBindingID: "binding", RuntimeImageID: "runtime", ConfiguredModelID: "model",
		ModelBudget:     ModelBudget{MaxInputTokens: 1000, MaxOutputTokens: 500, MaxCostAmount: "10.00"},
		ExecutionLimits: ExecutionLimits{TimeoutSeconds: 1800, CPUs: 2, MemoryBytes: 4 << 30, PIDs: 256, TempBytes: 10 << 30, Egress: "public"},
	}
}
