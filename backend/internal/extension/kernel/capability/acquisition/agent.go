package acquisition

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

// AgentCapabilityBridge serves as a bridge between Agent tool calls and the
// AcquisitionService. It translates tool-level input/output structs into
// AcquisitionService requests and results.
type AgentCapabilityBridge struct {
	acquisitionService *AcquisitionService
}

// NewAgentCapabilityBridge creates an AgentCapabilityBridge wired to the given
// AcquisitionService.
func NewAgentCapabilityBridge(service *AcquisitionService) *AgentCapabilityBridge {
	return &AgentCapabilityBridge{
		acquisitionService: service,
	}
}

// FindCapabilities translates a FindCapabilitiesInput into an AcquisitionRequest,
// invokes the AcquisitionService.FindCapabilities, and returns a
// FindCapabilitiesOutput.
func (b *AgentCapabilityBridge) FindCapabilities(ctx context.Context, input FindCapabilitiesInput, userID string) (*FindCapabilitiesOutput, error) {
	if input.CapabilityID == "" {
		return nil, NewAcquisitionError("invalid_input", "capabilityId is required", nil)
	}

	request := AcquisitionRequest{
		CapabilityID: capability.CapabilityID(input.CapabilityID),
		UserID:       runtimeidentity.UserID(userID),
		Description:  input.Description,
	}

	// Apply preferred kind filter if specified.
	if input.PreferredKind != "" {
		request.PreferredKinds = []CandidateKind{CandidateKind(input.PreferredKind)}
	}

	startTime := time.Now()
	resultSet, err := b.acquisitionService.FindCapabilities(ctx, request)
	elapsed := time.Since(startTime).Milliseconds()

	if err != nil {
		return nil, err
	}

	output := &FindCapabilitiesOutput{
		Candidates:   resultSet.Candidates,
		TotalFound:   len(resultSet.Candidates),
		SearchTimeMs: elapsed,
	}

	return output, nil
}

// AcquireCapability translates an AcquireInput into an AcquisitionRequest,
// invokes the AcquisitionService.Acquire, and returns an AcquireOutput.
func (b *AgentCapabilityBridge) AcquireCapability(ctx context.Context, input AcquireInput, userID string) (*AcquireOutput, error) {
	if input.CapabilityID == "" {
		return nil, NewAcquisitionError("invalid_input", "capabilityId is required", nil)
	}

	request := AcquisitionRequest{
		CapabilityID:        capability.CapabilityID(input.CapabilityID),
		RequestedCandidateID: input.CandidateID,
		UserID:              runtimeidentity.UserID(userID),
	}

	yes := input.Approval || input.UserConfirmed

	result, err := b.acquisitionService.Acquire(ctx, request, yes)
	if err != nil {
		// If approval is required, return a structured output rather than a
		// raw error so the agent can surface the approval request to the user.
		if err == ErrApprovalRequired {
			return &AcquireOutput{
				Success:       false,
				State:         StateAwaitingApproval,
				NeedsApproval: true,
				ResumeToken:   result.ResumeToken,
				ErrorMessage:  "user approval required before installation",
			}, nil
		}

		return &AcquireOutput{
			Success:      false,
			State:        StateFailed,
			ErrorMessage: err.Error(),
		}, err
	}

	output := &AcquireOutput{
		Success: result.IsReady(),
		State:   result.State,
	}

	if len(result.CapabilityIDs) > 0 {
		output.CapabilityID = string(result.CapabilityIDs[0])
	}

	if result.IsReady() {
		output.InstalledAt = result.UpdatedAt.Format(time.RFC3339)
	}

	if result.Error != "" {
		output.ErrorMessage = result.Error
	}

	return output, nil
}
