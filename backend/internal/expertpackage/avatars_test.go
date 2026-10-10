package expertpackage

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
)

func TestProfileAvatarAndThreeStartersRoundTrip(t *testing.T) {
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"avatars/expert": picture.Bytes(), "agents/expert.md": []byte("# Review")}
	manifest := Manifest{SchemaVersion: 1, ID: "example.profile", Version: "1.0.0", Kind: "expert", Expert: &Profile{Name: "Review", Introduction: "Review evidence", GuidanceFile: "agents/expert.md", AvatarFile: "avatars/expert", StarterPrompts: []string{"Review this", "Check evidence", "Find risks"}}}
	archive, err := exportContent(context.Background(), manifest, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Parse(context.Background(), archive)
	if err != nil || len(pkg.Expert.StarterPrompts) != 3 {
		t.Fatal(err)
	}
	exported, err := ExportExpert(context.Background(), domain.Expert{ID: "example", Version: 1, Name: pkg.Expert.Name, Introduction: pkg.Expert.Introduction, Guidance: pkg.Expert.Guidance, Icon: pkg.Expert.Icon, StarterPrompts: pkg.Expert.StarterPrompts})
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(context.Background(), exported)
	if err != nil || again.Expert.Icon != pkg.Expert.Icon || len(again.Expert.StarterPrompts) != 3 {
		t.Fatal("profile changed", err)
	}
	for _, content := range [][]byte{[]byte("fake png"), []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)} {
		files["avatars/expert"] = content
		if _, err := exportContent(context.Background(), manifest, files, nil); err == nil {
			t.Fatal("invalid image accepted")
		}
	}
	files["avatars/expert"] = picture.Bytes()
	manifest.Expert.StarterPrompts = append(manifest.Expert.StarterPrompts, "Fourth")
	if _, err := exportContent(context.Background(), manifest, files, nil); err == nil {
		t.Fatal("four starters accepted")
	}
}
