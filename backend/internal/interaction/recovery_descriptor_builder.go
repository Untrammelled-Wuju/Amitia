package interaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type GoalRecoveryReader interface {
	GetGoal(ctx context.Context, goalID string) (decision.Goal, bool)
}

type TaskRecoveryReader interface {
	GetTaskRun(ctx context.Context, taskRunID string) (*taskRunRecoveryView, error)
	GetLatestCheckpoint(ctx context.Context, taskRunID string) (*taskCheckpointRecoveryView, error)
}

type WorkflowRecoveryReader interface {
	GetRun(ctx context.Context, executionID string) (*workflowRunRecoveryView, error)
	ListCheckpoints(ctx context.Context, executionID string) ([]workflowCheckpointRecoveryView, error)
}

type InvocationRecoveryReader interface {
	GetInvocation(ctx context.Context, invocationID string) (*invocationRecoveryView, error)
}

type PipelineRecoveryReader interface {
	Load(ctx context.Context, conversationID, pipelineType string) (*pipelineRecordView, error)
}

type taskRunRecoveryView struct {
	TaskRunID        string
	TaskDefinitionID string
	Generation       int64
	Status           string
	DefinitionHash   string
	InputHash        string
	CheckpointID     *string
}

type taskCheckpointRecoveryView struct {
	CheckpointID   string
	Version        int64
	DefinitionHash string
	InputHash      string
}

type workflowRunRecoveryView struct {
	WorkflowID     string
	ExecutionID    string
	Generation     int64
	Status         string
	DefinitionHash string
}

type workflowCheckpointRecoveryView struct {
	NodeID string
}

type invocationRecoveryView struct {
	InvocationID string
	OperationID  string
	TraceID      string
	ToolID       string
	CapabilityID string
	Status       string
}

type pipelineRecordView struct {
	ConversationID      string
	PipelineType        string
	CheckpointVersion   int
	LastMessageSequence int64
}

type RecoveryDescriptorInput struct {
	Interaction           *InteractionRecord
	GoalRefs              []decision.GoalRef
	PlanID                string
	CandidateID           string
	ActionID              string
	ActionKind            string
	ObservationID         string
	ObservationOutcome    decision.ObservationOutcome
	TaskRunID             string
	WorkflowExecutionID   string
	InvocationID          string
	ReflectionCandidateID string
	ReconciliationScanID  string
	Require               RecoveryRequirement
}

type RecoveryDescriptorBuilder struct {
	goals       GoalRecoveryReader
	tasks       TaskRecoveryReader
	workflows   WorkflowRecoveryReader
	invocations InvocationRecoveryReader
	pipelines   PipelineRecoveryReader
	now         func() time.Time
}

func NewRecoveryDescriptorBuilder(
	goals GoalRecoveryReader,
	tasks TaskRecoveryReader,
	workflows WorkflowRecoveryReader,
	invocations InvocationRecoveryReader,
	pipelines PipelineRecoveryReader,
) *RecoveryDescriptorBuilder {
	return &RecoveryDescriptorBuilder{
		goals:       goals,
		tasks:       tasks,
		workflows:   workflows,
		invocations: invocations,
		pipelines:   pipelines,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (b *RecoveryDescriptorBuilder) Build(ctx context.Context, input RecoveryDescriptorInput) (*RecoveryDescriptor, error) {
	if input.Interaction == nil {
		return nil, errors.New("recovery: interaction is nil")
	}
	intr := input.Interaction
	scope := intr.Scope
	status := intr.Status
	statusVersion := intr.StatusVersion
	commitID := intr.CommitID
	requestID := scope.RequestID

	desc := &RecoveryDescriptor{
		SchemaVersion: RecoveryDescriptorSchemaVersion,
		Requirement:   input.Require,
		Revision:      1,
		State:         RecoveryDescriptorActive,
		CreatedAt:     b.now(),
		UpdatedAt:     b.now(),
		Interaction: RecoveryInteractionRef{
			InteractionID: intr.ID,
			RequestID:     requestID,
			Status:        status,
			StatusVersion: statusVersion,
			CommitID:      commitID,
		},
		Scope: RecoveryScopeRef{
			UserID:         scope.UserID,
			CharacterID:    scope.CharacterID,
			ConversationID: scope.ConversationID,
			Channel:        scope.Channel,
			PeerID:         scope.PeerID,
			SessionID:      scope.SessionID,
		},
		Goals: make([]RecoveryGoalRef, 0, len(input.GoalRefs)),
	}
	if desc.Requirement == "" {
		desc.Requirement = RecoveryBestEffort
	}
	if err := b.attachGoals(ctx, desc, input.GoalRefs, scope); err != nil {
		return nil, err
	}
	if input.PlanID != "" {
		desc.Plan = &RecoveryPlanRef{PlanID: input.PlanID, CandidateID: input.CandidateID, GoalRefs: toPlanGoalRefs(desc.Goals)}
	}
	if input.ActionID != "" {
		desc.Action = &RecoveryActionRef{ActionID: input.ActionID, Kind: input.ActionKind}
	}
	if input.ObservationID != "" {
		desc.Observation = &RecoveryObservationRef{
			ObservationID: input.ObservationID,
			ActionID:      input.ActionID,
			Outcome:       input.ObservationOutcome,
		}
	}
	if err := b.attachTask(ctx, desc, input, scope); err != nil {
		return nil, err
	}
	if err := b.attachWorkflow(ctx, desc, input, scope); err != nil {
		return nil, err
	}
	if err := b.attachInvocation(ctx, desc, input); err != nil {
		return nil, err
	}
	if err := b.attachPipeline(ctx, desc, scope); err != nil {
		return nil, err
	}
	if input.ReflectionCandidateID != "" || input.ReconciliationScanID != "" {
		desc.Mind = &RecoveryMindRef{
			ReflectionCandidateID: input.ReflectionCandidateID,
			ReconciliationScanID:  input.ReconciliationScanID,
		}
	}
	desc.ComputeFingerprint()
	return desc, nil
}

func (b *RecoveryDescriptorBuilder) attachGoals(ctx context.Context, desc *RecoveryDescriptor, refs []decision.GoalRef, scope InteractionScope) error {
	for _, ref := range refs {
		if ref.ID == "" {
			continue
		}
		if b.goals == nil {
			return fmt.Errorf("goal_missing: %s", ref.ID)
		}
		g, ok := b.goals.GetGoal(ctx, ref.ID)
		if !ok {
			return fmt.Errorf("goal_missing: %s", ref.ID)
		}
		if g.UserID != scope.UserID {
			return fmt.Errorf("goal_scope_mismatch: %s", ref.ID)
		}
		if g.CharacterID != "" && scope.CharacterID != "" && g.CharacterID != scope.CharacterID {
			return fmt.Errorf("goal_scope_mismatch: %s", ref.ID)
		}
		if g.ConversationID != "" && scope.ConversationID != "" && g.ConversationID != scope.ConversationID {
			return fmt.Errorf("goal_scope_mismatch: %s", ref.ID)
		}
		desc.Goals = append(desc.Goals, RecoveryGoalRef{
			GoalID:   g.ID,
			Revision: g.Revision,
			Status:   g.Status,
		})
	}
	return nil
}

func (b *RecoveryDescriptorBuilder) attachTask(ctx context.Context, desc *RecoveryDescriptor, input RecoveryDescriptorInput, scope InteractionScope) error {
	if input.TaskRunID == "" || b.tasks == nil {
		return nil
	}
	run, err := b.tasks.GetTaskRun(ctx, input.TaskRunID)
	if err != nil || run == nil {
		return fmt.Errorf("task_missing: %s", input.TaskRunID)
	}
	ref := &RecoveryTaskRef{
		TaskRunID:        run.TaskRunID,
		TaskDefinitionID: run.TaskDefinitionID,
		Generation:       run.Generation,
		Status:           task_runtime.TaskRunStatus(run.Status),
		DefinitionHash:   run.DefinitionHash,
		InputHash:        run.InputHash,
	}
	if run.CheckpointID != nil && *run.CheckpointID != "" {
		cp, err := b.tasks.GetLatestCheckpoint(ctx, run.TaskRunID)
		if err != nil || cp == nil {
			return fmt.Errorf("task_checkpoint_missing: %s", run.TaskRunID)
		}
		ref.CheckpointID = cp.CheckpointID
		ref.CheckpointVersion = cp.Version
	}
	desc.Task = ref
	return nil
}

func (b *RecoveryDescriptorBuilder) attachWorkflow(ctx context.Context, desc *RecoveryDescriptor, input RecoveryDescriptorInput, scope InteractionScope) error {
	if input.WorkflowExecutionID == "" || b.workflows == nil {
		return nil
	}
	run, err := b.workflows.GetRun(ctx, input.WorkflowExecutionID)
	if err != nil || run == nil {
		return fmt.Errorf("workflow_missing: %s", input.WorkflowExecutionID)
	}
	cps, err := b.workflows.ListCheckpoints(ctx, run.ExecutionID)
	if err != nil {
		return fmt.Errorf("workflow_checkpoint_list_failed: %s", run.ExecutionID)
	}
	nodeIDs := make([]string, 0, len(cps))
	for _, cp := range cps {
		nodeIDs = append(nodeIDs, cp.NodeID)
	}
	desc.Workflow = &RecoveryWorkflowRef{
		WorkflowID:               run.WorkflowID,
		ExecutionID:              run.ExecutionID,
		Generation:               run.Generation,
		Status:                   workflow.RunStatus(run.Status),
		DefinitionHash:           run.DefinitionHash,
		CompletedCheckpointNodes: nodeIDs,
	}
	return nil
}

func (b *RecoveryDescriptorBuilder) attachInvocation(ctx context.Context, desc *RecoveryDescriptor, input RecoveryDescriptorInput) error {
	if input.InvocationID == "" || b.invocations == nil {
		return nil
	}
	inv, err := b.invocations.GetInvocation(ctx, input.InvocationID)
	if err != nil || inv == nil {
		return fmt.Errorf("invocation_missing: %s", input.InvocationID)
	}
	desc.Kernel = &RecoveryInvocationRef{
		InvocationID: inv.InvocationID,
		OperationID:  inv.OperationID,
		TraceID:      inv.TraceID,
		ToolID:       inv.ToolID,
		Status:       inv.Status,
	}
	return nil
}

func (b *RecoveryDescriptorBuilder) attachPipeline(ctx context.Context, desc *RecoveryDescriptor, scope InteractionScope) error {
	if b.pipelines == nil || scope.ConversationID == "" {
		return nil
	}
	rec, err := b.pipelines.Load(ctx, scope.ConversationID, "default")
	if err != nil || rec == nil {
		return nil
	}
	desc.Pipeline = &RecoveryPipelineCheckpointRef{
		ConversationID:      rec.ConversationID,
		PipelineType:        rec.PipelineType,
		CheckpointVersion:   rec.CheckpointVersion,
		LastMessageSequence: rec.LastMessageSequence,
	}
	return nil
}

func toPlanGoalRefs(in []RecoveryGoalRef) []RecoveryGoalRef {
	if len(in) == 0 {
		return nil
	}
	out := make([]RecoveryGoalRef, len(in))
	copy(out, in)
	return out
}
