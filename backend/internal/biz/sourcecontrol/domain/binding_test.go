package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRegisterRepositoryBinding(t *testing.T) {
	binding, err := RegisterBinding(validBindingRegistration())
	if err != nil {
		t.Fatal(err)
	}
	if binding.RepositoryHost != "github.com" || binding.Version != 1 || binding.DefaultRuntimeImageID != "runtime-1" {
		t.Fatalf("unexpected Repository Binding: %+v", binding)
	}
	checkedAt := time.Now().UTC().Add(time.Minute)
	if err := binding.RecordValidation(ValidationReport{Valid: true, Errors: map[string]string{}, CheckedAt: checkedAt}); err != nil {
		t.Fatal(err)
	}
	if binding.ValidationReport == nil || !binding.ValidationReport.Valid || binding.Version != 2 {
		t.Fatalf("validation was not recorded: %+v", binding)
	}
}

func TestRepositoryBindingRejectsUnsafeConfiguration(t *testing.T) {
	tests := []func(*BindingRegistration){
		func(value *BindingRegistration) { value.RepositorySSHURL = "https://github.com/acme/repo.git" },
		func(value *BindingRegistration) { value.RepositorySSHURL = "ssh://root@github.com/acme/repo.git" },
		func(value *BindingRegistration) { value.DefaultBranch = "../main" },
		func(value *BindingRegistration) { value.DefaultRuntimeImageID = "runtime-2" },
		func(value *BindingRegistration) { value.ModelBudget.MaxCostAmount = "0" },
		func(value *BindingRegistration) { value.QualityCommands[0].Executable = "sh -c" },
		func(value *BindingRegistration) { value.EgressPolicy.Mode = "private" },
	}
	for index, mutate := range tests {
		registration := validBindingRegistration()
		mutate(&registration)
		if _, err := RegisterBinding(registration); !errors.Is(err, ErrInvalidBinding) {
			t.Fatalf("case %d error = %v, want ErrInvalidBinding", index, err)
		}
	}
}

func TestRepositoryBindingAcceptsGitLabSCPURL(t *testing.T) {
	registration := validBindingRegistration()
	registration.RepositorySSHURL = "git@gitlab.example.test:group/repository.git"
	binding, err := RegisterBinding(registration)
	if err != nil || binding.RepositoryHost != "gitlab.example.test" {
		t.Fatalf("RegisterBinding() = (%+v, %v)", binding, err)
	}
}

func validBindingRegistration() BindingRegistration {
	return BindingRegistration{
		ID: "binding-1", OrganizationID: "organization-1", TeamID: "team-1",
		SourceControlProviderID: "provider-1", Name: "agent-platform",
		RepositorySSHURL: "ssh://git@github.com/acme/agent-platform.git", DefaultBranch: "main",
		SSHCredentialProfileID: "credential-1", BuildCredentialProfileIDs: []string{"credential-2"},
		GitAuthorName: "Agent Platform", GitAuthorEmail: "agent@example.test",
		AllowedRuntimeImageIDs: []string{"runtime-1"}, DefaultRuntimeImageID: "runtime-1",
		DefaultModelID:  "model-1",
		ModelBudget:     ModelBudget{MaxInputTokens: 100_000, MaxOutputTokens: 20_000, MaxCostAmount: "50.00"},
		Instructions:    "Follow repository instructions.",
		QualityCommands: []QualityCommand{{Name: "test", Kind: QualityTest, Executable: "go", Arguments: []string{"test", "./..."}, TimeoutSeconds: 600}},
		EgressPolicy:    EgressPolicy{Mode: "public"}, Now: time.Now().UTC(),
	}
}
