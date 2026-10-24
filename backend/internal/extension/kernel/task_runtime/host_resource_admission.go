package task_runtime

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

const cloudTaskResourcePolicyEnv = "AMITIA_CLOUD_TASK_RESOURCE_POLICY"

func validateCloudHostResourceLimits(profile runtimeprofile.Profile, sandboxed bool, limits process.ResourceLimits, support process.ResourceLimitSupport, policy string) error {
	if profile != runtimeprofile.ProfileCloudCore || sandboxed {
		return nil
	}
	policy = strings.TrimSpace(policy)
	if policy != "" && policy != "strict" && policy != "allow-degraded" {
		return NewTaskError(ErrTaskScopeDenied, "云端任务资源策略无效，必须为 strict 或 allow-degraded")
	}

	if support.Processes && limits.MaxProcesses > 0 && limits.MaxProcesses < taskProcessPIDBudget(runtime.GOOS) {
		return NewTaskError(ErrTaskScopeDenied, fmt.Sprintf("Cloud Core 任务 pids 预算 %d 不足以启动 Node 运行时，至少需要 %d", limits.MaxProcesses, taskProcessPIDBudget(runtime.GOOS)))
	}
	missing := cloudHostMissingResourceLimits(limits, support)
	if len(missing) == 0 || policy == "allow-degraded" {
		return nil
	}
	return NewTaskError(ErrTaskScopeDenied, fmt.Sprintf(
		"Cloud Core 拒绝在缺少强制资源限制（%s）的宿主上运行任务；请配置可写入的 cgroup v2，或由管理员显式设置 %s=allow-degraded 以接受资源隔离降级风险",
		strings.Join(missing, ", "), cloudTaskResourcePolicyEnv,
	))
}

func cloudHostMissingResourceLimits(limits process.ResourceLimits, support process.ResourceLimitSupport) []string {
	missing := make([]string, 0, 3)
	if !support.Memory || limits.MaxMemoryBytes == 0 {
		missing = append(missing, "memory")
	}
	if !support.CPU || limits.MaxCPUPercent == 0 {
		missing = append(missing, "cpu")
	}
	if !support.Processes || limits.MaxProcesses == 0 {
		missing = append(missing, "pids")
	}
	return missing
}
