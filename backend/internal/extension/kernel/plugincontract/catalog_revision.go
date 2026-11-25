package plugincontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrCatalogConflict = errors.New("plugincontract: descriptor catalog conflict")

type CatalogConflictKind string

const (
	CatalogConflictDuplicateCapability CatalogConflictKind = "duplicate.capability"
	CatalogConflictDuplicateProvider   CatalogConflictKind = "duplicate.provider"
	CatalogConflictServiceMismatch     CatalogConflictKind = "service.mismatch"
)

type CatalogConflict struct {
	Kind         CatalogConflictKind `json:"kind"`
	CapabilityID string              `json:"capabilityId"`
	ProviderID   string              `json:"providerId,omitempty"`
	ServiceID    string              `json:"serviceId,omitempty"`
	Existing     DescriptorSource    `json:"existingSource"`
	Incoming     DescriptorSource    `json:"incomingSource"`
}

func (e *CatalogConflict) Error() string {
	switch e.Kind {
	case CatalogConflictDuplicateCapability:
		return fmt.Sprintf("plugincontract: capability %q declared twice (%s then %s)", e.CapabilityID, e.Existing, e.Incoming)
	case CatalogConflictDuplicateProvider:
		return fmt.Sprintf("plugincontract: provider %q claimed by capability %q after %s", e.ProviderID, e.CapabilityID, e.Existing)
	case CatalogConflictServiceMismatch:
		return fmt.Sprintf("plugincontract: service %q maps to capability %q but also to %q", e.ServiceID, e.CapabilityID, e.ProviderID)
	default:
		return fmt.Sprintf("plugincontract: catalog conflict %q", e.Kind)
	}
}

func (e *CatalogConflict) Unwrap() error {
	return ErrCatalogConflict
}

func CatalogRevision(catalog DescriptorCatalog) (string, error) {
	snapshot, err := catalog.Snapshot()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(snapshot)
	return hex.EncodeToString(sum[:]), nil
}

func (c DescriptorCatalog) Revision() (string, error) {
	return CatalogRevision(c)
}

func DetectCatalogConflicts(catalog DescriptorCatalog) []*CatalogConflict {
	conflicts := make([]*CatalogConflict, 0)
	byCapability := make(map[string]DescriptorEntry, len(catalog.Entries))
	byProvider := make(map[string]DescriptorEntry, len(catalog.Entries))
	byService := make(map[string]DescriptorEntry, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		capabilityID := strings.TrimSpace(entry.CapabilityID)
		if existing, found := byCapability[capabilityID]; found {
			conflicts = append(conflicts, &CatalogConflict{
				Kind:         CatalogConflictDuplicateCapability,
				CapabilityID: capabilityID,
				ProviderID:   entry.ProviderID,
				Existing:     existing.Source,
				Incoming:     entry.Source,
			})
			continue
		}
		byCapability[capabilityID] = entry

		providerID := strings.TrimSpace(entry.ProviderID)
		if providerID != "" {
			if existing, found := byProvider[providerID]; found && existing.CapabilityID != capabilityID {
				conflicts = append(conflicts, &CatalogConflict{
					Kind:         CatalogConflictDuplicateProvider,
					CapabilityID: capabilityID,
					ProviderID:   providerID,
					Existing:     existing.Source,
					Incoming:     entry.Source,
				})
			} else if !found {
				byProvider[providerID] = entry
			}
		}

		serviceID := strings.TrimSpace(entry.ServiceID)
		if serviceID != "" {
			if existing, found := byService[serviceID]; found && existing.CapabilityID != capabilityID {
				conflicts = append(conflicts, &CatalogConflict{
					Kind:         CatalogConflictServiceMismatch,
					CapabilityID: capabilityID,
					ProviderID:   existing.CapabilityID,
					ServiceID:    serviceID,
					Existing:     existing.Source,
					Incoming:     entry.Source,
				})
			} else if !found {
				byService[serviceID] = entry
			}
		}
	}
	return conflicts
}

func (c DescriptorCatalog) ConflictingEntries() []*CatalogConflict {
	return DetectCatalogConflicts(c)
}

type CatalogEntryChange struct {
	CapabilityID        string   `json:"capabilityId"`
	ProviderID          string   `json:"providerId"`
	Fields              []string `json:"fields"`
	InputSchemaChanged  bool     `json:"inputSchemaChanged"`
	OutputSchemaChanged bool     `json:"outputSchemaChanged"`
}

type CatalogDiff struct {
	PreviousRevision string               `json:"previousRevision"`
	NextRevision     string               `json:"nextRevision"`
	Added            []DescriptorEntry    `json:"added"`
	Removed          []DescriptorEntry    `json:"removed"`
	Changed          []CatalogEntryChange `json:"changed"`
}

func (d CatalogDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

func (d CatalogDiff) Len() int {
	return len(d.Added) + len(d.Removed) + len(d.Changed)
}

func (d CatalogDiff) TouchedCapabilityIDs() []string {
	touched := make([]string, 0, d.Len())
	for _, entry := range d.Added {
		touched = append(touched, entry.CapabilityID)
	}
	for _, entry := range d.Removed {
		touched = append(touched, entry.CapabilityID)
	}
	for _, change := range d.Changed {
		touched = append(touched, change.CapabilityID)
	}
	sort.Strings(touched)
	return touched
}

func DiffCatalogs(previous, next DescriptorCatalog) (CatalogDiff, error) {
	previousRevision, err := CatalogRevision(previous)
	if err != nil {
		return CatalogDiff{}, fmt.Errorf("plugincontract: previous catalog revision: %w", err)
	}
	nextRevision, err := CatalogRevision(next)
	if err != nil {
		return CatalogDiff{}, fmt.Errorf("plugincontract: next catalog revision: %w", err)
	}
	diff := CatalogDiff{
		PreviousRevision: previousRevision,
		NextRevision:     nextRevision,
		Added:            make([]DescriptorEntry, 0),
		Removed:          make([]DescriptorEntry, 0),
		Changed:          make([]CatalogEntryChange, 0),
	}
	previousEntries := indexCatalogEntries(previous)
	nextEntries := indexCatalogEntries(next)
	for capabilityID, entry := range nextEntries {
		if _, found := previousEntries[capabilityID]; !found {
			diff.Added = append(diff.Added, entry)
		}
	}
	for capabilityID, entry := range previousEntries {
		if _, found := nextEntries[capabilityID]; !found {
			diff.Removed = append(diff.Removed, entry)
		}
	}
	for capabilityID, entry := range nextEntries {
		previousEntry, found := previousEntries[capabilityID]
		if !found {
			continue
		}
		change := diffCatalogEntries(previousEntry, entry)
		if len(change.Fields) == 0 && !change.InputSchemaChanged && !change.OutputSchemaChanged {
			continue
		}
		diff.Changed = append(diff.Changed, change)
	}
	sort.Slice(diff.Added, func(left, right int) bool { return diff.Added[left].CapabilityID < diff.Added[right].CapabilityID })
	sort.Slice(diff.Removed, func(left, right int) bool { return diff.Removed[left].CapabilityID < diff.Removed[right].CapabilityID })
	sort.Slice(diff.Changed, func(left, right int) bool { return diff.Changed[left].CapabilityID < diff.Changed[right].CapabilityID })
	return diff, nil
}

func indexCatalogEntries(catalog DescriptorCatalog) map[string]DescriptorEntry {
	index := make(map[string]DescriptorEntry, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		capabilityID := strings.TrimSpace(entry.CapabilityID)
		if _, found := index[capabilityID]; found {
			continue
		}
		index[capabilityID] = entry
	}
	return index
}

func diffCatalogEntries(previous, next DescriptorEntry) CatalogEntryChange {
	change := CatalogEntryChange{
		CapabilityID: next.CapabilityID,
		ProviderID:   next.ProviderID,
		Fields:       make([]string, 0, 8),
	}
	if previous.ProviderID != next.ProviderID {
		change.Fields = append(change.Fields, "providerId")
	}
	if previous.ServiceID != next.ServiceID {
		change.Fields = append(change.Fields, "serviceId")
	}
	if previous.Source != next.Source {
		change.Fields = append(change.Fields, "source")
	}
	if previous.Version != next.Version {
		change.Fields = append(change.Fields, "version")
	}
	if previous.ContractHash != next.ContractHash {
		change.Fields = append(change.Fields, "contractHash")
	}
	if previous.Descriptor.Placement != next.Descriptor.Placement {
		change.Fields = append(change.Fields, "placement")
	}
	if !equalScopes(previous.Descriptor.Scopes, next.Descriptor.Scopes) {
		change.Fields = append(change.Fields, "scopes")
	}
	if !equalPermissions(previous.Descriptor.Permissions, next.Descriptor.Permissions) {
		change.Fields = append(change.Fields, "permissions")
	}
	change.InputSchemaChanged = !equalSchemaRef(previous.Descriptor.InputRef, next.Descriptor.InputRef) || !bytes.Equal(previous.InputSchema, next.InputSchema)
	change.OutputSchemaChanged = !equalSchemaRef(previous.Descriptor.OutputRef, next.Descriptor.OutputRef) || !bytes.Equal(previous.OutputSchema, next.OutputSchema)
	return change
}

func equalScopes(left, right []ServiceScope) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalPermissions(left, right []DescriptorPermission) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalSchemaRef(left, right *SchemaRef) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
