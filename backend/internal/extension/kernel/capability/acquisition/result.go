package acquisition

import (
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type AcquisitionResult struct {
	State AcquisitionState `json:"state"`

	TransactionID string `json:"transactionId,omitempty"`
	CandidateID   string `json:"candidateId,omitempty"`

	ExecutionID string `json:"executionId,omitempty"`
	ResumeID    string `json:"resumeId,omitempty"`

	Installed bool `json:"installed"`
	Enabled   bool `json:"enabled"`

	CapabilityIDs      []capability.CapabilityID      `json:"capabilityIds,omitempty"`
	ProviderIDs        []capability.ProviderID        `json:"providerIds,omitempty"`
	ProviderInstanceIDs []capability.ProviderInstanceID `json:"providerInstanceIds,omitempty"`

	Target DeploymentTarget `json:"target"`

	ResumeToken string   `json:"resumeToken,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Error       string   `json:"error,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (r AcquisitionResult) IsReady() bool {
	return r.State == StateReady
}

func (r AcquisitionResult) IsFailed() bool {
	return r.State == StateFailed || r.State == StateRolledBack
}

func (r AcquisitionResult) HasCapability(capID capability.CapabilityID) bool {
	for _, id := range r.CapabilityIDs {
		if id == capID {
			return true
		}
	}
	return false
}
