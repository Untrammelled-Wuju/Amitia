package plugincontract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type DescriptorSource string

const (
	DescriptorSourceCore               DescriptorSource = "builtin.core"
	DescriptorSourceManifestCapability DescriptorSource = "manifest.providedCapability"
	DescriptorSourceManifestSkill      DescriptorSource = "manifest.agentSkill"
	DescriptorSourceManifestUIProvider DescriptorSource = "manifest.uiProvider"
)

func (s DescriptorSource) IsValid() bool {
	switch s {
	case DescriptorSourceCore, DescriptorSourceManifestCapability, DescriptorSourceManifestSkill, DescriptorSourceManifestUIProvider:
		return true
	}
	return false
}

type DescriptorEntry struct {
	Source       DescriptorSource `json:"source"`
	ProviderID   string           `json:"providerId"`
	CapabilityID string           `json:"capabilityId"`
	ServiceID    string           `json:"serviceId"`
	Version      string           `json:"version"`
	ContractHash string           `json:"contractHash,omitempty"`
	Descriptor   Descriptor       `json:"descriptor"`
	InputSchema  json.RawMessage  `json:"inputSchema,omitempty"`
	OutputSchema json.RawMessage  `json:"outputSchema,omitempty"`
}

type DescriptorCatalog struct {
	Entries []DescriptorEntry          `json:"entries"`
	Schemas map[string]json.RawMessage `json:"schemas"`
}

func (c DescriptorCatalog) Len() int {
	return len(c.Entries)
}

func (c DescriptorCatalog) FindByServiceID(serviceID string) (DescriptorEntry, bool) {
	target := strings.TrimSpace(serviceID)
	for _, entry := range c.Entries {
		if entry.ServiceID == target {
			return entry, true
		}
	}
	return DescriptorEntry{}, false
}

func (c DescriptorCatalog) FindByCapabilityID(capabilityID string) (DescriptorEntry, bool) {
	target := strings.TrimSpace(capabilityID)
	for _, entry := range c.Entries {
		if entry.CapabilityID == target {
			return entry, true
		}
	}
	return DescriptorEntry{}, false
}

func (c DescriptorCatalog) CountBySource() map[DescriptorSource]int {
	counts := make(map[DescriptorSource]int, 4)
	for _, entry := range c.Entries {
		counts[entry.Source]++
	}
	return counts
}

func (c DescriptorCatalog) Listing() []Descriptor {
	out := make([]Descriptor, 0, len(c.Entries))
	for _, entry := range c.Entries {
		out = append(out, entry.Descriptor)
	}
	sort.Slice(out, func(left, right int) bool { return out[left].ID < out[right].ID })
	return out
}

func (c DescriptorCatalog) Resolve(ref SchemaRef) ([]byte, error) {
	return resolveSchemaRef(c.Schemas, ref)
}

func (c DescriptorCatalog) Snapshot() ([]byte, error) {
	canonical := DescriptorCatalog{
		Entries: make([]DescriptorEntry, len(c.Entries)),
		Schemas: make(map[string]json.RawMessage, len(c.Schemas)),
	}
	copy(canonical.Entries, c.Entries)
	sort.Slice(canonical.Entries, func(left, right int) bool {
		return canonical.Entries[left].CapabilityID < canonical.Entries[right].CapabilityID
	})
	for hash, schema := range c.Schemas {
		canonical.Schemas[hash] = schema
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: encode descriptor catalog: %w", err)
	}
	return encoded, nil
}

func (c DescriptorCatalog) Validate() error {
	seen := make(map[string]bool, len(c.Entries))
	for _, entry := range c.Entries {
		if !entry.Source.IsValid() {
			return fmt.Errorf("plugincontract: unknown descriptor source %q for %s", entry.Source, entry.CapabilityID)
		}
		if err := ValidateCapabilityIdentity(entry.CapabilityID); err != nil {
			return fmt.Errorf("plugincontract: catalog capability id %q: %w", entry.CapabilityID, err)
		}
		if err := ValidateProviderID(entry.ProviderID); err != nil {
			return fmt.Errorf("plugincontract: catalog provider id %q: %w", entry.ProviderID, err)
		}
		if err := ValidateServiceID(entry.ServiceID); err != nil {
			return fmt.Errorf("plugincontract: catalog service id %q: %w", entry.ServiceID, err)
		}
		if entry.Descriptor.ID != entry.CapabilityID {
			return fmt.Errorf("plugincontract: descriptor id %q does not match capability %q", entry.Descriptor.ID, entry.CapabilityID)
		}
		if entry.ContractHash != "" {
			if err := ValidateContractHash(entry.ContractHash); err != nil {
				return fmt.Errorf("plugincontract: catalog contract hash for %s: %w", entry.CapabilityID, err)
			}
		}
		if seen[entry.CapabilityID] {
			return fmt.Errorf("plugincontract: duplicated capability %q in catalog", entry.CapabilityID)
		}
		seen[entry.CapabilityID] = true
		if entry.Descriptor.InputRef != nil {
			if _, err := resolveSchemaRef(c.Schemas, *entry.Descriptor.InputRef); err != nil {
				return fmt.Errorf("plugincontract: catalog input schema for %s: %w", entry.CapabilityID, err)
			}
		}
		if entry.Descriptor.OutputRef != nil {
			if _, err := resolveSchemaRef(c.Schemas, *entry.Descriptor.OutputRef); err != nil {
				return fmt.Errorf("plugincontract: catalog output schema for %s: %w", entry.CapabilityID, err)
			}
		}
	}
	return nil
}

func (c CoreDescriptorCatalog) ToCatalog() DescriptorCatalog {
	catalog := DescriptorCatalog{
		Entries: make([]DescriptorEntry, 0, len(c.Entries)),
		Schemas: make(map[string]json.RawMessage, len(c.Schemas)),
	}
	for _, entry := range c.Entries {
		catalog.Entries = append(catalog.Entries, DescriptorEntry{
			Source:       DescriptorSourceCore,
			ProviderID:   entry.ProviderID,
			CapabilityID: entry.CapabilityID,
			ServiceID:    entry.ServiceID,
			Version:      entry.Version,
			ContractHash: entry.ContractHash,
			Descriptor:   entry.Descriptor,
			InputSchema:  entry.InputSchema,
			OutputSchema: entry.OutputSchema,
		})
	}
	for hash, schema := range c.Schemas {
		catalog.Schemas[hash] = schema
	}
	return catalog
}
