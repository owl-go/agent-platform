package connectorpackage

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
		if manifest.AuthenticationDriver == "" {
			if meta.AuthMode == "none" {
				manifest.AuthenticationDriver = "none"
			} else {
				manifest.AuthenticationDriver = "connector_package"
			}
		}
		if meta.AuthMode == "none" && manifest.AuthenticationDriver != "none" || meta.AuthMode != "none" && manifest.AuthenticationDriver == "none" {
			return Package{}, fmt.Errorf("cli.json authentication_driver does not match connector auth_mode")
		}
		if err := validateCLI(manifest); err != nil {
			return Package{}, err
		}
		pkg.CLI = &manifest
		if bundle, ok := byName["cli-bundle.tgz"]; ok {
			bundlePath := manifest.BundlePath
			if bundlePath == "" {
				bundlePath = "bin/" + manifest.Executable
			}
			if err := validateCLIBundle(bundle, bundlePath); err != nil {
				return Package{}, err
			}
			digest := sha256.Sum256(bundle)
			pkg.CLIBundle = append([]byte(nil), bundle...)
			pkg.CLIBundleSHA256 = hex.EncodeToString(digest[:])
		}
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

func validateCLIBundle(content []byte, executablePath string) error {
	if len(content) == 0 || len(content) > 256<<20 {
		return fmt.Errorf("cli-bundle.tgz must be non-empty and bounded")
	}
	cleaned := path.Clean(strings.TrimPrefix(executablePath, "./"))
	if cleaned == "." || path.IsAbs(executablePath) || strings.HasPrefix(cleaned, "../") || strings.ContainsAny(executablePath, "\\\x00\r\n") {
		return fmt.Errorf("cli.json bundle_path is unsafe")
	}
	reader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("open cli-bundle.tgz: %w", err)
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)
	seen := map[string]struct{}{}
	executables := map[string]bool{}
	symlinks := map[string]string{}
	var expanded int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read cli-bundle.tgz: %w", err)
		}
		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		if name == "." && header.Typeflag == tar.TypeDir {
			continue
		}
		if name == "." || path.IsAbs(header.Name) || name == ".." || strings.HasPrefix(name, "../") || strings.ContainsAny(header.Name, "\\\x00\r\n") {
			return fmt.Errorf("cli-bundle.tgz contains an unsafe path")
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("cli-bundle.tgz contains duplicate paths")
		}
		seen[name] = struct{}{}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA, tar.TypeDir:
		case tar.TypeSymlink:
			link := path.Clean(header.Linkname)
			resolved := path.Clean(path.Join(path.Dir(name), link))
			if header.Linkname == "" || path.IsAbs(header.Linkname) || strings.ContainsAny(header.Linkname, "\\\x00\r\n") || resolved == ".." || strings.HasPrefix(resolved, "../") || header.Size != 0 {
				return fmt.Errorf("cli-bundle.tgz contains an unsafe symlink")
			}
			symlinks[name] = resolved
		default:
			return fmt.Errorf("cli-bundle.tgz contains an unsupported entry")
		}
		if header.Size < 0 || header.Size > 256<<20 {
			return fmt.Errorf("cli-bundle.tgz contains an invalid entry size")
		}
		expanded += header.Size
		if expanded > 256<<20 {
			return fmt.Errorf("cli-bundle.tgz expanded content is too large")
		}
		if (header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA) && header.Mode&0o111 != 0 {
			executables[name] = true
		}
	}
	target := cleaned
	for index := 0; index <= len(symlinks); index++ {
		if executables[target] {
			return nil
		}
		next, ok := symlinks[target]
		if !ok {
			break
		}
		target = next
	}
	return fmt.Errorf("cli-bundle.tgz is missing executable %s", cleaned)
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
