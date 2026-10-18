package expertpackage

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/skillstore"
)

type BundledSkill struct {
	Name    string
	SHA256  string
	Archive []byte
}

func readBundledSkills(ctx context.Context, manifest Manifest, files map[string][]byte, modes map[string]fs.FileMode, used map[string]bool) (map[string]BundledSkill, error) {
	profiles := []*Profile{}
	if manifest.Expert != nil {
		profiles = append(profiles, manifest.Expert)
	}
	if manifest.Team != nil {
		for i := range manifest.Team.Members {
			profiles = append(profiles, &manifest.Team.Members[i].Expert)
		}
	}
	result := map[string]BundledSkill{}
	for _, profile := range profiles {
		if len(profile.BundledSkills) > 50 {
			return nil, fmt.Errorf("%w: too many bundled Skills", domain.ErrInvalid)
		}
		seen := map[string]bool{}
		for _, directory := range profile.BundledSkills {
			if path.Dir(directory) != "skills" || !packageIdentity.MatchString(path.Base(directory)) || seen[directory] {
				return nil, fmt.Errorf("%w: invalid bundled Skill directory", domain.ErrInvalid)
			}
			seen[directory] = true
			if _, ok := result[directory]; ok {
				continue
			}
			entries := map[string][]byte{}
			permissions := map[string]fs.FileMode{}
			for name, data := range files {
				if strings.HasPrefix(name, directory+"/") {
					relative := strings.TrimPrefix(name, directory+"/")
					entries[relative] = data
					permissions[relative] = modes[name]
					used[name] = true
				}
			}
			archive, err := archiveFiles(entries, permissions)
			if err != nil {
				return nil, err
			}
			normalized, digest, metadata, err := skillstore.ValidateUpload(ctx, archive)
			if err != nil {
				return nil, fmt.Errorf("%w: bundled Skill %s: %v", domain.ErrInvalid, directory, err)
			}
			result[directory] = BundledSkill{Name: metadata.DisplayName, SHA256: digest, Archive: normalized}
		}
	}
	return result, nil
}
