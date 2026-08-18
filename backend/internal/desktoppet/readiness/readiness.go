// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package readiness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/pkg/database/qdrant"
	"gorm.io/gorm"
)

type SystemStatus string

const (
	StatusReady       SystemStatus = "ready"
	StatusDegraded    SystemStatus = "degraded"
	StatusBlocked     SystemStatus = "blocked"
	StatusMaintenance SystemStatus = "maintenance"
)

type CheckResult struct {
	Name     string       `json:"name"`
	Status   SystemStatus `json:"status"`
	Required bool         `json:"required"`
	Message  string       `json:"message,omitempty"`
	Duration int64        `json:"durationMs"`
}

type ReadinessSnapshot struct {
	OverallStatus SystemStatus           `json:"overallStatus"`
	Checks        map[string]CheckResult `json:"checks"`
	BlockingCount int                    `json:"blockingCount"`
	DegradedCount int                    `json:"degradedCount"`
	Timestamp     string                 `json:"timestamp"`
}

type ReadinessChecker interface {
	Name() string
	IsRequired() bool
	Evaluate() (SystemStatus, string)
}

type ReadinessService struct {
	mu       sync.RWMutex
	checkers []ReadinessChecker
	nowFn    func() time.Time
}

func NewReadinessService() *ReadinessService {
	return &ReadinessService{
		checkers: make([]ReadinessChecker, 0),
		nowFn:    time.Now,
	}
}

func (s *ReadinessService) Register(checker ReadinessChecker) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.checkers {
		if existing.Name() == checker.Name() {
			return fmt.Errorf("duplicate readiness checker: %s", checker.Name())
		}
	}
	s.checkers = append(s.checkers, checker)
	return nil
}

func (s *ReadinessService) checkerNames() map[string]bool {
	names := make(map[string]bool, len(s.checkers))
	for _, c := range s.checkers {
		names[c.Name()] = true
	}
	return names
}

var RequiredCheckerNames = []string{
	"sqlite",
	"extension",
	"desktop_session",
	"ownership_guard",
	"runtime_ticket",
	"runtime_gateway",
	"path_guard",
	"generation_worker",
	"processing_worker",
	"quality_worker",
	"installation_worker",
	"behavior_worker",
	"migration_state",
	"legacy_chain",
	"canonical_cutover",
	"mesh",
}

func (s *ReadinessService) Snapshot() ReadinessSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	checks := make(map[string]CheckResult)
	registered := make(map[string]bool, len(s.checkers))
	var blocking, degraded int
	overall := StatusReady

	if len(s.checkers) == 0 {
		overall = StatusBlocked
		checks["_meta"] = CheckResult{
			Name:     "_meta",
			Status:   StatusBlocked,
			Required: true,
			Message:  "zero checkers registered",
		}
		return ReadinessSnapshot{
			OverallStatus: overall,
			Checks:        checks,
			BlockingCount: 1,
			DegradedCount: 0,
			Timestamp:     s.nowFn().UTC().Format(time.RFC3339Nano),
		}
	}

	for _, c := range s.checkers {
		registered[c.Name()] = true
		start := s.nowFn()
		status, message := c.Evaluate()
		duration := s.nowFn().Sub(start).Milliseconds()
		result := CheckResult{
			Name:     c.Name(),
			Status:   status,
			Required: c.IsRequired(),
			Message:  message,
			Duration: duration,
		}
		checks[c.Name()] = result
		switch status {
		case StatusBlocked:
			if c.IsRequired() {
				blocking++
			} else {
				degraded++
			}
		case StatusDegraded:
			degraded++
		}
	}

	for _, name := range RequiredCheckerNames {
		if !registered[name] {
			blocking++
			checks[name] = CheckResult{
				Name:     name,
				Status:   StatusBlocked,
				Required: true,
				Message:  "required checker is not registered",
			}
		}
	}

	if blocking > 0 {
		overall = StatusBlocked
	} else if degraded > 0 {
		overall = StatusDegraded
	}
	return ReadinessSnapshot{
		OverallStatus: overall,
		Checks:        checks,
		BlockingCount: blocking,
		DegradedCount: degraded,
		Timestamp:     s.nowFn().UTC().Format(time.RFC3339Nano),
	}
}

type MaintenanceGateFunc func() bool

type SafeModeController struct {
	mu         sync.RWMutex
	inSafeMode bool
	reason     string
	setAt      time.Time
}

func NewSafeModeController() *SafeModeController {
	return &SafeModeController{}
}

func (s *SafeModeController) Enter(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inSafeMode = true
	s.reason = reason
	s.setAt = time.Now()
}

func (s *SafeModeController) Exit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inSafeMode = false
	s.reason = ""
	s.setAt = time.Time{}
}

func (s *SafeModeController) IsInSafeMode() (bool, string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inSafeMode, s.reason, s.setAt
}

type WorkerRegistryEntry struct {
	WorkerName     string    `json:"workerName"`
	Required       bool      `json:"required"`
	StartedAt      time.Time `json:"startedAt"`
	LastHeartbeat  time.Time `json:"lastHeartbeat"`
	Status         string    `json:"status"`
	DependencyHash string    `json:"dependencyHash,omitempty"`
}

type WorkerRegistry struct {
	mu      sync.RWMutex
	entries map[string]WorkerRegistryEntry
	nowFn   func() time.Time
}

func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		entries: make(map[string]WorkerRegistryEntry),
		nowFn:   time.Now,
	}
}

func (r *WorkerRegistry) Register(name string, required bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = WorkerRegistryEntry{
		WorkerName: name,
		Required:   required,
		StartedAt:  r.nowFn(),
		Status:     "started",
	}
}

func (r *WorkerRegistry) Heartbeat(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[name]; ok {
		entry.LastHeartbeat = r.nowFn()
		entry.Status = "running"
		r.entries[name] = entry
	}
}

func (r *WorkerRegistry) Stop(name string, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[name]; ok {
		entry.LastHeartbeat = r.nowFn()
		entry.Status = status
		r.entries[name] = entry
	}
}

func (r *WorkerRegistry) GetStatus() []WorkerRegistryEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]WorkerRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		result = append(result, entry)
	}
	return result
}

func (r *WorkerRegistry) AllRequiredHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.entries {
		if entry.Required && entry.Status != "started" && entry.Status != "running" {
			return false
		}
	}
	return true
}

type ComponentChecker struct {
	checkerName string
	required    bool
	evaluate    func() (SystemStatus, string)
}

func NewComponentChecker(name string, required bool, fn func() (SystemStatus, string)) *ComponentChecker {
	return &ComponentChecker{
		checkerName: name,
		required:    required,
		evaluate:    fn,
	}
}

func (c *ComponentChecker) Name() string { return c.checkerName }

func (c *ComponentChecker) IsRequired() bool { return c.required }

func (c *ComponentChecker) Evaluate() (SystemStatus, string) { return c.evaluate() }

func NewStartupReadinessService(db *gorm.DB, ext *extension.Runtime) *ReadinessService {
	svc := NewReadinessService()
	_ = svc.Register(NewComponentChecker("sqlite", true, makeSQLitePingChecker(db)))
	_ = svc.Register(NewComponentChecker("extension", true, makeExtensionPingChecker(ext)))
	return svc
}

type StartupReadinessDeps struct {
	DB                      *gorm.DB
	Extension               *extension.Runtime
	DesktopSessionReady     func() error
	OwnershipReady          func() error
	RuntimeTicketReady      func() error
	RuntimeGatewayReady     func() error
	PathGuardReady          func() error
	GenerationWorkerReady   func() error
	ProcessingWorkerReady   func() error
	QualityWorkerReady      func() error
	InstallationWorkerReady func() error
	BehaviorWorkerReady     func() error
	MigrationReady          func() error
	LegacyChainReady        func() error
	CanonicalCutoverReady   func() error
	MeshReady               func() error
}

func NewFullStartupReadinessService(deps StartupReadinessDeps) (*ReadinessService, error) {
	svc := NewReadinessService()
	register := func(name string, required bool, fn func() (SystemStatus, string)) error {
		return svc.Register(NewComponentChecker(name, required, fn))
	}
	if err := register("sqlite", true, makeSQLitePingChecker(deps.DB)); err != nil {
		return nil, err
	}
	if err := register("extension", true, func() (SystemStatus, string) {
		if deps.Extension == nil {
			return StatusBlocked, "extension: runtime is not initialized"
		}
		return StatusReady, "extension: runtime is operational"
	}); err != nil {
		return nil, err
	}
	if err := register("desktop_session", true, wrapReadyFunc(deps.DesktopSessionReady, "desktop_session")); err != nil {
		return nil, err
	}
	if err := register("ownership_guard", true, wrapReadyFunc(deps.OwnershipReady, "ownership_guard")); err != nil {
		return nil, err
	}
	if err := register("runtime_ticket", true, wrapReadyFunc(deps.RuntimeTicketReady, "runtime_ticket")); err != nil {
		return nil, err
	}
	if err := register("runtime_gateway", true, wrapReadyFunc(deps.RuntimeGatewayReady, "runtime_gateway")); err != nil {
		return nil, err
	}
	if err := register("path_guard", true, wrapReadyFunc(deps.PathGuardReady, "path_guard")); err != nil {
		return nil, err
	}
	if err := register("generation_worker", true, wrapReadyFunc(deps.GenerationWorkerReady, "generation_worker")); err != nil {
		return nil, err
	}
	if err := register("processing_worker", true, wrapReadyFunc(deps.ProcessingWorkerReady, "processing_worker")); err != nil {
		return nil, err
	}
	if err := register("quality_worker", true, wrapReadyFunc(deps.QualityWorkerReady, "quality_worker")); err != nil {
		return nil, err
	}
	if err := register("installation_worker", true, wrapReadyFunc(deps.InstallationWorkerReady, "installation_worker")); err != nil {
		return nil, err
	}
	if err := register("behavior_worker", true, wrapReadyFunc(deps.BehaviorWorkerReady, "behavior_worker")); err != nil {
		return nil, err
	}
	if err := register("migration_state", true, wrapReadyFunc(deps.MigrationReady, "migration_state")); err != nil {
		return nil, err
	}
	if err := register("legacy_chain", true, wrapReadyFunc(deps.LegacyChainReady, "legacy_chain")); err != nil {
		return nil, err
	}
	if err := register("canonical_cutover", true, wrapReadyFunc(deps.CanonicalCutoverReady, "canonical_cutover")); err != nil {
		return nil, err
	}
	if err := register("mesh", true, wrapReadyFunc(deps.MeshReady, "mesh")); err != nil {
		return nil, err
	}
	return svc, nil
}

func wrapReadyFunc(fn func() error, name string) func() (SystemStatus, string) {
	return func() (SystemStatus, string) {
		if fn == nil {
			return StatusBlocked, name + ": ready checker not provided"
		}
		if err := fn(); err != nil {
			return StatusBlocked, name + ": " + err.Error()
		}
		return StatusReady, name + ": ready"
	}
}

func makeSQLitePingChecker(db *gorm.DB) func() (SystemStatus, string) {
	return func() (SystemStatus, string) {
		if db == nil {
			return StatusBlocked, "sqlite: database instance is nil"
		}
		sqlDB, err := db.DB()
		if err != nil {
			return StatusBlocked, fmt.Sprintf("sqlite: failed to get underlying *sql.DB: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			return StatusBlocked, fmt.Sprintf("sqlite: ping failed: %v", err)
		}
		return StatusReady, "sqlite: connection is healthy"
	}
}

func makeSurrealPingChecker() func() (SystemStatus, string) {
	return func() (SystemStatus, string) {
		cfg := config.AppCfg.Providers.GraphStore.SurrealDB
		url := fmt.Sprintf("ws://%s:%d/rpc", cfg.Host, cfg.Port)
		db, err := surrealdb.New(url)
		if err != nil {
			return StatusBlocked, fmt.Sprintf("surrealdb: connection failed: %v", err)
		}
		defer db.Close(context.Background())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := db.SignIn(ctx, map[string]string{"user": cfg.Username, "pass": cfg.Password}); err != nil {
			if _, err2 := db.SignIn(ctx, map[string]string{"user": "root", "pass": "root"}); err2 != nil {
				return StatusBlocked, fmt.Sprintf("surrealdb: authentication failed: %v", err)
			}
		}

		if err := db.Use(ctx, cfg.Namespace, cfg.Database); err != nil {
			return StatusBlocked, fmt.Sprintf("surrealdb: use namespace/database failed: %v", err)
		}

		_, err = surrealdb.Query[any](ctx, db, "SELECT 1", nil)
		if err != nil {
			return StatusDegraded, fmt.Sprintf("surrealdb: query test failed: %v", err)
		}
		return StatusReady, "surrealdb: connection is healthy"
	}
}

func makeQdrantPingChecker() func() (SystemStatus, string) {
	return func() (SystemStatus, string) {
		if qdrant.Client == nil {
			return StatusBlocked, "qdrant: client is not initialized"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := qdrant.Client.HealthCheck(ctx)
		if err != nil {
			return StatusBlocked, fmt.Sprintf("qdrant: health check failed: %v", err)
		}
		return StatusReady, "qdrant: connection is healthy"
	}
}

func makeExtensionPingChecker(ext *extension.Runtime) func() (SystemStatus, string) {
	return func() (SystemStatus, string) {
		if ext == nil {
			return StatusBlocked, "extension: runtime is not initialized"
		}
		if ext.Kernel == nil {
			return StatusBlocked, "extension: kernel is not attached"
		}
		container := ext.Kernel.Container()
		if container == nil {
			return StatusBlocked, "extension: kernel has no container"
		}
		return StatusReady, "extension: runtime and kernel are operational"
	}
}
