// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
)

type ScriptRuntimeDeps struct {
	InterpreterResolver ScriptInterpreterResolver
	ProcessSupervisor   runtimehost.ProcessSupervisor
	FileInspector       ScriptFileInspector
	TempDirResolver     func(ctx context.Context, skillName string) (string, error)
}

type ScriptRuntime struct {
	deps          ScriptRuntimeDeps
	discovery     *scriptDiscovery
	executor      *scriptExecutor
	policyContext ScriptPolicyContext
}

type ScriptExecutionRequest struct {
	ExtensionID      string
	SkillName        string
	RelativePath      string
	Content          []byte
	ExpectedHash     string
	Args             map[string]any
	Inputs           map[string]string
	Timeout          time.Duration
	WorkingDirPolicy string
	OutputMode       string
	DeclaredArgs     []SkillScriptArgSpec
	Permissions      []string
}

func NewScriptRuntime(deps ScriptRuntimeDeps) *ScriptRuntime {
	if deps.FileInspector == nil {
		deps.FileInspector = defaultScriptFileInspector{}
	}
	return &ScriptRuntime{
		deps: deps,
		discovery: NewScriptDiscovery(ScriptDiscoveryContext{
			SkillRootResolver: func(ctx context.Context, extensionID string) (string, error) {
				return "", fmt.Errorf("skill root resolver not configured")
			},
			FileInspector: deps.FileInspector,
		}),
		executor: NewScriptExecutor(ScriptExecutorContext{
			InterpreterResolver: deps.InterpreterResolver,
			ProcessSupervisor:   deps.ProcessSupervisor,
			TempDirResolver:     deps.TempDirResolver,
		}),
		policyContext: DefaultScriptPolicyContext(),
	}
}

func (r *ScriptRuntime) Execute(ctx context.Context, req ScriptExecutionRequest) (*SkillScriptResult, error) {
	if len(req.Content) == 0 {
		return nil, ErrScriptInvalidDescriptor
	}

	fileHash := ComputeFileHash(req.Content)
	if req.ExpectedHash != "" && !equalHashes(fileHash, req.ExpectedHash) {
		return nil, ErrScriptHashMismatch
	}

	skillRoot, err := r.prepareScriptFile(ctx, req.RelativePath, req.Content)
	if err != nil {
		return nil, err
	}

	desc := SkillScriptDescriptor{
		ExtensionID:      req.ExtensionID,
		SkillName:        req.SkillName,
		RelativePath:     req.RelativePath,
		FileHash:         fileHash,
		Runtime:          ScriptRuntimeNode,
		Kind:             ScriptKindExec,
		EntryName:        filepath.Base(req.RelativePath),
		DeclaredArgs:     req.DeclaredArgs,
		Permissions:      req.Permissions,
		Timeout:          req.Timeout,
		WorkingDirPolicy: req.WorkingDirPolicy,
		OutputMode:       req.OutputMode,
	}

	plan, err := r.executor.BuildPlan(ctx, desc, req.Args, req.Inputs)
	if err != nil {
		return nil, err
	}

	workingDir, err := r.executor.ResolveWorkingDir(plan, skillRoot)
	if err != nil {
		return nil, err
	}

	if err := r.executor.SetWorkingDir(plan, workingDir); err != nil {
		return nil, err
	}

	executionID := fmt.Sprintf("script-%d", time.Now().UnixNano())

	result, err := r.executor.Execute(ctx, plan, executionID)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *ScriptRuntime) prepareScriptFile(ctx context.Context, relPath string, content []byte) (string, error) {
	cleanRel := filepath.Clean(relPath)
	if filepath.IsAbs(cleanRel) || strings.Contains(cleanRel, "..") {
		return "", ErrScriptPathEscape
	}

	var baseDir string
	if r.deps.TempDirResolver != nil {
		tmpDir, err := r.deps.TempDirResolver(ctx, "skill-script")
		if err == nil && tmpDir != "" {
			baseDir = tmpDir
		}
	}
	if baseDir == "" {
		tmp, err := os.MkdirTemp("", "skill-script-*")
		if err != nil {
			return "", ErrScriptInternalError
		}
		baseDir = tmp
	}

	scriptDir := filepath.Join(baseDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o750); err != nil {
		return "", ErrScriptInternalError
	}

	scriptPath := filepath.Join(scriptDir, filepath.Base(cleanRel))
	if err := os.WriteFile(scriptPath, content, 0o640); err != nil {
		return "", ErrScriptInternalError
	}

	return baseDir, nil
}

func (r *ScriptRuntime) ValidateScript(skillRoot, relPath string) error {
	_, err := ValidateScriptPath(skillRoot, relPath, r.policyContext)
	return err
}

func (r *ScriptRuntime) GetExecutor() *scriptExecutor {
	return r.executor
}

func (r *ScriptRuntime) GetDiscovery() *scriptDiscovery {
	return r.discovery
}

func equalHashes(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i]|0x20 != b[i]|0x20 {
			return false
		}
	}
	return true
}
