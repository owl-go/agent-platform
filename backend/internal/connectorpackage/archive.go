package connectorpackage

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	maxArchiveSize  = 50 << 20
	maxExpandedSize = 100 << 20
)

type archiveFile struct {
	name string
	body []byte
}

func Parse(content []byte) (Package, error) {
	files, err := readArchive(content)
	if err != nil {
		return Package{}, err
	}
	byName := make(map[string][]byte, len(files))
	for _, file := range files {
		byName[file.name] = file.body
	}
	metaBody, ok := byName["connector-meta.json"]
	if !ok {
		return Package{}, fmt.Errorf("Connector Package requires connector-meta.json")
	}
	if _, ok := byName["icon.svg"]; !ok {
		return Package{}, fmt.Errorf("Connector Package requires icon.svg")
	}
	if len(byName["icon.svg"]) == 0 || len(byName["icon.svg"]) > 512<<10 || !bytes.Contains(byName["icon.svg"], []byte("<svg")) {
		return Package{}, fmt.Errorf("icon.svg must be a bounded SVG document")
	}
	_, hasMCP := byName["mcp.json"]
	_, hasCLI := byName["cli.json"]
	if hasMCP == hasCLI {
		return Package{}, fmt.Errorf("Connector Package requires exactly one of mcp.json or cli.json")
	}
	var meta Metadata
	if err := decodeStrict("connector-meta.json", metaBody, &meta); err != nil {
		return Package{}, err
	}
	if err := validateMetadata(meta); err != nil {
		return Package{}, err
	}
	if meta.Type == TypeMCP && !hasMCP || meta.Type == TypeCLI && !hasCLI {
		return Package{}, fmt.Errorf("connector-meta.json type does not match the mode manifest")
	}

	pkg := Package{Metadata: meta}
	if hasMCP {
		var manifest MCPManifest
		if err := decodeStrict("mcp.json", byName["mcp.json"], &manifest); err != nil {
			return Package{}, err
		}
		if err := validateMCP(manifest); err != nil {
			return Package{}, err
		}
		pkg.MCP = &manifest
	} else {
		var manifest CLIManifest
		if err := decodeStrict("cli.json", byName["cli.json"], &manifest); err != nil {
			return Package{}, err
		}
		if err := validateCLI(manifest); err != nil {
			return Package{}, err
		}
		pkg.CLI = &manifest
	}

	for name, body := range byName {
		if !strings.HasPrefix(name, "skills/") || !strings.HasSuffix(name, "/SKILL.md") || strings.Count(name, "/") != 2 {
			continue
		}
		directory := path.Dir(name)
		if err := validateSkillResources(directory, body, byName); err != nil {
			return Package{}, err
		}
		skill, err := parseSkill(directory, body)
		if err != nil {
			return Package{}, err
		}
		pkg.Skills = append(pkg.Skills, skill)
	}
	if len(pkg.Skills) == 0 {
		return Package{}, fmt.Errorf("Connector Package requires at least one skills/*/SKILL.md")
	}
	sort.Slice(pkg.Skills, func(i, j int) bool { return pkg.Skills[i].Directory < pkg.Skills[j].Directory })

	normalized, err := writeArchive(files)
	if err != nil {
		return Package{}, err
	}
	digest := sha256.Sum256(normalized)
	pkg.NormalizedArchive = normalized
	pkg.SHA256 = hex.EncodeToString(digest[:])
	return pkg, nil
}

func readArchive(content []byte) ([]archiveFile, error) {
	if len(content) == 0 || len(content) > maxArchiveSize {
		return nil, fmt.Errorf("Connector Package ZIP must contain 1-50 MiB")
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("open Connector Package ZIP: %w", err)
	}
	root := enclosingRoot(reader.File)
	seen := map[string]struct{}{}
	files := make([]archiveFile, 0, len(reader.File))
	var expanded int64
	for _, entry := range reader.File {
		name := strings.TrimPrefix(entry.Name, "./")
		for _, component := range strings.Split(name, "/") {
			if component == ".." {
				return nil, fmt.Errorf("Connector Package contains an unsafe path")
			}
		}
		cleaned := path.Clean(name)
		if path.IsAbs(name) || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.ContainsAny(name, "\\\x00\r\n") {
			return nil, fmt.Errorf("Connector Package contains an unsafe path")
		}
		if cleaned == "." || cleaned == "" || entry.FileInfo().IsDir() {
			continue
		}
		if entry.Mode()&os.ModeType != 0 {
			return nil, fmt.Errorf("Connector Package contains an unsupported symbolic or special entry")
		}
		if _, duplicate := seen[cleaned]; duplicate {
			return nil, fmt.Errorf("Connector Package contains duplicate paths")
		}
		seen[cleaned] = struct{}{}
		expanded += int64(entry.UncompressedSize64)
		if expanded > maxExpandedSize {
			return nil, fmt.Errorf("Connector Package expanded content exceeds 100 MiB")
		}
		stream, err := entry.Open()
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, int64(entry.UncompressedSize64)+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || uint64(len(body)) != entry.UncompressedSize64 {
			return nil, fmt.Errorf("read Connector Package entry %s", cleaned)
		}
		files = append(files, archiveFile{name: cleaned, body: body})
	}
	if root != "" {
		return nil, fmt.Errorf("Connector Package contains an enclosing folder")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

func enclosingRoot(entries []*zip.File) string {
	root := ""
	for _, entry := range entries {
		name := path.Clean(strings.TrimPrefix(entry.Name, "./"))
		if name == "." || strings.Contains(name, "/") == false {
			return ""
		}
		first := strings.SplitN(name, "/", 2)[0]
		if root == "" {
			root = first
		} else if root != first {
			return ""
		}
	}
	return root
}

func writeArchive(files []archiveFile) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		header.SetMode(0o644)
		header.SetModTime(time.Unix(0, 0).UTC())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(file.body); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
