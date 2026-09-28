package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateEvidenceAcceptsBoundedKnowledgeCitation(t *testing.T) {
	items := []Evidence{{ID: "evidence-1", Kind: "knowledge", SourceID: "document-1", SourceName: "Policy.pdf", ContainerID: "base-1", State: "succeeded", Action: "retrieved", StagePosition: 1, Citation: &EvidenceCitation{RevisionID: "revision-1", SourceLocation: "page 2", Relevance: .8}}}
	if err := ValidateEvidence(items); err != nil {
		t.Fatalf("ValidateEvidence() error = %v", err)
	}
	payload, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"raw_output", "prompt", "credential"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("bounded Evidence leaked forbidden field %q: %s", forbidden, payload)
		}
	}
}

func TestValidateEvidenceRejectsIncompleteOrUnboundedEvidence(t *testing.T) {
	valid := Evidence{ID: "evidence-1", Kind: "connector", SourceID: "connector-1", SourceName: "CRM", State: "succeeded", Action: "contact.search", StagePosition: 1}
	tests := []struct {
		name   string
		mutate func(*Evidence)
	}{
		{name: "unknown kind", mutate: func(item *Evidence) { item.Kind = "tool" }},
		{name: "raw-sized action", mutate: func(item *Evidence) { item.Action = strings.Repeat("x", 241) }},
		{name: "citation on connector", mutate: func(item *Evidence) { item.Citation = &EvidenceCitation{RevisionID: "revision"} }},
		{name: "knowledge success without citation", mutate: func(item *Evidence) { item.Kind = "knowledge"; item.ContainerID = "base-1" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := valid
			test.mutate(&item)
			if err := ValidateEvidence([]Evidence{item}); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateEvidence() error = %v, want ErrInvalid", err)
			}
		})
	}
}
