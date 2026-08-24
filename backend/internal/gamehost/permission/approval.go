package permission

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
)

type ApprovalStatus string

const (
	ApprovalStatusPending   ApprovalStatus = "pending"
	ApprovalStatusApproved  ApprovalStatus = "approved"
	ApprovalStatusRejected  ApprovalStatus = "rejected"
	ApprovalStatusCancelled ApprovalStatus = "cancelled"
	ApprovalStatusExpired   ApprovalStatus = "expired"
	ApprovalStatusConsumed  ApprovalStatus = "consumed"
)

// PendingApproval is the user-facing representation of a GameHost permission
// request. It deliberately contains only host/runtime identity and the Kernel
// permission ID; concrete game semantics remain opaque to GameHost.
type PendingApproval struct {
	ID           string                            `json:"id"`
	RuntimeID    string                            `json:"runtimeId"`
	PluginID     string                            `json:"pluginId"`
	ServiceID    string                            `json:"serviceId,omitempty"`
	ExtensionID  string                            `json:"extensionId"`
	PermissionID string                            `json:"permissionId"`
	Target       kernelpermission.PermissionTarget `json:"target,omitempty"`
	Status       ApprovalStatus                    `json:"status"`
	RequestedAt  time.Time                         `json:"requestedAt"`
	ExpiresAt    time.Time                         `json:"expiresAt"`
	ResolvedAt   *time.Time                        `json:"resolvedAt,omitempty"`
	ResolvedBy   string                            `json:"resolvedBy,omitempty"`
	Reason       string                            `json:"reason,omitempty"`
}

type approvalEntry struct {
	view     PendingApproval
	decision chan ApprovalStatus
}

type keyedApprovalLock struct {
	mu   sync.Mutex
	refs int
}

// perUsePermissionResolver is implemented by the canonical Kernel broker.
// Keeping it optional avoids widening PermissionBroker while still ensuring
// ordinary persistent permissions are not accidentally converted into
// one-operation approvals by GameHost.
type perUsePermissionResolver interface {
	RequiresPerUse(permissionID string) bool
}

// ApprovalCoordinator turns Kernel require_approval decisions into a real
// suspend -> user decision -> allow_once -> resume flow. A key lock serializes
// requests for the same runtime/service/permission so an allow_once grant
// cannot be consumed by a competing GameHost request before the approved
// request resumes.
type ApprovalCoordinator struct {
	broker kernelpermission.PermissionBroker

	mu      sync.Mutex
	entries map[string]*approvalEntry
	locks   map[string]*keyedApprovalLock
	ttl     time.Duration
	clock   func() time.Time
}

func NewApprovalCoordinator(broker kernelpermission.PermissionBroker) (*ApprovalCoordinator, error) {
	if broker == nil {
		return nil, fmt.Errorf("gamehost approval: kernel permission broker is required")
	}
	return &ApprovalCoordinator{
		broker:  broker,
		entries: make(map[string]*approvalEntry),
		locks:   make(map[string]*keyedApprovalLock),
		ttl:     2 * time.Minute,
		clock:   time.Now,
	}, nil
}

func (c *ApprovalCoordinator) SetTTL(ttl time.Duration) {
	if c == nil || ttl <= 0 {
		return
	}
	c.mu.Lock()
	c.ttl = ttl
	c.mu.Unlock()
}

// Evaluate performs the Kernel permission evaluation. If approval is required,
// it waits for an explicit user decision. Approval creates a one-time grant and
// consumes that grant in the same serialized critical section before returning
// to the protected GameHost operation.
func (c *ApprovalCoordinator) Evaluate(
	ctx context.Context,
	subject EffectiveSubject,
	permissionID string,
	request kernelpermission.PermissionEvaluationRequest,
) kernelpermission.PermissionEvaluationResult {
	if c == nil || c.broker == nil {
		return kernelpermission.PermissionEvaluationResult{
			Decision: kernelpermission.DecisionDeny,
			Reasons: []kernelpermission.PermissionReason{{
				Code:       "approval_coordinator_unavailable",
				Permission: permissionID,
			}},
		}
	}

	lockKey := approvalLockKey(subject, permissionID)
	unlock := c.lock(lockKey)
	defer unlock()

	result := c.broker.Evaluate(ctx, request)
	if result.Decision == kernelpermission.DecisionAllowOnce {
		return c.consumeAllowOnce(ctx, permissionID, result)
	}
	if result.Decision != kernelpermission.DecisionRequireApproval {
		return result
	}
	resolver, ok := c.broker.(perUsePermissionResolver)
	if !ok || !resolver.RequiresPerUse(permissionID) {
		// Missing persistent/session permissions belong to the normal extension
		// permission-management flow. GameHost only suspends live operations for
		// permissions whose canonical definition explicitly requires per-use approval.
		return result
	}

	entry := c.createPending(subject, permissionID, request)
	status := c.wait(ctx, entry)
	if status != ApprovalStatusApproved {
		code := "approval_rejected"
		switch status {
		case ApprovalStatusExpired:
			code = "approval_expired"
		case ApprovalStatusCancelled:
			code = "approval_cancelled"
		}
		return kernelpermission.PermissionEvaluationResult{
			Decision: kernelpermission.DecisionDeny,
			Reasons: []kernelpermission.PermissionReason{{
				Code:       code,
				Permission: permissionID,
			}},
		}
	}

	if err := ctx.Err(); err != nil {
		c.finish(entry.view.ID, ApprovalStatusCancelled, "system", err.Error())
		return kernelpermission.PermissionEvaluationResult{
			Decision: kernelpermission.DecisionDeny,
			Reasons: []kernelpermission.PermissionReason{{
				Code:       "approval_cancelled",
				Permission: permissionID,
				Detail:     err.Error(),
			}},
		}
	}

	expires := c.clock().UTC().Add(30 * time.Second)
	if entry.view.ExpiresAt.Before(expires) {
		expires = entry.view.ExpiresAt
	}
	approvalInput, err := json.Marshal(map[string]string{"gameHostApprovalId": entry.view.ID})
	if err != nil {
		c.finish(entry.view.ID, ApprovalStatusRejected, "system", "failed to bind allow_once grant")
		return kernelpermission.PermissionEvaluationResult{
			Decision: kernelpermission.DecisionDeny,
			Reasons: []kernelpermission.PermissionReason{{
				Code:       "allow_once_binding_failed",
				Permission: permissionID,
				Detail:     err.Error(),
			}},
		}
	}
	boundRequest := request
	boundRequest.InvocationID = entry.view.ID
	boundRequest.Input = approvalInput
	grant, err := c.broker.Grant(ctx, kernelpermission.PermissionGrantRequest{
		Subject:      request.Subject,
		PermissionID: permissionID,
		Scope:        approvalScope(boundRequest),
		Decision:     kernelpermission.DecisionAllowOnce,
		InputHash:    approvalInputHash(approvalInput),
		ExpiresAt:    &expires,
		IssuedBy:     kernelpermission.IssuerUser,
		Reason:       "gamehost per-use approval " + entry.view.ID,
	})
	if err != nil {
		c.finish(entry.view.ID, ApprovalStatusRejected, "system", "failed to create allow_once grant")
		return kernelpermission.PermissionEvaluationResult{
			Decision: kernelpermission.DecisionDeny,
			Reasons: []kernelpermission.PermissionReason{{
				Code:       "allow_once_grant_failed",
				Permission: permissionID,
				Detail:     err.Error(),
			}},
		}
	}
	cleanupGrant := func() {
		_ = c.broker.Revoke(context.Background(), grant.GrantID)
	}
	if err := ctx.Err(); err != nil {
		cleanupGrant()
		c.finish(entry.view.ID, ApprovalStatusCancelled, "system", err.Error())
		return kernelpermission.PermissionEvaluationResult{
			Decision: kernelpermission.DecisionDeny,
			Reasons: []kernelpermission.PermissionReason{{
				Code:       "approval_cancelled",
				Permission: permissionID,
				Detail:     err.Error(),
			}},
		}
	}

	// Re-evaluate inside the per-key lock and consume any one-time grant before
	// the approved operation resumes. Evaluation itself is intentionally side-effect
	// free so UI/diagnostic permission checks cannot accidentally consume approval.
	result = c.broker.Evaluate(ctx, boundRequest)
	if result.Decision == kernelpermission.DecisionAllowOnce {
		result = c.consumeAllowOnce(ctx, permissionID, result)
	}
	if result.Decision == kernelpermission.DecisionAllow {
		c.finish(entry.view.ID, ApprovalStatusConsumed, entry.view.ResolvedBy, entry.view.Reason)
		return result
	}

	cleanupGrant()
	c.finish(entry.view.ID, ApprovalStatusRejected, "system", "allow_once grant did not authorize the request")
	return result
}

func (c *ApprovalCoordinator) consumeAllowOnce(ctx context.Context, permissionID string, result kernelpermission.PermissionEvaluationResult) kernelpermission.PermissionEvaluationResult {
	consumed := false
	for _, grant := range result.MatchedGrants {
		if !grant.IsOneTime() {
			continue
		}
		if err := c.broker.Revoke(ctx, grant.GrantID); err != nil {
			return kernelpermission.PermissionEvaluationResult{
				Decision: kernelpermission.DecisionDeny,
				Reasons: []kernelpermission.PermissionReason{{
					Code:       "allow_once_consume_failed",
					Permission: permissionID,
					Detail:     err.Error(),
				}},
			}
		}
		consumed = true
	}
	if !consumed {
		return kernelpermission.PermissionEvaluationResult{
			Decision: kernelpermission.DecisionDeny,
			Reasons: []kernelpermission.PermissionReason{{
				Code:       "allow_once_grant_missing",
				Permission: permissionID,
			}},
		}
	}
	result.Decision = kernelpermission.DecisionAllow
	return result
}

func approvalInputHash(input []byte) string {
	h := sha256.Sum256(input)
	return hex.EncodeToString(h[:])
}

func approvalScope(request kernelpermission.PermissionEvaluationRequest) kernelpermission.PermissionScope {
	if len(request.Requirements) > 0 && request.Requirements[0].Scope.IsValid() {
		return request.Requirements[0].Scope
	}
	switch request.Subject.Type {
	case kernelpermission.SubjectExtension, kernelpermission.SubjectTool:
		if strings.TrimSpace(request.Subject.ExtensionID) != "" {
			return kernelpermission.ScopeForExtension(request.Subject.ExtensionID)
		}
	}
	return kernelpermission.ScopeGlobalOnly()
}

func (c *ApprovalCoordinator) createPending(subject EffectiveSubject, permissionID string, request kernelpermission.PermissionEvaluationRequest) *approvalEntry {
	now := c.clock().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := &approvalEntry{
		view: PendingApproval{
			ID:           newApprovalID(),
			RuntimeID:    subject.RuntimeID,
			PluginID:     subject.PluginID,
			ServiceID:    subject.ServiceID,
			ExtensionID:  subject.ExtensionID,
			PermissionID: permissionID,
			Target:       request.Target,
			Status:       ApprovalStatusPending,
			RequestedAt:  now,
			ExpiresAt:    now.Add(c.ttl),
		},
		decision: make(chan ApprovalStatus, 1),
	}
	c.entries[entry.view.ID] = entry
	return entry
}

func (c *ApprovalCoordinator) wait(ctx context.Context, entry *approvalEntry) ApprovalStatus {
	if entry == nil {
		return ApprovalStatusRejected
	}
	timer := time.NewTimer(time.Until(entry.view.ExpiresAt))
	if time.Until(entry.view.ExpiresAt) <= 0 {
		if !timer.Stop() {
			<-timer.C
		}
		c.finish(entry.view.ID, ApprovalStatusExpired, "system", "approval request expired")
		return ApprovalStatusExpired
	}
	defer timer.Stop()

	select {
	case status := <-entry.decision:
		return status
	case <-ctx.Done():
		c.finish(entry.view.ID, ApprovalStatusCancelled, "system", ctx.Err().Error())
		return ApprovalStatusCancelled
	case <-timer.C:
		c.finish(entry.view.ID, ApprovalStatusExpired, "system", "approval request expired")
		return ApprovalStatusExpired
	}
}

func (c *ApprovalCoordinator) ListPending() []PendingApproval {
	if c == nil {
		return nil
	}
	now := c.clock().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]PendingApproval, 0)
	for _, entry := range c.entries {
		if entry.view.Status == ApprovalStatusPending && !entry.view.ExpiresAt.After(now) {
			status := ApprovalStatusExpired
			entry.view.Status = status
			resolved := now
			entry.view.ResolvedAt = &resolved
			entry.view.ResolvedBy = "system"
			entry.view.Reason = "approval request expired"
			select {
			case entry.decision <- status:
			default:
			}
		}
		if entry.view.Status == ApprovalStatusPending {
			out = append(out, entry.view)
		}
	}
	return out
}

func (c *ApprovalCoordinator) Approve(id, actor, reason string) error {
	return c.resolve(id, ApprovalStatusApproved, actor, reason)
}

func (c *ApprovalCoordinator) Reject(id, actor, reason string) error {
	return c.resolve(id, ApprovalStatusRejected, actor, reason)
}

func (c *ApprovalCoordinator) resolve(id string, status ApprovalStatus, actor, reason string) error {
	if c == nil {
		return fmt.Errorf("gamehost approval: coordinator unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("gamehost approval: approval id is required")
	}
	if status != ApprovalStatusApproved && status != ApprovalStatusRejected {
		return fmt.Errorf("gamehost approval: invalid resolution status %q", status)
	}
	c.mu.Lock()
	entry, ok := c.entries[id]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("gamehost approval: approval %s not found", id)
	}
	if entry.view.Status != ApprovalStatusPending {
		current := entry.view.Status
		c.mu.Unlock()
		return fmt.Errorf("gamehost approval: approval %s is already %s", id, current)
	}
	now := c.clock().UTC()
	if !entry.view.ExpiresAt.After(now) {
		entry.view.Status = ApprovalStatusExpired
		entry.view.ResolvedAt = &now
		entry.view.ResolvedBy = "system"
		entry.view.Reason = "approval request expired"
		select {
		case entry.decision <- ApprovalStatusExpired:
		default:
		}
		c.mu.Unlock()
		return fmt.Errorf("gamehost approval: approval %s expired", id)
	}
	if strings.TrimSpace(actor) == "" {
		actor = "user"
	}
	entry.view.Status = status
	entry.view.ResolvedAt = &now
	entry.view.ResolvedBy = actor
	entry.view.Reason = strings.TrimSpace(reason)
	select {
	case entry.decision <- status:
	default:
	}
	c.mu.Unlock()
	return nil
}

func (c *ApprovalCoordinator) finish(id string, status ApprovalStatus, actor, reason string) {
	if c == nil || strings.TrimSpace(id) == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok {
		return
	}
	// Preserve the human resolution metadata when transitioning approved -> consumed.
	if entry.view.Status != ApprovalStatusApproved || status != ApprovalStatusConsumed {
		entry.view.ResolvedBy = actor
		entry.view.Reason = reason
	}
	entry.view.Status = status
	now := c.clock().UTC()
	entry.view.ResolvedAt = &now
}

func (c *ApprovalCoordinator) lock(key string) func() {
	c.mu.Lock()
	lk := c.locks[key]
	if lk == nil {
		lk = &keyedApprovalLock{}
		c.locks[key] = lk
	}
	lk.refs++
	c.mu.Unlock()

	lk.mu.Lock()
	return func() {
		lk.mu.Unlock()
		c.mu.Lock()
		lk.refs--
		if lk.refs == 0 {
			delete(c.locks, key)
		}
		c.mu.Unlock()
	}
}

func approvalLockKey(subject EffectiveSubject, permissionID string) string {
	return strings.Join([]string{subject.RuntimeID, subject.PluginID, subject.ServiceID, subject.ExtensionID, permissionID}, "\x00")
}

func newApprovalID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "ghap_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("ghap_%d", time.Now().UnixNano())
}
