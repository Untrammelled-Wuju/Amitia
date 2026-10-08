package task_runtime

import (
	"context"
	"fmt"
	"runtime"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type SourceTaskCapabilities struct {
	IsolatedExecution bool   `json:"isolatedExecution"`
	ExecuteTool       bool   `json:"executeTool"`
	EmitEvent         bool   `json:"emitEvent"`
	UnavailableReason string `json:"unavailableReason,omitempty"`
}

type sourceTaskPreflightKey struct{}

func CurrentSourceTaskCapabilities() SourceTaskCapabilities {
	return sourceTaskCapabilitiesForPlatform(runtime.GOOS)
}

func sourceTaskCapabilitiesForPlatform(platform string) SourceTaskCapabilities {
	capabilities := SourceTaskCapabilities{}
	if platform != "windows" {
		capabilities.UnavailableReason = fmt.Sprintf("当前设备（%s）尚不支持已验证的任务强隔离，不能提交设备任务", platform)
		return capabilities
	}
	if err := trusted_service.ValidateTaskSandboxPrerequisites(); err != nil {
		capabilities.UnavailableReason = fmt.Sprintf("当前设备（%s）尚不支持已验证的任务强隔离，不能提交设备任务：%v", runtime.GOOS, err)
		return capabilities
	}
	capabilities.IsolatedExecution = true
	return capabilities
}

func validateSourceTaskExecutionAvailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return context.Cause(ctx)
	}
	if validated, _ := ctx.Value(sourceTaskPreflightKey{}).(bool); validated {
		return nil
	}
	capabilities := CurrentSourceTaskCapabilities()
	return validateSourceTaskCapabilities(capabilities)
}

func validateSourceTaskCapabilities(capabilities SourceTaskCapabilities) error {
	if !capabilities.IsolatedExecution {
		return NewTaskError(ErrTaskDependencyUnavailable, capabilities.UnavailableReason)
	}
	return nil
}

func validateSourceTaskDeclaredCapabilities(definition *TaskDefinition) error {
	requirements, err := sourceTaskPermissionRequirements(definition)
	if err != nil {
		return err
	}
	for _, requirement := range requirements {
		if requirement.PermissionID == "service.tool.execute" {
			return NewTaskError(ErrTaskDependencyUnavailable, "当前设备任务运行时尚未提供已授权的工具执行端口，不能提交要求执行工具的任务")
		}
	}
	return nil
}
