package expertpackage

import (
	"context"
	"encoding/json"
	"testing"
)

func TestBilingualDisplayPackageRoundTrip(t *testing.T) {
	translation := DisplayTranslation{Name: "方案专家", Introduction: "评审方案", StarterPrompts: []string{"评审方案"}}
	english := DisplayTranslation{Name: "Proposal Expert", Introduction: "Review proposals", StarterPrompts: []string{"Review this proposal"}}
	for _, test := range []struct {
		name   string
		change func(*Profile)
		valid  bool
	}{
		{name: "bilingual", valid: true},
		{name: "legacy default only", valid: true, change: func(p *Profile) { p.Translations = nil }},
		{name: "missing English name", change: func(p *Profile) { p.Translations.English.Name = "" }},
		{name: "missing English introduction", change: func(p *Profile) { p.Translations.English.Introduction = "" }},
		{name: "missing corresponding task", change: func(p *Profile) { p.Translations.English.StarterPrompts = nil }},
		{name: "mismatched Chinese name", change: func(p *Profile) { p.Translations.Chinese.Name = "其他专家" }},
		{name: "blank translated task", change: func(p *Profile) { p.Translations.English.StarterPrompts = []string{" "} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := Profile{Name: translation.Name, Introduction: translation.Introduction, StarterPrompts: translation.StarterPrompts, GuidanceFile: "agents/expert.md", Translations: &DisplayTranslations{Chinese: translation, English: english}}
			if test.change != nil {
				test.change(&profile)
			}
			manifest := Manifest{SchemaVersion: 1, ID: "example.bilingual", Version: "1.0.0", Kind: "expert", Expert: &profile}
			metadata, _ := json.Marshal(manifest)
			archive, err := archiveFiles(map[string][]byte{".plugin/plugin.json": metadata, "agents/expert.md": []byte("# 方案评审")}, nil)
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := Parse(context.Background(), archive)
			if !test.valid {
				if err == nil {
					t.Fatal("invalid translations accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			exported, err := pkg.Archive()
			if err != nil {
				t.Fatal(err)
			}
			again, err := Parse(context.Background(), exported)
			if err != nil {
				t.Fatal(err)
			}
			if again.SHA256 != pkg.SHA256 {
				t.Fatal("display translations changed during round-trip")
			}
			if profile.Translations != nil && again.Manifest.Expert.Translations.English.Name != english.Name {
				t.Fatal("English display content lost")
			}
		})
	}
}
