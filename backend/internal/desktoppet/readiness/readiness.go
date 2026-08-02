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

func (s *ReadinessService) Register(checker ReadinessChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers = append(s.checkers, checker)
}

func (s *ReadinessService) Snapshot() ReadinessSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	checks := make(map[string]CheckResult)
	var blocking, degraded int
	overall := StatusReady
	for _, c := range s.checkers {
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
	svc.Register(NewComponentChecker("sqlite", true, makeSQLitePingChecker(db)))
	svc.Register(NewComponentChecker("surrealdb", true, makeSurrealPingChecker()))
	svc.Register(NewComponentChecker("qdrant", true, makeQdrantPingChecker()))
	svc.Register(NewComponentChecker("extension", true, makeExtensionPingChecker(ext)))
	return svc
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
		cfg := config.AppCfg.Surreal
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
