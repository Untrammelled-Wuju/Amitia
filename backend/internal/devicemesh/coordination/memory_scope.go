package coordination

import (
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

func RoleRuntimeStateID(kind, roleID string) string {
	switch kind {
	case RuntimeStateKindPsyche, RuntimeStateKindNeeds:
		return "role/" + roleID
	case RuntimeStateKindRelationship:
		return "user_character/" + roleID
	default:
		return roleID
	}
}

func LegacyRuntimeStateID(kind string) string {
	switch kind {
	case RuntimeStateKindPsyche, RuntimeStateKindNeeds:
		return "role"
	case RuntimeStateKindRelationship:
		return "user_character"
	default:
		return ""
	}
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
	case "memory", "fact", "vector", "graph", "episodic", "continuity", "profile":
		return true
	}
	return false
}

func StorageRole(scope ExecutionScope, kind string) string {
	if scope.ConversationRoleID != "" && GroupConversationKind(kind) {
		return scope.ConversationRoleID
	}
	if kind == "continuity" && scope.ContinuityRoleID != "" {
		return scope.ContinuityRoleID
	}
	if kind == "worldbook" && scope.WorldbookRoleID != "" {
		return scope.WorldbookRoleID
	}
	if scope.MemoryRoleID == "" {
		return scope.RoleID
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

func ResolveMemoryScope(scope *ExecutionScope, view string) error {
	scope.MemoryRoleID = scope.RoleID
	scope.MemoryViewAll = view == "all"
	switch view {
	case "", "all", "character":
	case "global":
		scope.MemoryRoleID = GlobalMemoryRole
	default:
		if !strings.HasPrefix(view, "role:") || len(view) > 517 || len(view) <= 5 || strings.ContainsAny(view, "\x00\r\n") || strings.HasPrefix(strings.TrimPrefix(view, "role:"), "@") {
			return ErrWrongOwner
		}
		scope.MemoryRoleID = strings.TrimPrefix(view, "role:")
	}
	return nil
}
