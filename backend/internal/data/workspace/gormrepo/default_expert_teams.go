package gormrepo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/defaultresources"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func seedExpertTeam(tx *gorm.DB, owner string, definition defaultresources.Team, skills map[string]string) error {
	digest := contentDigest(definition)
	seed, seedErr := readSeed(tx, "expert_team", definition.Key, definition.Version, digest)
	if seedErr != nil && !errors.Is(seedErr, gorm.ErrRecordNotFound) {
		return seedErr
	}
	if seedErr == nil && !seed.Managed {
		return nil
	}
	var row expertTeamRecord
	err := tx.Where("system_key=?", definition.Key).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if seedErr == nil {
			return fmt.Errorf("managed default Expert Team is missing")
		}
		if err = tx.Where("owner_user_id=? AND lower(btrim(name))=?", owner, normalizeResourceName(definition.Definition.Name)).Take(&row).Error; err == nil {
			return saveSeed(tx, "expert_team", definition.Key, row.ID, definition.Version, digest, false)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = expertTeamRecord{ID: uuid.NewString(), OwnerID: owner, SystemKey: definition.Key, SystemManaged: true, Version: 1, CreatedAt: time.Now().UTC()}
	} else {
		if !row.SystemManaged || row.OwnerID != owner {
			return fmt.Errorf("default Expert Team has invalid ownership")
		}
		if seedErr == nil && seed.CatalogVersion == definition.Version && seed.ContentSHA256 == digest {
			return nil
		}
		row.Version++
	}
	input := definition.Definition
	for i, member := range input.Members {
		if member.Definition == nil {
			return fmt.Errorf("default Team requires independent members")
		}
		copy := *member.Definition
		copy.SkillIDs = nil
		for _, key := range definition.MemberSkillKeys[member.ID] {
			if skills[key] == "" {
				return fmt.Errorf("default Team references missing Skill %s", key)
			}
			copy.SkillIDs = append(copy.SkillIDs, skills[key])
		}
		input.Members[i].Definition = &copy
	}
	if err := input.Validate(); err != nil {
		return err
	}
	if err := input.ValidateLead(); err != nil {
		return err
	}
	owned, err := ownTeamMembers(tx, owner, input, nil)
	if err != nil {
		return err
	}
	row.Members, err = json.Marshal(owned)
	if err != nil {
		return err
	}
	row.StarterPrompts = jsonBytes(nonNilStrings(input.StarterPrompts))
	row.Name = input.Name
	row.Introduction = input.Introduction
	row.CoreCapability = input.CoreCapability
	row.Icon = defaultString(input.Icon, "users")
	row.IconBackground = defaultString(input.IconBackground, "sage")
	row.LeadMemberID = input.LeadMemberID
	row.ExpertIDs = []byte("[]")
	row.ExpertiseTags = []byte("[]")
	row.UpdatedAt = time.Now().UTC()
	if err := tx.Save(&row).Error; err != nil {
		return err
	}
	return saveSeed(tx, "expert_team", definition.Key, row.ID, definition.Version, digest, true)
}
