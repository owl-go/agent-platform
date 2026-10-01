package connectorpackage_test

import (
	"testing"

	"agent-platform/backend/internal/connectorpackage"
)

func TestDisplayIconProjectsReviewedBrands(t *testing.T) {
	for _, source := range []string{"feishu", "dingtalk", "modao"} {
		if got := connectorpackage.DisplayIcon(source); got != source {
			t.Fatalf("DisplayIcon(%q) = %q", source, got)
		}
	}
	if got := connectorpackage.DisplayIcon("unreviewed"); got != "plug" {
		t.Fatalf("unknown source icon = %q", got)
	}
}
