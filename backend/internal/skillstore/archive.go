package skillstore

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

func normalizeArchive(ctx context.Context, archive []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("Skill archive is not a valid ZIP: %w", err)
	}
	files := make(map[string]*zip.File)
	seen := make(map[string]bool)
	var expandedSize uint64
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(file.Name, "/")
		if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\:\x00") {
			return nil, fmt.Errorf("Skill archive contains an unsafe path")
		}
		if !file.Mode().IsRegular() && !file.Mode().IsDir() {
			return nil, fmt.Errorf("Skill archive may only contain regular files and directories")
		}
		if seen[name] {
			return nil, fmt.Errorf("Skill archive contains duplicate paths")
		}
		seen[name] = true
		// Finder adds metadata beside the Skill folder and each resource.
		if name == "__MACOSX" || strings.HasPrefix(name, "__MACOSX/") || path.Base(name) == ".DS_Store" || strings.HasPrefix(path.Base(name), "._") {
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > maxArchiveSize-expandedSize {
			return nil, fmt.Errorf("Skill archive expanded contents exceed 50 MiB")
		}
		expandedSize += file.UncompressedSize64
		files[name] = file
	}
	prefix := ""
	if files["SKILL.md"] == nil {
		for name := range files {
			folder, rest, ok := strings.Cut(name, "/")
			if ok && rest == "SKILL.md" {
				if prefix != "" {
					return nil, fmt.Errorf("Skill archive must contain a single Skill folder")
				}
				prefix = folder + "/"
			}
		}
		if prefix == "" {
			return nil, fmt.Errorf("Skill archive root or single top-level folder must contain SKILL.md")
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if !strings.HasPrefix(name, prefix) {
			return nil, fmt.Errorf("Skill archive contains files outside its Skill folder")
		}
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if files[parent] != nil {
				return nil, fmt.Errorf("Skill archive contains conflicting file and directory paths")
			}
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file := files[name]
		header := &zip.FileHeader{Name: strings.TrimPrefix(name, prefix), Method: zip.Deflate}
		header.SetMode(file.Mode().Perm())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		content, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open Skill archive entry: %w", err)
		}
		_, copyErr := io.Copy(entry, io.LimitReader(content, int64(file.UncompressedSize64)+1))
		closeErr := content.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("read Skill archive entry: %w", copyErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if buffer.Len() > maxArchiveSize {
			return nil, fmt.Errorf("Skill archive exceeds 50 MiB")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if buffer.Len() > maxArchiveSize {
		return nil, fmt.Errorf("Skill archive exceeds 50 MiB")
	}
	return buffer.Bytes(), nil
}
