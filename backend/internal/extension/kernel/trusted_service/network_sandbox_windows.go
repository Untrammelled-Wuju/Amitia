//go:build windows

package trusted_service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/platform/process"
)

type windowsSandboxConfig struct {
	ProfileName  string   `json:"profileName"`
	Executable   string   `json:"executable"`
	CommandLine  string   `json:"commandLine"`
	WorkingDir   string   `json:"workingDir"`
	Writable     []string `json:"writable"`
	ReadOnly     []string `json:"readOnly"`
	Traversal    []string `json:"traversal"`
	Capabilities []string `json:"capabilities"`
	Loopback     bool     `json:"loopback"`
	BlockInbound bool     `json:"blockInbound"`
	FirewallRule string   `json:"firewallRule"`
	Icacls       string   `json:"icacls"`
	CheckNet     string   `json:"checkNet"`
	StateFile    string   `json:"stateFile"`
	MemoryBytes  uint64   `json:"memoryBytes"`
	CPUPercent   uint32   `json:"cpuPercent"`
	ProcessLimit uint32   `json:"processLimit"`
	TemporaryDir string   `json:"temporaryDir"`
}

// prepareWindowsAppContainerLaunch creates an actual AppContainer boundary for
// enforced game services in none/loopback/restricted/unrestricted modes. The trusted
// PowerShell wrapper only exists to call Windows AppContainer APIs; the plugin
// process itself receives the
// inherited stdio handles and runs inside the AppContainer. ACL grants are
// scoped to a deterministic per-service-instance SID and removed when the child exits.
// Durable state lets the next host start recover resources after forced termination.
func prepareWindowsAppContainerLaunch(mode, executable string, args []string, workingDir, tempDir, stateRoot string, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	return prepareWindowsContainerWithLimits(mode, executable, args, workingDir, tempDir, stateRoot, process.ResourceLimits{}, readOnlyRoots...)
}

func prepareWindowsTaskContainerLaunch(executable string, args []string, workingDir, tempDir string, limits process.ResourceLimits, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	return prepareWindowsContainerWithLimits("none", executable, args, workingDir, tempDir, "", limits, readOnlyRoots...)
}

func prepareWindowsContainerWithLimits(mode, executable string, args []string, workingDir, tempDir, stateRoot string, limits process.ResourceLimits, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "none" && mode != "loopback" && mode != "restricted" && mode != "unrestricted" {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: unsupported windows sandbox mode %q", ErrUnauthorizedNetwork, mode)
	}

	systemRootRaw := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRootRaw == "" || !filepath.IsAbs(systemRootRaw) {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: trusted SystemRoot is unavailable", ErrNetworkSandboxUnavailable)
	}
	systemRoot := filepath.Clean(systemRootRaw)
	powershell := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	icacls := filepath.Join(systemRoot, "System32", "icacls.exe")
	checkNet := filepath.Join(systemRoot, "System32", "CheckNetIsolation.exe")
	for _, path := range []string{powershell, icacls} {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return sandboxLaunchPlan{}, fmt.Errorf("%w: trusted Windows sandbox component %q is unavailable", ErrNetworkSandboxUnavailable, path)
		}
	}
	needsLoopbackExemption := mode == "loopback" || mode == "unrestricted"
	if needsLoopbackExemption {
		if info, err := os.Stat(checkNet); err != nil || info.IsDir() {
			return sandboxLaunchPlan{}, fmt.Errorf("%w: CheckNetIsolation.exe is required for Windows %s mode", ErrNetworkSandboxUnavailable, mode)
		}
	}

	work, err := cleanWindowsSandboxPath(workingDir, true)
	if err != nil {
		return sandboxLaunchPlan{}, err
	}
	tmp, err := cleanWindowsSandboxPath(tempDir, true)
	if err != nil {
		return sandboxLaunchPlan{}, err
	}
	exe, err := cleanWindowsSandboxPath(executable, true)
	if err != nil {
		return sandboxLaunchPlan{}, err
	}
	if work == "" {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: Windows sandbox requires an explicit plugin working directory", ErrNetworkSandboxUnavailable)
	}
	profileName := windowsSandboxProfileName(work, tmp)
	stateDir, err := windowsSandboxStateDir(stateRoot)
	if err != nil {
		return sandboxLaunchPlan{}, err
	}
	stateFile := filepath.Join(stateDir, profileName+".json")
	// A previous host may have been force-terminated before the PowerShell
	// wrapper reached its finally block. Recover this deterministic instance's
	// stale AppContainer resources before reusing its identity.
	if _, statErr := os.Stat(stateFile); statErr == nil {
		if recoverErr := recoverWindowsSandboxRecord(stateFile); recoverErr != nil {
			return sandboxLaunchPlan{}, recoverErr
		}
	} else if !os.IsNotExist(statErr) {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: inspect Windows sandbox state %q: %v", ErrNetworkSandboxUnavailable, stateFile, statErr)
	}

	readOnly := make([]string, 0, len(readOnlyRoots)+1)
	seen := make(map[string]struct{})
	for _, root := range append(readOnlyRoots, exe) {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		clean, cleanErr := cleanWindowsSandboxPath(root, true)
		if cleanErr != nil {
			return sandboxLaunchPlan{}, cleanErr
		}
		key := strings.ToLower(clean)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		readOnly = append(readOnly, clean)
	}
	writable := make([]string, 0, 2)
	for _, root := range []string{work, tmp} {
		if root == "" {
			continue
		}
		key := strings.ToLower(root)
		if _, duplicate := seen[key+"|w"]; duplicate {
			continue
		}
		seen[key+"|w"] = struct{}{}
		writable = append(writable, root)
	}

	capabilities := []string(nil)
	blockInbound := false
	if mode == "unrestricted" {
		// AppContainer has no ambient network rights. Grant the two outbound
		// capability families needed to reach public and private networks, then
		// explicitly install a package-scoped inbound block below so the generic
		// ServiceNetworkPolicy invariant (no non-loopback inbound) still holds.
		capabilities = []string{"internetClient", "privateNetworkClientServer"}
		blockInbound = true
	}

	traversal := []string(nil)
	cfg := windowsSandboxConfig{
		ProfileName:  profileName,
		Executable:   exe,
		CommandLine:  windowsBuildCommandLine(exe, args),
		WorkingDir:   work,
		Writable:     writable,
		ReadOnly:     readOnly,
		Traversal:    traversal,
		Capabilities: capabilities,
		Loopback:     needsLoopbackExemption,
		BlockInbound: blockInbound,
		FirewallRule: "AmitiaGameSandbox-" + strings.TrimPrefix(profileName, "amitia.game."),
		Icacls:       icacls,
		CheckNet:     checkNet,
		StateFile:    stateFile,
		MemoryBytes:  limits.MaxMemoryBytes,
		CPUPercent:   limits.MaxCPUPercent,
		ProcessLimit: limits.MaxProcesses,
		TemporaryDir: tmp,
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: encode Windows sandbox launch config: %v", ErrNetworkSandboxUnavailable, err)
	}
	if err := writeWindowsSandboxState(stateFile, payload); err != nil {
		return sandboxLaunchPlan{}, err
	}
	script := strings.ReplaceAll(windowsAppContainerPowerShell, "__AMITIA_CONFIG__", base64.StdEncoding.EncodeToString(payload))
	// The launcher is host-trusted code and must never be placed in the plugin's
	// writable temp directory. An already-running malicious service could watch
	// that directory and race-rewrite the script before PowerShell opens it. Keep
	// the one-shot launcher in the host user's temp area, which the AppContainer
	// is not granted access to, and let the script delete itself immediately.
	file, err := os.CreateTemp("", "amitia-gamehost-sandbox-*.ps1")
	if err != nil {
		_ = os.Remove(stateFile)
		return sandboxLaunchPlan{}, fmt.Errorf("%w: create one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	launcherPath := file.Name()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(launcherPath)
		_ = os.Remove(stateFile)
		return sandboxLaunchPlan{}, fmt.Errorf("%w: protect one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	if _, err = file.WriteString(script); err != nil {
		_ = file.Close()
		_ = os.Remove(launcherPath)
		_ = os.Remove(stateFile)
		return sandboxLaunchPlan{}, fmt.Errorf("%w: write one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(launcherPath)
		_ = os.Remove(stateFile)
		return sandboxLaunchPlan{}, fmt.Errorf("%w: close one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	return sandboxLaunchPlan{
		Path:                  powershell,
		Args:                  []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", launcherPath},
		WorkingDir:            filepath.Join(systemRoot, "System32"),
		FilesystemIsolated:    true,
		NetworkPolicyEnforced: true,
		// PowerShell deletes the launcher as its first action. Cleanup is still
		// required for the failure path where cmd.Start never succeeds, otherwise
		// a security-sensitive one-shot launcher would remain in the host temp dir.
		Cleanup: func() { _ = os.Remove(launcherPath) },
	}, nil
}
func windowsSandboxProfileName(workingDir, tempDir string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(workingDir)) + "\x00" + strings.ToLower(filepath.Clean(tempDir))))
	return "amitia.game." + hex.EncodeToString(sum[:16])
}

func windowsSandboxStateDir(root string) (string, error) {
	root = strings.TrimSpace(root)
	var dir string
	if root == "" {
		dir = filepath.Join(os.TempDir(), "amitia-gamehost-sandbox-state", "windows")
	} else {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("%w: resolve Windows sandbox state root: %v", ErrNetworkSandboxUnavailable, err)
		}
		dir = filepath.Join(filepath.Clean(abs), "sandbox_state", "windows")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("%w: create Windows sandbox state directory: %v", ErrNetworkSandboxUnavailable, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("%w: protect Windows sandbox state directory: %v", ErrNetworkSandboxUnavailable, err)
	}
	return dir, nil
}

func writeWindowsSandboxState(path string, payload []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("%w: write Windows sandbox state: %v", ErrNetworkSandboxUnavailable, err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: protect Windows sandbox state: %v", ErrNetworkSandboxUnavailable, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: commit Windows sandbox state: %v", ErrNetworkSandboxUnavailable, err)
	}
	return nil
}

func recoverPlatformSandboxResidue(rootDir string) error {
	dir, err := windowsSandboxStateDir(rootDir)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("%w: enumerate Windows sandbox state: %v", ErrNetworkSandboxUnavailable, err)
	}
	var failures []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if err := recoverWindowsSandboxRecord(path); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%w: %s", ErrNetworkSandboxUnavailable, strings.Join(failures, "; "))
	}
	return nil
}

func recoverWindowsSandboxRecord(stateFile string) error {
	payload, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%w: read Windows sandbox state %q: %v", ErrNetworkSandboxUnavailable, stateFile, err)
	}
	var cfg windowsSandboxConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return fmt.Errorf("%w: decode Windows sandbox state %q: %v", ErrNetworkSandboxUnavailable, stateFile, err)
	}
	if !strings.HasPrefix(cfg.ProfileName, "amitia.game.") || filepath.Base(stateFile) != cfg.ProfileName+".json" {
		return fmt.Errorf("%w: invalid Windows sandbox state identity %q", ErrNetworkSandboxUnavailable, stateFile)
	}
	systemRootRaw := strings.TrimSpace(os.Getenv("SystemRoot"))
	if systemRootRaw == "" || !filepath.IsAbs(systemRootRaw) {
		return fmt.Errorf("%w: trusted SystemRoot is unavailable during residue recovery", ErrNetworkSandboxUnavailable)
	}
	systemRoot := filepath.Clean(systemRootRaw)
	powershell := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	icacls := filepath.Join(systemRoot, "System32", "icacls.exe")
	checkNet := filepath.Join(systemRoot, "System32", "CheckNetIsolation.exe")
	for _, path := range []string{powershell, icacls} {
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			return fmt.Errorf("%w: trusted Windows recovery component %q is unavailable", ErrNetworkSandboxUnavailable, path)
		}
	}
	cfg.Icacls = icacls
	cfg.CheckNet = checkNet
	cfg.FirewallRule = "AmitiaGameSandbox-" + strings.TrimPrefix(cfg.ProfileName, "amitia.game.")
	cfg.StateFile = stateFile
	trustedPayload, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%w: encode Windows sandbox recovery config: %v", ErrNetworkSandboxUnavailable, err)
	}
	script := strings.ReplaceAll(windowsAppContainerRecoveryPowerShell, "__AMITIA_CONFIG__", base64.StdEncoding.EncodeToString(trustedPayload))
	file, err := os.CreateTemp("", "amitia-gamehost-sandbox-recover-*.ps1")
	if err != nil {
		return fmt.Errorf("%w: create Windows sandbox recovery launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	launcherPath := file.Name()
	defer os.Remove(launcherPath)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("%w: protect Windows sandbox recovery launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		return fmt.Errorf("%w: write Windows sandbox recovery launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("%w: close Windows sandbox recovery launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	cmd := exec.Command(powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", launcherPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: recover Windows sandbox %s: %v: %s", ErrNetworkSandboxUnavailable, cfg.ProfileName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func cleanWindowsSandboxPath(path string, mustExist bool) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: resolve sandbox path %q: %v", ErrNetworkSandboxUnavailable, path, err)
	}
	abs = filepath.Clean(abs)
	volume := filepath.VolumeName(abs)
	if abs == volume+string(filepath.Separator) {
		return "", fmt.Errorf("%w: refusing to grant an AppContainer access to volume root %q", ErrNetworkSandboxUnavailable, abs)
	}
	if mustExist {
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("%w: sandbox path %q is unavailable: %v", ErrNetworkSandboxUnavailable, abs, err)
		}
	}
	return abs, nil
}

func windowsBuildCommandLine(executable string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, windowsQuoteArg(executable))
	for _, arg := range args {
		parts = append(parts, windowsQuoteArg(arg))
	}
	return strings.Join(parts, " ")
}

// windowsQuoteArg follows the CommandLineToArgvW/CRT escaping convention used
// by os/exec, without invoking cmd.exe or a shell.
func windowsQuoteArg(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}
	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for _, r := range arg {
		if r == '\\' {
			backslashes++
			continue
		}
		if r == '"' {
			b.WriteString(strings.Repeat("\\", backslashes*2+1))
			b.WriteRune(r)
			backslashes = 0
			continue
		}
		if backslashes > 0 {
			b.WriteString(strings.Repeat("\\", backslashes))
			backslashes = 0
		}
		b.WriteRune(r)
	}
	if backslashes > 0 {
		b.WriteString(strings.Repeat("\\", backslashes*2))
	}
	b.WriteByte('"')
	return b.String()
}

const windowsTraversalNative = `
public static class AmitiaTraversalAcl {
    [System.Runtime.InteropServices.DllImport("kernel32.dll", CharSet = System.Runtime.InteropServices.CharSet.Unicode, SetLastError = true)]
    static extern System.IntPtr CreateFileW(string path, uint access, uint sharing, System.IntPtr security, uint creation, uint flags, System.IntPtr template);
    [System.Runtime.InteropServices.DllImport("kernel32.dll")]
    static extern bool CloseHandle(System.IntPtr handle);
    [System.Runtime.InteropServices.DllImport("advapi32.dll", SetLastError = true)]
    static extern bool GetKernelObjectSecurity(System.IntPtr handle, uint information, byte[] descriptor, uint length, out uint needed);
    [System.Runtime.InteropServices.DllImport("ntdll.dll")]
    static extern int NtSetSecurityObject(System.IntPtr handle, uint information, byte[] descriptor);
    [System.Runtime.InteropServices.DllImport("ntdll.dll")]
    static extern uint RtlNtStatusToDosError(int status);
    public static void Update(string path, string sidText, bool grant) {
        System.IntPtr handle = CreateFileW(path, 0x20000, 7, System.IntPtr.Zero, 3, 0x2200000, System.IntPtr.Zero);
        if (handle == new System.IntPtr(-1)) throw new System.ComponentModel.Win32Exception(System.Runtime.InteropServices.Marshal.GetLastWin32Error());
        try {
            uint needed;
            GetKernelObjectSecurity(handle, 4, null, 0, out needed);
            if (needed == 0) throw new System.ComponentModel.Win32Exception(System.Runtime.InteropServices.Marshal.GetLastWin32Error());
            byte[] original = new byte[needed];
            if (!GetKernelObjectSecurity(handle, 4, original, needed, out needed)) throw new System.ComponentModel.Win32Exception(System.Runtime.InteropServices.Marshal.GetLastWin32Error());
            var descriptor = new System.Security.AccessControl.RawSecurityDescriptor(original, 0);
            if (descriptor.DiscretionaryAcl == null) throw new System.InvalidOperationException("parent directory has no explicit DACL");
            var sid = new System.Security.Principal.SecurityIdentifier(sidText);
            var acl = descriptor.DiscretionaryAcl;
            bool changed = grant;
            for (int i = acl.Count - 1; i >= 0; i--) {
                var ace = acl[i] as System.Security.AccessControl.CommonAce;
                if (ace != null && ace.SecurityIdentifier.Equals(sid) && ace.AceFlags == System.Security.AccessControl.AceFlags.None && ace.AccessMask == 0xA0 && ace.AceQualifier == System.Security.AccessControl.AceQualifier.AccessAllowed) { acl.RemoveAce(i); changed = true; }
            }
            if (!changed) return;
            if (grant) {
                int index = 0;
                while (index < acl.Count && (acl[index].AceFlags & System.Security.AccessControl.AceFlags.Inherited) == 0) index++;
                acl.InsertAce(index, new System.Security.AccessControl.CommonAce(System.Security.AccessControl.AceFlags.None, System.Security.AccessControl.AceQualifier.AccessAllowed, 0xA0, sid, false, null));
            }
            byte[] updated = new byte[descriptor.BinaryLength];
            descriptor.GetBinaryForm(updated, 0);
            CloseHandle(handle);
            handle = CreateFileW(path, 0x60000, 7, System.IntPtr.Zero, 3, 0x2200000, System.IntPtr.Zero);
            if (handle == new System.IntPtr(-1)) throw new System.ComponentModel.Win32Exception(System.Runtime.InteropServices.Marshal.GetLastWin32Error());
            int status = NtSetSecurityObject(handle, 4, updated);
            if (status < 0) throw new System.ComponentModel.Win32Exception((int)RtlNtStatusToDosError(status));
        } finally { CloseHandle(handle); }
    }
}
`

const windowsAppContainerPowerShell = `$ErrorActionPreference = 'Stop'
$cfg = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('__AMITIA_CONFIG__')) | ConvertFrom-Json
$self = $MyInvocation.MyCommand.Path
if ($self) { Remove-Item -LiteralPath $self -Force -ErrorAction SilentlyContinue }
function ConvertTo-AmitiaExtendedPath([string]$path) {
    if ([string]::IsNullOrWhiteSpace($path) -or $path.StartsWith('\\?\')) { return $path }
    if ($path.StartsWith('\\')) { return '\\?\UNC\' + $path.TrimStart('\') }
    return '\\?\' + $path
}
$native = @'
using System;
using System.Collections.Generic;
using System.Collections;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Text;

public static class AmitiaAppContainer {
    const int ERROR_ALREADY_EXISTS_HR = unchecked((int)0x800700B7);
    const uint EXTENDED_STARTUPINFO_PRESENT = 0x00080000;
    const uint CREATE_UNICODE_ENVIRONMENT = 0x00000400;
    const uint CREATE_NO_WINDOW = 0x08000000;
    const uint CREATE_SUSPENDED = 0x00000004;
    const int STARTF_USESTDHANDLES = 0x00000100;
    const int STD_INPUT_HANDLE = -10;
    const int STD_OUTPUT_HANDLE = -11;
    const int STD_ERROR_HANDLE = -12;
    const uint HANDLE_FLAG_INHERIT = 0x00000001;
    const uint SE_GROUP_ENABLED = 0x00000004;
    static readonly IntPtr PROC_THREAD_ATTRIBUTE_HANDLE_LIST = (IntPtr)0x00020002;
    static readonly IntPtr PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES = (IntPtr)0x00020009;

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct STARTUPINFO {
        public int cb; public string lpReserved; public string lpDesktop; public string lpTitle;
        public int dwX; public int dwY; public int dwXSize; public int dwYSize;
        public int dwXCountChars; public int dwYCountChars; public int dwFillAttribute;
        public int dwFlags; public short wShowWindow; public short cbReserved2;
        public IntPtr lpReserved2; public IntPtr hStdInput; public IntPtr hStdOutput; public IntPtr hStdError;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct STARTUPINFOEX { public STARTUPINFO StartupInfo; public IntPtr lpAttributeList; }
    [StructLayout(LayoutKind.Sequential)]
    struct PROCESS_INFORMATION { public IntPtr hProcess; public IntPtr hThread; public int dwProcessId; public int dwThreadId; }
    [StructLayout(LayoutKind.Sequential)]
    struct SID_AND_ATTRIBUTES { public IntPtr Sid; public uint Attributes; }
    [StructLayout(LayoutKind.Sequential)]
    struct SECURITY_CAPABILITIES { public IntPtr AppContainerSid; public IntPtr Capabilities; public uint CapabilityCount; public uint Reserved; }
    [StructLayout(LayoutKind.Sequential)]
    struct BASIC_LIMITS {
        public long PerProcessUserTimeLimit, PerJobUserTimeLimit;
        public uint LimitFlags;
        public UIntPtr MinimumWorkingSetSize, MaximumWorkingSetSize;
        public uint ActiveProcessLimit;
        public UIntPtr Affinity;
        public uint PriorityClass, SchedulingClass;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct IO_COUNTERS { public ulong ReadOperationCount, WriteOperationCount, OtherOperationCount, ReadTransferCount, WriteTransferCount, OtherTransferCount; }
    [StructLayout(LayoutKind.Sequential)]
    struct EXTENDED_LIMITS {
        public BASIC_LIMITS BasicLimitInformation;
        public IO_COUNTERS IoInfo;
        public UIntPtr ProcessMemoryLimit, JobMemoryLimit, PeakProcessMemoryUsed, PeakJobMemoryUsed;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct CPU_LIMITS { public uint ControlFlags, CPURate; }

    [DllImport("userenv.dll", CharSet = CharSet.Unicode)]
    static extern int CreateAppContainerProfile(string name, string displayName, string description, IntPtr capabilities, uint capabilityCount, out IntPtr appContainerSid);
    [DllImport("userenv.dll", CharSet = CharSet.Unicode)]
    static extern int DeriveAppContainerSidFromAppContainerName(string name, out IntPtr appContainerSid);
    [DllImport("userenv.dll", CharSet = CharSet.Unicode)]
    public static extern int DeleteAppContainerProfile(string name);
    [DllImport("kernelbase.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool DeriveCapabilitySidsFromName(string capabilityName, out IntPtr capabilityGroupSids, out uint capabilityGroupSidCount, out IntPtr capabilitySids, out uint capabilitySidCount);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool ConvertSidToStringSid(IntPtr sid, out IntPtr stringSid);
    [DllImport("advapi32.dll", SetLastError = true)]
    static extern IntPtr FreeSid(IntPtr sid);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr LocalFree(IntPtr hMem);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool InitializeProcThreadAttributeList(IntPtr list, int count, int flags, ref IntPtr size);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool UpdateProcThreadAttribute(IntPtr list, uint flags, IntPtr attribute, IntPtr value, IntPtr size, IntPtr previous, IntPtr returnSize);
    [DllImport("kernel32.dll")]
    static extern void DeleteProcThreadAttributeList(IntPtr list);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool CreateProcessW(string applicationName, StringBuilder commandLine, IntPtr processAttributes, IntPtr threadAttributes, bool inheritHandles, uint creationFlags, IntPtr environment, string currentDirectory, ref STARTUPINFOEX startupInfo, out PROCESS_INFORMATION processInformation);
    [DllImport("kernel32.dll")]
    static extern IntPtr GetStdHandle(int stdHandle);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetHandleInformation(IntPtr handle, out uint flags);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetHandleInformation(IntPtr handle, uint mask, uint flags);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern uint WaitForSingleObject(IntPtr handle, uint milliseconds);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool GetExitCodeProcess(IntPtr process, out uint exitCode);
    [DllImport("kernel32.dll")]
    static extern bool CloseHandle(IntPtr handle);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern IntPtr CreateJobObjectW(IntPtr attributes, string name);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool SetInformationJobObject(IntPtr job, int informationClass, IntPtr information, uint size);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern uint ResumeThread(IntPtr thread);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern bool TerminateProcess(IntPtr process, uint exitCode);

    static void SetJobInfo(IntPtr job, int kind, object value) {
        int size = Marshal.SizeOf(value);
        IntPtr buffer = Marshal.AllocHGlobal(size);
        try {
            Marshal.StructureToPtr(value, buffer, false);
            if (!SetInformationJobObject(job, kind, buffer, (uint)size)) throw new Win32Exception(Marshal.GetLastWin32Error());
        } finally { Marshal.FreeHGlobal(buffer); }
    }

    static IntPtr CreateTaskJob(ulong memoryBytes, uint cpuPercent, uint processLimit) {
        if (memoryBytes == 0 && cpuPercent == 0 && processLimit == 0) return IntPtr.Zero;
        if (memoryBytes == 0 || memoryBytes > 536870912UL || cpuPercent == 0 || cpuPercent > 50 || processLimit != 1)
            throw new InvalidOperationException("invalid task resource budget");
        IntPtr job = CreateJobObjectW(IntPtr.Zero, null);
        if (job == IntPtr.Zero) throw new Win32Exception(Marshal.GetLastWin32Error());
        try {
            EXTENDED_LIMITS limits = new EXTENDED_LIMITS();
            limits.BasicLimitInformation.LimitFlags = 0x00002000 | 0x00000400 | 0x00000200 | 0x00000008;
            limits.BasicLimitInformation.ActiveProcessLimit = processLimit;
            limits.JobMemoryLimit = new UIntPtr(memoryBytes);
            SetJobInfo(job, 9, limits);
            SetJobInfo(job, 15, new CPU_LIMITS { ControlFlags = 5, CPURate = cpuPercent * 100 });
            return job;
        } catch { CloseHandle(job); throw; }
    }

    sealed class CapabilityAllocation : IDisposable {
        public readonly List<IntPtr> SidPointers = new List<IntPtr>();
        public readonly List<IntPtr> Arrays = new List<IntPtr>();
        public IntPtr AttributeBuffer = IntPtr.Zero;
        public uint Count = 0;
        public void Dispose() {
            if (AttributeBuffer != IntPtr.Zero) Marshal.FreeHGlobal(AttributeBuffer);
            foreach (IntPtr sid in SidPointers) if (sid != IntPtr.Zero) LocalFree(sid);
            foreach (IntPtr array in Arrays) if (array != IntPtr.Zero) LocalFree(array);
        }
    }

    static IntPtr Derive(string name) {
        IntPtr sid;
        int hr = DeriveAppContainerSidFromAppContainerName(name, out sid);
        if (hr < 0) Marshal.ThrowExceptionForHR(hr);
        return sid;
    }

    public static string EnsureProfile(string name) {
        IntPtr sid;
        int hr = CreateAppContainerProfile(name, name, "Amitia isolated game plugin", IntPtr.Zero, 0, out sid);
        if (hr == ERROR_ALREADY_EXISTS_HR) sid = Derive(name);
        else if (hr < 0) Marshal.ThrowExceptionForHR(hr);
        try {
            IntPtr text;
            if (!ConvertSidToStringSid(sid, out text)) throw new Win32Exception(Marshal.GetLastWin32Error());
            try { return Marshal.PtrToStringUni(text); }
            finally { LocalFree(text); }
        } finally { FreeSid(sid); }
    }

    static CapabilityAllocation BuildCapabilities(string[] names) {
        CapabilityAllocation result = new CapabilityAllocation();
        try {
            if (names == null || names.Length == 0) return result;
            List<IntPtr> appSids = new List<IntPtr>();
            foreach (string name in names) {
                if (String.IsNullOrWhiteSpace(name)) continue;
                IntPtr groups, caps;
                uint groupCount, capCount;
                if (!DeriveCapabilitySidsFromName(name, out groups, out groupCount, out caps, out capCount))
                    throw new Win32Exception(Marshal.GetLastWin32Error(), "derive capability " + name);
                if (groups != IntPtr.Zero) {
                    result.Arrays.Add(groups);
                    for (uint i = 0; i < groupCount; i++) {
                        IntPtr sid = Marshal.ReadIntPtr(groups, checked((int)(i * (uint)IntPtr.Size)));
                        if (sid != IntPtr.Zero) result.SidPointers.Add(sid);
                    }
                }
                if (caps != IntPtr.Zero) {
                    result.Arrays.Add(caps);
                    for (uint i = 0; i < capCount; i++) {
                        IntPtr sid = Marshal.ReadIntPtr(caps, checked((int)(i * (uint)IntPtr.Size)));
                        if (sid != IntPtr.Zero) {
                            result.SidPointers.Add(sid);
                            appSids.Add(sid);
                        }
                    }
                }
                if (capCount == 0 || caps == IntPtr.Zero)
                    throw new InvalidOperationException("capability produced no AppContainer SID: " + name);
            }
            result.Count = (uint)appSids.Count;
            if (result.Count == 0) return result;
            int stride = Marshal.SizeOf(typeof(SID_AND_ATTRIBUTES));
            result.AttributeBuffer = Marshal.AllocHGlobal(checked(stride * appSids.Count));
            for (int i = 0; i < appSids.Count; i++) {
                SID_AND_ATTRIBUTES item = new SID_AND_ATTRIBUTES { Sid = appSids[i], Attributes = SE_GROUP_ENABLED };
                Marshal.StructureToPtr(item, IntPtr.Add(result.AttributeBuffer, i * stride), false);
            }
            return result;
        } catch {
            result.Dispose();
            throw;
        }
    }
    static IntPtr[] UniqueStdHandles(STARTUPINFO si) {
        List<IntPtr> handles = new List<IntPtr>();
        foreach (IntPtr handle in new [] { si.hStdInput, si.hStdOutput, si.hStdError }) {
            if (handle == IntPtr.Zero || handle == new IntPtr(-1) || handles.Contains(handle)) continue;
            handles.Add(handle);
        }
        if (handles.Count == 0) throw new InvalidOperationException("no valid stdio handles available for sandbox child");
        return handles.ToArray();
    }

    public static int Run(string profileName, string application, string commandLine, string currentDirectory, string[] capabilityNames, ulong memoryBytes, uint cpuPercent, uint processLimit, string temporaryDir) {
        IntPtr sid = IntPtr.Zero, attrs = IntPtr.Zero, securityPtr = IntPtr.Zero, handleBuffer = IntPtr.Zero;
        IntPtr environmentPtr = IntPtr.Zero;
        IntPtr job = IntPtr.Zero;
        PROCESS_INFORMATION pi = new PROCESS_INFORMATION();
        CapabilityAllocation capabilityAllocation = null;
        IntPtr[] inheritedHandles = null;
        uint[] oldHandleFlags = null;
        try {
            sid = Derive(profileName);
            capabilityAllocation = BuildCapabilities(capabilityNames);
            IntPtr size = IntPtr.Zero;
            InitializeProcThreadAttributeList(IntPtr.Zero, 2, 0, ref size);
            attrs = Marshal.AllocHGlobal(size);
            if (!InitializeProcThreadAttributeList(attrs, 2, 0, ref size)) throw new Win32Exception(Marshal.GetLastWin32Error());
            SECURITY_CAPABILITIES security = new SECURITY_CAPABILITIES {
                AppContainerSid = sid,
                Capabilities = capabilityAllocation.AttributeBuffer,
                CapabilityCount = capabilityAllocation.Count,
                Reserved = 0
            };
            securityPtr = Marshal.AllocHGlobal(Marshal.SizeOf(typeof(SECURITY_CAPABILITIES)));
            Marshal.StructureToPtr(security, securityPtr, false);
            if (!UpdateProcThreadAttribute(attrs, 0, PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES, securityPtr, (IntPtr)Marshal.SizeOf(typeof(SECURITY_CAPABILITIES)), IntPtr.Zero, IntPtr.Zero))
                throw new Win32Exception(Marshal.GetLastWin32Error());

            STARTUPINFOEX si = new STARTUPINFOEX();
            si.StartupInfo.cb = Marshal.SizeOf(typeof(STARTUPINFOEX));
            si.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
            si.StartupInfo.hStdInput = GetStdHandle(STD_INPUT_HANDLE);
            si.StartupInfo.hStdOutput = GetStdHandle(STD_OUTPUT_HANDLE);
            si.StartupInfo.hStdError = GetStdHandle(STD_ERROR_HANDLE);
            inheritedHandles = UniqueStdHandles(si.StartupInfo);
            oldHandleFlags = new uint[inheritedHandles.Length];
            for (int i = 0; i < inheritedHandles.Length; i++) {
                uint flags;
                if (!GetHandleInformation(inheritedHandles[i], out flags)) throw new Win32Exception(Marshal.GetLastWin32Error());
                oldHandleFlags[i] = flags;
                if (!SetHandleInformation(inheritedHandles[i], HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT)) throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            handleBuffer = Marshal.AllocHGlobal(checked(IntPtr.Size * inheritedHandles.Length));
            for (int i = 0; i < inheritedHandles.Length; i++) Marshal.WriteIntPtr(handleBuffer, i * IntPtr.Size, inheritedHandles[i]);
            if (!UpdateProcThreadAttribute(attrs, 0, PROC_THREAD_ATTRIBUTE_HANDLE_LIST, handleBuffer, (IntPtr)(IntPtr.Size * inheritedHandles.Length), IntPtr.Zero, IntPtr.Zero))
                throw new Win32Exception(Marshal.GetLastWin32Error());

            si.lpAttributeList = attrs;
            if (processLimit != 0) {
                SortedDictionary<string,string> env = new SortedDictionary<string,string>(StringComparer.OrdinalIgnoreCase);
                foreach (DictionaryEntry entry in Environment.GetEnvironmentVariables()) env[(string)entry.Key] = (string)entry.Value;
                env["TEMP"] = temporaryDir;
                env["TMP"] = temporaryDir;
                env["HOME"] = currentDirectory;
                env["USERPROFILE"] = currentDirectory;
                env["LOCALAPPDATA"] = temporaryDir;
                env["APPDATA"] = temporaryDir;
                env["PATH"] = Environment.GetFolderPath(Environment.SpecialFolder.System);
                StringBuilder block = new StringBuilder();
                foreach (KeyValuePair<string,string> entry in env) block.Append(entry.Key).Append('=').Append(entry.Value).Append('\0');
                block.Append('\0');
                environmentPtr = Marshal.StringToHGlobalUni(block.ToString());
            }
            bool ok = CreateProcessW(application, new StringBuilder(commandLine), IntPtr.Zero, IntPtr.Zero, true,
                EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT | CREATE_NO_WINDOW | CREATE_SUSPENDED,
                environmentPtr, currentDirectory, ref si, out pi);
            if (!ok) throw new Win32Exception(Marshal.GetLastWin32Error());
            job = CreateTaskJob(memoryBytes, cpuPercent, processLimit);
            if (job != IntPtr.Zero && !AssignProcessToJobObject(job, pi.hProcess)) throw new Win32Exception(Marshal.GetLastWin32Error());
            if (ResumeThread(pi.hThread) == 0xFFFFFFFF) throw new Win32Exception(Marshal.GetLastWin32Error());
            WaitForSingleObject(pi.hProcess, 0xFFFFFFFF);
            uint exitCode;
            if (!GetExitCodeProcess(pi.hProcess, out exitCode)) throw new Win32Exception(Marshal.GetLastWin32Error());
            return unchecked((int)exitCode);
        } finally {
            if (job != IntPtr.Zero) CloseHandle(job);
            if (pi.hProcess != IntPtr.Zero) TerminateProcess(pi.hProcess, 125);
            if (inheritedHandles != null && oldHandleFlags != null) {
                for (int i = 0; i < inheritedHandles.Length; i++) {
                    try { SetHandleInformation(inheritedHandles[i], HANDLE_FLAG_INHERIT, oldHandleFlags[i] & HANDLE_FLAG_INHERIT); } catch { }
                }
            }
            if (pi.hThread != IntPtr.Zero) CloseHandle(pi.hThread);
            if (pi.hProcess != IntPtr.Zero) CloseHandle(pi.hProcess);
            if (attrs != IntPtr.Zero) { DeleteProcThreadAttributeList(attrs); Marshal.FreeHGlobal(attrs); }
            if (handleBuffer != IntPtr.Zero) Marshal.FreeHGlobal(handleBuffer);
            if (securityPtr != IntPtr.Zero) Marshal.FreeHGlobal(securityPtr);
            if (environmentPtr != IntPtr.Zero) Marshal.FreeHGlobal(environmentPtr);
            if (capabilityAllocation != null) capabilityAllocation.Dispose();
            if (sid != IntPtr.Zero) FreeSid(sid);
        }
    }
}
'@
Add-Type -TypeDefinition ($native + @'
` + windowsTraversalNative + `
'@) -Language CSharp
$created = $false
$loopback = $false
$firewall = $false
$sid = $null
$code = 125
$cleanupOk = $true
try {
    $sid = [AmitiaAppContainer]::EnsureProfile([string]$cfg.profileName)
    $created = $true
    foreach ($path in @($cfg.traversal)) {
        if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
        $aclPath = ConvertTo-AmitiaExtendedPath ([string]$path)
        [AmitiaTraversalAcl]::Update($aclPath, [string]$sid, $true)
    }
    foreach ($path in @($cfg.readOnly)) {
        if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
        $item = Get-Item -LiteralPath ([string]$path) -Force
        $aclPath = ConvertTo-AmitiaExtendedPath ([string]$path)
        if ($item.PSIsContainer) {
            & $cfg.icacls $aclPath /grant "*$($sid):(OI)(CI)RX" /T /C /Q /L | Out-Null
        } else {
            & $cfg.icacls $aclPath /grant "*$($sid):RX" /C /Q /L | Out-Null
        }
        if ($LASTEXITCODE -ne 0) { throw "icacls read grant failed for $path (exit $LASTEXITCODE)" }
    }
    foreach ($path in @($cfg.writable)) {
        if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
        $aclPath = ConvertTo-AmitiaExtendedPath ([string]$path)
        & $cfg.icacls $aclPath /grant "*$($sid):(OI)(CI)M" /T /C /Q /L | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "icacls write grant failed for $path (exit $LASTEXITCODE)" }
    }
    if ([bool]$cfg.loopback) {
        & $cfg.checkNet LoopbackExempt -a "-n=$($cfg.profileName)" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "unable to add AppContainer loopback exemption (exit $LASTEXITCODE)" }
        $loopback = $true
    }
    if ([bool]$cfg.blockInbound) {
        New-NetFirewallRule -DisplayName ([string]$cfg.firewallRule) -Direction Inbound -Action Block -Enabled True -Profile Any -Package ([string]$sid) -ErrorAction Stop | Out-Null
        $firewall = $true
    }
    $caps = @($cfg.capabilities | ForEach-Object { [string]$_ })
    $code = [AmitiaAppContainer]::Run([string]$cfg.profileName, [string]$cfg.executable, [string]$cfg.commandLine, [string]$cfg.workingDir, $caps, [uint64]$cfg.memoryBytes, [uint32]$cfg.cpuPercent, [uint32]$cfg.processLimit, [string]$cfg.temporaryDir)
} finally {
    if ($firewall) {
        try { Remove-NetFirewallRule -DisplayName ([string]$cfg.firewallRule) -ErrorAction Stop } catch { $cleanupOk = $false }
    }
    if ($loopback) {
        & $cfg.checkNet LoopbackExempt -d "-n=$($cfg.profileName)" | Out-Null
        if ($LASTEXITCODE -ne 0) { $cleanupOk = $false }
    }
    if ($sid) {
        foreach ($path in @($cfg.traversal)) {
            if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
            $aclPath = ConvertTo-AmitiaExtendedPath ([string]$path)
            try { [AmitiaTraversalAcl]::Update($aclPath, [string]$sid, $false) } catch { $cleanupOk = $false }
        }
        foreach ($path in @($cfg.writable) + @($cfg.readOnly)) {
            if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
            $item = Get-Item -LiteralPath ([string]$path) -Force -ErrorAction SilentlyContinue
            if (-not $item) { continue }
            $aclPath = ConvertTo-AmitiaExtendedPath ([string]$path)
            if ($item.PSIsContainer) {
                & $cfg.icacls $aclPath /remove:g "*$sid" /T /C /Q /L | Out-Null
            } else {
                & $cfg.icacls $aclPath /remove:g "*$sid" /C /Q | Out-Null
            }
            if ($LASTEXITCODE -ne 0) { $cleanupOk = $false }
        }
    }
    if ($created) {
        $deleteResult = [AmitiaAppContainer]::DeleteAppContainerProfile([string]$cfg.profileName)
        if ($deleteResult -lt 0) { $cleanupOk = $false }
    }
    if ($cleanupOk -and -not [string]::IsNullOrWhiteSpace([string]$cfg.stateFile)) {
        Remove-Item -LiteralPath ([string]$cfg.stateFile) -Force -ErrorAction SilentlyContinue
    }
}
if (-not $cleanupOk) { exit 125 }
exit $code
`

const windowsAppContainerRecoveryPowerShell = `$ErrorActionPreference = 'Stop'
$cfg = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('__AMITIA_CONFIG__')) | ConvertFrom-Json
$self = $MyInvocation.MyCommand.Path
if ($self) { Remove-Item -LiteralPath $self -Force -ErrorAction SilentlyContinue }
function ConvertTo-AmitiaExtendedPath([string]$path) {
    if ([string]::IsNullOrWhiteSpace($path) -or $path.StartsWith('\\?\')) { return $path }
    if ($path.StartsWith('\\')) { return '\\?\UNC\' + $path.TrimStart('\') }
    return '\\?\' + $path
}
$native = @'
using System;
using System.Runtime.InteropServices;

public static class AmitiaAppContainerRecovery {
    [DllImport("userenv.dll", CharSet = CharSet.Unicode)]
    static extern int DeriveAppContainerSidFromAppContainerName(string name, out IntPtr appContainerSid);
    [DllImport("userenv.dll", CharSet = CharSet.Unicode)]
    public static extern int DeleteAppContainerProfile(string name);
    [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool ConvertSidToStringSid(IntPtr sid, out IntPtr stringSid);
    [DllImport("advapi32.dll", SetLastError = true)]
    static extern IntPtr FreeSid(IntPtr sid);
    [DllImport("kernel32.dll", SetLastError = true)]
    static extern IntPtr LocalFree(IntPtr hMem);

    public static string TrySid(string name) {
        IntPtr sid;
        int hr = DeriveAppContainerSidFromAppContainerName(name, out sid);
        if (hr < 0 || sid == IntPtr.Zero) return null;
        try {
            IntPtr text;
            if (!ConvertSidToStringSid(sid, out text)) return null;
            try { return Marshal.PtrToStringUni(text); }
            finally { LocalFree(text); }
        } finally { FreeSid(sid); }
    }
}
'@
Add-Type -TypeDefinition ($native + @'
` + windowsTraversalNative + `
'@) -Language CSharp
$cleanupOk = $true
$sid = [AmitiaAppContainerRecovery]::TrySid([string]$cfg.profileName)
try { Remove-NetFirewallRule -DisplayName ([string]$cfg.firewallRule) -ErrorAction SilentlyContinue } catch { $cleanupOk = $false }
if ([bool]$cfg.loopback -and (Test-Path -LiteralPath ([string]$cfg.checkNet))) {
    & $cfg.checkNet LoopbackExempt -d "-n=$($cfg.profileName)" | Out-Null
}
if ($sid) {
    foreach ($path in @($cfg.traversal)) {
        if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
        $aclPath = ConvertTo-AmitiaExtendedPath ([string]$path)
        try { [AmitiaTraversalAcl]::Update($aclPath, [string]$sid, $false) } catch { $cleanupOk = $false }
    }
    foreach ($path in @($cfg.writable) + @($cfg.readOnly)) {
        if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
            $item = Get-Item -LiteralPath ([string]$path) -Force -ErrorAction SilentlyContinue
            if (-not $item) { continue }
            $aclPath = ConvertTo-AmitiaExtendedPath ([string]$path)
            if ($item.PSIsContainer) {
                & $cfg.icacls $aclPath /remove:g "*$sid" /T /C /Q /L | Out-Null
            } else {
                & $cfg.icacls $aclPath /remove:g "*$sid" /C /Q | Out-Null
            }
        if ($LASTEXITCODE -ne 0) { $cleanupOk = $false }
    }
    $deleteResult = [AmitiaAppContainerRecovery]::DeleteAppContainerProfile([string]$cfg.profileName)
    if ($deleteResult -lt 0) { $cleanupOk = $false }
}
if ($cleanupOk) {
    Remove-Item -LiteralPath ([string]$cfg.stateFile) -Force -ErrorAction SilentlyContinue
    exit 0
}
exit 125
`
