package plugincontract

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrScopeContractInvalid     = errors.New("plugincontract: provider scope contract invalid")
	ErrScopeProviderUnavailable = errors.New("plugincontract: scoped provider unavailable")
	ErrScopeNoMatch             = errors.New("plugincontract: no provider matches the scope requirement")
)

type ScopeDimension string

const (
	ScopeDimensionDevice  ScopeDimension = "device"
	ScopeDimensionCore    ScopeDimension = "core"
	ScopeDimensionSpace   ScopeDimension = "space"
	ScopeDimensionRole    ScopeDimension = "role"
	ScopeDimensionSurface ScopeDimension = "surface"
)

var ScopeDimensions = []ScopeDimension{
	ScopeDimensionDevice,
	ScopeDimensionCore,
	ScopeDimensionSpace,
	ScopeDimensionRole,
	ScopeDimensionSurface,
}

type ScopeClaims map[ScopeDimension]string

type ScopeRequirement map[ScopeDimension]string

type ProviderScopeContract struct {
	ProviderID string      `json:"providerId"`
	ServiceID  string      `json:"serviceId"`
	Claims     ScopeClaims `json:"claims"`
}

type ScopeMatch struct {
	ProviderID string         `json:"providerId"`
	Matched    bool           `json:"matched"`
	Dimension  ScopeDimension `json:"dimension,omitempty"`
	Required   string         `json:"required,omitempty"`
	Actual     string         `json:"actual,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

type ScopeContractInvalidError struct {
	Field  string
	Value  string
	Reason string
}

func (e *ScopeContractInvalidError) Error() string {
	return fmt.Sprintf("plugincontract: provider scope field %q value %q: %s", e.Field, e.Value, e.Reason)
}

func (e *ScopeContractInvalidError) Unwrap() error { return ErrScopeContractInvalid }

type ScopeProviderUnavailableError struct {
	ServiceID  string
	Required   ScopeRequirement
	Candidates []string
	Reasons    []string
}

func (e *ScopeProviderUnavailableError) Error() string {
	return fmt.Sprintf(
		"plugincontract: scoped providers for service %q are unavailable (required=%s candidates=%s)",
		e.ServiceID, renderScope(e.Required), strings.Join(e.Candidates, ","),
	)
}

func (e *ScopeProviderUnavailableError) Unwrap() error { return ErrScopeProviderUnavailable }

type ScopeNoMatchError struct {
	ServiceID string
	Required  ScopeRequirement
	Reasons   []string
}

func (e *ScopeNoMatchError) Error() string {
	return fmt.Sprintf(
		"plugincontract: no provider matches service %q scope (required=%s reasons=%s)",
		e.ServiceID, renderScope(e.Required), strings.Join(e.Reasons, "; "),
	)
}

func (e *ScopeNoMatchError) Unwrap() error { return ErrScopeNoMatch }

func validateScopeDimension(dimension ScopeDimension, field string) error {
	for _, known := range ScopeDimensions {
		if dimension == known {
			return nil
		}
	}
	return &ScopeContractInvalidError{Field: field, Value: string(dimension), Reason: "want device, core, space, role or surface"}
}

func (c ProviderScopeContract) validate() error {
	if strings.TrimSpace(c.ProviderID) == "" {
		return &ScopeContractInvalidError{Field: "providerId", Value: "", Reason: "must not be empty"}
	}
	if strings.TrimSpace(c.ServiceID) == "" {
		return &ScopeContractInvalidError{Field: "serviceId", Value: "", Reason: "must not be empty"}
	}
	for dimension, value := range c.Claims {
		if err := validateScopeDimension(dimension, "claims.dimension"); err != nil {
			return err
		}
		if strings.TrimSpace(value) == "" {
			return &ScopeContractInvalidError{Field: "claims." + string(dimension), Value: "", Reason: "must not be empty"}
		}
	}
	if _, core := c.Claims[ScopeDimensionCore]; core {
		if _, device := c.Claims[ScopeDimensionDevice]; device {
			return &ScopeContractInvalidError{Field: "claims", Value: "core+device", Reason: "a core provider is process local and cannot claim a device"}
		}
	}
	return nil
}

func (r ScopeRequirement) validate() error {
	for dimension, value := range r {
		if err := validateScopeDimension(dimension, "requirement.dimension"); err != nil {
			return err
		}
		if strings.TrimSpace(value) == "" {
			return &ScopeContractInvalidError{Field: "requirement." + string(dimension), Value: "", Reason: "must not be empty"}
		}
	}
	return nil
}

func (c ProviderScopeContract) Matches(requirement ScopeRequirement) (ScopeMatch, error) {
	if err := c.validate(); err != nil {
		return ScopeMatch{}, err
	}
	if err := requirement.validate(); err != nil {
		return ScopeMatch{}, err
	}
	match := ScopeMatch{ProviderID: c.ProviderID, Matched: true}
	if len(requirement) == 0 {
		return match, nil
	}
	if required, ok := requirement[ScopeDimensionDevice]; ok {
		if _, core := c.Claims[ScopeDimensionCore]; core {
			match.Matched = false
			match.Dimension = ScopeDimensionDevice
			match.Required = required
			match.Reason = "core_cannot_serve_device"
			return match, nil
		}
	}
	for _, dimension := range ScopeDimensions {
		required, ok := requirement[dimension]
		if !ok {
			continue
		}
		actual, declared := c.Claims[dimension]
		if !declared {
			match.Matched = false
			match.Dimension = dimension
			match.Required = required
			match.Reason = "missing_claim"
			return match, nil
		}
		if actual != required {
			match.Matched = false
			match.Dimension = dimension
			match.Required = required
			match.Actual = actual
			match.Reason = "claim_mismatch"
			return match, nil
		}
	}
	return match, nil
}

type ScopeProviderAvailability func(providerID string) bool

func SelectScopedProvider(
	contracts []ProviderScopeContract,
	requirement ScopeRequirement,
	availability ScopeProviderAvailability,
) (ProviderScopeContract, ScopeMatch, error) {
	if len(contracts) == 0 {
		return ProviderScopeContract{}, ScopeMatch{}, &ScopeNoMatchError{Required: requirement, Reasons: []string{"no provider contracts supplied"}}
	}
	serviceID := ""
	matches := make([]ScopeMatch, 0, len(contracts))
	candidates := make([]string, 0, len(contracts))
	reasons := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		match, err := contract.Matches(requirement)
		if err != nil {
			return ProviderScopeContract{}, ScopeMatch{}, err
		}
		if serviceID == "" {
			serviceID = strings.TrimSpace(contract.ServiceID)
		}
		matches = append(matches, match)
		if !match.Matched {
			reasons = append(reasons, fmt.Sprintf("%s=%s", contract.ProviderID, match.Reason))
			continue
		}
		if availability != nil && !availability(contract.ProviderID) {
			candidates = append(candidates, contract.ProviderID)
			reasons = append(reasons, fmt.Sprintf("%s=unavailable", contract.ProviderID))
			continue
		}
		return contract, match, nil
	}
	if len(candidates) > 0 {
		return ProviderScopeContract{}, ScopeMatch{}, &ScopeProviderUnavailableError{
			ServiceID:  serviceID,
			Required:   requirement,
			Candidates: candidates,
			Reasons:    reasons,
		}
	}
	return ProviderScopeContract{}, ScopeMatch{}, &ScopeNoMatchError{ServiceID: serviceID, Required: requirement, Reasons: reasons}
}

func renderScope(scope ScopeRequirement) string {
	if len(scope) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(scope))
	for _, dimension := range ScopeDimensions {
		value, ok := scope[dimension]
		if !ok {
			continue
		}
		parts = append(parts, string(dimension)+"="+value)
	}
	return strings.Join(parts, ",")
}
