package permission

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PermissionDecision string

const (
	DecisionDeny            PermissionDecision = "deny"
	DecisionAllow           PermissionDecision = "allow"
	DecisionRequireApproval PermissionDecision = "require_approval"
	DecisionAllowOnce       PermissionDecision = "allow_once"
	DecisionAllowSession    PermissionDecision = "allow_session"
	DecisionAllowPersistent PermissionDecision = "allow_persistent"
)

type GrantIssuer string

const (
	IssuerUser   GrantIssuer = "user"
	IssuerSystem GrantIssuer = "system"
	IssuerPolicy GrantIssuer = "policy"
)

type InputBinding struct {
	InputHash string `json:"inputHash"`
}

type TargetBinding struct {
	TargetType          string                    `json:"targetType"`
	TargetID            string                    `json:"targetId"`
	TargetHash          string                    `json:"targetHash,omitempty"`
	ExecutionBindingKey string                    `json:"executionBindingKey,omitempty"`
	ProviderID          string                    `json:"providerId,omitempty"`
	ProviderInstanceID  string                    `json:"providerInstanceId,omitempty"`
	DeviceID            runtimeidentity.DeviceID  `json:"deviceId,omitempty"`
	RuntimeID           runtimeidentity.RuntimeID `json:"runtimeId,omitempty"`
}

func (b *TargetBinding) MatchesExecutionContext(ctx PermissionExecutionContext) bool {
	if b == nil {
		return true
	}
	if b.ProviderID != "" && b.ProviderID != ctx.ProviderID {
		return false
	}
	if b.ProviderInstanceID != "" && b.ProviderInstanceID != ctx.ProviderInstanceID {
		return false
	}
	if b.DeviceID != "" && b.DeviceID != ctx.DeviceID {
		return false
	}
	if b.RuntimeID != "" && b.RuntimeID != ctx.RuntimeID {
		return false
	}
	if b.ExecutionBindingKey != "" && b.ExecutionBindingKey != ctx.StableBindingKey() {
		return false
	}
	return true
}

type PermissionGrant struct {
	GrantID       string             `json:"grantId"`
	Subject       PermissionSubject  `json:"subject"`
	PermissionID  string             `json:"permissionId"`
	Scope         PermissionScope    `json:"scope"`
	Decision      PermissionDecision `json:"decision"`
	InputBinding  *InputBinding      `json:"inputBinding,omitempty"`
	TargetBinding *TargetBinding     `json:"targetBinding,omitempty"`
	IssuedAt      time.Time          `json:"issuedAt"`
	ExpiresAt     *time.Time         `json:"expiresAt,omitempty"`
	IssuedBy      GrantIssuer        `json:"issuedBy"`
	Reason        string             `json:"reason,omitempty"`
	RevokedAt     *time.Time         `json:"revokedAt,omitempty"`
	ManifestVer   string             `json:"manifestVer,omitempty"`
	Metadata      map[string]any     `json:"metadata,omitempty"`
}

func (g PermissionGrant) IsValid() bool {
	if g.RevokedAt != nil {
		return false
	}
	if g.ExpiresAt != nil && time.Now().After(*g.ExpiresAt) {
		return false
	}
	if g.Decision == DecisionDeny {
		return false
	}
	return true
}

func (g PermissionGrant) IsOneTime() bool {
	return g.Decision == DecisionAllowOnce
}

func (g PermissionGrant) IsPersistent() bool {
	return g.Decision == DecisionAllowPersistent || g.Decision == DecisionAllow
}

type PermissionGrantRequest struct {
	Subject       PermissionSubject  `json:"subject"`
	PermissionID  string             `json:"permissionId"`
	Scope         PermissionScope    `json:"scope"`
	Decision      PermissionDecision `json:"decision"`
	InputHash     string             `json:"inputHash,omitempty"`
	TargetBinding *TargetBinding     `json:"targetBinding,omitempty"`
	ExpiresAt     *time.Time         `json:"expiresAt,omitempty"`
	IssuedBy      GrantIssuer        `json:"issuedBy"`
	Reason        string             `json:"reason,omitempty"`
	ManifestVer   string             `json:"manifestVer,omitempty"`
}

type PermissionGrantFilter struct {
	Subject      *PermissionSubject  `json:"subject,omitempty"`
	PermissionID string              `json:"permissionId,omitempty"`
	Scope        *PermissionScope    `json:"scope,omitempty"`
	Decision     *PermissionDecision `json:"decision,omitempty"`
	ActiveOnly   bool                `json:"activeOnly,omitempty"`
}

type PermissionExplanation struct {
	Decision        PermissionDecision `json:"decision"`
	Reasons         []PermissionReason `json:"reasons"`
	MatchedGrants   []PermissionGrant  `json:"matchedGrants"`
	RequiredAction  string             `json:"requiredAction,omitempty"`
	AvailableScopes []ScopeType        `json:"availableScopes,omitempty"`
}

type StoredGrant struct {
	GrantID       string          `json:"grantId"`
	SubjectType   string          `json:"subjectType"`
	SubjectID     string          `json:"subjectId"`
	PermissionID  string          `json:"permissionId"`
	ScopeType     string          `json:"scopeType"`
	ScopeID       string          `json:"scopeId"`
	ScopeData     json.RawMessage `json:"scopeData,omitempty"`
	Decision      string          `json:"decision"`
	InputBinding  json.RawMessage `json:"inputBinding,omitempty"`
	TargetBinding json.RawMessage `json:"targetBinding,omitempty"`
	IssuedAt      time.Time       `json:"issuedAt"`
	ExpiresAt     *time.Time      `json:"expiresAt,omitempty"`
	IssuedBy      string          `json:"issuedBy"`
	Reason        string          `json:"reason"`
	RevokedAt     *time.Time      `json:"revokedAt,omitempty"`
	ManifestVer   string          `json:"manifestVer,omitempty"`
}

func TargetBindingFromExecutionContext(ctx PermissionExecutionContext) *TargetBinding {
	if ctx.IsEmpty() {
		return nil
	}
	return &TargetBinding{
		TargetType:          "execution_context",
		TargetID:            ctx.StableBindingKey(),
		TargetHash:          ctx.StableBindingKey(),
		ExecutionBindingKey: ctx.StableBindingKey(),
		ProviderID:          ctx.ProviderID,
		ProviderInstanceID:  ctx.ProviderInstanceID,
		DeviceID:            ctx.DeviceID,
		RuntimeID:           ctx.RuntimeID,
	}
}

func NewExecutionTargetBinding(ctx PermissionExecutionContext) *TargetBinding {
	return TargetBindingFromExecutionContext(ctx)
}
