// Package expertpackage validates portable specialist definitions without
// granting resource access, selecting a Runtime, or mutating persistent state.
package expertpackage

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/strictjson"
)

const MaxBytes = 100 << 20
const MaxFiles = 4000

var packageIdentity = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
var packageVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

type Profile struct {
	Translations   *DisplayTranslations               `json:"translations,omitempty"`
	Connectors     []domain.ExpertConnectorDependency `json:"connectors,omitempty"`
	StarterPrompts []string                           `json:"starter_prompts,omitempty"`
	AvatarFile     string                             `json:"avatar_file,omitempty"`
	Name           string                             `json:"name"`
	Introduction   string                             `json:"introduction"`
	Icon           string                             `json:"icon,omitempty"`
	IconBackground string                             `json:"icon_background,omitempty"`
	GuidanceFile   string                             `json:"guidance_file"`
	SkillKeys      []string                           `json:"skill_keys,omitempty"`
	BundledSkills  []string                           `json:"bundled_skills,omitempty"`
}

type Manifest struct {
	SchemaVersion int          `json:"schema_version"`
	ID            string       `json:"id"`
	Version       string       `json:"version"`
	Kind          string       `json:"kind"`
	Expert        *Profile     `json:"expert,omitempty"`
	Team          *TeamProfile `json:"team,omitempty"`
}

type Package struct {
	Manifest      Manifest
	Expert        domain.ExpertInput
	Team          domain.ExpertTeamInput
	SHA256        string
	BundledSkills map[string]BundledSkill
	modes         map[string]fs.FileMode
	files         map[string][]byte
}

func Parse(ctx context.Context, archive []byte) (Package, error) {
	var result Package
	if len(archive) == 0 || len(archive) > MaxBytes {
		return result, fmt.Errorf("%w: Expert Package size exceeds limits", domain.ErrInvalid)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return result, fmt.Errorf("%w: read Expert Package ZIP: %v", domain.ErrInvalid, err)
	}
	if len(reader.File) > MaxFiles {
		return result, fmt.Errorf("%w: too many package files", domain.ErrInvalid)
	}
	files := make(map[string][]byte)
	seen := make(map[string]bool)
	modes := make(map[string]fs.FileMode)
	var total int64
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		name := strings.TrimSuffix(file.Name, "/")
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || path.Clean(name) != name || seen[strings.ToLower(name)] {
			return result, fmt.Errorf("%w: unsafe or duplicate package path", domain.ErrInvalid)
		}
		seen[strings.ToLower(name)] = true
		if file.Mode()&fs.ModeSymlink != 0 || !file.Mode().IsRegular() && !file.FileInfo().IsDir() {
			return result, fmt.Errorf("%w: package links or special files are forbidden", domain.ErrInvalid)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if name != ".plugin/plugin.json" && !(strings.HasPrefix(name, "agents/") && strings.HasSuffix(name, ".md")) && !strings.HasPrefix(name, "skills/") && !strings.HasPrefix(name, "avatars/") {
			return result, fmt.Errorf("%w: unsupported package file", domain.ErrInvalid)
		}
		if file.UncompressedSize64 > MaxBytes || total+int64(file.UncompressedSize64) > MaxBytes {
			return result, fmt.Errorf("%w: expanded package size exceeds limits", domain.ErrInvalid)
		}
		stream, err := file.Open()
		if err != nil {
			return result, fmt.Errorf("%w: open package file: %v", domain.ErrInvalid, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, MaxBytes-total+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil {
			return result, fmt.Errorf("%w: read package file: %v", domain.ErrInvalid, readErr)
		}
		total += int64(len(data))
		if total > MaxBytes || ((name == ".plugin/plugin.json" || strings.HasPrefix(name, "agents/")) && !utf8.Valid(data)) {
			return result, fmt.Errorf("%w: package text or size is invalid", domain.ErrInvalid)
		}
		files[name] = data
		modes[name] = file.Mode().Perm()
	}
	metadata, ok := files[".plugin/plugin.json"]
	if !ok || len(metadata) > 100<<10 {
		return result, fmt.Errorf("%w: .plugin/plugin.json is required", domain.ErrInvalid)
	}
	if err := strictjson.Decode(metadata, &result.Manifest); err != nil {
		return result, fmt.Errorf("%w: invalid package manifest: %v", domain.ErrInvalid, err)
	}

	manifest := result.Manifest
	if manifest.SchemaVersion != 1 || !packageIdentity.MatchString(manifest.ID) || !validVersion(manifest.Version) {
		return result, fmt.Errorf("%w: unsupported package identity, schema or version", domain.ErrInvalid)
	}
	used := map[string]bool{".plugin/plugin.json": true}
	result.BundledSkills, err = readBundledSkills(ctx, manifest, files, modes, used)
	if err != nil {
		return result, err
	}
	switch manifest.Kind {
	case "expert":
		if manifest.Expert == nil || manifest.Team != nil {
			return result, fmt.Errorf("%w: Expert profile is required", domain.ErrInvalid)
		}
		result.Expert, err = readProfile(manifest.Expert, files, used)
	case "expert_team":
		if manifest.Team == nil || manifest.Expert != nil {
			return result, fmt.Errorf("%w: Team profile is required", domain.ErrInvalid)
		}
		profile := manifest.Team
		if err := profile.Translations.validate(profile.Name, profile.Introduction, profile.StarterPrompts); err != nil {
			return result, err
		}
		teamIcon, avatarErr := readAvatar(profile.Icon, profile.AvatarFile, files, used)
		if avatarErr != nil {
			return result, avatarErr
		}
		result.Team = domain.ExpertTeamInput{StarterPrompts: profile.StarterPrompts, Name: profile.Name, Introduction: profile.Introduction, Icon: teamIcon, IconBackground: profile.IconBackground, CoreCapability: profile.CoreCapability, LeadMemberID: profile.LeadMemberID}
		for _, member := range profile.Members {
			var definition domain.ExpertInput
			definition, err = readProfile(&member.Expert, files, used)
			if err != nil {
				break
			}
			result.Team.Members = append(result.Team.Members, domain.ExpertTeamMemberInput{ID: member.ID, Name: member.Name, Labels: member.Labels, Definition: &definition})
		}
		if err == nil {
			err = result.Team.Validate()
		}
		if err == nil {
			err = result.Team.ValidateLead()
		}
	default:
		err = fmt.Errorf("%w: unsupported specialist type", domain.ErrInvalid)
	}
	if err != nil {
		return result, err
	}
	if len(used) != len(files) {
		return result, fmt.Errorf("%w: unreferenced package content", domain.ErrInvalid)
	}
	// ZIP timestamps, compression and entry ordering are transport metadata.
	canonicalMetadata, _ := json.Marshal(result.Manifest)
	files[".plugin/plugin.json"] = canonicalMetadata
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		if strings.HasPrefix(name, "skills/") {
			fmt.Fprintf(hash, "%o:", modes[name]&0111)
		}
		fmt.Fprintf(hash, "%d:%s%d:", len(name), name, len(files[name]))
		_, _ = hash.Write(files[name])
	}
	result.SHA256 = hex.EncodeToString(hash.Sum(nil))
	result.files = files
	result.modes = modes
	return result, nil
}

func validVersion(value string) bool {
	if !packageVersion.MatchString(value) {
		return false
	}
	release := strings.SplitN(value, "+", 2)[0]
	if parts := strings.SplitN(release, "-", 2); len(parts) == 2 {
		for _, identifier := range strings.Split(parts[1], ".") {
			numeric := strings.Trim(identifier, "0123456789") == ""
			if numeric && len(identifier) > 1 && identifier[0] == '0' {
				return false
			}
		}
	}
	return true
}

// Archive produces deterministic neutral transport bytes for a validated package.
func (pkg Package) Archive() ([]byte, error) {
	files := make(map[string][]byte, len(pkg.files))
	for name, data := range pkg.files {
		files[name] = data
	}
	metadata, err := json.Marshal(pkg.Manifest)
	if err != nil {
		return nil, err
	}
	files[".plugin/plugin.json"] = metadata
	return archiveFiles(files, pkg.modes)
}

func archiveFiles(files map[string][]byte, modes ...map[string]fs.FileMode) ([]byte, error) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		mode := fs.FileMode(0644)
		if len(modes) > 0 {
			mode |= modes[0][name] & 0111
		}
		header.SetMode(mode)
		file, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err = file.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

func ExportExpert(ctx context.Context, item domain.Expert) ([]byte, error) {
	return ExportExpertContent(ctx, item, nil)
}
