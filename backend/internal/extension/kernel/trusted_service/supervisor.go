package trusted_service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type ProcessSupervisor struct {
	mu          sync.Mutex
	instances   map[string]*ServiceInstance
	defs        map[string]*ServiceRuntimeDefinition
	verifier    *BinaryVerifier
	selector    *PlatformSelector
	envBuilder  *EnvBuilder
	logger      func(level, msg string, fields map[string]any)
	healthMon   *HealthMonitor
	rootDir     string
}

func NewProcessSupervisor(rootDir string) *ProcessSupervisor {
	return &ProcessSupervisor{
		instances:  make(map[string]*ServiceInstance),
		defs:       make(map[string]*ServiceRuntimeDefinition),
		verifier:   NewBinaryVerifier(),
		selector:   NewPlatformSelector(),
		envBuilder: NewEnvBuilder(),
		healthMon:  NewHealthMonitor(),
		rootDir:    rootDir,
		logger:     func(level, msg string, fields map[string]any) {},
	}
}

func (s *ProcessSupervisor) SetLogger(l func(level, msg string, fields map[string]any)) {
	s.logger = l
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

type StartRequest struct {
	ServiceID     string
	InstanceID    string
	Generation    int64
	PublisherTrust TrustLevel
	BasePath      string
	WorkingDir    string
	SessionToken  string
	SecretLease   string
	LogLevel      string
	Args          map[string]string
}

type StartResult struct {
	InstanceID string
	PID        int
	State      ServiceState
	StartedAt  time.Time
	Generation int64
}

func (s *ProcessSupervisor) Start(ctx context.Context, req StartRequest) (*StartResult, error) {
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

	env := s.envBuilder.Build(exe, req.SessionToken, instanceID, req.Generation, tempDir, defaultLogLevel(req.LogLevel), req.SecretLease)
	cmd := exec.CommandContext(ctx, fullExePath, args...)
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = newLogWriter(s.logger, "info", req.ServiceID)
	cmd.Stderr = newLogWriter(s.logger, "warn", req.ServiceID)
	cmd.SysProcAttr = newSysProcAttr()

	instance := &ServiceInstance{
		InstanceID: instanceID,
		ServiceID:  req.ServiceID,
		Definition: def,
		Generation: req.Generation,
		Platform:   s.selector.current,
		Executable: exe,
		State:      ServiceStateStarting,
		WorkingDir: workingDir,
		SessionToken: req.SessionToken,
	}
	instance.SetState(ServiceStateStarting)

	if err := cmd.Start(); err != nil {
		instance.SetState(ServiceStateFailed)
		s.mu.Lock()
		s.instances[req.ServiceID] = instance
		s.mu.Unlock()
		return nil, fmt.Errorf("trusted_service: start process: %w", err)
	}

	instance.PID = cmd.Process.Pid
	instance.MarkStarted()

	s.mu.Lock()
	s.instances[req.ServiceID] = instance
	s.mu.Unlock()

	go s.watchProcess(instance, cmd)
	go s.healthMon.Monitor(instance, def)

	return &StartResult{
		InstanceID: instanceID,
		PID:        instance.PID,
		State:      ServiceStateReady,
		StartedAt:  *instance.StartedAt,
		Generation: req.Generation,
	}, nil
}

func (s *ProcessSupervisor) watchProcess(inst *ServiceInstance, cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		s.log("warn", "service process exited", map[string]any{
			"service": inst.ServiceID, "instance": inst.InstanceID, "error": err.Error(),
		})
	}
	stopped := inst.State_()
	if stopped == ServiceStateStopping || stopped == ServiceStateStopped {
		return
	}
	inst.MarkCrashed()
	if inst.Definition != nil {
		policy := inst.Definition.Recovery
		if inst.RestartCount < policy.MaxRestarts {
			multiplier := policy.BackoffMultiplier
			if multiplier <= 0 {
				multiplier = 1
			}
			backoff := 1.0
			for i := 0; i < inst.RestartCount; i++ {
				backoff *= multiplier
			}
			delay := time.Duration(float64(policy.RestartDelay) * backoff)
			if delay > policy.MaxRestartDelay {
				delay = policy.MaxRestartDelay
			}
			go func() {
				time.Sleep(delay)
				s.restart(inst)
			}()
		} else if policy.QuarantineOnFail {
			s.log("error", "service quarantined", map[string]any{
				"service": inst.ServiceID, "restarts": inst.RestartCount,
			})
			inst.SetState(ServiceStateQuarantined)
		}
	}
}

func (s *ProcessSupervisor) restart(inst *ServiceInstance) {
	inst.IncrementRestart()
	def := inst.Definition
	if def == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := s.Start(ctx, StartRequest{
		ServiceID:     inst.ServiceID,
		InstanceID:    newServiceInstanceID(inst.ServiceID),
		Generation:    inst.Generation,
		PublisherTrust: TrustLevel(def.TrustLevel),
		WorkingDir:    inst.WorkingDir,
		SessionToken:  inst.SessionToken,
		LogLevel:      "info",
	})
	if err != nil {
		s.log("error", "restart failed", map[string]any{"service": inst.ServiceID, "error": err.Error()})
		if inst.RestartCount >= def.Recovery.MaxRestarts {
			inst.SetState(ServiceStateQuarantined)
		}
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

	graceCtx, cancel := context.WithTimeout(ctx, grace+killTimeout)
	defer cancel()

	pid := inst.PID
	proc, err := os.FindProcess(pid)
	if err != nil {
		inst.MarkStopped()
		return &StopResult{ServiceID: req.ServiceID, State: ServiceStateStopped, StoppedAt: time.Now().UTC()}, nil
	}

	if !req.Force {
		if runtimeGOOS() == "windows" {
			_ = proc.Signal(syscall.SIGTERM)
		} else {
			_ = proc.Signal(syscall.SIGTERM)
		}
		select {
		case <-graceCtx.Done():
		case <-time.After(grace):
		}
		if procIsAlive(pid) {
			_ = proc.Kill()
		}
	} else {
		_ = proc.Kill()
	}
	if def.Shutdown.CleanupChildren {
		s.killProcessTree(pid)
	}
	if def.Shutdown.RemoveTempDir {
		tempDir := filepath.Join(s.rootDir, "temp", req.ServiceID, inst.InstanceID)
		_ = os.RemoveAll(tempDir)
	}
	inst.MarkStopped()
	return &StopResult{ServiceID: req.ServiceID, State: ServiceStateStopped, StoppedAt: time.Now().UTC()}, nil
}

func (s *ProcessSupervisor) killProcessTree(pid int) {
	if runtimeGOOS() == "windows" {
		cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T", "/F")
		_ = cmd.Run()
	}
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

func (s *ProcessSupervisor) StopAll(ctx context.Context, reason string) {
	s.mu.Lock()
	ids := make([]string, 0, len(s.instances))
	for id := range s.instances {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	for _, id := range ids {
		_, _ = s.Stop(ctx, StopRequest{ServiceID: id, Reason: reason, Force: false})
	}
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
		"service": serviceID, "reason": reason,
	})
	_, err := s.Stop(context.Background(), StopRequest{ServiceID: serviceID, Reason: reason, Force: true})
	return err
}

func (s *ProcessSupervisor) log(level, msg string, fields map[string]any) {
	if s.logger != nil {
		s.logger(level, msg, fields)
	}
}

type HealthMonitor struct {
	mu sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{cancels: make(map[string]context.CancelFunc)}
}

func (m *HealthMonitor) Monitor(inst *ServiceInstance, def *ServiceRuntimeDefinition) {
	if def.HealthCheck.Type == "" {
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
				} else {
					fails := inst.RecordHealthFail()
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
		if procIsAlive(inst.PID) {
			return true
		}
		return false
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
	if runtimeGOOS() == "windows" {
		return proc.Signal(syscall.Signal(0)) == nil
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func runtimeGOOS() string {
	return runtime.GOOS
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

func pow(base float64, exp float64) time.Duration {
	if base <= 0 {
		return 1
	}
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return time.Duration(result)
}

var _ = pow

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

func newSysProcAttr() *syscall.SysProcAttr {
	return newPlatformSysProcAttr()
}
