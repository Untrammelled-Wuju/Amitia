package lifecycle

import (
	"context"
	"sync"
	"time"
)

type ReadinessReport struct {
	Ready              bool
	StartupID          string
	Reason             string
	ComponentStates    map[string]string
	FailedComponents   []string
	MissingComponents  []string
	DegradedComponents []string
	CheckedAt          time.Time
}

type ReadinessService struct {
	mu            sync.RWMutex
	registry      *ComponentRegistry
	requiredCore  map[string]struct{}
	ready         bool
	startupID     string
	currentReport ReadinessReport
	audit         LifecycleAuditWriter
}

func NewReadinessService(registry *ComponentRegistry, audit LifecycleAuditWriter) *ReadinessService {
	return &ReadinessService{
		registry:     registry,
		requiredCore: make(map[string]struct{}),
		audit:        audit,
	}
}

func (r *ReadinessService) MarkCore(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requiredCore[id] = struct{}{}
}

func (r *ReadinessService) IsReady() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ready
}

func (r *ReadinessService) SetReady(startupID string, ready bool, reason string) {
	r.mu.Lock()
	r.ready = ready
	r.startupID = startupID
	r.currentReport.Ready = ready
	r.currentReport.Reason = reason
	r.currentReport.StartupID = startupID
	r.currentReport.CheckedAt = now()
	audit := r.audit
	r.mu.Unlock()
	if audit != nil {
		audit.RecordReadinessEvent(context.Background(), ReadinessAuditEvent{
			Ready:     ready,
			StartupID: startupID,
			Reason:    reason,
			Timestamp: now(),
		})
	}
}

func (r *ReadinessService) Check(ctx context.Context) ReadinessReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	report := ReadinessReport{
		StartupID:       r.startupID,
		ComponentStates: make(map[string]string),
		CheckedAt:       now(),
	}
	coreMissing := false
	for _, component := range r.registry.All() {
		health := component.Health(ctx)
		report.ComponentStates[component.ID()] = string(health.State)
		switch health.State {
		case ComponentStateReady:
		case ComponentStateDegraded:
			report.DegradedComponents = append(report.DegradedComponents, component.ID())
		case ComponentStateFailed, ComponentStateQuarantined:
			report.FailedComponents = append(report.FailedComponents, component.ID())
			if _, core := r.requiredCore[component.ID()]; core {
				coreMissing = true
			}
		case ComponentStateSkipped:
			if _, core := r.requiredCore[component.ID()]; core {
				coreMissing = true
			}
		}
	}
	for id := range r.requiredCore {
		if _, ok := r.registry.Get(id); !ok {
			report.MissingComponents = append(report.MissingComponents, id)
			coreMissing = true
		}
	}
	if coreMissing {
		report.Ready = false
		report.Reason = "core_components_not_ready"
	} else if len(report.FailedComponents) > 0 {
		report.Ready = false
		report.Reason = "failed_components_present"
	} else {
		report.Ready = true
	}
	r.currentReport = report
	r.ready = report.Ready
	if r.audit != nil {
		r.audit.RecordReadinessEvent(ctx, ReadinessAuditEvent{
			Ready:     report.Ready,
			StartupID: report.StartupID,
			Reason:    report.Reason,
			Timestamp: report.CheckedAt,
		})
	}
	return report
}

func (r *ReadinessService) LastReport() ReadinessReport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentReport
}
