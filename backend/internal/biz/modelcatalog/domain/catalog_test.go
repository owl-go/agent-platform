package domain

import (
	"testing"
	"time"
)

func TestCredentialAndConfiguredModelRegistration(t *testing.T) {
	now := time.Now().UTC()
	credential, err := RegisterCredential(CredentialRegistration{
		ID: "credential-1", OrganizationID: "org-1", Name: "model-key", Kind: ModelCredential,
		SecretRef: "vault://agent-platform/model-key", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	model, err := RegisterModel(ModelRegistration{
		ID: "model-1", OrganizationID: "org-1", Name: "primary", ModelID: "model-name",
		Endpoint: "https://models.example.test/v1", Credential: credential, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Enabled || model.CredentialProfileID != credential.ID || model.Version != 1 {
		t.Fatalf("Configured Model = %+v", model)
	}
	if err := credential.SetEnabled(false, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if credential.Enabled() || credential.Version != 2 {
		t.Fatalf("disabled Credential Profile = %+v", credential)
	}
	if err := model.SetEnabled(false, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if model.Enabled || model.Version != 2 {
		t.Fatalf("disabled Configured Model = %+v", model)
	}
}

func TestCatalogRejectsSecretMaterialAndInvalidModelBindings(t *testing.T) {
	now := time.Now().UTC()
	for _, secretRef := range []string{"plain-api-key", "sk-secret value", "https://"} {
		if _, err := RegisterCredential(CredentialRegistration{
			ID: "credential", OrganizationID: "org", Name: "model", Kind: ModelCredential,
			SecretRef: secretRef, Now: now,
		}); err == nil {
			t.Fatalf("accepted Secret Ref %q", secretRef)
		}
	}
	credential, err := RegisterCredential(CredentialRegistration{
		ID: "credential", OrganizationID: "org", Name: "model", Kind: ModelCredential,
		SecretRef: "secret://model", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []ModelRegistration{
		{ID: "model", OrganizationID: "org", Name: "m", ModelID: "id", Endpoint: "http://models.example.test", Credential: credential, Now: now},
		{ID: "model", OrganizationID: "other", Name: "m", ModelID: "id", Endpoint: "https://models.example.test", Credential: credential, Now: now},
	}
	team := "team"
	teamCredential := credential
	teamCredential.TeamID = &team
	tests = append(tests, ModelRegistration{ID: "model", OrganizationID: "org", Name: "m", ModelID: "id", Endpoint: "https://models.example.test", Credential: teamCredential, Now: now})
	disabledCredential := credential
	_ = disabledCredential.SetEnabled(false, now.Add(time.Second))
	tests = append(tests, ModelRegistration{ID: "model", OrganizationID: "org", Name: "m", ModelID: "id", Endpoint: "https://models.example.test", Credential: disabledCredential, Now: now})
	for _, registration := range tests {
		if _, err := RegisterModel(registration); err == nil {
			t.Fatalf("RegisterModel accepted %+v", registration)
		}
	}
}
