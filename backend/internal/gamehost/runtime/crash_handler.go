package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type CrashResolution string

const (
	CrashDeferToSupervisor CrashResolution = "defer_to_supervisor"
	CrashUnrecoverable     CrashResolution = "unrecoverable"
)

type CrashDecision struct {
	Resolution CrashResolution
	UpdateHealth bool
	Health    domain.HealthStatus
	Reason    string
}

type CrashContextAccessor interface {
	IsRuntimeStopping(runtimeID domain.RuntimeInstanceID) (bool, error)
	IsRuntimeTerminal(runtimeID domain.RuntimeInstanceID) (bool, error)
	IsRuntimeShutdown() bool
}

type CrashHandler interface {
	HandleProcessExit(ctx context.Context, event ProcessExitEvent) (CrashDecision, error)
}

type crashHandler struct {
	mu      sync.RWMutex
	context CrashContextAccessor
}

func NewCrashHandler(ctx CrashContextAccessor) CrashHandler {
	return &crashHandler{
		context: ctx,
	}
}

func (h *crashHandler) HandleProcessExit(ctx context.Context, event ProcessExitEvent) (CrashDecision, error) {
	if event.Expected {
		return CrashDecision{
			Resolution:  CrashDeferToSupervisor,
			UpdateHealth: true,
			Health:      domain.HealthUnknown,
			Reason:      "planned_stop",
		}, nil
	}

	h.mu.RLock()
	ctxAccessor := h.context
	h.mu.RUnlock()

	if ctxAccessor != nil {
		if ctxAccessor.IsRuntimeShutdown() {
			return CrashDecision{
				Resolution:  CrashDeferToSupervisor,
				UpdateHealth: false,
				Reason:      "backend_shutdown",
			}, nil
		}

		isStopping, err := ctxAccessor.IsRuntimeStopping(event.RuntimeID)
		if err == nil && isStopping {
			return CrashDecision{
				Resolution:  CrashDeferToSupervisor,
				UpdateHealth: false,
				Reason:      "runtime_stopping",
			}, nil
		}

		isTerminal, err := ctxAccessor.IsRuntimeTerminal(event.RuntimeID)
		if err == nil && isTerminal {
			return CrashDecision{
				Resolution:  CrashDeferToSupervisor,
				UpdateHealth: false,
				Reason:      "runtime_terminal",
			}, nil
		}
	}

	return CrashDecision{
		Resolution:   CrashDeferToSupervisor,
		UpdateHealth: true,
		Health:       domain.HealthUnhealthy,
		Reason:       "unexpected_exit",
	}, nil
}

func (h *crashHandler) UpdateContext(ctx CrashContextAccessor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.context = ctx
}

type CrashRecord struct {
	ServiceID  domain.ServiceID
	ExitCode   int
	OccurredAt time.Time
	Reason     string
}

type CrashRecorder interface {
	RecordCrash(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, exitCode int, reason string, now time.Time)
	GetLastCrash(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (*CrashRecord, bool)
}

type crashRecorder struct {
	mu      sync.RWMutex
	records map[domain.RuntimeInstanceID]map[domain.ServiceID]CrashRecord
}

func NewCrashRecorder() CrashRecorder {
	return &crashRecorder{
		records: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]CrashRecord),
	}
}

func (r *crashRecorder) RecordCrash(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, exitCode int, reason string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	services, ok := r.records[runtimeID]
	if !ok {
		services = make(map[domain.ServiceID]CrashRecord)
		r.records[runtimeID] = services
	}

	services[serviceID] = CrashRecord{
		ServiceID:  serviceID,
		ExitCode:   exitCode,
		OccurredAt: now,
		Reason:     truncateReason(reason),
	}
}

func (r *crashRecorder) GetLastCrash(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (*CrashRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services, ok := r.records[runtimeID]
	if !ok {
		return nil, false
	}
	rec, ok := services[serviceID]
	if !ok {
		return nil, false
	}
	return &CrashRecord{
		ServiceID:  rec.ServiceID,
		ExitCode:   rec.ExitCode,
		OccurredAt: rec.OccurredAt,
		Reason:     rec.Reason,
	}, true
}

func (r *crashRecorder) RemoveService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if services, ok := r.records[runtimeID]; ok {
		delete(services, serviceID)
		if len(services) == 0 {
			delete(r.records, runtimeID)
		}
	}
}

func ValidateProcessExit(expected bool, exitCode int) CrashDecision {
	if expected {
		return CrashDecision{
			Resolution:   CrashDeferToSupervisor,
			UpdateHealth: true,
			Health:       domain.HealthUnknown,
			Reason:       "planned_stop",
		}
	}

	return CrashDecision{
		Resolution:   CrashDeferToSupervisor,
		UpdateHealth: true,
		Health:       domain.HealthUnhealthy,
		Reason:       "unexpected_exit",
	}
}
