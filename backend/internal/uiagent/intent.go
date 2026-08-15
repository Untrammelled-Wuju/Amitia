package uiagent

import (
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type UIIntent struct {
	RootExecutionID string `json:"rootExecutionId,omitempty"`
	ExecutionID     string `json:"executionId,omitempty"`
	TraceID         string `json:"traceId,omitempty"`

	ExecContext *execution.ExecutionContext `json:"-"`

	Action          UIActionType `json:"action"`
	Description     string       `json:"description"`
	Target          UITarget     `json:"target"`
	Constraints     UIConstraints `json:"constraints"`
	DesiredOutcome  string       `json:"desiredOutcome,omitempty"`
}

type UITarget struct {
	Type UITargetType `json:"type"`

	WorkspaceID string `json:"workspaceId,omitempty"`

	ExtensionID    string `json:"extensionId,omitempty"`
	ContributionID string `json:"contributionId,omitempty"`

	Route         string `json:"route,omitempty"`
	PageID        string `json:"pageId,omitempty"`
	ComponentHint string `json:"componentHint,omitempty"`

	Platform string `json:"platform,omitempty"`

	RuntimeTarget *runtimeidentity.Identity `json:"runtimeTarget,omitempty"`
}

type UIConstraints struct {
	PreserveBehavior  bool     `json:"preserveBehavior"`
	PreservePublicAPI bool     `json:"preservePublicAPI"`
	Responsive        bool     `json:"responsive"`
	AllowedFiles      []string `json:"allowedFiles,omitempty"`
	DeniedFiles       []string `json:"deniedFiles,omitempty"`
	DesignLanguage    string   `json:"designLanguage,omitempty"`
	MaxFilesChanged   int      `json:"maxFilesChanged"`
	RequirePreview    bool     `json:"requirePreview"`
}

func (c UIConstraints) IsPathAllowed(path string) bool {
	for _, denied := range c.DeniedFiles {
		if denied == path {
			return false
		}
	}
	if len(c.AllowedFiles) == 0 {
		return true
	}
	for _, allowed := range c.AllowedFiles {
		if allowed == path {
			return true
		}
	}
	return false
}
