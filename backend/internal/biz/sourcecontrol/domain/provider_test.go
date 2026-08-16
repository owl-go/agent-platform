package domain

import (
	"testing"
	"time"
)

func TestRegisterSourceControlProviders(t *testing.T) {
	now := time.Now().UTC()
	github, err := Register(Registration{ID: "p1", OrganizationID: "o", Name: "GitHub", Kind: GitHubCom, BaseURL: "https://github.com/", Now: now})
	if err != nil || github.BaseURL != "https://github.com" {
		t.Fatalf("GitHub Register() = (%+v, %v)", github, err)
	}
	gitlab, err := Register(Registration{ID: "p2", OrganizationID: "o", Name: "GitLab", Kind: GitLabSelfManaged, BaseURL: "https://gitlab.example.test/", Now: now})
	if err != nil || gitlab.BaseURL != "https://gitlab.example.test" {
		t.Fatalf("GitLab Register() = (%+v, %v)", gitlab, err)
	}
	if err := gitlab.SetEnabled(false, now.Add(time.Minute)); err != nil || gitlab.Enabled || gitlab.Version != 2 {
		t.Fatalf("disabled GitLab = (%+v, %v)", gitlab, err)
	}
}

func TestProviderRejectsUnsupportedOrigins(t *testing.T) {
	now := time.Now().UTC()
	for _, registration := range []Registration{
		{ID: "p", OrganizationID: "o", Name: "bad", Kind: GitHubCom, BaseURL: "https://github.example.test", Now: now},
		{ID: "p", OrganizationID: "o", Name: "bad", Kind: GitLabSelfManaged, BaseURL: "http://gitlab.example.test", Now: now},
		{ID: "p", OrganizationID: "o", Name: "bad", Kind: "other", BaseURL: "https://git.example.test", Now: now},
	} {
		if _, err := Register(registration); err == nil {
			t.Fatalf("Register accepted %+v", registration)
		}
	}
}
