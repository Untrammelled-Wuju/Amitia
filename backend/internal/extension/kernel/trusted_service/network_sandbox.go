package trusted_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

var (
	ErrNetworkSandboxUnavailable = errors.New("trusted_service: network sandbox unavailable")
	// ErrGranularNetworkPolicyUnsupported is retained for callers compiled against
	// the previous API. Restricted network policies are now enforced through the
	// host-mediated network routes; unsupported policy shapes fail closed with the
	// more specific sandbox/authorization errors.
	ErrGranularNetworkPolicyUnsupported = errors.New("trusted_service: granular network policy requires a network proxy/firewall backend")
)

// prepareNetworkLaunch turns an enforceable ServiceNetworkPolicy into an OS
// launch boundary. Policies are never silently weakened: any policy that the
// active platform cannot enforce fails closed before plugin code is allowed to run.
type sandboxLaunchPlan struct {
	Path                  string
	Args                  []string
	WorkingDir            string
	FilesystemIsolated    bool
	NetworkPolicyEnforced bool
	ExtraFiles            []*os.File
	AfterStart            func(context.Context, *exec.Cmd) error
	Cleanup               func()
}

func (p sandboxLaunchPlan) cleanup() {
	if p.Cleanup != nil {
		p.Cleanup()
	}
}

func directSandboxLaunch(executable string, args []string, workingDir string) sandboxLaunchPlan {
	return sandboxLaunchPlan{Path: executable, Args: args, WorkingDir: workingDir}
}

// ValidateNetworkPolicySupport rejects policy/platform combinations that the
// trusted service launcher cannot enforce without weakening the requested
// boundary. It is intentionally safe to call during GameHost provisioning so
// unsupported modes fail before a child process is created. prepareNetworkLaunch
// performs the same validation again at the final launch boundary.
func ValidateNetworkPolicySupport(policy ServiceNetworkPolicy) error {
	return validateNetworkPolicySupportForOS(policy, runtime.GOOS)
}

func validateNetworkPolicySupportForOS(policy ServiceNetworkPolicy, goos string) error {
	if !policy.Enforce {
		return nil
	}
	goos = strings.ToLower(strings.TrimSpace(goos))
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return fmt.Errorf("%w on %s", ErrNetworkSandboxUnavailable, goos)
	}
	mode := normalizedNetworkMode(policy)
	if policy.AllowInbound && !policy.LoopbackOnly {
		return fmt.Errorf("%w: non-loopback inbound access is forbidden", ErrUnauthorizedNetwork)
	}
	if policy.AuditAll {
		return fmt.Errorf("%w: packet-level network audit backend is unavailable", ErrNetworkSandboxUnavailable)
	}

	switch mode {
	case "unrestricted":
		if !policy.AllowOutbound || policy.LoopbackOnly || policy.RequireProxy || len(policy.AllowedDomains) > 0 || len(policy.AllowedIPs) > 0 || len(policy.AllowedPorts) > 0 || len(policy.AllowedTransports) > 0 || policy.AllowHostLoopback || policy.MaxConnections > 0 {
			return fmt.Errorf("%w: inconsistent unrestricted policy", ErrUnauthorizedNetwork)
		}
	case "restricted":
		// Restricted mode is deliberately host-mediated: the plugin child itself
		// remains in the same no-network sandbox as mode=none. The allowlist is
		// enforced by host.network.*, not by ambient child sockets.
		if policy.AllowOutbound || policy.AllowInbound || policy.LoopbackOnly || !policy.RequireProxy {
			return fmt.Errorf("%w: inconsistent restricted policy", ErrUnauthorizedNetwork)
		}
		if err := validateRestrictedAllowlist(policy.AllowedDomains, policy.AllowedIPs, policy.AllowedPorts, policy.AllowedTransports, policy.AllowHostLoopback, policy.MaxConnections); err != nil {
			return err
		}
	case "none", "loopback":
		return nil
	default:
		return fmt.Errorf("%w: unsupported network mode %q", ErrUnauthorizedNetwork, mode)
	}
	return nil
}

func validateRestrictedAllowlist(domains, ips []string, ports []int, transports []string, allowHostLoopback bool, maxConnections int) error {
	if (len(domains) == 0 && len(ips) == 0 && !allowHostLoopback) || len(ports) == 0 {
		return fmt.Errorf("%w: restricted mode requires a non-empty destination allowlist and port allowlist", ErrUnauthorizedNetwork)
	}
	seenDomains := make(map[string]struct{}, len(domains))
	for _, raw := range domains {
		value := strings.ToLower(strings.TrimSpace(raw))
		if strings.HasPrefix(value, "*.") {
			value = strings.TrimPrefix(value, "*.")
		}
		if value == "" || len(value) > 253 || strings.ContainsAny(value, "*/:@?#\\") || strings.HasSuffix(value, ".") {
			return fmt.Errorf("%w: invalid restricted domain %q", ErrUnauthorizedNetwork, raw)
		}
		for _, label := range strings.Split(value, ".") {
			if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return fmt.Errorf("%w: invalid restricted domain %q", ErrUnauthorizedNetwork, raw)
			}
			for _, ch := range label {
				if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
					return fmt.Errorf("%w: invalid restricted domain %q", ErrUnauthorizedNetwork, raw)
				}
			}
		}
		key := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := seenDomains[key]; ok {
			return fmt.Errorf("%w: duplicate restricted domain %q", ErrUnauthorizedNetwork, raw)
		}
		seenDomains[key] = struct{}{}
	}
	seenIPs := make(map[netip.Addr]struct{}, len(ips))
	for _, raw := range ips {
		addr, err := netip.ParseAddr(strings.TrimSpace(raw))
		if err != nil || addr.Zone() != "" {
			return fmt.Errorf("%w: invalid restricted IP %q", ErrUnauthorizedNetwork, raw)
		}
		addr = addr.Unmap()
		if !addr.IsValid() || addr.IsUnspecified() || addr.IsMulticast() {
			return fmt.Errorf("%w: invalid restricted IP %q", ErrUnauthorizedNetwork, raw)
		}
		if _, ok := seenIPs[addr]; ok {
			return fmt.Errorf("%w: duplicate restricted IP %q", ErrUnauthorizedNetwork, raw)
		}
		seenIPs[addr] = struct{}{}
	}
	seenPorts := make(map[int]struct{}, len(ports))
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("%w: invalid restricted port %d", ErrUnauthorizedNetwork, port)
		}
		if _, ok := seenPorts[port]; ok {
			return fmt.Errorf("%w: duplicate restricted port %d", ErrUnauthorizedNetwork, port)
		}
		seenPorts[port] = struct{}{}
	}
	seenTransports := make(map[string]struct{}, len(transports))
	for _, raw := range transports {
		transport := strings.ToLower(strings.TrimSpace(raw))
		switch transport {
		case "http", "https", "tcp", "udp", "websocket":
		default:
			return fmt.Errorf("%w: invalid restricted transport %q", ErrUnauthorizedNetwork, raw)
		}
		if _, ok := seenTransports[transport]; ok {
			return fmt.Errorf("%w: duplicate restricted transport %q", ErrUnauthorizedNetwork, raw)
		}
		seenTransports[transport] = struct{}{}
	}
	if maxConnections < 0 || maxConnections > 64 {
		return fmt.Errorf("%w: max_connections must be between 0 and 64; 0 uses the host default", ErrUnauthorizedNetwork)
	}
	return nil
}

func normalizedNetworkMode(policy ServiceNetworkPolicy) string {
	mode := strings.ToLower(strings.TrimSpace(policy.Mode))
	if mode != "" {
		return mode
	}
	switch {
	case policy.LoopbackOnly:
		return "loopback"
	case policy.RequireProxy || len(policy.AllowedDomains) > 0 || len(policy.AllowedIPs) > 0 || len(policy.AllowedPorts) > 0 || len(policy.AllowedTransports) > 0 || policy.AllowHostLoopback || policy.MaxConnections > 0:
		return "restricted"
	case policy.AllowOutbound:
		return "unrestricted"
	default:
		return "none"
	}
}

func prepareNetworkLaunch(policy ServiceNetworkPolicy, executable string, args []string, workingDir, tempDir string, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	return prepareNetworkLaunchWithStateRoot(policy, executable, args, workingDir, tempDir, "", readOnlyRoots...)
}

// prepareNetworkLaunchWithStateRoot is the production launch path. stateRoot is
// host-owned durable storage used by platform sandboxes that need crash-recovery
// metadata (currently Windows AppContainer). Tests and legacy callers may use
// prepareNetworkLaunch, which falls back to a host-only OS temp state directory.
func prepareNetworkLaunchWithStateRoot(policy ServiceNetworkPolicy, executable string, args []string, workingDir, tempDir, stateRoot string, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	if !policy.Enforce {
		return directSandboxLaunch(executable, args, workingDir), nil
	}
	mode := normalizedNetworkMode(policy)
	if err := ValidateNetworkPolicySupport(policy); err != nil {
		return sandboxLaunchPlan{}, err
	}

	// Network access can only proceed when the platform implementation above has
	// proved that the requested boundary is enforceable. No mode is ever
	// downgraded to the host network namespace merely because an OS backend is
	// unavailable.

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
				"--tmpfs", "/", "--dev", "/dev", "--proc", "/proc", "--dir", "/etc",
			}
			// Every enforced Linux mode gets its own network namespace. none and
			// restricted keep that namespace disconnected, loopback only brings lo
			// up, and unrestricted attaches user-mode NAT after bwrap has created
			// the namespace. The host network namespace is never shared.
			wrapped = append(wrapped, "--unshare-net")
			createdDirs := map[string]struct{}{
				"/": {}, "/dev": {}, "/proc": {}, "/etc": {},
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
			if mode == "unrestricted" {
				slirp := firstTrustedLauncher("/usr/bin/slirp4netns", "/bin/slirp4netns")
				if slirp == "" {
					return sandboxLaunchPlan{}, fmt.Errorf("%w: linux unrestricted mode requires trusted slirp4netns", ErrNetworkSandboxUnavailable)
				}
				plan, planErr := prepareLinuxSlirpLaunch(bwrap, wrapped, executable, args, workingDir, slirp)
				if planErr != nil {
					return sandboxLaunchPlan{}, planErr
				}
				return plan, nil
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
		return prepareWindowsAppContainerLaunch(mode, executable, args, workingDir, tempDir, stateRoot, readOnlyRoots...)
	}

	return sandboxLaunchPlan{}, fmt.Errorf("%w on %s for mode %s", ErrNetworkSandboxUnavailable, runtime.GOOS, mode)
}

type bubblewrapInfo struct {
	ChildPID int `json:"child-pid"`
}

// prepareLinuxSlirpLaunch keeps the plugin inside a private bwrap network
// namespace while attaching slirp4netns as an outbound-only user-mode NAT.
// bwrap blocks the sandbox child until slirp reports readiness, eliminating the
// startup race where plugin code could run before the network boundary exists.
func prepareLinuxSlirpLaunch(bwrap string, wrapped []string, executable string, args []string, workingDir, slirp string) (sandboxLaunchPlan, error) {
	resolverFile, err := os.CreateTemp("", "amitia-gamehost-resolv-*.conf")
	if err != nil {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: create slirp resolver config: %v", ErrNetworkSandboxUnavailable, err)
	}
	resolverPath := resolverFile.Name()
	removeResolver := func() { _ = os.Remove(resolverPath) }
	if chmodErr := resolverFile.Chmod(0o600); chmodErr != nil {
		_ = resolverFile.Close()
		removeResolver()
		return sandboxLaunchPlan{}, fmt.Errorf("%w: protect slirp resolver config: %v", ErrNetworkSandboxUnavailable, chmodErr)
	}
	if _, writeErr := resolverFile.WriteString("nameserver 10.0.2.3\noptions ndots:0\n"); writeErr != nil {
		_ = resolverFile.Close()
		removeResolver()
		return sandboxLaunchPlan{}, fmt.Errorf("%w: write slirp resolver config: %v", ErrNetworkSandboxUnavailable, writeErr)
	}
	if closeErr := resolverFile.Close(); closeErr != nil {
		removeResolver()
		return sandboxLaunchPlan{}, fmt.Errorf("%w: close slirp resolver config: %v", ErrNetworkSandboxUnavailable, closeErr)
	}
	wrapped = append(wrapped, "--ro-bind", resolverPath, "/etc/resolv.conf")

	blockRead, blockWrite, err := os.Pipe()
	if err != nil {
		removeResolver()
		return sandboxLaunchPlan{}, fmt.Errorf("%w: create bwrap network block pipe: %v", ErrNetworkSandboxUnavailable, err)
	}
	infoRead, infoWrite, err := os.Pipe()
	if err != nil {
		_ = blockRead.Close()
		_ = blockWrite.Close()
		removeResolver()
		return sandboxLaunchPlan{}, fmt.Errorf("%w: create bwrap info pipe: %v", ErrNetworkSandboxUnavailable, err)
	}

	all := []*os.File{blockRead, blockWrite, infoRead, infoWrite}
	cleanup := func() {
		for _, file := range all {
			if file != nil {
				_ = file.Close()
			}
		}
		removeResolver()
	}

	// ExtraFiles become fd 3 and 4 in bwrap. --block-fd stops the child after
	// namespace setup; --info-fd gives us the namespace-owning child PID.
	wrapped = append(wrapped, "--block-fd", "3", "--info-fd", "4", "--", executable)
	wrapped = append(wrapped, args...)

	plan := sandboxLaunchPlan{
		Path:                  bwrap,
		Args:                  wrapped,
		WorkingDir:            workingDir,
		FilesystemIsolated:    true,
		NetworkPolicyEnforced: true,
		ExtraFiles:            []*os.File{blockRead, infoWrite},
		Cleanup:               cleanup,
	}
	plan.AfterStart = func(ctx context.Context, sandboxCmd *exec.Cmd) error {
		// The child now owns duplicate fd 3/4 handles. Drop the host copies that
		// are not used for coordination so EOF/readiness semantics remain strict.
		_ = blockRead.Close()
		_ = infoWrite.Close()

		var info bubblewrapInfo
		decodeCh := make(chan error, 1)
		go func() {
			decodeCh <- json.NewDecoder(infoRead).Decode(&info)
		}()
		select {
		case decodeErr := <-decodeCh:
			_ = infoRead.Close()
			if decodeErr != nil || info.ChildPID <= 0 {
				if decodeErr == nil {
					decodeErr = errors.New("missing child-pid")
				}
				return fmt.Errorf("%w: read bubblewrap namespace identity: %v", ErrNetworkSandboxUnavailable, decodeErr)
			}
		case <-time.After(10 * time.Second):
			_ = infoRead.Close()
			return fmt.Errorf("%w: timed out waiting for bubblewrap namespace identity", ErrNetworkSandboxUnavailable)
		case <-ctx.Done():
			_ = infoRead.Close()
			return fmt.Errorf("%w: sandbox startup canceled: %v", ErrNetworkSandboxUnavailable, ctx.Err())
		}

		readyRead, readyWrite, pipeErr := os.Pipe()
		if pipeErr != nil {
			return fmt.Errorf("%w: create slirp readiness pipe: %v", ErrNetworkSandboxUnavailable, pipeErr)
		}
		defer readyRead.Close()

		slirpCmd := exec.CommandContext(ctx, slirp,
			"--configure",
			"--cidr=10.0.2.0/24",
			"--mtu=65520",
			"--enable-sandbox",
			"--enable-seccomp",
			"--ready-fd=3",
			fmt.Sprintf("%d", info.ChildPID),
			"tap0",
		)
		slirpCmd.ExtraFiles = []*os.File{readyWrite}
		process.ConfigureProcess(slirpCmd)
		if startErr := slirpCmd.Start(); startErr != nil {
			_ = readyWrite.Close()
			return fmt.Errorf("%w: start slirp4netns: %v", ErrNetworkSandboxUnavailable, startErr)
		}
		_ = readyWrite.Close()

		readyCh := make(chan error, 1)
		go func() {
			var one [1]byte
			_, readErr := readyRead.Read(one[:])
			readyCh <- readErr
		}()
		select {
		case readyErr := <-readyCh:
			if readyErr != nil {
				_ = slirpCmd.Process.Kill()
				_ = slirpCmd.Wait()
				return fmt.Errorf("%w: slirp4netns failed before readiness: %v", ErrNetworkSandboxUnavailable, readyErr)
			}
		case <-time.After(10 * time.Second):
			_ = slirpCmd.Process.Kill()
			_ = slirpCmd.Wait()
			return fmt.Errorf("%w: timed out waiting for slirp4netns readiness", ErrNetworkSandboxUnavailable)
		case <-ctx.Done():
			_ = slirpCmd.Process.Kill()
			_ = slirpCmd.Wait()
			return fmt.Errorf("%w: slirp4netns startup canceled: %v", ErrNetworkSandboxUnavailable, ctx.Err())
		}

		// Wait has exactly one owner. The sidecar is terminated by CommandContext
		// when the plugin lifetime ends, and Pdeathsig also covers abrupt host exit.
		go func() { _ = slirpCmd.Wait() }()

		if _, writeErr := blockWrite.Write([]byte{1}); writeErr != nil {
			return fmt.Errorf("%w: release bubblewrap network barrier: %v", ErrNetworkSandboxUnavailable, writeErr)
		}
		_ = blockWrite.Close()
		return nil
	}
	return plan, nil
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
	if mode != "none" && mode != "loopback" && mode != "restricted" && mode != "unrestricted" {
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
