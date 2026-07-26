package javascript_main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime"
)

type HostState string

const (
	HostStateCreated    HostState = "created"
	HostStateStarting   HostState = "starting"
	HostStateReady      HostState = "ready"
	HostStateStopping   HostState = "stopping"
	HostStateStopped    HostState = "stopped"
	HostStateCrashed    HostState = "crashed"
	HostStateUnhealthy  HostState = "unhealthy"
)

type PluginHost struct {
	mu              sync.RWMutex
	instanceID      string
	extensionID     string
	moduleID        string
	state           HostState
	spec            runtime.BootstrapSpec
	boundary        runtime.ProcessBoundary
	startedAt       *time.Time
	readyAt         *time.Time
	stoppedAt       *time.Time
	crashCount      int
	lastError       string
	handlers        *HandlerRegistry
	dispatcher      *InvocationDispatcher
	watchdog        *Watchdog
	session         *RuntimeSession
	shutdownCoordinator *ShutdownCoordinator
	rpcVersion      string
	definitionHash  string
}

type PluginHostConfig struct {
	InstanceID         string
	ExtensionID        string
	ModuleID           string
	BootstrapSpec      runtime.BootstrapSpec
	ProcessBoundary    runtime.ProcessBoundary
	DefinitionHash     string
	HostAPIVersion     string
	AllowedContributions []AllowedContribution
}

type AllowedContribution struct {
	ContributionID string
	EntryType      string
	EntryName      string
}

func NewPluginHost(cfg PluginHostConfig) (*PluginHost, error) {
	if cfg.InstanceID == "" {
		return nil, errors.New("javascript_main: instance id required")
	}
	if cfg.ExtensionID == "" {
		return nil, errors.New("javascript_main: extension id required")
	}
	if cfg.ModuleID == "" {
		return nil, errors.New("javascript_main: module id required")
	}
	host := &PluginHost{
		instanceID:     cfg.InstanceID,
		extensionID:    cfg.ExtensionID,
		moduleID:       cfg.ModuleID,
		state:          HostStateCreated,
		spec:           cfg.BootstrapSpec,
		boundary:       cfg.ProcessBoundary,
		definitionHash: cfg.DefinitionHash,
		rpcVersion:     cfg.HostAPIVersion,
		handlers:       NewHandlerRegistry(cfg.AllowedContributions),
		dispatcher:     NewInvocationDispatcher(cfg.BootstrapSpec.ResourceLimits),
		watchdog:       NewWatchdog(cfg.InstanceID),
		shutdownCoordinator: NewShutdownCoordinator(),
	}
	return host, nil
}

func (h *PluginHost) InstanceID() string  { return h.instanceID }
func (h *PluginHost) ExtensionID() string { return h.extensionID }
func (h *PluginHost) ModuleID() string    { return h.moduleID }
func (h *PluginHost) State() HostState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

type StartResult struct {
	Success    bool
	InstanceID string
	Reason     string
	ReadyAt    *time.Time
	Steps      []StartStep
}

type StartStep struct {
	Name   string
	Status string
	Error  string
}

func (h *PluginHost) Start(ctx context.Context) StartResult {
	result := StartResult{InstanceID: h.instanceID}

	h.mu.Lock()
	if h.state != HostStateCreated {
		h.mu.Unlock()
		result.Reason = fmt.Sprintf("host in state %s, expected created", h.state)
		return result
	}
	h.state = HostStateStarting
	now := time.Now().UTC()
	h.startedAt = &now
	h.mu.Unlock()

	sequence := runtime.DefaultBootstrapSequence()
	for _, step := range sequence.Steps {
		startStep := StartStep{Name: step.Name, Status: "succeeded"}
		if err := h.executeBootstrapStep(ctx, step.Name); err != nil {
			startStep.Status = "failed"
			startStep.Error = err.Error()
			result.Steps = append(result.Steps, startStep)
			result.Reason = fmt.Sprintf("bootstrap step %s failed: %v", step.Name, err)
			h.mu.Lock()
			h.state = HostStateCrashed
			h.lastError = err.Error()
			h.mu.Unlock()
			return result
		}
		result.Steps = append(result.Steps, startStep)
	}

	h.mu.Lock()
	h.state = HostStateReady
	readyNow := time.Now().UTC()
	h.readyAt = &readyNow
	h.session = &RuntimeSession{
		InstanceID:     h.instanceID,
		ExtensionID:    h.extensionID,
		ModuleID:       h.moduleID,
		SessionToken:   h.spec.SessionToken,
		DefinitionHash: h.definitionHash,
		State:          runtime.SessionStateReady,
		StartedAt:      h.startedAt.Format(time.RFC3339),
		Ready:          true,
	}
	h.mu.Unlock()

	h.watchdog.Start(ctx, h)

	result.Success = true
	result.ReadyAt = h.readyAt
	return result
}

func (h *PluginHost) executeBootstrapStep(ctx context.Context, stepName string) error {
	switch stepName {
	case "process_start":
		return nil
	case "read_bootstrap_spec":
		if h.spec.InstanceID == "" {
			return errors.New("bootstrap spec missing instance id")
		}
		if h.spec.Entry == "" {
			return errors.New("bootstrap spec missing entry")
		}
		return nil
	case "open_rpc_channel":
		if h.rpcVersion == "" {
			return errors.New("rpc version required")
		}
		return nil
	case "authenticate_session":
		if h.spec.SessionToken == "" {
			return errors.New("session token required")
		}
		return nil
	case "verify_definition":
		if h.definitionHash == "" {
			return errors.New("definition hash required")
		}
		return nil
	case "initialize_sdk":
		return nil
	case "load_entry_module":
		if h.spec.Entry == "" {
			return errors.New("entry module required")
		}
		return nil
	case "call_activate":
		return nil
	case "report_ready":
		return nil
	}
	return nil
}

func (h *PluginHost) Stop(ctx context.Context, reason string) error {
	h.mu.Lock()
	if h.state == HostStateStopped || h.state == HostStateCrashed {
		h.mu.Unlock()
		return nil
	}
	if h.state != HostStateReady && h.state != HostStateUnhealthy {
		h.mu.Unlock()
		return fmt.Errorf("javascript_main: cannot stop host in state %s", h.state)
	}
	h.state = HostStateStopping
	h.mu.Unlock()

	h.watchdog.Stop()
	h.shutdownCoordinator.BeginShutdown()

	h.dispatcher.RejectNewInvocations()
	h.dispatcher.CancelQueued(reason)

	completed := h.dispatcher.WaitForRunning(ctx, 5*time.Second)
	if !completed {
		h.shutdownCoordinator.MarkForceStopped()
	}

	h.shutdownCoordinator.MarkDeactivateCalled()

	h.mu.Lock()
	h.state = HostStateStopped
	now := time.Now().UTC()
	h.stoppedAt = &now
	h.mu.Unlock()

	h.shutdownCoordinator.MarkSessionClosed()
	h.shutdownCoordinator.MarkStoppedSent()
	h.shutdownCoordinator.Complete()

	return nil
}

func (h *PluginHost) MarkCrashed(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = HostStateCrashed
	h.crashCount++
	h.lastError = reason
	now := time.Now().UTC()
	h.stoppedAt = &now
}

func (h *PluginHost) CrashCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.crashCount
}

func (h *PluginHost) LastError() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastError
}

func (h *PluginHost) Handlers() *HandlerRegistry { return h.handlers }
func (h *PluginHost) Dispatcher() *InvocationDispatcher { return h.dispatcher }
func (h *PluginHost) Watchdog() *Watchdog { return h.watchdog }
func (h *PluginHost) Session() *RuntimeSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.session
}

type RuntimeSession struct {
	InstanceID     string
	ExtensionID    string
	ModuleID       string
	SessionToken   string
	DefinitionHash string
	State          runtime.SessionState
	StartedAt      string
	Ready          bool
}

func (h *PluginHost) Health() HealthReport {
	h.mu.RLock()
	defer h.mu.RUnlock()
	report := HealthReport{
		InstanceID:  h.instanceID,
		ExtensionID: h.extensionID,
		ModuleID:    h.moduleID,
		State:       h.state,
		CrashCount:  h.crashCount,
	}
	if h.watchdog != nil {
		report.Watchdog = h.watchdog.Report()
	}
	if h.dispatcher != nil {
		report.ActiveInvocations = h.dispatcher.ActiveCount()
		report.QueuedInvocations = h.dispatcher.QueuedCount()
	}
	return report
}

type HealthReport struct {
	InstanceID         string
	ExtensionID        string
	ModuleID           string
	State              HostState
	CrashCount         int
	ActiveInvocations  int
	QueuedInvocations  int
	Watchdog           WatchdogReport
}
