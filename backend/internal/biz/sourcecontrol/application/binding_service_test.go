package application

import (
	"context"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/sourcecontrol/domain"
)

type bindingRepositoryStub struct{ binding domain.RepositoryBinding }

func (repository *bindingRepositoryStub) CreateBinding(_ context.Context, binding domain.RepositoryBinding) error {
	repository.binding = binding
	return nil
}
func (repository *bindingRepositoryStub) GetBinding(context.Context, string, string, string) (domain.RepositoryBinding, error) {
	return repository.binding, nil
}
func (*bindingRepositoryStub) ListBindings(context.Context, string, string) ([]domain.RepositoryBinding, error) {
	return nil, nil
}
func (repository *bindingRepositoryStub) UpdateBinding(_ context.Context, binding domain.RepositoryBinding, _ int64) error {
	repository.binding = binding
	return nil
}
func (repository *bindingRepositoryStub) UpdateBindingValidation(_ context.Context, binding domain.RepositoryBinding, _ int64) error {
	repository.binding = binding
	return nil
}

type bindingValidatorStub struct{ errors map[string]string }

func (bindingValidatorStub) CheckReferences(context.Context, domain.RepositoryBinding) error {
	return nil
}
func (validator bindingValidatorStub) Validate(context.Context, domain.RepositoryBinding) (map[string]string, error) {
	return validator.errors, nil
}

type bindingFixedClock time.Time

func (clock bindingFixedClock) Now() time.Time { return time.Time(clock) }

type bindingFixedID string

func (id bindingFixedID) NewID() string { return string(id) }

func TestBindingServiceRegistersAndValidates(t *testing.T) {
	now := time.Now().UTC()
	repository := &bindingRepositoryStub{}
	service := NewBindingServiceWithDependencies(repository, bindingValidatorStub{}, bindingFixedClock(now), bindingFixedID("binding-1"))
	binding, err := service.Register(context.Background(), validBindingCommand())
	if err != nil {
		t.Fatal(err)
	}
	validated, err := service.Validate(context.Background(), binding.OrganizationID, binding.TeamID, binding.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if validated.ValidationReport == nil || !validated.ValidationReport.Valid || validated.Version != 2 {
		t.Fatalf("unexpected validated Binding: %+v", validated)
	}
}

func validBindingCommand() RegisterBindingCommand {
	return RegisterBindingCommand{
		OrganizationID: "organization", TeamID: "team", SourceControlProviderID: "provider", Name: "repository",
		RepositorySSHURL: "git@github.com:acme/repository.git", DefaultBranch: "main",
		SSHCredentialProfileID: "ssh-credential", GitAuthorName: "Agent", GitAuthorEmail: "agent@example.test",
		AllowedRuntimeImageIDs: []string{"runtime"}, DefaultRuntimeImageID: "runtime", DefaultModelID: "model",
		ModelBudget:     domain.ModelBudget{MaxInputTokens: 1000, MaxOutputTokens: 500, MaxCostAmount: "10.00"},
		QualityCommands: []domain.QualityCommand{{Name: "test", Kind: domain.QualityTest, Executable: "go", Arguments: []string{"test", "./..."}, TimeoutSeconds: 600}},
		EgressPolicy:    domain.EgressPolicy{Mode: "public"},
	}
}
