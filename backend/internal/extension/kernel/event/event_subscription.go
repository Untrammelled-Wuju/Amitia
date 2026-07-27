package event

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type ScopeRule struct {
	RequiredScope string
	CharacterBinding bool
	ConversationBinding bool
}

type DependencyRequirement struct {
	DependencyID string
	VersionRange string
	Optional     bool
}

type RuntimeBinding struct {
	RuntimeType  string
	Entry       string
	Timeout     time.Duration
	MaxInFlight int
}

type SubscriptionDeliveryPolicy struct {
	Timeout    time.Duration
	MaxInFlight int
	Ordering   EventOrderingRequirement
}

type EventSubscriptionDefinition struct {
	ContributionID         string
	ExtensionID            string
	ModuleID               string
	EventTypeID            EventTypeID
	EventVersionRange      string
	Entry                  string
	Filter                 EventFilterDefinition
	Projection             EventProjectionRequest
	DeliveryPolicy         SubscriptionDeliveryPolicy
	RetryPolicy             RetryPolicy
	OrderingRequirement    EventOrderingRequirement
	Timeout               time.Duration
	MaxInFlight            int
	PermissionRequirements []PermissionRequirement
	ScopeRule             ScopeRule
	DependencyRequirements []DependencyRequirement
	RuntimeBinding         RuntimeBinding
	DefinitionHash        string
	Generation            int64
	Enabled               bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (d EventSubscriptionDefinition) Hash() string {
	h := sha256.New()
	fmt.Fprintf(h, "contrib=%s\next=%s\nmod=%s\ntype=%s\nver=%s\nentry=%s\n",
		d.ContributionID, d.ExtensionID, d.ModuleID, d.EventTypeID, d.EventVersionRange, d.Entry)
	fb, _ := json.Marshal(d.Filter)
	h.Write(fb)
	pb, _ := json.Marshal(d.Projection)
	h.Write(pb)
	db, _ := json.Marshal(d.DeliveryPolicy)
	h.Write(db)
	rb, _ := json.Marshal(d.RetryPolicy)
	h.Write(rb)
	fmt.Fprintf(h, "ordering=%s\ntimeout=%v\nmax_inflight=%d\n",
		d.OrderingRequirement, d.Timeout, d.MaxInFlight)
	prb, _ := json.Marshal(d.PermissionRequirements)
	h.Write(prb)
	sb, _ := json.Marshal(d.ScopeRule)
	h.Write(sb)
	depb, _ := json.Marshal(d.DependencyRequirements)
	h.Write(depb)
	rtb, _ := json.Marshal(d.RuntimeBinding)
	h.Write(rtb)
	return hex.EncodeToString(h.Sum(nil))
}

func (d *EventSubscriptionDefinition) Validate() error {
	if d.ContributionID == "" {
		return errors.New("event: contribution id required")
	}
	if d.ExtensionID == "" {
		return errors.New("event: extension id required")
	}
	if d.EventTypeID == "" {
		return errors.New("event: event type id required")
	}
	if d.Entry == "" {
		return errors.New("event: entry required")
	}
	if d.Timeout == 0 {
		d.Timeout = 5 * time.Second
	}
	if d.MaxInFlight == 0 {
		d.MaxInFlight = 4
	}
	if d.RetryPolicy.MaxAttempts == 0 {
		d.RetryPolicy = DefaultRetryPolicy()
	}
	if d.DeliveryPolicy.Timeout == 0 {
		d.DeliveryPolicy.Timeout = d.Timeout
	}
	if d.DeliveryPolicy.MaxInFlight == 0 {
		d.DeliveryPolicy.MaxInFlight = d.MaxInFlight
	}
	if d.RuntimeBinding.Timeout == 0 {
		d.RuntimeBinding.Timeout = d.Timeout
	}
	if d.RuntimeBinding.MaxInFlight == 0 {
		d.RuntimeBinding.MaxInFlight = d.MaxInFlight
	}
	d.DefinitionHash = d.Hash()
	return nil
}

func (d EventSubscriptionDefinition) MatchesVersion(version int) bool {
	if d.EventVersionRange == "" {
		return true
	}
	if d.EventVersionRange == "*" {
		return true
	}
	if d.EventVersionRange[0] == '^' {
		var major int
		if _, err := fmt.Sscanf(d.EventVersionRange[1:], "%d", &major); err != nil {
			return true
		}
		return version >= major && version < major+1
	}
	var v int
	if _, err := fmt.Sscanf(d.EventVersionRange, "%d", &v); err == nil {
		return version == v
	}
	return true
}

type ResolvedSubscription struct {
	Definition      EventSubscriptionDefinition
	CompiledFilter  *CompiledFilter
	Projector       *PayloadProjector
	Effective       SubscriptionEffectiveState
}

type SubscriptionEffectiveState struct {
	Enabled          bool
	Generation       int64
	PermissionGranted bool
	ScopeValid       bool
	DependenciesReady bool
	RuntimeAvailable  bool
	CircuitState     CircuitState
	Reason           string
}

func (s SubscriptionEffectiveState) IsActive() bool {
	return s.Enabled && s.PermissionGranted && s.ScopeValid && s.DependenciesReady && s.RuntimeAvailable && s.CircuitState != CircuitOpen
}

func (s SubscriptionEffectiveState) DenyReason() string {
	if !s.Enabled {
		return "subscription_disabled"
	}
	if !s.PermissionGranted {
		return "permission_denied"
	}
	if !s.ScopeValid {
		return "scope_denied"
	}
	if !s.DependenciesReady {
		return "dependency_missing"
	}
	if !s.RuntimeAvailable {
		return "runtime_unavailable"
	}
	if s.CircuitState == CircuitOpen {
		return "circuit_open"
	}
	return s.Reason
}
