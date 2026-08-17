package source

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/uiagent"
	"github.com/u-ai/backend/internal/workspace"
)

// InspectionResult 描述一个 source target 的检查结果
type InspectionResult struct {
	WorkspaceID   string              `json:"workspaceId"`
	TotalFiles    int                 `json:"totalFiles"`
	HasEntrypoint bool                `json:"hasEntrypoint"`
	UIFilePaths   []string            `json:"uiFilePaths"`
	RouteHints    []string            `json:"routeHints,omitempty"`
	UILibraries   []string            `json:"uiLibraries,omitempty"`
	Editable      bool                `json:"editable"`
	Framework     string              `json:"framework,omitempty"`
	Symbols       []SymbolInfo        `json:"symbols,omitempty"`
	FileHashes    map[string]string   `json:"fileHashes,omitempty"`
}

// SymbolInfo describes a symbol/component found in the source code.
type SymbolInfo struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	FilePath  string `json:"filePath"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
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
		Symbols:     []SymbolInfo{},
		FileHashes:  map[string]string{},
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
			fileURI := "ws://" + workspaceID + "/" + entry.Name
			readResult, err := s.workspaceSvc.Read(ctx, fileURI, workspace.ReadOptions{})
			if err == nil && len(readResult.Content) > 0 {
				hash := fmt.Sprintf("%x", sha256.Sum256(readResult.Content))
				result.FileHashes[entry.Name] = hash
				symbols := extractSymbols(entry.Name, string(readResult.Content))
				result.Symbols = append(result.Symbols, symbols...)
			}
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

// extractSymbols extracts component/symbol definitions from source code.
func extractSymbols(filePath, content string) []SymbolInfo {
	var symbols []SymbolInfo
	lines := strings.Split(content, "\n")
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".vue":
		symbols = append(symbols, extractVueSymbols(filePath, lines)...)
	case ".tsx", ".jsx":
		symbols = append(symbols, extractReactSymbols(filePath, lines)...)
	case ".dart":
		symbols = append(symbols, extractFlutterSymbols(filePath, lines)...)
	case ".svelte":
		symbols = append(symbols, extractSvelteSymbols(filePath, lines)...)
	}
	return symbols
}

var (
	vueComponentRe   = regexp.MustCompile(`export\s+default\s+(?:class\s+)?(\w+)`)
	reactCompRe      = regexp.MustCompile(`(?:function|const|class)\s+([A-Z]\w*)\s*(?:\(|=|extends|{)`)
	reactExportRe    = regexp.MustCompile(`export\s+(?:default\s+)?(?:function|const)\s+([A-Z]\w*)`)
	flutterClassRe   = regexp.MustCompile(`class\s+(\w+)\s+extends\s+(?:StatefulWidget|StatelessWidget)`)
	svelteCompRe     = regexp.MustCompile(`export\s+(?:let|const|function)\s+(\w+)`)
)

func extractVueSymbols(filePath string, lines []string) []SymbolInfo {
	var symbols []SymbolInfo
	for i, line := range lines {
		if matches := vueComponentRe.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, SymbolInfo{
				Name:      matches[1],
				Kind:      "component",
				FilePath:  filePath,
				LineStart: i + 1,
				LineEnd:   i + 1,
			})
		}
	}
	return symbols
}

func extractReactSymbols(filePath string, lines []string) []SymbolInfo {
	var symbols []SymbolInfo
	for i, line := range lines {
		if matches := reactExportRe.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, SymbolInfo{
				Name:      matches[1],
				Kind:      "component",
				FilePath:  filePath,
				LineStart: i + 1,
				LineEnd:   i + 1,
			})
			continue
		}
		if matches := reactCompRe.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, SymbolInfo{
				Name:      matches[1],
				Kind:      "component",
				FilePath:  filePath,
				LineStart: i + 1,
				LineEnd:   i + 1,
			})
		}
	}
	return symbols
}

func extractFlutterSymbols(filePath string, lines []string) []SymbolInfo {
	var symbols []SymbolInfo
	for i, line := range lines {
		if matches := flutterClassRe.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, SymbolInfo{
				Name:      matches[1],
				Kind:      "widget",
				FilePath:  filePath,
				LineStart: i + 1,
				LineEnd:   i + 1,
			})
		}
	}
	return symbols
}

func extractSvelteSymbols(filePath string, lines []string) []SymbolInfo {
	var symbols []SymbolInfo
	for i, line := range lines {
		if matches := svelteCompRe.FindStringSubmatch(line); matches != nil {
			symbols = append(symbols, SymbolInfo{
				Name:      matches[1],
				Kind:      "component",
				FilePath:  filePath,
				LineStart: i + 1,
				LineEnd:   i + 1,
			})
		}
	}
	return symbols
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
