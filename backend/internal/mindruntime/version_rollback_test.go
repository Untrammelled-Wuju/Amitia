package mindruntime

import (
	"strings"
	"testing"
	"time"
)

func TestNewVersionHistory(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	if h.CharacterID != "char-001" {
		t.Errorf("expected char-001, got %s", h.CharacterID)
	}
	if h.Target != SupervisorTargetSummary {
		t.Errorf("expected target summary, got %s", h.Target)
	}
	if h.CurrentVersion != 0 {
		t.Errorf("expected version 0, got %d", h.CurrentVersion)
	}
	if len(h.Records) != 0 {
		t.Errorf("expected 0 records, got %d", len(h.Records))
	}
	if h.EngineVersion != RollbackEngineVersion {
		t.Errorf("expected engine version %s, got %s", RollbackEngineVersion, h.EngineVersion)
	}
}

func TestPushIncrementsVersion(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetReflection)
	r1 := VersionRecord{SnapshotID: "snap-001", DecisionID: "dec-001", CreatedAt: time.Now()}
	h = h.Push(r1)
	if h.CurrentVersion != 1 {
		t.Errorf("expected version 1 after first push, got %d", h.CurrentVersion)
	}
	if len(h.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(h.Records))
	}
	if h.Records[0].Version != 1 {
		t.Errorf("expected record version 1, got %d", h.Records[0].Version)
	}
	if h.Records[0].CharacterID != "char-001" {
		t.Errorf("expected char-001, got %s", h.Records[0].CharacterID)
	}
	if h.Records[0].Target != SupervisorTargetReflection {
		t.Errorf("expected target reflection, got %s", h.Records[0].Target)
	}

	r2 := VersionRecord{SnapshotID: "snap-002", DecisionID: "dec-002", CreatedAt: time.Now()}
	h = h.Push(r2)
	if h.CurrentVersion != 2 {
		t.Errorf("expected version 2 after second push, got %d", h.CurrentVersion)
	}
	if len(h.Records) != 2 {
		t.Errorf("expected 2 records, got %d", len(h.Records))
	}
	if h.Records[1].Version != 2 {
		t.Errorf("expected record version 2, got %d", h.Records[1].Version)
	}
}

func TestLatestOnEmptyHistory(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	_, ok := h.Latest()
	if ok {
		t.Error("expected Latest to return false on empty history")
	}
}

func TestLatestOnNonEmpty(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetGrowth)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	h = h.Push(VersionRecord{SnapshotID: "snap-002"})
	latest, ok := h.Latest()
	if !ok {
		t.Fatal("expected Latest to return true")
	}
	if latest.Version != 2 {
		t.Errorf("expected latest version 2, got %d", latest.Version)
	}
	if latest.SnapshotID != "snap-002" {
		t.Errorf("expected snap-002, got %s", latest.SnapshotID)
	}
}

func TestAtVersion(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetPersonality)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	h = h.Push(VersionRecord{SnapshotID: "snap-002"})
	h = h.Push(VersionRecord{SnapshotID: "snap-003"})

	r, ok := h.AtVersion(2)
	if !ok {
		t.Fatal("expected to find version 2")
	}
	if r.SnapshotID != "snap-002" {
		t.Errorf("expected snap-002, got %s", r.SnapshotID)
	}

	_, ok = h.AtVersion(99)
	if ok {
		t.Error("expected not to find version 99")
	}
}

func TestActiveVersions(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	now := time.Now()
	h = h.Push(VersionRecord{SnapshotID: "snap-001", CreatedAt: now})
	h = h.Push(VersionRecord{SnapshotID: "snap-002", CreatedAt: now})
	h = h.Push(VersionRecord{SnapshotID: "snap-003", CreatedAt: now})

	active := h.ActiveVersions(now)
	if len(active) != 3 {
		t.Errorf("expected 3 active versions, got %d", len(active))
	}

	// mark version 2 as rolled back
	rolled := h.Records
	rolled[1].RolledBackAt = now
	h.Records = rolled

	active = h.ActiveVersions(now)
	if len(active) != 2 {
		t.Errorf("expected 2 active versions after rollback, got %d", len(active))
	}
}

func TestRolledBackVersions(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	now := time.Now()
	h = h.Push(VersionRecord{SnapshotID: "snap-001", CreatedAt: now})
	h = h.Push(VersionRecord{SnapshotID: "snap-002", CreatedAt: now})

	rolled := h.RolledBackVersions()
	if len(rolled) != 0 {
		t.Errorf("expected 0 rolled back, got %d", len(rolled))
	}

	marked := h.Records
	marked[0].RolledBackAt = now
	h.Records = marked

	rolled = h.RolledBackVersions()
	if len(rolled) != 1 {
		t.Errorf("expected 1 rolled back, got %d", len(rolled))
	}
}

func TestDefaultRollbackEngineConfig(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled true")
	}
	if cfg.MaxHistoryPerTarget != 50 {
		t.Errorf("expected MaxHistoryPerTarget 50, got %d", cfg.MaxHistoryPerTarget)
	}
	if !cfg.RequireCompensation {
		t.Error("expected RequireCompensation true")
	}
	if !cfg.AutoCascade {
		t.Error("expected AutoCascade true")
	}
	if cfg.DefaultCascadeAction != CascadeActionInvalidate {
		t.Errorf("expected CascadeActionInvalidate, got %s", cfg.DefaultCascadeAction)
	}
}

func TestNewVersionRollbackEngine(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	engine := NewVersionRollbackEngine(cfg)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.Config.Enabled != true {
		t.Error("expected engine config to be enabled")
	}
}

func TestPlanRollback_EmptyHistory(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	plan := engine.PlanRollback(h, 0, "test reason", nil)
	if plan.ID != "" {
		t.Errorf("expected empty plan for empty history, got ID %s", plan.ID)
	}
}

func TestPlanRollback_InvalidTarget(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	// target version >= from version
	plan := engine.PlanRollback(h, 1, "test reason", nil)
	if plan.ID != "" {
		t.Errorf("expected empty plan when target >= current, got ID %s", plan.ID)
	}
}

func TestPlanRollback_Valid(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	h = h.Push(VersionRecord{SnapshotID: "snap-002"})
	h = h.Push(VersionRecord{SnapshotID: "snap-003"})

	plan := engine.PlanRollback(h, 1, "incorrect reflection data", nil)
	if plan.ID == "" {
		t.Fatal("expected non-empty plan")
	}
	if plan.CharacterID != "char-001" {
		t.Errorf("expected char-001, got %s", plan.CharacterID)
	}
	if plan.Target != SupervisorTargetSummary {
		t.Errorf("expected target summary, got %s", plan.Target)
	}
	if plan.FromVersion != 3 {
		t.Errorf("expected from version 3, got %d", plan.FromVersion)
	}
	if plan.ToVersion != 1 {
		t.Errorf("expected to version 1, got %d", plan.ToVersion)
	}
	if plan.Reason != "incorrect reflection data" {
		t.Errorf("unexpected reason: %s", plan.Reason)
	}
}

func TestPlanRollback_GeneratesCompensation(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	cfg.RequireCompensation = true
	engine := NewVersionRollbackEngine(cfg)
	h := NewVersionHistory("char-001", SupervisorTargetGrowth)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	h = h.Push(VersionRecord{SnapshotID: "snap-002"})

	plan := engine.PlanRollback(h, 1, "growth correction", nil)
	if len(plan.Compensations) != 1 {
		t.Fatalf("expected 1 compensation event, got %d", len(plan.Compensations))
	}
	comp := plan.Compensations[0]
	if comp.CharacterID != "char-001" {
		t.Errorf("expected char-001, got %s", comp.CharacterID)
	}
	if comp.Target != CompensationTargetGrowth {
		t.Errorf("expected target growth, got %s", comp.Target)
	}
	if comp.SourceVersion != 2 {
		t.Errorf("expected source version 2, got %d", comp.SourceVersion)
	}
	if comp.TargetVersion != 1 {
		t.Errorf("expected target version 1, got %d", comp.TargetVersion)
	}
}

func TestPlanRollback_NoCompensationWhenDisabled(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	cfg.RequireCompensation = false
	engine := NewVersionRollbackEngine(cfg)
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	h = h.Push(VersionRecord{SnapshotID: "snap-002"})

	plan := engine.PlanRollback(h, 1, "test", nil)
	if len(plan.Compensations) != 0 {
		t.Errorf("expected 0 compensations when disabled, got %d", len(plan.Compensations))
	}
}

func TestPlanRollback_GeneratesCascades(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	cfg.AutoCascade = true
	engine := NewVersionRollbackEngine(cfg)
	h := NewVersionHistory("char-001", SupervisorTargetReflection)
	h = h.Push(VersionRecord{SnapshotID: "snap-001", ID: "version-parent"})
	h = h.Push(VersionRecord{SnapshotID: "snap-002", ID: "version-child"})

	derived := []VersionRecord{
		{ID: "derived-belief-001", CharacterID: "char-001", Target: SupervisorTargetReflection},
		{ID: "derived-summary-001", CharacterID: "char-001", Target: SupervisorTargetReflection},
	}

	plan := engine.PlanRollback(h, 1, "test cascade", derived)
	if len(plan.Cascades) != 2 {
		t.Fatalf("expected 2 cascade instructions, got %d", len(plan.Cascades))
	}
	if plan.Cascades[0].Action != CascadeActionInvalidate {
		t.Errorf("expected INVALIDATE action, got %s", plan.Cascades[0].Action)
	}
	if plan.Cascades[0].TargetID != "derived-belief-001" {
		t.Errorf("expected derived-belief-001, got %s", plan.Cascades[0].TargetID)
	}
}

func TestPlanRollback_NoCascadeWhenDisabled(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	cfg.AutoCascade = false
	engine := NewVersionRollbackEngine(cfg)
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	h = h.Push(VersionRecord{SnapshotID: "snap-002"})

	plan := engine.PlanRollback(h, 1, "test", []VersionRecord{{ID: "derived-001"}})
	if len(plan.Cascades) != 0 {
		t.Errorf("expected 0 cascades when disabled, got %d", len(plan.Cascades))
	}
}

func TestExecuteRollback_MarksRecords(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-v1"})
	h = h.Push(VersionRecord{SnapshotID: "snap-v2"})
	h = h.Push(VersionRecord{SnapshotID: "snap-v3"})

	updated, plan, comps := engine.ExecuteRollback(h, 1, "manual correction", "admin-user", nil)
	if plan.ID == "" {
		t.Fatal("expected non-empty plan")
	}
	if len(comps) != 1 {
		t.Errorf("expected 1 compensation, got %d", len(comps))
	}

	// Check versions 2 and 3 are rolled back
	for _, r := range updated.Records {
		if r.Version == 1 {
			if !r.RolledBackAt.IsZero() {
				t.Error("expected version 1 to remain active")
			}
		}
		if r.Version == 2 || r.Version == 3 {
			if r.RolledBackAt.IsZero() {
				t.Errorf("expected version %d to be rolled back", r.Version)
			}
			if r.RolledBackBy != "admin-user" {
				t.Errorf("expected rollbackBy admin-user, got %s", r.RolledBackBy)
			}
		}
	}
}

func TestExecuteRollback_PreservesHistory(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-v1"})
	h = h.Push(VersionRecord{SnapshotID: "snap-v2"})

	updated, _, _ := engine.ExecuteRollback(h, 1, "test", "system", nil)
	if len(updated.Records) != 2 {
		t.Errorf("expected 2 records after rollback, got %d", len(updated.Records))
	}
	// CurrentVersion should not be changed by ExecuteRollback
	if updated.CurrentVersion != 2 {
		t.Errorf("expected CurrentVersion 2, got %d", updated.CurrentVersion)
	}
}

func TestTrimHistory_UnderLimit(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	cfg.MaxHistoryPerTarget = 50
	engine := NewVersionRollbackEngine(cfg)
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	for i := 0; i < 10; i++ {
		h = h.Push(VersionRecord{SnapshotID: "snap"})
	}
	trimmed := engine.TrimHistory(h)
	if len(trimmed.Records) != 10 {
		t.Errorf("expected 10 records (under limit), got %d", len(trimmed.Records))
	}
}

func TestTrimHistory_OverLimit(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	cfg.MaxHistoryPerTarget = 5
	engine := NewVersionRollbackEngine(cfg)
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	for i := 0; i < 10; i++ {
		h = h.Push(VersionRecord{SnapshotID: "snap"})
	}
	trimmed := engine.TrimHistory(h)
	if len(trimmed.Records) > 5 {
		t.Errorf("expected at most 5 records after trim, got %d", len(trimmed.Records))
	}
	if trimmed.CurrentVersion != 10 {
		t.Errorf("expected CurrentVersion 10, got %d", trimmed.CurrentVersion)
	}
}

func TestTrimHistory_PreservesActive(t *testing.T) {
	cfg := DefaultRollbackEngineConfig()
	cfg.MaxHistoryPerTarget = 3
	engine := NewVersionRollbackEngine(cfg)
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	now := time.Now()
	for i := 0; i < 6; i++ {
		h = h.Push(VersionRecord{SnapshotID: "snap", CreatedAt: now})
	}

	// mark first 3 as rolled back
	records := make([]VersionRecord, len(h.Records))
	copy(records, h.Records)
	for i := 0; i < 3; i++ {
		records[i].RolledBackAt = now
	}
	h.Records = records

	trimmed := engine.TrimHistory(h)
	if len(trimmed.Records) == 0 {
		t.Fatal("expected at least some records after trim")
	}
}

func TestBuildCompensationChain(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	source := CompensationEvent{
		CharacterID:   "char-001",
		Target:        CompensationTargetReflect,
		SourceVersion: 3,
		TargetVersion: 1,
		Reason:        "test",
		CreatedAt:     time.Now(),
	}

	derived := []CompensationTarget{
		CompensationTargetBelief,
		CompensationTargetSummary,
	}
	events := engine.BuildCompensationChain(source, derived)
	if len(events) != 2 {
		t.Fatalf("expected 2 compensation events, got %d", len(events))
	}
	for _, ev := range events {
		if ev.CharacterID != "char-001" {
			t.Errorf("expected char-001, got %s", ev.CharacterID)
		}
		if ev.SourceVersion != 3 {
			t.Errorf("expected source version 3, got %d", ev.SourceVersion)
		}
		if ev.TargetVersion != 1 {
			t.Errorf("expected target version 1, got %d", ev.TargetVersion)
		}
		if len(ev.DerivedEvents) != 1 || ev.DerivedEvents[0] != source.ID {
			t.Error("expected derived event reference to source")
		}
	}
}

func TestMarkCompensationProcessed(t *testing.T) {
	ev := CompensationEvent{
		CharacterID: "char-001",
		Target:      CompensationTargetSummary,
		CreatedAt:   time.Now(),
	}
	processed := MarkCompensationProcessed(ev)
	if !processed.Processed {
		t.Error("expected Processed to be true")
	}
	if processed.ProcessedAt.IsZero() {
		t.Error("expected non-zero ProcessedAt")
	}
	// original should not be modified
	if ev.Processed {
		t.Error("expected original event to remain unmodified")
	}
}

func TestMergeCompensationEvents_Deduplicates(t *testing.T) {
	now := time.Now()
	events := []CompensationEvent{
		{ID: "comp-001", CharacterID: "char-001", Target: CompensationTargetBelief, SourceVersion: 5, CreatedAt: now},
		{ID: "comp-002", CharacterID: "char-001", Target: CompensationTargetBelief, SourceVersion: 5, CreatedAt: now.Add(time.Second)},
		{ID: "comp-003", CharacterID: "char-001", Target: CompensationTargetSummary, SourceVersion: 5, CreatedAt: now},
	}
	merged := MergeCompensationEvents(events)
	if len(merged) != 2 {
		t.Errorf("expected 2 merged events, got %d", len(merged))
	}
}

func TestMergeCompensationEvents_PrefersProcessed(t *testing.T) {
	now := time.Now()
	unprocessed := CompensationEvent{
		ID: "comp-001", CharacterID: "char-001", Target: CompensationTargetBelief,
		SourceVersion: 5, CreatedAt: now, Processed: false,
	}
	processed := CompensationEvent{
		ID: "comp-002", CharacterID: "char-001", Target: CompensationTargetBelief,
		SourceVersion: 5, CreatedAt: now.Add(time.Second), Processed: true, ProcessedAt: now,
	}
	merged := MergeCompensationEvents([]CompensationEvent{unprocessed, processed})
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged event, got %d", len(merged))
	}
	if merged[0].Processed != true {
		t.Error("expected merged event to retain processed=true")
	}
}

func TestSummary(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	now := time.Now()

	h1 := NewVersionHistory("char-001", SupervisorTargetSummary)
	h1 = h1.Push(VersionRecord{SnapshotID: "snap-001", CreatedAt: now})
	h1 = h1.Push(VersionRecord{SnapshotID: "snap-002", CreatedAt: now})
	h1 = h1.Push(VersionRecord{SnapshotID: "snap-003", CreatedAt: now})

	h2 := NewVersionHistory("char-002", SupervisorTargetReflection)
	h2 = h2.Push(VersionRecord{SnapshotID: "snap-004", CreatedAt: now})

	compensations := []CompensationEvent{
		{CharacterID: "char-001", Target: CompensationTargetSummary, Processed: false},
		{CharacterID: "char-002", Target: CompensationTargetReflect, Processed: true},
	}

	summary := engine.Summary([]VersionHistory{h1, h2}, compensations)
	if summary.TotalHistories != 2 {
		t.Errorf("expected 2 histories, got %d", summary.TotalHistories)
	}
	if summary.TotalRecords != 4 {
		t.Errorf("expected 4 total records, got %d", summary.TotalRecords)
	}
	if summary.ActiveRecords != 4 {
		t.Errorf("expected 4 active records, got %d", summary.ActiveRecords)
	}
	if summary.RolledBackRecords != 0 {
		t.Errorf("expected 0 rolled back, got %d", summary.RolledBackRecords)
	}
	if summary.PendingCompensations != 1 {
		t.Errorf("expected 1 pending, got %d", summary.PendingCompensations)
	}
	if summary.ProcessedCompensations != 1 {
		t.Errorf("expected 1 processed, got %d", summary.ProcessedCompensations)
	}
}

func TestSummary_WithRolledBack(t *testing.T) {
	engine := NewVersionRollbackEngine(DefaultRollbackEngineConfig())
	now := time.Now()

	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-001", CreatedAt: now})
	h = h.Push(VersionRecord{SnapshotID: "snap-002", CreatedAt: now})

	records := h.Records
	records[0].RolledBackAt = now
	h.Records = records

	summary := engine.Summary([]VersionHistory{h}, nil)
	if summary.ActiveRecords != 1 {
		t.Errorf("expected 1 active, got %d", summary.ActiveRecords)
	}
	if summary.RolledBackRecords != 1 {
		t.Errorf("expected 1 rolled back, got %d", summary.RolledBackRecords)
	}
}

func TestNewRollbackVersion(t *testing.T) {
	v := NewRollbackVersion()
	if !strings.HasPrefix(string(v), "rollback-engine-v") {
		t.Errorf("expected prefix rollback-engine-v, got %s", v)
	}
}

func TestPushAutoGeneratesID(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	if h.Records[0].ID == "" {
		t.Error("expected auto-generated ID")
	}
	if len(h.Records[0].ID) < 10 {
		t.Errorf("expected reasonable ID length, got %s", h.Records[0].ID)
	}
}

func TestPushPreservesID(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{ID: "custom-id", SnapshotID: "snap-001"})
	if h.Records[0].ID != "custom-id" {
		t.Errorf("expected custom-id, got %s", h.Records[0].ID)
	}
}

func TestVersionsBetween(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	for i := 0; i < 5; i++ {
		h = h.Push(VersionRecord{SnapshotID: "snap"})
	}
	between := versionsBetween(h, 5, 2)
	if len(between) != 3 {
		t.Errorf("expected 3 versions between 5 and 2, got %d", len(between))
	}
	if between[0].Version != 3 {
		t.Errorf("expected first version 3, got %d", between[0].Version)
	}
	if between[1].Version != 4 {
		t.Errorf("expected second version 4, got %d", between[1].Version)
	}
	if between[2].Version != 5 {
		t.Errorf("expected third version 5, got %d", between[2].Version)
	}
}

func TestVersionsBetween_NoMatch(t *testing.T) {
	h := NewVersionHistory("char-001", SupervisorTargetSummary)
	h = h.Push(VersionRecord{SnapshotID: "snap-001"})
	between := versionsBetween(h, 1, 1)
	if len(between) != 0 {
		t.Errorf("expected 0 versions, got %d", len(between))
	}
}

func TestMapSupervisorTargetToCompensation(t *testing.T) {
	cases := []struct {
		input SupervisorTarget
		want  CompensationTarget
	}{
		{SupervisorTargetPersonality, CompensationTargetProfile},
		{SupervisorTargetSummary, CompensationTargetSummary},
		{SupervisorTargetReflection, CompensationTargetReflect},
		{SupervisorTargetGrowth, CompensationTargetGrowth},
		{"unknown", CompensationTargetBelief},
	}
	for _, c := range cases {
		got := mapSupervisorTargetToCompensation(c.input)
		if got != c.want {
			t.Errorf("mapSupervisorTargetToCompensation(%s) = %s, want %s", c.input, got, c.want)
		}
	}
}




