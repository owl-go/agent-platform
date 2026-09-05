package gormrepo

import (
	"encoding/json"
	"testing"
)

func TestDeletionImpactBindsAffectedExpertVersions(t *testing.T) {
	ids, _ := json.Marshal([]string{"skill-1"})
	experts := []expertRecord{{ID: "expert-1", Name: "Reviewer", Version: 3, SkillIDs: ids}}
	first, err := deletionImpact("skill", "skill-1", 2, experts)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.AffectedExperts) != 1 || first.AffectedExperts[0].Name != "Reviewer" {
		t.Fatalf("affected Experts = %#v", first.AffectedExperts)
	}
	experts[0].Version++
	changed, err := deletionImpact("skill", "skill-1", 2, experts)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfirmationToken == changed.ConfirmationToken {
		t.Fatal("confirmation token did not bind the affected Expert version")
	}
}
