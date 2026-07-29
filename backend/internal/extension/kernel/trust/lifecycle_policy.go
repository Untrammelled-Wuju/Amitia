package trust

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type LifecycleAction string

const (
	LifecycleActionAllow            LifecycleAction = "allow"
	LifecycleActionAllowWithConfirm LifecycleAction = "allow_with_confirmation"
	LifecycleActionInstallDisabled  LifecycleAction = "install_disabled"
	LifecycleActionDeny             LifecycleAction = "deny"
	LifecycleActionQuarantine       LifecycleAction = "quarantine"
)

type RuntimeTypeRisk string

const (
	RuntimeRiskNone     RuntimeTypeRisk = "none"
	RuntimeRiskLow      RuntimeTypeRisk = "low"
	RuntimeRiskMedium   RuntimeTypeRisk = "medium"
	RuntimeRiskHigh     RuntimeTypeRisk = "high"
	RuntimeRiskCritical RuntimeTypeRisk = "critical"
)

type PermissionRiskSummary struct {
	HighRiskCount    int
	MediumRiskCount  int
	LowRiskCount     int
	HasNetworkAccess bool
	HasFileAccess    bool
	HasSecretAccess  bool
	HasSubprocess    bool
}

type PolicyInput struct {
	PublisherID          string
	KeyID                string
	PackageHash          string
	ExtensionID          string
	Version              string
	SignatureResult      SignatureVerificationResult
	RuntimeTypeRisk      RuntimeTypeRisk
	PermissionRisk       PermissionRiskSummary
	HasPlatformBinary    bool
	IsUpdate             bool
	PreviousPublisherID  string
	PermissionExpanded   bool
	HasHighRiskMigration bool
	WorkspacePath        string
	UserSettings         UserPolicySettings
}

type UserPolicySettings struct {
	AllowUnknownPublisher                    bool
	AllowAutoUpdateUnknown                   bool
	RequireConfirmationForBinary             bool
	RequireConfirmationForHighRiskPermission bool
	BlockAllNonOfficial                      bool
}

type PolicyResult struct {
	Action            LifecycleAction
	Reason            string
	TrustLevel        TrustLevel
	EffectiveTrust    TrustLevel
	Warnings          []string
	Confirmations     []ConfirmationRequest
	RequiresUserInput bool
	QuarantineReason  string
}

type ConfirmationRequest struct {
	Field   string
	Message string
	Default bool
}

type LifecyclePolicyEngine struct {
	store          *PublisherStore
	userTrust      *UserTrustStore
	revocationList *RevocationList
	blocklist      *PackageBlocklist
}

func NewLifecyclePolicyEngine(store *PublisherStore, userTrust *UserTrustStore, revocationList *RevocationList, blocklist *PackageBlocklist) *LifecyclePolicyEngine {
	return &LifecyclePolicyEngine{
		store:          store,
		userTrust:      userTrust,
		revocationList: revocationList,
		blocklist:      blocklist,
	}
}

func (e *LifecyclePolicyEngine) Evaluate(ctx context.Context, input PolicyInput) PolicyResult {
	result := PolicyResult{
		Action: LifecycleActionAllow,
	}

	if blocked := e.blocklist.Check(input.PackageHash); blocked != nil {
		result.Action = LifecycleActionDeny
		result.Reason = fmt.Sprintf("package blocked: %s (%s)", blocked.Reason, blocked.Details)
		return result
	}

	if rev := e.revocationList.CheckPackage(input.PackageHash); rev != nil {
		result.Action = LifecycleActionDeny
		result.Reason = fmt.Sprintf("package revoked: %s", rev.Reason)
		return result
	}

	if rev := e.revocationList.CheckKey(input.PublisherID, input.KeyID); rev != nil {
		result.Action = LifecycleActionDeny
		result.Reason = fmt.Sprintf("key revoked: %s", rev.Reason)
		return result
	}

	if rev := e.revocationList.CheckPublisher(input.PublisherID); rev != nil {
		result.Action = LifecycleActionDeny
		result.Reason = fmt.Sprintf("publisher revoked: %s", rev.Reason)
		return result
	}

	if rev := e.revocationList.CheckExtension(input.ExtensionID, input.Version); rev != nil {
		result.Action = LifecycleActionDeny
		result.Reason = fmt.Sprintf("extension version revoked: %s", rev.Reason)
		return result
	}

	publisherLevel := TrustLevelUnknown
	if identity, err := e.store.Get(ctx, input.PublisherID); err == nil {
		publisherLevel = identity.TrustLevel
	}
	result.TrustLevel = publisherLevel

	workspaceDecision := e.userTrust.Lookup(TrustScopeWorkspace, "", "", "", "", "")
	if workspaceDecision != nil && workspaceDecision.WorkspacePath == input.WorkspacePath {
		publisherLevel = TrustLevelDevelopment
		result.EffectiveTrust = TrustLevelDevelopment
	}

	if userDecision := e.userTrust.Lookup(TrustScopePackage, input.PublisherID, input.KeyID, input.PackageHash, "", ""); userDecision != nil {
		if userDecision.GrantedLevel == TrustLevelBlocked {
			result.Action = LifecycleActionDeny
			result.Reason = "user blocked package"
			return result
		}
		publisherLevel = userDecision.GrantedLevel
	} else if userDecision := e.userTrust.Lookup(TrustScopeKey, input.PublisherID, input.KeyID, "", "", ""); userDecision != nil {
		if userDecision.GrantedLevel == TrustLevelBlocked {
			result.Action = LifecycleActionDeny
			result.Reason = "user blocked key"
			return result
		}
		publisherLevel = userDecision.GrantedLevel
	} else if userDecision := e.userTrust.Lookup(TrustScopePublisher, input.PublisherID, "", "", "", ""); userDecision != nil {
		if userDecision.GrantedLevel == TrustLevelBlocked {
			result.Action = LifecycleActionDeny
			result.Reason = "user blocked publisher"
			return result
		}
		publisherLevel = userDecision.GrantedLevel
	} else if userDecision := e.userTrust.Lookup(TrustScopeVersion, "", "", "", input.ExtensionID, input.Version); userDecision != nil {
		if userDecision.GrantedLevel == TrustLevelBlocked {
			result.Action = LifecycleActionDeny
			result.Reason = "user blocked version"
			return result
		}
		publisherLevel = userDecision.GrantedLevel
	}

	result.EffectiveTrust = publisherLevel

	if !input.SignatureResult.Valid {
		switch input.SignatureResult.Status {
		case SignatureStatusUnknownKey:
			if input.UserSettings.BlockAllNonOfficial {
				result.Action = LifecycleActionDeny
				result.Reason = "non-official package blocked by user setting"
				return result
			}
			if !input.UserSettings.AllowUnknownPublisher {
				result.Action = LifecycleActionInstallDisabled
				result.Reason = "unknown publisher; user confirmation required"
				result.Confirmations = append(result.Confirmations, ConfirmationRequest{
					Field:   "trust_unknown_publisher",
					Message: fmt.Sprintf("Publisher %s is unknown. Install anyway (disabled by default)?", input.PublisherID),
					Default: false,
				})
				result.RequiresUserInput = true
				return result
			}
		case SignatureStatusInvalidSignature, SignatureStatusPublisherMismatch, SignatureStatusContentMismatch, SignatureStatusPayloadMismatch:
			result.Action = LifecycleActionDeny
			result.Reason = fmt.Sprintf("signature verification failed: %s", input.SignatureResult.Status)
			return result
		case SignatureStatusRevokedKey:
			result.Action = LifecycleActionDeny
			result.Reason = "key revoked"
			return result
		case SignatureStatusExpiredKey:
			result.Action = LifecycleActionAllowWithConfirm
			result.Reason = "key expired; confirmation required"
			result.Confirmations = append(result.Confirmations, ConfirmationRequest{
				Field:   "accept_expired_key",
				Message: "Signing key has expired. Continue install?",
				Default: false,
			})
			result.RequiresUserInput = true
			return result
		default:
			result.Action = LifecycleActionDeny
			result.Reason = fmt.Sprintf("signature invalid: %s", input.SignatureResult.Status)
			return result
		}
	}

	if publisherLevel.IsBlocked() {
		result.Action = LifecycleActionDeny
		result.Reason = "publisher blocked or revoked"
		return result
	}

	if input.IsUpdate && input.PreviousPublisherID != "" && input.PreviousPublisherID != input.PublisherID {
		result.Action = LifecycleActionAllowWithConfirm
		result.Reason = "ownership transfer detected"
		result.Confirmations = append(result.Confirmations, ConfirmationRequest{
			Field:   "accept_ownership_transfer",
			Message: fmt.Sprintf("Extension ownership changed from %s to %s. Continue?", input.PreviousPublisherID, input.PublisherID),
			Default: false,
		})
		result.RequiresUserInput = true
		result.Warnings = append(result.Warnings, "ownership transfer pauses auto-update")
		return result
	}

	if input.PermissionExpanded && publisherLevel != TrustLevelOfficial && publisherLevel != TrustLevelTrusted {
		result.Action = LifecycleActionAllowWithConfirm
		result.Reason = "permission expansion requires confirmation"
		result.Confirmations = append(result.Confirmations, ConfirmationRequest{
			Field:   "accept_permission_expansion",
			Message: "The new version requests additional permissions.",
			Default: false,
		})
		result.RequiresUserInput = true
	}

	if input.HasPlatformBinary && input.UserSettings.RequireConfirmationForBinary {
		result.Action = LifecycleActionAllowWithConfirm
		result.Reason = "platform binary requires confirmation"
		result.Confirmations = append(result.Confirmations, ConfirmationRequest{
			Field:   "accept_platform_binary",
			Message: "This package contains platform binaries.",
			Default: false,
		})
		result.RequiresUserInput = true
	}

	if input.RuntimeTypeRisk == RuntimeRiskCritical && !publisherLevel.AllowsHighRiskRuntime() {
		result.Action = LifecycleActionDeny
		result.Reason = "high-risk runtime requires official or trusted publisher"
		return result
	}

	if input.RuntimeTypeRisk == RuntimeRiskHigh && !publisherLevel.AllowsHighRiskRuntime() {
		result.Action = LifecycleActionAllowWithConfirm
		result.Reason = "high-risk runtime requires confirmation"
		result.Confirmations = append(result.Confirmations, ConfirmationRequest{
			Field:   "accept_high_risk_runtime",
			Message: "This package declares a high-risk runtime.",
			Default: false,
		})
		result.RequiresUserInput = true
	}

	if input.PermissionRisk.HasSubprocess && publisherLevel != TrustLevelOfficial && publisherLevel != TrustLevelTrusted {
		result.Action = LifecycleActionAllowWithConfirm
		result.Reason = "subprocess permission requires confirmation"
		result.Confirmations = append(result.Confirmations, ConfirmationRequest{
			Field:   "accept_subprocess",
			Message: "This package requests subprocess execution permission.",
			Default: false,
		})
		result.RequiresUserInput = true
	}

	if input.HasHighRiskMigration {
		result.Action = LifecycleActionAllowWithConfirm
		result.Reason = "high-risk data migration requires confirmation"
		result.Confirmations = append(result.Confirmations, ConfirmationRequest{
			Field:   "accept_high_risk_migration",
			Message: "This update includes data migrations.",
			Default: false,
		})
		result.RequiresUserInput = true
	}

	if !publisherLevel.AllowsInstallation() {
		result.Action = LifecycleActionInstallDisabled
		result.Reason = "publisher trust level does not allow installation"
		return result
	}

	result.Action = LifecycleActionAllow
	result.Reason = "policy evaluation passed"
	return result
}

type AutoUpdateEligibility struct {
	Eligible bool
	Reason   string
	Warnings []string
}

func (e *LifecyclePolicyEngine) EvaluateAutoUpdate(ctx context.Context, input PolicyInput) AutoUpdateEligibility {
	result := AutoUpdateEligibility{Eligible: true}

	if !input.SignatureResult.Valid {
		result.Eligible = false
		result.Reason = "signature invalid"
		return result
	}

	publisherLevel := result_fromEffectiveOrStore(e, ctx, input)
	if !publisherLevel.AllowsAutoUpdate() {
		result.Eligible = false
		result.Reason = "publisher not eligible for auto-update"
		return result
	}

	if input.IsUpdate && input.PreviousPublisherID != "" && input.PreviousPublisherID != input.PublisherID {
		result.Eligible = false
		result.Reason = "ownership transfer detected"
		return result
	}

	if input.PermissionExpanded {
		result.Eligible = false
		result.Reason = "permission expansion requires user confirmation"
		return result
	}

	if input.HasHighRiskMigration {
		result.Eligible = false
		result.Reason = "high-risk migration requires user confirmation"
		return result
	}

	if e.revocationList.CheckKey(input.PublisherID, input.KeyID) != nil {
		result.Eligible = false
		result.Reason = "key revoked"
		return result
	}

	if e.revocationList.CheckExtension(input.ExtensionID, input.Version) != nil {
		result.Eligible = false
		result.Reason = "extension version revoked"
		return result
	}

	if e.blocklist.Check(input.PackageHash) != nil {
		result.Eligible = false
		result.Reason = "package blocked"
		return result
	}

	if input.HasPlatformBinary && !publisherLevel.AllowsHighRiskRuntime() {
		result.Warnings = append(result.Warnings, "platform binary present in auto-update")
	}

	if !input.IsUpdate {
		result.Warnings = append(result.Warnings, "first install; user confirmation recommended")
	}

	return result
}

func result_fromEffectiveOrStore(e *LifecyclePolicyEngine, ctx context.Context, input PolicyInput) TrustLevel {
	if userDecision := e.userTrust.Lookup(TrustScopePackage, input.PublisherID, input.KeyID, input.PackageHash, "", ""); userDecision != nil {
		return userDecision.GrantedLevel
	}
	if identity, err := e.store.Get(ctx, input.PublisherID); err == nil {
		return identity.TrustLevel
	}
	return TrustLevelUnknown
}

type QuarantineEntry struct {
	ExtensionID   string
	Version       string
	PackageHash   string
	PublisherID   string
	Reason        string
	QuarantinedAt time.Time
	Severity      string
}

type QuarantineManager struct {
	entries map[string]QuarantineEntry
}

func NewQuarantineManager() *QuarantineManager {
	return &QuarantineManager{entries: make(map[string]QuarantineEntry)}
}

func (q *QuarantineManager) Quarantine(entry QuarantineEntry) {
	if entry.QuarantinedAt.IsZero() {
		entry.QuarantinedAt = time.Now().UTC()
	}
	q.entries[entry.ExtensionID] = entry
}

func (q *QuarantineManager) Release(extensionID string) bool {
	if _, ok := q.entries[extensionID]; !ok {
		return false
	}
	delete(q.entries, extensionID)
	return true
}

func (q *QuarantineManager) Get(extensionID string) *QuarantineEntry {
	if entry, ok := q.entries[extensionID]; ok {
		e := entry
		return &e
	}
	return nil
}

func (q *QuarantineManager) List() []QuarantineEntry {
	result := make([]QuarantineEntry, 0, len(q.entries))
	for _, entry := range q.entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].QuarantinedAt.Before(result[j].QuarantinedAt)
	})
	return result
}
