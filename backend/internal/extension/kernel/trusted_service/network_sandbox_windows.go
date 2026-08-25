//go:build windows

package trusted_service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type windowsSandboxConfig struct {
	ProfileName  string   `json:"profileName"`
	Executable   string   `json:"executable"`
	CommandLine  string   `json:"commandLine"`
	WorkingDir   string   `json:"workingDir"`
	Writable     []string `json:"writable"`
	ReadOnly     []string `json:"readOnly"`
	Capabilities []string `json:"capabilities"`
	Loopback     bool     `json:"loopback"`
	BlockInbound bool     `json:"blockInbound"`
	FirewallRule string   `json:"firewallRule"`
	Icacls       string   `json:"icacls"`
	CheckNet     string   `json:"checkNet"`
}

// prepareWindowsAppContainerLaunch creates an actual AppContainer boundary for
// enforced game services in none/loopback/unrestricted modes. The trusted
// PowerShell wrapper only exists to call Windows AppContainer APIs; the plugin
// process itself receives the
// inherited stdio handles and runs inside the AppContainer. ACL grants are
// scoped to a random per-launch SID and removed when the child exits.
func prepareWindowsAppContainerLaunch(mode, executable string, args []string, workingDir, tempDir string, readOnlyRoots ...string) (sandboxLaunchPlan, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "none" && mode != "loopback" && mode != "unrestricted" {
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

	profileName, err := randomWindowsSandboxProfileName()
	if err != nil {
		return sandboxLaunchPlan{}, err
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

	cfg := windowsSandboxConfig{
		ProfileName:  profileName,
		Executable:   exe,
		CommandLine:  windowsBuildCommandLine(exe, args),
		WorkingDir:   work,
		Writable:     writable,
		ReadOnly:     readOnly,
		Capabilities: capabilities,
		Loopback:     needsLoopbackExemption,
		BlockInbound: blockInbound,
		FirewallRule: "AmitiaGameSandbox-" + strings.TrimPrefix(profileName, "amitia.game."),
		Icacls:       icacls,
		CheckNet:     checkNet,
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: encode Windows sandbox launch config: %v", ErrNetworkSandboxUnavailable, err)
	}
	script := strings.ReplaceAll(windowsAppContainerPowerShell, "__AMITIA_CONFIG__", base64.StdEncoding.EncodeToString(payload))
	// The launcher is host-trusted code and must never be placed in the plugin's
	// writable temp directory. An already-running malicious service could watch
	// that directory and race-rewrite the script before PowerShell opens it. Keep
	// the one-shot launcher in the host user's temp area, which the AppContainer
	// is not granted access to, and let the script delete itself immediately.
	file, err := os.CreateTemp("", "amitia-gamehost-sandbox-*.ps1")
	if err != nil {
		return sandboxLaunchPlan{}, fmt.Errorf("%w: create one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	launcherPath := file.Name()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(launcherPath)
		return sandboxLaunchPlan{}, fmt.Errorf("%w: protect one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	if _, err = file.WriteString(script); err != nil {
		_ = file.Close()
		_ = os.Remove(launcherPath)
		return sandboxLaunchPlan{}, fmt.Errorf("%w: write one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(launcherPath)
		return sandboxLaunchPlan{}, fmt.Errorf("%w: close one-shot Windows sandbox launcher: %v", ErrNetworkSandboxUnavailable, err)
	}
	return sandboxLaunchPlan{
		Path:                  powershell,
		Args:                  []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", launcherPath},
		WorkingDir:            filepath.Join(systemRoot, "System32"),
		FilesystemIsolated:    true,
		NetworkPolicyEnforced: true,
	}, nil
}
func randomWindowsSandboxProfileName() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("%w: generate AppContainer profile id: %v", ErrNetworkSandboxUnavailable, err)
	}
	return "amitia.game." + hex.EncodeToString(id[:]), nil
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

const windowsAppContainerPowerShell = `$ErrorActionPreference = 'Stop'
$cfg = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('__AMITIA_CONFIG__')) | ConvertFrom-Json
$self = $MyInvocation.MyCommand.Path
if ($self) { Remove-Item -LiteralPath $self -Force -ErrorAction SilentlyContinue }
$native = @'
using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Text;

public static class AmitiaAppContainer {
    const int ERROR_ALREADY_EXISTS_HR = unchecked((int)0x800700B7);
    const uint EXTENDED_STARTUPINFO_PRESENT = 0x00080000;
    const uint CREATE_UNICODE_ENVIRONMENT = 0x00000400;
    const uint CREATE_NO_WINDOW = 0x08000000;
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

    public static int Run(string profileName, string application, string commandLine, string currentDirectory, string[] capabilityNames) {
        IntPtr sid = IntPtr.Zero, attrs = IntPtr.Zero, securityPtr = IntPtr.Zero, handleBuffer = IntPtr.Zero;
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
            bool ok = CreateProcessW(application, new StringBuilder(commandLine), IntPtr.Zero, IntPtr.Zero, true,
                EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT | CREATE_NO_WINDOW,
                IntPtr.Zero, currentDirectory, ref si, out pi);
            if (!ok) throw new Win32Exception(Marshal.GetLastWin32Error());
            WaitForSingleObject(pi.hProcess, 0xFFFFFFFF);
            uint exitCode;
            if (!GetExitCodeProcess(pi.hProcess, out exitCode)) throw new Win32Exception(Marshal.GetLastWin32Error());
            return unchecked((int)exitCode);
        } finally {
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
            if (capabilityAllocation != null) capabilityAllocation.Dispose();
            if (sid != IntPtr.Zero) FreeSid(sid);
        }
    }
}
'@
Add-Type -TypeDefinition $native -Language CSharp
$created = $false
$loopback = $false
$firewall = $false
$sid = $null
try {
    $sid = [AmitiaAppContainer]::EnsureProfile([string]$cfg.profileName)
    $created = $true
    foreach ($path in @($cfg.readOnly)) {
        if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
        $item = Get-Item -LiteralPath ([string]$path) -Force
        if ($item.PSIsContainer) {
            & $cfg.icacls ([string]$path) /grant "*$($sid):(OI)(CI)RX" /T /C /Q | Out-Null
        } else {
            & $cfg.icacls ([string]$path) /grant "*$($sid):RX" /C /Q | Out-Null
        }
        if ($LASTEXITCODE -ne 0) { throw "icacls read grant failed for $path (exit $LASTEXITCODE)" }
    }
    foreach ($path in @($cfg.writable)) {
        if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
        & $cfg.icacls ([string]$path) /grant "*$($sid):(OI)(CI)M" /T /C /Q | Out-Null
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
    $code = [AmitiaAppContainer]::Run([string]$cfg.profileName, [string]$cfg.executable, [string]$cfg.commandLine, [string]$cfg.workingDir, $caps)
    exit $code
} finally {
    if ($firewall) { Remove-NetFirewallRule -DisplayName ([string]$cfg.firewallRule) -ErrorAction SilentlyContinue }
    if ($loopback) { & $cfg.checkNet LoopbackExempt -d "-n=$($cfg.profileName)" | Out-Null }
    if ($sid) {
        foreach ($path in @($cfg.writable) + @($cfg.readOnly)) {
            if ([string]::IsNullOrWhiteSpace([string]$path)) { continue }
            try {
                $item = Get-Item -LiteralPath ([string]$path) -Force -ErrorAction SilentlyContinue
                if ($item -and $item.PSIsContainer) {
                    & $cfg.icacls ([string]$path) /remove:g "*$sid" /T /C /Q | Out-Null
                } else {
                    & $cfg.icacls ([string]$path) /remove:g "*$sid" /C /Q | Out-Null
                }
            } catch { }
        }
    }
    if ($created) { [AmitiaAppContainer]::DeleteAppContainerProfile([string]$cfg.profileName) | Out-Null }
}
`
