package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/relationship"
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
	MemSvc    memory.Service
	EpicSvc   episodic.Service
	ExtSvc    *extension.Runtime
	Workspace *workspace.Service
}

func buildDataPortabilityCoordinator(deps dataPortabilityDeps) (*dataportability.Coordinator, error) {
	appVersion := os.Getenv("AMITIA_VERSION")
	if appVersion == "" {
		appVersion = "26.2.0-beta"
	}
	schemaFinger := fmt.Sprintf("mig-%d-%s", len(migration.DefaultMigrations()), runtime.GOOS)

	staging := dataportability.NewStagingManager(deps.DataDir)
	coord := dataportability.NewCoordinator(deps.DataDir, appVersion, runtime.GOOS, schemaFinger, staging)

	contributors := []dataportability.BackupContributor{
		dataportability.NewCharacterContributor(deps.DB),
		memory.NewMemoryBackupContributor(deps.MemSvc),
		chat.NewChatBackupContributor(deps.DB),
		episodic.NewEpisodicBackupContributor(deps.DB),
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
