package interaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

const (
	RecoveryDescriptorSchemaVersion = 1
	RecoveryDescriptorMaxSizeBytes  = 64 * 1024
)

type RecoveryDescriptorState string

const (
	RecoveryDescriptorActive            RecoveryDescriptorState = "active"
	RecoveryDescriptorPauseReady        RecoveryDescriptorState = "pause_ready"
	RecoveryDescriptorRecoveryRequired  RecoveryDescriptorState = "recovery_required"
	RecoveryDescriptorManualIntervention RecoveryDescriptorState = "manual_intervention"
	RecoveryDescriptorTerminal          RecoveryDescriptorState = "terminal"
)

type RecoveryRequirement string

const (
	RecoveryBestEffort RecoveryRequirement = "best_effort"
	RecoveryRequired   RecoveryRequirement = "required"
)

type RecoveryCompatibility string

const (
	RecoveryCompatible RecoveryCompatibility = "compatible"
	RecoveryStale     RecoveryCompatibility = "stale"
	RecoveryIncomplete RecoveryCompatibility = "incomplete"
	RecoveryManual     RecoveryCompatibility = "manual"
)

type AgentRecoveryClass string

const (
	AgentRecoveryNotRecoverable   AgentRecoveryClass = "not_recoverable"
	AgentRecoverySafeResume       AgentRecoveryClass = "safe_resume_candidate"
	AgentRecoveryTaskRequired     AgentRecoveryClass = "task_recovery_required"
	AgentRecoveryWorkflowRequired AgentRecoveryClass = "workflow_recovery_required"
	AgentRecoveryReplanRequired   AgentRecoveryClass = "replan_required"
	AgentRecoveryManual           AgentRecoveryClass = "manual_intervention"
	AgentRecoveryTerminal         AgentRecoveryClass = "already_terminal"
)

type RecoveryInteractionRef struct {
	InteractionID string            `json:"interactionId"`
	RequestID     string            `json:"requestId,omitempty"`
	Status        InteractionStatus `json:"status"`
	StatusVersion int64             `json:"statusVersion"`
	CommitID      string            `json:"commitId,omitempty"`
}

type RecoveryScopeRef struct {
	UserID         string `json:"userId"`
	CharacterID    string `json:"characterId"`
	ConversationID string `json:"conversationId"`
	Channel        string `json:"channel,omitempty"`
	PeerID         string `json:"peerId,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
}

type RecoveryGoalRef struct {
	GoalID   string              `json:"goalId"`
	Revision int64               `json:"revision"`
	Status   decision.GoalStatus `json:"status"`
}

type RecoveryPlanRef struct {
	PlanID      string            `json:"planId"`
	CandidateID string            `json:"candidateId,omitempty"`
	GoalRefs    []RecoveryGoalRef `json:"goalRefs,omitempty"`
}

type RecoveryActionRef struct {
	ActionID       string `json:"actionId"`
	Kind           string `json:"kind,omitempty"`
	TargetID       string `json:"targetId,omitempty"`
	ExternalCallID string `json:"externalCallId,omitempty"`
}

type RecoveryObservationRef struct {
	ObservationID string                      `json:"observationId"`
	ActionID      string                      `json:"actionId"`
	Outcome       decision.ObservationOutcome `json:"outcome"`
	InvocationID  string                      `json:"invocationId,omitempty"`
}

type RecoveryTaskRef struct {
	TaskRunID         string                     `json:"taskRunId"`
	TaskDefinitionID string                     `json:"taskDefinitionId,omitempty"`
	CheckpointID      string                     `json:"checkpointId,omitempty"`
	CheckpointVersion int64                      `json:"checkpointVersion"`
	Generation        int64                      `json:"generation"`
	DefinitionHash    string                     `json:"definitionHash,omitempty"`
	InputHash         string                     `json:"inputHash,omitempty"`
	Status            task_runtime.TaskRunStatus `json:"status"`
}

type RecoveryWorkflowRef struct {
	WorkflowID               string             `json:"workflowId"`
	ExecutionID              string             `json:"executionId"`
	DefinitionHash           string             `json:"definitionHash,omitempty"`
	Generation               int64              `json:"generation"`
	Status                   workflow.RunStatus `json:"status"`
	CompletedCheckpointNodes []string           `json:"completedCheckpointNodes,omitempty"`
}

type RecoveryInvocationRef struct {
	InvocationID string `json:"invocationId"`
	OperationID  string `json:"operationId,omitempty"`
	TraceID      string `json:"traceId,omitempty"`
	ToolID       string `json:"toolId,omitempty"`
	Status       string `json:"status"`
}

type RecoveryPipelineCheckpointRef struct {
	ConversationID      string `json:"conversationId"`
	PipelineType        string `json:"pipelineType,omitempty"`
	CheckpointVersion   int    `json:"checkpointVersion"`
	LastMessageSequence int64  `json:"lastMessageSequence"`
}

type RecoveryMindRef struct {
	ReflectionCandidateID string `json:"reflectionCandidateId,omitempty"`
	ReconciliationScanID  string `json:"reconciliationScanId,omitempty"`
	SnapshotVersion       string `json:"snapshotVersion,omitempty"`
}

type RecoveryDescriptor struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Requirement   RecoveryRequirement             `json:"requirement"`
	Revision      int64                          `json:"revision"`
	Interaction   RecoveryInteractionRef         `json:"interaction"`
	Scope         RecoveryScopeRef               `json:"scope"`
	Goals         []RecoveryGoalRef              `json:"goals,omitempty"`
	Plan          *RecoveryPlanRef               `json:"plan,omitempty"`
	Action        *RecoveryActionRef             `json:"action,omitempty"`
	Observation   *RecoveryObservationRef        `json:"observation,omitempty"`
	Task          *RecoveryTaskRef               `json:"task,omitempty"`
	Workflow      *RecoveryWorkflowRef           `json:"workflow,omitempty"`
	Kernel        *RecoveryInvocationRef         `json:"kernel,omitempty"`
	Pipeline      *RecoveryPipelineCheckpointRef `json:"pipeline,omitempty"`
	Mind          *RecoveryMindRef               `json:"mind,omitempty"`
	State         RecoveryDescriptorState        `json:"state"`
	Fingerprint   string                         `json:"fingerprint"`
	CreatedAt     time.Time                      `json:"createdAt"`
	UpdatedAt     time.Time                      `json:"updatedAt"`
}

func (d *RecoveryDescriptor) Canonicalize() {
	if d == nil {
		return
	}
	sort.Slice(d.Goals, func(i, j int) bool {
		if d.Goals[i].GoalID != d.Goals[j].GoalID {
			return d.Goals[i].GoalID < d.Goals[j].GoalID
		}
		return d.Goals[i].Revision < d.Goals[j].Revision
	})
	if d.Workflow != nil {
		sort.Strings(d.Workflow.CompletedCheckpointNodes)
	}
}

func (d *RecoveryDescriptor) ComputeFingerprint() {
	if d == nil {
		return
	}
	d.Canonicalize()
	h := sha256.New()
	writeKV := func(k, v string) {
		h.Write([]byte(k))
		h.Write([]byte{0x00})
		h.Write([]byte(v))
		h.Write([]byte{0x00})
	}
	writeKVI := func(k string, v int64) {
		writeKV(k, fmt.Sprintf("%d", v))
	}
	writeKVIU := func(k string, v uint64) {
		writeKV(k, fmt.Sprintf("%d", v))
	}
	writeKV("interaction.id", d.Interaction.InteractionID)
	writeKV("interaction.status", string(d.Interaction.Status))
	writeKVI("interaction.statusVersion", d.Interaction.StatusVersion)
	writeKV("interaction.commitId", d.Interaction.CommitID)
	writeKV("scope.userId", d.Scope.UserID)
	writeKV("scope.characterId", d.Scope.CharacterID)
	writeKV("scope.conversationId", d.Scope.ConversationID)
	for _, g := range d.Goals {
		writeKV("goal.id", g.GoalID)
		writeKVI("goal.revision", g.Revision)
		writeKV("goal.status", string(g.Status))
	}
	if d.Plan != nil {
		writeKV("plan.id", d.Plan.PlanID)
		writeKV("plan.candidateId", d.Plan.CandidateID)
	}
	if d.Action != nil {
		writeKV("action.id", d.Action.ActionID)
		writeKV("action.kind", d.Action.Kind)
		writeKV("action.externalCallId", d.Action.ExternalCallID)
	}
	if d.Observation != nil {
		writeKV("obs.id", d.Observation.ObservationID)
		writeKV("obs.actionId", d.Observation.ActionID)
		writeKV("obs.outcome", string(d.Observation.Outcome))
	}
	if d.Task != nil {
		writeKV("task.runId", d.Task.TaskRunID)
		writeKV("task.checkpointId", d.Task.CheckpointID)
		writeKVI("task.checkpointVersion", d.Task.CheckpointVersion)
		writeKVI("task.generation", d.Task.Generation)
		writeKV("task.defHash", d.Task.DefinitionHash)
		writeKV("task.inputHash", d.Task.InputHash)
		writeKV("task.status", string(d.Task.Status))
	}
	if d.Workflow != nil {
		writeKV("workflow.wfId", d.Workflow.WorkflowID)
		writeKV("workflow.execId", d.Workflow.ExecutionID)
		writeKVI("workflow.generation", d.Workflow.Generation)
		writeKV("workflow.status", string(d.Workflow.Status))
		writeKVIU("workflow.completedCount", uint64(len(d.Workflow.CompletedCheckpointNodes)))
		for _, n := range d.Workflow.CompletedCheckpointNodes {
			writeKV("workflow.completedNode", n)
		}
	}
	if d.Kernel != nil {
		writeKV("kernel.invocationId", d.Kernel.InvocationID)
		writeKV("kernel.toolId", d.Kernel.ToolID)
		writeKV("kernel.status", d.Kernel.Status)
	}
	if d.Pipeline != nil {
		writeKV("pipeline.conversationId", d.Pipeline.ConversationID)
		writeKVI("pipeline.lastSeq", d.Pipeline.LastMessageSequence)
		writeKVIU("pipeline.version", uint64(d.Pipeline.CheckpointVersion))
	}
	if d.Mind != nil {
		writeKV("mind.reflectionId", d.Mind.ReflectionCandidateID)
		writeKV("mind.scanId", d.Mind.ReconciliationScanID)
		writeKV("mind.snapshotVersion", d.Mind.SnapshotVersion)
	}
	writeKV("state", string(d.State))
	d.Fingerprint = "fp:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func (d *RecoveryDescriptor) NormalizeOnSerialize() ([]byte, error) {
	if d == nil {
		return nil, nil
	}
	d.Canonicalize()
	if d.Fingerprint == "" {
		d.ComputeFingerprint()
	}
	if d.SchemaVersion == 0 {
		d.SchemaVersion = RecoveryDescriptorSchemaVersion
	}
	type alias RecoveryDescriptor
	data, err := json.Marshal((*alias)(d))
	if err != nil {
		return nil, err
	}
	if len(data) > RecoveryDescriptorMaxSizeBytes {
		return nil, fmt.Errorf("recovery_descriptor_too_large: %d bytes", len(data))
	}
	return data, nil
}

func DescriptorFromJSON(data []byte) (*RecoveryDescriptor, error) {
	var d RecoveryDescriptor
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	if d.SchemaVersion != 0 && d.SchemaVersion != RecoveryDescriptorSchemaVersion {
		return nil, fmt.Errorf("descriptor_version_unsupported: schema=%d current=%d", d.SchemaVersion, RecoveryDescriptorSchemaVersion)
	}
	return &d, nil
}
