package plugincontract

import (
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var (
	ErrRoutePolicyInvalid   = errors.New("plugincontract: route policy invalid")
	ErrRouteModeUnsatisfied = errors.New("plugincontract: route mode unsatisfied")
	ErrRoutePinConflict     = errors.New("plugincontract: route pin conflict")
)

type RouteMode string

const (
	RouteModeAuto       RouteMode = "auto"
	RouteModeCoreOnly   RouteMode = "core_only"
	RouteModePluginOnly RouteMode = "plugin_only"
)

type RoutePolicy struct {
	Mode             RouteMode `json:"mode"`
	PinnedProviderID string    `json:"pinnedProviderId,omitempty"`
	RequiredContract string    `json:"requiredContractHash,omitempty"`
	RequiredVersion  string    `json:"requiredVersionRange,omitempty"`
}

type RouteSkip struct {
	ProviderID string `json:"providerId"`
	Reason     string `json:"reason"`
}

type RouteDecision struct {
	Mode       RouteMode         `json:"mode"`
	ServiceID  string            `json:"serviceId"`
	Selected   ServiceDescriptor `json:"selected"`
	Candidates []ServiceDescriptor
	Skipped    []RouteSkip `json:"skipped"`
	Pinned     bool        `json:"pinned"`
	Fallback   bool        `json:"fallback"`
}

type RoutePolicyInvalidError struct {
	Field  string
	Value  string
	Reason string
}

func (e *RoutePolicyInvalidError) Error() string {
	return fmt.Sprintf("plugincontract: route policy field %q value %q: %s", e.Field, e.Value, e.Reason)
}

func (e *RoutePolicyInvalidError) Unwrap() error { return ErrRoutePolicyInvalid }

type RouteModeUnsatisfiedError struct {
	Mode      RouteMode
	ServiceID string
	Reason    string
	Skipped   []RouteSkip
}

func (e *RouteModeUnsatisfiedError) Error() string {
	parts := make([]string, 0, len(e.Skipped))
	for _, skip := range e.Skipped {
		parts = append(parts, skip.ProviderID+"="+skip.Reason)
	}
	return fmt.Sprintf(
		"plugincontract: route mode %q cannot satisfy service %q (%s) skipped: %s",
		e.Mode, e.ServiceID, e.Reason, strings.Join(parts, ", "),
	)
}

func (e *RouteModeUnsatisfiedError) Unwrap() error { return ErrRouteModeUnsatisfied }

type RoutePinConflictError struct {
	Pinned string
	Mode   RouteMode
	Reason string
}

func (e *RoutePinConflictError) Error() string {
	return fmt.Sprintf("plugincontract: pinned provider %q conflicts with route mode %q: %s", e.Pinned, e.Mode, e.Reason)
}

func (e *RoutePinConflictError) Unwrap() error { return ErrRoutePinConflict }

func (p RoutePolicy) validate() (RouteMode, *dependency.VersionRange, error) {
	mode := p.Mode
	if mode == "" {
		mode = RouteModeAuto
	}
	switch mode {
	case RouteModeAuto, RouteModeCoreOnly, RouteModePluginOnly:
	default:
		return "", nil, &RoutePolicyInvalidError{Field: "mode", Value: string(p.Mode), Reason: "want auto, core_only or plugin_only"}
	}
	var versionRange *dependency.VersionRange
	if trimmed := strings.TrimSpace(p.RequiredVersion); trimmed != "" {
		parsed, err := dependency.ParseRange(trimmed)
		if err != nil {
			return "", nil, &RoutePolicyInvalidError{Field: "requiredVersionRange", Value: trimmed, Reason: err.Error()}
		}
		versionRange = parsed
	}
	return mode, versionRange, nil
}

func SelectRoute(candidates ProviderCandidates, policy RoutePolicy) (RouteDecision, error) {
	mode, versionRange, err := policy.validate()
	if err != nil {
		return RouteDecision{}, err
	}
	pinned := strings.TrimSpace(policy.PinnedProviderID)
	requiredContract := strings.TrimSpace(policy.RequiredContract)
	decision := RouteDecision{
		Mode:       mode,
		ServiceID:  candidates.ServiceID,
		Candidates: make([]ServiceDescriptor, 0, len(candidates.Candidates)),
		Skipped:    make([]RouteSkip, 0),
		Pinned:     pinned != "",
	}
	for _, candidate := range candidates.Candidates {
		provider := normalizeServiceDescriptor(candidate)
		reason := ""
		switch {
		case mode == RouteModeCoreOnly && !isCoreProvider(provider.ProviderID):
			reason = "mode_excluded"
		case mode == RouteModePluginOnly && isCoreProvider(provider.ProviderID):
			reason = "mode_excluded"
		case requiredContract != "" && provider.ContractHash != requiredContract:
			reason = "contract_mismatch"
		case versionRange != nil:
			version, parseErr := domain.ParseVersion(provider.Version)
			if parseErr != nil {
				reason = "version_invalid"
			} else if !versionRange.Satisfies(version) {
				reason = "version_mismatch"
			}
		}
		if reason != "" {
			decision.Skipped = append(decision.Skipped, RouteSkip{ProviderID: provider.ProviderID, Reason: reason})
			continue
		}
		decision.Candidates = append(decision.Candidates, provider)
	}

	if pinned != "" {
		selected, found := pickCandidate(decision.Candidates, pinned)
		if !found {
			reason := skipReason(decision.Skipped, pinned)
			if reason == "" {
				reason = "not_a_candidate"
			}
			if reason == "mode_excluded" {
				return RouteDecision{}, &RoutePinConflictError{Pinned: pinned, Mode: mode, Reason: "the pinned provider is excluded by this mode"}
			}
			return RouteDecision{}, &RoutePinConflictError{Pinned: pinned, Mode: mode, Reason: reason}
		}
		decision.Selected = selected
		return decision, nil
	}

	if mode == RouteModeAuto {
		selected, found := pickCandidate(decision.Candidates, candidates.Selected.ProviderID)
		if found {
			decision.Selected = selected
			return decision, nil
		}
		if len(decision.Candidates) == 0 {
			reason := skipReason(decision.Skipped, candidates.Selected.ProviderID)
			if reason == "" {
				reason = "not_a_candidate"
			}
			return RouteDecision{}, &RouteModeUnsatisfiedError{
				Mode:      mode,
				ServiceID: candidates.ServiceID,
				Reason:    "selected provider " + candidates.Selected.ProviderID + " is not eligible: " + reason,
				Skipped:   decision.Skipped,
			}
		}
		decision.Selected = decision.Candidates[0]
		decision.Fallback = true
		return decision, nil
	}

	if len(decision.Candidates) == 0 {
		return RouteDecision{}, &RouteModeUnsatisfiedError{
			Mode:      mode,
			ServiceID: candidates.ServiceID,
			Reason:    "no candidate survives the mode and compatibility gates",
			Skipped:   decision.Skipped,
		}
	}
	decision.Selected = decision.Candidates[0]
	return decision, nil
}

func isCoreProvider(providerID string) bool {
	return strings.HasPrefix(providerID, CoreProviderPluginID+"/")
}

func pickCandidate(candidates []ServiceDescriptor, providerID string) (ServiceDescriptor, bool) {
	for _, candidate := range candidates {
		if candidate.ProviderID == providerID {
			return candidate, true
		}
	}
	return ServiceDescriptor{}, false
}

func skipReason(skipped []RouteSkip, providerID string) string {
	for _, skip := range skipped {
		if skip.ProviderID == providerID {
			return skip.Reason
		}
	}
	return ""
}
