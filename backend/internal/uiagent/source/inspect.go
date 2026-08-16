package source

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/uiagent"
	"github.com/u-ai/backend/internal/workspace"
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

type defaultSourceInspector struct {
	workspaceSvc *workspace.Service
}

func NewSourceInspector() SourceInspector {
	return &defaultSourceInspector{}
}

func NewSourceInspectorWithWorkspace(svc *workspace.Service) SourceInspector {
	return &defaultSourceInspector{workspaceSvc: svc}
}

func (s *defaultSourceInspector) Inspect(ctx context.Context, workspaceID string, paths []string) (*InspectionResult, error) {
	if workspaceID == "" {
		return nil, ErrWorkspaceIDRequired
	}

	result := &InspectionResult{
		WorkspaceID: workspaceID,
		Editable:    false,
		UILibraries: []string{},
	}

	if s.workspaceSvc == nil {
		return result, nil
	}

	uri := "ws://" + workspaceID + "/"
	listOpts := workspace.ListOptions{}
	listResult, err := s.workspaceSvc.List(ctx, uri, listOpts)
	if err != nil {
		return result, nil
	}

	result.TotalFiles = len(listResult.Entries)
	result.Editable = true

	uiExts := map[string]bool{".vue": true, ".tsx": true, ".jsx": true, ".svelte": true, ".html": true, ".dart": true}
	libHints := map[string]bool{"vue": true, "react": true, "svelte": true, "flutter": true, "angular": true}

	for _, entry := range listResult.Entries {
		if entry.Type == workspace.WorkspaceEntryTypeDirectory {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name))
		if uiExts[ext] {
			result.UIFilePaths = append(result.UIFilePaths, entry.Name)
		}
		lower := strings.ToLower(entry.Name)
		if strings.Contains(lower, "router") || strings.Contains(lower, "route") {
			result.RouteHints = append(result.RouteHints, entry.Name)
		}
		if strings.Contains(lower, "main") || strings.Contains(lower, "app") || strings.Contains(lower, "index") {
			result.HasEntrypoint = true
		}
		for lib := range libHints {
			if strings.Contains(lower, lib) {
				result.UILibraries = append(result.UILibraries, lib)
			}
		}
	}

	if len(result.UIFilePaths) > 0 {
		result.Framework = detectFramework(result.UIFilePaths, result.UILibraries)
	}

	return result, nil
}

func detectFramework(uiFiles []string, libs []string) string {
	for _, lib := range libs {
		switch lib {
		case "vue":
			return "vue"
		case "react":
			return "react"
		case "svelte":
			return "svelte"
		case "flutter":
			return "flutter"
		case "angular":
			return "angular"
		}
	}
	for _, f := range uiFiles {
		ext := strings.ToLower(filepath.Ext(f))
		switch ext {
		case ".vue":
			return "vue"
		case ".tsx", ".jsx":
			return "react"
		case ".svelte":
			return "svelte"
		case ".dart":
			return "flutter"
		}
	}
	return ""
}

var ErrWorkspaceIDRequired = errors.New("source inspector: workspace ID is required")

// refUITargetSource ensures the uiagent package import remains meaningful
// as the source package grows to depend on shared uiagent types.
var _ uiagent.UITargetType = uiagent.UITargetSource
