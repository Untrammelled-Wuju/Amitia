package trusted_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"
)

type ServiceState string

const (
	ServiceStateRegistered  ServiceState = "registered"
	ServiceStateValidating  ServiceState = "validating"
	ServiceStateStarting    ServiceState = "starting"
	ServiceStateReady       ServiceState = "ready"
	ServiceStateDegraded    ServiceState = "degraded"
	ServiceStateStopping    ServiceState = "stopping"
	ServiceStateStopped     ServiceState = "stopped"
	ServiceStateCrashed     ServiceState = "crashed"
	ServiceStateQuarantined ServiceState = "quarantined"
	ServiceStateFailed      ServiceState = "failed"
)

func (s ServiceState) IsTerminal() bool {
	switch s {
	case ServiceStateStopped, ServiceStateFailed, ServiceStateQuarantined:
		return true
	}
	return false
}

func (s ServiceState) IsHealthy() bool {
	return s == ServiceStateReady
}

type Platform string

const (
	PlatformWindowsAMD64 Platform = "windows/amd64"
	PlatformWindowsARM64 Platform = "windows/arm64"
	PlatformDarwinAMD64  Platform = "darwin/amd64"
	PlatformDarwinARM64  Platform = "darwin/arm64"
	PlatformLinuxAMD64   Platform = "linux/amd64"
	PlatformLinuxARM64   Platform = "linux/arm64"
)

func CurrentPlatform() Platform {
	os := runtime.GOOS
	arch := runtime.GOARCH
	return Platform(fmt.Sprintf("%s/%s", os, arch))
}

type PlatformExecutable struct {
	Platform     Platform          `json:"platform"`
	Path         string            `json:"path"`
	Sha256       string            `json:"sha256"`
	Entry        string            `json:"entry,omitempty"`
	ArgsTemplate []string          `json:"args_template,omitempty"`
	MinOSVersion string            `json:"min_os_version,omitempty"`
	Signature    BinarySignature   `json:"signature"`
	Dependencies []LibraryDep      `json:"dependencies,omitempty"`
	License      string            `json:"license,omitempty"`
	EnvTemplate  map[string]string `json:"env_template,omitempty"`
}

type BinarySignature struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
	Signer    string `json:"signer,omitempty"`
	Trusted   bool   `json:"trusted"`
}

type LibraryDep struct {
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	Sha256   string `json:"sha256,omitempty"`
	Required bool   `json:"required"`
}

type ServiceHealthCheck struct {
	Type                string          `json:"type"`
	Interval            time.Duration   `json:"interval"`
	Timeout             time.Duration   `json:"timeout"`
	GracePeriod         time.Duration   `json:"grace_period"`
	MaxConsecutiveFails int             `json:"max_consecutive_fails"`
	Endpoint            string          `json:"endpoint,omitempty"`
	CustomParams        json.RawMessage `json:"custom_params,omitempty"`
}

type ServiceShutdownPolicy struct {
	GracePeriod     time.Duration `json:"grace_period"`
	KillTimeout     time.Duration `json:"kill_timeout"`
	CleanupChildren bool          `json:"cleanup_children"`
	RemoveTempDir   bool          `json:"remove_temp_dir"`
}

type RecoveryDecisionMode string

const (
	RecoveryDecisionAutoRestart RecoveryDecisionMode = "auto_restart"
	RecoveryDecisionExternal    RecoveryDecisionMode = "external"
)

type ServiceRecoveryPolicy struct {
	MaxRestarts          int                  `json:"max_restarts"`
	RestartDelay         time.Duration        `json:"restart_delay"`
	BackoffMultiplier    float64              `json:"backoff_multiplier"`
	MaxRestartDelay      time.Duration        `json:"max_restart_delay"`
	QuarantineOnFail     bool                 `json:"quarantine_on_fail"`
	RecoveryDecisionMode RecoveryDecisionMode `json:"recovery_decision_mode"`
}

type ServiceResourceLimits struct {
	MaxMemoryMB        int64         `json:"max_memory_mb"`
	MaxCPUPercent      int           `json:"max_cpu_percent"`
	MaxFileDescriptors int           `json:"max_file_descriptors"`
	MaxDiskMB          int64         `json:"max_disk_mb"`
	MaxSubprocesses    int           `json:"max_subprocesses"`
	CPUTime            time.Duration `json:"cpu_time,omitempty"`
}

type ServiceRuntimeDefinition struct {
	ServiceID         string                `json:"service_id"`
	ExtensionID       string                `json:"extension_id"`
	ModuleID          string                `json:"module_id"`
	Name              string                `json:"name"`
	Description       string                `json:"description"`
	Publisher         string                `json:"publisher"`
	TrustLevel        string                `json:"trust_level"`
	Executables       []PlatformExecutable  `json:"executables"`
	Protocol          string                `json:"protocol"`
	InstancePolicy    string                `json:"instance_policy"`
	HealthCheck       ServiceHealthCheck    `json:"health_check"`
	Recovery          ServiceRecoveryPolicy `json:"recovery"`
	Shutdown          ServiceShutdownPolicy `json:"shutdown"`
	Limits            ServiceResourceLimits `json:"limits"`
	Network           ServiceNetworkPolicy  `json:"network"`
	AllowedNamespaces []string              `json:"allowed_namespaces"`
	// SandboxReadOnlyRoot is an authoritative host-computed package root. It is
	// deliberately excluded from JSON so an extension manifest cannot request
	// arbitrary host filesystem mounts. Strict OS sandboxes may expose this root
	// read-only to managed interpreters while keeping host data inaccessible.
	SandboxReadOnlyRoot string `json:"-"`
	ManifestHash        string `json:"manifest_hash"`
	DefinitionVersion   int    `json:"definition_version"`
	AutoStart           bool   `json:"auto_start"`
}

type ServiceNetworkPolicy struct {
	Mode           string   `json:"mode,omitempty"`
	Enforce        bool     `json:"enforce,omitempty"`
	AllowInbound   bool     `json:"allow_inbound"`
	AllowOutbound  bool     `json:"allow_outbound"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	AllowedIPs     []string `json:"allowed_ips,omitempty"`
	AllowedPorts   []int    `json:"allowed_ports,omitempty"`
	LoopbackOnly   bool     `json:"loopback_only"`
	RequireProxy   bool     `json:"require_proxy"`
	AuditAll       bool     `json:"audit_all"`
}

type ServiceInstance struct {
	InstanceID        string                    `json:"instance_id"`
	ProcessInstanceID string                    `json:"process_instance_id"`
	ServiceID         string                    `json:"service_id"`
	RuntimeID         string                    `json:"runtime_id,omitempty"`
	ExtensionID       string                    `json:"extension_id"`
	PluginID          string                    `json:"plugin_id"`
	LogicalServiceID  string                    `json:"logical_service_id"`
	HostInstanceID    string                    `json:"host_instance_id"`
	HostSessionID     string                    `json:"host_session_id"`
	ModuleID          string                    `json:"module_id"`
	Definition        *ServiceRuntimeDefinition `json:"-"`
	Generation        int64                     `json:"generation"`
	Platform          Platform                  `json:"platform"`
	Executable        *PlatformExecutable       `json:"executable"`
	State             ServiceState              `json:"state"`
	PID               int                       `json:"pid,omitempty"`
	StartedAt         *time.Time                `json:"started_at,omitempty"`
	StoppedAt         *time.Time                `json:"stopped_at,omitempty"`
	LastHealthAt      *time.Time                `json:"last_health_at,omitempty"`
	RestartCount      int                       `json:"restart_count"`
	HealthFails       int                       `json:"health_fails"`
	WorkingDir        string                    `json:"working_dir"`
	StdioConn         string                    `json:"stdio_conn,omitempty"`
	SessionToken      string                    `json:"session_token,omitempty"`
	rpcSession        *RPCSession               `json:"-"`
	protocolSession   io.Closer                 `json:"-"`
	managedProc       interface{}               `json:"-"`
	procHandle        uint64                    `json:"-"`
	circuit           *CircuitBreaker           `json:"-"`
	lastExitCode      int                       `json:"-"`
	lastExitError     string                    `json:"-"`
	mu                sync.RWMutex
	stopCh            chan struct{}
	healthCancel      context.CancelFunc
	processCancel     context.CancelFunc
	startupDetached   bool
	waitOwnerClaimed  bool
	restartRequest    StartRequest
}

func (i *ServiceInstance) bindProcessLifetime(cancel context.CancelFunc) {
	i.mu.Lock()
	i.processCancel = cancel
	i.startupDetached = false
	i.mu.Unlock()
}

// cancelProcessForStartup propagates cancellation only while Start is still
// establishing the process. Once startup is detached, request/operation
// contexts no longer own the long-lived service process.
func (i *ServiceInstance) cancelProcessForStartup() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.startupDetached || i.processCancel == nil {
		return
	}
	i.processCancel()
}

func (i *ServiceInstance) detachStartupCancellation() {
	i.mu.Lock()
	i.startupDetached = true
	i.mu.Unlock()
}

func (i *ServiceInstance) cancelProcessLifetime() {
	i.mu.Lock()
	cancel := i.processCancel
	i.processCancel = nil
	i.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// claimWaitOwner guarantees that exactly one goroutine may call Wait on the
// exec.Cmd backing this service instance.
func (i *ServiceInstance) claimWaitOwner() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.waitOwnerClaimed {
		return false
	}
	i.waitOwnerClaimed = true
	return true
}

func (i *ServiceInstance) SetState(state ServiceState) {
	i.mu.Lock()
	i.State = state
	i.mu.Unlock()
}

func (i *ServiceInstance) State_() ServiceState {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.State
}

func (i *ServiceInstance) MarkStarted() {
	now := time.Now().UTC()
	i.mu.Lock()
	i.StartedAt = &now
	i.State = ServiceStateReady
	i.stopCh = make(chan struct{})
	i.mu.Unlock()
}

func (i *ServiceInstance) MarkStopped() {
	now := time.Now().UTC()
	i.mu.Lock()
	i.StoppedAt = &now
	i.State = ServiceStateStopped
	if i.stopCh != nil {
		close(i.stopCh)
		i.stopCh = nil
	}
	i.mu.Unlock()
}

func (i *ServiceInstance) MarkCrashed() {
	i.mu.Lock()
	i.State = ServiceStateCrashed
	// Keep stopCh open while recovery is pending. Stop/disable/uninstall closes
	// it via MarkStopped, which cancels a delayed automatic restart.
	i.mu.Unlock()
}

func (i *ServiceInstance) cancelPendingRecovery() {
	i.mu.Lock()
	if i.stopCh != nil {
		close(i.stopCh)
		i.stopCh = nil
	}
	i.mu.Unlock()
}

func (i *ServiceInstance) RecordHealthFail() int {
	i.mu.Lock()
	i.HealthFails++
	count := i.HealthFails
	i.mu.Unlock()
	return count
}

func (i *ServiceInstance) RecordHealthSuccess() {
	i.mu.Lock()
	i.HealthFails = 0
	now := time.Now().UTC()
	i.LastHealthAt = &now
	i.mu.Unlock()
}

func (i *ServiceInstance) IncrementRestart() int {
	i.mu.Lock()
	i.RestartCount++
	count := i.RestartCount
	i.mu.Unlock()
	return count
}

var (
	ErrUnknownPublisher       = errors.New("trusted_service: unknown publisher not allowed")
	ErrInvalidSignature       = errors.New("trusted_service: invalid binary signature")
	ErrPlatformNotSupported   = errors.New("trusted_service: platform not supported")
	ErrServiceNotFound        = errors.New("trusted_service: service not found")
	ErrServiceQuarantined     = errors.New("trusted_service: service quarantined")
	ErrAlreadyRunning         = errors.New("trusted_service: service already running")
	ErrShellDisallowed        = errors.New("trusted_service: shell command disallowed")
	ErrMaxRestartsExceeded    = errors.New("trusted_service: max restarts exceeded")
	ErrHealthCheckFailed      = errors.New("trusted_service: health check failed")
	ErrBinaryHashMismatch     = errors.New("trusted_service: binary hash mismatch")
	ErrUnauthorizedNetwork    = errors.New("trusted_service: unauthorized network access")
	ErrTrustLevelInsufficient = errors.New("trusted_service: trust level insufficient")
)

type TrustLevel string

const (
	TrustLevelOfficial  TrustLevel = "official"
	TrustLevelTrusted   TrustLevel = "trusted"
	TrustLevelCommunity TrustLevel = "community"
	TrustLevelUnknown   TrustLevel = "unknown"
)

// AllowedForService is intentionally stricter than package installation trust.
// Community publishers may be discovered/previewed, but executable services
// must be explicitly promoted by a user trust decision first. The GameHost
// mapper converts user_trusted to trusted. This avoids advertising an
// impossible "full sandbox" guarantee on desktop platforms and never silently
// weakens isolation to make an untrusted executable run.
func (t TrustLevel) AllowedForService() bool {
	return t == TrustLevelOfficial || t == TrustLevelTrusted
}

// RequiresFullSandbox remains a defense-in-depth marker if a future admission
// path permits community executables after a platform sandbox implementation.
func (t TrustLevel) RequiresFullSandbox() bool { return t == TrustLevelCommunity }
