package skillstore

import "testing"

func TestParseMetadataUsesDisplayName(t *testing.T) {
	metadata, err := ParseMetadata("---\nname: pdf\ndisplay_name: PDF 文档处理\ndescription: Process PDFs.\ndescription_zh: 处理 PDF。\n---\n# PDF\n")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.DisplayName != "PDF 文档处理" {
		t.Fatalf("DisplayName = %q", metadata.DisplayName)
	}
}

func TestParseMetadataRequiresDisplayName(t *testing.T) {
	for _, document := range []string{"# Skill", "---\nname: pdf\n---\n# Skill"} {
		if _, err := ParseMetadata(document); err == nil {
			t.Fatalf("ParseMetadata accepted %q", document)
		}
	}
}
