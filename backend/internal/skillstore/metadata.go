package skillstore

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Metadata contains the catalog fields owned by the installed SKILL.md document.
type Metadata struct {
	DisplayName string
}

func ParseMetadata(document string) (Metadata, error) {
	if !utf8.ValidString(document) {
		return Metadata{}, fmt.Errorf("SKILL.md must be UTF-8 text")
	}
	normalized := strings.ReplaceAll(document, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return Metadata{}, fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	end := strings.Index(normalized[4:], "\n---")
	if end < 0 {
		return Metadata{}, fmt.Errorf("SKILL.md has unterminated YAML frontmatter")
	}
	var values struct {
		DisplayName string `yaml:"display_name"`
	}
	if err := yaml.Unmarshal([]byte(normalized[4:4+end]), &values); err != nil {
		return Metadata{}, fmt.Errorf("parse SKILL.md frontmatter: %w", err)
	}
	values.DisplayName = strings.TrimSpace(values.DisplayName)
	if values.DisplayName == "" || utf8.RuneCountInString(values.DisplayName) > 100 {
		return Metadata{}, fmt.Errorf("SKILL.md display_name must contain 1-100 characters")
	}
	return Metadata{DisplayName: values.DisplayName}, nil
}

func metadataFromArchive(content []byte) (Metadata, error) {
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return Metadata{}, err
	}
	for _, file := range archive.File {
		if file.Name != "SKILL.md" {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return Metadata{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, maxArchiveSize+1))
		closeErr := stream.Close()
		if readErr != nil {
			return Metadata{}, readErr
		}
		if closeErr != nil {
			return Metadata{}, closeErr
		}
		if len(body) > maxArchiveSize || !utf8.Valid(body) {
			return Metadata{}, fmt.Errorf("Skill document is not bounded UTF-8 text")
		}
		return ParseMetadata(string(body))
	}
	return Metadata{}, fmt.Errorf("Skill package has no SKILL.md")
}
