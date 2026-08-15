package acquisition

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type CapabilityResumeContext struct {
	ConversationID        string `json:"conversationId,omitempty"`
	TaskID                string `json:"taskId,omitempty"`
	ParentInvocationID    string `json:"parentInvocationId,omitempty"`
	OriginalUserMessageID string `json:"originalUserMessageId,omitempty"`

	FailedToolCall *ToolCallSnapshot `json:"failedToolCall,omitempty"`

	CapabilityID capability.CapabilityID `json:"capabilityId"`

	AcquisitionTransactionID string `json:"acquisitionTransactionId,omitempty"`

	State ResumeState `json:"state"`
}

type ToolCallSnapshot struct {
	ToolName      string         `json:"toolName"`
	ToolID        string         `json:"toolId,omitempty"`
	Input         map[string]any `json:"input,omitempty"`
	RequirementID string         `json:"requirementId,omitempty"`
}

func (c CapabilityResumeContext) HasFailedToolCall() bool {
	return c.FailedToolCall != nil
}

type CapabilityRecoveryBudget struct {
	MaxAcquisitionsPerTask int `json:"maxAcquisitionsPerTask"`
	MaxAttemptsPerCapability int `json:"maxAttemptsPerCapability"`
	CurrentAcquisitions    int `json:"currentAcquisitions"`
	AttemptedCapabilities  map[string]int `json:"attemptedCapabilities"`
}

func NewCapabilityRecoveryBudget() CapabilityRecoveryBudget {
	return CapabilityRecoveryBudget{
		MaxAcquisitionsPerTask:  3,
		MaxAttemptsPerCapability: 1,
		AttemptedCapabilities:   make(map[string]int),
	}
}

func (b *CapabilityRecoveryBudget) CanAttempt(acqID string) bool {
	if b.CurrentAcquisitions >= b.MaxAcquisitionsPerTask {
		return false
	}
	if count, ok := b.AttemptedCapabilities[acqID]; ok && count >= b.MaxAttemptsPerCapability {
		return false
	}
	return true
}

func (b *CapabilityRecoveryBudget) RecordAttempt(acqID string) {
	b.CurrentAcquisitions++
	b.AttemptedCapabilities[acqID]++
}
