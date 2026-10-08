//go:build !windows

package trusted_service

import (
	"fmt"
	"github.com/u-ai/backend/internal/platform/process"
)

func prepareWindowsTaskContainerLaunch(executable string, args []string, workingDir, tempDir string, limits process.ResourceLimits, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	return sandboxLaunchPlan{}, ErrNetworkSandboxUnavailable
}

func prepareWindowsAppContainerLaunch(mode, executable string, args []string, workingDir, tempDir, stateRoot string, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	return sandboxLaunchPlan{}, fmt.Errorf("%w: Windows AppContainer backend is not available on this platform", ErrNetworkSandboxUnavailable)
}
