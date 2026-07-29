package migration

import (
	"testing"
)

func makeTestMigration(id, fromRange, toVersion, reversibility string) MigrationDefinition {
	return MigrationDefinition{
		MigrationID:            id,
		ExtensionID:            "com.test/migration-test",
		ModuleID:               "main",
		FromVersionRange:       fromRange,
		ToVersion:              toVersion,
		Entry:                  "./migrations/forward.js",
		RuntimeType:            "javascript",
		Direction:              DirectionForward,
		DataDomains:            []DataDomain{{Domain: "extension_storage", Storage: "sqlite", Namespace: "test_data"}},
		Idempotency:            IdempotencyIdempotent,
		Reversibility:          Reversibility(reversibility),
		PermissionRequirements: []PermissionRequirement{{PermissionID: "storage:read", Scope: "extension"}},
		ScopeRule:              ScopeRule{BindingType: "module", ModuleIDs: []string{"main"}},
		ResourceLimits:         TaskResourceLimits{MaxMemoryMB: 256, MaxCPUPercent: 50, MaxDiskMB: 512, MaxDurationSecs: 300},
		DefinitionHash:         id,
		Precondition:           []MigrationCondition{},
		Postcondition:          []MigrationCondition{},
	}
}

func makeTestMigrationDefinitions() []MigrationDefinition {
	return []MigrationDefinition{
		makeTestMigration("mig-1.0.0-to-1.1.0", "1.0.0", "1.1.0", "fully_reversible"),
		makeTestMigration("mig-1.1.0-to-1.2.0", "1.1.0", "1.2.0", "fully_reversible"),
		makeTestMigration("mig-1.2.0-to-2.0.0", "1.2.0", "2.0.0", "snapshot_reversible"),
		makeTestMigration("mig-2.0.0-to-2.1.0", "2.0.0", "2.1.0", "fully_reversible"),
		makeTestMigration("mig-2.1.0-to-3.0.0", "2.1.0", "3.0.0", "reverse_script_required"),
		makeTestMigration("mig-3.0.0-to-3.1.0", "3.0.0", "3.1.0", "fully_reversible"),
	}
}

func TestBuildGraph(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	defs := makeTestMigrationDefinitions()
	graph, err := resolver.BuildGraph(defs)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	if len(graph.Nodes) != 6 {
		t.Errorf("expected 6 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(graph.Edges))
	}
	if graph.ExtensionID != "com.test/migration-test" {
		t.Errorf("expected extension id com.test/migration-test, got %s", graph.ExtensionID)
	}
	nodeMap := make(map[MigrationNodeID]bool, len(graph.Nodes))
	for _, n := range graph.Nodes {
		nodeMap[n.NodeID] = true
	}
	for _, d := range defs {
		if !nodeMap[MigrationNodeID(d.MigrationID)] {
			t.Errorf("expected node %s in graph", d.MigrationID)
		}
	}
}

func TestDetectCycle(t *testing.T) {
	t.Run("NoCycle", func(t *testing.T) {
		resolver := NewMigrationGraphResolver()
		defs := makeTestMigrationDefinitions()
		graph, err := resolver.BuildGraph(defs)
		if err != nil {
			t.Fatalf("BuildGraph failed: %v", err)
		}
		if err := resolver.DetectCycle(graph); err != nil {
			t.Errorf("expected no cycle, got error: %v", err)
		}
	})
	t.Run("WithCycle", func(t *testing.T) {
		defs := makeTestMigrationDefinitions()
		cycleMig := makeTestMigration("mig-3.1.0-to-1.0.0", "3.1.0", "1.0.0", "fully_reversible")
		cycleMig.Direction = DirectionReverse
		selfRef := cycleMig.MigrationID
		cycleMig.ForwardMigrationID = &selfRef
		defs = append(defs, cycleMig)
		resolver := NewMigrationGraphResolver()
		_, err := resolver.BuildGraph(defs)
		if err == nil {
			t.Fatalf("expected cycle detection error, got nil")
		}
	})
}

func TestTopologicalSort(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	defs := makeTestMigrationDefinitions()
	graph, err := resolver.BuildGraph(defs)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	result, err := resolver.TopologicalSort(graph)
	if err != nil {
		t.Fatalf("TopologicalSort failed: %v", err)
	}
	if len(result) != 6 {
		t.Fatalf("expected 6 nodes in topological order, got %d", len(result))
	}
	expected := []MigrationNodeID{
		"mig-1.0.0-to-1.1.0",
		"mig-1.1.0-to-1.2.0",
		"mig-1.2.0-to-2.0.0",
		"mig-2.0.0-to-2.1.0",
		"mig-2.1.0-to-3.0.0",
		"mig-3.0.0-to-3.1.0",
	}
	for i, id := range expected {
		if result[i] != id {
			t.Errorf("at index %d: expected %s, got %s", i, id, result[i])
		}
	}
}

func TestResolvePath_DirectMigration(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	defs := makeTestMigrationDefinitions()
	graph, err := resolver.BuildGraph(defs)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	path, err := resolver.ResolvePath(graph, "1.0.0", "1.1.0")
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}
	if !path.IsDirect {
		t.Errorf("expected direct path")
	}
	if len(path.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(path.Steps))
	}
	if path.Steps[0].MigrationID != "mig-1.0.0-to-1.1.0" {
		t.Errorf("expected migration mig-1.0.0-to-1.1.0, got %s", path.Steps[0].MigrationID)
	}
	if path.FromVersion != "1.0.0" {
		t.Errorf("expected from_version 1.0.0, got %s", path.FromVersion)
	}
	if path.ToVersion != "1.1.0" {
		t.Errorf("expected to_version 1.1.0, got %s", path.ToVersion)
	}
	if path.Steps[0].StepID != 1 {
		t.Errorf("expected step_id 1, got %d", path.Steps[0].StepID)
	}
}

func TestResolvePath_ChainedMigration(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	defs := makeTestMigrationDefinitions()
	graph, err := resolver.BuildGraph(defs)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	path, err := resolver.ResolvePath(graph, "1.0.0", "3.1.0")
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}
	if path.IsDirect {
		t.Errorf("expected chained (non-direct) path")
	}
	if len(path.Steps) != 6 {
		t.Fatalf("expected 6 steps, got %d", len(path.Steps))
	}
	if path.FromVersion != "1.0.0" {
		t.Errorf("expected from_version 1.0.0, got %s", path.FromVersion)
	}
	if path.ToVersion != "3.1.0" {
		t.Errorf("expected to_version 3.1.0, got %s", path.ToVersion)
	}
	if path.Steps[0].FromVersion != "1.0.0" {
		t.Errorf("expected first step from 1.0.0, got %s", path.Steps[0].FromVersion)
	}
	if path.Steps[5].ToVersion != "3.1.0" {
		t.Errorf("expected last step to 3.1.0, got %s", path.Steps[5].ToVersion)
	}
	for i, step := range path.Steps {
		if step.StepID != i+1 {
			t.Errorf("step %d: expected step_id %d, got %d", i, i+1, step.StepID)
		}
	}
}

func TestResolvePath_NoPath(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	defs := makeTestMigrationDefinitions()
	graph, err := resolver.BuildGraph(defs)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	_, err = resolver.ResolvePath(graph, "1.0.0", "3.2.0")
	if err == nil {
		t.Fatalf("expected error for no path from 1.0.0 to 3.2.0, got nil")
	}
}

func TestPlanMigration_LowRisk(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	planner := NewMigrationPlanner(resolver)
	input := MigrationPlanInput{
		ExtensionID:         "com.test/migration-test",
		FromVersion:         "1.0.0",
		ToVersion:           "1.1.0",
		AvailableMigrations: makeTestMigrationDefinitions(),
	}
	output, err := planner.PlanMigration(input)
	if err != nil {
		t.Fatalf("PlanMigration failed: %v", err)
	}
	if output.EstimatedRisk != "none" {
		t.Errorf("expected risk none, got %s", output.EstimatedRisk)
	}
	if !output.CanAutoRollback {
		t.Errorf("expected CanAutoRollback true")
	}
	if output.RequiresUserConfirm {
		t.Errorf("expected RequiresUserConfirm false")
	}
	if output.HasIrreversible {
		t.Errorf("expected HasIrreversible false")
	}
	if output.Reversibility != ReversibilityFullyReversible {
		t.Errorf("expected reversibility fully_reversible, got %s", output.Reversibility)
	}
	if len(output.Path.Steps) != 1 {
		t.Errorf("expected 1 path step, got %d", len(output.Path.Steps))
	}
}

func TestPlanMigration_HighRisk(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	planner := NewMigrationPlanner(resolver)
	input := MigrationPlanInput{
		ExtensionID:         "com.test/migration-test",
		FromVersion:         "1.0.0",
		ToVersion:           "3.1.0",
		AvailableMigrations: makeTestMigrationDefinitions(),
	}
	output, err := planner.PlanMigration(input)
	if err != nil {
		t.Fatalf("PlanMigration failed: %v", err)
	}
	if output.EstimatedRisk != "medium" {
		t.Errorf("expected risk medium, got %s", output.EstimatedRisk)
	}
	if output.CanAutoRollback {
		t.Errorf("expected CanAutoRollback false")
	}
	if output.Reversibility != ReversibilityReverseScriptRequired {
		t.Errorf("expected reversibility reverse_script_required, got %s", output.Reversibility)
	}
	if len(output.Path.Steps) != 6 {
		t.Errorf("expected 6 path steps, got %d", len(output.Path.Steps))
	}
}

func TestPlanMigration_RequiresConfirm(t *testing.T) {
	resolver := NewMigrationGraphResolver()
	planner := NewMigrationPlanner(resolver)
	defs := []MigrationDefinition{
		makeTestMigration("mig-1.0.0-to-1.1.0", "1.0.0", "1.1.0", "irreversible"),
	}
	input := MigrationPlanInput{
		ExtensionID:         "com.test/migration-test",
		FromVersion:         "1.0.0",
		ToVersion:           "1.1.0",
		AvailableMigrations: defs,
	}
	output, err := planner.PlanMigration(input)
	if err != nil {
		t.Fatalf("PlanMigration failed: %v", err)
	}
	if !output.RequiresUserConfirm {
		t.Errorf("expected RequiresUserConfirm true")
	}
	if !output.HasIrreversible {
		t.Errorf("expected HasIrreversible true")
	}
	if output.EstimatedRisk != "critical" {
		t.Errorf("expected risk critical, got %s", output.EstimatedRisk)
	}
	if output.CanAutoRollback {
		t.Errorf("expected CanAutoRollback false")
	}
	if output.Reversibility != ReversibilityIrreversible {
		t.Errorf("expected reversibility irreversible, got %s", output.Reversibility)
	}
}

func TestValidateMigrationDefinition_Valid(t *testing.T) {
	validator := NewPreconditionValidator()
	def := makeTestMigration("mig-1.0.0-to-1.1.0", "1.0.0", "1.1.0", "fully_reversible")
	result, err := validator.ValidateMigrationDefinition(&def)
	if err != nil {
		t.Fatalf("ValidateMigrationDefinition failed: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected validation passed, got errors: %v", result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestValidateMigrationDefinition_ForbiddenDomain(t *testing.T) {
	validator := NewPreconditionValidator()
	def := makeTestMigration("mig-1.0.0-to-1.1.0", "1.0.0", "1.1.0", "fully_reversible")
	def.DataDomains = []DataDomain{{Domain: "host_database", Storage: "sqlite", Namespace: "test_data"}}
	result, err := validator.ValidateMigrationDefinition(&def)
	if err != nil {
		t.Fatalf("ValidateMigrationDefinition failed: %v", err)
	}
	if result.Passed {
		t.Errorf("expected validation to fail due to forbidden domain")
	}
	if len(result.Errors) == 0 {
		t.Errorf("expected errors for forbidden domain")
	}
}

func TestValidateMigrationDefinition_InvalidResourceLimits(t *testing.T) {
	validator := NewPreconditionValidator()
	def := makeTestMigration("mig-1.0.0-to-1.1.0", "1.0.0", "1.1.0", "fully_reversible")
	def.ResourceLimits = TaskResourceLimits{MaxMemoryMB: 0, MaxCPUPercent: 50, MaxDiskMB: 512, MaxDurationSecs: 300}
	result, err := validator.ValidateMigrationDefinition(&def)
	if err != nil {
		t.Fatalf("ValidateMigrationDefinition failed: %v", err)
	}
	if result.Passed {
		t.Errorf("expected validation to fail due to invalid resource limits")
	}
	if len(result.Errors) == 0 {
		t.Errorf("expected errors for invalid resource limits")
	}
}

func TestCheckReversibility(t *testing.T) {
	validator := NewPreconditionValidator()
	cases := []struct {
		name          string
		reversibility Reversibility
		expected      bool
	}{
		{"fully_reversible", ReversibilityFullyReversible, true},
		{"snapshot_reversible", ReversibilitySnapshotReversible, true},
		{"reverse_script_required", ReversibilityReverseScriptRequired, false},
		{"irreversible", ReversibilityIrreversible, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validator.CheckReversibility(tc.reversibility)
			if got != tc.expected {
				t.Errorf("CheckReversibility(%s) = %v, want %v", tc.reversibility, got, tc.expected)
			}
		})
	}
}

func TestCheckIdempotency(t *testing.T) {
	validator := NewPreconditionValidator()
	cases := []struct {
		name        string
		idempotency Idempotency
		expected    bool
	}{
		{"idempotent", IdempotencyIdempotent, true},
		{"checkpoint_idempotent", IdempotencyCheckpointIdempotent, true},
		{"non_idempotent", IdempotencyNonIdempotent, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validator.CheckIdempotency(tc.idempotency)
			if got != tc.expected {
				t.Errorf("CheckIdempotency(%s) = %v, want %v", tc.idempotency, got, tc.expected)
			}
		})
	}
}
