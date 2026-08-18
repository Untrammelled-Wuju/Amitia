package adapters

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/workspace"
)

type WorkspaceResolver interface {
	ResolvePath(workspaceID string, filePath string) (string, error)
}

type flutterAnalyzer struct {
	workspaceResolver WorkspaceResolver
	workspaceService  *workspace.Service
}

func NewFlutterAnalyzer(ws *workspace.Service) FlutterAnalyzer {
	return &flutterAnalyzer{workspaceService: ws}
}

func (a *flutterAnalyzer) Analyze(ctx context.Context, workspaceID string, paths []string) ([]string, []string, error) {
	wsPath, err := a.getWorkspacePath(ctx, workspaceID)
	if err != nil {
		return []string{fmt.Sprintf("resolve workspace path: %v", err)}, nil, nil
	}

	if len(paths) > 0 {
		for _, p := range paths {
			fullPath := filepath.Join(wsPath, p)
			if _, err := exec.LookPath("flutter"); err == nil {
				cmd := exec.CommandContext(ctx, "flutter", "analyze", "--no-fatal-infos", "--no-fatal-warnings", fullPath)
				cmd.Dir = wsPath
				output, err := a.runCommand(cmd)
				if err != nil {
					return []string{fmt.Sprintf("flutter analyze %s: %v", p, err)}, output, nil
				}
			} else if _, err := exec.LookPath("dart"); err == nil {
				cmd := exec.CommandContext(ctx, "dart", "analyze", "--fatal-infos", fullPath)
				cmd.Dir = wsPath
				output, err := a.runCommand(cmd)
				if err != nil {
					return []string{fmt.Sprintf("dart analyze %s: %v", p, err)}, output, nil
				}
			}
		}
		return nil, nil, nil
	}

	if _, err := exec.LookPath("flutter"); err == nil {
		cmd := exec.CommandContext(ctx, "flutter", "analyze", "--no-fatal-infos", "--no-fatal-warnings")
		cmd.Dir = wsPath
		output, err := a.runCommand(cmd)
		if err != nil {
			return []string{fmt.Sprintf("flutter analyze: %v", err)}, output, nil
		}
	} else if _, err := exec.LookPath("dart"); err == nil {
		cmd := exec.CommandContext(ctx, "dart", "analyze", "--fatal-infos")
		cmd.Dir = wsPath
		output, err := a.runCommand(cmd)
		if err != nil {
			return []string{fmt.Sprintf("dart analyze: %v", err)}, output, nil
		}
	}

	return nil, []string{"flutter/dart not available, skipping analyze"}, nil
}

func (a *flutterAnalyzer) runCommand(cmd *exec.Cmd) ([]string, error) {
	output, err := cmd.CombinedOutput()
	if err != nil {
		return parseDiagnosticOutput(string(output)), err
	}
	return parseDiagnosticOutput(string(output)), nil
}

func (a *flutterAnalyzer) getWorkspacePath(ctx context.Context, workspaceID string) (string, error) {
	if a.workspaceResolver != nil {
		return a.workspaceResolver.ResolvePath(workspaceID, "")
	}
	return "/tmp/workspace/" + workspaceID, nil
}

type webAnalyzer struct {
	workspaceResolver WorkspaceResolver
	workspaceService  *workspace.Service
}

func NewWebAnalyzer(ws *workspace.Service) WebAnalyzer {
	return &webAnalyzer{workspaceService: ws}
}

func (a *webAnalyzer) TypeCheck(ctx context.Context, workspaceID string, paths []string) ([]string, []string, error) {
	wsPath, err := a.getWorkspacePath(ctx, workspaceID)
	if err != nil {
		return []string{fmt.Sprintf("resolve workspace path: %v", err)}, nil, nil
	}

	if _, err := exec.LookPath("tsc"); err == nil {
		cmd := exec.CommandContext(ctx, "tsc", "--noEmit", "--skipLibCheck")
		cmd.Dir = wsPath
		output, err := a.runCommand(cmd)
		if err != nil {
			return []string{fmt.Sprintf("tsc: %v", err)}, output, nil
		}
	} else if _, err := exec.LookPath("npx"); err == nil {
		cmd := exec.CommandContext(ctx, "npx", "tsc", "--noEmit", "--skipLibCheck")
		cmd.Dir = wsPath
		output, err := a.runCommand(cmd)
		if err != nil {
			return []string{fmt.Sprintf("npx tsc: %v", err)}, output, nil
		}
	}

	return nil, []string{"tsc not available, skipping typecheck"}, nil
}

func (a *webAnalyzer) runCommand(cmd *exec.Cmd) ([]string, error) {
	output, err := cmd.CombinedOutput()
	if err != nil {
		return parseDiagnosticOutput(string(output)), err
	}
	return parseDiagnosticOutput(string(output)), nil
}

func (a *webAnalyzer) getWorkspacePath(ctx context.Context, workspaceID string) (string, error) {
	if a.workspaceResolver != nil {
		return a.workspaceResolver.ResolvePath(workspaceID, "")
	}
	return "/tmp/workspace/" + workspaceID, nil
}

func parseDiagnosticOutput(output string) []string {
	var results []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			results = append(results, line)
		}
	}
	if len(results) > 50 {
		results = results[:50]
		results = append(results, "... (truncated)")
	}
	return results
}

var _ = time.Now
