package expertpackage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"github.com/google/uuid"
)

// InstallBundledSkills writes validated immutable content into a caller-owned
// namespace. It never creates Skill catalog resources or external account grants.
func (pkg *Package) InstallBundledSkills(ctx context.Context, objects objectstore.Provider, owner, namespace string) (cleanup func(), err error) {
	keys := []string{}
	cleanup = func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		for _, key := range keys {
			_ = objects.Delete(cleanupCtx, key)
		}
	}
	if len(pkg.BundledSkills) > 0 && objects == nil {
		return cleanup, fmt.Errorf("bundled Skill object storage is unavailable")
	}
	installed := map[string]domain.SkillSnapshot{}
	directories := []string{}
	for directory := range pkg.BundledSkills {
		directories = append(directories, directory)
	}
	sort.Strings(directories)
	for _, directory := range directories {
		bundle := pkg.BundledSkills[directory]
		key := "expert-packages/" + owner + "/" + namespace + "/" + directory + ".zip"
		stored, statErr := objects.Stat(ctx, key)
		if errors.Is(statErr, objectstore.ErrNotFound) {
			stored, statErr = objects.Put(ctx, key, bytes.NewReader(bundle.Archive), objectstore.PutOptions{Size: int64(len(bundle.Archive)), SHA256: bundle.SHA256, ContentType: "application/zip"})
			if statErr == nil {
				keys = append(keys, key)
			}
		}
		if statErr != nil {
			return cleanup, statErr
		}
		if stored.SHA256 != bundle.SHA256 || stored.Size != int64(len(bundle.Archive)) {
			return cleanup, fmt.Errorf("bundled Skill object integrity mismatch")
		}
		installed[directory] = domain.SkillSnapshot{ID: uuid.NewSHA1(uuid.Nil, []byte(owner+":"+pkg.Manifest.ID+":"+directory+":"+bundle.SHA256)).String(), Name: bundle.Name, ObjectKey: key, SHA256: bundle.SHA256}
	}
	bindings := func(profile Profile) []domain.SkillSnapshot {
		result := []domain.SkillSnapshot{}
		for _, directory := range profile.BundledSkills {
			result = append(result, installed[directory])
		}
		return result
	}
	if pkg.Manifest.Expert != nil {
		pkg.Expert.BundledSkills = bindings(*pkg.Manifest.Expert)
	}
	if pkg.Manifest.Team != nil {
		for i, member := range pkg.Manifest.Team.Members {
			pkg.Team.Members[i].Definition.BundledSkills = bindings(member.Expert)
		}
	}
	return cleanup, nil
}
