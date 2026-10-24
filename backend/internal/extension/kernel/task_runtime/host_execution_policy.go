package task_runtime

import "github.com/u-ai/backend/internal/runtimeprofile"

func validateHostExecutionIdentity(profile runtimeprofile.Profile, effectiveUID int) error {
	if profile == runtimeprofile.ProfileCloudCore && effectiveUID == 0 {
		return NewTaskError(ErrTaskScopeDenied, "Cloud Core 不能以 root 身份执行任务插件；请使用受限的独立服务账户")
	}
	return nil
}
