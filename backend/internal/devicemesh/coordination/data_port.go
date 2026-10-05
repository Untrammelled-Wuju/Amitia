package coordination

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Role struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Revision int64           `json:"revision"`
	Profile  json.RawMessage `json:"profile"`
}

type DataQuery struct {
	Management             bool      `json:"management,omitempty"`
	Vector                 []float32 `json:"vector,omitempty"`
	VectorModel            string    `json:"vectorModel,omitempty"`
	Cursor                 string    `json:"cursor,omitempty"`
	HistoricalCursor       string    `json:"historicalCursor,omitempty"`
	LegacyCursor           string    `json:"legacyCursor,omitempty"`
	HistoricalLegacyCursor string    `json:"historicalLegacyCursor,omitempty"`
	HistoricalListCursor   string    `json:"historicalListCursor,omitempty"`
	ResourceKind           string    `json:"resourceKind,omitempty"`
	ResourceID             string    `json:"resourceId,omitempty"`
	RequestID              string    `json:"requestId,omitempty"`
	ConversationID         string    `json:"conversationId,omitempty"`
	Query                  string    `json:"query,omitempty"`
	SearchQuery            string    `json:"searchQuery,omitempty"`
	Limit                  int       `json:"limit,omitempty"`
	HistoricalRoleID       string    `json:"historicalRoleId,omitempty"`
	ListConversations      bool      `json:"listConversations,omitempty"`
}

func NormalizeConversationSearch(value string) (string, error) {
	if len(value) > 256 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return "", errors.New("会话搜索内容无效或超过 256 字节")
	}
	return strings.ToLower(strings.TrimSpace(value)), nil
}

const OwnedConversationSearchPredicate = ` AND (instr(lower(COALESCE(json_extract(CAST(body AS TEXT),'$.title'),'')),?)>0 OR EXISTS (SELECT 1 FROM kernel_device_owned_resources AS matched WHERE matched.owner_id=kernel_device_owned_resources.owner_id AND matched.role_id=kernel_device_owned_resources.role_id AND matched.kind='message' AND matched.deleted=0 AND json_extract(CAST(matched.body AS TEXT),'$.conversationId')=kernel_device_owned_resources.resource_id AND instr(lower(COALESCE(json_extract(CAST(matched.body AS TEXT),'$.content'),'')),?)>0))`

type DataSnapshot struct {
	NextCursors         map[string]string `json:"nextCursors,omitempty"`
	OwnerID             string            `json:"ownerId"`
	Role                Role              `json:"role"`
	Resources           []Resource        `json:"resources"`
	LegacyMessages      []json.RawMessage `json:"legacyMessages,omitempty"`
	LegacySummary       json.RawMessage   `json:"legacySummary,omitempty"`
	LegacyMemories      []json.RawMessage `json:"legacyMemories,omitempty"`
	LegacyProfiles      []json.RawMessage `json:"legacyProfiles,omitempty"`
	LegacyEpisodes      []json.RawMessage `json:"legacyEpisodes,omitempty"`
	LegacyConversations []json.RawMessage `json:"legacyConversations,omitempty"`
}

type DataPort interface {
	Roles(context.Context, ExecutionScope) ([]Role, error)
	Snapshot(context.Context, ExecutionScope, DataQuery) (DataSnapshot, error)
	Commit(context.Context, Commit) (Acknowledgement, error)
}

type HistoricalDataPort interface {
	HistoricalSnapshot(context.Context, ExecutionScope, DataQuery) (*DataSnapshot, error)
}

type HistoricalRole struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type HistoricalRolesPort interface {
	HistoricalRoles(context.Context, ExecutionScope) ([]HistoricalRole, error)
}

type HistoricalConversationPort interface {
	HistoricalConversations(context.Context, ExecutionScope) ([]json.RawMessage, error)
}

type HistoricalConversationPage struct {
	Conversations []json.RawMessage `json:"conversations"`
	NextCursor    string            `json:"nextCursor,omitempty"`
}

type HistoricalConversationPagePort interface {
	HistoricalConversationPage(context.Context, ExecutionScope, DataQuery) (HistoricalConversationPage, error)
}

type ResourcePort interface {
	Resource(context.Context, ExecutionScope, string, string) (*Resource, error)
}

func ResolveRole(requested string, available []Role) (Role, error) {
	ids := make([]string, len(available))
	for i, role := range available {
		if role.ID == "" || role.Revision < 0 || !json.Valid(role.Profile) {
			return Role{}, errors.New("角色来源返回了无效数据")
		}
		ids[i] = role.ID
	}
	id, err := SelectRole(requested, ids)
	if err != nil {
		return Role{}, err
	}
	for _, role := range available {
		if role.ID == id {
			return role, nil
		}
	}
	return Role{}, ErrRoleRequired
}

func ValidateSnapshot(scope ExecutionScope, snapshot DataSnapshot) error {
	if snapshot.OwnerID != scope.ResourceOwnerID || snapshot.Role.ID != scope.RoleID || snapshot.Role.Revision != scope.RoleRevision || !json.Valid(snapshot.Role.Profile) {
		return ErrWrongOwner
	}
	if len(snapshot.Resources) > 4096 || len(snapshot.LegacyMessages) > 256 || len(snapshot.LegacyMemories) > 256 || len(snapshot.LegacyProfiles) > 256 || len(snapshot.LegacyEpisodes) > 256 || len(snapshot.LegacyConversations) > 256 {
		return ErrPendingLimit
	}
	if len(snapshot.NextCursors) > 16 {
		return ErrPendingLimit
	}
	for kind, cursor := range snapshot.NextCursors {
		if (!validKind(kind) && kind != "legacyMessage" && kind != "legacyConversation" && kind != "legacyMemory" && kind != "legacyProfile" && kind != "legacyEpisode") || len(cursor) > 4096 {
			return ErrWrongOwner
		}
	}
	seen := make(map[string]bool)
	for _, resource := range snapshot.Resources {
		key := resource.Kind + "/" + resource.ID
		if resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID || resource.Deleted || !validKind(resource.Kind) || resource.ID == "" || resource.Revision < 1 || !json.Valid(resource.Body) || seen[key] {
			return fmt.Errorf("%w: 无效上下文记录", ErrWrongOwner)
		}
		seen[key] = true
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if len(encoded) > 4<<20 {
		return ErrPendingLimit
	}
	return nil
}

func ValidateRoleRevision(ctx context.Context, port DataPort, scope ExecutionScope) error {
	roles, err := port.Roles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := ResolveRole(scope.RoleID, roles)
	if err != nil {
		return err
	}
	if role.Revision != scope.RoleRevision {
		return ErrScopeExpired
	}
	return nil
}

type SourceRoleExecutionGuard interface {
	WithSourceRole(context.Context, ExecutionScope, func() error) error
}
