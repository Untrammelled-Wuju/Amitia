//go:build !windows

package trusted_service

import "fmt"

func prepareWindowsAppContainerLaunch(mode, executable string, args []string, workingDir, tempDir string, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	return sandboxLaunchPlan{}, fmt.Errorf("%w: Windows AppContainer backend is not available on this platform", ErrNetworkSandboxUnavailable)
}
