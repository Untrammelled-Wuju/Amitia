package resource

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type PendingRegistry interface {
	Register(req *PendingRegisterRequest) (bool, error)
	Release(key string)
	Count() int
}

type PendingRegisterRequest struct {
	RuntimeID  string
	ServiceID  string
	RequestID  string
	Fingerprint string
}

type BinaryRegistry interface {
	CountActive() int
	LimitActive() int
}

type RuntimeGovernor interface {
	ConfigureResourceLimits(runtimeID string, limits ServiceResourceLimitsSet) error
}

type ServiceResourceLimitsSet = trusted_service.ServiceResourceLimits

type ResourceAdmissionAdapter struct {
	mapper       *SubjectMapper
	pending      PendingRegistry
	binary       BinaryRegistry
	governor     RuntimeGovernor

	mu        sync.Mutex
	starting  map[string]time.Time // runtimeID -> startup start time
	stopping  map[string]bool      // runtimeID -> marked stopping
	shutdown  bool
}

func NewResourceAdmissionAdapter(
	mapper *SubjectMapper,
	pending PendingRegistry,
	binary BinaryRegistry,
	governor RuntimeGovernor,
) *ResourceAdmissionAdapter {
	return &ResourceAdmissionAdapter{
		mapper:   mapper,
		pending:  pending,
		binary:   binary,
		governor: governor,
		starting: make(map[string]time.Time),
		stopping: make(map[string]bool),
	}
}

func (a *ResourceAdmissionAdapter) AcquireRuntimeStartup(
	ctx context.Context,
	subj RuntimeIdentitySubject,
	profile *RuntimeResourceProfile,
) (StartupRevertFunc, error) {
	if err := a.checkHostState(subj.RuntimeID); err != nil {
		return nil, err
	}

	validated, err := a.mapper.Validate(subj.RuntimeID, subj.PluginID, subj.ServiceID, subj.Generation)
	if err != nil {
		return nil, err
	}

	if err := sanitizeProfile(profile); err != nil {
		return nil, err
	}

	if a.governor != nil && profile != nil {
		limits := ServiceResourceLimitsSet{
			MaxMemoryMB:        profile.MaxMemoryMB,
			MaxCPUPercent:      clampCPU(profile.MaxCPUPercent),
			MaxFileDescriptors: profile.MaxFileDescriptors,
			MaxDiskMB:          profile.MaxDiskMB,
			MaxSubprocesses:    profile.MaxSubprocesses,
		}
		if err := a.governor.ConfigureResourceLimits(subj.RuntimeID, limits); err != nil {
			return nil, ErrProfileInvalid
		}
	}

	a.mu.Lock()
	a.starting[validated.RuntimeID] = time.Now().UTC()
	a.mu.Unlock()

	return func() {
		a.mu.Lock()
		delete(a.starting, validated.RuntimeID)
		a.mu.Unlock()
	}, nil
}

func (a *ResourceAdmissionAdapter) AcquireRPCPending(
	ctx context.Context,
	subj RuntimeIdentitySubject,
) (AdmissionDecision, ReleaseFunc) {
	if err := a.checkHostState(subj.RuntimeID); err != nil {
		return decisionDenied(DenyPendingLimit, err), func() {}
	}

	if a.pending == nil || !a.subjectActive(subj.RuntimeID) {
		return decisionDenied(DenyPendingLimit, ErrRuntimeNotFound), func() {}
	}

	ok, err := a.pending.Register(&PendingRegisterRequest{
		RuntimeID:   subj.RuntimeID,
		ServiceID:   subj.ServiceID,
		RequestID:   subj.RuntimeID + "/" + subj.ServiceID + "/" + time.Now().Format("20060102T150405.000000000"),
		Fingerprint: "",
	})
	if err != nil || !ok {
		return decisionDenied(DenyPendingLimit, ErrResourceDenied), func() {}
	}

	return AdmissionDecision{Allowed: true, Class: ResourcePendingRPC}, func() {
		// Pending registry is canonical; real release occurs at Complete/Fail/Timeout/Cancel.
		// This is a logical slot marker — no-op to preserve double-release safety.
	}
}

func (a *ResourceAdmissionAdapter) AcquireBinaryObject(
	ctx context.Context,
	subj RuntimeIdentitySubject,
	requestedBytes int64,
) (AdmissionDecision, BinaryRevertFunc) {
	if err := a.checkHostState(subj.RuntimeID); err != nil {
		return decisionDenied(DenyBinaryCountLimit, err), func() {}
	}

	if a.binary == nil || !a.subjectActive(subj.RuntimeID) {
		return decisionDenied(DenyBinaryCountLimit, ErrRuntimeNotFound), func() {}
	}

	if requestedBytes < 0 {
		return decisionDenied(DenyBinaryBytesLimit, ErrOverflow), func() {}
	}

	current := int64(a.binary.CountActive())
	limit := int64(a.binary.LimitActive())
	if limit > 0 && current >= limit {
		return AdmissionDecision{
			Allowed: false,
			Reason:  DenyBinaryCountLimit,
			Class:   ResourceBinaryCount,
			Current: current,
			Limit:   limit,
		}, func() {}
	}

	return AdmissionDecision{Allowed: true, Class: ResourceBinaryCount, Current: current, Limit: limit}, func() {}
}

func (a *ResourceAdmissionAdapter) AcquireQueuePublish(
	ctx context.Context,
	subj RuntimeIdentitySubject,
) AdmissionDecision {
	if err := a.checkHostState(subj.RuntimeID); err != nil {
		return decisionDenied(DenyQueueLimit, err)
	}
	if !a.subjectActive(subj.RuntimeID) {
		return decisionDenied(DenyQueueLimit, ErrRuntimeNotFound)
	}
	return AdmissionDecision{Allowed: true, Class: ResourceQueue}
}

func (a *ResourceAdmissionAdapter) ActiveSubjects() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := make([]string, 0, len(a.starting))
	for id := range a.starting {
		ids = append(ids, id)
	}
	return ids
}

func (a *ResourceAdmissionAdapter) MarkStopping(runtimeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopping[runtimeID] = true
	a.deleteStartingLocked(runtimeID)
}

func (a *ResourceAdmissionAdapter) ClearStopping(runtimeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.stopping, runtimeID)
}

func (a *ResourceAdmissionAdapter) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdown = true
	a.starting = make(map[string]time.Time)
}

func (a *ResourceAdmissionAdapter) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdown = false
	a.starting = make(map[string]time.Time)
	a.stopping = make(map[string]bool)
}

func (a *ResourceAdmissionAdapter) checkHostState(runtimeID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.shutdown {
		return ErrHostShutdown
	}
	if a.stopping[runtimeID] {
		return ErrRuntimeStopping
	}
	return nil
}

func (a *ResourceAdmissionAdapter) subjectActive(runtimeID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.shutdown || a.stopping[runtimeID] {
		return false
	}
	_, ok := a.starting[runtimeID]
	return ok
}

func (a *ResourceAdmissionAdapter) deleteStartingLocked(runtimeID string) {
	delete(a.starting, runtimeID)
}

func decisionDenied(reason DenyReason, _ error) AdmissionDecision {
	return AdmissionDecision{Allowed: false, Reason: reason}
}

func sanitizeProfile(p *RuntimeResourceProfile) error {
	if p == nil {
		return nil
	}
	if p.MaxMemoryMB < 0 {
		return ErrProfileInvalid
	}
	if p.MaxDiskMB < 0 {
		return ErrProfileInvalid
	}
	if p.MaxFileDescriptors < 0 {
		return ErrProfileInvalid
	}
	if p.MaxSubprocesses < 0 {
		return ErrProfileInvalid
	}
	if !safeLte(0, int64(p.MaxMemoryMB), int64(p.MaxMemoryMB)) {
		return ErrOverflow
	}
	return nil
}

func clampCPU(cpu int) int {
	if cpu <= 0 {
		return 0
	}
	if cpu > 100 {
		return 100
	}
	return cpu
}

