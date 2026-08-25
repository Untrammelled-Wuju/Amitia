package resource

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type PendingRegistry interface {
	Count() int
	CountByPeer(runtimeID, serviceID string) int
	LimitPerPeer() int
	LimitGlobal() int
}

type BinaryRegistry interface {
	CountActive() int
	LimitActive() int
	ActiveBytes() int64
	LimitActiveBytes() int64
}

type RuntimeGovernor interface {
	ConfigureResourceLimits(runtimeID, serviceID string, limits ServiceResourceLimitsSet) error
	ClearServiceResourceLimits(runtimeID, serviceID string)
	ClearRuntimeResourceLimits(runtimeID string)
	ClearAllResourceLimits()
}

type ServiceResourceLimitsSet = trusted_service.ServiceResourceLimits

const defaultQueuePublishLimit = 256

type ResourceAdmissionAdapter struct {
	mapper   *SubjectMapper
	pending  PendingRegistry
	binary   BinaryRegistry
	governor RuntimeGovernor

	mu            sync.Mutex
	starting      map[string]time.Time
	stopping      map[string]bool
	queueInFlight map[string]int
	queueLimit    int
	shutdown      bool
}

func NewResourceAdmissionAdapter(mapper *SubjectMapper, pending PendingRegistry, binary BinaryRegistry, governor RuntimeGovernor) *ResourceAdmissionAdapter {
	return &ResourceAdmissionAdapter{
		mapper:        mapper,
		pending:       pending,
		binary:        binary,
		governor:      governor,
		starting:      make(map[string]time.Time),
		stopping:      make(map[string]bool),
		queueInFlight: make(map[string]int),
		queueLimit:    defaultQueuePublishLimit,
	}
}

func (a *ResourceAdmissionAdapter) AcquireRuntimeStartup(ctx context.Context, subj RuntimeIdentitySubject, profile *RuntimeResourceProfile) (StartupRevertFunc, error) {
	if err := a.checkHostState(subj.RuntimeID); err != nil {
		return nil, err
	}
	if a.mapper == nil {
		return nil, ErrSubjectInvalid
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
			MaxMemoryMB: profile.MaxMemoryMB, MaxCPUPercent: clampCPU(profile.MaxCPUPercent),
			MaxFileDescriptors: profile.MaxFileDescriptors, MaxDiskMB: profile.MaxDiskMB,
			MaxSubprocesses: profile.MaxSubprocesses,
		}
		if err := a.governor.ConfigureResourceLimits(subj.RuntimeID, subj.ServiceID, limits); err != nil {
			return nil, ErrProfileInvalid
		}
	}
	startupKey := subjectKey(validated.RuntimeID, subj.ServiceID)
	a.mu.Lock()
	a.starting[startupKey] = time.Now().UTC()
	a.mu.Unlock()
	return func() {
		a.mu.Lock()
		delete(a.starting, startupKey)
		a.mu.Unlock()
		if a.governor != nil {
			a.governor.ClearServiceResourceLimits(subj.RuntimeID, subj.ServiceID)
		}
	}, nil
}

// CommitRuntimeStartup ends transient per-service startup accounting while
// retaining configured limits for the lifetime of the running process.
func (a *ResourceAdmissionAdapter) CommitRuntimeStartup(runtimeID, serviceID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.starting, subjectKey(runtimeID, serviceID))
}

// ReleaseService clears transient accounting and the declarative resource
// profile for one stopped service without disturbing sibling services in the
// same runtime.
func (a *ResourceAdmissionAdapter) ReleaseService(runtimeID, serviceID string) {
	if a == nil {
		return
	}
	if a.governor != nil {
		a.governor.ClearServiceResourceLimits(runtimeID, serviceID)
	}
	key := subjectKey(runtimeID, serviceID)
	a.mu.Lock()
	delete(a.starting, key)
	delete(a.queueInFlight, key)
	a.mu.Unlock()
}

func (a *ResourceAdmissionAdapter) validateSubject(subj RuntimeIdentitySubject) (RuntimeIdentitySubject, error) {
	if err := a.checkHostState(subj.RuntimeID); err != nil {
		return subj, err
	}
	if a.mapper == nil {
		return subj, ErrSubjectInvalid
	}
	return a.mapper.Validate(subj.RuntimeID, subj.PluginID, subj.ServiceID, subj.Generation)
}

func (a *ResourceAdmissionAdapter) AcquireRPCPending(ctx context.Context, subj RuntimeIdentitySubject) (AdmissionDecision, ReleaseFunc) {
	if _, err := a.validateSubject(subj); err != nil {
		return decisionDenied(DenyPendingLimit, err), func() {}
	}
	if a.pending == nil {
		return decisionDenied(DenyPendingLimit, ErrResourceDenied), func() {}
	}
	peerCurrent := int64(a.pending.CountByPeer(subj.RuntimeID, subj.ServiceID))
	peerLimit := int64(a.pending.LimitPerPeer())
	globalCurrent := int64(a.pending.Count())
	globalLimit := int64(a.pending.LimitGlobal())
	if peerLimit > 0 && peerCurrent >= peerLimit {
		return AdmissionDecision{Allowed: false, Reason: DenyPendingLimit, Class: ResourcePendingRPC, Current: peerCurrent, Limit: peerLimit}, func() {}
	}
	if globalLimit > 0 && globalCurrent >= globalLimit {
		return AdmissionDecision{Allowed: false, Reason: DenyPendingLimit, Class: ResourcePendingRPC, Current: globalCurrent, Limit: globalLimit}, func() {}
	}
	// The canonical rpc.PendingRequestRegistry owns the actual reservation.
	// Admission only performs identity and capacity checks, avoiding duplicate
	// synthetic pending entries and double-release races.
	return AdmissionDecision{Allowed: true, Class: ResourcePendingRPC, Current: peerCurrent, Limit: peerLimit}, func() {}
}

func (a *ResourceAdmissionAdapter) AcquireBinaryObject(ctx context.Context, subj RuntimeIdentitySubject, requestedBytes int64) (AdmissionDecision, BinaryRevertFunc) {
	if _, err := a.validateSubject(subj); err != nil {
		return decisionDenied(DenyBinaryCountLimit, err), func() {}
	}
	if a.binary == nil {
		return decisionDenied(DenyBinaryCountLimit, ErrResourceDenied), func() {}
	}
	if requestedBytes < 0 {
		return decisionDenied(DenyBinaryBytesLimit, ErrOverflow), func() {}
	}
	currentCount, countLimit := int64(a.binary.CountActive()), int64(a.binary.LimitActive())
	if countLimit > 0 && currentCount >= countLimit {
		return AdmissionDecision{Allowed: false, Reason: DenyBinaryCountLimit, Class: ResourceBinaryCount, Current: currentCount, Limit: countLimit}, func() {}
	}
	currentBytes, byteLimit := a.binary.ActiveBytes(), a.binary.LimitActiveBytes()
	if requestedBytes > 0 && byteLimit > 0 && (currentBytes > byteLimit-requestedBytes) {
		return AdmissionDecision{Allowed: false, Reason: DenyBinaryBytesLimit, Class: ResourceBinaryBytes, Current: currentBytes, Limit: byteLimit}, func() {}
	}
	return AdmissionDecision{Allowed: true, Class: ResourceBinaryBytes, Current: currentBytes, Limit: byteLimit}, func() {}
}

func (a *ResourceAdmissionAdapter) AcquireQueuePublish(ctx context.Context, subj RuntimeIdentitySubject) (AdmissionDecision, ReleaseFunc) {
	if _, err := a.validateSubject(subj); err != nil {
		return decisionDenied(DenyQueueLimit, err), func() {}
	}
	key := subj.RuntimeID + "/" + subj.ServiceID
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.shutdown || a.stopping[subj.RuntimeID] {
		return decisionDenied(DenyQueueLimit, ErrRuntimeStopping), func() {}
	}
	current := a.queueInFlight[key]
	if a.queueLimit > 0 && current >= a.queueLimit {
		return AdmissionDecision{Allowed: false, Reason: DenyQueueLimit, Class: ResourceQueue, Current: int64(current), Limit: int64(a.queueLimit)}, func() {}
	}
	a.queueInFlight[key] = current + 1
	var once sync.Once
	return AdmissionDecision{Allowed: true, Class: ResourceQueue, Current: int64(current), Limit: int64(a.queueLimit)}, func() {
		once.Do(func() {
			a.mu.Lock()
			if a.queueInFlight[key] <= 1 {
				delete(a.queueInFlight, key)
			} else {
				a.queueInFlight[key]--
			}
			a.mu.Unlock()
		})
	}
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

func (a *ResourceAdmissionAdapter) RuntimeIDsByExtension(extensionID string) []string {
	if a == nil || a.mapper == nil {
		return nil
	}
	return a.mapper.RuntimeIDsByExtension(extensionID)
}

func (a *ResourceAdmissionAdapter) QueueUsage(runtimeID, serviceID string) (current, limit int) {
	if a == nil {
		return 0, 0
	}
	key := runtimeID + "/" + serviceID
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queueInFlight[key], a.queueLimit
}

func (a *ResourceAdmissionAdapter) MarkStopping(runtimeID string) {
	if a.governor != nil {
		a.governor.ClearRuntimeResourceLimits(runtimeID)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopping[runtimeID] = true
	for key := range a.starting {
		if hasRuntimePrefix(key, runtimeID) {
			delete(a.starting, key)
		}
	}
	for key := range a.queueInFlight {
		if hasRuntimePrefix(key, runtimeID) {
			delete(a.queueInFlight, key)
		}
	}
}
func (a *ResourceAdmissionAdapter) ClearStopping(runtimeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.stopping, runtimeID)
}
func (a *ResourceAdmissionAdapter) Shutdown() {
	if a.governor != nil {
		a.governor.ClearAllResourceLimits()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdown = true
	a.starting = make(map[string]time.Time)
	a.queueInFlight = make(map[string]int)
}
func (a *ResourceAdmissionAdapter) Reset() {
	if a.governor != nil {
		a.governor.ClearAllResourceLimits()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdown = false
	a.starting = make(map[string]time.Time)
	a.stopping = make(map[string]bool)
	a.queueInFlight = make(map[string]int)
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
func subjectKey(runtimeID, serviceID string) string {
	return runtimeID + "/" + serviceID
}

func hasRuntimePrefix(key, runtimeID string) bool {
	return len(key) > len(runtimeID) && key[:len(runtimeID)] == runtimeID && key[len(runtimeID)] == '/'
}

func decisionDenied(reason DenyReason, _ error) AdmissionDecision {
	return AdmissionDecision{Allowed: false, Reason: reason}
}
func sanitizeProfile(p *RuntimeResourceProfile) error {
	if p == nil {
		return nil
	}
	if p.MaxMemoryMB < 0 || p.MaxCPUPercent < 0 || p.MaxDiskMB < 0 || p.MaxFileDescriptors < 0 || p.MaxSubprocesses < 0 {
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
