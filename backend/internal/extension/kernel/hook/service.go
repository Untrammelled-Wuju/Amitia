package hook

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type Service struct {
	PointRegistry *DefaultHookPointRegistry
	ContribStore  ContributionStore
	Pipeline      *Pipeline
	RuntimeBridge RuntimeBridge
	Permission    PermissionChecker
	Scope         ScopeChecker
	Dependency    DependencyChecker
	Trace         TraceRecorder
	Circuit       *CircuitBreaker
	DepthGuard    *DepthGuard
	Validator     *PatchValidator
	Integrator    *HostHookIntegrator
	ReadModel     *HookReadModel
	Lifecycle     *HookLifecycleManager
	Supervisor    runtime_supervisor.Supervisor
}

type ServiceConfig struct {
	DB                 *sql.DB
	Supervisor         runtime_supervisor.Supervisor
	PermissionBroker   permission.PermissionBroker
	ScopeManager       scope.ScopeManager
	DependencyResolver dependency.Resolver
	UseSQLite          bool
}

func NewService(cfg ServiceConfig) (*Service, error) {
	pointRegistry := NewHookPointRegistry()
	ctx := context.Background()
	if err := RegisterDefaultHookPoints(ctx, pointRegistry); err != nil {
		return nil, fmt.Errorf("hook: register default points: %w", err)
	}
	for _, p := range SystemOnlyHookPoints() {
		if err := pointRegistry.RegisterPoint(ctx, p); err != nil {
			return nil, fmt.Errorf("hook: register system point %s: %w", p.HookPointID, err)
		}
	}

	var contribStore ContributionStore
	var traceRecorder TraceRecorder

	if cfg.UseSQLite && cfg.DB != nil {
		contribRepo := NewHookContributionRepository(cfg.DB)
		contribStore = contribRepo
		traceRecorder = NewHookTraceRecorder(cfg.DB)
	} else {
		contribStore = NewMemoryContributionStore()
		traceRecorder = NopTraceRecorder{}
	}

	var runtimeBridge RuntimeBridge
	if cfg.Supervisor != nil {
		runtimeBridge = NewSupervisorRuntimeBridge(cfg.Supervisor, nil)
	} else {
		runtimeBridge = NewDirectRuntimeBridge()
	}

	var permChecker PermissionChecker = NopPermissionChecker{}
	if cfg.PermissionBroker != nil {
		permChecker = NewPermissionBrokerChecker(cfg.PermissionBroker)
	}

	var scopeChecker ScopeChecker = NopScopeChecker{}
	if cfg.ScopeManager != nil {
		scopeChecker = NewScopeManagerChecker(cfg.ScopeManager)
	}

	var depChecker DependencyChecker = NopDependencyChecker{}
	if cfg.DependencyResolver != nil {
		depChecker = NewDependencyResolverChecker(cfg.DependencyResolver)
	}

	circuit := NewCircuitBreaker()
	depthGuard := NewDepthGuard(DefaultMaxDepth)
	validator := NewPatchValidator()

	pipeline := NewPipeline(pointRegistry, contribStore, runtimeBridge)
	pipeline.Permission = permChecker
	pipeline.Scope = scopeChecker
	pipeline.Dependency = depChecker
	pipeline.Trace = traceRecorder
	pipeline.Circuit = circuit
	pipeline.DepthGuard = depthGuard
	pipeline.Validator = validator

	integrator := &HostHookIntegrator{Pipeline: pipeline}
	readModel := &HookReadModel{Pipeline: pipeline, Registry: pointRegistry}

	lifecycle := &HookLifecycleManager{
		PointRegistry: pointRegistry,
		ContribStore:  contribStore,
		Circuit:       circuit,
		RuntimeBridge: runtimeBridge,
		Permission:    permChecker,
		Scope:         scopeChecker,
		Dependency:    depChecker,
	}

	return &Service{
		PointRegistry: pointRegistry,
		ContribStore:  contribStore,
		Pipeline:      pipeline,
		RuntimeBridge: runtimeBridge,
		Permission:    permChecker,
		Scope:         scopeChecker,
		Dependency:    depChecker,
		Trace:         traceRecorder,
		Circuit:       circuit,
		DepthGuard:    depthGuard,
		Validator:     validator,
		Integrator:    integrator,
		ReadModel:     readModel,
		Lifecycle:     lifecycle,
		Supervisor:    cfg.Supervisor,
	}, nil
}

func (s *Service) Invoke(ctx context.Context, hookPointID string, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return s.Integrator.invokeHook(ctx, hookPointID, payload, hookCtx)
}

func (s *Service) Close() error {
	if s.Circuit != nil {
		s.Circuit.ResetAll()
	}
	return nil
}
