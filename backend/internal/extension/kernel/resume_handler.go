package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
)

// CapabilityAcquisitionResumeHandler handles resumption of capability acquisitions
// that were suspended awaiting user approval. It bridges the high-level
// ExecutionService resume mechanism with the AcquisitionService.
type CapabilityAcquisitionResumeHandler struct {
	acquisitionService *acquisition.AcquisitionService
}

// NewCapabilityAcquisitionResumeHandler creates a new resume handler.
func NewCapabilityAcquisitionResumeHandler(acqSvc *acquisition.AcquisitionService) *CapabilityAcquisitionResumeHandler {
	return &CapabilityAcquisitionResumeHandler{acquisitionService: acqSvc}
}

// CanHandle returns true for capability acquisition resume types.
func (h *CapabilityAcquisitionResumeHandler) CanHandle(resumeType execution.ResumeType) bool {
	return resumeType == execution.ResumeTypeCapabilityAcquisition
}

// ResumeExecution continues a suspended acquisition after user approval.
func (h *CapabilityAcquisitionResumeHandler) ResumeExecution(
	ctx context.Context,
	resume execution.ResumeContext,
	execCtx execution.ExecutionContext,
) (*execution.ExecutionContext, error) {
	if h.acquisitionService == nil {
		return nil, fmt.Errorf("capability acquisition resume handler: acquisition service not configured")
	}

	acquisitionResumeToken := resume.AcquisitionTransactionID
	if acquisitionResumeToken == "" {
		return nil, fmt.Errorf("capability acquisition resume handler: no acquisition resume token in context")
	}

	result, err := h.acquisitionService.ResumeAcquire(ctx, acquisitionResumeToken)
	if err != nil {
		return nil, fmt.Errorf("resume capability acquisition: %w", err)
	}

	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]any)
	}

	execCtx.Metadata["acquisition_state"] = string(result.State)
	execCtx.Metadata["acquisition_transaction_id"] = result.TransactionID
	if result.ResumeToken != "" {
		execCtx.Metadata["acquisition_resume_token"] = result.ResumeToken
	}

	return &execCtx, nil
}
