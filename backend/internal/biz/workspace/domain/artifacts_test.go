package domain

import "testing"

func TestFinalArtifactPathsSelectsOnlyNamedDeliverables(t *testing.T) {
	paths := []string{"assets/font.ttf", "create_pdf.js", "node_modules/pdf/package.json", "output/report.pdf"}
	selected := FinalArtifactPaths("已生成 `report.pdf`。", paths)
	if len(selected) != 1 {
		t.Fatalf("selected paths = %#v", selected)
	}
	if _, ok := selected["output/report.pdf"]; !ok {
		t.Fatalf("final PDF was not selected: %#v", selected)
	}
}

func TestFinalArtifactPathsRequiresAnUnambiguousBasename(t *testing.T) {
	paths := []string{"draft/report.pdf", "output/report.pdf"}
	if selected := FinalArtifactPaths("已生成 `report.pdf`。", paths); len(selected) != 0 {
		t.Fatalf("ambiguous basename selected paths = %#v", selected)
	}
	selected := FinalArtifactPaths("已生成 `/workspace/output/report.pdf`。", paths)
	if len(selected) != 1 {
		t.Fatalf("exact path selected paths = %#v", selected)
	}
	if _, ok := selected["output/report.pdf"]; !ok {
		t.Fatalf("exact final PDF was not selected: %#v", selected)
	}
}
