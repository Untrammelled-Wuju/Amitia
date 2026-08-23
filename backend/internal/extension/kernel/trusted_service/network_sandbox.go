package trusted_service

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var (
	ErrNetworkSandboxUnavailable        = errors.New("trusted_service: network sandbox unavailable")
	ErrGranularNetworkPolicyUnsupported = errors.New("trusted_service: granular network policy requires a network proxy/firewall backend")
)

// prepareNetworkLaunch turns an enforceable ServiceNetworkPolicy into an OS
// launch boundary. Policies are never silently weakened: unsupported restricted
// policies fail closed instead of starting an unsandboxed process.
func prepareNetworkLaunch(policy ServiceNetworkPolicy, executable string, args []string, workingDir, tempDir string) (string, []string, error) {
	if !policy.Enforce {
		return executable, args, nil
	}
	mode := strings.ToLower(strings.TrimSpace(policy.Mode))
	if mode == "" {
		switch {
		case policy.LoopbackOnly:
			mode = "loopback"
		case policy.RequireProxy || len(policy.AllowedDomains) > 0 || len(policy.AllowedPorts) > 0:
			mode = "restricted"
		case policy.AllowOutbound:
			mode = "unrestricted"
		default:
			mode = "none"
		}
	}
	if policy.AllowInbound && !policy.LoopbackOnly {
		return "", nil, fmt.Errorf("%w: non-loopback inbound access is forbidden", ErrUnauthorizedNetwork)
	}
	if policy.AuditAll {
		return "", nil, fmt.Errorf("%w: packet-level network audit backend is unavailable", ErrNetworkSandboxUnavailable)
	}

	switch mode {
	case "unrestricted":
		if !policy.AllowOutbound || policy.LoopbackOnly || policy.RequireProxy || len(policy.AllowedDomains) > 0 || len(policy.AllowedPorts) > 0 {
			return "", nil, fmt.Errorf("%w: inconsistent unrestricted policy", ErrUnauthorizedNetwork)
		}
		// Explicit unrestricted mode is the user-approved escape hatch for games
		// such as Mineflayer that need arbitrary TCP destinations.
		return executable, args, nil
	case "restricted":
		// Restricted arbitrary TCP requires a platform firewall/proxy backend.
		// Until such a backend is present, deny startup rather than degrading to
		// unrestricted connectivity.
		return "", nil, fmt.Errorf("%w: domains=%v ports=%v requireProxy=%v", ErrGranularNetworkPolicyUnsupported, policy.AllowedDomains, policy.AllowedPorts, policy.RequireProxy)
	case "none", "loopback":
		// Continue to OS network namespace isolation below.
	default:
		return "", nil, fmt.Errorf("%w: unsupported network mode %q", ErrUnauthorizedNetwork, mode)
	}

	// Linux bubblewrap creates a separate network namespace, preventing access
	// to host/public interfaces. The child sees only its private namespace.
	if runtime.GOOS == "linux" {
		if bwrap, err := exec.LookPath("bwrap"); err == nil {
			wrapped := []string{
				"--die-with-parent", "--new-session", "--unshare-net",
				"--ro-bind", "/", "/",
				"--dev", "/dev", "--proc", "/proc",
			}
			if strings.TrimSpace(workingDir) != "" {
				wrapped = append(wrapped, "--bind", workingDir, workingDir, "--chdir", workingDir)
			}
			if strings.TrimSpace(tempDir) != "" && tempDir != workingDir {
				wrapped = append(wrapped, "--bind", tempDir, tempDir)
			}
			wrapped = append(wrapped, "--", executable)
			wrapped = append(wrapped, args...)
			return bwrap, wrapped, nil
		}

		// util-linux unshare is a safe fallback for a strict deny-all policy.
		// It cannot reliably configure loopback inside the namespace, so loopback
		// mode stays fail-closed unless bubblewrap is available.
		if mode == "none" {
			if unshare, err := exec.LookPath("unshare"); err == nil {
				wrapped := []string{"--user", "--map-root-user", "--net", "--fork", "--kill-child", "--", executable}
				wrapped = append(wrapped, args...)
				return unshare, wrapped, nil
			}
		}
		return "", nil, fmt.Errorf("%w: no compatible linux network namespace launcher is available for mode %s", ErrNetworkSandboxUnavailable, mode)
	}

	return "", nil, fmt.Errorf("%w on %s for mode %s", ErrNetworkSandboxUnavailable, runtime.GOOS, mode)
}
