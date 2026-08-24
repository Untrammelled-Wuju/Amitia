package permission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type PermissionBroker interface {
	Evaluate(ctx context.Context, request PermissionEvaluationRequest) PermissionEvaluationResult
	Grant(ctx context.Context, request PermissionGrantRequest) (PermissionGrant, error)
	Revoke(ctx context.Context, grantID string) error
	RevokeBySubject(ctx context.Context, subject PermissionSubject) (int, error)
	RevokeByExtension(ctx context.Context, extensionID string) (int, error)
	ListGrants(ctx context.Context, filter PermissionGrantFilter) ([]PermissionGrant, error)
	Explain(ctx context.Context, request PermissionEvaluationRequest) PermissionExplanation
	DetectUpgrade(ctx context.Context, oldPermissions, newPermissions []PermissionRequirement) []PermissionUpgrade
	RecordApproval(ctx context.Context, request PermissionApprovalRecordRequest) (PermissionApprovalRecord, error)
	ValidateSnapshot(ctx context.Context, snapshotID string, inv PermissionEvaluationRequest) error
}

type TrustLevelChecker interface {
	IsTrusted(subject PermissionSubject) bool
}

type DefaultPermissionBroker struct {
	registry        *PermissionDefinitionRegistry
	storage         PermissionStorage
	cache           *PermissionCache
	auditRec        *PermissionAuditRecorder
	mu              sync.RWMutex
	trustChecker    TrustLevelChecker
	approvalRecords map[string]PermissionApprovalRecord
	snapshotStore   PermissionSnapshotStore

	SystemPolicy    func(ctx context.Context, subject PermissionSubject, permissionID string, scope PermissionScope) (PermissionDecision, bool)
	ExecutionPolicy func(ctx context.Context, request PermissionEvaluationRequest, requirement PermissionRequirement, definition PermissionDefinition) (PermissionDecision, bool)

	OnPermissionRevoked func(extensionID, runtimeID string)
}

func NewDefaultPermissionBroker(registry *PermissionDefinitionRegistry, storage PermissionStorage) *DefaultPermissionBroker {
	return &DefaultPermissionBroker{
		registry: registry,
		storage:  storage,
		cache:    NewPermissionCache(5 * time.Minute),
		auditRec: NewPermissionAuditRecorder(),
	}
}

func (b *DefaultPermissionBroker) Close() error {
	if b == nil || b.cache == nil {
		return nil
	}
	b.cache.Close()
	return nil
}

func (b *DefaultPermissionBroker) SetSnapshotStore(store PermissionSnapshotStore) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.snapshotStore = store
}

func (b *DefaultPermissionBroker) SetTrustLevelChecker(checker TrustLevelChecker) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.trustChecker = checker
	b.cache.InvalidateAll()
}

func (b *DefaultPermissionBroker) GetTrustLevelChecker() TrustLevelChecker {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.trustChecker
}

// RequiresPerUse reports whether the canonical permission definition requires
// a fresh, one-operation approval. It is intentionally read-only so adapters
// can distinguish per-use approval from ordinary persistent permission setup.
func (b *DefaultPermissionBroker) RequiresPerUse(permissionID string) bool {
	if b == nil || b.registry == nil {
		return false
	}
	def, ok := b.registry.Get(permissionID)
	return ok && def.RequiresPerUse
}

func (b *DefaultPermissionBroker) Evaluate(ctx context.Context, request PermissionEvaluationRequest) PermissionEvaluationResult {
	request.ExecutionContext = request.ExecutionContext.Normalize()
	if !request.ExecutionContext.IsEmpty() {
		if err := request.ExecutionContext.Validate(); err != nil {
			b.auditRec.RecordEvaluation(ctx, request, PermissionEvaluationResult{
				Decision: DecisionDeny,
				Reasons:  []PermissionReason{{Code: "execution_context_invalid", Detail: err.Error()}},
			})
			return PermissionEvaluationResult{
				Decision: DecisionDeny,
				Reasons:  []PermissionReason{{Code: "execution_context_invalid", Detail: err.Error()}},
			}
		}
	}

	if len(request.Requirements) == 0 {
		return PermissionEvaluationResult{
			Decision: DecisionAllow,
			Reasons:  []PermissionReason{{Code: "no_requirements"}},
		}
	}

	result := PermissionEvaluationResult{
		Decision:      DecisionAllow,
		Reasons:       make([]PermissionReason, 0),
		MatchedGrants: make([]PermissionGrant, 0),
		Missing:       make([]PermissionRequirement, 0),
	}

	hasHardDeny := false
	hasForcedApprovalMissing := false
	hasNormalMissing := false

	for _, req := range request.Requirements {
		def, ok := b.registry.Get(req.PermissionID)
		if !ok {
			result.Missing = append(result.Missing, req)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "unknown_permission",
				Permission: req.PermissionID,
			})
			hasHardDeny = true
			continue
		}

		if def.TrustedOnly && b.isNotTrusted(request.Subject) {
			result.Missing = append(result.Missing, req)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "trusted_only",
				Permission: req.PermissionID,
			})
			hasHardDeny = true
			continue
		}

		if request.IsBackground && !def.BackgroundAllowed {
			result.Missing = append(result.Missing, req)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "background_not_allowed",
				Permission: req.PermissionID,
			})
			hasHardDeny = true
			continue
		}

		effectiveScope := b.resolveEffectiveScope(req, request)
		if !b.isScopeAllowed(def, effectiveScope) {
			result.Missing = append(result.Missing, req)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "scope_not_allowed",
				Permission: req.PermissionID,
			})
			hasHardDeny = true
			continue
		}

		if b.SystemPolicy != nil {
			if decision, handled := b.SystemPolicy(ctx, request.Subject, req.PermissionID, effectiveScope); handled {
				if decision == DecisionDeny {
					result.Missing = append(result.Missing, req)
					result.Reasons = append(result.Reasons, PermissionReason{
						Code:       "system_policy_deny",
						Permission: req.PermissionID,
					})
					hasHardDeny = true
				}
				continue
			}
		}

		if b.ExecutionPolicy != nil {
			if decision, handled := b.ExecutionPolicy(ctx, request, req, def); handled {
				if decision == DecisionDeny {
					result.Missing = append(result.Missing, req)
					result.Reasons = append(result.Reasons, PermissionReason{
						Code:       "execution_policy_deny",
						Permission: req.PermissionID,
					})
					hasHardDeny = true
				}
				continue
			}
		}

		remoteDecision := b.applyRemoteExecutionPolicy(def, request.ExecutionContext)
		if remoteDecision == DecisionDeny {
			result.Missing = append(result.Missing, req)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "remote_execution_denied",
				Permission: req.PermissionID,
			})
			hasHardDeny = true
			continue
		}

		if remoteDecision == DecisionRequireApproval {
			if !b.validateApprovalRecord(req.PermissionID, request) {
				result.Missing = append(result.Missing, req)
				result.Reasons = append(result.Reasons, PermissionReason{
					Code:       "remote_approval_required",
					Permission: req.PermissionID,
				})
				hasForcedApprovalMissing = true
				continue
			}
		}

		grants := b.cache.GetOrLoad(ctx, request.Subject, req.PermissionID, func() []PermissionGrant {
			filter := PermissionGrantFilter{
				Subject:      &request.Subject,
				PermissionID: req.PermissionID,
				ActiveOnly:   true,
			}
			loaded, _ := b.storage.List(ctx, filter)
			result := make([]PermissionGrant, 0, len(loaded))
			for _, sg := range loaded {
				result = append(result, b.storedToGrant(sg))
			}
			return result
		})

		matched := b.matchGrants(grants, effectiveScope, req, request.ExecutionContext, request.Input)
		if len(matched) > 0 {
			result.MatchedGrants = append(result.MatchedGrants, matched...)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "grant_matched",
				Permission: req.PermissionID,
			})
		} else if b.requiresBoundGrant(def, request.ExecutionContext) {
			result.Missing = append(result.Missing, req)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "remote_bound_grant_required",
				Permission: req.PermissionID,
			})
			hasForcedApprovalMissing = true
		} else {
			result.Missing = append(result.Missing, req)
			result.Reasons = append(result.Reasons, PermissionReason{
				Code:       "missing_grant",
				Permission: req.PermissionID,
			})
			hasNormalMissing = true
		}
	}

	if hasHardDeny {
		b.determineDenyOrApproval(&result, request)
	} else if hasForcedApprovalMissing {
		b.determineForcedApproval(&result, request)
	} else if hasNormalMissing {
		b.determineDenyOrApproval(&result, request)
	} else {
		result.Decision = decisionForMatchedGrants(result.MatchedGrants)
	}

	b.auditRec.RecordEvaluation(ctx, request, result)

	return result
}

func (b *DefaultPermissionBroker) applyRemoteExecutionPolicy(def PermissionDefinition, execCtx PermissionExecutionContext) PermissionDecision {
	if !execCtx.IsDeviceExecution() {
		return ""
	}
	switch def.RemoteExecution {
	case RemoteExecutionDeny:
		return DecisionDeny
	case RemoteExecutionRequireApproval:
		return DecisionRequireApproval
	}
	return ""
}

func (b *DefaultPermissionBroker) requiresBoundGrant(def PermissionDefinition, execCtx PermissionExecutionContext) bool {
	if !execCtx.IsDeviceExecution() {
		return false
	}
	switch def.RiskLevel {
	case "high", "critical":
		return true
	}
	return false
}

func (b *DefaultPermissionBroker) validateApprovalRecord(permissionID string, request PermissionEvaluationRequest) bool {
	if request.ApprovalRecordID == "" {
		return false
	}

	record, ok := b.getApprovalRecord(request.ApprovalRecordID)
	if !ok {
		return false
	}

	if record.Decision != ApprovalDecisionApproved {
		return false
	}

	if record.InvocationID != request.InvocationID {
		return false
	}

	permFound := false
	for _, pid := range record.PermissionIDs {
		if pid == permissionID {
			permFound = true
			break
		}
	}
	if !permFound {
		return false
	}

	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return false
	}

	if request.RiskLevel != "" && record.RiskLevel != "" {
		if riskRank(request.RiskLevel) > riskRank(record.RiskLevel) {
			return false
		}
	}

	if record.ScopeSnapshotID == "" || request.ScopeSnapshotID == "" {
		return false
	}
	if record.ScopeSnapshotID != request.ScopeSnapshotID {
		return false
	}

	currentKey := request.ExecutionContext.BindingKey()
	if record.ExecutionBindingKey == "" || currentKey == "" {
		return false
	}
	if currentKey != record.ExecutionBindingKey {
		return false
	}

	if !record.ExecutionContext.IsDeviceExecution() {
		return false
	}

	return true
}

func riskRank(risk string) int {
	switch risk {
	case "":
		return 0
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return 0
	}
}

func (b *DefaultPermissionBroker) Grant(ctx context.Context, request PermissionGrantRequest) (PermissionGrant, error) {
	def, ok := b.registry.Get(request.PermissionID)
	if !ok {
		return PermissionGrant{}, fmt.Errorf("unknown permission: %s", request.PermissionID)
	}

	if !b.isScopeAllowed(def, request.Scope) {
		return PermissionGrant{}, fmt.Errorf("scope %s not allowed for permission %s", request.Scope.Type, request.PermissionID)
	}

	if request.Decision == DecisionAllowPersistent && !def.PersistentGrantable {
		return PermissionGrant{}, fmt.Errorf("persistent grant not allowed for permission %s", request.PermissionID)
	}
	if def.RequiresPerUse && request.Decision != DecisionAllowOnce {
		return PermissionGrant{}, fmt.Errorf("permission %s requires an allow_once grant", request.PermissionID)
	}

	grantID := b.generateGrantID(request)
	now := time.Now()

	var inputBinding *InputBinding
	if request.InputHash != "" {
		inputBinding = &InputBinding{InputHash: request.InputHash}
	}

	grant := PermissionGrant{
		GrantID:       grantID,
		Subject:       request.Subject,
		PermissionID:  request.PermissionID,
		Scope:         request.Scope,
		Decision:      request.Decision,
		InputBinding:  inputBinding,
		TargetBinding: request.TargetBinding,
		IssuedAt:      now,
		ExpiresAt:     request.ExpiresAt,
		IssuedBy:      request.IssuedBy,
		Reason:        request.Reason,
		ManifestVer:   request.ManifestVer,
	}

	stored := b.grantToStored(grant)
	if err := b.storage.Save(ctx, stored); err != nil {
		return PermissionGrant{}, err
	}

	b.cache.Invalidate(request.Subject, request.PermissionID)
	b.auditRec.RecordGrant(ctx, grant)

	return grant, nil
}

func (b *DefaultPermissionBroker) Revoke(ctx context.Context, grantID string) error {
	stored, found, err := b.storage.GetByGrantID(ctx, grantID)
	if err != nil {
		return fmt.Errorf("revoke: lookup grant: %w", err)
	}
	if !found {
		return fmt.Errorf("revoke: grant %s not found", grantID)
	}

	if err := b.storage.MarkRevoked(ctx, grantID); err != nil {
		return err
	}

	b.cache.InvalidateAll()
	b.auditRec.RecordRevoke(ctx, grantID)

	if b.OnPermissionRevoked != nil {
		extID := stored.SubjectID
		runtimeID := ""
		if stored.SubjectType == string(SubjectRuntime) {
			runtimeID = stored.SubjectID
			extID = ""
		}
		b.OnPermissionRevoked(extID, runtimeID)
	}

	return nil
}

func (b *DefaultPermissionBroker) RevokeBySubject(ctx context.Context, subject PermissionSubject) (int, error) {
	grants, err := b.storage.ListBySubject(ctx, subject)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, g := range grants {
		if g.RevokedAt == nil && (g.ExpiresAt == nil || time.Now().Before(*g.ExpiresAt)) {
			if err := b.storage.MarkRevoked(ctx, g.GrantID); err != nil {
				return count, err
			}
			count++
		}
	}

	b.cache.InvalidateAll()

	if count > 0 && b.OnPermissionRevoked != nil {
		extID := subject.ExtensionID
		if extID == "" {
			extID = subject.ID
		}
		runtimeID := ""
		if subject.Type == SubjectRuntime {
			runtimeID = subject.ID
		}
		b.OnPermissionRevoked(extID, runtimeID)
	}

	return count, nil
}

func (b *DefaultPermissionBroker) RevokeByExtension(ctx context.Context, extensionID string) (int, error) {
	return b.RevokeBySubject(ctx, SubjectForExtension(extensionID))
}

func (b *DefaultPermissionBroker) ListGrants(ctx context.Context, filter PermissionGrantFilter) ([]PermissionGrant, error) {
	stored, err := b.storage.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	grants := make([]PermissionGrant, 0, len(stored))
	for _, sg := range stored {
		grants = append(grants, b.storedToGrant(sg))
	}
	return grants, nil
}

func (b *DefaultPermissionBroker) Explain(ctx context.Context, request PermissionEvaluationRequest) PermissionExplanation {
	result := b.Evaluate(ctx, request)

	explanation := PermissionExplanation{
		Decision:      result.Decision,
		Reasons:       result.Reasons,
		MatchedGrants: result.MatchedGrants,
	}

	switch result.Decision {
	case DecisionDeny:
		explanation.RequiredAction = "revoked_or_blocked"
	case DecisionRequireApproval:
		explanation.RequiredAction = "manual_approval"
	case DecisionAllow:
		explanation.RequiredAction = "none"
	}

	for _, req := range request.Requirements {
		if def, ok := b.registry.Get(req.PermissionID); ok {
			explanation.AvailableScopes = append(explanation.AvailableScopes, def.AllowedScopes...)
		}
	}

	return explanation
}

func (b *DefaultPermissionBroker) DetectUpgrade(ctx context.Context, oldPermissions, newPermissions []PermissionRequirement) []PermissionUpgrade {
	detector := NewUpgradeDetector(b.registry)
	return detector.Detect(ctx, oldPermissions, newPermissions)
}

func (b *DefaultPermissionBroker) ValidateSnapshot(ctx context.Context, snapshotID string, inv PermissionEvaluationRequest) error {
	if snapshotID == "" {
		return ErrPermissionSnapshotNotFound
	}

	if b.snapshotStore == nil {
		return ErrPermissionSnapshotNotFound
	}

	snap, err := b.snapshotStore.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return ErrPermissionSnapshotNotFound
	}

	if snap.RevokedAt != nil {
		return ErrPermissionSnapshotRevoked
	}

	if snap.ExpiresAt != nil && time.Now().After(*snap.ExpiresAt) {
		return ErrPermissionSnapshotExpired
	}

	if err := snap.VerifyIdentity(inv.Subject.ExtensionID, inv.Subject.ModuleID, inv.Generation); err != nil {
		return err
	}

	if b.registry != nil {
		for _, permID := range snap.GrantedPerms {
			if _, ok := b.registry.Get(permID); !ok {
				return ErrPermissionUnknown
			}
		}
	}

	if !inv.ExecutionContext.IsEmpty() {
		if err := snap.VerifyExecutionContext(inv.ExecutionContext); err != nil {
			return err
		}
	}

	return nil
}

func (b *DefaultPermissionBroker) resolveEffectiveScope(req PermissionRequirement, evalReq PermissionEvaluationRequest) PermissionScope {
	if req.Scope.IsValid() {
		return req.Scope
	}

	switch evalReq.Subject.Type {
	case SubjectExtension:
		return ScopeForExtension(evalReq.Subject.ExtensionID)
	case SubjectTool:
		return ScopeForExtension(evalReq.Subject.ExtensionID)
	default:
		return ScopeGlobalOnly()
	}
}

func (b *DefaultPermissionBroker) isScopeAllowed(def PermissionDefinition, scope PermissionScope) bool {
	if scope.Type == ScopeGlobal {
		return true
	}
	for _, allowed := range def.AllowedScopes {
		if allowed == scope.Type {
			return true
		}
	}
	return false
}

func (b *DefaultPermissionBroker) matchGrants(grants []PermissionGrant, scope PermissionScope, req PermissionRequirement, execCtx PermissionExecutionContext, input json.RawMessage) []PermissionGrant {
	def, _ := b.registry.Get(req.PermissionID)
	matched := make([]PermissionGrant, 0)
	for _, g := range grants {
		if !g.IsValid() {
			continue
		}
		// RequiresPerUse permissions must never be satisfied by a legacy/session/
		// persistent grant. Only an explicit allow_once grant may authorize one
		// operation.
		if def.RequiresPerUse && !g.IsOneTime() {
			continue
		}
		if scope.Type != ScopeGlobal && !g.Scope.Contains(scope) {
			continue
		}
		if !g.TargetBinding.MatchesExecutionContext(execCtx) {
			continue
		}
		if b.requiresBoundGrant(def, execCtx) && g.TargetBinding == nil {
			continue
		}
		if g.InputBinding != nil {
			if len(input) == 0 || g.InputBinding.InputHash != permissionInputHash(input) {
				continue
			}
		}
		if g.IsOneTime() || g.IsPersistent() || g.Decision == DecisionAllowSession || g.Decision == DecisionAllow {
			matched = append(matched, g)
		}
	}
	return matched
}

func permissionInputHash(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	h := sha256.Sum256(input)
	return hex.EncodeToString(h[:])
}

func decisionForMatchedGrants(grants []PermissionGrant) PermissionDecision {
	for _, grant := range grants {
		if grant.IsOneTime() {
			return DecisionAllowOnce
		}
	}
	return DecisionAllow
}

func (b *DefaultPermissionBroker) determineForcedApproval(result *PermissionEvaluationResult, request PermissionEvaluationRequest) {
	result.Decision = DecisionRequireApproval
	result.ApprovalRequest = b.buildApprovalRequest(request, result.Missing)
}

func (b *DefaultPermissionBroker) determineDenyOrApproval(result *PermissionEvaluationResult, request PermissionEvaluationRequest) {
	hasApprovalPath := false
	for _, req := range result.Missing {
		if req.Optional {
			continue
		}
		def, ok := b.registry.Get(req.PermissionID)
		if !ok {
			result.Decision = DecisionDeny
			return
		}
		if def.DefaultApproval == ApprovalManual || def.DefaultApproval == ApprovalFullControl {
			hasApprovalPath = true
		}
		if def.DefaultApproval == ApprovalDeny {
			result.Decision = DecisionDeny
			return
		}
	}

	if hasApprovalPath {
		result.Decision = DecisionRequireApproval
	} else {
		result.Decision = DecisionDeny
	}

	if result.Decision == DecisionRequireApproval || result.Decision == DecisionDeny {
		result.ApprovalRequest = b.buildApprovalRequest(request, result.Missing)
	}
}

func (b *DefaultPermissionBroker) buildApprovalRequest(request PermissionEvaluationRequest, missing []PermissionRequirement) *ApprovalRequest {
	approval := &ApprovalRequest{
		Source:           request.ExecutionContext.Source,
		RiskLevel:        request.RiskLevel,
		SideEffects:      request.SideEffects,
		Target:           request.Target,
		ExecutionContext: request.ExecutionContext,
		RemoteExecution:  request.ExecutionContext.IsDeviceExecution(),
	}
	return approval
}

func (b *DefaultPermissionBroker) isNotTrusted(subject PermissionSubject) bool {
	b.mu.RLock()
	checker := b.trustChecker
	b.mu.RUnlock()
	if checker == nil {
		return true
	}
	return !checker.IsTrusted(subject)
}

func (b *DefaultPermissionBroker) generateGrantID(request PermissionGrantRequest) string {
	input := fmt.Sprintf("%s:%s:%s:%d", request.Subject.ID, request.PermissionID, request.Scope.Type, time.Now().UnixNano())
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16])
}

func (b *DefaultPermissionBroker) grantToStored(grant PermissionGrant) StoredGrant {
	scopeData, _ := json.Marshal(grant.Scope)
	var ib json.RawMessage
	if grant.InputBinding != nil {
		ib, _ = json.Marshal(grant.InputBinding)
	}
	var tb json.RawMessage
	if grant.TargetBinding != nil {
		tb, _ = json.Marshal(grant.TargetBinding)
	}

	return StoredGrant{
		GrantID:       grant.GrantID,
		SubjectType:   string(grant.Subject.Type),
		SubjectID:     grant.Subject.ID,
		PermissionID:  grant.PermissionID,
		ScopeType:     string(grant.Scope.Type),
		ScopeID:       grant.Scope.ID,
		ScopeData:     scopeData,
		Decision:      string(grant.Decision),
		InputBinding:  ib,
		TargetBinding: tb,
		IssuedAt:      grant.IssuedAt,
		ExpiresAt:     grant.ExpiresAt,
		IssuedBy:      string(grant.IssuedBy),
		Reason:        grant.Reason,
		RevokedAt:     grant.RevokedAt,
		ManifestVer:   grant.ManifestVer,
	}
}

func (b *DefaultPermissionBroker) storedToGrant(sg StoredGrant) PermissionGrant {
	var scope PermissionScope
	if sg.ScopeData != nil {
		json.Unmarshal(sg.ScopeData, &scope)
	}
	if scope.Type == "" {
		scope.Type = ScopeType(sg.ScopeType)
		scope.ID = sg.ScopeID
	}

	var inputBinding *InputBinding
	if sg.InputBinding != nil {
		var ib InputBinding
		if json.Unmarshal(sg.InputBinding, &ib) == nil {
			inputBinding = &ib
		}
	}

	var targetBinding *TargetBinding
	if sg.TargetBinding != nil {
		var tb TargetBinding
		if json.Unmarshal(sg.TargetBinding, &tb) == nil {
			targetBinding = &tb
		}
	}

	return PermissionGrant{
		GrantID:       sg.GrantID,
		Subject:       PermissionSubject{Type: SubjectType(sg.SubjectType), ID: sg.SubjectID},
		PermissionID:  sg.PermissionID,
		Scope:         scope,
		Decision:      PermissionDecision(sg.Decision),
		InputBinding:  inputBinding,
		TargetBinding: targetBinding,
		IssuedAt:      sg.IssuedAt,
		ExpiresAt:     sg.ExpiresAt,
		IssuedBy:      GrantIssuer(sg.IssuedBy),
		Reason:        sg.Reason,
		RevokedAt:     sg.RevokedAt,
		ManifestVer:   sg.ManifestVer,
	}
}
