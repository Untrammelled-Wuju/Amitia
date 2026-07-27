package desktop_update

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func validMetadata() ExtensionUpdateMetadata {
	return ExtensionUpdateMetadata{
		ExtensionID:     "dev.amitia.example/my-ext",
		Version:         "1.2.0",
		ManifestVersion: 1,
		PackageURL:      "https://registry.amitia.dev/pkg/my-ext-1.2.0.zip",
		PackageSHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PackageSize:     1024,
		PublisherID:     "amitia",
		ReleaseChannel:  "stable",
	}
}

func TestStateMachine_ValidTransitions(t *testing.T) {
	sm := NewStateMachine()
	cases := []struct {
		from UpdateState
		to   UpdateState
	}{
		{StateCreated, StateChecking},
		{StateCreated, StateCancelled},
		{StateCreated, StateFailed},
		{StateChecking, StateAvailable},
		{StateChecking, StateCompleted},
		{StateChecking, StateFailed},
		{StateAvailable, StateDownloading},
		{StateAvailable, StateCancelled},
		{StateDownloading, StateDownloaded},
		{StateDownloading, StateFailed},
		{StateDownloading, StateRecoveryRequired},
		{StateDownloaded, StateVerifying},
		{StateVerifying, StateStaging},
		{StateStaging, StatePreflight},
		{StateStaging, StateRecoveryRequired},
		{StatePreflight, StateDraining},
		{StatePreflight, StateWaitingConfirmation},
		{StateWaitingConfirmation, StateDraining},
		{StateDraining, StateMigrating},
		{StateDraining, StateRecoveryRequired},
		{StateMigrating, StateActivating},
		{StateMigrating, StateRollbackPending},
		{StateActivating, StateVerifyingHealth},
		{StateActivating, StateRollbackPending},
		{StateVerifyingHealth, StateCommitting},
		{StateVerifyingHealth, StateRollbackPending},
		{StateCommitting, StateCompleted},
		{StateCommitting, StateRollbackPending},
		{StateFailed, StateCreated},
		{StateRecoveryRequired, StateCreated},
		{StateRecoveryRequired, StateManualIntervention},
		{StateManualIntervention, StateCreated},
		{StateRollbackPending, StateRollingBack},
		{StateRollbackPending, StateManualIntervention},
		{StateRollingBack, StateRolledBack},
		{StateRollingBack, StateManualIntervention},
	}
	for _, c := range cases {
		if !sm.CanTransition(c.from, c.to) {
			t.Errorf("expected transition %s -> %s to be allowed", c.from, c.to)
		}
		if err := sm.Transition(c.from, c.to); err != nil {
			t.Errorf("expected transition %s -> %s to succeed, got error: %v", c.from, c.to, err)
		}
	}
}

func TestStateMachine_InvalidTransitions(t *testing.T) {
	sm := NewStateMachine()
	cases := []struct {
		from UpdateState
		to   UpdateState
	}{
		{StateCreated, StateDownloading},
		{StateCreated, StateCompleted},
		{StateCreated, StateMigrating},
		{StateChecking, StateDownloading},
		{StateAvailable, StateVerifying},
		{StateAvailable, StateStaging},
		{StateDownloading, StateStaging},
		{StateDownloaded, StatePreflight},
		{StateVerifying, StatePreflight},
		{StateStaging, StateDraining},
		{StatePreflight, StateActivating},
		{StateWaitingConfirmation, StateMigrating},
		{StateDraining, StateActivating},
		{StateMigrating, StateVerifyingHealth},
		{StateActivating, StateCommitting},
		{StateVerifyingHealth, StateCompleted},
		{StateCommitting, StateDraining},
		{StateCompleted, StateCreated},
		{StateCompleted, StateFailed},
		{StateRolledBack, StateCreated},
		{StateCancelled, StateCreated},
		{StateManualIntervention, StateFailed},
		{StateFailed, StateCompleted},
		{StateFailed, StateRollbackPending},
	}
	for _, c := range cases {
		if sm.CanTransition(c.from, c.to) {
			t.Errorf("expected transition %s -> %s to be disallowed", c.from, c.to)
		}
		if err := sm.Transition(c.from, c.to); err == nil {
			t.Errorf("expected transition %s -> %s to return error", c.from, c.to)
		}
	}
}

func TestStateMachine_AllowedTransitions(t *testing.T) {
	sm := NewStateMachine()
	created := sm.AllowedTransitions(StateCreated)
	if len(created) != 3 {
		t.Fatalf("expected 3 allowed transitions from StateCreated, got %d", len(created))
	}
	seen := map[UpdateState]bool{}
	for _, s := range created {
		seen[s] = true
	}
	for _, expected := range []UpdateState{StateChecking, StateCancelled, StateFailed} {
		if !seen[expected] {
			t.Errorf("expected %s in allowed transitions from StateCreated", expected)
		}
	}
	if allowed := sm.AllowedTransitions(StateCompleted); len(allowed) != 0 {
		t.Errorf("expected no allowed transitions from terminal StateCompleted, got %d", len(allowed))
	}
}

func TestStateMachine_IsTerminal(t *testing.T) {
	sm := NewStateMachine()
	terminal := []UpdateState{StateCompleted, StateRolledBack, StateCancelled, StateManualIntervention}
	for _, s := range terminal {
		if !sm.IsTerminal(s) {
			t.Errorf("expected %s to be terminal", s)
		}
	}
	nonTerminal := []UpdateState{
		StateCreated, StateChecking, StateAvailable, StateDownloading,
		StateDownloaded, StateVerifying, StateStaging, StatePreflight,
		StateWaitingConfirmation, StateDraining, StateMigrating, StateActivating,
		StateVerifyingHealth, StateCommitting, StateRollbackPending, StateRollingBack,
		StateFailed, StateRecoveryRequired,
	}
	for _, s := range nonTerminal {
		if sm.IsTerminal(s) {
			t.Errorf("expected %s to NOT be terminal", s)
		}
	}
}

func TestStateMachine_IsRecoverable(t *testing.T) {
	sm := NewStateMachine()
	recoverable := []UpdateState{
		StateDownloading, StateVerifying, StateStaging, StateDraining,
		StateMigrating, StateActivating, StateVerifyingHealth, StateCommitting,
		StateRollingBack,
	}
	for _, s := range recoverable {
		if !sm.IsRecoverable(s) {
			t.Errorf("expected %s to be recoverable", s)
		}
	}
	notRecoverable := []UpdateState{
		StateCreated, StateChecking, StateAvailable, StateDownloaded,
		StatePreflight, StateWaitingConfirmation, StateRollbackPending,
		StateCompleted, StateRolledBack, StateCancelled, StateFailed,
		StateRecoveryRequired, StateManualIntervention,
	}
	for _, s := range notRecoverable {
		if sm.IsRecoverable(s) {
			t.Errorf("expected %s to NOT be recoverable", s)
		}
	}
}

func TestUpdateJournal_RecordAndGetEntries(t *testing.T) {
	j := NewUpdateJournal()
	entries := j.GetEntries("op-1")
	if len(entries) != 0 {
		t.Fatalf("expected empty entries for unknown op, got %d", len(entries))
	}
	now := time.Now().UTC()
	j.Record(JournalEntry{
		OperationID: "op-1",
		Step:        "create",
		Status:      JournalStatusCompleted,
		StartedAt:   now,
	})
	j.Record(JournalEntry{
		OperationID: "op-1",
		Step:        "download",
		Status:      JournalStatusInProgress,
		StartedAt:   now,
	})
	got := j.GetEntries("op-1")
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Step != "create" || got[1].Step != "download" {
		t.Errorf("unexpected entry order: %s, %s", got[0].Step, got[1].Step)
	}
	mutated := got[0]
	mutated.Step = "mutated"
	again := j.GetEntries("op-1")
	if again[0].Step != "create" {
		t.Errorf("GetEntries should return a copy, original was mutated")
	}
}

func TestUpdateJournal_RecordAutoTimestamp(t *testing.T) {
	j := NewUpdateJournal()
	j.Record(JournalEntry{
		OperationID: "op-auto",
		Step:        "step",
		Status:      JournalStatusPending,
	})
	got := j.GetEntries("op-auto")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].StartedAt.IsZero() {
		t.Errorf("expected StartedAt to be auto-filled when zero")
	}
}

func TestUpdateJournal_RecordStep(t *testing.T) {
	j := NewUpdateJournal()
	started := time.Now().UTC().Add(-time.Minute)
	j.RecordStep("op-rs", "step-a", JournalStatusInProgress, started, "")
	j.RecordStep("op-rs", "step-b", JournalStatusCompleted, time.Now().UTC(), "")
	j.RecordStep("op-rs", "step-c", JournalStatusFailed, time.Now().UTC(), "boom")
	got := j.GetEntries("op-rs")
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got[0].FinishedAt != nil {
		t.Errorf("in_progress entry should not have FinishedAt set")
	}
	if got[1].FinishedAt == nil {
		t.Errorf("completed entry should have FinishedAt set")
	}
	if got[2].Error != "boom" {
		t.Errorf("expected error 'boom', got %s", got[2].Error)
	}
	if got[2].FinishedAt == nil {
		t.Errorf("failed entry should have FinishedAt set")
	}
}

func TestUpdateJournal_RecordStep_ZeroStartedFillsNow(t *testing.T) {
	j := NewUpdateJournal()
	j.RecordStep("op-z", "step", JournalStatusPending, time.Time{}, "")
	got := j.GetEntries("op-z")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].StartedAt.IsZero() {
		t.Errorf("expected zero StartedAt to be filled with current time")
	}
}

func TestUpdateJournal_CompleteStep(t *testing.T) {
	j := NewUpdateJournal()
	j.Record(JournalEntry{
		OperationID: "op-c",
		Step:        "download",
		Status:      JournalStatusInProgress,
		StartedAt:   time.Now().UTC(),
	})
	j.CompleteStep("op-c", "download", "hash-abc")
	got := j.GetEntries("op-c")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Status != JournalStatusCompleted {
		t.Errorf("expected status completed, got %s", got[0].Status)
	}
	if got[0].OutputHash != "hash-abc" {
		t.Errorf("expected output hash hash-abc, got %s", got[0].OutputHash)
	}
	if got[0].FinishedAt == nil {
		t.Errorf("expected FinishedAt to be set after CompleteStep")
	}
}

func TestUpdateJournal_CompleteStep_NoMatchingInProgress(t *testing.T) {
	j := NewUpdateJournal()
	j.Record(JournalEntry{
		OperationID: "op-nm",
		Step:        "download",
		Status:      JournalStatusCompleted,
		StartedAt:   time.Now().UTC(),
	})
	j.CompleteStep("op-nm", "download", "hash")
	got := j.GetEntries("op-nm")
	if got[0].Status != JournalStatusCompleted {
		t.Errorf("expected status unchanged completed, got %s", got[0].Status)
	}
	if got[0].OutputHash != "" {
		t.Errorf("expected output hash unchanged, got %s", got[0].OutputHash)
	}
}

func TestUpdateJournal_FailStep(t *testing.T) {
	j := NewUpdateJournal()
	j.Record(JournalEntry{
		OperationID: "op-f",
		Step:        "verify",
		Status:      JournalStatusInProgress,
		StartedAt:   time.Now().UTC(),
	})
	j.FailStep("op-f", "verify", "hash mismatch", "rollback")
	got := j.GetEntries("op-f")
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Status != JournalStatusFailed {
		t.Errorf("expected status failed, got %s", got[0].Status)
	}
	if got[0].Error != "hash mismatch" {
		t.Errorf("expected error 'hash mismatch', got %s", got[0].Error)
	}
	if got[0].Compensation != "rollback" {
		t.Errorf("expected compensation 'rollback', got %s", got[0].Compensation)
	}
	if got[0].FinishedAt == nil {
		t.Errorf("expected FinishedAt to be set after FailStep")
	}
}

func TestUpdateJournal_GetLastEntry(t *testing.T) {
	j := NewUpdateJournal()
	if last := j.GetLastEntry("op-l"); last != nil {
		t.Errorf("expected nil last entry for unknown op")
	}
	j.Record(JournalEntry{OperationID: "op-l", Step: "first", Status: JournalStatusCompleted, StartedAt: time.Now().UTC()})
	j.Record(JournalEntry{OperationID: "op-l", Step: "second", Status: JournalStatusCompleted, StartedAt: time.Now().UTC()})
	last := j.GetLastEntry("op-l")
	if last == nil {
		t.Fatalf("expected non-nil last entry")
	}
	if last.Step != "second" {
		t.Errorf("expected last entry step 'second', got %s", last.Step)
	}
}

func TestUpdateJournal_ListPending(t *testing.T) {
	j := NewUpdateJournal()
	j.Record(JournalEntry{OperationID: "op-p1", Step: "s", Status: JournalStatusPending, StartedAt: time.Now().UTC()})
	j.Record(JournalEntry{OperationID: "op-p2", Step: "s", Status: JournalStatusInProgress, StartedAt: time.Now().UTC()})
	j.Record(JournalEntry{OperationID: "op-p3", Step: "s", Status: JournalStatusCompleted, StartedAt: time.Now().UTC()})
	j.Record(JournalEntry{OperationID: "op-p4", Step: "s", Status: JournalStatusFailed, StartedAt: time.Now().UTC()})
	pending := j.ListPending()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending entries, got %d", len(pending))
	}
	for _, e := range pending {
		if e.Status != JournalStatusPending && e.Status != JournalStatusInProgress {
			t.Errorf("expected only pending/in_progress, got %s", e.Status)
		}
	}
}

func TestUpdateJournal_ListAllAndListOperations(t *testing.T) {
	j := NewUpdateJournal()
	j.Record(JournalEntry{OperationID: "op-a", Step: "s1", Status: JournalStatusCompleted, StartedAt: time.Now().UTC()})
	j.Record(JournalEntry{OperationID: "op-b", Step: "s2", Status: JournalStatusCompleted, StartedAt: time.Now().UTC()})
	j.Record(JournalEntry{OperationID: "op-a", Step: "s3", Status: JournalStatusCompleted, StartedAt: time.Now().UTC()})
	all := j.ListAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 total entries, got %d", len(all))
	}
	ops := j.ListOperations()
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}
	if ops[0] != "op-a" || ops[1] != "op-b" {
		t.Errorf("expected sorted operations [op-a op-b], got %v", ops)
	}
}

func TestUpdateJournal_Clear(t *testing.T) {
	j := NewUpdateJournal()
	j.Record(JournalEntry{OperationID: "op-cl", Step: "s", Status: JournalStatusCompleted, StartedAt: time.Now().UTC()})
	j.Clear("op-cl")
	if entries := j.GetEntries("op-cl"); len(entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(entries))
	}
	if ops := j.ListOperations(); len(ops) != 0 {
		t.Errorf("expected 0 operations after clear, got %d", len(ops))
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name    string
		current string
		target  string
		want    UpdateType
		wantErr bool
	}{
		{"same", "1.0.0", "1.0.0", UpdateTypeSame, false},
		{"patch", "1.0.0", "1.0.1", UpdateTypePatch, false},
		{"minor", "1.0.0", "1.1.0", UpdateTypeMinor, false},
		{"major", "1.0.0", "2.0.0", UpdateTypeMajor, false},
		{"downgrade", "2.0.0", "1.0.0", UpdateTypeDowngrade, false},
		{"downgrade patch", "1.0.1", "1.0.0", UpdateTypeDowngrade, false},
		{"downgrade minor", "1.1.0", "1.0.5", UpdateTypeDowngrade, false},
		{"prerelease target higher base", "1.0.0", "2.0.0-beta", UpdateTypePrerelease, false},
		{"prerelease both same base", "1.0.0-alpha", "1.0.0-beta", UpdateTypePrerelease, false},
		{"stable over prerelease is downgrade", "1.0.0", "1.0.0-beta", UpdateTypeDowngrade, false},
		{"prerelease to patch", "1.0.0-alpha", "1.0.1", UpdateTypePatch, false},
		{"prerelease to minor", "1.0.0-alpha", "1.1.0", UpdateTypeMinor, false},
		{"invalid current", "not-a-version", "1.0.0", "", true},
		{"invalid target", "1.0.0", "not-a-version", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CompareVersions(c.current, c.target)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("CompareVersions(%q, %q) = %s, want %s", c.current, c.target, got, c.want)
			}
		})
	}
}

func TestIsDowngrade(t *testing.T) {
	cases := []struct {
		current string
		target  string
		want    bool
		wantErr bool
	}{
		{"2.0.0", "1.0.0", true, false},
		{"1.0.0", "2.0.0", false, false},
		{"1.0.0", "1.0.0", false, false},
		{"1.0.1", "1.0.0", true, false},
		{"1.0.0", "1.0.0-beta", true, false},
		{"1.0.0-beta", "1.0.0", false, false},
		{"bad", "1.0.0", false, true},
		{"1.0.0", "bad", false, true},
	}
	for _, c := range cases {
		got, err := IsDowngrade(c.current, c.target)
		if c.wantErr {
			if err == nil {
				t.Errorf("IsDowngrade(%q, %q) expected error, got nil", c.current, c.target)
			}
			continue
		}
		if err != nil {
			t.Errorf("IsDowngrade(%q, %q) unexpected error: %v", c.current, c.target, err)
			continue
		}
		if got != c.want {
			t.Errorf("IsDowngrade(%q, %q) = %v, want %v", c.current, c.target, got, c.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		input      string
		major      int
		minor      int
		patch      int
		preRelease string
		build      string
		wantErr    bool
	}{
		{"1.2.3", 1, 2, 3, "", "", false},
		{"0.0.0", 0, 0, 0, "", "", false},
		{"10.20.30", 10, 20, 30, "", "", false},
		{"1.2.3-beta", 1, 2, 3, "beta", "", false},
		{"1.2.3-beta.1", 1, 2, 3, "beta.1", "", false},
		{"1.2.3+build.001", 1, 2, 3, "", "build.001", false},
		{"1.2.3-beta+build", 1, 2, 3, "beta", "build", false},
		{"", 0, 0, 0, "", "", true},
		{"1", 0, 0, 0, "", "", true},
		{"1.2", 0, 0, 0, "", "", true},
		{"1.2.3.4", 0, 0, 0, "", "", true},
		{"v1.2.3", 0, 0, 0, "", "", true},
	}
	for _, c := range cases {
		v, err := ParseVersion(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q) expected error, got nil", c.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", c.input, err)
			continue
		}
		if v.Major != c.major || v.Minor != c.minor || v.Patch != c.patch {
			t.Errorf("ParseVersion(%q) version = %d.%d.%d, want %d.%d.%d", c.input, v.Major, v.Minor, v.Patch, c.major, c.minor, c.patch)
		}
		if v.PreRelease != c.preRelease {
			t.Errorf("ParseVersion(%q) preRelease = %q, want %q", c.input, v.PreRelease, c.preRelease)
		}
		if v.Build != c.build {
			t.Errorf("ParseVersion(%q) build = %q, want %q", c.input, v.Build, c.build)
		}
	}
}

func TestSemanticVersion_Compare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.0.0", "1.0.0-beta", 1},
		{"1.0.0-beta", "1.0.0", -1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-beta", "1.0.0-alpha", 1},
	}
	for _, c := range cases {
		va, err := domain.ParseVersion(c.a)
		if err != nil {
			t.Fatalf("ParseVersion(%q) error: %v", c.a, err)
		}
		vb, err := domain.ParseVersion(c.b)
		if err != nil {
			t.Fatalf("ParseVersion(%q) error: %v", c.b, err)
		}
		if got := va.Compare(vb); got != c.want {
			t.Errorf("%q.Compare(%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestUpdateSourceRegistry_BuiltInSources(t *testing.T) {
	r := NewUpdateSourceRegistry()
	lf, ok := r.Get("local-file")
	if !ok {
		t.Fatalf("expected built-in local-file source to exist")
	}
	if lf.SourceType != SourceTypeLocalFile {
		t.Errorf("expected local-file type, got %s", lf.SourceType)
	}
	if !lf.Enabled {
		t.Errorf("expected local-file enabled")
	}
	or, ok := r.Get("official-registry")
	if !ok {
		t.Fatalf("expected built-in official-registry source to exist")
	}
	if or.SourceType != SourceTypeOfficialRegistry {
		t.Errorf("expected official_registry type, got %s", or.SourceType)
	}
	if or.BaseURL == "" {
		t.Errorf("expected official-registry to have base url")
	}
	if _, ok := r.Get("does-not-exist"); ok {
		t.Errorf("expected Get to return false for unknown source")
	}
}

func TestUpdateSourceRegistry_RegisterAndGet(t *testing.T) {
	r := NewUpdateSourceRegistry()
	src := &ExtensionUpdateSource{
		SourceID:    "publisher-1",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://pub.example.com",
		PublisherID: "pub",
		TrustPolicy: TrustPolicyStrict,
		Enabled:     true,
		Priority:    10,
	}
	if err := r.Register(src); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	got, ok := r.Get("publisher-1")
	if !ok {
		t.Fatalf("expected to get registered source")
	}
	if got.SourceID != "publisher-1" {
		t.Errorf("unexpected source id %s", got.SourceID)
	}
	got.Priority = 999
	again, _ := r.Get("publisher-1")
	if again.Priority == 999 {
		t.Errorf("Get should return a copy, not the internal pointer")
	}
}

func TestUpdateSourceRegistry_RegisterInvalid(t *testing.T) {
	r := NewUpdateSourceRegistry()
	cases := []struct {
		name   string
		source *ExtensionUpdateSource
	}{
		{
			"empty id",
			&ExtensionUpdateSource{SourceID: "", SourceType: SourceTypeOfficialRegistry, BaseURL: "https://x"},
		},
		{
			"invalid type",
			&ExtensionUpdateSource{SourceID: "s", SourceType: "weird", BaseURL: "https://x"},
		},
		{
			"missing base url for registry",
			&ExtensionUpdateSource{SourceID: "s", SourceType: SourceTypeOfficialRegistry, BaseURL: ""},
		},
		{
			"invalid trust policy",
			&ExtensionUpdateSource{SourceID: "s", SourceType: SourceTypeOfficialRegistry, BaseURL: "https://x", TrustPolicy: "weird"},
		},
		{
			"custom registry none trust",
			&ExtensionUpdateSource{SourceID: "s", SourceType: SourceTypeCustomRegistry, BaseURL: "https://x", TrustPolicy: TrustPolicyNone},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := r.Register(c.source); err == nil {
				t.Errorf("expected error for %s", c.name)
			}
		})
	}
}

func TestUpdateSourceRegistry_ListAndListEnabled(t *testing.T) {
	r := NewUpdateSourceRegistry()
	r.Register(&ExtensionUpdateSource{
		SourceID:    "pub-low",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://low.example.com",
		TrustPolicy: TrustPolicyLenient,
		Enabled:     true,
		Priority:    1,
	})
	r.Register(&ExtensionUpdateSource{
		SourceID:    "pub-high",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://high.example.com",
		TrustPolicy: TrustPolicyLenient,
		Enabled:     true,
		Priority:    200,
	})
	r.Register(&ExtensionUpdateSource{
		SourceID:    "pub-disabled",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://disabled.example.com",
		TrustPolicy: TrustPolicyLenient,
		Enabled:     false,
		Priority:    500,
	})
	all := r.List()
	if len(all) != 5 {
		t.Fatalf("expected 5 sources total, got %d", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Priority < all[i].Priority {
			t.Errorf("List not sorted by priority desc: %d before %d", all[i-1].Priority, all[i].Priority)
		}
	}
	enabled := r.ListEnabled()
	if len(enabled) != 4 {
		t.Fatalf("expected 4 enabled sources, got %d", len(enabled))
	}
	for _, s := range enabled {
		if !s.Enabled {
			t.Errorf("expected only enabled sources, got disabled %s", s.SourceID)
		}
	}
}

func TestUpdateSourceRegistry_Remove(t *testing.T) {
	r := NewUpdateSourceRegistry()
	r.Register(&ExtensionUpdateSource{
		SourceID:    "removable",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://rem.example.com",
		TrustPolicy: TrustPolicyLenient,
		Enabled:     true,
		Priority:    1,
	})
	if err := r.Remove("removable"); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, ok := r.Get("removable"); ok {
		t.Errorf("expected removed source to be gone")
	}
	if err := r.Remove("nonexistent"); err == nil {
		t.Errorf("expected error removing nonexistent source")
	}
	if err := r.Remove("local-file"); err == nil {
		t.Errorf("expected error removing built-in local-file source")
	}
	if err := r.Remove("official-registry"); err == nil {
		t.Errorf("expected error removing built-in official-registry source")
	}
}

func TestUpdateSourceRegistry_EnableDisable(t *testing.T) {
	r := NewUpdateSourceRegistry()
	r.Register(&ExtensionUpdateSource{
		SourceID:    "disablable",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://dis.example.com",
		TrustPolicy: TrustPolicyLenient,
		Enabled:     true,
		Priority:    1,
	})
	if err := r.Disable("disablable"); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	s, _ := r.Get("disablable")
	if s.Enabled {
		t.Errorf("expected source to be disabled")
	}
	if err := r.Enable("disablable"); err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	s, _ = r.Get("disablable")
	if !s.Enabled {
		t.Errorf("expected source to be enabled")
	}
	if err := r.Disable("local-file"); err == nil {
		t.Errorf("expected error disabling built-in local-file source")
	}
	if err := r.Disable("official-registry"); err == nil {
		t.Errorf("expected error disabling built-in official-registry source")
	}
	if err := r.Enable("nonexistent"); err == nil {
		t.Errorf("expected error enabling nonexistent source")
	}
	if err := r.Disable("nonexistent"); err == nil {
		t.Errorf("expected error disabling nonexistent source")
	}
}

func TestUpdateSourceRegistry_IsTrusted(t *testing.T) {
	r := NewUpdateSourceRegistry()
	if !r.IsTrusted("local-file") {
		t.Errorf("expected local-file to be trusted")
	}
	if !r.IsTrusted("official-registry") {
		t.Errorf("expected official-registry to be trusted")
	}
	r.Register(&ExtensionUpdateSource{
		SourceID:    "pub-strict",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://ps.example.com",
		TrustPolicy: TrustPolicyStrict,
		Enabled:     true,
		Priority:    1,
	})
	r.Register(&ExtensionUpdateSource{
		SourceID:    "pub-none",
		SourceType:  SourceTypePublisherRegistry,
		BaseURL:     "https://pn.example.com",
		TrustPolicy: TrustPolicyNone,
		Enabled:     true,
		Priority:    1,
	})
	if !r.IsTrusted("pub-strict") {
		t.Errorf("expected publisher with strict policy to be trusted")
	}
	if r.IsTrusted("pub-none") {
		t.Errorf("expected publisher with none policy to NOT be trusted")
	}
	if r.IsTrusted("unknown") {
		t.Errorf("expected unknown source to NOT be trusted")
	}
}

func TestExtensionUpdateSource_IsTrusted(t *testing.T) {
	cases := []struct {
		source  ExtensionUpdateSource
		trusted bool
	}{
		{ExtensionUpdateSource{SourceType: SourceTypeOfficialRegistry, TrustPolicy: TrustPolicyStrict}, true},
		{ExtensionUpdateSource{SourceType: SourceTypeLocalFile, TrustPolicy: TrustPolicyStrict}, true},
		{ExtensionUpdateSource{SourceType: SourceTypePublisherRegistry, TrustPolicy: TrustPolicyStrict}, true},
		{ExtensionUpdateSource{SourceType: SourceTypePublisherRegistry, TrustPolicy: TrustPolicyLenient}, true},
		{ExtensionUpdateSource{SourceType: SourceTypePublisherRegistry, TrustPolicy: TrustPolicyNone}, false},
		{ExtensionUpdateSource{SourceType: SourceTypeCustomRegistry, TrustPolicy: TrustPolicyStrict}, true},
		{ExtensionUpdateSource{SourceType: SourceTypeCustomRegistry, TrustPolicy: TrustPolicyLenient}, false},
		{ExtensionUpdateSource{SourceType: "unknown"}, false},
	}
	for _, c := range cases {
		if got := c.source.IsTrusted(); got != c.trusted {
			t.Errorf("IsTrusted() for type=%s policy=%s = %v, want %v", c.source.SourceType, c.source.TrustPolicy, got, c.trusted)
		}
	}
}

func TestUpdateManager_Creation(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	if mgr == nil {
		t.Fatalf("expected non-nil manager")
	}
	if mgr.Sources() == nil {
		t.Errorf("expected Sources accessor to return non-nil")
	}
	if mgr.Journal() == nil {
		t.Errorf("expected Journal accessor to return non-nil")
	}
	if mgr.StateMachine() == nil {
		t.Errorf("expected StateMachine accessor to return non-nil")
	}
	if mgr.Downloads() == nil {
		t.Errorf("expected Downloads accessor to return non-nil")
	}
	if mgr.Preflight() == nil {
		t.Errorf("expected Preflight accessor to return non-nil")
	}
	if mgr.Health() == nil {
		t.Errorf("expected Health accessor to return non-nil")
	}
	if mgr.Recovery() == nil {
		t.Errorf("expected Recovery accessor to return non-nil")
	}
}

func TestUpdateManager_SetGetCurrentVersion(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	if v := mgr.GetCurrentVersion("ext-1"); v != "" {
		t.Errorf("expected empty version for unknown extension, got %s", v)
	}
	mgr.SetCurrentVersion("ext-1", "1.2.3")
	if v := mgr.GetCurrentVersion("ext-1"); v != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", v)
	}
	mgr.SetCurrentVersion("ext-1", "2.0.0")
	if v := mgr.GetCurrentVersion("ext-1"); v != "2.0.0" {
		t.Errorf("expected updated version 2.0.0, got %s", v)
	}
}

func TestUpdateManager_CheckForUpdates_NoSources(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	results, err := mgr.CheckForUpdates(ctx, "dev.amitia.example/my-ext")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestUpdateManager_CheckForUpdates_ActiveOperation(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	meta := validMetadata()
	op, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("CreateUpdateOperation failed: %v", err)
	}
	if op.Status != StateCreated {
		t.Errorf("expected initial status created, got %s", op.Status)
	}
	_, err = mgr.CheckForUpdates(ctx, meta.ExtensionID)
	if err == nil {
		t.Fatalf("expected error for active operation")
	}
	if !errors.Is(err, ErrUpdateAlreadyRunning) {
		t.Errorf("expected ErrUpdateAlreadyRunning, got %v", err)
	}
}

func TestUpdateManager_CheckForUpdates_AfterCancel(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	meta := validMetadata()
	op, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("CreateUpdateOperation failed: %v", err)
	}
	if err := mgr.CancelUpdate(ctx, op.OperationID); err != nil {
		t.Fatalf("CancelUpdate failed: %v", err)
	}
	refreshed, _ := mgr.GetOperation(op.OperationID)
	if refreshed.Status != StateCancelled {
		t.Errorf("expected status cancelled, got %s", refreshed.Status)
	}
	results, err := mgr.CheckForUpdates(ctx, meta.ExtensionID)
	if err != nil {
		t.Fatalf("expected no error after cancel, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results after cancel, got %d", len(results))
	}
}

func TestUpdateManager_CreateUpdateOperation_Valid(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	meta := validMetadata()
	op, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.OperationID == "" {
		t.Errorf("expected non-empty operation id")
	}
	if op.ExtensionID != meta.ExtensionID {
		t.Errorf("expected extension id %s, got %s", meta.ExtensionID, op.ExtensionID)
	}
	if op.ToVersion != meta.Version {
		t.Errorf("expected to version %s, got %s", meta.Version, op.ToVersion)
	}
	if op.Status != StateCreated {
		t.Errorf("expected status created, got %s", op.Status)
	}
	if op.Plan == nil {
		t.Errorf("expected non-nil plan")
	}
	if op.Plan.OperationID != op.OperationID {
		t.Errorf("expected plan operation id to match")
	}
	if op.Plan.RollbackPlan.CanRollback != true {
		t.Errorf("expected default rollback plan to allow rollback")
	}
	journalEntries := mgr.Journal().GetEntries(op.OperationID)
	if len(journalEntries) == 0 {
		t.Errorf("expected journal to record operation creation")
	}
}

func TestUpdateManager_CreateUpdateOperation_Invalid(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	cases := []struct {
		name string
		meta ExtensionUpdateMetadata
	}{
		{
			"empty extension id",
			ExtensionUpdateMetadata{
				Version: "1.0.0", ManifestVersion: 1, PackageURL: "https://x",
				PackageSHA256: "abc", PackageSize: 1, PublisherID: "p",
			},
		},
		{
			"empty version",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", ManifestVersion: 1, PackageURL: "https://x",
				PackageSHA256: "abc", PackageSize: 1, PublisherID: "p",
			},
		},
		{
			"invalid version",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", Version: "bad",
				ManifestVersion: 1, PackageURL: "https://x",
				PackageSHA256: "abc", PackageSize: 1, PublisherID: "p",
			},
		},
		{
			"empty package url",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", Version: "1.0.0",
				ManifestVersion: 1, PackageSHA256: "abc", PackageSize: 1, PublisherID: "p",
			},
		},
		{
			"empty sha",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", Version: "1.0.0",
				ManifestVersion: 1, PackageURL: "https://x", PackageSize: 1, PublisherID: "p",
			},
		},
		{
			"empty publisher",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", Version: "1.0.0",
				ManifestVersion: 1, PackageURL: "https://x", PackageSHA256: "abc", PackageSize: 1,
			},
		},
		{
			"zero manifest version",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", Version: "1.0.0",
				PackageURL: "https://x", PackageSHA256: "abc", PackageSize: 1, PublisherID: "p",
			},
		},
		{
			"non-positive size",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", Version: "1.0.0",
				ManifestVersion: 1, PackageURL: "https://x", PackageSHA256: "abc", PackageSize: 0, PublisherID: "p",
			},
		},
		{
			"invalid channel",
			ExtensionUpdateMetadata{
				ExtensionID: "dev.amitia.example/e", Version: "1.0.0",
				ManifestVersion: 1, PackageURL: "https://x", PackageSHA256: "abc", PackageSize: 1,
				PublisherID: "p", ReleaseChannel: "weird",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := mgr.CreateUpdateOperation(ctx, c.meta.ExtensionID, c.meta)
			if err == nil {
				t.Errorf("expected error for %s", c.name)
			}
			if !errors.Is(err, ErrInvalidMetadata) {
				t.Errorf("expected ErrInvalidMetadata for %s, got %v", c.name, err)
			}
		})
	}
}

func TestUpdateManager_CreateUpdateOperation_AlreadyRunning(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	meta := validMetadata()
	if _, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta); err != nil {
		t.Fatalf("first CreateUpdateOperation failed: %v", err)
	}
	_, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err == nil {
		t.Fatalf("expected error for second operation")
	}
	if !errors.Is(err, ErrUpdateAlreadyRunning) {
		t.Errorf("expected ErrUpdateAlreadyRunning, got %v", err)
	}
}

func TestUpdateManager_GetOperation(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	if _, ok := mgr.GetOperation("nonexistent"); ok {
		t.Errorf("expected false for unknown operation")
	}
	meta := validMetadata()
	op, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("CreateUpdateOperation failed: %v", err)
	}
	got, ok := mgr.GetOperation(op.OperationID)
	if !ok {
		t.Fatalf("expected to get operation")
	}
	if got.OperationID != op.OperationID {
		t.Errorf("expected operation id %s, got %s", op.OperationID, got.OperationID)
	}
}

func TestUpdateManager_ListOperations(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	if all := mgr.ListAllOperations(); len(all) != 0 {
		t.Errorf("expected empty list, got %d", len(all))
	}
	meta := validMetadata()
	op1, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("CreateUpdateOperation failed: %v", err)
	}
	meta2 := validMetadata()
	meta2.ExtensionID = "dev.amitia.example/other-ext"
	meta2.Version = "2.0.0"
	op2, err := mgr.CreateUpdateOperation(ctx, meta2.ExtensionID, meta2)
	if err != nil {
		t.Fatalf("second CreateUpdateOperation failed: %v", err)
	}
	all := mgr.ListAllOperations()
	if len(all) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(all))
	}
	byExt := mgr.ListOperationsByExtension(meta.ExtensionID)
	if len(byExt) != 1 {
		t.Fatalf("expected 1 operation for extension, got %d", len(byExt))
	}
	if byExt[0].OperationID != op1.OperationID {
		t.Errorf("unexpected operation id %s", byExt[0].OperationID)
	}
	byExt2 := mgr.ListOperationsByExtension(meta2.ExtensionID)
	if len(byExt2) != 1 || byExt2[0].OperationID != op2.OperationID {
		t.Errorf("unexpected operations for second extension")
	}
	if byExtEmpty := mgr.ListOperationsByExtension("unknown"); len(byExtEmpty) != 0 {
		t.Errorf("expected 0 operations for unknown extension, got %d", len(byExtEmpty))
	}
}

func TestUpdateManager_CancelOperation(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	meta := validMetadata()
	op, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("CreateUpdateOperation failed: %v", err)
	}
	if err := mgr.CancelUpdate(ctx, op.OperationID); err != nil {
		t.Fatalf("CancelUpdate failed: %v", err)
	}
	got, _ := mgr.GetOperation(op.OperationID)
	if got.Status != StateCancelled {
		t.Errorf("expected status cancelled, got %s", got.Status)
	}
	if got.FinishedAt == nil {
		t.Errorf("expected FinishedAt to be set")
	}
	if !mgr.StateMachine().IsTerminal(got.Status) {
		t.Errorf("expected cancelled to be terminal")
	}
	entries := mgr.Journal().GetEntries(op.OperationID)
	foundCancel := false
	for _, e := range entries {
		if e.Step == "cancel" && e.Status == JournalStatusCompleted {
			foundCancel = true
		}
	}
	if !foundCancel {
		t.Errorf("expected journal to record cancel step")
	}
}

func TestUpdateManager_CancelNonexistent(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	err := mgr.CancelUpdate(ctx, "nonexistent")
	if err == nil {
		t.Fatalf("expected error for unknown operation")
	}
	if !errors.Is(err, ErrUpdateOperationNotFound) {
		t.Errorf("expected ErrUpdateOperationNotFound, got %v", err)
	}
}

func TestUpdateManager_StageUpdate(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	meta := validMetadata()
	op, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("CreateUpdateOperation failed: %v", err)
	}
	stored, _ := mgr.GetOperation(op.OperationID)
	stored.Status = StateStaging
	if err := mgr.StageUpdate(ctx, op.OperationID); err != nil {
		t.Fatalf("StageUpdate failed: %v", err)
	}
	got, _ := mgr.GetOperation(op.OperationID)
	if got.Status != StatePreflight {
		t.Errorf("expected status preflight after staging, got %s", got.Status)
	}
	if got.StagingPath == "" {
		t.Errorf("expected staging path to be set")
	}
	if got.NewGeneration <= 0 {
		t.Errorf("expected new generation > 0, got %d", got.NewGeneration)
	}
}

func TestUpdateManager_ActivateGeneration_AssignsGeneration(t *testing.T) {
	mgr := NewUpdateManager(t.TempDir(), "1.0.0")
	ctx := context.Background()
	meta := validMetadata()
	op, err := mgr.CreateUpdateOperation(ctx, meta.ExtensionID, meta)
	if err != nil {
		t.Fatalf("CreateUpdateOperation failed: %v", err)
	}
	stored, _ := mgr.GetOperation(op.OperationID)
	stored.Status = StateActivating
	stored.NewGeneration = 0
	if err := mgr.ActivateGeneration(ctx, op.OperationID); err != nil {
		t.Fatalf("ActivateGeneration failed: %v", err)
	}
	got, _ := mgr.GetOperation(op.OperationID)
	if got.NewGeneration <= 0 {
		t.Errorf("expected generation to be assigned, got %d", got.NewGeneration)
	}
	if got.Status != StateVerifyingHealth {
		t.Errorf("expected status verifying_health, got %s", got.Status)
	}
}

func TestUpdateMetadata_Validate(t *testing.T) {
	m := validMetadata()
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid metadata to pass, got %v", err)
	}
	empty := ExtensionUpdateMetadata{}
	if err := empty.Validate(); err == nil {
		t.Errorf("expected error for empty metadata")
	}
	m = ExtensionUpdateMetadata{
		ExtensionID:     "dev.amitia.example/e",
		Version:         "1.0.0",
		ManifestVersion: 1,
		PackageURL:      "https://x",
		PackageSHA256:   "abc",
		PackageSize:     1,
		PublisherID:     "p",
		ReleaseChannel:  "stable",
	}
	if err := m.Validate(); err != nil {
		t.Errorf("expected stable channel to pass, got %v", err)
	}
	m.ReleaseChannel = "beta"
	if err := m.Validate(); err != nil {
		t.Errorf("expected beta channel to pass, got %v", err)
	}
	m.ReleaseChannel = "nightly"
	if err := m.Validate(); err != nil {
		t.Errorf("expected nightly channel to pass, got %v", err)
	}
	m.ReleaseChannel = "weird"
	if err := m.Validate(); err == nil {
		t.Errorf("expected error for invalid channel")
	}
}

func TestUpdateMetadata_MigrationHelpers(t *testing.T) {
	m := ExtensionUpdateMetadata{
		ExtensionID:     "dev.amitia.example/e",
		Version:         "1.0.0",
		ManifestVersion: 1,
		PackageURL:      "https://x",
		PackageSHA256:   "abc",
		PackageSize:     1,
		PublisherID:     "p",
	}
	if m.HasMigration() {
		t.Errorf("expected no migration when Migration is nil")
	}
	if !m.IsMigrationReversible() {
		t.Errorf("expected reversible when Migration is nil")
	}
	if m.RequiresConfirmation() {
		t.Errorf("expected no confirmation when Migration is nil and rollback not none")
	}
	m.Migration = &MigrationMetadata{HasMigration: true, IsReversible: false, RequiresManualConfirmation: true}
	if !m.HasMigration() {
		t.Errorf("expected has migration")
	}
	if m.IsMigrationReversible() {
		t.Errorf("expected not reversible")
	}
	if !m.RequiresConfirmation() {
		t.Errorf("expected requires confirmation")
	}
	m.Migration = &MigrationMetadata{HasMigration: true, IsReversible: true}
	m.RollbackPolicy = "none"
	if !m.RequiresConfirmation() {
		t.Errorf("expected requires confirmation when rollback policy none")
	}
}

func TestUpdateMetadata_SupportsPlatformArch(t *testing.T) {
	m := ExtensionUpdateMetadata{}
	if !m.SupportsPlatform("windows") {
		t.Errorf("expected any platform supported when list empty")
	}
	if !m.SupportsArch("amd64") {
		t.Errorf("expected any arch supported when list empty")
	}
	m.SupportedPlatforms = []string{"windows", "linux"}
	m.SupportedArch = []string{"amd64", "arm64"}
	if !m.SupportsPlatform("windows") {
		t.Errorf("expected windows supported")
	}
	if m.SupportsPlatform("darwin") {
		t.Errorf("expected darwin not supported")
	}
	if !m.SupportsArch("amd64") {
		t.Errorf("expected amd64 supported")
	}
	if m.SupportsArch("386") {
		t.Errorf("expected 386 not supported")
	}
}
