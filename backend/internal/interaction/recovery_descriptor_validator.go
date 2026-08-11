package interaction

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/decision"
)

type RecoveryIssueCode string

const (
	IssueInteractionMissing           RecoveryIssueCode = "interaction_missing"
	IssueInteractionVersionMismatch   RecoveryIssueCode = "interaction_version_mismatch"
	IssueGoalMissing                  RecoveryIssueCode = "goal_missing"
	IssueStaleGoalRevision            RecoveryIssueCode = "stale_goal_revision"
	IssueGoalScopeMismatch            RecoveryIssueCode = "goal_scope_mismatch"
	IssueTaskMissing                  RecoveryIssueCode = "task_missing"
	IssueTaskGenerationMismatch       RecoveryIssueCode = "task_generation_mismatch"
	IssueTaskCheckpointMissing        RecoveryIssueCode = "task_checkpoint_missing"
	IssueTaskCheckpointIncompatible   RecoveryIssueCode = "task_checkpoint_incompatible"
	IssueWorkflowMissing              RecoveryIssueCode = "workflow_missing"
	IssueWorkflowGenerationMismatch   RecoveryIssueCode = "workflow_generation_mismatch"
	IssueWorkflowCheckpointMissing    RecoveryIssueCode = "workflow_checkpoint_missing"
	IssueInvocationMissing            RecoveryIssueCode = "invocation_missing"
	IssueInvocationNonTerminal        RecoveryIssueCode = "invocation_non_terminal"
	IssueInvocationOutcomeUnknown     RecoveryIssueCode = "invocation_outcome_unknown"
	IssuePipelineCheckpointStale      RecoveryIssueCode = "pipeline_checkpoint_stale"
	IssueScopeMismatch                RecoveryIssueCode = "scope_mismatch"
	IssueDescriptorCorrupt            RecoveryIssueCode = "descriptor_corrupt"
	IssueDescriptorVersionUnsupported RecoveryIssueCode = "descriptor_version_unsupported"
)

type RecoveryIssue struct {
	Code          RecoveryIssueCode `json:"code"`
	ReferenceType string            `json:"referenceType,omitempty"`
	ReferenceID   string            `json:"referenceId,omitempty"`
	Severity      string            `json:"severity"`
}

type RecoveryReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type RecoveryValidationResult struct {
	Compatibility   RecoveryCompatibility `json:"compatibility"`
	RecoveryClass   AgentRecoveryClass    `json:"recoveryClass"`
	Issues          []RecoveryIssue       `json:"issues"`
	RecoverableRefs []RecoveryReference   `json:"recoverableRefs,omitempty"`
	StaleRefs       []RecoveryReference   `json:"staleRefs,omitempty"`
}

type RecoveryDescriptorValidator struct {
	goals       GoalRecoveryReader
	tasks       TaskRecoveryReader
	workflows   WorkflowRecoveryReader
	invocations InvocationRecoveryReader
	pipelines   PipelineRecoveryReader
}

func NewRecoveryDescriptorValidator(
	goals GoalRecoveryReader,
	tasks TaskRecoveryReader,
	workflows WorkflowRecoveryReader,
	invocations InvocationRecoveryReader,
	pipelines PipelineRecoveryReader,
) *RecoveryDescriptorValidator {
	return &RecoveryDescriptorValidator{
		goals:       goals,
		tasks:       tasks,
		workflows:   workflows,
		invocations: invocations,
		pipelines:   pipelines,
	}
}

func (v *RecoveryDescriptorValidator) Validate(ctx context.Context, d *RecoveryDescriptor) (RecoveryValidationResult, error) {
	result := RecoveryValidationResult{
		Compatibility: RecoveryCompatible,
		RecoveryClass: AgentRecoveryNotRecoverable,
		Issues:        make([]RecoveryIssue, 0),
	}
	if d == nil {
		result.Issues = append(result.Issues, RecoveryIssue{Code: IssueDescriptorCorrupt, Severity: "critical"})
		result.Compatibility = RecoveryIncomplete
		result.RecoveryClass = AgentRecoveryManual
		return result, fmt.Errorf("descriptor is nil")
	}
	if d.SchemaVersion != RecoveryDescriptorSchemaVersion {
		result.Issues = append(result.Issues, RecoveryIssue{Code: IssueDescriptorVersionUnsupported, Severity: "critical"})
		result.Compatibility = RecoveryIncomplete
		result.RecoveryClass = AgentRecoveryManual
		return result, fmt.Errorf("unsupported schema")
	}
	v.validateInteraction(ctx, d, &result)
	v.validateGoals(ctx, d, &result)
	v.validateTask(ctx, d, &result)
	v.validateWorkflow(ctx, d, &result)
	v.validateInvocation(ctx, d, &result)
	v.validatePipeline(ctx, d, &result)
	v.classify(d, &result)
	return result, nil
}

func (v *RecoveryDescriptorValidator) addIssue(result *RecoveryValidationResult, code RecoveryIssueCode, refType, refID, severity string) {
	result.Issues = append(result.Issues, RecoveryIssue{
		Code:          code,
		ReferenceType: refType,
		ReferenceID:   refID,
		Severity:      severity,
	})
	switch severity {
	case "critical":
		result.Compatibility = RecoveryIncomplete
	case "warning":
		if result.Compatibility == RecoveryCompatible || result.Compatibility == RecoveryIncomplete {
			// Only escalate if not already incompatible
		}
		if result.Compatibility == RecoveryCompatible {
			result.Compatibility = RecoveryStale
		}
	}
}

func (v *RecoveryDescriptorValidator) validateInteraction(ctx context.Context, d *RecoveryDescriptor, result *RecoveryValidationResult) {
	if d.Interaction.InteractionID == "" {
		v.addIssue(result, IssueInteractionMissing, "interaction", d.Interaction.InteractionID, "critical")
	}
}

func (v *RecoveryDescriptorValidator) validateGoals(ctx context.Context, d *RecoveryDescriptor, result *RecoveryValidationResult) {
	if len(d.Goals) == 0 {
		return
	}
	if v.goals == nil {
		v.addIssue(result, IssueGoalMissing, "goals", "", "critical")
		return
	}
	for i := range d.Goals {
		ref := d.Goals[i]
		g, ok := v.goals.GetGoal(ctx, ref.GoalID)
		if !ok {
			v.addIssue(result, IssueGoalMissing, "goal", ref.GoalID, "critical")
			continue
		}
		if g.Revision != ref.Revision {
			v.addIssue(result, IssueStaleGoalRevision, "goal", ref.GoalID, "warning")
			result.StaleRefs = append(result.StaleRefs, RecoveryReference{Type: "goal", ID: ref.GoID()})
			continue
		}
		if g.UserID != d.Scope.UserID {
			v.addIssue(result, IssueGoalScopeMismatch, "goal", ref.GoalID, "critical")
			continue
		}
		if d.Scope.CharacterID != "" && g.CharacterID != "" && g.CharacterID != d.Scope.CharacterID {
			v.addIssue(result, IssueGoalScopeMismatch, "goal", ref.GoalID, "critical")
			continue
		}
		result.RecoverableRefs = append(result.RecoverableRefs, RecoveryReference{Type: "goal", ID: ref.GoalID})
	}
}

func (r *RecoveryGoalRef) GoID() string { return r.GoalID }

func (v *RecoveryDescriptorValidator) validateTask(ctx context.Context, d *RecoveryDescriptor, result *RecoveryValidationResult) {
	if d.Task == nil {
		return
	}
	if v.tasks == nil {
		v.addIssue(result, IssueTaskMissing, "task", d.Task.TaskRunID, "critical")
		return
	}
	run, err := v.tasks.GetTaskRun(ctx, d.Task.TaskRunID)
	if err != nil || run == nil {
		v.addIssue(result, IssueTaskMissing, "task", d.Task.TaskRunID, "critical")
		return
	}
	if run.Generation != d.Task.Generation {
		v.addIssue(result, IssueTaskGenerationMismatch, "task", d.Task.TaskRunID, "warning")
		result.StaleRefs = append(result.StaleRefs, RecoveryReference{Type: "task", ID: d.Task.TaskRunID})
		return
	}
	if d.Task.CheckpointID != "" {
		cp, err := v.tasks.GetLatestCheckpoint(ctx, d.Task.TaskRunID)
		if err != nil || cp == nil {
			v.addIssue(result, IssueTaskCheckpointMissing, "task", d.Task.TaskRunID, "critical")
			return
		}
		if cp.DefinitionHash != d.Task.DefinitionHash {
			v.addIssue(result, IssueTaskCheckpointIncompatible, "task", d.Task.TaskRunID, "warning")
		}
		if cp.InputHash != d.Task.InputHash {
			v.addIssue(result, IssueTaskCheckpointIncompatible, "task", d.Task.TaskRunID, "warning")
		}
		result.RecoverableRefs = append(result.RecoverableRefs, RecoveryReference{Type: "task", ID: d.Task.TaskRunID})
	} else {
		result.RecoverableRefs = append(result.RecoverableRefs, RecoveryReference{Type: "task", ID: d.Task.TaskRunID})
	}
}

func (v *RecoveryDescriptorValidator) validateWorkflow(ctx context.Context, d *RecoveryDescriptor, result *RecoveryValidationResult) {
	if d.Workflow == nil {
		return
	}
	if v.workflows == nil {
		v.addIssue(result, IssueWorkflowMissing, "workflow", d.Workflow.ExecutionID, "critical")
		return
	}
	run, err := v.workflows.GetRun(ctx, d.Workflow.ExecutionID)
	if err != nil || run == nil {
		v.addIssue(result, IssueWorkflowMissing, "workflow", d.Workflow.ExecutionID, "critical")
		return
	}
	if run.Generation != d.Workflow.Generation {
		v.addIssue(result, IssueWorkflowGenerationMismatch, "workflow", d.Workflow.ExecutionID, "warning")
		result.StaleRefs = append(result.StaleRefs, RecoveryReference{Type: "workflow", ID: d.Workflow.ExecutionID})
		return
	}
	cps, err := v.workflows.ListCheckpoints(ctx, d.Workflow.ExecutionID)
	if err != nil {
		v.addIssue(result, IssueWorkflowCheckpointMissing, "workflow", d.Workflow.ExecutionID, "warning")
		return
	}
	existing := make(map[string]bool, len(cps))
	for _, cp := range cps {
		existing[cp.NodeID] = true
	}
	for _, nodeID := range d.Workflow.CompletedCheckpointNodes {
		if !existing[nodeID] {
			v.addIssue(result, IssueWorkflowCheckpointMissing, "workflow:node", nodeID, "warning")
		}
	}
	result.RecoverableRefs = append(result.RecoverableRefs, RecoveryReference{Type: "workflow", ID: d.Workflow.ExecutionID})
}

func (v *RecoveryDescriptorValidator) validateInvocation(ctx context.Context, d *RecoveryDescriptor, result *RecoveryValidationResult) {
	if d.Kernel == nil {
		return
	}
	if v.invocations == nil {
		v.addIssue(result, IssueInvocationMissing, "kernel", d.Kernel.InvocationID, "critical")
		return
	}
	inv, err := v.invocations.GetInvocation(ctx, d.Kernel.InvocationID)
	if err != nil || inv == nil {
		v.addIssue(result, IssueInvocationMissing, "kernel", d.Kernel.InvocationID, "critical")
		return
	}
	switch inv.Status {
	case "succeeded", "failed", "cancelled", "timed_out", "denied":
		result.RecoverableRefs = append(result.RecoverableRefs, RecoveryReference{Type: "kernel", ID: d.Kernel.InvocationID})
	case "", "starting", "running", "pending":
		v.addIssue(result, IssueInvocationNonTerminal, "kernel", d.Kernel.InvocationID, "critical")
	default:
		v.addIssue(result, IssueInvocationOutcomeUnknown, "kernel", d.Kernel.InvocationID, "critical")
	}
}

func (v *RecoveryDescriptorValidator) validatePipeline(ctx context.Context, d *RecoveryDescriptor, result *RecoveryValidationResult) {
	if d.Pipeline == nil || v.pipelines == nil {
		return
	}
	rec, err := v.pipelines.Load(ctx, d.Pipeline.ConversationID, d.Pipeline.PipelineType)
	if err != nil || rec == nil {
		return
	}
	if rec.LastMessageSequence > d.Pipeline.LastMessageSequence {
		v.addIssue(result, IssuePipelineCheckpointStale, "pipeline", d.Pipeline.ConversationID, "warning")
	}
}

func (v *RecoveryDescriptorValidator) classify(d *RecoveryDescriptor, result *RecoveryValidationResult) {
	if result.Compatibility == RecoveryIncomplete {
		result.RecoveryClass = AgentRecoveryManual
		return
	}
	if len(result.Issues) == 0 {
		if d.Task != nil && hasRecoverableTask(d, result) {
			result.RecoveryClass = AgentRecoveryTaskRequired
			return
		}
		if d.Workflow != nil && hasRecoverableWorkflow(d, result) {
			result.RecoveryClass = AgentRecoveryWorkflowRequired
			return
		}
		if isTerminalDescriptor(d) {
			result.RecoveryClass = AgentRecoveryTerminal
			return
		}
		result.RecoveryClass = AgentRecoverySafeResume
		return
	}
	if result.Compatibility == RecoveryManual {
		result.RecoveryClass = AgentRecoveryManual
		return
	}
	if hasNonIdempotentUnknownInvocation(result) {
		result.RecoveryClass = AgentRecoveryManual
		return
	}
	if staleGoalOnly(d, result) {
		result.RecoveryClass = AgentRecoveryReplanRequired
		return
	}
	result.RecoveryClass = AgentRecoveryManual
}

func hasRecoverableTask(d *RecoveryDescriptor, result *RecoveryValidationResult) bool {
	if d.Task == nil {
		return false
	}
	if d.Task.Status == "recovery_required" {
		return true
	}
	return false
}

func hasRecoverableWorkflow(d *RecoveryDescriptor, result *RecoveryValidationResult) bool {
	if d.Workflow == nil {
		return false
	}
	switch d.Workflow.Status {
	case "running", "stale", "compensating":
		return true
	}
	return false
}

func isTerminalDescriptor(d *RecoveryDescriptor) bool {
	return d.State == RecoveryDescriptorTerminal
}

func hasNonIdempotentUnknownInvocation(result *RecoveryValidationResult) bool {
	for _, i := range result.Issues {
		if i.Code == IssueInvocationOutcomeUnknown {
			return true
		}
	}
	return false
}

func staleGoalOnly(d *RecoveryDescriptor, result *RecoveryValidationResult) bool {
	hasStale := false
	for _, i := range result.Issues {
		switch i.Code {
		case IssueInteractionMissing, IssueGoalMissing, IssueGoalScopeMismatch,
			IssueTaskMissing, IssueTaskCheckpointMissing, IssueWorkflowMissing,
			IssueWorkflowCheckpointMissing, IssueInvocationMissing, IssueInvocationOutcomeUnknown:
			return false
		case IssueStaleGoalRevision:
			hasStale = true
		}
	}
	return hasStale && hasStaleGoalRef(d)
}

func hasStaleGoalRef(d *RecoveryDescriptor) bool {
	if len(d.Goals) == 0 {
		return false
	}
	for _, g := range d.Goals {
		if isGoalStatusRevoked(g.Status) {
			return true
		}
	}
	return false
}

func isGoalStatusRevoked(s decision.GoalStatus) bool {
	switch s {
	case decision.GoalStatusAbandoned, decision.GoalStatusSuspended:
		return true
	}
	return false
}
