// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
)

type ScriptExecutorContext struct {
	InterpreterResolver ScriptInterpreterResolver
	ProcessSupervisor   runtimehost.ProcessSupervisor
	TempDirResolver     func(ctx context.Context, skillName string) (string, error)
}

type scriptExecutor struct {
	ctx            ScriptExecutorContext
	runningScripts map[string]context.CancelFunc
	mu             sync.Mutex
}

func NewScriptExecutor(ctx ScriptExecutorContext) *scriptExecutor {
	return &scriptExecutor{
		ctx:            ctx,
		runningScripts: make(map[string]context.CancelFunc),
	}
}

func (e *scriptExecutor) BuildPlan(ctx context.Context, desc SkillScriptDescriptor, args map[string]any, inputs map[string]string) (*SkillScriptExecutionPlan, error) {
	if err := ValidateScriptRuntime(desc.Runtime); err != nil {
		return nil, err
	}
	if err := ValidateScriptKind(desc.Kind); err != nil {
		return nil, err
	}
	if err := ValidateDependencies(desc.Dependencies); err != nil {
		return nil, err
	}
	if err := ValidateWorkingDirPolicy(desc.WorkingDirPolicy); err != nil {
		return nil, err
	}
	if err := ValidateOutputMode(desc.OutputMode); err != nil {
		return nil, err
	}

	timeout := desc.Timeout
	if timeout <= 0 {
		timeout = DefaultScriptTimeout
	}
	if timeout > MaxScriptTimeout {
		return nil, ErrScriptInvalidTimeout
	}

	interpreter, err := e.ctx.InterpreterResolver.ResolveFromDescriptor(ctx, desc)
	if err != nil {
		return nil, err
	}

	resolvedArgs, err := SanitizeArgs(args, desc.DeclaredArgs)
	if err != nil {
		return nil, err
	}

	envPolicy := runtimehost.EnvPolicyMinimal
	if desc.Runtime == ScriptRuntimeNode {
		envPolicy = runtimehost.EnvPolicyMinimal
	}

	plan := &SkillScriptExecutionPlan{
		Descriptor:     desc,
		Interpreter:    interpreter,
		ResolvedArgs:   resolvedArgs,
		ResolvedInputs: inputs,
		Permissions:    desc.Permissions,
		Timeout:        timeout,
		EnvPolicy:      string(envPolicy),
		Env:            make(map[string]string),
	}

	if desc.Runtime == ScriptRuntimeNode && interpreter.Executable != "" {
		plan.Executable = interpreter.Executable
		plan.Args = buildNodeArgs(desc, interpreter)
	} else if desc.Runtime == ScriptRuntimeNative {
		plan.Executable = interpreter.Executable
	}

	return plan, nil
}

func (e *scriptExecutor) SetWorkingDir(plan *SkillScriptExecutionPlan, dir string) error {
	if dir == "" {
		return ErrScriptInvalidWorkingDir
	}
	plan.WorkingDir = dir
	return nil
}

func buildNodeArgs(desc SkillScriptDescriptor, interp ScriptInterpreter) []string {
	args := make([]string, 0)
	args = append(args, interp.ArgsPrefix...)
	entryPath := filepath.ToSlash(filepath.Join("scripts", desc.EntryName))
	args = append(args, entryPath)
	return args
}

func (e *scriptExecutor) Execute(ctx context.Context, plan *SkillScriptExecutionPlan, executionID string) (*SkillScriptResult, error) {
	if plan == nil {
		return nil, ErrScriptInvalidDescriptor
	}
	if plan.Interpreter.Kind == "" {
		return nil, ErrScriptInterpreterUnavailable
	}

	execCtx, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()

	e.registerExecution(executionID, cancel)
	defer e.unregisterExecution(executionID)

	result := &SkillScriptResult{
		ExecutionID: executionID,
		Status:      StatusSuccess,
	}

	processID := fmt.Sprintf("skill-script-%s", executionID)
	if len(processID) > 128 {
		processID = processID[:128]
	}

	spec := runtimehost.ProcessSpec{
		ID:             runtimehost.ProcessID(processID),
		Executable:     plan.Executable,
		Args:           plan.Args,
		WorkingDir:     plan.WorkingDir,
		RestartPolicy:  runtimehost.RestartPolicy{Mode: runtimehost.RestartNever},
		StartupTimeout: plan.Timeout,
		StopGracePeriod: 5 * time.Second,
	}

	if plan.EnvPolicy != "" {
		spec.Environment.Policy = runtimehost.EnvironmentPolicy(plan.EnvPolicy)
	}
	if len(plan.Env) > 0 {
		spec.Environment.Values = plan.Env
	}

	if spec.WorkingDir == "" {
		return nil, ErrScriptInvalidWorkingDir
	}

	if e.ctx.ProcessSupervisor == nil {
		return nil, ErrScriptProcessSupervisorFailed
	}

	if err := e.ctx.ProcessSupervisor.Register(spec); err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = fmt.Sprintf("process register failed: %v", err)
		return result, ErrScriptProcessRegisterFailed
	}

	if err := e.ctx.ProcessSupervisor.Start(execCtx, spec.ID); err != nil {
		result.Status = StatusFailed
		result.ErrorMessage = fmt.Sprintf("process start failed: %v", err)
		return result, ErrScriptProcessStartFailed
	}

	waitErr := e.ctx.ProcessSupervisor.WaitReady(execCtx, spec.ID)

	e.ctx.ProcessSupervisor.Stop(execCtx, spec.ID)

	if waitErr != nil {
		if errors.Is(waitErr, context.DeadlineExceeded) {
			result.Status = StatusTimeout
			result.ErrorMessage = "script execution timed out"
			return result, ErrScriptTimeoutExceeded
		}
		result.Status = StatusFailed
		result.ErrorMessage = waitErr.Error()
		return result, ErrScriptProcessFailed
	}

	result.ExitCode = 0
	result.Output = ""
	result.Status = StatusSuccess

	return result, nil
}

func (e *scriptExecutor) Cancel(executionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	cancel, ok := e.runningScripts[executionID]
	if !ok {
		return ErrScriptNotFound
	}
	cancel()
	delete(e.runningScripts, executionID)
	return nil
}

func (e *scriptExecutor) registerExecution(id string, cancel context.CancelFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runningScripts[id] = cancel
}

func (e *scriptExecutor) unregisterExecution(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.runningScripts, id)
}

func (e *scriptExecutor) RunningCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.runningScripts)
}

func (e *scriptExecutor) ResolveWorkingDir(plan *SkillScriptExecutionPlan, skillRoot string) (string, error) {
	switch plan.Descriptor.WorkingDirPolicy {
	case WorkingDirPolicySkillRoot, "":
		return skillRoot, nil
	case WorkingDirPolicyTemp:
		if e.ctx.TempDirResolver == nil {
			return "", ErrScriptInvalidWorkingDir
		}
		tmpDir, err := e.ctx.TempDirResolver(context.Background(), plan.Descriptor.SkillName)
		if err != nil {
			return "", ErrScriptInvalidWorkingDir
		}
		return tmpDir, nil
	case WorkingDirPolicyExplicit:
		if plan.WorkingDir == "" {
			return "", ErrScriptInvalidWorkingDir
		}
		return plan.WorkingDir, nil
	default:
		return "", ErrScriptInvalidWorkingDir
	}
}
