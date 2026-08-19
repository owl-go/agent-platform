package gormrepo_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sourceapplication "agent-platform/backend/internal/biz/sourcecontrol/application"
	sourcedomain "agent-platform/backend/internal/biz/sourcecontrol/domain"
	"agent-platform/backend/internal/data/sourcecontrol/bindingvalidator"
	sourcegorm "agent-platform/backend/internal/data/sourcecontrol/gormrepo"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type bindingFixture struct {
	organizationID  string
	teamID          string
	providerID      string
	sshCredential   string
	buildCredential string
	runtimeID       string
	modelID         string
}

func TestRepositoryBindingLifecycleAndTenantBoundary(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	fixture := seedBindingFixture(t, tx)
	now := time.Now().UTC()
	service := sourceapplication.NewBindingServiceWithDependencies(
		sourcegorm.New(tx), bindingvalidator.New(tx), fixedClock(now), fixedID(uuid.NewString()),
	)
	binding, err := service.Register(context.Background(), bindingCommand(fixture))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.Get(context.Background(), fixture.organizationID, fixture.teamID, binding.ID)
	if err != nil || loaded.RepositoryHost != "github.com" || loaded.Version != 1 {
		t.Fatalf("Get() = (%+v, %v)", loaded, err)
	}
	if _, err := service.Get(context.Background(), fixture.organizationID, uuid.NewString(), binding.ID); !errors.Is(err, sourcedomain.ErrBindingNotFound) {
		t.Fatalf("cross-Team read error = %v", err)
	}
	invalidCommand := bindingCommand(fixture)
	invalidCommand.Name = "invalid-policy-" + fixture.teamID
	invalidCommand.ModelBudget.MaxCostAmount = "0"
	invalidCommand.QualityCommands = nil
	invalidService := sourceapplication.NewBindingServiceWithDependencies(sourcegorm.New(tx), bindingvalidator.New(tx), fixedClock(now), fixedID(uuid.NewString()))
	invalidBinding, err := invalidService.Register(context.Background(), invalidCommand)
	if err != nil {
		t.Fatal(err)
	}
	invalidBinding, err = invalidService.Validate(context.Background(), fixture.organizationID, fixture.teamID, invalidBinding.ID, 1)
	if err != nil || invalidBinding.ValidationReport == nil || invalidBinding.ValidationReport.Errors["model_budget"] == "" || invalidBinding.ValidationReport.Errors["quality_commands"] == "" {
		t.Fatalf("field-scoped policy Validation Report = (%+v, %v)", invalidBinding.ValidationReport, err)
	}
	validated, err := service.Validate(context.Background(), fixture.organizationID, fixture.teamID, binding.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if validated.ValidationReport == nil || validated.ValidationReport.Valid || validated.ValidationReport.Errors["allowed_runtime_image_ids"] == "" {
		t.Fatalf("experimental Runtime should block validation: %+v", validated.ValidationReport)
	}
	if err := tx.Exec("UPDATE runtime_images SET status = 'production', capabilities = '{}', conformance_evidence_key = 'phase-0/test/evidence.tar', conformance_evidence_sha256 = ? WHERE id = ?", strings.Repeat("a", 64), fixture.runtimeID).Error; err != nil {
		t.Fatal(err)
	}
	validated, err = service.Validate(context.Background(), fixture.organizationID, fixture.teamID, binding.ID, validated.Version)
	if err != nil {
		t.Fatal(err)
	}
	if validated.ValidationReport == nil || validated.ValidationReport.Valid || validated.ValidationReport.Errors["required_runtime_capabilities"] == "" {
		t.Fatalf("missing required Runtime Capability should block validation: %+v", validated.ValidationReport)
	}
	if err := tx.Exec("UPDATE runtime_images SET capabilities = '{\"streaming\":true}' WHERE id = ?", fixture.runtimeID).Error; err != nil {
		t.Fatal(err)
	}
	validated, err = service.Validate(context.Background(), fixture.organizationID, fixture.teamID, binding.ID, validated.Version)
	if err != nil {
		t.Fatal(err)
	}
	if validated.ValidationReport == nil || !validated.ValidationReport.Valid || len(validated.ValidationReport.Errors) != 0 {
		t.Fatalf("valid dependencies should pass validation: %+v", validated.ValidationReport)
	}
	if err := tx.Exec("UPDATE credential_profiles SET disabled_at = now() WHERE id = ?", fixture.sshCredential).Error; err != nil {
		t.Fatal(err)
	}
	validated, err = service.Validate(context.Background(), fixture.organizationID, fixture.teamID, binding.ID, validated.Version)
	if err != nil {
		t.Fatal(err)
	}
	if validated.ValidationReport == nil || validated.ValidationReport.Valid || validated.ValidationReport.Errors["ssh_credential_profile_id"] == "" {
		t.Fatalf("disabled SSH Credential Profile should invalidate Binding: %+v", validated.ValidationReport)
	}
	update := bindingCommand(fixture)
	update.Instructions = "Updated instructions invalidate the previous report."
	updated, err := service.Update(context.Background(), sourceapplication.UpdateBindingCommand{
		ID: binding.ID, ExpectedVersion: validated.Version, RegisterBindingCommand: update,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 6 || updated.ValidationReport != nil || updated.ValidatedAt != nil {
		t.Fatalf("updated Binding retained stale validation: %+v", updated)
	}

	other := seedBindingFixture(t, tx)
	command := bindingCommand(fixture)
	command.Name = "cross-organization"
	command.SSHCredentialProfileID = other.sshCredential
	if _, err := service.Register(context.Background(), command); !errors.Is(err, sourcedomain.ErrInvalidBinding) {
		t.Fatalf("cross-Organization Credential error = %v, want ErrInvalidBinding", err)
	}
}

func bindingCommand(fixture bindingFixture) sourceapplication.RegisterBindingCommand {
	return sourceapplication.RegisterBindingCommand{
		OrganizationID: fixture.organizationID, TeamID: fixture.teamID,
		SourceControlProviderID: fixture.providerID, Name: "repository-" + fixture.teamID,
		RepositorySSHURL: "git@github.com:acme/repository.git", DefaultBranch: "main",
		SSHCredentialProfileID: fixture.sshCredential, BuildCredentialProfileIDs: []string{fixture.buildCredential},
		GitAuthorName: "Agent Platform", GitAuthorEmail: "agent@example.test",
		AllowedRuntimeImageIDs: []string{fixture.runtimeID}, DefaultRuntimeImageID: fixture.runtimeID,
		RequiredRuntimeCapabilities: []string{"streaming"},
		DefaultModelID:              fixture.modelID,
		ModelBudget:                 sourcedomain.ModelBudget{MaxInputTokens: 1000, MaxOutputTokens: 500, MaxCostAmount: "10.00"},
		Instructions:                "Follow repository instructions.",
		QualityCommands:             []sourcedomain.QualityCommand{{Name: "test", Kind: sourcedomain.QualityTest, Executable: "go", Arguments: []string{"test", "./..."}, TimeoutSeconds: 600}},
		EgressPolicy:                sourcedomain.EgressPolicy{Mode: "public"},
	}
}

func seedBindingFixture(t *testing.T, db *gorm.DB) bindingFixture {
	t.Helper()
	suffix := fmt.Sprintf("binding-%d-%s", time.Now().UnixNano(), uuid.NewString()[:8])
	fixture := bindingFixture{
		organizationID: uuid.NewString(), teamID: uuid.NewString(), providerID: uuid.NewString(),
		sshCredential: uuid.NewString(), buildCredential: uuid.NewString(), runtimeID: uuid.NewString(), modelID: uuid.NewString(),
	}
	modelCredential := uuid.NewString()
	digest := fmt.Sprintf("registry.example/agent-platform/claude@sha256:%064x", time.Now().UnixNano())
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organizations (id, slug, name) VALUES (?, ?, ?)`, []any{fixture.organizationID, suffix, suffix}},
		{`INSERT INTO teams (id, organization_id, slug, name) VALUES (?, ?, ?, ?)`, []any{fixture.teamID, fixture.organizationID, suffix, suffix}},
		{`INSERT INTO source_control_providers (id, organization_id, name, kind, base_url) VALUES (?, ?, ?, 'github_com', 'https://github.com')`, []any{fixture.providerID, fixture.organizationID, suffix}},
		{`INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES (?, ?, ?, ?, 'git_ssh', 'secret://git')`, []any{fixture.sshCredential, fixture.organizationID, fixture.teamID, suffix + "-ssh"}},
		{`INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES (?, ?, ?, ?, 'build', 'secret://build')`, []any{fixture.buildCredential, fixture.organizationID, fixture.teamID, suffix + "-build"}},
		{`INSERT INTO credential_profiles (id, organization_id, name, kind, secret_ref) VALUES (?, ?, ?, 'model', 'secret://model')`, []any{modelCredential, fixture.organizationID, suffix + "-model"}},
		{`INSERT INTO configured_models (id, organization_id, name, model_id, endpoint, credential_profile_id) VALUES (?, ?, ?, 'model', 'https://model.example.test', ?)`, []any{fixture.modelID, fixture.organizationID, suffix, modelCredential}},
		{`INSERT INTO runtime_images (id, organization_id, runtime, cli_version, adapter_version, image_digest, capabilities, status) VALUES (?, ?, 'claude', '1', '1', ?, '{"streaming":true}', 'experimental')`, []any{fixture.runtimeID, fixture.organizationID, digest}},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatalf("seed Repository Binding fixture: %v", err)
		}
	}
	return fixture
}
