package execution

import (
	"time"

	"github.com/google/uuid"
)

type ResumeType string

const (
	ResumeTypeAgent              ResumeType = "agent"
	ResumeTypeCapabilityAcquisition ResumeType = "capability_acquisition"
	ResumeTypeApproval           ResumeType = "approval"
	ResumeTypeTask               ResumeType = "task"
	ResumeTypeUISource           ResumeType = "ui_source"
	ResumeTypeUISchema           ResumeType = "ui_schema"
	ResumeTypeRemoteRuntime      ResumeType = "remote_runtime"
)

type ResumeState string

const (
	ResumeStatePending    ResumeState = "pending"
	ResumeStateInProgress ResumeState = "in_progress"
	ResumeStateCompleted  ResumeState = "completed"
	ResumeStateFailed     ResumeState = "failed"
	ResumeStateCancelled  ResumeState = "cancelled"
)

type ResumeContext struct {
	ResumeID string `json:"resumeId"`

	RootExecutionID   string `json:"rootExecutionId,omitempty"`
	ParentExecutionID string `json:"parentExecutionID,omitempty"`

	Type  ResumeType  `json:"type"`
	State ResumeState `json:"state"`

	CheckpointRef string `json:"checkpointRef,omitempty"`

	RequiredCapabilityID string `json:"requiredCapabilityId,omitempty"`

	AcquisitionTransactionID string `json:"acquisitionTransactionId,omitempty"`
	TaskID                  string `json:"taskId,omitempty"`

	PayloadRef string `json:"payloadRef,omitempty"`

	Reason string `json:"reason,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewResumeContext(execCtx ExecutionContext, resumeType ResumeType) ResumeContext {
	now := time.Now().UTC()
	return ResumeContext{
		ResumeID:        "resume_" + uuid.NewString(),
		RootExecutionID: execCtx.RootExecutionID,
		Type:            resumeType,
		State:           ResumeStatePending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (r *ResumeContext) MarkInProgress() {
	r.State = ResumeStateInProgress
	r.UpdatedAt = time.Now().UTC()
}

func (r *ResumeContext) MarkCompleted() {
	r.State = ResumeStateCompleted
	r.UpdatedAt = time.Now().UTC()
}

func (r *ResumeContext) MarkFailed(reason string) {
	r.State = ResumeStateFailed
	r.Reason = reason
	r.UpdatedAt = time.Now().UTC()
}

func (r ResumeContext) IsActive() bool {
	return r.State == ResumeStatePending || r.State == ResumeStateInProgress
}
