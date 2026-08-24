package permission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PermissionDecisionHostPolicy func(ctx context.Context, subject EffectiveSubject, permID string) (allowed bool, handled bool)

type Broker interface {
	Evaluate(ctx context.Context, request permission.PermissionEvaluationRequest) permission.PermissionEvaluationResult
}

type EffectivePermissionAdapter struct {
	broker        Broker
	policy        PermissionDecisionHostPolicy
	subjectMapper *GameHostSubjectMapper
	approvals     *ApprovalCoordinator
	clock         func() time.Time
}

func NewEffectivePermissionAdapter(
	broker Broker,
	policy PermissionDecisionHostPolicy,
	mapper *GameHostSubjectMapper,
) *EffectivePermissionAdapter {
	return &EffectivePermissionAdapter{
		broker:        broker,
		policy:        policy,
		subjectMapper: mapper,
		clock:         time.Now,
	}
}

func NewEffectivePermissionAdapterWithClock(
	broker Broker,
	policy PermissionDecisionHostPolicy,
	mapper *GameHostSubjectMapper,
	clock func() time.Time,
) *EffectivePermissionAdapter {
	return &EffectivePermissionAdapter{
		broker:        broker,
		policy:        policy,
		subjectMapper: mapper,
		clock:         clock,
	}
}

func (a *EffectivePermissionAdapter) SetApprovalCoordinator(coordinator *ApprovalCoordinator) {
	if a == nil {
		return
	}
	a.approvals = coordinator
}

func (a *EffectivePermissionAdapter) Check(
	ctx context.Context,
	subject EffectiveSubject,
	permID string,
) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	if permID == "" {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "empty permission id"}
	}

	if subject.RuntimeID == "" {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "runtime id required"}
	}

	if subject.PluginID == "" {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "plugin id required"}
	}

	if subject.ExtensionID == "" {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "extension id required"}
	}

	if a.policy != nil {
		allowed, handled := a.policy(ctx, subject, permID)
		if handled && !allowed {
			return DecisionResult{Decision: DecisionDenied, Reason: ReasonPolicyDenied, Detail: "host policy denied"}
		}
	}

	kernelSubject := subject.KernelSubject()
	return a.kernelEvaluate(ctx, subject, kernelSubject, permID, permission.PermissionTarget{})
}

func (a *EffectivePermissionAdapter) CheckTarget(
	ctx context.Context,
	subject EffectiveSubject,
	permID string,
	target permission.PermissionTarget,
) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	if permID == "" || subject.RuntimeID == "" || subject.PluginID == "" || subject.ExtensionID == "" {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "invalid permission subject or permission id"}
	}
	if a.policy != nil {
		allowed, handled := a.policy(ctx, subject, permID)
		if handled && !allowed {
			return DecisionResult{Decision: DecisionDenied, Reason: ReasonPolicyDenied, Detail: "host policy denied"}
		}
	}
	return a.kernelEvaluate(ctx, subject, subject.KernelSubject(), permID, target)
}

func (a *EffectivePermissionAdapter) kernelEvaluate(
	ctx context.Context,
	effectiveSubject EffectiveSubject,
	kernelSubject permission.PermissionSubject,
	permID string,
	target permission.PermissionTarget,
) DecisionResult {
	return a.kernelEvaluateMode(ctx, effectiveSubject, kernelSubject, permID, target, true)
}

// kernelEvaluateMode keeps permission inspection side-effect free. Interactive
// evaluation is reserved for an operation that is actually about to execute;
// diagnostic/effective-view reads must never create a pending approval request.
func (a *EffectivePermissionAdapter) kernelEvaluateMode(
	ctx context.Context,
	effectiveSubject EffectiveSubject,
	kernelSubject permission.PermissionSubject,
	permID string,
	target permission.PermissionTarget,
	interactive bool,
) DecisionResult {
	scope := permission.ScopeForExtension(effectiveSubject.ExtensionID)
	moduleID := effectiveSubject.EffectiveModuleID()
	if moduleID != "" {
		scope = permission.PermissionScope{Type: permission.ScopeModule, ID: moduleID}
	}
	request := permission.PermissionEvaluationRequest{
		Subject: kernelSubject,
		Requirements: []permission.PermissionRequirement{{
			PermissionID: permID,
			Scope:        scope,
		}},
		Target: target,
		ExecutionContext: permission.PermissionExecutionContext{
			Placement:   permission.ExecutionPlacementLocal,
			RuntimeID:   runtimeidentity.ParseRuntimeID(effectiveSubject.RuntimeID),
			ExtensionID: effectiveSubject.ExtensionID,
			ModuleID:    moduleID,
			Source:      "gamehost",
		},
	}
	var kernelResult permission.PermissionEvaluationResult
	if interactive && a.approvals != nil {
		kernelResult = a.approvals.Evaluate(ctx, effectiveSubject, permID, request)
	} else {
		kernelResult = a.broker.Evaluate(ctx, request)
	}

	switch kernelResult.Decision {
	case permission.DecisionAllow, permission.DecisionAllowOnce, permission.DecisionAllowSession, permission.DecisionAllowPersistent:
		return DecisionResult{Decision: DecisionAllowed}
	case permission.DecisionRequireApproval:
		return DecisionResult{Decision: DecisionRequireApproval}
	default:
		reason := mapKernelDenyReason(kernelResult.Reasons)
		detail := ""
		if len(kernelResult.Missing) > 0 {
			detail = kernelResult.Missing[0].PermissionID
		}
		return DecisionResult{Decision: DecisionDenied, Reason: reason, Detail: detail}
	}
}

func (a *EffectivePermissionAdapter) InspectRuntimePermission(
	ctx context.Context,
	runtimeID string,
	pluginID string,
	permID string,
) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	subject, err := a.subjectMapper.MapSubject(runtimeID, pluginID)
	if err != nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: err.Error()}
	}
	return a.inspect(ctx, subject, permID)
}

func (a *EffectivePermissionAdapter) InspectServicePermission(
	ctx context.Context,
	runtimeID string,
	pluginID string,
	serviceID string,
	permID string,
) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	subject, err := a.subjectMapper.MapServiceSubject(runtimeID, pluginID, serviceID)
	if err != nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: err.Error()}
	}
	return a.inspect(ctx, subject, permID)
}

func (a *EffectivePermissionAdapter) inspect(ctx context.Context, subject EffectiveSubject, permID string) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	if permID == "" || subject.RuntimeID == "" || subject.PluginID == "" || subject.ExtensionID == "" {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "invalid permission subject or permission id"}
	}
	if a.policy != nil {
		allowed, handled := a.policy(ctx, subject, permID)
		if handled && !allowed {
			return DecisionResult{Decision: DecisionDenied, Reason: ReasonPolicyDenied, Detail: "host policy denied"}
		}
	}
	return a.kernelEvaluateMode(ctx, subject, subject.KernelSubject(), permID, permission.PermissionTarget{}, false)
}

func (a *EffectivePermissionAdapter) CheckRuntimePermission(
	ctx context.Context,
	runtimeID string,
	pluginID string,
	permID string,
) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	subject, err := a.subjectMapper.MapSubject(runtimeID, pluginID)
	if err != nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: err.Error()}
	}
	return a.Check(ctx, subject, permID)
}

func (a *EffectivePermissionAdapter) CheckServicePermission(
	ctx context.Context,
	runtimeID string,
	pluginID string,
	serviceID string,
	permID string,
) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	subject, err := a.subjectMapper.MapServiceSubject(runtimeID, pluginID, serviceID)
	if err != nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: err.Error()}
	}
	return a.Check(ctx, subject, permID)
}

func (a *EffectivePermissionAdapter) CheckServicePermissionTarget(
	ctx context.Context,
	runtimeID string,
	pluginID string,
	serviceID string,
	permID string,
	target permission.PermissionTarget,
) DecisionResult {
	if a == nil || a.broker == nil || a.subjectMapper == nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: "permission adapter unavailable"}
	}
	subject, err := a.subjectMapper.MapServiceSubject(runtimeID, pluginID, serviceID)
	if err != nil {
		return DecisionResult{Decision: DecisionDenied, Reason: ReasonInvalidSubject, Detail: err.Error()}
	}
	return a.CheckTarget(ctx, subject, permID, target)
}

func (a *EffectivePermissionAdapter) ResolveRuntimePermissions(
	ctx context.Context,
	subject EffectiveSubject,
	declared ...string,
) *EffectiveView {
	return a.resolvePermissions(ctx, subject, declared)
}

func (a *EffectivePermissionAdapter) ResolveServicePermissions(
	ctx context.Context,
	subject EffectiveSubject,
	declared ...string,
) *EffectiveView {
	return a.resolvePermissions(ctx, subject, declared)
}

func (a *EffectivePermissionAdapter) resolvePermissions(
	ctx context.Context,
	subject EffectiveSubject,
	declared []string,
) *EffectiveView {
	view := &EffectiveView{
		Subject:    subject,
		ResolvedAt: a.clock(),
		Checks:     make([]PermissionCheck, 0, len(declared)),
	}

	for _, permID := range declared {
		if permID == "" {
			continue
		}

		decision := a.inspect(ctx, subject, permID)
		view.Checks = append(view.Checks, PermissionCheck{
			PermissionID: permID,
			Decision:     decision.Decision,
			Reason:       decision.Reason,
			Detail:       decision.Detail,
		})
	}
	view.Revision = permissionViewRevision(subject, view.Checks)

	return view
}

func (a *EffectivePermissionAdapter) MapSubject(runtimeID string, pluginID string) (EffectiveSubject, error) {
	if a == nil || a.subjectMapper == nil {
		return EffectiveSubject{}, ErrInvalidSubject
	}
	return a.subjectMapper.MapSubject(runtimeID, pluginID)
}

func (a *EffectivePermissionAdapter) MapServiceSubject(runtimeID string, pluginID string, serviceID string) (EffectiveSubject, error) {
	if a == nil || a.subjectMapper == nil {
		return EffectiveSubject{}, ErrInvalidSubject
	}
	return a.subjectMapper.MapServiceSubject(runtimeID, pluginID, serviceID)
}

func permissionViewRevision(subject EffectiveSubject, checks []PermissionCheck) string {
	stable := append([]PermissionCheck(nil), checks...)
	sort.Slice(stable, func(i, j int) bool {
		if stable[i].PermissionID != stable[j].PermissionID {
			return stable[i].PermissionID < stable[j].PermissionID
		}
		if stable[i].Decision != stable[j].Decision {
			return stable[i].Decision < stable[j].Decision
		}
		if stable[i].Reason != stable[j].Reason {
			return stable[i].Reason < stable[j].Reason
		}
		return stable[i].Detail < stable[j].Detail
	})
	h := sha256.New()
	_, _ = h.Write([]byte(subject.RuntimeID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(subject.PluginID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(subject.ServiceID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(subject.ModuleID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(subject.ExtensionID))
	for _, check := range stable {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(check.PermissionID))
		_, _ = h.Write([]byte{0, byte(check.Decision), 0})
		_, _ = h.Write([]byte(check.Reason))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(check.Detail))
	}
	return "prev-" + hex.EncodeToString(h.Sum(nil)[:16])
}

func mapKernelDenyReason(reasons []permission.PermissionReason) DenyReason {
	for _, r := range reasons {
		switch r.Code {
		case "unknown_permission":
			return ReasonUnknownPerm
		case "missing_grant":
			return ReasonNotGranted
		case "scope_not_allowed":
			return ReasonScopeDenied
		case "system_policy_deny":
			return ReasonPolicyDenied
		case "trusted_only":
			return ReasonNotGranted
		case "background_not_allowed":
			return ReasonScopeDenied
		}
	}
	return ReasonNotGranted
}

var _ = domain.RuntimeState("")
