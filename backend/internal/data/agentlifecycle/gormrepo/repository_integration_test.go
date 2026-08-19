package gormrepo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/agentlifecycle/application"
	"agent-platform/backend/internal/biz/agentlifecycle/domain"
	"agent-platform/backend/internal/data/agentlifecycle/draftvalidator"
	"agent-platform/backend/internal/data/agentlifecycle/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fixedClock time.Time

func (clock fixedClock) Now() time.Time { return time.Time(clock) }

type randomIDs struct{}

func (randomIDs) NewID() string { return uuid.NewString() }

type lifecycleFixture struct {
	organizationID, teamID, builderID, reviewerID string
	bindingID, runtimeID, modelID                 string
}

func TestAgentLifecyclePersistsValidatedLowAndHighRiskReleases(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	fixture := seedLifecycleFixture(t, tx)
	now := time.Now().UTC()
	service := application.NewWithDependencies(gormrepo.New(tx), draftvalidator.New(tx), fixedClock(now), randomIDs{})

	agent, err := service.CreateAgent(context.Background(), application.CreateAgentCommand{OrganizationID: fixture.organizationID, TeamID: fixture.teamID, Name: "Coding Agent", Description: "Test Agent", CreatedBy: fixture.builderID})
	if err != nil {
		t.Fatal(err)
	}
	low, err := service.CreateDraft(context.Background(), application.CreateDraftCommand{OrganizationID: fixture.organizationID, TeamID: fixture.teamID, AgentID: agent.ID, CreatedBy: fixture.builderID, Configuration: lifecycleConfiguration(fixture, false), ReleaseRisk: domain.ReleaseRiskLow})
	if err != nil {
		t.Fatal(err)
	}
	ready, err := service.ValidateDraft(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, low.ID, low.Version)
	if err != nil || ready.State != domain.DraftStateReady {
		t.Fatalf("ValidateDraft() = (%+v, %v)", ready, err)
	}
	release, err := service.Publish(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, ready.ID, fixture.builderID)
	if err != nil || release.ReleaseNumber != 1 {
		t.Fatalf("Publish() = (%+v, %v)", release, err)
	}
	if _, err := service.Publish(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, ready.ID, fixture.builderID); !errors.Is(err, domain.ErrDraftAlreadyReleased) {
		t.Fatalf("duplicate Publish error = %v", err)
	}
	deprecated, err := service.DeprecateRelease(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, release.ID, release.Version)
	if err != nil || deprecated.Status != domain.ReleaseStatusDeprecated || deprecated.Version != 2 {
		t.Fatalf("DeprecateRelease() = (%+v, %v)", deprecated, err)
	}
	if _, err := service.DeprecateRelease(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, release.ID, release.Version); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("stale deprecation error = %v", err)
	}

	high, err := service.CreateDraft(context.Background(), application.CreateDraftCommand{OrganizationID: fixture.organizationID, TeamID: fixture.teamID, AgentID: agent.ID, CreatedBy: fixture.builderID, Configuration: lifecycleConfiguration(fixture, true), ReleaseRisk: domain.ReleaseRiskHigh})
	if err != nil || high.Revision != 2 {
		t.Fatalf("high-risk CreateDraft() = (%+v, %v)", high, err)
	}
	high, err = service.ValidateDraft(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, high.ID, high.Version)
	if err != nil || high.State != domain.DraftStateReady {
		t.Fatalf("high-risk ValidateDraft() = (%+v, %v)", high, err)
	}
	approval, err := service.RequestApproval(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, high.ID, fixture.builderID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.DecideApproval(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, high.ID, approval.Version, true, fixture.builderID, ""); !errors.Is(err, domain.ErrInvalidAgent) {
		t.Fatalf("self approval error = %v", err)
	}
	approval, err = service.DecideApproval(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, high.ID, approval.Version, true, fixture.reviewerID, "reviewed")
	if err != nil || approval.State != domain.ApprovalApproved {
		t.Fatalf("DecideApproval() = (%+v, %v)", approval, err)
	}
	highRelease, err := service.Publish(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, high.ID, fixture.builderID)
	if err != nil || highRelease.ReleaseNumber != 2 {
		t.Fatalf("high-risk Publish() = (%+v, %v)", highRelease, err)
	}
	blocked, err := service.BlockRelease(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, highRelease.ID, highRelease.Version)
	if err != nil || blocked.Status != domain.ReleaseStatusBlocked || blocked.Version != 2 {
		t.Fatalf("BlockRelease() = (%+v, %v)", blocked, err)
	}
	if _, err := service.GetAgent(context.Background(), fixture.organizationID, uuid.NewString(), agent.ID); !errors.Is(err, domain.ErrAgentNotFound) {
		t.Fatalf("cross-Team GetAgent error = %v", err)
	}

	overBudgetConfiguration := lifecycleConfiguration(fixture, false)
	overBudgetConfiguration.ModelBudget.MaxInputTokens = 2001
	overBudget, err := service.CreateDraft(context.Background(), application.CreateDraftCommand{OrganizationID: fixture.organizationID, TeamID: fixture.teamID, AgentID: agent.ID, CreatedBy: fixture.builderID, Configuration: overBudgetConfiguration, ReleaseRisk: domain.ReleaseRiskLow})
	if err != nil {
		t.Fatal(err)
	}
	overBudget, err = service.ValidateDraft(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, overBudget.ID, overBudget.Version)
	if err != nil || overBudget.State != domain.DraftStateBlocked || overBudget.ValidationReport == nil || overBudget.ValidationReport.Errors["model_budget"] == "" {
		t.Fatalf("over-budget ValidateDraft() = (%+v, %v)", overBudget, err)
	}
	edited, err := service.EditDraft(context.Background(), application.EditDraftCommand{OrganizationID: fixture.organizationID, TeamID: fixture.teamID, AgentID: agent.ID, DraftID: overBudget.ID, Configuration: lifecycleConfiguration(fixture, false), ReleaseRisk: domain.ReleaseRiskLow, ExpectedVersion: overBudget.Version})
	if err != nil || edited.State != domain.DraftStateDraft || edited.ValidationReport != nil || edited.Version != overBudget.Version+1 {
		t.Fatalf("EditDraft() did not clear Validation Report = (%+v, %v)", edited, err)
	}

	if err := tx.Exec(`UPDATE runtime_images SET capabilities = '{"subagents":false}'::jsonb WHERE id = ?`, fixture.runtimeID).Error; err != nil {
		t.Fatal(err)
	}
	missingCapability, err := service.CreateDraft(context.Background(), application.CreateDraftCommand{OrganizationID: fixture.organizationID, TeamID: fixture.teamID, AgentID: agent.ID, CreatedBy: fixture.builderID, Configuration: lifecycleConfiguration(fixture, true), ReleaseRisk: domain.ReleaseRiskHigh})
	if err != nil {
		t.Fatal(err)
	}
	missingCapability, err = service.ValidateDraft(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, missingCapability.ID, missingCapability.Version)
	if err != nil || missingCapability.State != domain.DraftStateBlocked || missingCapability.ValidationReport == nil || missingCapability.ValidationReport.Errors["native_subagents"] == "" {
		t.Fatalf("missing-capability ValidateDraft() = (%+v, %v)", missingCapability, err)
	}

	invalidBindingReport := fmt.Sprintf(`{"valid":false,"errors":{"egress_policy":"Egress Policy is invalid","quality_commands":"quality command is invalid"},"checked_at":%q}`, now.Format(time.RFC3339Nano))
	if err := tx.Exec(`UPDATE repository_bindings SET validation_report = ?::jsonb WHERE id = ?`, invalidBindingReport, fixture.bindingID).Error; err != nil {
		t.Fatal(err)
	}
	bindingBlocked, err := service.CreateDraft(context.Background(), application.CreateDraftCommand{OrganizationID: fixture.organizationID, TeamID: fixture.teamID, AgentID: agent.ID, CreatedBy: fixture.builderID, Configuration: lifecycleConfiguration(fixture, false), ReleaseRisk: domain.ReleaseRiskLow})
	if err != nil {
		t.Fatal(err)
	}
	bindingBlocked, err = service.ValidateDraft(context.Background(), fixture.organizationID, fixture.teamID, agent.ID, bindingBlocked.ID, bindingBlocked.Version)
	if err != nil || bindingBlocked.ValidationReport == nil || bindingBlocked.ValidationReport.Errors["egress_policy"] == "" || bindingBlocked.ValidationReport.Errors["quality_commands"] == "" {
		t.Fatalf("Binding field errors were not preserved by ValidateDraft() = (%+v, %v)", bindingBlocked, err)
	}
}

func lifecycleConfiguration(fixture lifecycleFixture, subagents bool) domain.Configuration {
	return domain.Configuration{Instructions: "Implement and validate the requested change.", RepositoryBindingID: fixture.bindingID, RuntimeImageID: fixture.runtimeID, ConfiguredModelID: fixture.modelID, NativeSubagents: subagents, ModelBudget: domain.ModelBudget{MaxInputTokens: 1000, MaxOutputTokens: 500, MaxCostAmount: "10.00"}, ExecutionLimits: domain.ExecutionLimits{TimeoutSeconds: 1800, CPUs: 2, MemoryBytes: 4 << 30, PIDs: 256, TempBytes: 10 << 30, Egress: "public"}}
}

func seedLifecycleFixture(t *testing.T, db *gorm.DB) lifecycleFixture {
	t.Helper()
	suffix := fmt.Sprintf("lifecycle-%d", time.Now().UnixNano())
	fixture := lifecycleFixture{organizationID: uuid.NewString(), teamID: uuid.NewString(), builderID: uuid.NewString(), reviewerID: uuid.NewString(), bindingID: uuid.NewString(), runtimeID: uuid.NewString(), modelID: uuid.NewString()}
	providerID, sshCredentialID, modelCredentialID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	digest := fmt.Sprintf("registry.example/agent-platform/claude@sha256:%064x", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339Nano)
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organizations (id, slug, name) VALUES (?, ?, ?)`, []any{fixture.organizationID, suffix, suffix}},
		{`INSERT INTO teams (id, organization_id, slug, name) VALUES (?, ?, ?, ?)`, []any{fixture.teamID, fixture.organizationID, suffix, suffix}},
		{`INSERT INTO users (id, organization_id, oidc_subject, email, display_name) VALUES (?, ?, ?, ?, 'Builder'), (?, ?, ?, ?, 'Reviewer')`, []any{fixture.builderID, fixture.organizationID, suffix + "-builder", suffix + "-builder@example.test", fixture.reviewerID, fixture.organizationID, suffix + "-reviewer", suffix + "-reviewer@example.test"}},
		{`INSERT INTO source_control_providers (id, organization_id, name, kind, base_url) VALUES (?, ?, ?, 'github_com', 'https://github.com')`, []any{providerID, fixture.organizationID, suffix}},
		{`INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES (?, ?, ?, ?, 'git_ssh', 'secret://git')`, []any{sshCredentialID, fixture.organizationID, fixture.teamID, suffix + "-ssh"}},
		{`INSERT INTO credential_profiles (id, organization_id, name, kind, secret_ref) VALUES (?, ?, ?, 'model', 'secret://model')`, []any{modelCredentialID, fixture.organizationID, suffix + "-model"}},
		{`INSERT INTO configured_models (id, organization_id, name, model_id, endpoint, credential_profile_id) VALUES (?, ?, ?, 'model', 'https://model.example.test', ?)`, []any{fixture.modelID, fixture.organizationID, suffix, modelCredentialID}},
		{`INSERT INTO runtime_images (id, organization_id, runtime, cli_version, adapter_version, image_digest, capabilities, status, conformance_evidence_key, conformance_evidence_sha256) VALUES (?, ?, 'claude', '1', '1', ?, '{"subagents":true}', 'production', 'test/lifecycle/evidence.tar', ?)`, []any{fixture.runtimeID, fixture.organizationID, digest, strings.Repeat("e", 64)}},
		{`INSERT INTO repository_bindings (id, organization_id, team_id, source_control_provider_id, name, repository_ssh_url, default_branch, ssh_credential_profile_id, git_author_name, git_author_email, allowed_runtime_image_ids, default_runtime_image_id, default_model_id, model_budget, instructions, quality_commands, egress_policy, validation_report, validated_at) VALUES (?, ?, ?, ?, ?, 'git@github.com:acme/repository.git', 'main', ?, 'Agent', 'agent@example.test', ?::jsonb, ?, ?, '{"max_input_tokens":2000,"max_output_tokens":1000,"max_cost_amount":"20.00"}', '', '[]', '{"mode":"public"}', ?::jsonb, ?)`, []any{fixture.bindingID, fixture.organizationID, fixture.teamID, providerID, suffix, sshCredentialID, `["` + fixture.runtimeID + `"]`, fixture.runtimeID, fixture.modelID, `{"valid":true,"errors":{},"checked_at":"` + now + `"}`, now}},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatalf("seed Agent Lifecycle fixture: %v", err)
		}
	}
	return fixture
}

func openIntegrationDatabase(t *testing.T) *gormdb.Database {
	t.Helper()
	dsn := os.Getenv("EXECUTION_DATABASE_DSN")
	if dsn == "" {
		t.Skip("EXECUTION_DATABASE_DSN is required for PostgreSQL integration")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{DSN: dsn, MaxOpenConnections: 5, MaxIdleConnections: 2, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
