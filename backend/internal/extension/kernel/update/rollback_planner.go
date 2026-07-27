package update

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RollbackPlanner struct {
	points      *RollbackPointStore
	assessor    *SideEffectAssessor
	generations *GenerationManager
}

func NewRollbackPlanner(points *RollbackPointStore, assessor *SideEffectAssessor, generations *GenerationManager) *RollbackPlanner {
	return &RollbackPlanner{
		points:      points,
		assessor:    assessor,
		generations: generations,
	}
}

func (p *RollbackPlanner) PlanRollback(ctx context.Context, extensionID string, fromGen, toGen int64, point *RollbackPoint, level RollbackLevel) (*RollbackPlan, error) {
	plan := &RollbackPlan{
		RollbackID:     uuid.New().String(),
		OperationID:    uuid.New().String(),
		ExtensionID:    extensionID,
		FromGeneration: fromGen,
		ToGeneration:   toGen,
		Level:          level,
		Status:         RollbackStatusPlanning,
	}
	now := time.Now().UTC()
	plan.StartedAt = &now

	if point != nil {
		plan.ArtifactPlan = ArtifactRollbackPlan{
			OldArtifactPath:   point.ArtifactPath,
			OldPackageHash:    point.PackageHash,
			OldSignatureKeyID: point.SignatureKeyID,
			CleanupNew:        true,
		}
		plan.DefinitionPlan = DefinitionRollbackPlan{
			OldDefinitionHash:    point.DefinitionHash,
			RestoreModules:       true,
			RestoreContributions: true,
		}
		plan.RuntimePlan = RuntimeRollbackPlan{
			StopNewFirst: true,
			RestartOld:   true,
		}
		plan.DataPlan = DataRollbackPlan{
			SnapshotID:       point.StorageSnapshotID,
			RequiresSnapshot: point.StorageSnapshotID != "",
			StopWritesFirst:  true,
		}
		if level == RollbackLevelDataAndGeneration || level == RollbackLevelDataSnapshotRequired {
			plan.DataPlan.RequiresReverse = true
		}

		grantsToRestore := make([]string, 0, len(point.Permissions))
		for _, perm := range point.Permissions {
			grantsToRestore = append(grantsToRestore, perm.ID)
		}
		plan.PermissionPlan = PermissionRollbackPlan{
			GrantsToRestore:  grantsToRestore,
			RecomputeFromOld: true,
		}

		bindingsToRestore := make([]string, 0, len(point.ScopeReferences))
		for _, scope := range point.ScopeReferences {
			bindingsToRestore = append(bindingsToRestore, scope.ScopeID)
		}
		plan.ScopePlan = ScopeRollbackPlan{
			BindingsToRestore: bindingsToRestore,
			RecomputeFromOld:  true,
		}

		plan.DesktopPlan = DesktopRollbackPlan{
			CloseNewUI:            true,
			RestoreOldSnapshot:    true,
			UnregisterNewShortcut: true,
			RestoreOldShortcut:    true,
		}
		plan.UIPlan = UIRollbackPlan{
			CloseNewSessions:   true,
			RevokeNewBridge:    true,
			RestoreOldContrib:  true,
			RestoreOldSnapshot: true,
		}
		plan.BackgroundPlan = BackgroundRollbackPlan{
			TransferSchedule:   true,
			TransferEventSub:   true,
			TransferHook:       true,
			TransferMCP:        true,
			TransferTrustedSvc: true,
			UseOwnershipLease:  true,
			UseGenerationGate:  true,
		}
	}

	feasibility := p.assessFeasibility(ctx, extensionID, point, level)
	plan.Preconditions = p.buildPreconditions(feasibility)
	plan.Postconditions = p.buildPostconditions()

	if !feasibility.Feasible {
		plan.RequiresUserAction = true
		plan.Status = RollbackStatusManualIntervention
	}

	assessments, err := p.AssessSideEffects(ctx, extensionID, nil)
	if err != nil {
		return nil, fmt.Errorf("update: assess side effects: %w", err)
	}
	plan.SideEffectPlan = SideEffectRollbackPlan{
		Assessments:      assessments,
		HasNonReversible: p.assessor.HasNonReversible(ctx, assessments),
		RequiresManual:   p.assessor.RequiresManualIntervention(ctx, assessments),
	}
	if plan.SideEffectPlan.HasNonReversible {
		plan.SideEffectPlan.PartialRollback = true
		plan.RequiresUserAction = true
	}

	if plan.Status == RollbackStatusPlanning {
		plan.Status = RollbackStatusCreated
	}

	return plan, nil
}

func (p *RollbackPlanner) AssessSideEffects(ctx context.Context, extensionID string, contributions []string) ([]SideEffectAssessment, error) {
	infos := make([]ContributionSideEffectInfo, 0, len(contributions))
	for _, cID := range contributions {
		infos = append(infos, ContributionSideEffectInfo{
			ContributionID:  cID,
			SideEffectClass: "unknown",
		})
	}
	return p.assessor.Assess(ctx, extensionID, infos)
}

func (p *RollbackPlanner) DetermineRollbackLevel(ctx context.Context, failureType string, hasDataIncompatible bool, hasNonReversibleSE bool) RollbackLevel {
	if hasNonReversibleSE {
		return RollbackLevelManualRecovery
	}
	if hasDataIncompatible {
		return RollbackLevelDataAndGeneration
	}
	switch failureType {
	case "runtime_crash", "runtime_unhealthy", "runtime_oom":
		return RollbackLevelRuntimeOnly
	case "contribution_failure", "hook_failure", "ui_failure", "tool_failure":
		return RollbackLevelContributionOnly
	case "generation_failure", "full_extension_failure":
		return RollbackLevelGeneration
	case "data_incompatible", "data_corruption":
		return RollbackLevelDataAndGeneration
	case "manual_recovery":
		return RollbackLevelManualRecovery
	default:
		return RollbackLevelFullExtension
	}
}

func (p *RollbackPlanner) assessFeasibility(ctx context.Context, extensionID string, point *RollbackPoint, level RollbackLevel) RollbackFeasibility {
	f := RollbackFeasibility{
		Level: level,
	}
	if point == nil {
		f.Feasible = false
		f.Blockers = append(f.Blockers, "no rollback point available")
		f.RequiresUserAction = true
		return f
	}
	if point.ArtifactPath == "" {
		f.Blockers = append(f.Blockers, "old artifact path missing")
	}
	if point.PackageHash == "" {
		f.Blockers = append(f.Blockers, "old package hash missing")
	}
	if point.SignatureKeyID == "" {
		f.Blockers = append(f.Blockers, "old signature key missing")
	}
	if point.DefinitionHash == "" {
		f.Blockers = append(f.Blockers, "old definition hash missing")
	}
	if point.StorageSnapshotID != "" {
		f.RequiresDataRestore = true
	}
	if level == RollbackLevelDataAndGeneration || level == RollbackLevelDataSnapshotRequired {
		f.RequiresReverse = true
		if point.StorageSnapshotID == "" {
			f.Blockers = append(f.Blockers, "data snapshot required but missing")
		}
	}

	target, err := p.generations.RollbackTarget(ctx, extensionID)
	if err != nil || target == nil {
		f.Blockers = append(f.Blockers, "no rollback target generation available")
	}

	if len(point.Permissions) == 0 {
		f.Blockers = append(f.Blockers, "no permission snapshot available")
	}

	if level == RollbackLevelManualRecovery {
		f.HasNonReversibleSE = true
		f.RequiresUserAction = true
	}

	f.Feasible = len(f.Blockers) == 0 && level != RollbackLevelManualRecovery
	return f
}

func (p *RollbackPlanner) buildPreconditions(f RollbackFeasibility) []RollbackCondition {
	conds := []RollbackCondition{
		{
			Name: "old_artifact_intact", Type: "artifact", Required: true,
			Passed: f.Feasible, Detail: "old artifact must be intact and verifiable",
		},
		{
			Name: "old_signature_valid", Type: "artifact", Required: true,
			Passed: f.Feasible, Detail: "old signature key must be valid",
		},
		{
			Name: "old_definition_exists", Type: "definition", Required: true,
			Passed: f.Feasible, Detail: "old definition must exist",
		},
		{
			Name: "old_runtime_startable", Type: "runtime", Required: true,
			Passed: f.Feasible, Detail: "old runtime must be startable",
		},
		{
			Name: "data_compatible", Type: "data", Required: f.RequiresDataRestore,
			Passed: f.Feasible, Detail: "data must be compatible or restorable",
		},
		{
			Name: "permission_restorable", Type: "permission", Required: true,
			Passed: f.Feasible, Detail: "permissions must be restorable",
		},
		{
			Name: "scope_restorable", Type: "scope", Required: true,
			Passed: f.Feasible, Detail: "scope bindings must be restorable",
		},
		{
			Name: "dependencies_satisfied", Type: "dependency", Required: true,
			Passed: f.Feasible, Detail: "dependencies must still be satisfied",
		},
	}
	return conds
}

func (p *RollbackPlanner) buildPostconditions() []RollbackCondition {
	conds := []RollbackCondition{
		{
			Name: "old_runtime_ready", Type: "runtime", Required: true,
			Passed: false, Detail: "old runtime must be ready",
		},
		{
			Name: "old_contribution_active", Type: "contribution", Required: true,
			Passed: false, Detail: "old contributions must be active",
		},
		{
			Name: "no_new_gen_calls", Type: "generation", Required: true,
			Passed: false, Detail: "no calls to new generation",
		},
		{
			Name: "background_unique", Type: "background", Required: true,
			Passed: false, Detail: "background capabilities must be uniquely owned",
		},
		{
			Name: "storage_postcondition", Type: "data", Required: true,
			Passed: false, Detail: "storage postcondition must hold",
		},
	}
	return conds
}
