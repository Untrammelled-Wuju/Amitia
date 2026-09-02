package workflow

import (
	"fmt"
	"strings"
)

const LegacyWorkflowSchemaVersion = "workflow-v1"

type WorkflowMigrationResult struct {
	Definition  WorkflowDefinition `json:"definition"`
	FromVersion string             `json:"fromVersion"`
	ToVersion   string             `json:"toVersion"`
	Migrated    bool               `json:"migrated"`
}

// WorkflowMigrator performs deterministic, in-memory schema upgrades. It does
// not overwrite the stored source revision; callers must explicitly save the
// migrated definition if they want to publish it as a new revision.
type WorkflowMigrator struct{}

func NewWorkflowMigrator() *WorkflowMigrator { return &WorkflowMigrator{} }

func (m *WorkflowMigrator) Migrate(def WorkflowDefinition) (WorkflowMigrationResult, error) {
	from := strings.TrimSpace(def.SchemaVersion)
	if from == "" {
		// Schema-less definitions are treated as newly-authored definitions, not
		// legacy persisted data. Defaults are applied by NormalizeDefinition.
		def.SchemaVersion = UserWorkflowSchemaVersion
		return WorkflowMigrationResult{Definition: def, FromVersion: "", ToVersion: UserWorkflowSchemaVersion}, nil
	}
	if from == UserWorkflowSchemaVersion {
		return WorkflowMigrationResult{Definition: def, FromVersion: from, ToVersion: from}, nil
	}

	switch from {
	case LegacyWorkflowSchemaVersion, "1.0", "1.0.0":
		if len(def.Edges) == 0 {
			def.Edges = DeriveEdges(def.Nodes)
		}
		def.SchemaVersion = UserWorkflowSchemaVersion
		def.ConcurrencyPolicy = def.ConcurrencyPolicy.Normalize()
		return WorkflowMigrationResult{
			Definition:  def,
			FromVersion: from,
			ToVersion:   UserWorkflowSchemaVersion,
			Migrated:    true,
		}, nil
	default:
		return WorkflowMigrationResult{}, fmt.Errorf("workflow: unsupported schema version %q (runtime supports %q)", from, UserWorkflowSchemaVersion)
	}
}

func MigrateDefinition(def WorkflowDefinition) (WorkflowDefinition, error) {
	result, err := NewWorkflowMigrator().Migrate(def)
	if err != nil {
		return def, err
	}
	return result.Definition, nil
}
