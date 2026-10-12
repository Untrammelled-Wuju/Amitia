package coordination

import (
	"context"
	"encoding/json"
	"strings"
)

const GlobalMemoryRole = "@global-memory"
const AllMemoryRole = "@all-memory"

type LegacyMemoryTransferRequest struct {
	Kind           string          `json:"kind"`
	ID             string          `json:"id"`
	ExpectedRecord json.RawMessage `json:"expectedRecord"`
	TargetRoleID   string          `json:"targetRoleId"`
}
type LegacyMemoryTransferPort interface {
	TransferLegacyMemory(context.Context, ExecutionScope, LegacyMemoryTransferRequest) (Acknowledgement, error)
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
