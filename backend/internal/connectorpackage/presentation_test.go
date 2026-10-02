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

func TestLinearDisplayIconIsAFrontendSupportedBrandImage(t *testing.T) {
	value := DisplayIcon("linear")
	if err := icon.Validate(value); err != nil {
		t.Fatal(err)
	}
	encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
	if !ok {
		t.Fatal("Linear must project its official brand image")
	}
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.DecodeConfig(bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
}

func TestDisplayIconProjectsReviewedBrands(t *testing.T) {
	for _, source := range []string{"feishu", "dingtalk", "notion", "modao", "github"} {
		if got := DisplayIcon(source); got != source {
			t.Fatalf("DisplayIcon(%q) = %q", source, got)
		}
	}
	if got := DisplayIcon("unreviewed"); got != "plug" {
		t.Fatalf("unknown source icon = %q", got)
	}
}

func TestKlingDisplayIconIsOfficialPNG(t *testing.T) {
	value := DisplayIcon("kling-ai")
	if err := icon.Validate(value); err != nil {
		t.Fatal(err)
	}
	encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
	if !ok {
		t.Fatal("missing brand image")
	}
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width != 180 || config.Height != 180 {
		t.Fatalf("invalid Kling icon: %v", err)
	}
}

func TestPixsoDisplayIconIsAFrontendSupportedBrandImage(t *testing.T) {
	value := DisplayIcon("pixso")
	if err := icon.Validate(value); err != nil {
		t.Fatal(err)
	}
	encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
	if !ok {
		t.Fatal("Pixso must project its official brand image")
	}
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.DecodeConfig(bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
}

func TestAIHiveDisplayIconUsesReviewedOfficialSVG(t *testing.T) {
	value := DisplayIcon("ai-hive")
	prefix := "data:image/svg+xml;base64,"
	if !strings.HasPrefix(value, prefix) {
		t.Fatal("AI-Hive icon is not an image Data URL")
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil || !bytes.Equal(body, aiHiveIcon) {
		t.Fatal("AI-Hive icon differs from bundled brand asset")
	}
}
