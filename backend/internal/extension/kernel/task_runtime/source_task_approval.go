package task_runtime

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type SourceTaskApprovalBinding struct {
	Scope           coordination.ExecutionScope `json:"executionScope"`
	TaskRunID       string                      `json:"taskRunId"`
	TaskGeneration  int64                       `json:"taskGeneration"`
	InputHash       string                      `json:"inputHash"`
	Target          TargetTaskDefinitionPin     `json:"target"`
	ExecutionTarget TaskExecutionTarget         `json:"executionTarget"`
}

type SourceTaskApproval struct {
	ID        string                    `json:"id"`
	Binding   SourceTaskApprovalBinding `json:"binding"`
	Status    string                    `json:"status"`
	Revision  int64                     `json:"revision"`
	CreatedAt time.Time                 `json:"createdAt"`
	ExpiresAt time.Time                 `json:"expiresAt"`
}

type sourceTaskApprovalEntry struct {
	value           SourceTaskApproval
	attemptID       string
	leaseID         string
	recordID        string
	recordExpiresAt time.Time
}

type SourceTaskApprovalLedger struct {
	mu      sync.Mutex
	entries map[string]*sourceTaskApprovalEntry
	now     func() time.Time
	closed  bool
}

func NewSourceTaskApprovalLedger() *SourceTaskApprovalLedger {
	return &SourceTaskApprovalLedger{entries: make(map[string]*sourceTaskApprovalEntry), now: time.Now}
}

func validateSourceTaskApprovalBinding(ctx context.Context, binding SourceTaskApprovalBinding) error {
	scope, owned := coordination.FromContext(ctx)
	target := binding.ExecutionTarget
	owner := scope.TargetDeviceID
	if scope.Coordinated {
		owner = scope.CoreID
	}
	if !owned || scope != binding.Scope || scope.CoreID == "" || scope.SpaceID != scope.CoreID || scope.AuthorizationRealm != scope.CoreID || scope.InitiatorDeviceID == "" || scope.TargetDeviceID == "" || scope.RoleID == "" || scope.RoleRevision < 1 || scope.ProviderEpoch < 1 || scope.TargetProviderEpoch < 1 || scope.PermissionRevision < 1 || scope.TargetPermissionRevision < 1 || scope.ModeRevision < 1 || scope.RequestID == "" || scope.ExecutionID == "" || scope.TurnID == "" || scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner || binding.TaskRunID == "" || len(binding.TaskRunID) > 256 || !validTaskFingerprint(binding.InputHash) || binding.Target.DeviceID != scope.TargetDeviceID || binding.Target.TaskID == "" || binding.Target.InstalledGeneration < 1 || !validTaskFingerprint(binding.Target.DefinitionFingerprint) || !validTaskFingerprint(binding.Target.PortableFingerprint) || target.SpaceID.String() != scope.CoreID || target.DeviceID.String() != scope.TargetDeviceID || target.RuntimeID == "" || target.RuntimeSessionID == "" || target.ConnectionGeneration < 1 || target.SourceTaskDefinitionID != "" && target.SourceTaskDefinitionID != binding.Target.TaskID {
		return NewTaskError(ErrTaskPermissionDenied, "设备任务单次审批缺少完整来源身份")
	}
	encoded, err := json.Marshal(binding)
	if binding.TaskGeneration < 1 {
		return NewTaskError(ErrTaskPermissionDenied, "设备任务审批缺少执行代次")
	}
	if err != nil || len(encoded) > 16<<10 {
		return NewTaskError(ErrTaskPermissionDenied, "设备任务单次审批身份超过上限")
	}
	return coordination.ValidateCurrent(ctx)
}

func (l *SourceTaskApprovalLedger) purgeLocked(now time.Time) {
	for id, entry := range l.entries {
		if !entry.value.ExpiresAt.After(now) {
			delete(l.entries, id)
		}
	}
}

func (l *SourceTaskApprovalLedger) Request(ctx context.Context, binding SourceTaskApprovalBinding) (SourceTaskApproval, error) {
	if err := validateSourceTaskApprovalBinding(ctx, binding); err != nil {
		return SourceTaskApproval{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return SourceTaskApproval{}, NewTaskError(ErrTaskDependencyUnavailable, "设备任务审批服务已经停止")
	}
	now := l.now().UTC()
	l.purgeLocked(now)
	for _, entry := range l.entries {
		if entry.value.Binding == binding {
			return entry.value, nil
		}
	}
	if len(l.entries) >= 64 {
		return SourceTaskApproval{}, coordination.ErrPendingLimit
	}
	value := SourceTaskApproval{ID: "task-approval-" + uuid.NewString(), Binding: binding, Status: "pending", Revision: 1, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	l.entries[value.ID] = &sourceTaskApprovalEntry{value: value}
	return value, nil
}

func (l *SourceTaskApprovalLedger) Decide(ctx context.Context, id string, revision int64, binding SourceTaskApprovalBinding, approved bool) (SourceTaskApproval, error) {
	if err := validateSourceTaskApprovalBinding(ctx, binding); err != nil {
		return SourceTaskApproval{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(l.now())
	entry := l.entries[id]
	if entry == nil || entry.value.Binding != binding || entry.value.Revision != revision || entry.value.Status != "pending" {
		return SourceTaskApproval{}, coordination.ErrRequestConflict
	}
	entry.value.Status = "denied"
	if approved {
		entry.value.Status = "approved"
	}
	entry.value.Revision++
	return entry.value, nil
}

func (l *SourceTaskApprovalLedger) Claim(ctx context.Context, id string, binding SourceTaskApprovalBinding, attemptID, leaseID string, deadline time.Time) error {
	if err := validateSourceTaskApprovalBinding(ctx, binding); err != nil {
		return err
	}
	if attemptID == "" || len(attemptID) > 256 || leaseID == "" || len(leaseID) > 256 {
		return NewTaskError(ErrTaskPermissionDenied, "单次审批缺少执行代次或租约")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	l.purgeLocked(now)
	entry := l.entries[id]
	if entry == nil || entry.value.Binding != binding {
		return NewTaskError(ErrTaskPermissionDenied, "单次审批不存在或来源已变化")
	}
	if entry.value.Status == "claimed" && entry.attemptID == attemptID && entry.leaseID == leaseID {
		return nil
	}
	if entry.value.Status != "approved" || entry.recordID == "" || !entry.recordExpiresAt.After(now) || !deadline.After(now) || deadline.After(now.Add(30*time.Minute)) {
		return NewTaskError(ErrTaskPermissionDenied, "单次审批已消费、未批准或执行期限无效")
	}
	entry.value.Status, entry.attemptID, entry.leaseID = "claimed", attemptID, leaseID
	if deadline.After(entry.recordExpiresAt) {
		deadline = entry.recordExpiresAt
	}
	entry.value.ExpiresAt, entry.value.Revision = deadline.UTC(), entry.value.Revision+1
	return nil
}

func (l *SourceTaskApprovalLedger) ValidateClaim(ctx context.Context, id string, binding SourceTaskApprovalBinding, attemptID, leaseID string) error {
	if err := validateSourceTaskApprovalBinding(ctx, binding); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(l.now())
	entry := l.entries[id]
	if entry == nil || entry.recordID == "" || entry.value.Status != "claimed" || entry.value.Binding != binding || entry.attemptID != attemptID || entry.leaseID != leaseID {
		return NewTaskError(ErrTaskPermissionDenied, "单次审批执行身份已失效")
	}
	return nil
}

func (l *SourceTaskApprovalLedger) Revoke(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if entry := l.entries[id]; entry != nil {
		entry.value.Status, entry.value.Revision = "revoked", entry.value.Revision+1
	}
}

func (l *SourceTaskApprovalLedger) RevokeCurrent(ctx context.Context, id string, revision int64, binding SourceTaskApprovalBinding) (SourceTaskApproval, error) {
	if err := validateSourceTaskApprovalBinding(ctx, binding); err != nil {
		return SourceTaskApproval{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(l.now())
	entry := l.entries[id]
	if entry == nil || entry.value.Revision != revision || entry.value.Binding != binding || entry.value.Status != "approved" && entry.value.Status != "claimed" {
		return SourceTaskApproval{}, coordination.ErrRequestConflict
	}
	entry.value.Status, entry.value.Revision = "revoked", entry.value.Revision+1
	return entry.value, nil
}

func (l *SourceTaskApprovalLedger) Get(id string) (SourceTaskApproval, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(l.now())
	entry := l.entries[id]
	if entry == nil {
		return SourceTaskApproval{}, false
	}
	return entry.value, true
}

func (l *SourceTaskApprovalLedger) List() []SourceTaskApproval {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.purgeLocked(l.now())
	items := make([]SourceTaskApproval, 0, len(l.entries))
	for _, entry := range l.entries {
		items = append(items, entry.value)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}

func (l *SourceTaskApprovalLedger) SetApprovalRecord(ctx context.Context, id string, revision int64, binding SourceTaskApprovalBinding, recordID string, expiresAt time.Time) error {
	if err := validateSourceTaskApprovalBinding(ctx, binding); err != nil {
		return err
	}
	if recordID == "" || len(recordID) > 256 {
		return NewTaskError(ErrTaskPermissionDenied, "单次审批记录无效")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	l.purgeLocked(now)
	if !expiresAt.After(now) || expiresAt.After(now.Add(30*time.Minute)) {
		return NewTaskError(ErrTaskPermissionDenied, "单次权限记录的执行期限无效")
	}
	entry := l.entries[id]
	if entry == nil || entry.value.Status != "approved" || entry.value.Revision != revision || entry.value.Binding != binding || entry.recordID != "" && entry.recordID != recordID {
		return coordination.ErrRequestConflict
	}
	entry.recordID = recordID
	entry.recordExpiresAt = expiresAt.UTC()
	return nil
}

func (l *SourceTaskApprovalLedger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed, l.entries = true, nil
}

type sourceTaskApprovalProofKey struct{}

type sourceTaskApprovalProof struct {
	ledger    *SourceTaskApprovalLedger
	id        string
	binding   SourceTaskApprovalBinding
	attemptID string
	leaseID   string
}

func (l *SourceTaskApprovalLedger) permissionRecord(proof sourceTaskApprovalProof) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	l.purgeLocked(now)
	entry := l.entries[proof.id]
	valid := entry != nil && entry.value.Binding == proof.binding && entry.recordID != "" && entry.recordExpiresAt.After(now)
	if valid && proof.attemptID == "" && proof.leaseID == "" {
		valid = entry.value.Status == "approved"
	} else if valid {
		valid = entry.value.Status == "claimed" && entry.attemptID == proof.attemptID && entry.leaseID == proof.leaseID
	}
	if !valid {
		return "", NewTaskError(ErrTaskPermissionDenied, "单次设备资源审批已失效或被其他执行消费")
	}
	return entry.recordID, nil
}

func sourceTaskPermissionRecord(ctx context.Context, run *TaskRun, definition *TaskDefinition) (string, error) {
	proof, ok := ctx.Value(sourceTaskApprovalProofKey{}).(sourceTaskApprovalProof)
	if !ok {
		return "", nil
	}
	scope, owned := coordination.FromContext(ctx)
	fingerprint, err := taskDefinitionFingerprint(definition)
	if err != nil || !owned || proof.ledger == nil || run == nil || proof.binding.Scope != scope || proof.binding.TaskRunID != run.TaskRunID || proof.binding.TaskGeneration != sourceTaskApprovalGeneration(run.Generation) || proof.binding.InputHash != run.InputHash || proof.binding.ExecutionTarget != run.ExecutionTarget || proof.binding.Target.TaskID != definition.TaskID || proof.binding.Target.InstalledGeneration != definition.InstalledGeneration || proof.binding.Target.DefinitionFingerprint != fingerprint || run.InvocationID != run.TaskRunID || run.ScopeSnapshotID != run.TaskRunID || proof.attemptID != "" && proof.attemptID != run.ExecutionAttemptID.String() {
		return "", NewTaskError(ErrTaskPermissionDenied, "单次设备资源审批与当前执行不一致")
	}
	return proof.ledger.permissionRecord(proof)
}

func sourceTaskApprovalGeneration(generation int64) int64 {
	if generation == 0 {
		return 1
	}
	return generation
}
