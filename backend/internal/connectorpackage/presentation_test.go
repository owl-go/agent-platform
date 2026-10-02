package connectorpackage

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"testing"

	"agent-platform/backend/internal/icon"
)

func TestTeambitionDisplayIconIsAFrontendSupportedBrandImage(t *testing.T) {
	value := DisplayIcon("teambition")
	if err := icon.Validate(value); err != nil {
		t.Fatal(err)
	}
	encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
	if !ok {
		t.Fatal("Teambition must project a displayable image instead of a preset fallback")
	}
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width != 128 || config.Height != 128 {
		t.Fatalf("brand image: %v, %v", config, err)
	}
}

func TestDisplayIconProjectsReviewedBrands(t *testing.T) {
	for _, source := range []string{"feishu", "dingtalk", "notion", "modao", "openboost"} {
		if got := DisplayIcon(source); got != source {
			t.Fatalf("DisplayIcon(%q) = %q", source, got)
		}
	}
	if got := DisplayIcon("unreviewed"); got != "plug" {
		t.Fatalf("unknown source icon = %q", got)
	}
}
