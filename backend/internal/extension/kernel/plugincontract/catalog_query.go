package plugincontract

import (
	"sort"
	"strings"
)

type catalogQueryStats struct {
	entryVisits int
	schemaReads int
}

func (c DescriptorCatalog) SelectByPermission(permission string) []DescriptorEntry {
	entries, _ := selectByPermission(c, permission)
	return entries
}

func (c DescriptorCatalog) HasPermission(capabilityID, permission string) bool {
	entry, found := c.FindByCapabilityID(capabilityID)
	if !found {
		return false
	}
	return entryAllowsPermission(entry, canonicalPermissionName(permission))
}

func selectByPermission(catalog DescriptorCatalog, permission string) ([]DescriptorEntry, catalogQueryStats) {
	stats := catalogQueryStats{}
	target := canonicalPermissionName(permission)
	if target == "" {
		return []DescriptorEntry{}, stats
	}
	matched := make([]DescriptorEntry, 0, 8)
	for _, entry := range catalog.Entries {
		stats.entryVisits++
		if entryAllowsPermission(entry, target) {
			matched = append(matched, entry)
		}
	}
	sort.Slice(matched, func(left, right int) bool {
		return matched[left].CapabilityID < matched[right].CapabilityID
	})
	return matched, stats
}

func entryAllowsPermission(entry DescriptorEntry, permission string) bool {
	if permission == "" {
		return false
	}
	for _, granted := range entry.Descriptor.Permissions {
		if canonicalPermissionName(granted.Capability) == permission {
			return true
		}
	}
	return false
}

func canonicalPermissionName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
