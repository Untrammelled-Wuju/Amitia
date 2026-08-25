package trusted_service

import (
	"errors"
	"fmt"
	"os"
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
type sandboxLaunchPlan struct {
	Path                  string
	Args                  []string
	WorkingDir            string
	FilesystemIsolated    bool
	NetworkPolicyEnforced bool
}

func directSandboxLaunch(executable string, args []string, workingDir string) sandboxLaunchPlan {
	return sandboxLaunchPlan{Path: executable, Args: args, WorkingDir: workingDir}
}

func prepareNetworkLaunch(policy ServiceNetworkPolicy, executable string, args []string, workingDir, tempDir string, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	if !policy.Enforce {
		return directSandboxLaunch(executable, args, workingDir), nil
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
		return sandboxLaunchPlan{}, fmt.Errorf("%w: non-loopback inbound access is forbidden", ErrUnauthorizedNetwork)
	}
	if policy.AuditAll {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: packet-level network audit backend is unavailable", ErrNetworkSandboxUnavailable)
	}

	switch mode {
	case "unrestricted":
		if !policy.AllowOutbound || policy.LoopbackOnly || policy.RequireProxy || len(policy.AllowedDomains) > 0 || len(policy.AllowedPorts) > 0 {
			return sandboxLaunchPlan{}, fmt.Errorf("%w: inconsistent unrestricted policy", ErrUnauthorizedNetwork)
		}
		// Network access is unrestricted, but Enforce still means the plugin must
		// cross the platform filesystem/process sandbox. Do not bypass the sandbox
		// merely because the user approved outbound networking.
	case "restricted":
		// Restricted arbitrary TCP requires a platform firewall/proxy backend.
		// Until such a backend is present, deny startup rather than degrading to
		// unrestricted connectivity.
		return sandboxLaunchPlan{}, fmt.Errorf("%w: domains=%v ports=%v requireProxy=%v", ErrGranularNetworkPolicyUnsupported, policy.AllowedDomains, policy.AllowedPorts, policy.RequireProxy)
	case "none", "loopback":
		// Continue to the platform sandbox below.
	default:
		return sandboxLaunchPlan{}, fmt.Errorf("%w: unsupported network mode %q", ErrUnauthorizedNetwork, mode)
	}

	// Linux bubblewrap can either share the host network namespace (which would
	// also permit non-loopback inbound listeners) or create a private namespace
	// with no outbound path. Until an outbound-only slirp/firewall backend is
	// wired, unrestricted outbound cannot be enforced safely on Linux.
	if runtime.GOOS == "linux" && mode == "unrestricted" {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: linux outbound-only unrestricted mode requires a dedicated network backend", ErrGranularNetworkPolicyUnsupported)
	}

	// Linux bubblewrap creates a separate network namespace, preventing access
	// to host/public interfaces. The child sees only its private namespace.
	if runtime.GOOS == "linux" {
		if bwrap := firstTrustedLauncher("/usr/bin/bwrap", "/bin/bwrap"); bwrap != "" {
			// Do not expose the host filesystem with --ro-bind / /. The network
			// sandbox is also a confidentiality boundary: give the child a fresh
			// root and bind only runtime prerequisites plus its owned work/temp
			// directories. In particular, user home directories are never mounted.
			wrapped := []string{
				"--die-with-parent", "--new-session",
				"--tmpfs", "/", "--dev", "/dev", "--proc", "/proc",
			}
			if mode == "none" || mode == "loopback" {
				wrapped = append(wrapped, "--unshare-net")
			}
			createdDirs := map[string]struct{}{
				"/": {}, "/dev": {}, "/proc": {},
			}
			for _, systemPath := range []string{
				"/usr", "/bin", "/sbin", "/lib", "/lib64",
				"/etc/ld.so.cache", "/etc/ld.so.conf", "/etc/ld.so.conf.d",
				"/etc/ssl", "/etc/ca-certificates", "/etc/localtime",
			} {
				if _, statErr := os.Stat(systemPath); statErr == nil {
					wrapped = appendSandboxParentDirs(wrapped, systemPath, createdDirs)
					wrapped = append(wrapped, "--ro-bind", systemPath, systemPath)
					if info, infoErr := os.Stat(systemPath); infoErr == nil && info.IsDir() {
						createdDirs[filepath.Clean(systemPath)] = struct{}{}
					}
				}
			}
			work := strings.TrimSpace(workingDir)
			tmp := strings.TrimSpace(tempDir)
			if work != "" {
				wrapped = appendSandboxParentDirs(wrapped, work, createdDirs)
				wrapped = append(wrapped, "--bind", work, work, "--chdir", work)
				createdDirs[filepath.Clean(work)] = struct{}{}
			}
			if tmp != "" && tmp != work {
				wrapped = appendSandboxParentDirs(wrapped, tmp, createdDirs)
				wrapped = append(wrapped, "--bind", tmp, tmp)
				createdDirs[filepath.Clean(tmp)] = struct{}{}
			}

			// Game runtimes frequently execute a managed interpreter (for example
			// Node) while loading the actual plugin entry point and sibling modules
			// from the immutable installed package. Expose only caller-approved
			// package/dependency roots read-only; otherwise strict network mode would
			// accidentally make valid JS plugins unable to read their own code.
			seenReadOnly := make(map[string]struct{})
			for _, root := range readOnlyRoots {
				root = strings.TrimSpace(root)
				if root == "" || root == work || root == tmp {
					continue
				}
				abs, absErr := filepath.Abs(root)
				if absErr != nil {
					return sandboxLaunchPlan{}, fmt.Errorf("%w: resolve read-only sandbox root %q: %v", ErrNetworkSandboxUnavailable, root, absErr)
				}
				abs = filepath.Clean(abs)
				if abs == string(filepath.Separator) {
					return sandboxLaunchPlan{}, fmt.Errorf("%w: refusing to expose the host filesystem root", ErrNetworkSandboxUnavailable)
				}
				if (work != "" && pathWithin(abs, work)) || (tmp != "" && pathWithin(abs, tmp)) {
					return sandboxLaunchPlan{}, fmt.Errorf("%w: read-only root %q must not contain writable work/temp directories", ErrNetworkSandboxUnavailable, abs)
				}
				if _, duplicate := seenReadOnly[abs]; duplicate {
					continue
				}
				if _, statErr := os.Stat(abs); statErr != nil {
					return sandboxLaunchPlan{}, fmt.Errorf("%w: read-only sandbox root %q is unavailable: %v", ErrNetworkSandboxUnavailable, abs, statErr)
				}
				seenReadOnly[abs] = struct{}{}
				wrapped = appendSandboxParentDirs(wrapped, abs, createdDirs)
				wrapped = append(wrapped, "--ro-bind", abs, abs)
				if info, infoErr := os.Stat(abs); infoErr == nil && info.IsDir() {
					createdDirs[filepath.Clean(abs)] = struct{}{}
				}
			}

			// If the executable is not already reachable through work or an approved
			// read-only root, expose that single file read-only.
			executableCovered := work != "" && pathWithin(work, executable)
			if !executableCovered {
				for root := range seenReadOnly {
					if pathWithin(root, executable) {
						executableCovered = true
						break
					}
				}
			}
			if !executableCovered {
				if _, statErr := os.Stat(executable); statErr == nil {
					wrapped = appendSandboxParentDirs(wrapped, executable, createdDirs)
					wrapped = append(wrapped, "--ro-bind", executable, executable)
				}
			}
			if mode == "loopback" {
				// A new Linux network namespace starts with loopback down. Bring only lo
				// up inside the namespace; no host/public interface is present. This is
				// still fail-closed if the platform cannot configure the namespace.
				ip := firstTrustedLauncher("/usr/sbin/ip", "/usr/bin/ip", "/sbin/ip", "/bin/ip")
				sh := firstTrustedLauncher("/bin/sh", "/usr/bin/sh")
				if ip == "" || sh == "" {
					return sandboxLaunchPlan{}, fmt.Errorf("%w: loopback mode requires a trusted ip and sh binary", ErrNetworkSandboxUnavailable)
				}
				wrapped = append(wrapped, "--", sh, "-c", `"$1" link set lo up >/dev/null 2>&1 || exit 126; shift; exec "$@"`, "amitia-loopback", ip, executable)
				wrapped = append(wrapped, args...)
				return sandboxLaunchPlan{Path: bwrap, Args: wrapped, WorkingDir: workingDir, FilesystemIsolated: true, NetworkPolicyEnforced: true}, nil
			}
			wrapped = append(wrapped, "--", executable)
			wrapped = append(wrapped, args...)
			return sandboxLaunchPlan{Path: bwrap, Args: wrapped, WorkingDir: workingDir, FilesystemIsolated: true, NetworkPolicyEnforced: true}, nil
		}

		return sandboxLaunchPlan{}, fmt.Errorf("%w: no compatible linux network namespace launcher is available for mode %s", ErrNetworkSandboxUnavailable, mode)
	}

	// macOS Seatbelt can enforce process-level network policy without granting
	// the plugin ambient network access. sandbox-exec is deprecated but remains
	// an OS-enforced fail-closed backend on supported macOS releases; if it is
	// absent we reject the launch rather than silently weakening the policy.
	if runtime.GOOS == "darwin" {
		// Never resolve a security launcher through PATH: an extension-controlled
		// PATH entry must not be able to replace the OS sandbox executable.
		sandboxExec := firstTrustedLauncher("/usr/bin/sandbox-exec")
		if sandboxExec == "" {
			return sandboxLaunchPlan{}, fmt.Errorf("%w: /usr/bin/sandbox-exec is unavailable for mode %s", ErrNetworkSandboxUnavailable, mode)
		}
		profile, err := buildDarwinSandboxProfile(mode, executable, workingDir, tempDir, readOnlyRoots...)
		if err != nil {
			return sandboxLaunchPlan{}, err
		}
		wrapped := []string{"-p", profile, executable}
		wrapped = append(wrapped, args...)
		return sandboxLaunchPlan{Path: sandboxExec, Args: wrapped, WorkingDir: workingDir, FilesystemIsolated: true, NetworkPolicyEnforced: true}, nil
	}

	if runtime.GOOS == "windows" {
		return prepareWindowsAppContainerLaunch(mode, executable, args, workingDir, tempDir, readOnlyRoots...)
	}

	return sandboxLaunchPlan{}, fmt.Errorf("%w on %s for mode %s", ErrNetworkSandboxUnavailable, runtime.GOOS, mode)
}

func firstTrustedLauncher(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = filepath.Clean(strings.TrimSpace(candidate))
		if candidate == "" || !filepath.IsAbs(candidate) {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		return candidate
	}
	return ""
}

// appendSandboxParentDirs creates only destination parent directories inside
// bubblewrap's fresh tmpfs root. It never exposes host content; bind operations
// that follow decide which verified paths become visible.
func appendSandboxParentDirs(args []string, target string, created map[string]struct{}) []string {
	target = filepath.Clean(strings.TrimSpace(target))
	if target == "" || target == "." || !filepath.IsAbs(target) {
		return args
	}
	parent := filepath.Dir(target)
	if parent == "/" || parent == "." {
		return args
	}
	var missing []string
	for current := parent; current != "/" && current != "."; current = filepath.Dir(current) {
		if _, ok := created[current]; ok {
			break
		}
		missing = append(missing, current)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		dir := missing[i]
		if _, ok := created[dir]; ok {
			continue
		}
		args = append(args, "--dir", dir)
		created[dir] = struct{}{}
	}
	return args
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

func buildDarwinSandboxProfile(mode, executable, workingDir, tempDir string, readOnlyRoots ...string) (string, error) {
	if mode != "none" && mode != "loopback" && mode != "unrestricted" {
		return "", fmt.Errorf("%w: unsupported darwin mode %q", ErrUnauthorizedNetwork, mode)
	}
	var b strings.Builder
	b.WriteString("(version 1)\n")
	b.WriteString("(deny default)\n")
	b.WriteString("(allow process*)\n")
	b.WriteString("(allow signal (target self))\n")
	b.WriteString("(allow sysctl-read)\n")
	b.WriteString("(allow mach-lookup)\n")
	b.WriteString("(allow file-read-metadata)\n")
	// Runtime/linker/system roots needed by native and managed runtimes. User
	// homes are deliberately absent; plugin/package roots are added explicitly.
	for _, root := range []string{"/System", "/usr", "/bin", "/sbin", "/Library", "/private/etc", "/private/var/db", "/dev"} {
		b.WriteString("(allow file-read* (subpath ")
		b.WriteString(seatbeltQuote(root))
		b.WriteString("))\n")
	}
	seen := make(map[string]struct{})
	addRead := func(path string) error {
		path = strings.TrimSpace(path)
		if path == "" {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		abs = filepath.Clean(abs)
		if abs == string(filepath.Separator) {
			return fmt.Errorf("%w: refusing filesystem root", ErrNetworkSandboxUnavailable)
		}
		if _, ok := seen[abs]; ok {
			return nil
		}
		seen[abs] = struct{}{}
		b.WriteString("(allow file-read* (subpath ")
		b.WriteString(seatbeltQuote(abs))
		b.WriteString("))\n")
		return nil
	}
	for _, root := range readOnlyRoots {
		if err := addRead(root); err != nil {
			return "", err
		}
	}
	if err := addRead(filepath.Dir(executable)); err != nil {
		return "", err
	}
	for _, root := range []string{workingDir, tempDir} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		if err := addRead(abs); err != nil {
			return "", err
		}
		b.WriteString("(allow file-write* (subpath ")
		b.WriteString(seatbeltQuote(filepath.Clean(abs)))
		b.WriteString("))\n")
	}
	b.WriteString("(allow file-write* (literal \"/dev/null\"))\n")
	switch mode {
	case "unrestricted":
		// Outbound approval must never imply permission to bind host-facing
		// listeners. Seatbelt can express this distinction directly.
		b.WriteString("(allow network-outbound)\n")
	case "loopback":
		b.WriteString("(allow network-outbound (remote ip \"localhost:*\"))\n")
		b.WriteString("(allow network-inbound (local ip \"localhost:*\"))\n")
	}
	return b.String(), nil
}

func seatbeltQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}
