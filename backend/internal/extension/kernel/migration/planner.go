package migration

import (
	"fmt"
)

type MigrationPlanner struct {
	resolver *MigrationGraphResolver
}

func NewMigrationPlanner(resolver *MigrationGraphResolver) *MigrationPlanner {
	return &MigrationPlanner{
		resolver: resolver,
	}
}

func (p *MigrationPlanner) PlanMigration(input MigrationPlanInput) (*MigrationPlanOutput, error) {
	if p == nil || p.resolver == nil {
		return nil, fmt.Errorf("migration: planner resolver is nil")
	}
	if input.FromVersion == "" || input.ToVersion == "" {
		return nil, fmt.Errorf("migration: from_version and to_version are required")
	}
	graph, err := p.resolver.BuildGraph(input.AvailableMigrations)
	if err != nil {
		return nil, fmt.Errorf("migration: build graph failed: %w", err)
	}
	path, err := p.resolver.ResolvePath(graph, input.FromVersion, input.ToVersion)
	if err != nil {
		return nil, fmt.Errorf("migration: resolve path failed: %w", err)
	}
	definitions := p.collectDefinitions(path, input.AvailableMigrations)
	snapshotScope := p.calculateSnapshotScope(definitions)
	risk := p.estimateRisk(*path, definitions)
	space := p.estimateSpace(input)
	canRollback := p.canAutoRollback(*path, definitions)
	hasIrrev := p.hasIrreversible(definitions)
	requiresConfirm := hasIrrev || risk == "high" || risk == "critical"
	reversibility := p.inferOverallReversibility(definitions)
	if len(snapshotScope) == 0 {
		snapshotScope = []string{}
	}
	output := &MigrationPlanOutput{
		Path:                *path,
		SnapshotScope:       snapshotScope,
		EstimatedRisk:       risk,
		EstimatedSpaceBytes: space,
		CanAutoRollback:     canRollback,
		RequiresUserConfirm: requiresConfirm,
		HasIrreversible:     hasIrrev,
		Reversibility:       reversibility,
	}
	return output, nil
}

func (p *MigrationPlanner) collectDefinitions(path *MigrationPath, available []MigrationDefinition) []MigrationDefinition {
	if path == nil || len(path.Steps) == 0 {
		return nil
	}
	defMap := make(map[string]MigrationDefinition, len(available))
	for _, m := range available {
		defMap[m.MigrationID] = m
	}
	result := make([]MigrationDefinition, 0, len(path.Steps))
	for _, step := range path.Steps {
		if def, ok := defMap[step.MigrationID]; ok {
			result = append(result, def)
		}
	}
	return result
}

func (p *MigrationPlanner) calculateSnapshotScope(migrations []MigrationDefinition) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range migrations {
		for _, dd := range m.DataDomains {
			if dd.Domain == "" {
				continue
			}
			if !seen[dd.Domain] {
				seen[dd.Domain] = true
				result = append(result, dd.Domain)
			}
		}
	}
	return result
}

func (p *MigrationPlanner) estimateRisk(path MigrationPath, definitions []MigrationDefinition) string {
	if len(path.Steps) == 0 {
		return "none"
	}
	if len(definitions) == 0 {
		return "low"
	}
	hasIrrev := false
	hasNonIdempotent := false
	hasReverseScript := false
	hasSnapshotOnly := false
	for _, d := range definitions {
		switch d.Reversibility {
		case ReversibilityIrreversible:
			hasIrrev = true
		case ReversibilityReverseScriptRequired:
			hasReverseScript = true
		case ReversibilitySnapshotReversible:
			hasSnapshotOnly = true
		}
		if d.Idempotency == IdempotencyNonIdempotent {
			hasNonIdempotent = true
		}
	}
	if hasIrrev {
		return "critical"
	}
	if hasNonIdempotent {
		return "high"
	}
	if hasReverseScript {
		return "medium"
	}
	if hasSnapshotOnly {
		return "low"
	}
	return "none"
}

func (p *MigrationPlanner) canAutoRollback(path MigrationPath, definitions []MigrationDefinition) bool {
	if len(path.Steps) == 0 {
		return true
	}
	if len(definitions) == 0 {
		return true
	}
	for _, d := range definitions {
		if d.Reversibility != ReversibilityFullyReversible && d.Reversibility != ReversibilitySnapshotReversible {
			return false
		}
	}
	return true
}

func (p *MigrationPlanner) hasIrreversible(definitions []MigrationDefinition) bool {
	for _, d := range definitions {
		if d.Reversibility == ReversibilityIrreversible {
			return true
		}
	}
	return false
}

func (p *MigrationPlanner) estimateSpace(input MigrationPlanInput) int64 {
	const perMigrationBytes int64 = 1024 * 1024
	const perDomainBytes int64 = 512 * 1024
	const baseBytes int64 = 1024 * 1024
	migrationCount := int64(len(input.AvailableMigrations))
	domainSet := make(map[string]bool)
	for _, m := range input.AvailableMigrations {
		for _, dd := range m.DataDomains {
			if dd.Domain != "" {
				domainSet[dd.Domain] = true
			}
		}
	}
	domainCount := int64(len(domainSet))
	total := baseBytes + migrationCount*perMigrationBytes + domainCount*perDomainBytes
	safetyMargin := total / 5
	total += safetyMargin
	return total
}

func (p *MigrationPlanner) inferOverallReversibility(definitions []MigrationDefinition) Reversibility {
	if len(definitions) == 0 {
		return ReversibilityFullyReversible
	}
	hasIrrev := false
	hasReverseScript := false
	hasSnapshotOnly := false
	for _, d := range definitions {
		switch d.Reversibility {
		case ReversibilityIrreversible:
			hasIrrev = true
		case ReversibilityReverseScriptRequired:
			hasReverseScript = true
		case ReversibilitySnapshotReversible:
			hasSnapshotOnly = true
		}
	}
	if hasIrrev {
		return ReversibilityIrreversible
	}
	if hasReverseScript {
		return ReversibilityReverseScriptRequired
	}
	if hasSnapshotOnly {
		return ReversibilitySnapshotReversible
	}
	return ReversibilityFullyReversible
}
