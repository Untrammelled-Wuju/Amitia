package source

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/uiagent"
)

// InspectionResult 描述一个 source target 的检查结果
type InspectionResult struct {
	WorkspaceID   string   `json:"workspaceId"`
	TotalFiles    int      `json:"totalFiles"`
	HasEntrypoint bool     `json:"hasEntrypoint"`
	UIFilePaths   []string `json:"uiFilePaths"`
	RouteHints    []string `json:"routeHints,omitempty"`
	UILibraries   []string `json:"uiLibraries,omitempty"`
	Editable      bool     `json:"editable"`
	Framework     string   `json:"framework,omitempty"`
}

type SourceInspector interface {
	Inspect(ctx context.Context, workspaceID string, paths []string) (*InspectionResult, error)
}

type defaultSourceInspector struct{}

func NewSourceInspector() SourceInspector {
	return &defaultSourceInspector{}
}

func (s *defaultSourceInspector) Inspect(ctx context.Context, workspaceID string, paths []string) (*InspectionResult, error) {
	if workspaceID == "" {
		return nil, ErrWorkspaceIDRequired
	}
	return &InspectionResult{
		WorkspaceID: workspaceID,
		Editable:    true,
		UILibraries: []string{},
	}, nil
}

var ErrWorkspaceIDRequired = errors.New("source inspector: workspace ID is required")

// refUITargetSource ensures the uiagent package import remains meaningful
// as the source package grows to depend on shared uiagent types.
var _ uiagent.UITargetType = uiagent.UITargetSource
