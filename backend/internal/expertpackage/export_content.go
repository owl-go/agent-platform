package expertpackage

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
)

func exportProfile(ctx context.Context, expert domain.Expert, stem string, objects objectstore.Provider, files map[string][]byte, modes map[string]fs.FileMode) (Profile, error) {
	if len(expert.MCPServerIDs)+len(expert.SkillIDs)+len(expert.CLIConnectorDefinitionIDs) > 0 {
		return Profile{}, fmt.Errorf("%w: bound resources need portable declarations", domain.ErrInvalid)
	}
	profile := Profile{Connectors: expert.ConnectorDependencies, Name: expert.Name, Introduction: expert.Introduction, IconBackground: expert.IconBackground, StarterPrompts: expert.StarterPrompts, GuidanceFile: "agents/" + stem + ".md"}
	var err error
	profile.Icon, profile.AvatarFile, err = exportAvatar(expert.Icon, "avatars/"+stem, files)
	if err != nil {
		return profile, err
	}
	files[profile.GuidanceFile] = []byte(expert.Guidance)
	for _, skill := range expert.BundledSkills {
		if objects == nil {
			return profile, fmt.Errorf("%w: bundled Skill content is unavailable", domain.ErrInvalid)
		}
		reader, metadata, err := objects.Get(ctx, skill.ObjectKey)
		if err != nil {
			return profile, err
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, MaxBytes+1))
		_ = reader.Close()
		sum := sha256.Sum256(data)
		if readErr != nil || int64(len(data)) != metadata.Size || len(data) > MaxBytes || hex.EncodeToString(sum[:]) != skill.SHA256 {
			return profile, fmt.Errorf("%w: bundled Skill integrity mismatch", domain.ErrInvalid)
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return profile, err
		}
		directory := "skills/" + skill.ID
		profile.BundledSkills = append(profile.BundledSkills, directory)
		for _, entry := range archive.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			stream, err := entry.Open()
			if err != nil {
				return profile, err
			}
			content, err := io.ReadAll(io.LimitReader(stream, MaxBytes+1))
			_ = stream.Close()
			if err != nil {
				return profile, err
			}
			name := directory + "/" + entry.Name
			files[name] = content
			modes[name] = entry.Mode().Perm()
		}
	}
	return profile, nil
}
func exportContent(ctx context.Context, manifest Manifest, files map[string][]byte, modes map[string]fs.FileMode) ([]byte, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	files[".plugin/plugin.json"] = data
	archive, err := archiveFiles(files, modes)
	if err != nil {
		return nil, err
	}
	pkg, err := Parse(ctx, archive)
	if err != nil {
		return nil, err
	}
	return pkg.Archive()
}
func ExportExpertContent(ctx context.Context, item domain.Expert, objects objectstore.Provider) ([]byte, error) {
	files := map[string][]byte{}
	modes := map[string]fs.FileMode{}
	profile, err := exportProfile(ctx, item, "expert", objects, files, modes)
	if err != nil {
		return nil, err
	}
	return exportContent(ctx, Manifest{SchemaVersion: 1, ID: "custom." + item.ID, Version: fmt.Sprintf("0.0.%d", item.Version), Kind: "expert", Expert: &profile}, files, modes)
}
func ExportTeamContent(ctx context.Context, item domain.ExpertTeam, objects objectstore.Provider) ([]byte, error) {
	files := map[string][]byte{}
	modes := map[string]fs.FileMode{}
	profile := TeamProfile{Name: item.Name, Introduction: item.Introduction, IconBackground: item.IconBackground, CoreCapability: item.CoreCapability, LeadMemberID: item.LeadMemberID, StarterPrompts: item.StarterPrompts}
	var err error
	profile.Icon, profile.AvatarFile, err = exportAvatar(item.Icon, "avatars/team", files)
	if err != nil {
		return nil, err
	}
	for _, member := range item.Members {
		expert, err := exportProfile(ctx, member.Expert, member.ID, objects, files, modes)
		if err != nil {
			return nil, err
		}
		profile.Members = append(profile.Members, Member{ID: member.ID, Name: member.Name, Labels: member.Labels, Expert: expert})
	}
	return exportContent(ctx, Manifest{SchemaVersion: 1, ID: "custom." + item.ID, Version: fmt.Sprintf("0.0.%d", item.Version), Kind: "expert_team", Team: &profile}, files, modes)
}
