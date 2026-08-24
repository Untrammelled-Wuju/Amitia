package permission

import "time"

type Decision int

const (
	DecisionDenied Decision = iota
	DecisionAllowed
	DecisionRequireApproval
)

type DenyReason string

const (
	ReasonNotDeclared         DenyReason = "not_declared"
	ReasonNotGranted          DenyReason = "not_granted"
	ReasonScopeDenied         DenyReason = "scope_denied"
	ReasonPolicyDenied        DenyReason = "host_policy_denied"
	ReasonUnknownPerm         DenyReason = "unknown_permission"
	ReasonInvalidSubject      DenyReason = "invalid_subject"
	ReasonRuntimeInactive     DenyReason = "runtime_inactive"
	ReasonSnapshotUnavailable DenyReason = "snapshot_unavailable"
)

type DecisionResult struct {
	Decision Decision
	Reason   DenyReason
	Detail   string
}

func (r DecisionResult) Allowed() bool {
	return r.Decision == DecisionAllowed
}

type PermissionCheck struct {
	PermissionID string
	Decision     Decision
	Reason       DenyReason
	Detail       string
}

type EffectiveView struct {
	Subject    EffectiveSubject
	Revision   string
	ResolvedAt time.Time
	Checks     []PermissionCheck
}

func (v *EffectiveView) Allowed(permID string) bool {
	for _, c := range v.Checks {
		if c.PermissionID == permID {
			return c.Decision == DecisionAllowed
		}
	}
	return false
}

func (v *EffectiveView) DenyReasons() []PermissionCheck {
	result := make([]PermissionCheck, 0)
	for _, c := range v.Checks {
		if c.Decision != DecisionAllowed {
			result = append(result, c)
		}
	}
	return result
}

func (v *EffectiveView) AllowedPermissions() []string {
	result := make([]string, 0)
	for _, c := range v.Checks {
		if c.Decision == DecisionAllowed {
			result = append(result, c.PermissionID)
		}
	}
	return result
}
