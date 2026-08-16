package uiagent

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/uiagent/preview"
)

type UIOperation struct {
	Type      string          `json:"type"`
	Target    string          `json:"target"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	DependsOn []string        `json:"dependsOn,omitempty"`
}

type UIChangePlan struct {
	RootExecutionID string `json:"rootExecutionId,omitempty"`
	ExecutionID     string `json:"executionId,omitempty"`
	TraceID         string `json:"traceId,omitempty"`

	ExecContext *execution.ExecutionContext `json:"-"`

	Intent              UIIntent               `json:"intent"`
	Mode                UITargetType           `json:"mode"`
	RequiredCapabilities []capability.CapabilityID `json:"requiredCapabilities,omitempty"`
	TargetRuntime       *DeploymentTarget       `json:"targetRuntime,omitempty"`
	Operations          []UIOperation          `json:"operations"`
	Risk                UIRisk                 `json:"risk"`
	PreviewStrategy     PreviewStrategy        `json:"previewStrategy"`
	RollbackStrategy    RollbackStrategy       `json:"rollbackStrategy"`
}

func (p UIChangePlan) NeedsCapability(capID capability.CapabilityID) bool {
	for _, c := range p.RequiredCapabilities {
		if c == capID {
			return true
		}
	}
	return false
}

type UIResult struct {
	State           string                `json:"state"`
	ChangedFiles    []string              `json:"changedFiles,omitempty"`
	ContributionIDs []string              `json:"contributionIds,omitempty"`
	PreviewRefs     []string              `json:"previewRefs,omitempty"`
	DiffSummary     string                `json:"diffSummary,omitempty"`
	RollbackToken   string                `json:"rollbackToken,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
	PreviewState    string                `json:"previewState,omitempty"`
	RefineResult    *preview.RefineResult `json:"refineResult,omitempty"`
	ObserveIssues   []string              `json:"observeIssues,omitempty"`
}
