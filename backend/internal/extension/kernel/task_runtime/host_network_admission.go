package task_runtime

import (
	"strings"

	"github.com/u-ai/backend/internal/runtimeprofile"
)

const cloudTaskNetworkPolicyEnv = "AMITIA_CLOUD_TASK_NETWORK_POLICY"

const (
	cloudTaskNetworkUnrestricted = "unrestricted"
	cloudTaskNetworkIsolated     = "isolated"
	cloudTaskNetworkDegraded     = "degraded"
)

func cloudHostNetworkMode(profile runtimeprofile.Profile, sandboxed bool, policy string) (string, error) {
	if profile != runtimeprofile.ProfileCloudCore || sandboxed {
		return cloudTaskNetworkUnrestricted, nil
	}
	switch strings.TrimSpace(policy) {
	case "", "strict":
		return cloudTaskNetworkIsolated, nil
	case "allow-degraded":
		return cloudTaskNetworkDegraded, nil
	default:
		return "", NewTaskError(ErrTaskScopeDenied, "云端任务网络策略无效：只能使用 strict 或 allow-degraded")
	}
}
