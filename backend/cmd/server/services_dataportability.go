package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/relationship"
	"github.com/u-ai/backend/internal/reminder"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/internal/workspace"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type dataPortabilityDeps struct {
	DataDir   string
	DB        *gorm.DB
	Memory    business.MemoryArchivePort
	ExtSvc    *extension.Runtime
	Workspace *workspace.Service
}

func buildDataPortabilityCoordinator(deps dataPortabilityDeps) (*dataportability.Coordinator, error) {
	schemaFinger := fmt.Sprintf("mig-%d-%s", len(migration.DefaultMigrations()), runtime.GOOS)

	staging := dataportability.NewStagingManager(deps.DataDir)
	coord := dataportability.NewCoordinator(deps.DataDir, config.AppVersion(), runtime.GOOS, schemaFinger, staging)

	contributors := []dataportability.BackupContributor{
		dataportability.NewCharacterContributor(deps.DB),
		business.NewMemoryBackupContributor(deps.Memory),
		chat.NewChatBackupContributor(deps.DB),
		relationship.NewRelationshipBackupContributor(deps.DB),
		psyche.NewPsycheBackupContributor(deps.DB),
		worldbook.NewWorldbookBackupContributor(deps.DB),
		system.NewSettingsBackupContributor(deps.DB),
		chat.NewModelConfigBackupContributor(deps.DB),
		tts.NewVoiceBackupContributor(deps.DB),
		embedding_config.NewEmbeddingBackupContributor(deps.DB),
		system.NewResourceBackupContributor(deps.DB, deps.DataDir),
		extension.NewExtensionBackupContributor(deps.DB),
		workspace.NewWorkspaceBackupContributor(deps.DB),
	}

	if err := coord.RegisterContributors(contributors...); err != nil {
		return nil, fmt.Errorf("register data portability contributors: %w", err)
	}

	log.Info("DataPortability Coordinator initialized", "contributors", len(contributors))
	return coord, nil
}

func registerReminderDataPortability(services *AppServices, dataDir string) error {
	if services.Reminders == nil || services.DataPortability == nil {
		return nil
	}
	signer, err := agent.NewIdentityStore(dataDir).CryptoSigner()
	if err != nil {
		return err
	}
	contributor := reminder.NewTaskBackupContributor(func(ctx context.Context) (*reminder.Service, reminder.Caller, error) {
		actor, ok := auth.FromContext(ctx)
		if !ok || actor == nil || actor.SpaceID == "" {
			return nil, reminder.Caller{}, reminder.ErrForbidden
		}
		device := actor.DeviceID.String()
		if device == "" && actor.IsLocalTrusted && actor.PrincipalType == auth.PrincipalLocalUI && services.DeviceMesh != nil {
			device = services.DeviceMesh.LocalDeviceID
		}
		if device == "" {
			return nil, reminder.Caller{}, reminder.ErrForbidden
		}
		return services.Reminders, reminder.Caller{SpaceID: actor.SpaceID.String(), DeviceID: device}, nil
	}, signer)
	return services.DataPortability.RegisterContributors(contributor)
}
