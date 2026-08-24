package trusted_service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
			// Do not expose the host filesystem with --ro-bind / /. The network
			// sandbox is also a confidentiality boundary: give the child a fresh
			// root and bind only runtime prerequisites plus its owned work/temp
			// directories. In particular, user home directories are never mounted.
			wrapped := []string{
				"--die-with-parent", "--new-session", "--unshare-net",
				"--tmpfs", "/", "--dev", "/dev", "--proc", "/proc",
			}
			for _, systemPath := range []string{
				"/usr", "/bin", "/sbin", "/lib", "/lib64",
				"/etc/ld.so.cache", "/etc/ld.so.conf", "/etc/ld.so.conf.d",
				"/etc/ssl", "/etc/ca-certificates", "/etc/localtime",
			} {
				if _, statErr := os.Stat(systemPath); statErr == nil {
					wrapped = append(wrapped, "--ro-bind", systemPath, systemPath)
				}
			}
			work := strings.TrimSpace(workingDir)
			tmp := strings.TrimSpace(tempDir)
			if work != "" {
				wrapped = append(wrapped, "--bind", work, work, "--chdir", work)
			}
			if tmp != "" && tmp != work {
				wrapped = append(wrapped, "--bind", tmp, tmp)
			}
			// Executables normally live under work. If a trusted runtime points at
			// a different path, expose only that executable instead of its parent.
			if work == "" || !pathWithin(work, executable) {
				if _, statErr := os.Stat(executable); statErr == nil {
					wrapped = append(wrapped, "--ro-bind", executable, executable)
				}
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

func pathWithin(root, candidate string) bool {
	root = strings.TrimSpace(root)
	candidate = strings.TrimSpace(candidate)
	if root == "" || candidate == "" {
		return false
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
