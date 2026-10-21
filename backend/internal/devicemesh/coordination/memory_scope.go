package coordination

import (
	"encoding/json"
	"strings"
)

const GlobalMemoryRole = "@global-memory"
const AllMemoryRole = "@all-memory"
const GlobalWorldbookRole = "@global-worldbook"

const (
	RuntimeStateKindPsyche       = "psyche"
	RuntimeStateKindRelationship = "relationship"
	RuntimeStateKindNeeds        = "needs"
)

var RuntimeStateKinds = []string{
	RuntimeStateKindPsyche,
	RuntimeStateKindRelationship,
	RuntimeStateKindNeeds,
}

func RuntimeStateKind(kind string) bool {
	switch kind {
	case RuntimeStateKindPsyche, RuntimeStateKindRelationship, RuntimeStateKindNeeds:
		return true
	}
	return false
}

func MemoryKind(kind string) bool {
	switch kind {
	case "memory", "fact", "vector", "graph", "episodic", "continuity":
		return true
	}
	return false
}

func StorageRole(scope ExecutionScope, kind string) string {
	if kind == "continuity" && scope.ContinuityRoleID != "" {
		return scope.ContinuityRoleID
	}
	if kind == "worldbook" && scope.WorldbookRoleID != "" {
		return scope.WorldbookRoleID
	}
	if scope.MemoryRoleID == "" {
		return scope.RoleID
	}
	if kind == "profile" {
		return GlobalMemoryRole
	}
	if MemoryKind(kind) {
		return scope.MemoryRoleID
	}
	return scope.RoleID
}

func ResolveWorldbookScope(scope *ExecutionScope, global bool) {
	scope.WorldbookRoleID = scope.RoleID
	if global {
		scope.WorldbookRoleID = GlobalWorldbookRole
	}
}

func ResolveMemoryScope(scope *ExecutionScope, role Role, view string) error {
	var profile struct {
		Global *bool `json:"globalMemoryEnabled"`
	}
	if err := json.Unmarshal(role.Profile, &profile); err != nil {
		return ErrWrongOwner
	}
	scope.MemoryRoleID = scope.RoleID
	if profile.Global == nil || *profile.Global {
		scope.MemoryRoleID = GlobalMemoryRole
	}
	scope.MemoryViewAll = view == "all"
	switch view {
	case "", "all":
	case "global":
		scope.MemoryRoleID = GlobalMemoryRole
	case "character":
		scope.MemoryRoleID = scope.RoleID
	default:
		if !strings.HasPrefix(view, "role:") || len(view) > 517 || len(view) <= 5 || strings.ContainsAny(view, "\x00\r\n") || strings.HasPrefix(strings.TrimPrefix(view, "role:"), "@") {
			return ErrWrongOwner
		}
		scope.MemoryRoleID = strings.TrimPrefix(view, "role:")
	}
	return nil
}
