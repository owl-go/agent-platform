package cliconnector

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

type PackageDefinitionMetadata struct {
	Name                   string
	Version                string
	Executable             string
	AuthenticationDriver   string
	Capabilities           []Capability
	SupportedArchitectures []string
}

type packageManifest struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	Bin            json.RawMessage `json:"bin"`
	AgentWorkspace struct {
		Executable             string   `json:"executable"`
		AuthenticationDriver   string   `json:"authenticationDriver"`
		SupportedArchitectures []string `json:"supportedArchitectures"`
		Capabilities           []struct {
			ID             string   `json:"id"`
			ArgvPrefix     []string `json:"argvPrefix"`
			Risk           string   `json:"risk"`
			Identities     []string `json:"identities"`
			Scopes         []string `json:"scopes"`
			EgressHosts    []string `json:"egressHosts"`
			TimeoutSeconds int      `json:"timeoutSeconds"`
		} `json:"capabilities"`
	} `json:"agentWorkspace"`
}

func packageDefinitionMetadata(raw []byte) (PackageDefinitionMetadata, error) {
	var manifest packageManifest
	if len(raw) == 0 || json.Unmarshal(raw, &manifest) != nil {
		return PackageDefinitionMetadata{}, errors.New("CLI package has no readable package.json metadata")
	}
	metadata := PackageDefinitionMetadata{
		Name: manifest.Name, Version: manifest.Version,
		Executable:             manifest.AgentWorkspace.Executable,
		AuthenticationDriver:   manifest.AgentWorkspace.AuthenticationDriver,
		SupportedArchitectures: append([]string(nil), manifest.AgentWorkspace.SupportedArchitectures...),
	}
	for _, item := range manifest.AgentWorkspace.Capabilities {
		identities := make([]Identity, 0, len(item.Identities))
		for _, identity := range item.Identities {
			identities = append(identities, Identity(identity))
		}
		metadata.Capabilities = append(metadata.Capabilities, Capability{ID: item.ID, ArgvPrefix: append([]string(nil), item.ArgvPrefix...), Risk: Risk(item.Risk), Identities: identities, Scopes: append([]string(nil), item.Scopes...), EgressHosts: append([]string(nil), item.EgressHosts...), Timeout: time.Duration(item.TimeoutSeconds) * time.Second})
	}
	if manifest.Name == "@larksuite/cli" && len(metadata.Capabilities) == 0 {
		metadata.Executable = "lark-cli"
		metadata.AuthenticationDriver = "feishu"
		metadata.SupportedArchitectures = []string{"linux-amd64"}
		metadata.Capabilities = []Capability{{ID: "identity", ArgvPrefix: []string{"auth", "status"}, Risk: RiskLow, Identities: []Identity{IdentityUser}, EgressHosts: []string{"open.feishu.cn"}, Timeout: time.Minute}}
	}
	return metadata, nil
}

type ZIPPackageBuilder struct{}

func (ZIPPackageBuilder) Build(_ context.Context, archive []byte) (PackageArtifact, error) {
	files, manifest, bins, err := readZIPPackage(archive)
	if err != nil {
		return PackageArtifact{}, err
	}
	bundle, err := buildZIPBundle(files, manifest.Name, bins)
	if err != nil {
		return PackageArtifact{}, err
	}
	integrity := sha512.Sum512(archive)
	manifestBytes, _ := json.Marshal(manifest)
	return PackageArtifact{PackageBytes: append([]byte(nil), archive...), BundleBytes: bundle, Integrity: "sha512-" + base64.StdEncoding.EncodeToString(integrity[:]), Bins: bins, Manifest: manifestBytes}, nil
}

func ValidateZIPPackage(archive []byte) error {
	_, _, _, err := readZIPPackage(archive)
	return err
}

type zipPackageFile struct {
	Name string
	Data []byte
	Mode int64
}

func readZIPPackage(archive []byte) ([]zipPackageFile, packageManifest, map[string]string, error) {
	if len(archive) == 0 || len(archive) > maxSourceArchiveSize {
		return nil, packageManifest{}, nil, errors.New("CLI ZIP package must contain 1-50 MiB")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, packageManifest{}, nil, fmt.Errorf("open CLI ZIP package: %w", err)
	}
	var manifestCandidates []string
	for _, entry := range reader.File {
		name := path.Clean(strings.TrimPrefix(entry.Name, "./"))
		if name == "package.json" || strings.Count(name, "/") == 1 && strings.HasSuffix(name, "/package.json") {
			manifestCandidates = append(manifestCandidates, name)
		}
	}
	if len(manifestCandidates) != 1 {
		return nil, packageManifest{}, nil, errors.New("CLI ZIP package must contain one root package.json")
	}
	root := strings.TrimSuffix(manifestCandidates[0], "package.json")
	seen := map[string]struct{}{}
	files := make([]zipPackageFile, 0, len(reader.File))
	var total int64
	for _, entry := range reader.File {
		name := path.Clean(strings.TrimPrefix(entry.Name, "./"))
		if name == "." || entry.FileInfo().IsDir() {
			continue
		}
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00\r\n") || !strings.HasPrefix(name, root) {
			return nil, packageManifest{}, nil, errors.New("CLI ZIP package contains a file outside its package root")
		}
		name = strings.TrimPrefix(name, root)
		if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") || entry.Mode()&os.ModeType != 0 {
			return nil, packageManifest{}, nil, errors.New("CLI ZIP package contains an unsupported entry")
		}
		if _, exists := seen[name]; exists {
			return nil, packageManifest{}, nil, errors.New("CLI ZIP package contains duplicate paths")
		}
		seen[name] = struct{}{}
		total += int64(entry.UncompressedSize64)
		if total > maxBundleSize {
			return nil, packageManifest{}, nil, errors.New("expanded CLI ZIP package is too large")
		}
		body, err := entry.Open()
		if err != nil {
			return nil, packageManifest{}, nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(body, int64(entry.UncompressedSize64)+1))
		closeErr := body.Close()
		if readErr != nil || closeErr != nil || uint64(len(data)) != entry.UncompressedSize64 {
			return nil, packageManifest{}, nil, errors.New("read CLI ZIP package entry")
		}
		mode := int64(0o644)
		if entry.Mode()&0o111 != 0 {
			mode = 0o755
		}
		files = append(files, zipPackageFile{Name: name, Data: data, Mode: mode})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	var manifest packageManifest
	for _, file := range files {
		if file.Name == "package.json" {
			if len(file.Data) > 1<<20 || json.Unmarshal(file.Data, &manifest) != nil {
				return nil, packageManifest{}, nil, errors.New("CLI ZIP package has an invalid package.json")
			}
			break
		}
	}
	if !npmPackage.MatchString(manifest.Name) || !exactVersion.MatchString(manifest.Version) {
		return nil, packageManifest{}, nil, errors.New("CLI ZIP package requires a valid package name and exact version")
	}
	bins, err := manifestBins(manifest)
	if err != nil {
		return nil, packageManifest{}, nil, err
	}
	for _, binPath := range bins {
		if _, ok := seen[binPath]; !ok {
			return nil, packageManifest{}, nil, errors.New("CLI ZIP package bin points to a missing file")
		}
	}
	return files, manifest, bins, nil
}

func manifestBins(manifest packageManifest) (map[string]string, error) {
	var bins map[string]string
	if len(manifest.Bin) > 0 && manifest.Bin[0] == '"' {
		var target string
		if json.Unmarshal(manifest.Bin, &target) != nil {
			return nil, errors.New("CLI ZIP package has invalid bin metadata")
		}
		bins = map[string]string{path.Base(manifest.Name): target}
	} else if json.Unmarshal(manifest.Bin, &bins) != nil {
		return nil, errors.New("CLI ZIP package has invalid bin metadata")
	}
	if len(bins) == 0 {
		return nil, errors.New("CLI ZIP package has no bin metadata")
	}
	for name, target := range bins {
		target = path.Clean(strings.TrimPrefix(target, "./"))
		if !definitionID.MatchString(name) || target == "." || path.IsAbs(target) || strings.HasPrefix(target, "../") || strings.ContainsAny(target, "\\\x00\r\n") {
			return nil, errors.New("CLI ZIP package has unsafe bin metadata")
		}
		bins[name] = target
	}
	return bins, nil
}

func buildZIPBundle(files []zipPackageFile, packageName string, bins map[string]string) ([]byte, error) {
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	compressed.Header.ModTime = time.Unix(0, 0)
	archive := tar.NewWriter(compressed)
	for _, file := range files {
		mode := file.Mode
		for _, target := range bins {
			if file.Name == target {
				mode = 0o755
			}
		}
		name := path.Join("node_modules", packageName, file.Name)
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(file.Data)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		if _, err := archive.Write(file.Data); err != nil {
			return nil, err
		}
	}
	binNames := make([]string, 0, len(bins))
	for name := range bins {
		binNames = append(binNames, name)
	}
	sort.Strings(binNames)
	for _, name := range binNames {
		target := path.Join("..", packageName, bins[name])
		if err := archive.WriteHeader(&tar.Header{Name: path.Join("node_modules/.bin", name), Linkname: target, Mode: 0o777, ModTime: time.Unix(0, 0), Typeflag: tar.TypeSymlink}); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	if err := compressed.Close(); err != nil {
		return nil, err
	}
	if buffer.Len() > maxBundleSize {
		return nil, errors.New("CLI ZIP bundle is too large")
	}
	return buffer.Bytes(), nil
}

var _ UploadPackageBuilder = ZIPPackageBuilder{}
