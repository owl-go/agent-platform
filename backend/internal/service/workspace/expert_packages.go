package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/expertpackage"
	"github.com/google/uuid"
)

func (service *Service) ImportExpertPackage(ctx context.Context, request *workspacev1.ImportExpertPackageRequest) (*workspacev1.ExpertPackageImport, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	pkg, err := expertpackage.Parse(ctx, request.Archive)
	if err != nil {
		return nil, publicError(err)
	}
	if pkg.Manifest.Kind == "expert_team" {
		if _, err := service.administrator(ctx); err != nil {
			return nil, err
		}
	}
	if request.CopyName != "" && (request.TargetResourceId != "" || request.ExpectedVersion != 0) {
		return nil, publicError(domain.ErrInvalid)
	}
	if name := strings.TrimSpace(request.CopyName); name != "" {
		digest := sha256.Sum256([]byte(owner + ":" + pkg.Manifest.ID + ":" + name))
		pkg.Manifest.ID = "copy." + hex.EncodeToString(digest[:16])
		if pkg.Manifest.Expert != nil {
			pkg.Manifest.Expert.Name = name
		} else {
			pkg.Manifest.Team.Name = name
		}
		archive, err := pkg.Archive()
		if err != nil {
			return nil, publicError(err)
		}
		pkg, err = expertpackage.Parse(ctx, archive)
		if err != nil {
			return nil, publicError(err)
		}
	}
	resolveSkills := func(keys []string) ([]string, error) {
		if len(keys) == 0 {
			return nil, nil
		}
		items, err := service.workspace.Repository().ListSkills(ctx, owner)
		if err != nil {
			return nil, err
		}
		ids := map[string]string{}
		for _, item := range items {
			ids[item.SystemKey] = item.ID
		}
		var result []string
		for _, key := range keys {
			if ids[key] == "" {
				return nil, domain.ErrInvalid
			}
			result = append(result, ids[key])
		}
		return result, nil
	}
	if pkg.Manifest.Expert != nil {
		pkg.Expert.SkillIDs, err = resolveSkills(pkg.Manifest.Expert.SkillKeys)
	} else {
		for i, member := range pkg.Manifest.Team.Members {
			pkg.Team.Members[i].Definition.SkillIDs, err = resolveSkills(member.Expert.SkillKeys)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		return nil, publicError(err)
	}
	repository, ok := service.workspace.Repository().(interface {
		ImportExpertPackage(context.Context, string, domain.ExpertPackageDefinition) (domain.ExpertPackageImport, error)
	})
	if !ok {
		return nil, publicError(domain.ErrInvalid)
	}
	archive, err := pkg.Archive()
	if err != nil {
		return nil, publicError(err)
	}
	cleanup, err := pkg.InstallBundledSkills(ctx, service.objects, owner, uuid.NewString())
	acceptedContent := false
	defer func() {
		if !acceptedContent {
			cleanup()
		}
	}()
	if err != nil {
		return nil, publicError(err)
	}
	definition := domain.ExpertPackageDefinition{PackageID: pkg.Manifest.ID, PackageVersion: pkg.Manifest.Version, SHA256: pkg.SHA256, Archive: archive, ExpectedVersion: request.ExpectedVersion, TargetResourceID: request.TargetResourceId}
	if pkg.Manifest.Kind == "expert" {
		definition.Expert = &pkg.Expert
	} else {
		definition.Team = &pkg.Team
	}
	imported, err := repository.ImportExpertPackage(ctx, owner, definition)
	acceptedContent = err == nil && !imported.Replayed
	if err != nil {
		return nil, publicError(err)
	}
	if imported.Team != nil {
		availability, err := service.teamExpertAvailability(ctx, []domain.ExpertTeam{*imported.Team})
		if err != nil {
			return nil, publicError(err)
		}
		return &workspacev1.ExpertPackageImport{ExpertTeam: expertTeamResponse(*imported.Team, availability), PackageId: pkg.Manifest.ID, PackageVersion: pkg.Manifest.Version, Replayed: imported.Replayed}, nil
	}
	item := *imported.Expert
	availability, err := service.expertAvailability(ctx, []domain.Expert{item})
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ExpertPackageImport{Expert: expertResponse(item, availability[item.ID]), PackageId: pkg.Manifest.ID, PackageVersion: pkg.Manifest.Version, Replayed: imported.Replayed}, nil
}

func (service *Service) ExportExpertPackage(ctx context.Context, request *workspacev1.ExportExpertPackageRequest) (*workspacev1.ExportedExpertPackage, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	var expert domain.Expert
	var team domain.ExpertTeam
	var version int64
	switch request.ResourceKind {
	case "expert":
		expert, err = service.workspace.Repository().GetExpert(ctx, owner, request.ResourceId)
		version = expert.Version
	case "expert_team":
		team, err = service.workspace.Repository().GetExpertTeam(ctx, owner, request.ResourceId)
		version = team.Version
	default:
		return nil, publicError(domain.ErrInvalid)
	}
	if err != nil {
		return nil, publicError(err)
	}
	var archive []byte
	if repository, ok := service.workspace.Repository().(interface {
		ImportedExpertPackage(context.Context, string, string, string, int64) ([]byte, error)
	}); ok {
		archive, err = repository.ImportedExpertPackage(ctx, owner, request.ResourceKind, request.ResourceId, version)
		if err != nil {
			return nil, publicError(err)
		}
	}
	if len(archive) == 0 {
		if repo, ok := service.workspace.Repository().(interface {
			ExportableExpert(context.Context, string, string) (domain.Expert, error)
			ExportableExpertTeam(context.Context, string, string) (domain.ExpertTeam, error)
		}); ok {
			if request.ResourceKind == "expert" {
				expert, err = repo.ExportableExpert(ctx, owner, request.ResourceId)
			} else {
				team, err = repo.ExportableExpertTeam(ctx, owner, request.ResourceId)
			}
			if err != nil {
				return nil, publicError(err)
			}
		}
		if request.ResourceKind == "expert" {
			archive, err = expertpackage.ExportExpertContent(ctx, expert, service.objects)
		} else {
			archive, err = expertpackage.ExportTeamContent(ctx, team, service.objects)
		}
		if err != nil {
			return nil, publicError(err)
		}
	}
	return &workspacev1.ExportedExpertPackage{Archive: archive, Filename: "expert-package.zip"}, nil
}
