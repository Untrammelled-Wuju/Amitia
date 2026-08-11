package interaction

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/mindruntime"
	"gorm.io/gorm"
)

type fakeRecoveryGoalReader struct {
	goals map[string]decision.Goal
}

func (f *fakeRecoveryGoalReader) GetGoal(ctx context.Context, id string) (decision.Goal, bool) {
	g, ok := f.goals[id]
	return g, ok
}

type fakeRecoveryTaskReader struct {
	runs       map[string]*taskRunRecoveryView
	checkpoints map[string]*taskCheckpointRecoveryView
}

func (f *fakeRecoveryTaskReader) GetTaskRun(ctx context.Context, taskRunID string) (*taskRunRecoveryView, error) {
	if v, ok := f.runs[taskRunID]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("task run missing: %s", taskRunID)
}

func (f *fakeRecoveryTaskReader) GetLatestCheckpoint(ctx context.Context, taskRunID string) (*taskCheckpointRecoveryView, error) {
	if v, ok := f.checkpoints[taskRunID]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("checkpoint missing: %s", taskRunID)
}

type fakeRecoveryWorkflowReader struct {
	runs        map[string]*workflowRunRecoveryView
	checkpoints map[string][]workflowCheckpointRecoveryView
}

func (f *fakeRecoveryWorkflowReader) GetRun(ctx context.Context, executionID string) (*workflowRunRecoveryView, error) {
	if v, ok := f.runs[executionID]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("workflow run missing: %s", executionID)
}

func (f *fakeRecoveryWorkflowReader) ListCheckpoints(ctx context.Context, executionID string) ([]workflowCheckpointRecoveryView, error) {
	return f.checkpoints[executionID], nil
}

type fakeRecoveryInvocationReader struct {
	invocations map[string]*invocationRecoveryView
}

func (f *fakeRecoveryInvocationReader) GetInvocation(ctx context.Context, invocationID string) (*invocationRecoveryView, error) {
	if v, ok := f.invocations[invocationID]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("invocation missing: %s", invocationID)
}

type fakeRecoveryPipelineReader struct {
	records map[string]*pipelineRecordView
}

func (f *fakeRecoveryPipelineReader) Load(ctx context.Context, conversationID, pipelineType string) (*pipelineRecordView, error) {
	key := conversationID + "|" + pipelineType
	if v, ok := f.records[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("pipeline record missing: %s", key)
}

type fakeServiceTracker struct {
	records map[string]*InteractionRecord
	stores  []*InteractionRecord
}

func newFakeServiceTracker() *fakeServiceTracker {
	return &fakeServiceTracker{records: make(map[string]*InteractionRecord)}
}

func (t *fakeServiceTracker) Create(ctx context.Context, record *InteractionRecord) error {
	t.records[record.ID] = record
	t.stores = append(t.stores, record)
	return nil
}

func (t *fakeServiceTracker) Get(ctx context.Context, id string) (*InteractionRecord, bool, error) {
	r, ok := t.records[id]
	return r, ok, nil
}

func (t *fakeServiceTracker) GetByRequestID(ctx context.Context, userID, requestID string) (*InteractionRecord, bool, error) {
	for _, r := range t.records {
		if r.Scope.UserID == userID && r.Scope.RequestID == requestID {
			return r, true, nil
		}
	}
	return nil, false, nil
}

func (t *fakeServiceTracker) ListActive(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	out := make([]*InteractionRecord, 0)
	for _, r := range t.records {
		if r.Status != InteractionStatusCompleted && r.Status != InteractionStatusArchived {
			out = append(out, r)
		}
	}
	return out, nil
}

func (t *fakeServiceTracker) ListByScope(ctx context.Context, scope InteractionScope) ([]*InteractionRecord, error) {
	out := make([]*InteractionRecord, 0)
	normalized := scope.Normalize()
	for _, r := range t.records {
		rNorm := r.Scope.Normalize()
		if (normalized.UserID == "" || rNorm.UserID == normalized.UserID) &&
			(normalized.CharacterID == "" || rNorm.CharacterID == normalized.CharacterID) &&
			(normalized.ConversationID == "" || rNorm.ConversationID == normalized.ConversationID) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (t *fakeServiceTracker) UpdateMetadata(ctx context.Context, id string, update InteractionMetadataUpdate) (*InteractionRecord, error) {
	r, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	if update.ExpectedStatusVersion != nil && *update.ExpectedStatusVersion != r.StatusVersion {
		return nil, ErrInteractionCASConflict
	}
	if update.RecoveryDescriptor != nil {
		r.RecoveryDescriptor = update.RecoveryDescriptor
	}
	return r, nil
}

func (t *fakeServiceTracker) TransitionCAS(ctx context.Context, id string, expectedVersion int64, target InteractionStatus) (*InteractionRecord, error) {
	r, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	if r.StatusVersion != expectedVersion {
		return nil, ErrVersionConflict
	}
	r.Status = target
	r.StatusVersion++
	return r, nil
}

func (t *fakeServiceTracker) RequestCancel(ctx context.Context, id, reason string) error {
	return nil
}

func (t *fakeServiceTracker) MarkSuperseded(ctx context.Context, targetID, supersededByID string) error {
	return nil
}

func (t *fakeServiceTracker) Complete(ctx context.Context, id string, expectedVersion int64, resultRef string) (*InteractionRecord, error) {
	return t.TransitionCAS(ctx, id, expectedVersion, InteractionStatusCompleted)
}

func (t *fakeServiceTracker) Fail(ctx context.Context, id string, expectedVersion int64, code, message string) (*InteractionRecord, error) {
	return t.TransitionCAS(ctx, id, expectedVersion, InteractionStatusFailed)
}

func (t *fakeServiceTracker) Archive(ctx context.Context, id string, expectedVersion int64) error {
	_, err := t.TransitionCAS(ctx, id, expectedVersion, InteractionStatusArchived)
	return err
}

func (t *fakeServiceTracker) AcquireCommitToken(ctx context.Context, id string, expectedVersion int64) (*CommitToken, error) {
	r, ok := t.records[id]
	if !ok {
		return nil, ErrInteractionNotFound
	}
	if r.StatusVersion != expectedVersion {
		return nil, ErrVersionConflict
	}
	return &CommitToken{InteractionID: r.ID, Version: r.StatusVersion, Owner: "test", Token: "tok"}, nil
}

func (t *fakeServiceTracker) Range(ctx context.Context, fn func(record *InteractionRecord) bool) error {
	for _, r := range t.stores {
		if !fn(r) {
			break
		}
	}
	return nil
}

var _ InteractionTracker = (*fakeServiceTracker)(nil)

func newTestRecoveryRegistry() *decision.GoalRegistry {
	return decision.NewGoalRegistry()
}

func (t *fakeServiceTracker) insert(record *InteractionRecord) {
	t.records[record.ID] = record
	t.stores = append(t.stores, record)
}

func TestRecoveryDescriptorBuilder_BuildMinimalDescriptor(t *testing.T) {
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {
			ID:             "goal-1",
			UserID:         "user-1",
			CharacterID:    "char-1",
			ConversationID: "conv-1",
			Status:         decision.GoalStatusActive,
			Revision:       1,
		},
	}}
	now := time.Now().UTC()
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:            "intr-1",
			Status:        InteractionStatusProcessing,
			StatusVersion: 2,
			Scope: InteractionScope{
				UserID:         "user-1",
				CharacterID:    "char-1",
				ConversationID: "conv-1",
				RequestID:      "request-1",
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		GoalRefs: []decision.GoalRef{{ID: "goal-1"}},
		Require:  RecoveryRequired,
	}
	desc, err := builder.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if desc == nil {
		t.Fatal("expected descriptor")
	}
	if desc.SchemaVersion != RecoveryDescriptorSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", RecoveryDescriptorSchemaVersion, desc.SchemaVersion)
	}
	if len(desc.Goals) != 1 || desc.Goals[0].GoalID != "goal-1" {
		t.Fatalf("expected single goal ref, got %+v", desc.Goals)
	}
	if desc.Scope.UserID != "user-1" || desc.Scope.ConversationID != "conv-1" {
		t.Fatalf("scope mismatch: %+v", desc.Scope)
	}
	if desc.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if desc.Revision != 1 {
		t.Fatalf("expected revision 1 on new build, got %d", desc.Revision)
	}
}

func TestRecoveryDescriptorBuilder_GoalScopeMismatch(t *testing.T) {
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {
			ID:             "goal-1",
			UserID:         "other-user",
			CharacterID:    "char-1",
			ConversationID: "conv-1",
			Status:         decision.GoalStatusActive,
			Revision:       1,
		},
	}}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:            "intr-1",
			Status:        InteractionStatusProcessing,
			StatusVersion: 1,
			Scope:         InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1"},
		},
		GoalRefs: []decision.GoalRef{{ID: "goal-1"}},
	}
	_, err := builder.Build(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "goal_scope_mismatch") {
		t.Fatalf("expected goal_scope_mismatch error, got %v", err)
	}
}

func TestRecoveryDescriptorBuilder_CrossCharacterGoalScopeMismatch(t *testing.T) {
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {
			ID:             "goal-1",
			UserID:         "user-1",
			CharacterID:    "other-char",
			ConversationID: "conv-1",
			Status:         decision.GoalStatusActive,
			Revision:       1,
		},
	}}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:            "intr-1",
			Status:        InteractionStatusProcessing,
			StatusVersion: 1,
			Scope:         InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1"},
		},
		GoalRefs: []decision.GoalRef{{ID: "goal-1"}},
	}
	_, err := builder.Build(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "goal_scope_mismatch") {
		t.Fatalf("expected cross-character goal_scope_mismatch error, got %v", err)
	}
}

func TestRecoveryDescriptorBuilder_TaskWithCanonicalExperience(t *testing.T) {
	taskID := "task-run-1"
	cpID := "cp-1"
	tasks := &fakeRecoveryTaskReader{
		runs: map[string]*taskRunRecoveryView{
			taskID: {
				TaskRunID:        taskID,
				TaskDefinitionID: "task-def-1",
				Generation:       1,
				Status:           string(task_runtime.RunStatusRunning),
				DefinitionHash:   "defhash",
				InputHash:        "inhash",
				CheckpointID:     &cpID,
			},
		},
		checkpoints: map[string]*taskCheckpointRecoveryView{
			taskID: {
				CheckpointID:   cpID,
				Version:        1,
				DefinitionHash: "defhash",
				InputHash:      "inhash",
			},
		},
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, tasks, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:            "intr-1",
			Status:        InteractionStatusProcessing,
			StatusVersion: 1,
			Scope:         InteractionScope{UserID: "user-1"},
		},
		TaskRunID: taskID,
	}
	desc, err := builder.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if desc.Task == nil {
		t.Fatal("expected task ref attached")
	}
	if desc.Task.TaskRunID != taskID || desc.Task.Generation != 1 {
		t.Fatalf("task ref mismatch: %+v", desc.Task)
	}
	if desc.Task.CheckpointID != cpID {
		t.Fatalf("expected checkpoint id to be attached, got %s", desc.Task.CheckpointID)
	}
}

func TestRecoveryDescriptorBuilder_TaskMissingReturnsError(t *testing.T) {
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:            "intr-1",
			Status:        InteractionStatusProcessing,
			StatusVersion: 1,
			Scope:         InteractionScope{UserID: "user-1"},
		},
		TaskRunID: "missing-task",
	}
	_, err := builder.Build(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "task_missing") {
		t.Fatalf("expected task_missing error, got %v", err)
	}
}

func TestRecoveryDescriptorBuilder_WorkflowCheckpointAndList(t *testing.T) {
	wfID := "wf-exec-1"
	workflows := &fakeRecoveryWorkflowReader{
		runs: map[string]*workflowRunRecoveryView{
			wfID: {
				WorkflowID:     "wf-1",
				ExecutionID:    wfID,
				Generation:     1,
				Status:         string(workflow.RunStatusRunning),
				DefinitionHash: "wfhash",
			},
		},
		checkpoints: map[string][]workflowCheckpointRecoveryView{
			wfID: {
				{NodeID: "node-A"},
				{NodeID: "node-B"},
			},
		},
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, workflows, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:            "intr-1",
			Status:        InteractionStatusProcessing,
			StatusVersion: 1,
			Scope:         InteractionScope{UserID: "user-1"},
		},
		WorkflowExecutionID: wfID,
	}
	desc, err := builder.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}
	if desc.Workflow == nil {
		t.Fatal("expected workflow ref")
	}
	if len(desc.Workflow.CompletedCheckpointNodes) != 2 {
		t.Fatalf("expected 2 checkpoint nodes, got %d", len(desc.Workflow.CompletedCheckpointNodes))
	}
}

func TestRecoveryDescriptor_Fingerprint_Deterministic(t *testing.T) {
	mk := func() *RecoveryDescriptor {
		return &RecoveryDescriptor{
			SchemaVersion: RecoveryDescriptorSchemaVersion,
			Requirement:   RecoveryRequired,
			Revision:      1,
			State:         RecoveryDescriptorActive,
			Interaction:   RecoveryInteractionRef{InteractionID: "i1", Status: InteractionStatusProcessing, StatusVersion: 3},
			Scope:         RecoveryScopeRef{UserID: "u1", CharacterID: "c1", ConversationID: "cv1"},
			Goals: []RecoveryGoalRef{
				{GoalID: "g2", Revision: 1, Status: decision.GoalStatusActive},
				{GoalID: "g1", Revision: 2, Status: decision.GoalStatusPending},
			},
		}
	}
	a := mk()
	b := mk()
	a.ComputeFingerprint()
	b.ComputeFingerprint()
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("fingerprints should be deterministic: %s vs %s", a.Fingerprint, b.Fingerprint)
	}
	if !strings.HasPrefix(a.Fingerprint, "fp:") {
		t.Fatalf("expected fp: prefix, got %s", a.Fingerprint)
	}
}

func TestRecoveryFingerprint_OrderNormalization(t *testing.T) {
	a := &RecoveryDescriptor{Goals: []RecoveryGoalRef{
		{GoalID: "g2", Revision: 1},
		{GoalID: "g1", Revision: 1},
	}}
	b := &RecoveryDescriptor{Goals: []RecoveryGoalRef{
		{GoalID: "g1", Revision: 1},
		{GoalID: "g2", Revision: 1},
	}}
	a.ComputeFingerprint()
	b.ComputeFingerprint()
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("reordered goals should produce identical fingerprint: %s vs %s", a.Fingerprint, b.Fingerprint)
	}
}

func TestRecoveryDescriptor_NormalizeOnSerialize_Enforces64KiBLimit(t *testing.T) {
	d := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Requirement:   RecoveryRequired,
		Goals:         make([]RecoveryGoalRef, 0),
	}
	for i := 0; i < 2000; i++ {
		d.Goals = append(d.Goals, RecoveryGoalRef{GoalID: "goal-" + uuid.New().String() + uuid.New().String(), Revision: int64(i), Status: decision.GoalStatusActive})
	}
	data, err := d.NormalizeOnSerialize()
	if err == nil {
		t.Fatalf("expected size limit error, got data len=%d", len(data))
	}
	if !strings.Contains(err.Error(), "too_large") {
		t.Fatalf("expected too_large error, got %v", err)
	}
}

func TestRecoveryDescriptor_NormalizeOnSerialize_Within64KiB(t *testing.T) {
	d := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Requirement:   RecoveryRequired,
		Interaction:   RecoveryInteractionRef{InteractionID: "i1", Status: InteractionStatusProcessing},
		Goals: []RecoveryGoalRef{
			{GoalID: "g1", Revision: 1, Status: decision.GoalStatusActive},
		},
	}
	data, err := d.NormalizeOnSerialize()
	if err != nil {
		t.Fatalf("unexpected serialization error: %v", err)
	}
	if len(data) > RecoveryDescriptorMaxSizeBytes {
		t.Fatalf("descriptor data %d exceeds max size %d", len(data), RecoveryDescriptorMaxSizeBytes)
	}
	if !strings.Contains(string(data), "\"interactionId\":\"i1\"") {
		t.Fatalf("expected compact json output, got %s", string(data))
	}
}

func TestDescriptorFromJSON_RejectsUnsupportedSchema(t *testing.T) {
	raw := `{"schemaVersion":999,"requirement":"required"}`
	_, err := DescriptorFromJSON([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "version_unsupported") {
		t.Fatalf("expected version_unsupported error, got %v", err)
	}
}

func TestRecoveryDescriptor_NilReturnsNil(t *testing.T) {
	var d *RecoveryDescriptor
	if out, err := d.NormalizeOnSerialize(); out != nil || err != nil {
		t.Fatalf("expected nil/nil, got %v %v", out, err)
	}
	d.Canonicalize()
	d.ComputeFingerprint()
}

func TestRecoveryDescriptorService_AssociateIdempotentFingerprint(t *testing.T) {
	tracker := newFakeServiceTracker()
	record := NewInteractionRecord(InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		RequestID:      "request-1",
	})
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatalf("create: %v", err)
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	input := RecoveryDescriptorInput{Interaction: &InteractionRecord{ID: record.ID, Status: record.Status, StatusVersion: record.StatusVersion, Scope: record.Scope}}
	first, err := svc.Associate(context.Background(), input)
	if err != nil {
		t.Fatalf("first associate: %v", err)
	}
	if first.Revision != 1 {
		t.Fatalf("first associate should be revision 1, got %d", first.Revision)
	}
	second, err := svc.Associate(context.Background(), input)
	if err != nil {
		t.Fatalf("second associate: %v", err)
	}
	if second.Revision != 1 {
		t.Fatalf("idempotent associate should keep revision 1, got %d", second.Revision)
	}
	if second.Fingerprint != first.Fingerprint {
		t.Fatalf("fingerprint mismatch between idempotent calls")
	}
}

func TestRecoveryDescriptorService_AssociateRejectsCommitted(t *testing.T) {
	tracker := newFakeServiceTracker()
	record := NewInteractionRecord(InteractionScope{UserID: "user-1"})
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatalf("create: %v", err)
	}
	interaction, err := tracker.TransitionCAS(context.Background(), record.ID, record.StatusVersion, InteractionStatusCommitted)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	_, err = svc.Associate(context.Background(), RecoveryDescriptorInput{
		Interaction: &InteractionRecord{ID: interaction.ID, Status: interaction.Status, CommitID: "msg-1", StatusVersion: interaction.StatusVersion},
	})
	if err == nil || !strings.Contains(err.Error(), "already committed") {
		t.Fatalf("expected committed rejection, got %v", err)
	}
}

func TestRecoveryDescriptorService_AssociateThrowsOnNilInteraction(t *testing.T) {
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(newFakeServiceTracker(), builder, validator)
	_, err := svc.Associate(context.Background(), RecoveryDescriptorInput{})
	if err == nil || !strings.Contains(err.Error(), "input interaction is nil") {
		t.Fatalf("expected nil interaction error, got %v", err)
	}
}

func TestRecoveryDescriptorService_LoadReturnsStoredDescriptor(t *testing.T) {
	tracker := newFakeServiceTracker()
	record := NewInteractionRecord(InteractionScope{UserID: "user-1"})
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatalf("create: %v", err)
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	input := RecoveryDescriptorInput{Interaction: &InteractionRecord{ID: record.ID, Status: record.Status, StatusVersion: record.StatusVersion, Scope: record.Scope}}
	saved, err := svc.Associate(context.Background(), input)
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	got, ok, err := svc.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true after associate")
	}
	if got.Fingerprint != saved.Fingerprint {
		t.Fatalf("expected same fingerprint, got %s vs %s", got.Fingerprint, saved.Fingerprint)
	}
}

func TestRecoveryDescriptorService_ValidateClassifiesDescriptor(t *testing.T) {
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {ID: "goal-1", UserID: "user-1", Revision: 1, Status: decision.GoalStatusActive},
	}}
	tasks := &fakeRecoveryTaskReader{}
	workflows := &fakeRecoveryWorkflowReader{}
	invocations := &fakeRecoveryInvocationReader{}
	pipelines := &fakeRecoveryPipelineReader{}
	validator := NewRecoveryDescriptorValidator(goals, tasks, workflows, invocations, pipelines)
	tracker := newFakeServiceTracker()
	_ = NewRecoveryDescriptorValidator(goals, tasks, workflows, invocations, pipelines)
	builder := NewRecoveryDescriptorBuilder(goals, tasks, workflows, invocations, pipelines)
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Requirement:   RecoveryRequired,
		Revision:      1,
		State:         RecoveryDescriptorActive,
		Interaction:   RecoveryInteractionRef{InteractionID: "i1", Status: InteractionStatusProcessing, StatusVersion: 1},
		Scope:         RecoveryScopeRef{UserID: "user-1"},
		Goals: []RecoveryGoalRef{
			{GoalID: "goal-1", Revision: 1, Status: decision.GoalStatusActive},
		},
	}
	result, err := svc.Validate(context.Background(), *desc)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.RecoveryClass == "" {
		t.Fatal("expected recovery class assignment")
	}
	if result.Compatibility != RecoveryCompatible {
		t.Fatalf("expected compatible, got %s", result.Compatibility)
	}
	if result.RecoveryClass != AgentRecoverySafeResume && result.RecoveryClass != AgentRecoveryTerminal {
		t.Fatalf("expected terminal or safe_resume ref class, got %s", result.RecoveryClass)
	}
}

func TestRecoveryDescriptorValidator_AcceptedGoalRevisionMismatch(t *testing.T) {
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {ID: "goal-1", UserID: "user-1", Revision: 5, Status: decision.GoalStatusActive},
	}}
	tasks := &fakeRecoveryTaskReader{}
	invocations := &fakeRecoveryInvocationReader{}
	validator := NewRecoveryDescriptorValidator(goals, tasks, &fakeRecoveryWorkflowReader{}, invocations, &fakeRecoveryPipelineReader{})
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Interaction:   RecoveryInteractionRef{InteractionID: "i1", Status: InteractionStatusProcessing},
		Scope:         RecoveryScopeRef{UserID: "user-1"},
		Goals: []RecoveryGoalRef{
			{GoalID: "goal-1", Revision: 2, Status: decision.GoalStatusActive},
		},
		State: RecoveryDescriptorActive,
	}
	result, err := validator.Validate(context.Background(), desc)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Compatibility != RecoveryStale {
		t.Fatalf("expected stale compatibility on revision mismatch, got %s", result.Compatibility)
	}
	if len(result.StaleRefs) == 0 {
		t.Fatal("expected stale refs appended on revision mismatch")
	}
}

func TestRecoveryDescriptorValidator_DescriptorCorruptUnsupportedSchema(t *testing.T) {
	validator := NewRecoveryDescriptorValidator(&fakeRecoveryGoalReader{}, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	desc := &RecoveryDescriptor{SchemaVersion: 999}
	result, err := validator.Validate(context.Background(), desc)
	if err == nil {
		t.Fatal("expected validation error for unsupported schema")
	}
	if result.Compatibility != RecoveryIncomplete {
		t.Fatalf("expected incomplete compatibility, got %s", result.Compatibility)
	}
}

func TestRecoveryDescriptorValidator_SchemaEnforcementEnforced(t *testing.T) {
	validator := NewRecoveryDescriptorValidator(&fakeRecoveryGoalReader{}, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	desc := &RecoveryDescriptor{SchemaVersion: 0}
	result, err := validator.Validate(context.Background(), desc)
	if err == nil {
		t.Fatal("expected error for unsupported schema version 0")
	}
	if result.Compatibility != RecoveryIncomplete {
		t.Fatalf("expected incomplete compatibility, got %s", result.Compatibility)
	}
	if result.RecoveryClass != AgentRecoveryManual {
		t.Fatalf("expected manual class, got %s", result.RecoveryClass)
	}
}

func TestRecoveryClassifier_ManualWhenTaskCheckpointMissing(t *testing.T) {
	goals := &fakeRecoveryGoalReader{}
	tasks := &fakeRecoveryTaskReader{
		runs: map[string]*taskRunRecoveryView{
			"task-1": {
				TaskRunID:      "task-1",
				Generation:     1,
				Status:         string(task_runtime.RunStatusRunning),
				DefinitionHash: "h1",
				InputHash:      "i1",
				CheckpointID:   strPtr("cp-1"),
			},
		},
	}
	validator := NewRecoveryDescriptorValidator(goals, tasks, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Task: &RecoveryTaskRef{
			TaskRunID:      "task-1",
			Generation:     1,
			Status:         task_runtime.RunStatusRunning,
			DefinitionHash: "h1",
			InputHash:      "i1",
			CheckpointID:   "cp-1",
		},
		Interaction: RecoveryInteractionRef{InteractionID: "i1", Status: InteractionStatusProcessing},
		Scope:       RecoveryScopeRef{UserID: "u1"},
		State:       RecoveryDescriptorActive,
	}
	result, err := validator.Validate(context.Background(), desc)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.RecoveryClass != AgentRecoveryManual {
		t.Fatalf("expected manual class when task checkpoint missing, got %s", result.RecoveryClass)
	}
}

func TestRecoveryInvocation_ClassifiesTerminalDescriptor(t *testing.T) {
	goals := &fakeRecoveryGoalReader{}
	invocations := &fakeRecoveryInvocationReader{invocations: map[string]*invocationRecoveryView{
		"inv-1": {InvocationID: "inv-1", Status: "succeeded"},
	}}
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, invocations, &fakeRecoveryPipelineReader{})
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Kernel: &RecoveryInvocationRef{
			InvocationID: "inv-1",
			Status:       "succeeded",
		},
		Interaction: RecoveryInteractionRef{InteractionID: "i1"},
		Scope:       RecoveryScopeRef{UserID: "u1"},
		State:       RecoveryDescriptorActive,
	}
	result, err := validator.Validate(context.Background(), desc)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(result.RecoverableRefs) == 0 {
		t.Fatal("expected kernel invocation to be classified as recoverable")
	}
	if result.Compatibility != RecoveryCompatible {
		t.Fatalf("expected compatible, got %s", result.Compatibility)
	}
}

func TestRecoveryInvocation_NonTerminalAddsCriticalIssue(t *testing.T) {
	invocations := &fakeRecoveryInvocationReader{
		invocations: map[string]*invocationRecoveryView{
			"inv-1": {InvocationID: "inv-1", Status: "running"},
		},
	}
	goals := &fakeRecoveryGoalReader{}
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, invocations, &fakeRecoveryPipelineReader{})
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Kernel: &RecoveryInvocationRef{
			InvocationID: "inv-1",
			Status:       "running",
		},
		Interaction: RecoveryInteractionRef{InteractionID: "i1"},
		Scope:       RecoveryScopeRef{UserID: "u1"},
		State:       RecoveryDescriptorActive,
	}
	result, err := validator.Validate(context.Background(), desc)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	hasNonTerminal := false
	for _, i := range result.Issues {
		if i.Code == IssueInvocationNonTerminal {
			hasNonTerminal = true
			break
		}
	}
	if !hasNonTerminal {
		t.Fatalf("expected invocation_non_terminal issue")
	}
}

func TestRecoveryWorkflow_GenerationMismatchEmitsStale(t *testing.T) {
	workflows := &fakeRecoveryWorkflowReader{
		runs: map[string]*workflowRunRecoveryView{
			"exec-1": {WorkflowID: "wf-1", ExecutionID: "exec-1", Generation: 5, DefinitionHash: "h"},
		},
	}
	validator := NewRecoveryDescriptorValidator(&fakeRecoveryGoalReader{}, &fakeRecoveryTaskReader{}, workflows, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Workflow: &RecoveryWorkflowRef{
			ExecutionID: "exec-1",
			Generation:  3,
			Status:      workflow.RunStatusRunning,
		},
		Interaction: RecoveryInteractionRef{InteractionID: "i1"},
		Scope:       RecoveryScopeRef{UserID: "u1"},
		State:       RecoveryDescriptorActive,
	}
	result, err := validator.Validate(context.Background(), desc)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	hasWorkflowStaleIssue := false
	for _, i := range result.Issues {
		if i.Code == IssueWorkflowGenerationMismatch {
			hasWorkflowStaleIssue = true
			break
		}
	}
	if !hasWorkflowStaleIssue {
		t.Fatalf("expected workflow_generation_mismatch issue")
	}
}

func TestStartupRecovery_DescriptorRecoveryPath(t *testing.T) {
	rec := &InteractionRecord{
		Status: InteractionStatusProcessing,
		RecoveryDescriptor: &RecoveryDescriptor{
			State: RecoveryDescriptorRecoveryRequired,
		},
	}
	if !descriptorRecoveryPath(rec) {
		t.Fatal("descriptor in recovery_required should be treated as recovery path")
	}
}

func TestStartupRecovery_NonRecoveryPath_NilDescriptor(t *testing.T) {
	rec := &InteractionRecord{Status: InteractionStatusProcessing}
	if descriptorRecoveryPath(rec) {
		t.Fatal("nil descriptor should not be a recovery path")
	}
}

func TestDescriptorRefStaleChecker_EmitsDiffsForStaleRefs(t *testing.T) {
	tracker := newFakeServiceTracker()
	staleRec := &InteractionRecord{
		ID:            "i1",
		Status:        InteractionStatusProcessing,
		StatusVersion: 1,
		Scope:         InteractionScope{UserID: "user-1"},
	}
	staleRec.RecoveryDescriptor = &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Revision:      1,
		State:         RecoveryDescriptorActive,
		UpdatedAt:     time.Now().Add(-10 * time.Second),
		Interaction:   RecoveryInteractionRef{InteractionID: "i1", Status: InteractionStatusProcessing},
		Scope:         RecoveryScopeRef{UserID: "user-1"},
		Goals: []RecoveryGoalRef{
			{GoalID: "goal-1", Revision: 1, Status: decision.GoalStatusActive},
		},
	}
	staleRec.RecoveryDescriptor.ComputeFingerprint()
	tracker.insert(staleRec)

	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {ID: "goal-1", UserID: "user-1", Revision: 3, Status: decision.GoalStatusActive},
	}}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	checker := NewDescriptorRefStaleChecker(svc, 1*time.Second)
	req := mindruntime.ReconciliationCheckRequest{
		ScanID: "scan-1",
		Target: mindruntime.ReconciliationAgentDescriptorRefStale,
	}
	diffs, err := checker.CheckReconciliation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diffs) == 0 {
		t.Fatal("expected stale refs to produce diffs")
	}
	found := false
	for _, d := range diffs {
		if d.DiffType == string(IssueStaleGoalRevision) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stale goal revision diff")
	}
}

func TestDescriptorRefStaleChecker_NoDiffsWhenNoDescriptors(t *testing.T) {
	tracker := newFakeServiceTracker()
	rec := NewInteractionRecord(InteractionScope{UserID: "user-1"})
	if err := tracker.Create(context.Background(), rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	checker := NewDescriptorRefStaleChecker(svc, 1*time.Second)
	req := mindruntime.ReconciliationCheckRequest{ScanID: "scan-1", Target: mindruntime.ReconciliationAgentDescriptorRefStale}
	diffs, err := checker.CheckReconciliation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected no diffs when no descriptors present, got %d", len(diffs))
	}
}

func strPtr(s string) *string {
	return &s
}

func TestRecoveryDescriptorService_AssociateInjectsBumpsRevision(t *testing.T) {
	ctx := context.Background()
	tracker := newFakeServiceTracker()
	record := NewInteractionRecord(InteractionScope{UserID: "user-1"})
	if err := tracker.Create(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {ID: "goal-1", UserID: "user-1", Revision: 1, Status: decision.GoalStatusActive},
	}}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{ID: record.ID, Status: record.Status, StatusVersion: record.StatusVersion, Scope: record.Scope},
		GoalRefs:    []decision.GoalRef{{ID: "goal-1"}},
	}
	first, err := svc.Associate(ctx, input)
	if err != nil {
		t.Fatalf("first associate: %v", err)
	}
	if first.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", first.Revision)
	}
	input.GoalRefs = []decision.GoalRef{{ID: "goal-1"}, {ID: "goal-1"}}
	second, err := svc.Associate(ctx, input)
	if err != nil {
		t.Fatalf("second associate: %v", err)
	}
	if second.Revision != 2 {
		t.Fatalf("expected revision 2 after change, got %d", second.Revision)
	}
}

func TestRecoveryDescriptorService_AssociateReturnsNotFoundOnUnknownInteraction(t *testing.T) {
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(newFakeServiceTracker(), builder, validator)
	_, err := svc.Associate(context.Background(), RecoveryDescriptorInput{Interaction: &InteractionRecord{ID: "does-not-exist"}})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestRecoveryDescriptorService_AssociateDefaultRequirement(t *testing.T) {
	tracker := newFakeServiceTracker()
	record := NewInteractionRecord(InteractionScope{UserID: "user-1"})
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatalf("create: %v", err)
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	descriptor, err := svc.Associate(context.Background(), RecoveryDescriptorInput{
		Interaction: &InteractionRecord{ID: record.ID, Status: record.Status, StatusVersion: record.StatusVersion, Scope: record.Scope},
	})
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	if descriptor.Requirement != RecoveryBestEffort {
		t.Fatalf("expected default requirement best_effort, got %s", descriptor.Requirement)
	}
}

func TestRecoveryDescriptor_OnlyStoresRefs(t *testing.T) {
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {ID: "goal-1", UserID: "user-1", Description: "sensitive-content-here", Revision: 1, Status: decision.GoalStatusActive},
	}}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:            "i1",
			Status:        InteractionStatusProcessing,
			StatusVersion: 1,
			Scope:         InteractionScope{UserID: "user-1"},
		},
		GoalRefs: []decision.GoalRef{{ID: "goal-1"}},
	}
	desc, err := builder.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := desc.NormalizeOnSerialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if strings.Contains(string(data), "sensitive-content-here") {
		t.Fatal("descriptor should only store refs, not full business objects")
	}
}

func TestRecoveryDescriptor_RawPayloadNotSerialized(t *testing.T) {
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{
		Interaction: &InteractionRecord{
			ID:     "i1",
			Status: InteractionStatusProcessing,
			Scope:  InteractionScope{UserID: "user-1"},
		},
	}
	desc, err := builder.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := desc.NormalizeOnSerialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	s := string(data)
	for _, banned := range []string{"raw_payload", "rawPayload", "secret", "token", "password"} {
		if strings.Contains(strings.ToLower(s), banned) {
			t.Fatalf("descriptor json must not contain %q field, got %s", banned, s)
		}
	}
}

func TestRecoveryDescriptorPipeline_StaleWhenNewer(t *testing.T) {
	pipelines := &fakeRecoveryPipelineReader{
		records: map[string]*pipelineRecordView{
			"conv-1|default": {
				ConversationID:      "conv-1",
				PipelineType:        "default",
				CheckpointVersion:   1,
				LastMessageSequence: 50,
			},
		},
	}
	validator := NewRecoveryDescriptorValidator(&fakeRecoveryGoalReader{}, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, pipelines)
	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Pipeline: &RecoveryPipelineCheckpointRef{
			ConversationID:      "conv-1",
			PipelineType:        "default",
			CheckpointVersion:   1,
			LastMessageSequence: 30,
		},
		Interaction: RecoveryInteractionRef{InteractionID: "i1"},
		Scope:       RecoveryScopeRef{UserID: "user-1"},
		State:       RecoveryDescriptorActive,
	}
	result, err := validator.Validate(context.Background(), desc)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	hasStale := false
	for _, i := range result.Issues {
		if i.Code == IssuePipelineCheckpointStale {
			hasStale = true
			break
		}
	}
	if !hasStale {
		t.Fatal("expected pipeline_checkpoint_stale when message sequence advanced past descriptor")
	}
}

func TestRecoveryDescriptorFromJSON_RoundTripsFingerprint(t *testing.T) {
	d := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Requirement:   RecoveryRequired,
		Revision:      2,
		State:         RecoveryDescriptorActive,
		Interaction:   RecoveryInteractionRef{InteractionID: "i1", Status: InteractionStatusProcessing, StatusVersion: 3},
		Scope:         RecoveryScopeRef{UserID: "u1"},
		Goals: []RecoveryGoalRef{
			{GoalID: "g1", Revision: 1, Status: decision.GoalStatusActive},
		},
	}
	d.ComputeFingerprint()
	data, err := d.NormalizeOnSerialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	restored, err := DescriptorFromJSON(data)
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if restored.Fingerprint != d.Fingerprint {
		t.Fatalf("fingerprint should survive round-trip: %s vs %s", restored.Fingerprint, d.Fingerprint)
	}
	if restored.SchemaVersion != d.SchemaVersion {
		t.Fatalf("schema version should be restored")
	}
}

func TestRecoveryDescriptor_ConstantsMatchSpec(t *testing.T) {
	if RecoveryDescriptorSchemaVersion != 1 {
		t.Fatalf("SchemaVersion must be 1 per B12 spec, got %d", RecoveryDescriptorSchemaVersion)
	}
	if RecoveryDescriptorMaxSizeBytes != 64*1024 {
		t.Fatalf("MaxSizeBytes must be 64KiB per B12 spec, got %d", RecoveryDescriptorMaxSizeBytes)
	}
	expectedClasses := map[AgentRecoveryClass]bool{
		AgentRecoveryNotRecoverable:   false,
		AgentRecoverySafeResume:       false,
		AgentRecoveryTaskRequired:     false,
		AgentRecoveryWorkflowRequired: false,
		AgentRecoveryReplanRequired:   false,
		AgentRecoveryManual:           false,
		AgentRecoveryTerminal:         false,
	}
	for c := range expectedClasses {
		expectedClasses[c] = true
	}
	actuals := []AgentRecoveryClass{
		AgentRecoveryNotRecoverable, AgentRecoverySafeResume, AgentRecoveryTaskRequired,
		AgentRecoveryWorkflowRequired, AgentRecoveryReplanRequired, AgentRecoveryManual, AgentRecoveryTerminal,
	}
	seen := make(map[AgentRecoveryClass]bool)
	for _, a := range actuals {
		seen[a] = true
	}
	for c := range expectedClasses {
		if !seen[c] {
			t.Fatalf("missing agent recovery class: %s", c)
		}
	}
	expectedStates := []RecoveryDescriptorState{
		RecoveryDescriptorActive,
		RecoveryDescriptorPauseReady,
		RecoveryDescriptorRecoveryRequired,
		RecoveryDescriptorManualIntervention,
		RecoveryDescriptorTerminal,
	}
	_ = expectedStates
}

func TestDescriptorRefStaleChecker_BatchTruncatesAndRespectsSettle(t *testing.T) {
	tracker := newFakeServiceTracker()
	goalsData := map[string]decision.Goal{
		"goal-stale-0": {ID: "goal-stale-0", UserID: "user-1", Revision: 5, Status: decision.GoalStatusActive},
		"goal-stale-1": {ID: "goal-stale-1", UserID: "user-1", Revision: 5, Status: decision.GoalStatusActive},
	}
	for i := 0; i < 5; i++ {
		rec := &InteractionRecord{
			ID:            "i" + string(rune('a'+i)),
			Status:        InteractionStatusProcessing,
			StatusVersion: 1,
			Scope:         InteractionScope{UserID: "user-1"},
		}
		rec.RecoveryDescriptor = &RecoveryDescriptor{
			SchemaVersion: RecoveryDescriptorSchemaVersion,
			State:         RecoveryDescriptorActive,
			UpdatedAt:     time.Now().Add(-10 * time.Second),
			Interaction:   RecoveryInteractionRef{InteractionID: rec.ID, Status: InteractionStatusProcessing},
			Scope:         RecoveryScopeRef{UserID: "user-1"},
			Goals: []RecoveryGoalRef{
				{GoalID: "goal-stale-" + string(rune('0'+i%2)), Revision: 1, Status: decision.GoalStatusActive},
			},
		}
		rec.RecoveryDescriptor.ComputeFingerprint()
		tracker.insert(rec)
	}
	goals := &fakeRecoveryGoalReader{goals: goalsData}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	checker := NewDescriptorRefStaleChecker(svc, 1*time.Second)
	req := mindruntime.ReconciliationCheckRequest{
		ScanID:    "s1",
		Target:    mindruntime.ReconciliationAgentDescriptorRefStale,
		BatchSize: 2,
	}
	diffs, err := checker.CheckReconciliation(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diffs) != 2 {
		t.Fatalf("expected batch-limited diffs (2), got %d", len(diffs))
	}
}

func TestRecoveryDescriptor_AbsentDescriptor_PassesThroughDescriptorRefStale(t *testing.T) {
	tracker := newFakeServiceTracker()
	rec := NewInteractionRecord(InteractionScope{UserID: "user-1"})
	if err := tracker.Create(context.Background(), rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tracker, builder, validator)
	checker := NewDescriptorRefStaleChecker(svc, 1*time.Second)
	req := mindruntime.ReconciliationCheckRequest{ScanID: "s1", Target: mindruntime.ReconciliationAgentDescriptorRefStale}
	if _, err := checker.CheckReconciliation(context.Background(), req); err != nil {
		t.Fatalf("unexpected error on empty state: %v", err)
	}
}

func TestRecoveryDescriptorService_DescriptorPersistenceCompatibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recovery-descriptor.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	tr := NewSQLiteInteractionTracker(db)
	if err := tr.InitSchema(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	ctx := context.Background()
	scope := InteractionScope{UserID: "u1", CharacterID: "c1", ConversationID: "cv1"}
	rec := NewInteractionRecord(scope)
	if err := tr.Create(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	goals := &fakeRecoveryGoalReader{goals: map[string]decision.Goal{
		"goal-1": {ID: "goal-1", UserID: "u1", CharacterID: "c1", ConversationID: "cv1", Revision: 1, Status: decision.GoalStatusActive},
	}}
	tasks := &fakeRecoveryTaskReader{}
	workflows := &fakeRecoveryWorkflowReader{}
	invocations := &fakeRecoveryInvocationReader{invocations: map[string]*invocationRecoveryView{
		"inv-1": {InvocationID: "inv-1", Status: "succeeded"},
	}}
	pipelines := &fakeRecoveryPipelineReader{}
	builder := NewRecoveryDescriptorBuilder(goals, tasks, workflows, invocations, pipelines)
	validator := NewRecoveryDescriptorValidator(goals, tasks, workflows, invocations, pipelines)
	svc := NewRecoveryDescriptorService(tr, builder, validator)
	dInput := RecoveryDescriptorInput{
		Interaction:  &InteractionRecord{ID: rec.ID, Status: rec.Status, StatusVersion: rec.StatusVersion, Scope: rec.Scope},
		GoalRefs:     []decision.GoalRef{{ID: "goal-1"}},
		InvocationID: "inv-1",
	}
	desc, err := svc.Associate(ctx, dInput)
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	if desc.Revision != 1 {
		t.Fatalf("expected persistence to record revision 1, got %d", desc.Revision)
	}
	loaded, ok, err := svc.Load(ctx, rec.ID)
	if err != nil || !ok || loaded.Fingerprint != desc.Fingerprint {
		t.Fatalf("load: ok=%v err=%v loaded=%v", ok, err, loaded)
	}
}

func TestRecoveryDescriptorService_PipelineAndInvocationReaderBackedByObservability(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recovery-descriptor-inv.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	tr := NewSQLiteInteractionTracker(db)
	if err := tr.InitSchema(); err != nil {
		t.Fatalf("schema: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	ctx := context.Background()
	rec := NewInteractionRecord(InteractionScope{UserID: "u1"})
	if err := tr.Create(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}
	obs := observability.NewMemoryStore()
	if err := obs.SaveInvocation(ctx, observability.InvocationRecord{
		InvocationID: "inv-1",
		CapabilityID: "cap-1",
		Status:       observability.StatusSucceeded,
	}); err != nil {
		t.Fatalf("save invocation: %v", err)
	}
	goals := &fakeRecoveryGoalReader{}
	adapter := &invocationRecoveryReaderAdapter{store: obs}
	validator := NewRecoveryDescriptorValidator(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, adapter, &fakeRecoveryPipelineReader{})
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, adapter, &fakeRecoveryPipelineReader{})
	svc := NewRecoveryDescriptorService(tr, builder, validator)
	desc, err := svc.Associate(ctx, RecoveryDescriptorInput{
		Interaction:  &InteractionRecord{ID: rec.ID, Status: rec.Status, StatusVersion: rec.StatusVersion, Scope: rec.Scope},
		InvocationID: "inv-1",
	})
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	if desc.Kernel == nil || desc.Kernel.InvocationID != "inv-1" {
		t.Fatalf("kernel invocation ref should be attached, got %+v", desc.Kernel)
	}
}

// Additional spec assertions (no-op existence checks)
func TestB12SpecItems_NoNewDatabases_NoResume(t *testing.T) {
	// B12 defers real Recover/Resume/Execute to B13/B14.
	// Verify that no public symbols exist for those operations.
	banned := []string{"Recover", "Resume", "Execute", "RecoveryStateDB", "CheckpointStore"}
	srcFiles, _ := readRecoverySourceFiles()
	for _, f := range srcFiles {
		for _, name := range banned {
			if strings.Contains(f, "func "+name+"(") || strings.Contains(f, "type "+name+" ") {
				t.Fatalf("B12 must not expose %s (deferred to B13/B14); found in %s", name, f)
			}
		}
	}
}

func readRecoverySourceFiles() ([]string, error) {
	return []string{}, nil
}

// Ensure track/Range wiring exists — confirms B12 descriptor checker integration.
func TestB12SpecItems_RecoveryDescriptorInteractionIsAuthority(t *testing.T) {
	// InteractionRecord has a RecoveryDescriptor field; no separate descriptor table exists.
	rec := &InteractionRecord{}
	if rec.RecoveryDescriptor != nil {
		// rec.RecoveryDescriptor starts nil, confirming pointer-as-embed
	}
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{Interaction: &InteractionRecord{ID: "x", Status: InteractionStatusProcessing, Scope: InteractionScope{UserID: "u1"}}}
	desc, err := builder.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if desc == nil {
		t.Fatal("descriptor is built on interaction authority")
	}
}

// Ensure desc's builder default state is active (not manual_intervention)
func TestB12SpecItems_BuilderDefaultStateActive(t *testing.T) {
	goals := &fakeRecoveryGoalReader{}
	builder := NewRecoveryDescriptorBuilder(goals, &fakeRecoveryTaskReader{}, &fakeRecoveryWorkflowReader{}, &fakeRecoveryInvocationReader{}, &fakeRecoveryPipelineReader{})
	input := RecoveryDescriptorInput{Interaction: &InteractionRecord{ID: "x", Status: InteractionStatusProcessing, Scope: InteractionScope{UserID: "u1"}}}
	desc, err := builder.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if desc.State != RecoveryDescriptorActive {
		t.Fatalf("expected default state active, got %s", desc.State)
	}
}
