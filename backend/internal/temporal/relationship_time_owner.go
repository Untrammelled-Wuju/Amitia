package temporal

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func RelationshipOwnerSpace(spaceID, ownerID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("relationship-time\x00"+spaceID+"\x00"+ownerID)).String()
}

func (r *RelationshipTimeRepository) settingsID(characterID string) string {
	if r.ownerID == "" {
		return characterID
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("relationship-settings\x00"+r.ownerID+"\x00"+characterID)).String()
}

func (c *RelationshipTimeCoordinator) ForOwner(ownerID string) *RelationshipTimeCoordinator {
	repo := c.repo.WithDB(c.repo.db)
	repo.ownerID = ownerID
	return NewRelationshipTimeCoordinator(repo, c.clock)
}

func (c *RelationshipTimeCoordinator) MigrateOwner(ctx context.Context, spaceID, ownerID string) error {
	target := RelationshipOwnerSpace(spaceID, ownerID)
	return c.repo.WithTransaction(ctx, func(repo *RelationshipTimeRepository) error {
		for _, model := range []any{&GlobalPresenceState{}, &RelationshipPresenceState{}, &CadenceSample{}, &ReunionEpisode{}, &InteractionReceipt{}, &TemporalEffectLedgerEntry{}} {
			if err := repo.db.WithContext(ctx).Model(model).Where("space_id=?", spaceID).Update("space_id", target).Error; err != nil {
				return err
			}
		}
		var settings []RelationshipTimeSettings
		if err := repo.db.WithContext(ctx).Find(&settings).Error; err != nil {
			return err
		}
		owned := c.ForOwner(ownerID)
		for _, setting := range settings {
			var exists int64
			if err := repo.db.WithContext(ctx).Table("characters").Where("id=?", setting.CharacterID).Count(&exists).Error; err != nil {
				return err
			}
			if exists == 0 {
				continue
			}
			scoped := owned.repo.settingsID(setting.CharacterID)
			var current RelationshipTimeSettings
			err := repo.db.WithContext(ctx).Where("character_id=?", scoped).First(&current).Error
			if err == gorm.ErrRecordNotFound {
				if err := repo.db.WithContext(ctx).Model(&RelationshipTimeSettings{}).Where("character_id=?", setting.CharacterID).Update("character_id", scoped).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		return nil
	})
}
