package trusted_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type ServiceRecoveryPolicy struct {
	MaxRestarts       int           `json:"max_restarts"`
	RestartDelay      time.Duration `json:"restart_delay"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	MaxRestartDelay   time.Duration `json:"max_restart_delay"`
	QuarantineOnFail  bool          `json:"quarantine_on_fail"`
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
	ManifestHash      string                `json:"manifest_hash"`
	DefinitionVersion int                   `json:"definition_version"`
	AutoStart         bool                  `json:"auto_start"`
}

type ServiceNetworkPolicy struct {
	AllowInbound   bool     `json:"allow_inbound"`
	AllowOutbound  bool     `json:"allow_outbound"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	AllowedPorts   []int    `json:"allowed_ports,omitempty"`
	LoopbackOnly   bool     `json:"loopback_only"`
	RequireProxy   bool     `json:"require_proxy"`
	AuditAll       bool     `json:"audit_all"`
}

type ServiceInstance struct {
	InstanceID     string                    `json:"instance_id"`
	ServiceID      string                    `json:"service_id"`
	Definition     *ServiceRuntimeDefinition `json:"-"`
	Generation     int64                     `json:"generation"`
	Platform       Platform                  `json:"platform"`
	Executable     *PlatformExecutable       `json:"executable"`
	State          ServiceState              `json:"state"`
	PID            int                       `json:"pid,omitempty"`
	StartedAt      *time.Time                `json:"started_at,omitempty"`
	StoppedAt      *time.Time                `json:"stopped_at,omitempty"`
	LastHealthAt   *time.Time                `json:"last_health_at,omitempty"`
	RestartCount   int                       `json:"restart_count"`
	HealthFails    int                       `json:"health_fails"`
	WorkingDir     string                    `json:"working_dir"`
	StdioConn      string                    `json:"stdio_conn,omitempty"`
	SessionToken   string                    `json:"session_token,omitempty"`
	rpcSession     *RPCSession               `json:"-"`
	protocolSession io.Closer                `json:"-"`
	managedProc    interface{}               `json:"-"`
	procHandle     uint64                    `json:"-"`
	circuit        *CircuitBreaker           `json:"-"`
	lastExitCode   int                       `json:"-"`
	lastExitError  string                    `json:"-"`
	mu             sync.RWMutex
	stopCh         chan struct{}
	healthCancel   context.CancelFunc
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

func (t TrustLevel) AllowedForService() bool {
	return t == TrustLevelOfficial || t == TrustLevelTrusted
}
