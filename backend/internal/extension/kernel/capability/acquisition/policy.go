package acquisition

import "strings"

// sensitivePermissions enumerates the permission categories that always require
// explicit user approval before a candidate may be acquired.
var sensitivePermissions = []string{
	"filesystem.write",
	"shell",
	"process",
	"browser.automation",
	"account.authorization",
	"external.github",
	"external.gmail",
	"background.daemon",
	"network.listen",
	"device.control",
	"camera",
	"microphone",
	"secret.access",
}

// PolicyEngine encapsulates the rules used to decide whether a candidate may
// be installed automatically, requires explicit approval, or must be denied.
type PolicyEngine struct {
	autoInstallMaxRisk string
	blockedPublishers  map[string]bool
	trustedRegistries  map[string]bool
}

// NewPolicyEngine returns a PolicyEngine populated with sensible defaults.
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		autoInstallMaxRisk: "low",
		blockedPublishers:  make(map[string]bool),
		trustedRegistries:  make(map[string]bool),
	}
}

// Evaluate returns the policy decision for a single candidate.
//
// Decision flow:
//  1. /Blocked publisher/ or explicit blocked trust level  -> ActionDeny
//  2. /Signature anomaly/ (claimed but failed)             -> ActionDeny
//  3. /Unrepresentable permissions/ (empty DSL)           -> ActionDeny
//  4. /Installed, only enable/                            -> ActionAllowAuto
//  5. /Built-in/                                          -> ActionAllowAuto
//  6. /Signed & trusted publisher/                        -> ActionAllowAuto
//  7. /Trusted registry + low-risk MCP/                   -> ActionAllowAuto
//  8. Any sensitive permission scope                      -> ActionRequireApproval
//  9. /Unverified publisher/                              -> ActionRequireApproval
//  10. Otherwise                                          -> ActionRequireApproval
func (e *PolicyEngine) Evaluate(candidate CapabilityCandidate, request AcquisitionRequest) PolicyDecision {
	deny := e.evaluateDeny(candidate, request)
	if deny != nil {
		return *deny
	}

	auto := e.evaluateAuto(candidate, request)
	if auto != nil {
		return *auto
	}

	return e.requireApproval(candidate, request)
}

// evaluateDeny handles hard-deny rules. Returns nil if the candidate does not
// match any deny rule.
func (e *PolicyEngine) evaluateDeny(candidate CapabilityCandidate, request AcquisitionRequest) *PolicyDecision {
	if e.isBlockedPublisher(candidate.Source.Publisher) {
		return &PolicyDecision{
			Action:  ActionDeny,
			Reasons: []string{"candidate publisher is blocked"},
		}
	}

	if candidate.Trust.Level == TrustBlocked {
		return &PolicyDecision{
			Action:  ActionDeny,
			Reasons: []string{"candidate trust level is blocked"},
		}
	}

	if candidate.Trust.SignatureVerified && !candidate.Trust.PublisherVerified {
		return &PolicyDecision{
			Action:  ActionDeny,
			Reasons: []string{"signature verified but publisher not verified"},
		}
	}

	if !e.permissionsExpressible(candidate.Permissions) {
		return &PolicyDecision{
			Action:  ActionDeny,
			Reasons: []string{"candidate permissions cannot be expressed in policy DSL"},
		}
	}

	return nil
}

// evaluateAuto handles automatic-approval rules. Returns nil when the
// candidate does not qualify for automatic installation.
func (e *PolicyEngine) evaluateAuto(candidate CapabilityCandidate, request AcquisitionRequest) *PolicyDecision {
	if candidate.Install.Method == InstallEnableExisting {
		return &PolicyDecision{
			Action:  ActionAllowAuto,
			Reasons: []string{"already installed, only enable required"},
		}
	}

	if candidate.Kind == CandidateBuiltin {
		return &PolicyDecision{
			Action:  ActionAllowAuto,
			Reasons: []string{"built-in capability"},
		}
	}

	if candidate.Trust.SignatureVerified && e.isTrustedPublisher(candidate.Source.Publisher) {
		return &PolicyDecision{
			Action:  ActionAllowAuto,
			Reasons: []string{"signed and trusted publisher"},
		}
	}

	if candidate.Kind == CandidateMCP && e.isTrustedRegistry(candidate.Source.Registry) && e.riskLevelValue(candidate) <= e.maxAutoRisk() {
		return &PolicyDecision{
			Action:  ActionAllowAuto,
			Reasons: []string{"trusted registry low-risk MCP"},
		}
	}

	return nil
}

// requireApproval builds an ActionRequireApproval decision that enumerates the
// reasons approval is needed.
func (e *PolicyEngine) requireApproval(candidate CapabilityCandidate, request AcquisitionRequest) PolicyDecision {
	var reasons []string
	var scopes []string

	for _, p := range candidate.Permissions {
		if isSensitivePermission(p) {
			reasons = append(reasons, "sensitive permission: "+p)
			scopes = append(scopes, p)
		}
	}

	if candidate.Trust.PublisherVerified == false && candidate.Source.Publisher != "" {
		reasons = append(reasons, "unverified publisher: "+candidate.Source.Publisher)
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "default approval policy requires user confirmation")
	}

	return PolicyDecision{
		Action:  ActionRequireApproval,
		Reasons: reasons,
		RequiredApprovals: []ApprovalRequirement{
			{
				Reason:    strings.Join(reasons, "; "),
				Scopes:    scopes,
				RiskLevel: e.riskLevel(candidate),
			},
		},
		RequiredPermissions: append([]string(nil), candidate.Permissions...),
	}
}

// isBlockedPublisher reports whether the publisher is blocked. An empty
// publisher is treated as unknown (not blocked).
func (e *PolicyEngine) isBlockedPublisher(publisher string) bool {
	if publisher == "" {
		return false
	}
	return e.blockedPublishers[publisher]
}

// isTrustedPublisher reports whether the publisher is trusted. An empty
// publisher is treated as unknown (not trusted).
func (e *PolicyEngine) isTrustedPublisher(publisher string) bool {
	if publisher == "" {
		return false
	}
	return e.trustedRegistries[publisher]
}

// isTrustedRegistry reports whether the registry is present in the trusted
// registry set.
func (e *PolicyEngine) isTrustedRegistry(registry string) bool {
	if registry == "" {
		return false
	}
	return e.trustedRegistries[registry]
}

// riskLevel returns a string risk level for the candidate based on its
// permission footprint.
func (e *PolicyEngine) riskLevel(candidate CapabilityCandidate) string {
	return riskLevelString(e.riskLevelValue(candidate))
}

// riskLevelValue returns a numeric risk value for the candidate based on its
// permission footprint. Higher values indicate higher risk.
func (e *PolicyEngine) riskLevelValue(candidate CapabilityCandidate) int {
	if anySensitive(candidate.Permissions) {
		return 3
	}
	if len(candidate.Permissions) > 0 {
		return 2
	}
	return 1
}

func riskLevelString(level int) string {
	switch {
	case level >= 3:
		return "high"
	case level >= 2:
		return "medium"
	default:
		return "low"
	}
}

// maxAutoRisk returns the maximum risk level that is allowed for automatic
// installation.
func (e *PolicyEngine) maxAutoRisk() int {
	switch e.autoInstallMaxRisk {
	case "none":
		return 0
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	}
	return 1
}

// permissionsExpressible reports whether every permission string is non-empty.
// Candidates with malformed/unrepresentable permissions are denied.
func (e *PolicyEngine) permissionsExpressible(perms []string) bool {
	for _, p := range perms {
		if strings.TrimSpace(p) == "" {
			return false
		}
	}
	return true
}

// isSensitivePermission reports whether the granted permission requires user
// approval before use.
func isSensitivePermission(perm string) bool {
	for _, s := range sensitivePermissions {
		if strings.HasPrefix(perm, s) {
			return true
		}
	}
	return false
}

// anySensitive returns true when any permission is sensitive.
func anySensitive(perms []string) bool {
	for _, p := range perms {
		if isSensitivePermission(p) {
			return true
		}
	}
	return false
}
