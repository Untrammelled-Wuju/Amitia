package kernel

import (
	"context"

	"github.com/u-ai/backend/internal/execution"
)

// UISourceResumeHandler handles resumption of UI source edit operations
// that were suspended awaiting user action or approval.
type UISourceResumeHandler struct{}

// NewUISourceResumeHandler creates a new UI source resume handler.
func NewUISourceResumeHandler() *UISourceResumeHandler {
	return &UISourceResumeHandler{}
}

func (h *UISourceResumeHandler) CanHandle(resumeType execution.ResumeType) bool {
	return resumeType == execution.ResumeTypeUISource
}

func (h *UISourceResumeHandler) ResumeExecution(
	ctx context.Context,
	resume execution.ResumeContext,
	execCtx execution.ExecutionContext,
) (*execution.ExecutionContext, error) {
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]any)
	}
	execCtx.Metadata["ui_resume_type"] = string(resume.Type)
	execCtx.Metadata["ui_resume_completed"] = true
	return &execCtx, nil
}

// UISchemaResumeHandler handles resumption of UI schema generation operations
// that were suspended (e.g., awaiting model availability).
type UISchemaResumeHandler struct{}

// NewUISchemaResumeHandler creates a new UI schema resume handler.
func NewUISchemaResumeHandler() *UISchemaResumeHandler {
	return &UISchemaResumeHandler{}
}

func (h *UISchemaResumeHandler) CanHandle(resumeType execution.ResumeType) bool {
	return resumeType == execution.ResumeTypeUISchema
}

func (h *UISchemaResumeHandler) ResumeExecution(
	ctx context.Context,
	resume execution.ResumeContext,
	execCtx execution.ExecutionContext,
) (*execution.ExecutionContext, error) {
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]any)
	}
	execCtx.Metadata["ui_schema_resume_type"] = string(resume.Type)
	execCtx.Metadata["ui_schema_resume_completed"] = true
	return &execCtx, nil
}

// ApprovalResumeHandler handles resumption of operations awaiting user approval
// (generic approval flow not tied to a specific subsystem).
type ApprovalResumeHandler struct{}

// NewApprovalResumeHandler creates a new approval resume handler.
func NewApprovalResumeHandler() *ApprovalResumeHandler {
	return &ApprovalResumeHandler{}
}

func (h *ApprovalResumeHandler) CanHandle(resumeType execution.ResumeType) bool {
	return resumeType == execution.ResumeTypeApproval
}

func (h *ApprovalResumeHandler) ResumeExecution(
	ctx context.Context,
	resume execution.ResumeContext,
	execCtx execution.ExecutionContext,
) (*execution.ExecutionContext, error) {
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]any)
	}
	execCtx.Metadata["approval_resume_type"] = string(resume.Type)
	execCtx.Metadata["approval_consumed"] = true
	if resume.PayloadRef != "" {
		execCtx.Metadata["approval_payload_ref"] = resume.PayloadRef
	}
	return &execCtx, nil
}

// Ensure handlers implement the ResumeHandler interface.
var (
	_ execution.ResumeHandler = (*CapabilityAcquisitionResumeHandler)(nil)
	_ execution.ResumeHandler = (*UISourceResumeHandler)(nil)
	_ execution.ResumeHandler = (*UISchemaResumeHandler)(nil)
	_ execution.ResumeHandler = (*ApprovalResumeHandler)(nil)
)
