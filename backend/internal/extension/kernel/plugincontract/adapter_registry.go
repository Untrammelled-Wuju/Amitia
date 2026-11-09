package plugincontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrAdapterInvalid  = errors.New("plugincontract: invalid adapter")
	ErrAdapterExists   = errors.New("plugincontract: adapter already registered")
	ErrAdapterNotFound = errors.New("plugincontract: adapter not found")
	ErrAdapterTrust    = errors.New("plugincontract: adapter trust insufficient")
	ErrAdapterExternal = errors.New("plugincontract: external adapter registration disabled")
	ErrAdapterVersion  = errors.New("plugincontract: adapter contract version mismatch")
)

type TrustLevel string

const (
	TrustUntrusted   TrustLevel = "untrusted"
	TrustUserTrusted TrustLevel = "user_trusted"
	TrustVerified    TrustLevel = "verified"
	TrustTrusted     TrustLevel = "trusted"
	TrustOfficial    TrustLevel = "official"
	TrustSystem      TrustLevel = "system"
)

type AdapterDescriptor struct {
	ID                       string          `json:"adapterId"`
	ContractVersion          int             `json:"contractVersion"`
	SupportedKinds           []string        `json:"supportedKinds"`
	ContributionSchema       json.RawMessage `json:"contributionSchema"`
	Operations               []string        `json:"operations,omitempty"`
	RequiredHostCapabilities []string        `json:"requiredHostCapabilities,omitempty"`
	PermissionRequirements   []string        `json:"permissionRequirements,omitempty"`
	Platforms                []string        `json:"platforms,omitempty"`
	Trust                    TrustLevel      `json:"trust"`
	External                 bool            `json:"external,omitempty"`
}

type AdapterRegistrationPolicy struct {
	MinTrust      TrustLevel
	AllowExternal bool
}

type AdapterSnapshot struct {
	Generation uint64              `json:"generation"`
	Adapters   []AdapterDescriptor `json:"adapters"`
}

type AdapterRegistry struct {
	mu         sync.RWMutex
	policy     AdapterRegistrationPolicy
	adapters   map[string]map[int]AdapterDescriptor
	generation uint64
}

func DefaultAdapterRegistrationPolicy() AdapterRegistrationPolicy {
	return AdapterRegistrationPolicy{MinTrust: TrustOfficial}
}

func NewAdapterRegistry(policy AdapterRegistrationPolicy) *AdapterRegistry {
	if policy.MinTrust == "" {
		policy.MinTrust = TrustOfficial
	}
	return &AdapterRegistry{
		policy:   policy,
		adapters: make(map[string]map[int]AdapterDescriptor),
	}
}

func (r *AdapterRegistry) Register(descriptor AdapterDescriptor) error {
	descriptor = normalizeAdapterDescriptor(descriptor)
	if err := descriptor.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if descriptor.Trust.Rank() < r.policy.MinTrust.Rank() {
		return fmt.Errorf("%w: got %s, need %s", ErrAdapterTrust, descriptor.Trust, r.policy.MinTrust)
	}
	if descriptor.External && !r.policy.AllowExternal {
		return fmt.Errorf("%w: %s", ErrAdapterExternal, descriptor.ID)
	}
	versions := r.adapters[descriptor.ID]
	if versions == nil {
		versions = make(map[int]AdapterDescriptor)
		r.adapters[descriptor.ID] = versions
	}
	if _, exists := versions[descriptor.ContractVersion]; exists {
		return fmt.Errorf("%w: %s v%d", ErrAdapterExists, descriptor.ID, descriptor.ContractVersion)
	}
	versions[descriptor.ContractVersion] = descriptor
	r.generation++
	return nil
}

func (r *AdapterRegistry) Unregister(adapterID string, contractVersion int) error {
	adapterID = strings.TrimSpace(adapterID)
	r.mu.Lock()
	defer r.mu.Unlock()
	versions, ok := r.adapters[adapterID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrAdapterNotFound, adapterID)
	}
	if _, exists := versions[contractVersion]; !exists {
		return fmt.Errorf("%w: %s v%d", ErrAdapterNotFound, adapterID, contractVersion)
	}
	delete(versions, contractVersion)
	if len(versions) == 0 {
		delete(r.adapters, adapterID)
	}
	r.generation++
	return nil
}

func (r *AdapterRegistry) Get(adapterID string, contractVersion int) (AdapterDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions := r.adapters[strings.TrimSpace(adapterID)]
	if versions == nil {
		return AdapterDescriptor{}, false
	}
	descriptor, ok := versions[contractVersion]
	return cloneAdapterDescriptor(descriptor), ok
}

func (r *AdapterRegistry) Latest(adapterID string) (AdapterDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions := r.adapters[strings.TrimSpace(adapterID)]
	if len(versions) == 0 {
		return AdapterDescriptor{}, false
	}
	latest := 0
	for version := range versions {
		if version > latest {
			latest = version
		}
	}
	return cloneAdapterDescriptor(versions[latest]), true
}

func (r *AdapterRegistry) Snapshot() AdapterSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot := AdapterSnapshot{Generation: r.generation}
	for _, versions := range r.adapters {
		for _, descriptor := range versions {
			snapshot.Adapters = append(snapshot.Adapters, cloneAdapterDescriptor(descriptor))
		}
	}
	sort.Slice(snapshot.Adapters, func(i, j int) bool {
		if snapshot.Adapters[i].ID != snapshot.Adapters[j].ID {
			return snapshot.Adapters[i].ID < snapshot.Adapters[j].ID
		}
		return snapshot.Adapters[i].ContractVersion < snapshot.Adapters[j].ContractVersion
	})
	return snapshot
}

func (d AdapterDescriptor) Validate() error {
	if !adapterIDPattern.MatchString(strings.TrimSpace(d.ID)) {
		return fmt.Errorf("%w: invalid id", ErrAdapterInvalid)
	}
	if d.ContractVersion <= 0 {
		return fmt.Errorf("%w: contract version must be positive", ErrAdapterInvalid)
	}
	if len(d.SupportedKinds) == 0 {
		return fmt.Errorf("%w: supported kinds are required", ErrAdapterInvalid)
	}
	for index, kind := range d.SupportedKinds {
		if !kindPattern.MatchString(kind) {
			return fmt.Errorf("%w: invalid kind at index %d", ErrAdapterInvalid, index)
		}
	}
	for index, operation := range d.Operations {
		if !kindPattern.MatchString(operation) {
			return fmt.Errorf("%w: invalid operation at index %d", ErrAdapterInvalid, index)
		}
	}
	if len(d.ContributionSchema) == 0 {
		return fmt.Errorf("%w: contribution schema is required", ErrAdapterInvalid)
	}
	decoder := json.NewDecoder(bytes.NewReader(d.ContributionSchema))
	decoder.UseNumber()
	var schema map[string]any
	if err := decoder.Decode(&schema); err != nil {
		return fmt.Errorf("%w: contribution schema is not a JSON object", ErrAdapterInvalid)
	}
	if !d.Trust.Valid() {
		return fmt.Errorf("%w: invalid trust level", ErrAdapterInvalid)
	}
	return nil
}

func (t TrustLevel) Valid() bool {
	switch t {
	case TrustUntrusted, TrustUserTrusted, TrustVerified, TrustTrusted, TrustOfficial, TrustSystem:
		return true
	default:
		return false
	}
}

func (t TrustLevel) Rank() int {
	switch t {
	case TrustUntrusted:
		return 0
	case TrustUserTrusted:
		return 1
	case TrustVerified:
		return 2
	case TrustTrusted:
		return 3
	case TrustOfficial:
		return 4
	case TrustSystem:
		return 5
	default:
		return -1
	}
}

func normalizeAdapterDescriptor(descriptor AdapterDescriptor) AdapterDescriptor {
	descriptor.ID = strings.TrimSpace(descriptor.ID)
	descriptor.Trust = TrustLevel(strings.TrimSpace(string(descriptor.Trust)))
	descriptor.Platforms = normalizeStrings(descriptor.Platforms)
	descriptor.Operations = normalizeStrings(descriptor.Operations)
	descriptor.SupportedKinds = normalizeStrings(descriptor.SupportedKinds)
	descriptor.RequiredHostCapabilities = normalizeStrings(descriptor.RequiredHostCapabilities)
	descriptor.PermissionRequirements = normalizeStrings(descriptor.PermissionRequirements)
	return descriptor
}

func cloneAdapterDescriptor(descriptor AdapterDescriptor) AdapterDescriptor {
	descriptor.SupportedKinds = append([]string(nil), descriptor.SupportedKinds...)
	descriptor.ContributionSchema = append(json.RawMessage(nil), descriptor.ContributionSchema...)
	descriptor.RequiredHostCapabilities = append([]string(nil), descriptor.RequiredHostCapabilities...)
	descriptor.PermissionRequirements = append([]string(nil), descriptor.PermissionRequirements...)
	descriptor.Platforms = append([]string(nil), descriptor.Platforms...)
	descriptor.Operations = append([]string(nil), descriptor.Operations...)
	return descriptor
}

func normalizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
