package expertpackage

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"
)

func TestBundledSkillUsesSharedValidationAndRetainsExecutableArchiveMode(t *testing.T) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for name, content := range map[string]string{
		".plugin/plugin.json":            `{"schema_version":1,"id":"example.reviewer","version":"1.0.0","kind":"expert","expert":{"name":"Reviewer","introduction":"Review","guidance_file":"agents/reviewer.md","bundled_skills":["skills/review"]}}`,
		"agents/reviewer.md":             "# Review",
		"skills/review/SKILL.md":         "---\nname: review\ndisplay_name: Review skill\ndescription: Review supplied evidence.\n---\n\n# Review\nUse the supplied evidence.\n",
		"skills/review/scripts/check.sh": "#!/bin/sh\nprintf '%s' 'checked'\n",
	} {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0644)
		if name == "skills/review/scripts/check.sh" {
			header.SetMode(0755)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	pkg, err := Parse(context.Background(), data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	bundle, ok := pkg.BundledSkills["skills/review"]
	if !ok || bundle.Name != "Review skill" || bundle.SHA256 == "" {
		t.Fatalf("bundle=%+v", bundle)
	}
	archive, err := pkg.Archive()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name == "skills/review/scripts/check.sh" {
			if file.Mode().Perm()&0111 == 0 {
				t.Fatal("export erased executable mode")
			}
			stream, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			_, err = io.ReadAll(stream)
			stream.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := Parse(context.Background(), archive); err != nil {
		t.Fatal("portable bundled Skill cannot round trip", err)
	}
}
