package desktop_update

type PermissionDiff struct {
	Added   []string
	Removed []string
	Changed []string
}

type ScopeDiff struct {
	Expanded bool
	Details  []string
}

type RuntimeDiff struct {
	AddedRuntimes   []string
	RemovedRuntimes []string
	TypeUpgraded    bool
}

type ContributionDiff struct {
	AddedDesktopContributions []string
	AddedGlobalShortcuts      []string
}

type MigrationPlan struct {
	HasMigration         bool
	IsReversible         bool
	RequiresConfirmation bool
}

type RuntimeDrainPlan struct {
	StopNewInvocations bool
	CancelTimeoutSec   int
	PreserveArtifact   bool
}

type ResourcePlan struct {
	ReleaseShortcuts bool
	ReleaseTrayItems bool
	ReleaseMenuItems bool
}

type RollbackPlan struct {
	CanRollback     bool
	RollbackLevel   string
	PreserveOldData bool
}

const (
	RollbackLevelNone         = "none"
	RollbackLevelSoft         = "soft"
	RollbackLevelDataSnapshot = "data_snapshot"
	RollbackLevelFullRestore  = "full_restore"
)

type ExtensionUpdatePlan struct {
	OperationID              string
	ExtensionID              string
	FromVersion              string
	ToVersion                string
	SourceMetadata           ExtensionUpdateMetadata
	PermissionDiff           PermissionDiff
	ScopeDiff                ScopeDiff
	RuntimeDiff              RuntimeDiff
	ContributionDiff         ContributionDiff
	MigrationPlan            *MigrationPlan
	RuntimeDrainPlan         RuntimeDrainPlan
	ResourcePlan             ResourcePlan
	RollbackPlan             RollbackPlan
	Status                   string
	Generation               int64
	AutoUpdateEligible       bool
	RequiresUserConfirmation bool
}

func BuildUpdatePlan(operationID, extensionID, fromVersion string, metadata ExtensionUpdateMetadata) *ExtensionUpdatePlan {
	plan := &ExtensionUpdatePlan{
		OperationID:    operationID,
		ExtensionID:    extensionID,
		FromVersion:    fromVersion,
		ToVersion:      metadata.Version,
		SourceMetadata: metadata,
		Status:         string(StateCreated),
		Generation:     0,
	}

	if metadata.HasMigration() {
		plan.MigrationPlan = &MigrationPlan{
			HasMigration:         metadata.Migration.HasMigration,
			IsReversible:         metadata.Migration.IsReversible,
			RequiresConfirmation: metadata.Migration.RequiresManualConfirmation,
		}
	} else {
		plan.MigrationPlan = &MigrationPlan{
			HasMigration: false,
			IsReversible: true,
		}
	}

	plan.RuntimeDrainPlan = RuntimeDrainPlan{
		StopNewInvocations: true,
		CancelTimeoutSec:   30,
		PreserveArtifact:   plan.MigrationPlan.IsReversible,
	}

	plan.ResourcePlan = ResourcePlan{
		ReleaseShortcuts: len(plan.ContributionDiff.AddedGlobalShortcuts) > 0,
		ReleaseTrayItems: len(plan.ContributionDiff.AddedDesktopContributions) > 0,
		ReleaseMenuItems: true,
	}

	if metadata.RollbackPolicy == "none" || (metadata.Migration != nil && metadata.Migration.RollbackNotSupported) {
		plan.RollbackPlan = RollbackPlan{
			CanRollback:     false,
			RollbackLevel:   RollbackLevelNone,
			PreserveOldData: false,
		}
	} else if !plan.MigrationPlan.IsReversible {
		plan.RollbackPlan = RollbackPlan{
			CanRollback:     true,
			RollbackLevel:   RollbackLevelDataSnapshot,
			PreserveOldData: true,
		}
	} else {
		plan.RollbackPlan = RollbackPlan{
			CanRollback:     true,
			RollbackLevel:   RollbackLevelSoft,
			PreserveOldData: false,
		}
	}

	plan.RequiresUserConfirmation = plan.MigrationPlan.RequiresConfirmation ||
		plan.ScopeDiff.Expanded ||
		len(plan.PermissionDiff.Added) > 0 ||
		!plan.RollbackPlan.CanRollback ||
		metadata.PublisherKeyID == ""

	updateType, _ := CompareVersions(fromVersion, metadata.Version)
	plan.AutoUpdateEligible = !plan.RequiresUserConfirmation &&
		updateType == UpdateTypePatch &&
		metadata.ReleaseChannel == "stable"

	return plan
}

func (p *ExtensionUpdatePlan) IsHighRisk() bool {
	if !p.MigrationPlan.IsReversible {
		return true
	}
	if p.ScopeDiff.Expanded {
		return true
	}
	if len(p.PermissionDiff.Added) > 0 {
		return true
	}
	if !p.RollbackPlan.CanRollback {
		return true
	}
	if len(p.RuntimeDiff.RemovedRuntimes) > 0 {
		return true
	}
	return false
}

func (p *ExtensionUpdatePlan) IsDowngrade() bool {
	if p.FromVersion == "" {
		return false
	}
	downgrade, _ := IsDowngrade(p.FromVersion, p.ToVersion)
	return downgrade
}
