package trusted_service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

type StdioProtocolSessionMeta struct {
	Protocol    string
	ServiceID   string
	InstanceID  string
	ExtensionID string
	ModuleID    string
	Generation  int64
}

type StdioProtocolHandler interface {
	Attach(
		ctx context.Context,
		meta StdioProtocolSessionMeta,
		stdin io.WriteCloser,
		stdout io.ReadCloser,
	) (io.Closer, error)
}

type GameHostNotifier interface {
	Notify(ctx context.Context, extensionID, instanceID, serviceID, method string, params json.RawMessage)
}

type NotifyFunc func(ctx context.Context, extensionID, instanceID, serviceID, method string, params json.RawMessage)

func (f NotifyFunc) Notify(ctx context.Context, extensionID, instanceID, serviceID, method string, params json.RawMessage) {
	f(ctx, extensionID, instanceID, serviceID, method, params)
}

type ProcessExitEvent struct {
	ProcessInstanceID string
	ServiceID         string
	RuntimeID         string
	InstanceID        string
	ExtensionID       string
	PluginID          string
	LogicalServiceID  string
	Generation        int64
	ExitCode          int
	Expected          bool
	OccurredAt        time.Time
	RestartCount      int
	HostInstanceID    string
	HostSessionID     string
	ModuleID          string
}

type ProcessExitObserver interface {
	OnProcessExit(event ProcessExitEvent)
}

type ProcessSupervisor struct {
	mu               sync.Mutex
	instances        map[string]*ServiceInstance
	defs             map[string]*ServiceRuntimeDefinition
	verifier         *BinaryVerifier
	selector         *PlatformSelector
	envBuilder       *EnvBuilder
	logger           func(level, msg string, fields map[string]any)
	healthMon        *HealthMonitor
	quarantine       *QuarantineManager
	rootDir          string
	procMgr          *process.DefaultProcessManager
	gameHostNotifier GameHostNotifier
	protocolHandlers map[string]StdioProtocolHandler
	exitObservers    []ProcessExitObserver
	ownerStore       *ProcessOwnerStore
	hostIdentity     HostIdentityProvider
	ownerStoreErr    error
}

type HostIdentityProvider interface {
	GetHostInstanceID() string
	GetHostSessionID() string
}

func NewProcessSupervisor(rootDir string) *ProcessSupervisor {
	s := &ProcessSupervisor{
		instances:        make(map[string]*ServiceInstance),
		defs:             make(map[string]*ServiceRuntimeDefinition),
		verifier:         NewBinaryVerifier(),
		selector:         NewPlatformSelector(),
		envBuilder:       NewEnvBuilder(),
		healthMon:        NewHealthMonitor(),
		quarantine:       NewQuarantineManager(),
		procMgr:          process.NewDefaultProcessManager(),
		rootDir:          rootDir,
		logger:           func(level, msg string, fields map[string]any) {},
		protocolHandlers: make(map[string]StdioProtocolHandler),
		ownerStore:       NewProcessOwnerStore(filepath.Join(rootDir, "process_owners")),
	}
	if s.ownerStore != nil && s.ownerStore.durable {
		s.ownerStoreErr = s.ownerStore.load()
	}
	return s
}

func NewProcessSupervisorWithVerifier(rootDir string, verifier *BinaryVerifier) *ProcessSupervisor {
	if verifier == nil {
		verifier = NewBinaryVerifier()
	}
	return &ProcessSupervisor{
		instances:        make(map[string]*ServiceInstance),
		defs:             make(map[string]*ServiceRuntimeDefinition),
		verifier:         verifier,
		selector:         NewPlatformSelector(),
		envBuilder:       NewEnvBuilder(),
		healthMon:        NewHealthMonitor(),
		quarantine:       NewQuarantineManager(),
		procMgr:          process.NewDefaultProcessManager(),
		rootDir:          rootDir,
		logger:           func(level, msg string, fields map[string]any) {},
		protocolHandlers: make(map[string]StdioProtocolHandler),
	}
}

func (s *ProcessSupervisor) SetLogger(l func(level, msg string, fields map[string]any)) {
	s.logger = l
}

func (s *ProcessSupervisor) SetGameHostNotifier(n GameHostNotifier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gameHostNotifier = n
}

func (s *ProcessSupervisor) SetHostIdentityProvider(h HostIdentityProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostIdentity = h
}

func (s *ProcessSupervisor) LookupDurableOwnedProcess(processInstanceID string) (ProcessOwnerMetadata, bool) {
	if s.ownerStore == nil || processInstanceID == "" {
		return ProcessOwnerMetadata{}, false
	}
	return s.ownerStore.LookupByProcessInstanceID(processInstanceID)
}

func (s *ProcessSupervisor) GetHostInstanceID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hostIdentity != nil {
		return s.hostIdentity.GetHostInstanceID()
	}
	return ""
}

func (s *ProcessSupervisor) GetHostSessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hostIdentity != nil {
		return s.hostIdentity.GetHostSessionID()
	}
	return ""
}

func (s *ProcessSupervisor) RegisterProcessExitObserver(observer ProcessExitObserver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exitObservers = append(s.exitObservers, observer)
}

func (s *ProcessSupervisor) UnregisterProcessExitObserver(observer ProcessExitObserver) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, o := range s.exitObservers {
		if o == observer {
			s.exitObservers = append(s.exitObservers[:i], s.exitObservers[i+1:]...)
			return
		}
	}
}

func (s *ProcessSupervisor) notifyProcessExit(event ProcessExitEvent) {
	s.mu.Lock()
	observers := make([]ProcessExitObserver, len(s.exitObservers))
	copy(observers, s.exitObservers)
	s.mu.Unlock()
	for _, o := range observers {
		o.OnProcessExit(event)
	}
}

func (s *ProcessSupervisor) getCanonicalPluginID(inst *ServiceInstance) string {
	if inst.PluginID != "" {
		return inst.PluginID
	}
	if inst.Definition != nil && inst.Definition.Publisher != "" {
		return inst.Definition.Publisher + "/" + inst.Definition.ExtensionID
	}
	if inst.Definition != nil {
		return inst.Definition.ExtensionID
	}
	return inst.ExtensionID
}

func (s *ProcessSupervisor) RegisterStdioProtocolHandler(protocol string, handler StdioProtocolHandler) error {
	if protocol == "" || handler == nil {
		return errors.New("trusted_service: invalid protocol handler registration")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.protocolHandlers[protocol]; exists {
		return fmt.Errorf("trusted_service: protocol %s already has a registered handler", protocol)
	}
	s.protocolHandlers[protocol] = handler
	return nil
}

func (s *ProcessSupervisor) UnregisterStdioProtocolHandler(protocol string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.protocolHandlers, protocol)
}

func (s *ProcessSupervisor) QuarantineManager() *QuarantineManager {
	return s.quarantine
}

func (s *ProcessSupervisor) Register(def *ServiceRuntimeDefinition) error {
	if def == nil || def.ServiceID == "" {
		return errors.New("trusted_service: invalid definition")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.defs[def.ServiceID]; exists {
		return fmt.Errorf("trusted_service: service %s already registered", def.ServiceID)
	}
	s.defs[def.ServiceID] = def
	return nil
}

func (s *ProcessSupervisor) Unregister(serviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.defs[serviceID]; !exists {
		return ErrServiceNotFound
	}
	if inst, exists := s.instances[serviceID]; exists && !inst.State_().IsTerminal() {
		return fmt.Errorf("trusted_service: cannot unregister running service %s", serviceID)
	}
	delete(s.defs, serviceID)
	delete(s.instances, serviceID)
	return nil
}

func (s *ProcessSupervisor) GetDefinition(serviceID string) (*ServiceRuntimeDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, exists := s.defs[serviceID]
	if !exists {
		return nil, ErrServiceNotFound
	}
	execs := make([]PlatformExecutable, len(def.Executables))
	copy(execs, def.Executables)
	ns := make([]string, len(def.AllowedNamespaces))
	copy(ns, def.AllowedNamespaces)
	return &ServiceRuntimeDefinition{
		ServiceID:         def.ServiceID,
		ExtensionID:       def.ExtensionID,
		ModuleID:          def.ModuleID,
		Name:              def.Name,
		Description:       def.Description,
		Publisher:         def.Publisher,
		TrustLevel:        def.TrustLevel,
		Executables:       execs,
		Protocol:          def.Protocol,
		InstancePolicy:    def.InstancePolicy,
		HealthCheck:       def.HealthCheck,
		Recovery:          def.Recovery,
		Shutdown:          def.Shutdown,
		Limits:            def.Limits,
		Network:           def.Network,
		AllowedNamespaces: ns,
		ManifestHash:      def.ManifestHash,
		DefinitionVersion: def.DefinitionVersion,
		AutoStart:         def.AutoStart,
	}, nil
}

func (s *ProcessSupervisor) HasDefinition(serviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.defs[serviceID]
	return exists
}

type StartRequest struct {
	ServiceID        string
	InstanceID       string
	RuntimeID        string
	PluginID         string
	LogicalServiceID string
	ExtensionID      string
	ContributionID   string
	Generation       int64
	PublisherTrust   TrustLevel
	BasePath         string
	WorkingDir       string
	SessionToken     string
	SecretLease      string
	LogLevel         string
	Args             map[string]string
}

type StartResult struct {
	InstanceID string
	PID        int
	State      ServiceState
	StartedAt  time.Time
	Generation int64
}

func (s *ProcessSupervisor) Start(ctx context.Context, req StartRequest) (*StartResult, error) {
	if s.ownerStoreErr != nil {
		return nil, fmt.Errorf("trusted_service: durable process owner store unavailable: %w", s.ownerStoreErr)
	}
	s.mu.Lock()
	def, exists := s.defs[req.ServiceID]
	if !exists {
		s.mu.Unlock()
		return nil, ErrServiceNotFound
	}
	if inst, exists := s.instances[req.ServiceID]; exists && !inst.State_().IsTerminal() {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrAlreadyRunning, req.ServiceID)
	}
	if s.quarantine.IsQuarantined(req.ServiceID) {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrServiceQuarantined, req.ServiceID)
	}
	s.mu.Unlock()

	if err := ValidateTrust(def, req.PublisherTrust); err != nil {
		s.log("error", "trust validation failed", map[string]any{"service": req.ServiceID, "error": err.Error()})
		return nil, err
	}

	exe, err := s.selector.Select(def)
	if err != nil {
		return nil, err
	}

	if err := s.verifier.Verify(ctx, exe, req.BasePath); err != nil {
		return nil, err
	}

	argsBuilder := NewArgsBuilder(exe.ArgsTemplate)
	args, err := argsBuilder.Build(req.Args)
	if err != nil {
		return nil, err
	}
	if err := ValidateNoShell(append([]string{exe.Path}, args...)); err != nil {
		return nil, err
	}

	workingDir := req.WorkingDir
	if workingDir == "" {
		workingDir = filepath.Join(s.rootDir, "services", req.ServiceID, req.InstanceID)
		if err := os.MkdirAll(workingDir, 0o755); err != nil {
			return nil, fmt.Errorf("trusted_service: create working dir: %w", err)
		}
	}

	instanceID := req.InstanceID
	if instanceID == "" {
		instanceID = newServiceInstanceID(req.ServiceID)
	}
	tempDir := filepath.Join(s.rootDir, "temp", req.ServiceID, instanceID)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("trusted_service: create temp dir: %w", err)
	}

	fullExePath := exe.Path
	if !filepath.IsAbs(fullExePath) && req.BasePath != "" {
		fullExePath = filepath.Join(req.BasePath, fullExePath)
	}

	env := s.buildSafeEnvironment(exe, def, req.SessionToken, instanceID, req.Generation, tempDir, defaultLogLevel(req.LogLevel), req.SecretLease)

	instance := &ServiceInstance{
		InstanceID:        instanceID,
		ProcessInstanceID: req.ServiceID,
		ServiceID:         req.ServiceID,
		RuntimeID:         req.RuntimeID,
		ExtensionID:       def.ExtensionID,
		PluginID:          resolveCanonicalPluginID(req.PluginID, def.Publisher, def.ExtensionID),
		LogicalServiceID:  resolveCanonicalLogicalServiceID(req.LogicalServiceID, req.ServiceID, req.RuntimeID),
		ModuleID:          def.ModuleID,
		Definition:        def,
		Generation:        req.Generation,
		Platform:          s.selector.current,
		Executable:        exe,
		State:             ServiceStateStarting,
		WorkingDir:        workingDir,
		SessionToken:      req.SessionToken,
		circuit:           NewCircuitBreaker(DefaultCircuitConfig()),
		stopCh:            make(chan struct{}),
	}
	instance.SetState(ServiceStateStarting)

	if !instance.circuit.AllowStart() {
		instance.SetState(ServiceStateFailed)
		s.mu.Lock()
		s.instances[req.ServiceID] = instance
		s.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrCircuitOpen, req.ServiceID)
	}

	isRPC := def.Protocol == "jsonrpc" || def.Protocol == "amitia_jsonrpc_v1"

	if isRPC {
		if err := s.startRPCService(ctx, instance, def, exe, fullExePath, args, env, workingDir, req); err != nil {
			instance.SetState(ServiceStateFailed)
			instance.circuit.RecordFailure()
			s.mu.Lock()
			s.instances[req.ServiceID] = instance
			s.mu.Unlock()
			return nil, err
		}
	} else if handler, ok := s.protocolHandlers[def.Protocol]; ok {
		if err := s.startManagedStdioProtocolService(ctx, instance, def, exe, fullExePath, args, env, workingDir, req, handler); err != nil {
			instance.SetState(ServiceStateFailed)
			instance.circuit.RecordFailure()
			s.mu.Lock()
			s.instances[req.ServiceID] = instance
			s.mu.Unlock()
			return nil, err
		}
	} else if def.Protocol == "" || def.Protocol == "plain" {
		if err := s.startPlainService(ctx, instance, def, exe, fullExePath, args, env, workingDir, req); err != nil {
			instance.SetState(ServiceStateFailed)
			instance.circuit.RecordFailure()
			s.mu.Lock()
			s.instances[req.ServiceID] = instance
			s.mu.Unlock()
			return nil, err
		}
	} else {
		instance.SetState(ServiceStateFailed)
		instance.circuit.RecordFailure()
		s.mu.Lock()
		s.instances[req.ServiceID] = instance
		s.mu.Unlock()
		return nil, fmt.Errorf("trusted_service: unknown protocol %q and no registered handler", def.Protocol)
	}

	instance.MarkStarted()
	instance.circuit.RecordSuccess()

	instance.HostInstanceID = s.GetHostInstanceID()
	instance.HostSessionID = s.GetHostSessionID()

	s.mu.Lock()
	s.instances[req.ServiceID] = instance
	s.mu.Unlock()

	if s.ownerStore != nil {
		if _, err := s.ownerStore.RecordOwnership(instance, instance.HostInstanceID, instance.HostSessionID); err != nil {
			s.ownerStoreErr = err
			s.killProcessTree(instance)
			return nil, fmt.Errorf("trusted_service: persist process ownership: %w", err)
		}
	}

	go s.healthMon.Monitor(instance, def)

	return &StartResult{
		InstanceID: instanceID,
		PID:        instance.PID,
		State:      ServiceStateReady,
		StartedAt:  *instance.StartedAt,
		Generation: req.Generation,
	}, nil
}

func (s *ProcessSupervisor) startRPCService(ctx context.Context, instance *ServiceInstance, def *ServiceRuntimeDefinition, exe *PlatformExecutable, fullExePath string, args, env []string, workingDir string, req StartRequest) error {
	cmd := exec.CommandContext(ctx, fullExePath, args...)
	cmd.Dir = workingDir
	cmd.Env = env
	process.ConfigureProcess(cmd)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("trusted_service: create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return fmt.Errorf("trusted_service: create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		return fmt.Errorf("trusted_service: create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		return fmt.Errorf("trusted_service: start process: %w", err)
	}

	handle, attachErr := process.AttachProcessTree(cmd)
	if attachErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("trusted_service: attach process tree: %w", attachErr)
	}

	instance.PID = cmd.Process.Pid
	instance.managedProc = cmd
	instance.procHandle = uint64(handle)

	nonce := generateNonce()
	rpcSession := NewRPCSession(stdinPipe, stdoutPipe, stderrPipe, RPCSessionConfig{
		InstanceID:  instance.InstanceID,
		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
		Nonce:       nonce,
		OnLog: func(level, msg string) {
			s.log(level, msg, map[string]any{"service": instance.ServiceID, "instance": instance.InstanceID})
		},
		OnNotification: func(method string, params json.RawMessage) {
			s.log("debug", fmt.Sprintf("rpc notification: %s", method), map[string]any{"service": instance.ServiceID})
			s.mu.Lock()
			notifier := s.gameHostNotifier
			s.mu.Unlock()
			if notifier != nil {
				notifier.Notify(context.Background(), def.ExtensionID, instance.InstanceID, instance.ServiceID, method, params)
			}
		},
	})
	instance.rpcSession = rpcSession
	rpcSession.Start()

	helloTimeout := def.HealthCheck.GracePeriod
	if helloTimeout <= 0 {
		helloTimeout = 10 * time.Second
	}

	hello, err := rpcSession.WaitForHello(helloTimeout)
	if err != nil {
		s.killProcessTree(instance)
		rpcSession.Close()
		return fmt.Errorf("trusted_service: rpc handshake failed (hello): %w", err)
	}

	if hello.ProtocolVersion != protocolVersion {
		s.killProcessTree(instance)
		rpcSession.Close()
		return fmt.Errorf("trusted_service: protocol version mismatch: expected %s got %s", protocolVersion, hello.ProtocolVersion)
	}

	welcomeExpiry := time.Now().UTC().Add(30 * time.Minute)
	if err := rpcSession.SendWelcome(req.SessionToken, map[string]any{
		"max_memory_mb":        def.Limits.MaxMemoryMB,
		"max_cpu_percent":      def.Limits.MaxCPUPercent,
		"max_subprocesses":     def.Limits.MaxSubprocesses,
		"max_file_descriptors": def.Limits.MaxFileDescriptors,
	}, welcomeExpiry); err != nil {
		s.killProcessTree(instance)
		rpcSession.Close()
		return fmt.Errorf("trusted_service: rpc send welcome: %w", err)
	}

	initReq, err := rpcSession.WaitForInitialize(helloTimeout)
	if err != nil {
		s.killProcessTree(instance)
		rpcSession.Close()
		return fmt.Errorf("trusted_service: rpc handshake failed (initialize): %w", err)
	}

	s.log("info", "service initialized", map[string]any{
		"service":      instance.ServiceID,
		"version":      initReq.Version,
		"capabilities": initReq.Capabilities,
	})

	if err := rpcSession.RespondInitialize(true, "accepted"); err != nil {
		s.killProcessTree(instance)
		rpcSession.Close()
		return fmt.Errorf("trusted_service: rpc respond initialize: %w", err)
	}

	if err := rpcSession.WaitForReady(helloTimeout); err != nil {
		s.killProcessTree(instance)
		rpcSession.Close()
		return fmt.Errorf("trusted_service: rpc handshake failed (ready): %w", err)
	}

	instance.StdioConn = "jsonrpc-stdio"

	go s.watchProcess(instance, cmd)

	return nil
}

func (s *ProcessSupervisor) startPlainService(ctx context.Context, instance *ServiceInstance, def *ServiceRuntimeDefinition, exe *PlatformExecutable, fullExePath string, args, env []string, workingDir string, req StartRequest) error {
	cmd := exec.CommandContext(ctx, fullExePath, args...)
	cmd.Dir = workingDir
	cmd.Env = env
	cmd.Stdout = newLogWriter(s.logger, "info", req.ServiceID)
	cmd.Stderr = newLogWriter(s.logger, "warn", req.ServiceID)
	process.ConfigureProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("trusted_service: start process: %w", err)
	}

	handle, attachErr := process.AttachProcessTree(cmd)
	if attachErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("trusted_service: attach process tree: %w", attachErr)
	}

	instance.PID = cmd.Process.Pid
	instance.managedProc = cmd
	instance.procHandle = uint64(handle)

	go s.watchProcess(instance, cmd)

	return nil
}

func (s *ProcessSupervisor) startManagedStdioProtocolService(
	ctx context.Context,
	instance *ServiceInstance,
	def *ServiceRuntimeDefinition,
	exe *PlatformExecutable,
	fullExePath string,
	args, env []string,
	workingDir string,
	req StartRequest,
	handler StdioProtocolHandler,
) error {
	cmd := exec.CommandContext(ctx, fullExePath, args...)
	cmd.Dir = workingDir
	cmd.Env = env
	process.ConfigureProcess(cmd)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("trusted_service: create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return fmt.Errorf("trusted_service: create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		return fmt.Errorf("trusted_service: create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		return fmt.Errorf("trusted_service: start process: %w", err)
	}

	handle, attachErr := process.AttachProcessTree(cmd)
	if attachErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		return fmt.Errorf("trusted_service: attach process tree: %w", attachErr)
	}

	instance.PID = cmd.Process.Pid
	instance.managedProc = cmd
	instance.procHandle = uint64(handle)

	go s.drainStderr(stderrPipe, instance.ServiceID, instance.InstanceID)

	meta := StdioProtocolSessionMeta{
		Protocol:    def.Protocol,
		ServiceID:   instance.ServiceID,
		InstanceID:  instance.InstanceID,
		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
		Generation:  instance.Generation,
	}

	closer, err := handler.Attach(ctx, meta, stdinPipe, stdoutPipe)
	if err != nil {
		s.killProcessTree(instance)
		return fmt.Errorf("trusted_service: protocol handler attach failed: %w", err)
	}

	instance.protocolSession = closer
	instance.StdioConn = def.Protocol + "-stdio"

	go s.watchProcess(instance, cmd)

	return nil
}

func (s *ProcessSupervisor) drainStderr(stderrPipe io.ReadCloser, serviceID, instanceID string) {
	defer stderrPipe.Close()
	buf := make([]byte, 4096)
	for {
		n, err := stderrPipe.Read(buf)
		if n > 0 {
			s.log("warn", string(buf[:n]), map[string]any{
				"service":  serviceID,
				"instance": instanceID,
				"source":   "stderr",
			})
		}
		if err != nil {
			return
		}
	}
}

func (s *ProcessSupervisor) buildSafeEnvironment(exe *PlatformExecutable, def *ServiceRuntimeDefinition, session, instance string, generation int64, tempDir, logLevel, secretLease string) []string {
	envB := process.NewEnvironmentBuilder().
		SetRuntimeInstance(instance).
		SetExtensionID(def.ExtensionID).
		SetModuleID(def.ModuleID).
		SetGeneration(generation).
		SetRPCEndpoint("internal-rpc").
		SetLogLevel(logLevel).
		SetTempDir(tempDir)

	envB.Set("AMITIA_INSTANCE", instance)
	envB.Set("AMITIA_GENERATION", fmt.Sprintf("%d", generation))
	envB.Set("AMITIA_SECRET_LEASE", secretLease)
	envB.Set("AMITIA_HOST_API", "internal-rpc")
	envB.Set("AMITIA_PROTOCOL", def.Protocol)
	envB.Set("AMITIA_PLATFORM", string(CurrentPlatform()))

	for k, v := range exe.EnvTemplate {
		if s.envBuilder.IsAllowed(k) {
			envB.Set(k, v)
		}
	}

	return envB.BuildFiltered()
}

func (s *ProcessSupervisor) watchProcess(inst *ServiceInstance, cmd *exec.Cmd) {
	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	inst.mu.Lock()
	inst.lastExitCode = exitCode
	if err != nil {
		inst.lastExitError = err.Error()
	}
	inst.mu.Unlock()

	if inst.rpcSession != nil {
		inst.rpcSession.Close()
	}

	if inst.protocolSession != nil {
		inst.protocolSession.Close()
	}

	handle := process.ProcessTreeHandle(inst.procHandle)
	if handle != 0 {
		process.CloseProcessTree(handle)
	}

	stopped := inst.State_()
	expected := stopped == ServiceStateStopping || stopped == ServiceStateStopped

	pluginID := s.getCanonicalPluginID(inst)

	s.notifyProcessExit(ProcessExitEvent{
		ProcessInstanceID: inst.ProcessInstanceID,
		ServiceID:         inst.ServiceID,
		RuntimeID:         inst.RuntimeID,
		InstanceID:        inst.InstanceID,
		ExtensionID:       inst.ExtensionID,
		PluginID:          pluginID,
		LogicalServiceID:  inst.LogicalServiceID,
		Generation:        inst.Generation,
		ExitCode:          exitCode,
		Expected:          expected,
		OccurredAt:        time.Now(),
		RestartCount:      inst.RestartCount,
		HostInstanceID:    inst.HostInstanceID,
		HostSessionID:     inst.HostSessionID,
		ModuleID:          inst.ModuleID,
	})

	if expected {
		return
	}

	inst.MarkCrashed()
	inst.circuit.RecordFailure()

	s.log("warn", "service process exited unexpectedly", map[string]any{
		"service":   inst.ServiceID,
		"instance":  inst.InstanceID,
		"exit_code": exitCode,
		"error":     inst.lastExitError,
	})

	if inst.Definition == nil {
		s.log("warn", "service crash with no definition; fail-closed, no restart", map[string]any{
			"service":   inst.ServiceID,
			"instance":  inst.InstanceID,
			"exit_code": exitCode,
		})
		inst.MarkCrashed()
		return
	}
	policy := inst.Definition.Recovery
	if policy.MaxRestarts <= 0 && policy.RecoveryDecisionMode != RecoveryDecisionExternal {
		s.log("warn", "service crash with zero max_restarts; fail-closed, no restart", map[string]any{
			"service":   inst.ServiceID,
			"instance":  inst.InstanceID,
			"exit_code": exitCode,
		})
		inst.SetState(ServiceStateDegraded)
		return
	}
	if policy.RecoveryDecisionMode == RecoveryDecisionExternal {
		s.log("warn", "service crash recorded; recovery decision delegated to external authority", map[string]any{
			"service":   inst.ServiceID,
			"instance":  inst.InstanceID,
			"exit_code": exitCode,
		})
	} else if inst.RestartCount < policy.MaxRestarts && inst.circuit.AllowStart() {
		delay := s.calculateBackoff(policy, inst.RestartCount)
		go func() {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				s.restart(inst)
			case <-inst.stopCh:
				return
			}
		}()
	} else if policy.QuarantineOnFail {
		s.log("error", "service quarantined due to frequent crashes", map[string]any{
			"service":   inst.ServiceID,
			"restarts":  inst.RestartCount,
			"exit_code": exitCode,
		})
		inst.SetState(ServiceStateQuarantined)
		_ = s.quarantine.Quarantine(inst.ServiceID, inst.InstanceID, QuarantineFrequentCrash,
			fmt.Sprintf("exited with code %d, restarts %d/%d", exitCode, inst.RestartCount, policy.MaxRestarts),
			map[string]any{"exit_code": exitCode, "restarts": inst.RestartCount})
	}
}

func (s *ProcessSupervisor) calculateBackoff(policy ServiceRecoveryPolicy, restartCount int) time.Duration {
	multiplier := policy.BackoffMultiplier
	if multiplier <= 0 {
		multiplier = 2
	}
	baseDelay := policy.RestartDelay
	if baseDelay <= 0 {
		baseDelay = 1 * time.Second
	}
	delay := baseDelay
	for i := 0; i < restartCount; i++ {
		delay = time.Duration(float64(delay) * multiplier)
	}
	maxDelay := policy.MaxRestartDelay
	if maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func (s *ProcessSupervisor) restart(inst *ServiceInstance) {
	inst.IncrementRestart()
	def := inst.Definition
	if def == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := s.Start(ctx, StartRequest{
		ServiceID:      inst.ServiceID,
		InstanceID:     newServiceInstanceID(inst.ServiceID),
		RuntimeID:      inst.RuntimeID,
		Generation:     inst.Generation,
		PublisherTrust: TrustLevel(def.TrustLevel),
		WorkingDir:     inst.WorkingDir,
		SessionToken:   inst.SessionToken,
		LogLevel:       "info",
	})
	if err != nil {
		s.log("error", "restart failed", map[string]any{"service": inst.ServiceID, "error": err.Error()})
		if inst.RestartCount >= def.Recovery.MaxRestarts {
			inst.SetState(ServiceStateQuarantined)
			_ = s.quarantine.Quarantine(inst.ServiceID, inst.InstanceID, QuarantineFrequentCrash,
				fmt.Sprintf("restart failed, restarts %d/%d", inst.RestartCount, def.Recovery.MaxRestarts),
				map[string]any{"restarts": inst.RestartCount})
		}
	} else {
		s.log("info", "service restarted", map[string]any{"service": inst.ServiceID, "restarts": inst.RestartCount})
	}
}

type StopRequest struct {
	ServiceID string
	Reason    string
	Force     bool
}

type StopResult struct {
	ServiceID string
	State     ServiceState
	StoppedAt time.Time
}

func (s *ProcessSupervisor) Stop(ctx context.Context, req StopRequest) (*StopResult, error) {
	s.mu.Lock()
	inst, exists := s.instances[req.ServiceID]
	s.mu.Unlock()
	if !exists {
		return nil, ErrServiceNotFound
	}
	if inst.State_().IsTerminal() {
		return &StopResult{ServiceID: req.ServiceID, State: inst.State_(), StoppedAt: time.Now().UTC()}, nil
	}

	inst.SetState(ServiceStateStopping)
	def := inst.Definition
	grace := def.Shutdown.GracePeriod
	if grace <= 0 {
		grace = 5 * time.Second
	}
	if req.Force {
		grace = 0
	}
	killTimeout := def.Shutdown.KillTimeout
	if killTimeout <= 0 {
		killTimeout = 10 * time.Second
	}

	s.healthMon.Stop(inst)

	totalTimeout := grace + killTimeout
	stopCtx, cancel := context.WithTimeout(ctx, totalTimeout)
	defer cancel()

	if inst.rpcSession != nil && !req.Force {
		_, shutdownCancel := context.WithTimeout(stopCtx, grace)
		_ = inst.rpcSession.Shutdown(grace)
		shutdownCancel()
	}

	if inst.protocolSession != nil {
		inst.protocolSession.Close()
	}

	exited := s.waitForProcessExit(inst, grace)

	if !exited {
		s.killProcessTree(inst)
		exited = s.waitForProcessExit(inst, killTimeout)
	}

	if !exited && s.procMgr.IsAlive(inst.PID) {
		s.log("error", "service process still alive after kill", map[string]any{
			"service": req.ServiceID,
			"pid":     inst.PID,
		})
		return nil, fmt.Errorf("trusted_service: process %d for service %s still alive after kill", inst.PID, req.ServiceID)
	}

	if def.Shutdown.RemoveTempDir {
		tempDir := filepath.Join(s.rootDir, "temp", req.ServiceID, inst.InstanceID)
		_ = os.RemoveAll(tempDir)
	}

	inst.MarkStopped()
	if s.ownerStore != nil && inst.ProcessInstanceID != "" {
		if err := s.ownerStore.RemoveOwnership(inst.ProcessInstanceID); err != nil {
			s.ownerStoreErr = err
			return nil, fmt.Errorf("trusted_service: persist ownership removal: %w", err)
		}
	}
	s.log("info", "service stopped", map[string]any{
		"service": req.ServiceID,
		"reason":  req.Reason,
		"force":   req.Force,
	})
	return &StopResult{ServiceID: req.ServiceID, State: ServiceStateStopped, StoppedAt: time.Now().UTC()}, nil
}

func (s *ProcessSupervisor) waitForProcessExit(inst *ServiceInstance, timeout time.Duration) bool {
	if timeout <= 0 {
		return !s.procMgr.IsAlive(inst.PID)
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !s.procMgr.IsAlive(inst.PID) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ticker.C:
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func (s *ProcessSupervisor) killProcessTree(inst *ServiceInstance) {
	pid := inst.PID
	handle := process.ProcessTreeHandle(inst.procHandle)

	if inst.rpcSession != nil {
		inst.rpcSession.Close()
	}

	if inst.protocolSession != nil {
		inst.protocolSession.Close()
	}

	if err := process.TerminateProcessTree(pid, handle); err != nil {
		s.log("warn", "terminate process tree failed, falling back to kill", map[string]any{
			"service": inst.ServiceID,
			"pid":     pid,
			"error":   err.Error(),
		})
		if pid > 0 {
			proc, err := os.FindProcess(pid)
			if err == nil {
				_ = proc.Kill()
			}
		}
	}
}

func (s *ProcessSupervisor) CleanupDurableOwnedProcess(ctx context.Context, processInstanceID string, requestedPID int, hostInstanceID string) error {
	if s.ownerStoreErr != nil {
		return fmt.Errorf("trusted_service: durable process owner store unavailable: %w", s.ownerStoreErr)
	}
	if s.ownerStore == nil {
		return fmt.Errorf("trusted_service: durable process owner store unavailable")
	}
	meta, ok := s.ownerStore.LookupByProcessInstanceID(processInstanceID)
	if !ok {
		return nil
	}
	if hostInstanceID != "" && meta.HostInstanceID != "" && meta.HostInstanceID != hostInstanceID {
		return fmt.Errorf("trusted_service: durable owner belongs to foreign host")
	}
	if requestedPID > 0 && meta.PID > 0 && requestedPID != meta.PID {
		return fmt.Errorf("trusted_service: durable owner PID mismatch requested=%d durable=%d", requestedPID, meta.PID)
	}
	if meta.PID <= 0 {
		return fmt.Errorf("trusted_service: durable owner has invalid PID")
	}
	identity, err := process.ReadProcessIdentity(meta.PID)
	if err != nil {
		return fmt.Errorf("trusted_service: unable to prove process identity for pid %d: %w", meta.PID, err)
	}
	if !process.SameProcessIdentity(meta.Executable, meta.ProcessStartID, identity) {
		return fmt.Errorf("trusted_service: process identity mismatch for pid %d; refusing cleanup", meta.PID)
	}
	if err := process.TerminateProcessTree(meta.PID, 0); err != nil {
		proc, findErr := os.FindProcess(meta.PID)
		if findErr != nil {
			return fmt.Errorf("trusted_service: find durable process: %w", findErr)
		}
		if killErr := proc.Kill(); killErr != nil {
			return fmt.Errorf("trusted_service: terminate durable process: %w", killErr)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for s.procMgr.IsAlive(meta.PID) {
		if time.Now().After(deadline) {
			return fmt.Errorf("trusted_service: durable process %d still alive", meta.PID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if err := s.ownerStore.RemoveOwnership(processInstanceID); err != nil {
		s.ownerStoreErr = err
		return fmt.Errorf("trusted_service: persist durable ownership cleanup: %w", err)
	}
	return nil
}

func (s *ProcessSupervisor) Get(serviceID string) (*ServiceInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, exists := s.instances[serviceID]
	if !exists {
		return nil, ErrServiceNotFound
	}
	return inst, nil
}

func (s *ProcessSupervisor) List() []*ServiceInstance {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*ServiceInstance, 0, len(s.instances))
	for _, inst := range s.instances {
		out = append(out, inst)
	}
	return out
}

func (s *ProcessSupervisor) ListDurableOwnedProcesses() []ProcessOwnerMetadata {
	if s.ownerStore == nil {
		return nil
	}
	return s.ownerStore.GetOwnedProcesses()
}

func (s *ProcessSupervisor) LookupDurableByRuntimeID(runtimeID string) []ProcessOwnerMetadata {
	if s.ownerStore == nil {
		return nil
	}
	return s.ownerStore.LookupByRuntimeID(runtimeID)
}

func (s *ProcessSupervisor) LookupDurableByPluginID(pluginID string) []ProcessOwnerMetadata {
	if s.ownerStore == nil {
		return nil
	}
	return s.ownerStore.LookupByPluginID(pluginID)
}

func (s *ProcessSupervisor) StopAll(ctx context.Context, reason string) {
	s.mu.Lock()
	ids := make([]string, 0, len(s.instances))
	for id := range s.instances {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(serviceID string) {
			defer wg.Done()
			_, _ = s.Stop(ctx, StopRequest{ServiceID: serviceID, Reason: reason, Force: false})
		}(id)
	}
	wg.Wait()
}

func (s *ProcessSupervisor) Quarantine(serviceID, reason string) error {
	s.mu.Lock()
	inst, exists := s.instances[serviceID]
	s.mu.Unlock()
	if !exists {
		return ErrServiceNotFound
	}
	inst.SetState(ServiceStateQuarantined)
	s.log("error", "service quarantined", map[string]any{
		"service": serviceID,
		"reason":  reason,
	})
	_ = s.quarantine.Quarantine(serviceID, inst.InstanceID, QuarantineHostAPIViolation, reason, nil)
	_, err := s.Stop(context.Background(), StopRequest{ServiceID: serviceID, Reason: reason, Force: true})
	return err
}

func (s *ProcessSupervisor) Invoke(ctx context.Context, serviceID, operation string, input json.RawMessage, timeout time.Duration) (*InvokeResult, error) {
	s.mu.Lock()
	inst, exists := s.instances[serviceID]
	s.mu.Unlock()
	if !exists {
		return nil, ErrServiceNotFound
	}
	if inst.State_() != ServiceStateReady {
		return nil, fmt.Errorf("trusted_service: service not ready (state=%s)", inst.State_())
	}
	if inst.rpcSession == nil {
		return nil, errors.New("trusted_service: service does not support RPC invocation")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	result, err := inst.rpcSession.Invoke(operation, input, timeout)
	if err != nil {
		inst.circuit.RecordFailure()
		return nil, err
	}
	inst.circuit.RecordSuccess()
	return result, nil
}

func (s *ProcessSupervisor) HealthCheck(ctx context.Context, serviceID string) (*HealthResult, error) {
	s.mu.Lock()
	inst, exists := s.instances[serviceID]
	s.mu.Unlock()
	if !exists {
		return nil, ErrServiceNotFound
	}
	if inst.rpcSession != nil {
		timeout := inst.Definition.HealthCheck.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		return inst.rpcSession.Health(timeout)
	}
	if s.procMgr.IsAlive(inst.PID) {
		return &HealthResult{Status: "healthy"}, nil
	}
	return &HealthResult{Status: "unhealthy"}, nil
}

func (s *ProcessSupervisor) log(level, msg string, fields map[string]any) {
	if s.logger != nil {
		s.logger(level, msg, fields)
	}
}

type HealthMonitor struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{cancels: make(map[string]context.CancelFunc)}
}

func (m *HealthMonitor) Monitor(inst *ServiceInstance, def *ServiceRuntimeDefinition) {
	if def.HealthCheck.Type == "" || def.HealthCheck.Type == "none" {
		return
	}
	interval := def.HealthCheck.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	timeout := def.HealthCheck.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	grace := def.HealthCheck.GracePeriod
	if grace <= 0 {
		grace = 10 * time.Second
	}
	maxFails := def.HealthCheck.MaxConsecutiveFails
	if maxFails <= 0 {
		maxFails = 3
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancels[inst.InstanceID] = cancel
	m.mu.Unlock()

	go func() {
		graceDeadline := time.Now().Add(grace)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Now().Before(graceDeadline) {
					continue
				}
				if !inst.State_().IsHealthy() {
					continue
				}
				checkCtx, checkCancel := context.WithTimeout(ctx, timeout)
				healthy := m.check(checkCtx, inst, def)
				checkCancel()
				if healthy {
					inst.RecordHealthSuccess()
					if inst.circuit != nil {
						inst.circuit.RecordSuccess()
					}
				} else {
					fails := inst.RecordHealthFail()
					if inst.circuit != nil {
						inst.circuit.RecordFailure()
					}
					if fails >= maxFails {
						inst.SetState(ServiceStateDegraded)
					}
				}
			}
		}
	}()
}

func (m *HealthMonitor) Stop(inst *ServiceInstance) {
	m.mu.Lock()
	cancel, ok := m.cancels[inst.InstanceID]
	if ok {
		delete(m.cancels, inst.InstanceID)
	}
	m.mu.Unlock()
	if ok {
		cancel()
	}
}

func (m *HealthMonitor) check(ctx context.Context, inst *ServiceInstance, def *ServiceRuntimeDefinition) bool {
	switch def.HealthCheck.Type {
	case "process":
		return procIsAlive(inst.PID)
	case "rpc":
		if inst.rpcSession == nil {
			return procIsAlive(inst.PID)
		}
		timeout := def.HealthCheck.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		result, err := inst.rpcSession.Health(timeout)
		if err != nil {
			return false
		}
		return result.Status == "healthy" || result.Status == "ok"
	case "none":
		return true
	default:
		return procIsAlive(inst.PID)
	}
}

func procIsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func defaultLogLevel(level string) string {
	if level == "" {
		return "info"
	}
	return level
}

func newServiceInstanceID(serviceID string) string {
	return fmt.Sprintf("%s-%d", serviceID, time.Now().UnixNano())
}

func buildProcessInstanceID(runtimeID, serviceID string) string {
	if serviceID == "" {
		return runtimeID
	}
	if runtimeID == "" {
		return serviceID
	}
	return runtimeID + "/" + serviceID
}

func extractLogicalServiceID(supervisorKey, runtimeID string) string {
	if supervisorKey == "" {
		return ""
	}
	prefix := runtimeID + "/"
	if runtimeID != "" && len(supervisorKey) > len(prefix) && supervisorKey[:len(prefix)] == prefix {
		return supervisorKey[len(prefix):]
	}
	return supervisorKey
}

func resolveCanonicalPluginID(reqPluginID, publisher, extensionID string) string {
	if reqPluginID != "" {
		return reqPluginID
	}
	if publisher != "" && extensionID != "" {
		return publisher + "/" + extensionID
	}
	if extensionID != "" {
		return extensionID
	}
	return ""
}

func resolveCanonicalLogicalServiceID(reqLogicalServiceID, serviceID, runtimeID string) string {
	if reqLogicalServiceID != "" {
		return reqLogicalServiceID
	}
	return extractLogicalServiceID(serviceID, runtimeID)
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type logWriter struct {
	logger  func(level, msg string, fields map[string]any)
	level   string
	service string
	mu      sync.Mutex
	buf     []byte
}

func newLogWriter(logger func(level, msg string, fields map[string]any), level, service string) *logWriter {
	return &logWriter{logger: logger, level: level, service: service}
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		idx := -1
		for i, b := range w.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		if w.logger != nil {
			w.logger(w.level, line, map[string]any{"service": w.service})
		}
	}
	return len(p), nil
}

var _ io.Writer = (*logWriter)(nil)
