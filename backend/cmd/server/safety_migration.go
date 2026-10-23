package main

import (
	"context"

	"github.com/u-ai/backend/internal/safety"
	"gorm.io/gorm"
)

func migrateLegacySafetyGovernance(ctx context.Context, db *gorm.DB, spaceID, deviceID, coreID, activeOwnerID string) error {
	repository := safety.NewRepository(db)
	if err := repository.ImportLegacy(ctx, []safety.PolicyScope{
		{SpaceID: spaceID, OwnerID: coreID, CoreID: coreID},
		{SpaceID: spaceID, OwnerID: deviceID, CoreID: coreID},
	}); err != nil {
		return err
	}
	return repository.MigrateLegacyEvents(ctx, safety.PolicyScope{SpaceID: spaceID, OwnerID: activeOwnerID, CoreID: coreID})
}
