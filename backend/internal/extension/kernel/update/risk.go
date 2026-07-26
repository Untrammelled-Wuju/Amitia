package update

import (
	"time"
)

type RollbackLevel string

const (
	RollbackLevelFull                RollbackLevel = "full"
	RollbackLevelCodeOnly            RollbackLevel = "code_only"
	RollbackLevelDataSnapshotRequired RollbackLevel = "data_snapshot_required"
	RollbackLevelManual              RollbackLevel = "manual"
	RollbackLevelNotSupported        RollbackLevel = "not_supported"
)

type SwitchStrategy string

const (
	SwitchStopThenStart SwitchStrategy = "stop_then_start"
	SwitchStartThenSwitch SwitchStrategy = "start_then_switch"
	SwitchParallelCanary SwitchStrategy = "parallel_canary"
	SwitchManual         SwitchStrategy = "manual"
)

type RiskLevel string

const (
	RiskNone     RiskLevel = "none"
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type UpdateRisk struct {
	Level       RiskLevel
	Reasons     []string
	BreakingChanges []BreakingChange
	PermissionExpansion bool
	ScopeExpansion      bool
	HasIrreversibleMigration bool
	HasOwnershipTransfer  bool
}

func ClassifyRisk(diff DefinitionDiff) UpdateRisk {
	risk := UpdateRisk{
		BreakingChanges:           diff.BreakingChanges,
		PermissionExpansion:       diff.PermissionExpanded,
		ScopeExpansion:            diff.ScopeExpanded,
		HasIrreversibleMigration:  diff.HasHighRiskMigration,
		HasOwnershipTransfer:      diff.PublisherChanged,
	}

	if diff.PublisherChanged {
		risk.Level = RiskCritical
		risk.Reasons = append(risk.Reasons, "ownership transfer")
	}

	for _, bc := range diff.BreakingChanges {
		if bc.Severity == "critical" {
			risk.Level = RiskCritical
			risk.Reasons = append(risk.Reasons, bc.Field+": "+bc.Reason)
		} else if bc.Severity == "high" && risk.Level != RiskCritical {
			risk.Level = RiskHigh
			risk.Reasons = append(risk.Reasons, bc.Field+": "+bc.Reason)
		}
	}

	if diff.HasHighRiskMigration && risk.Level != RiskCritical {
		risk.Level = RiskHigh
		risk.Reasons = append(risk.Reasons, "irreversible migration present")
	}

	if diff.PermissionExpanded && risk.Level == RiskNone {
		risk.Level = RiskMedium
		risk.Reasons = append(risk.Reasons, "permission expansion")
	}

	if diff.SignatureKeyChanged && risk.Level == RiskNone {
		risk.Level = RiskMedium
		risk.Reasons = append(risk.Reasons, "signature key changed")
	}

	if len(diff.ModulesAdded) > 0 || len(diff.ContributionsAdded) > 0 {
		if risk.Level == RiskNone {
			risk.Level = RiskLow
			risk.Reasons = append(risk.Reasons, "new modules or contributions")
		}
	}

	if risk.Level == "" {
		risk.Level = RiskNone
	}

	return risk
}

func DetermineRollbackLevel(diff DefinitionDiff, migrations []MigrationSnapshot) RollbackLevel {
	if diff.PublisherChanged {
		return RollbackLevelDataSnapshotRequired
	}
	hasIrreversible := false
	for _, m := range migrations {
		if !m.Reversible {
			hasIrreversible = true
			break
		}
	}
	if hasIrreversible {
		return RollbackLevelDataSnapshotRequired
	}
	if diff.HasBreakingChanges {
		return RollbackLevelCodeOnly
	}
	return RollbackLevelFull
}

func DetermineSwitchStrategy(diff DefinitionDiff, runtimeSupportsParallel bool) SwitchStrategy {
	if diff.HasBreakingChanges {
		return SwitchStopThenStart
	}
	if diff.PublisherChanged || diff.PermissionExpanded {
		return SwitchStopThenStart
	}
	if runtimeSupportsParallel && diff.UpdateType == UpdateTypeMinor {
		return SwitchStartThenSwitch
	}
	return SwitchStopThenStart
}

type UpdatePlan struct {
	PlanID            string
	ExtensionID       string
	OldVersion        string
	NewVersion        string
	UpdateType        UpdateType
	Diff              DefinitionDiff
	Risk              UpdateRisk
	SwitchStrategy    SwitchStrategy
	RollbackLevel     RollbackLevel
	Migrations        []MigrationPlan
	Confirmations     []string
	RequiresUserConfirm bool
	AutoUpdateEligible bool
	EstimatedDowntime time.Duration
	RetainRollbackPoint bool
	CreatedAt         time.Time
}

type MigrationPlan struct {
	MigrationID   string
	FromRange     string
	ToRange       string
	RuntimeType   string
	Entry         string
	Reversible    bool
	Namespaces    []string
	RequiresSnapshot bool
	HighRisk      bool
}

func BuildPlan(planID string, old, new DefinitionSnapshot, migrations []MigrationSnapshot) UpdatePlan {
	diff := ComputeDiff(old, new)
	risk := ClassifyRisk(diff)
	strategy := DetermineSwitchStrategy(diff, false)
	rollbackLevel := DetermineRollbackLevel(diff, migrations)

	plan := UpdatePlan{
		PlanID:            planID,
		ExtensionID:       new.ExtensionID,
		OldVersion:        old.Version,
		NewVersion:        new.Version,
		UpdateType:        diff.UpdateType,
		Diff:              diff,
		Risk:              risk,
		SwitchStrategy:    strategy,
		RollbackLevel:     rollbackLevel,
		CreatedAt:         time.Now().UTC(),
		RetainRollbackPoint: true,
	}

	for _, m := range diff.MigrationsAdded {
		plan.Migrations = append(plan.Migrations, MigrationPlan{
			MigrationID:      m.ID,
			FromRange:        m.FromRange,
			ToRange:          m.ToRange,
			RuntimeType:      m.RuntimeType,
			Entry:            m.Entry,
			Reversible:       m.Reversible,
			HighRisk:         !m.Reversible,
			RequiresSnapshot: !m.Reversible,
		})
	}

	if diff.PermissionExpanded {
		plan.Confirmations = append(plan.Confirmations, "permission_expansion")
		plan.RequiresUserConfirm = true
	}
	if diff.PublisherChanged {
		plan.Confirmations = append(plan.Confirmations, "ownership_transfer")
		plan.RequiresUserConfirm = true
	}
	if diff.HasHighRiskMigration {
		plan.Confirmations = append(plan.Confirmations, "irreversible_migration")
		plan.RequiresUserConfirm = true
	}
	if diff.HasBreakingChanges {
		plan.Confirmations = append(plan.Confirmations, "breaking_changes")
		plan.RequiresUserConfirm = true
	}

	plan.AutoUpdateEligible = !plan.RequiresUserConfirm &&
		!diff.PublisherChanged &&
		!diff.PermissionExpanded &&
		!diff.HasHighRiskMigration &&
		!diff.HasBreakingChanges

	switch plan.SwitchStrategy {
	case SwitchStopThenStart:
		plan.EstimatedDowntime = 30 * time.Second
	case SwitchStartThenSwitch:
		plan.EstimatedDowntime = 2 * time.Second
	case SwitchParallelCanary:
		plan.EstimatedDowntime = 0
	default:
		plan.EstimatedDowntime = 60 * time.Second
	}

	return plan
}
