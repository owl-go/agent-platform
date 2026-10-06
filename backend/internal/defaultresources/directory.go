package defaultresources

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/icon"
	"agent-platform/backend/internal/skillstore"
)

const RootEnvironment = "AGENT_WORKSPACE_RESOURCE_ROOT"
const maxPackageBytes = 100 << 20
const maxPackageFiles = 4000

var resourceKey = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
var resourceVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?(?:\+[a-zA-Z0-9.-]+)?$`)

// Load is the context-free convenience entry point used by catalog checks.
func Load() (Catalog, error) { return LoadContext(context.Background()) }

// LoadContext uses the explicitly configured release directory. Local Go
// commands discover the repository marker by walking up from their workdir.
func LoadContext(ctx context.Context) (Catalog, error) {
	if root := os.Getenv(RootEnvironment); root != "" {
		return LoadDirectory(ctx, root)
	}
	root, err := os.Getwd()
	if err != nil {
		return Catalog{}, err
	}
	for {
		if _, err := os.Lstat(filepath.Join(root, "resources.json")); err == nil {
			return LoadDirectory(ctx, root)
		}
		parent := filepath.Dir(root)
		if parent == root {
			return Catalog{}, fmt.Errorf("resource directory missing; set %s", RootEnvironment)
		}
		root = parent
	}
}

// LoadDirectory freezes all definitions and package bytes before any database
// mutation. Directory contents pass the same validators as uploaded packages.
func LoadDirectory(ctx context.Context, directory string) (Catalog, error) {
	if err := ctx.Err(); err != nil {
		return Catalog{}, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Catalog{}, fmt.Errorf("resource root must be an existing regular directory: %s", directory)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return Catalog{}, fmt.Errorf("open resource directory: %w", err)
	}
	defer root.Close()
	files := root.FS()
	catalog := Catalog{archives: map[string][]byte{}}
	var header struct {
		Version string `json:"version"`
	}
	if err := readDefinition(files, "resources.json", &header); err != nil {
		return Catalog{}, err
	}
	if !resourceVersion.MatchString(header.Version) {
		return Catalog{}, fmt.Errorf("resources.json requires a semantic version")
	}
	catalog.Version = header.Version
	for _, kind := range []string{"skills", "connectors", "experts"} {
		entries, err := fs.ReadDir(files, kind)
		if err != nil {
			return Catalog{}, fmt.Errorf("read %s directory: %w", kind, err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return Catalog{}, err
			}
			name := path.Join(kind, entry.Name())
			if entry.Type()&fs.ModeSymlink != 0 {
				return Catalog{}, fmt.Errorf("resource directory contains a symbolic link: %s", name)
			}
			if !entry.IsDir() {
				if entry.Name() == "README.md" {
					continue
				}
				return Catalog{}, fmt.Errorf("resource must be in its own directory: %s", name)
			}
			if !resourceKey.MatchString(entry.Name()) {
				return Catalog{}, fmt.Errorf("invalid resource directory name: %s", name)
			}
			switch kind {
			case "skills":
				err = catalog.loadSkill(ctx, files, name)
			case "connectors":
				err = catalog.loadConnector(ctx, files, name)
			case "experts":
				err = catalog.loadExpert(files, name)
			}
			if err != nil {
				return Catalog{}, fmt.Errorf("load %s: %w", name, err)
			}
		}
	}
	if err := catalog.validate(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func readDefinition(files fs.FS, name string, target any) error {
	data, err := readFile(files, name, 100<<10)
	if err != nil {
		return err
	}
	if err := decodeDefinition(data, target); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func readFile(files fs.FS, name string, limit int64) ([]byte, error) {
	// Reject links in every path component, including JSON metadata and the
	// package directory itself. os.Root also prevents escaping during a race.
	parent := "."
	for _, component := range strings.Split(name, "/") {
		entries, err := fs.ReadDir(files, parent)
		if err != nil {
			return nil, fmt.Errorf("read resource path %s: %w", name, err)
		}
		for _, entry := range entries {
			if entry.Name() == component && entry.Type()&fs.ModeSymlink != 0 {
				return nil, fmt.Errorf("resource path contains a symbolic link: %s", name)
			}
		}
		parent = path.Join(parent, component)
	}
	info, err := fs.Stat(files, name)
	if err != nil {
		return nil, fmt.Errorf("read resource file %s: %w", name, err)
	}
	// os.Root confines traversal; WalkDir separately rejects even in-root links.
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("resource file is not regular or exceeds its size limit: %s", name)
	}
	file, err := files.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open resource file %s: %w", name, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read resource file %s: %w", name, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("resource file exceeds its size limit: %s", name)
	}
	return data, nil
}

func (catalog *Catalog) loadSkill(ctx context.Context, files fs.FS, directory string) error {
	var meta struct {
		Key     string `json:"key"`
		Version string `json:"version"`
		Icon    string `json:"icon"`
	}
	if err := readDefinition(files, directory+"/resource.json", &meta); err != nil {
		return err
	}
	if strings.HasPrefix(meta.Key, "system.") {
		return fmt.Errorf("built-in Skill keys are reserved")
	}
	archive, err := archiveDirectory(ctx, files, directory, "resource.json")
	if err != nil {
		return err
	}
	normalized, digest, metadata, err := skillstore.ValidateUpload(ctx, archive)
	if err != nil {
		return err
	}
	if meta.Icon == "" {
		meta.Icon = "sparkles"
	}
	if err := icon.Validate(meta.Icon); err != nil {
		return err
	}
	name := directory + ".zip"
	catalog.archives[name] = normalized
	catalog.Skills = append(catalog.Skills, Skill{Key: meta.Key, Name: metadata.DisplayName, Version: meta.Version, Icon: meta.Icon, Archive: name, SHA256: digest})
	return nil
}

func (catalog *Catalog) loadConnector(ctx context.Context, files fs.FS, directory string) error {
	// package/ isolates distributable bytes from build scripts and source files.
	packageDirectory := directory
	if info, err := fs.Stat(files, directory+"/package"); err == nil && info.IsDir() {
		packageDirectory += "/package"
	}
	archive, err := archiveDirectory(ctx, files, packageDirectory, "")
	if err != nil {
		return err
	}
	pkg, err := connectorpackage.Parse(archive)
	if err != nil {
		return err
	}
	if path.Base(directory) != pkg.Metadata.Source {
		return fmt.Errorf("Connector source must match its directory name")
	}
	if pkg.CLI != nil && len(pkg.CLIBundle) == 0 {
		return fmt.Errorf("CLI directory requires its immutable cli-bundle.tgz")
	}
	name := directory + ".zip"
	catalog.archives[name] = pkg.NormalizedArchive
	catalog.Connectors = append(catalog.Connectors, Connector{Source: pkg.Metadata.Source, Version: pkg.Metadata.Version, Archive: name, SHA256: pkg.SHA256})
	return nil
}

func (catalog *Catalog) loadExpert(files fs.FS, directory string) error {
	var definition Expert
	if err := readDefinition(files, directory+"/expert.json", &definition); err != nil {
		return err
	}
	input := domain.ExpertInput{Name: definition.Name, Icon: definition.Icon, IconBackground: definition.IconBackground, Introduction: definition.Introduction, CoreCapability: definition.CoreCapability, OperatingProcedure: definition.OperatingProcedure, OutputStandard: definition.OutputStandard, Cautions: definition.Cautions}
	if strings.TrimSpace(definition.Introduction) == "" || strings.TrimSpace(definition.CoreCapability) == "" || strings.TrimSpace(definition.OperatingProcedure) == "" || strings.TrimSpace(definition.OutputStandard) == "" {
		return fmt.Errorf("Expert requires complete structured guidance")
	}
	if err := input.Validate(); err != nil {
		return err
	}
	catalog.Experts = append(catalog.Experts, definition)
	return nil
}

func archiveDirectory(ctx context.Context, files fs.FS, directory, excluded string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	defer writer.Close()
	var size int64
	count := 0
	err := fs.WalkDir(files, directory, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("resource package contains a symbolic link: %s", name)
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(name, directory+"/")
		if relative == excluded {
			return nil
		}
		if !fs.ValidPath(relative) || strings.ContainsAny(relative, "\\:\x00") {
			return fmt.Errorf("unsafe resource path")
		}
		count++
		if count > maxPackageFiles {
			return fmt.Errorf("resource package contains too many files")
		}
		data, err := readFile(files, name, maxPackageBytes-size)
		if err != nil {
			return err
		}
		size += int64(len(data))
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header := &zip.FileHeader{Name: relative, Method: zip.Deflate}
		header.SetMode(info.Mode().Perm())
		file, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if buffer.Len() > 50<<20 {
		return nil, fmt.Errorf("resource package ZIP exceeds 50 MiB")
	}
	return buffer.Bytes(), nil
}
