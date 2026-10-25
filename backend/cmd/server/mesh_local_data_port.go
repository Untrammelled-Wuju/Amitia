package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/spaceidentity"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type meshLocalDataPort struct {
	services      *AppServices
	ownerID       string
	legacySpaceID string
	roles         character.Repository
	store         *coordination.OwnershipStore
}

type meshLegacyCursor struct {
	Owner        string `json:"owner"`
	Role         string `json:"role"`
	Conversation string `json:"conversation"`
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	Sequence     int64  `json:"sequence"`
	UpdatedAt    string `json:"updatedAt"`
	SearchHash   string `json:"searchHash,omitempty"`
}

func encodeMeshLegacyCursor(cursor meshLegacyCursor) string {
	encoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func newMeshLocalDataPort(services *AppServices, dataDir, ownerID string) (*meshLocalDataPort, error) {
	if services == nil || services.DB == nil || services.KernelContainer == nil || services.KernelContainer.DeviceRegistry == nil || ownerID == "" {
		return nil, errors.New("设备业务数据服务不可用")
	}
	space, err := spaceidentity.Open(dataDir)
	if err != nil {
		return nil, err
	}
	if services.RelTimeCoordinator != nil {
		identity, err := agent.NewIdentityStore(dataDir).Load()
		if err != nil {
			return nil, err
		}
		if identity.DeviceID.String() == ownerID {
			if err := services.RelTimeCoordinator.MigrateOwner(context.Background(), space.SpaceID(), ownerID); err != nil {
				return nil, err
			}
		}
	}
	return &meshLocalDataPort{services: services, ownerID: ownerID, legacySpaceID: space.SpaceID(), roles: character.NewRepository(app.NewAppContext(services.DB, nil)), store: coordination.NewOwnershipStore(services.KernelContainer.DeviceRegistry.Database(), ownerID)}, nil
}

func (p *meshLocalDataPort) Roles(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Role, error) {
	unlock := character.LockRuntimeRole(p.services.DB)
	defer unlock()
	return p.rolesLocked(ctx, scope)
}

func (p *meshLocalDataPort) rolesLocked(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Role, error) {
	return p.roleCatalogLocked(ctx, scope, false)
}

func (p *meshLocalDataPort) ListOwnedRoleCatalog(ctx context.Context, scope coordination.ExecutionScope, includeDisabled bool) ([]coordination.Role, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	var result []coordination.Role
	err := p.WithSourceRole(ctx, scope, func() error {
		var err error
		result, err = p.roleCatalogLocked(ctx, scope, includeDisabled)
		return err
	})
	return result, err
}

func (p *meshLocalDataPort) GroupState(ctx context.Context, scope coordination.ExecutionScope, conversationID string) (coordination.GroupState, error) {
	if strings.TrimSpace(conversationID) == "" || scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID || scope.RoleID == "" {
		return coordination.GroupState{}, coordination.ErrWrongOwner
	}
	resource, err := p.store.Get(ctx, "conversation", conversationID)
	if err != nil {
		return coordination.GroupState{}, err
	}
	if resource == nil || resource.Deleted || resource.OwnerID != p.ownerID {
		return coordination.GroupState{}, nil
	}
	state := coordination.GroupState{ConversationID: conversationID, ConversationRoleID: resource.RoleID, ConversationRevision: resource.Revision, RoleOwnerID: p.ownerID, ConversationType: "single", MembershipRevision: 1, SettingsRevision: 1}
	if err := json.Unmarshal(resource.Body, &state); err != nil {
		return coordination.GroupState{}, coordination.ErrWrongOwner
	}
	state.ConversationID = conversationID
	state.ConversationRoleID = resource.RoleID
	state.ConversationRevision = resource.Revision
	state.RoleOwnerID = p.ownerID
	if state.ConversationType == "" {
		state.ConversationType = "single"
	}
	if state.MembershipRevision < 1 {
		state.MembershipRevision = 1
	}
	if state.SettingsRevision < 1 {
		state.SettingsRevision = 1
	}
	if state.Settings.MaxAutoReplies < 0 || state.Settings.MaxAutoReplies > coordination.GroupDefaultAutoReplies {
		state.Settings.MaxAutoReplies = coordination.GroupDefaultAutoReplies
	}
	if !state.IsGroup() {
		if scope.RoleID != resource.RoleID {
			return coordination.GroupState{}, coordination.ErrWrongOwner
		}
		return state, state.Validate()
	}
	if _, ok := state.ActiveMember(scope.RoleID); !ok {
		return coordination.GroupState{}, coordination.ErrWrongOwner
	}
	return state, state.Validate()
}

func (p *meshLocalDataPort) GroupConversations(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Resource, map[string]string, error) {
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID || scope.RoleID == "" {
		return nil, nil, coordination.ErrWrongOwner
	}
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT resource_id,role_id,revision,body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='conversation' AND deleted=0 ORDER BY updated_at DESC,resource_id DESC LIMIT 4096`, p.ownerID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	resources := make([]coordination.Resource, 0)
	roles := make(map[string]string)
	for rows.Next() {
		var resource coordination.Resource
		var body []byte
		if err := rows.Scan(&resource.ID, &resource.RoleID, &resource.Revision, &body); err != nil {
			return nil, nil, err
		}
		resource.OwnerID = p.ownerID
		resource.Kind = "conversation"
		resource.Body = json.RawMessage(body)
		var state coordination.GroupState
		if json.Unmarshal(resource.Body, &state) != nil || state.ConversationType != "group" {
			continue
		}
		state.RoleOwnerID = p.ownerID
		if _, ok := state.ActiveMember(scope.RoleID); !ok {
			continue
		}
		resources = append(resources, resource)
		roles[resource.ID] = resource.RoleID
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return resources, roles, nil
}

func (p *meshLocalDataPort) roleCatalogLocked(ctx context.Context, scope coordination.ExecutionScope, includeDisabled bool) ([]coordination.Role, error) {
	if scope.RoleOwnerID != p.ownerID || scope.ResourceOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	characters, err := p.roles.List(includeDisabled)
	if err != nil {
		return nil, err
	}
	roles := make([]coordination.Role, 0, len(characters))
	for _, row := range characters {
		if row.SpaceID != "" && row.SpaceID != "default" && row.SpaceID != p.legacySpaceID {
			continue
		}
		if row.Status != "enabled" {
			roles = append(roles, coordination.Role{ID: row.ID, Name: row.Name, Avatar: row.Avatar, Description: row.Description, Disabled: true, Revision: row.Revision})
			continue
		}
		runtime, err := p.roles.GetRuntimeProfile(row.ID)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(struct {
			*character.RoleRuntimeProfile
			Voice tts.OwnedVoiceSnapshot `json:"voice"`
		}{runtime, tts.OwnedVoiceSnapshot{VoiceConfigID: row.VoiceConfigID, VoiceType: row.VoiceType, VoiceSpeed: row.VoiceSpeed, VoicePitch: row.VoicePitch, VoiceVolume: row.VoiceVolume, CustomVoiceID: row.CustomVoiceID, VoiceMode: row.VoiceMode, Emotion: row.Emotion, EmotionScale: row.EmotionScale, SilenceDuration: row.SilenceDuration}})
		if err != nil {
			return nil, err
		}
		roles = append(roles, coordination.Role{ID: row.ID, Name: row.Name, Avatar: row.Avatar, Description: row.Description, Disabled: row.Status != "enabled", Revision: row.Revision, Profile: body})
	}
	return roles, nil
}

func (p *meshLocalDataPort) WithSourceRole(ctx context.Context, scope coordination.ExecutionScope, execute func() error) error {
	unlock := character.LockRuntimeRole(p.services.DB)
	defer unlock()
	roles, err := p.rolesLocked(ctx, scope)
	if err != nil {
		return err
	}
	role, err := coordination.ResolveRole(scope.RoleID, roles)
	if err != nil {
		return err
	}
	if role.Revision != scope.RoleRevision {
		return coordination.ErrScopeExpired
	}
	return execute()
}

func appendMeshJSON[T any](items []T) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		body, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		result = append(result, body)
	}
	return result, nil
}

func (p *meshLocalDataPort) Snapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (coordination.DataSnapshot, error) {
	available, err := p.Roles(ctx, scope)
	if err != nil {
		return coordination.DataSnapshot{}, err
	}
	role, err := coordination.ResolveRole(scope.RoleID, available)
	if err != nil {
		return coordination.DataSnapshot{}, err
	}
	if role.Revision != scope.RoleRevision {
		return coordination.DataSnapshot{}, coordination.ErrScopeExpired
	}
	return p.snapshotWithRole(ctx, scope, query, role)
}

func (p *meshLocalDataPort) snapshotWithRole(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery, role coordination.Role) (coordination.DataSnapshot, error) {
	search, searchErr := coordination.NormalizeConversationSearch(query.SearchQuery)
	if searchErr != nil {
		return coordination.DataSnapshot{}, searchErr
	}
	if search != "" {
		if !query.ListConversations || query.ConversationID != "" || query.ResourceKind != "" && query.ResourceKind != "conversation" {
			return coordination.DataSnapshot{}, coordination.ErrWrongOwner
		}
		query.SearchQuery, query.ResourceKind = search, "conversation"
	}
	searchHash := coordination.ConversationSearchHash(search)
	if err := coordination.ValidateQueryVector(query); err != nil {
		return coordination.DataSnapshot{}, err
	}
	var err error
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID || scope.RoleID != role.ID || role.Revision != scope.RoleRevision {
		return coordination.DataSnapshot{}, coordination.ErrWrongOwner
	}
	if query.ConversationID != "" {
		group, err := p.GroupState(ctx, scope, query.ConversationID)
		if err != nil {
			return coordination.DataSnapshot{}, err
		}
		if group.IsGroup() {
			scope.ConversationRoleID = group.ConversationRoleID
			query.GroupVisibilityStart = 0
			if member, ok := group.ActiveMember(scope.RoleID); ok {
				query.GroupVisibilityStart = member.HistoryStart
			}
			snapshotGroup := group
			snapshotGroup.Members = append([]coordination.GroupMember(nil), group.Members...)
			query.GroupState = &snapshotGroup
		}
	}
	snapshot := coordination.DataSnapshot{OwnerID: p.ownerID, Role: role, Resources: make([]coordination.Resource, 0), NextCursors: make(map[string]string), Group: query.GroupState, GroupRoles: make(map[string]string)}
	if snapshot.Group != nil {
		snapshot.GroupRoles[snapshot.Group.ConversationID] = snapshot.Group.ConversationRoleID
	}
	usableMemories := make(map[string]bool)
	for _, kind := range []string{"memory", "conversation", "message", "summary", "working", "profile", "episodic", "fact", "vector", "graph", "continuity", business.EmotionSnapshotKind, "checkpoint", "project", "tool-result", "worldbook", "group-round", "group-delivery", "group-invitation", coordination.RuntimeStateKindPsyche, coordination.RuntimeStateKindRelationship, coordination.RuntimeStateKindNeeds} {
		if kind == "tool-result" && !query.RuntimeTurns {
			continue
		}
		if kind == "project" && query.ResourceKind != "project" {
			continue
		}
		if query.Management && query.Cursor == "" && query.LegacyCursor != "" {
			continue
		}
		if query.ResourceKind != "" && query.ResourceKind != kind {
			continue
		}
		partition := coordination.StorageRole(scope, kind)
		if scope.MemoryViewAll && query.Management && coordination.MemoryKind(kind) {
			partition = coordination.AllMemoryRole
		}
		var resources []coordination.Resource
		partitions := []string{partition}
		if coordination.MemoryKind(kind) && partition != coordination.GlobalMemoryRole && partition != coordination.AllMemoryRole && query.Cursor == "" {
			partitions = append(partitions, coordination.GlobalMemoryRole)
		}
		seenResources := map[string]bool{}
		for _, currentPartition := range partitions {
			resourceQuery := query
			resourceQuery.Cursor = ""
			if kind == "continuity" && !query.Management {
				if lease := coordination.CommitLease(ctx); lease != nil {
					resourceQuery.ResourceID = lease.ContinuityID
				}
			}
			var listed []coordination.Resource
			var nextCursor string
			if kind == "worldbook" {
				listed, err = p.listAllWorldbookResources(ctx, currentPartition, resourceQuery)
			} else {
				listed, nextCursor, err = p.store.ListPage(ctx, kind, currentPartition, resourceQuery)
			}
			if err != nil {
				return snapshot, err
			}
			if currentPartition == partition && nextCursor != "" {
				snapshot.NextCursors[kind] = nextCursor
			}
			for _, resource := range listed {
				key := resource.Kind + "/" + resource.ID
				if seenResources[key] {
					continue
				}
				seenResources[key] = true
				resources = append(resources, resource)
			}
		}
		if kind == "message" && query.RequestID != "" {
			for _, id := range []string{query.RequestID + "/user", query.RequestID + "/assistant"} {
				found := false
				for _, resource := range resources {
					found = found || resource.ID == id
				}
				if found {
					continue
				}
				resource, err := p.store.Get(ctx, kind, id)
				if err != nil {
					return snapshot, err
				}
				if resource != nil && !resource.Deleted && resource.RoleID == scope.RoleID {
					resources = append(resources, *resource)
				}
			}
		}
		for _, resource := range resources {
			if query.Management {
				snapshot.Resources = append(snapshot.Resources, resource)
				continue
			}
			if kind == "memory" {
				usableMemories[resource.ID] = coordination.ResourceUsable(resource.Body, time.Now())
				if !usableMemories[resource.ID] {
					continue
				}
			}
			if resource.SourceID != "" {
				if _, found := usableMemories[resource.SourceID]; !found || kind == "profile" {
					source, err := p.store.Get(ctx, "memory", resource.SourceID)
					if err != nil {
						return snapshot, err
					}
					usableMemories[resource.SourceID] = source != nil && !source.Deleted && source.RoleID == resource.RoleID && coordination.ResourceUsable(source.Body, time.Now())
					if usableMemories[resource.SourceID] {
						alreadyPresent := false
						for _, row := range snapshot.Resources {
							if row.Kind == "memory" && row.ID == source.ID {
								alreadyPresent = true
								break
							}
						}
						if !alreadyPresent {
							snapshot.Resources = append(snapshot.Resources, *source)
						}
					}
				}
				if !usableMemories[resource.SourceID] {
					continue
				}
			}
			if kind == "graph" {
				var document struct {
					Content struct {
						Target         string `json:"target"`
						TargetKind     string `json:"targetKind"`
						TargetRevision int64  `json:"targetRevision"`
					} `json:"content"`
				}
				if json.Unmarshal(resource.Body, &document) != nil {
					return snapshot, coordination.ErrResourceVersion
				}
				if document.Content.TargetKind == "memory" {
					target, err := p.store.Get(ctx, "memory", strings.TrimPrefix(document.Content.Target, "memory:"))
					if err != nil {
						return snapshot, err
					}
					if target == nil || target.Deleted || target.RoleID != resource.RoleID || !coordination.ResourceUsable(target.Body, time.Now()) {
						continue
					}
				}
			}
			if kind == "conversation" && !query.ListConversations && resource.ID != query.ConversationID {
				continue
			}
			if kind == "summary" && query.GroupVisibilityStart > 0 {
				continue
			}
			if kind == "message" || kind == "summary" || kind == "working" || kind == "group-round" || kind == "group-delivery" || kind == "group-invitation" {
				var location struct {
					ConversationID string `json:"conversationId"`
				}
				if err := json.Unmarshal(resource.Body, &location); err != nil {
					return snapshot, err
				}
				if location.ConversationID != query.ConversationID {
					continue
				}
			}
			snapshot.Resources = append(snapshot.Resources, resource)
		}
	}
	if query.ListConversations {
		groupResources, groupRoles, err := p.GroupConversations(ctx, scope)
		if err != nil {
			return snapshot, err
		}
		seen := make(map[string]bool)
		for _, resource := range snapshot.Resources {
			seen[resource.Kind+"/"+resource.ID] = true
		}
		for _, resource := range groupResources {
			if seen[resource.Kind+"/"+resource.ID] {
				continue
			}
			snapshot.Resources = append(snapshot.Resources, resource)
			snapshot.GroupRoles[resource.ID] = groupRoles[resource.ID]
		}
	}
	if (query.ResourceKind == "" || query.ResourceKind == "worldbook") && scope.WorldbookRoleID != coordination.GlobalWorldbookRole {
		resources, err := p.listAllWorldbookResources(ctx, coordination.GlobalWorldbookRole, coordination.DataQuery{ResourceKind: "worldbook"})
		if err != nil {
			return snapshot, err
		}
		snapshot.Resources = append(snapshot.Resources, resources...)
	}
	conversationIndexes := make([]int, 0)
	conversationBodies := make([]json.RawMessage, 0)
	for index := range snapshot.Resources {
		if snapshot.Resources[index].Kind != "conversation" {
			continue
		}
		conversationIndexes = append(conversationIndexes, index)
		conversationBodies = append(conversationBodies, snapshot.Resources[index].Body)
	}
	if len(conversationBodies) > 0 {
		enriched, err := chat.EnrichConversationActivityBodies(p.services.DB, conversationBodies)
		if err != nil {
			return snapshot, err
		}
		for index, resourceIndex := range conversationIndexes {
			snapshot.Resources[resourceIndex].Body = enriched[index]
		}
	}
	if query.ResourceKind == "" && len(query.Vector) > 0 {
		relevant, err := p.semanticSearch(ctx, coordination.StorageRole(scope, "memory"), query)
		if err != nil {
			return snapshot, err
		}
		seen := map[string]bool{}
		for _, resource := range snapshot.Resources {
			seen[resource.Kind+"/"+resource.ID] = true
		}
		for _, resource := range relevant {
			key := resource.Kind + "/" + resource.ID
			if !seen[key] {
				snapshot.Resources = append(snapshot.Resources, resource)
				seen[key] = true
			}
		}
	}
	if query.Management || query.ResourceKind == "continuity" || (query.Cursor != "" && query.LegacyCursor == "") {
		return snapshot, coordination.ValidateSnapshot(scope, snapshot)
	}
	var legacyConversations, legacyMessages []json.RawMessage
	var legacySummary json.RawMessage
	var legacyCursor meshLegacyCursor
	if query.LegacyCursor != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(query.LegacyCursor)
		if len(query.LegacyCursor) > 4096 || err != nil || json.Unmarshal(encoded, &legacyCursor) != nil || legacyCursor.Owner != p.ownerID || legacyCursor.Role != scope.RoleID || legacyCursor.Conversation != query.ConversationID || legacyCursor.Kind != query.ResourceKind || legacyCursor.SearchHash != searchHash || legacyCursor.ID == "" || len(legacyCursor.ID) > 512 {
			return snapshot, coordination.ErrWrongOwner
		}
	}
	limit := query.Limit
	if limit <= 0 || limit > 128 {
		limit = 64
	}
	db := p.services.DB.WithContext(ctx)
	if query.ListConversations {
		var conversations []chat.Conversation
		selection := db.Where("deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default') AND id IN (SELECT conversation_id FROM messages WHERE character_id=? AND deleted_at IS NULL)", p.legacySpaceID, scope.RoleID)
		if search != "" {
			selection = selection.Where("instr(lower(COALESCE(title,'')),?)>0 OR id IN (SELECT conversation_id FROM messages WHERE character_id=? AND deleted_at IS NULL AND instr(lower(content),?)>0)", search, scope.RoleID, search)
		}
		if query.LegacyCursor != "" {
			selection = selection.Where("updated_at<? OR (updated_at=? AND id<?)", legacyCursor.UpdatedAt, legacyCursor.UpdatedAt, legacyCursor.ID)
		}
		if err := selection.Order("updated_at DESC,id DESC").Limit(limit + 1).Find(&conversations).Error; err != nil {
			return snapshot, err
		}
		if len(conversations) > limit {
			last := conversations[limit-1]
			snapshot.NextCursors["legacyConversation"] = encodeMeshLegacyCursor(meshLegacyCursor{Owner: p.ownerID, Role: scope.RoleID, Kind: "conversation", ID: last.ID, UpdatedAt: last.UpdatedAt, SearchHash: searchHash})
			conversations = conversations[:limit]
		}
		legacyConversations, err = appendMeshJSON(conversations)
		if err != nil {
			return snapshot, err
		}
	}
	if query.ConversationID != "" {
		var conversation chat.Conversation
		err := db.Where("id=? AND deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default')", query.ConversationID, p.legacySpaceID).First(&conversation).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return snapshot, err
		}
		if err == nil {
			var messages []chat.Message
			selection := db.Where("conversation_id=? AND character_id=? AND deleted_at IS NULL", query.ConversationID, scope.RoleID)
			if query.LegacyCursor != "" {
				selection = selection.Where("sequence<? OR (sequence=? AND id<?)", legacyCursor.Sequence, legacyCursor.Sequence, legacyCursor.ID)
			}
			if err := selection.Order("sequence DESC,id DESC").Limit(limit + 1).Find(&messages).Error; err != nil {
				return snapshot, err
			}
			if len(messages) > limit {
				last := messages[limit-1]
				snapshot.NextCursors["legacyMessage"] = encodeMeshLegacyCursor(meshLegacyCursor{Owner: p.ownerID, Role: scope.RoleID, Conversation: query.ConversationID, Kind: "message", ID: last.ID, Sequence: last.Sequence})
				messages = messages[:limit]
			}
			for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
				messages[i], messages[j] = messages[j], messages[i]
			}
			legacyMessages, err = appendMeshJSON(messages)
			if err != nil {
				return snapshot, err
			}
			var summary chat.ConversationSummary
			var otherRoles int64
			if err := db.Model(&chat.Message{}).Where("conversation_id=? AND deleted_at IS NULL AND character_id<>?", query.ConversationID, scope.RoleID).Count(&otherRoles).Error; err != nil {
				return snapshot, err
			}
			if len(messages) > 0 && otherRoles == 0 {
				err = db.Where("conversation_id=?", query.ConversationID).First(&summary).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return snapshot, err
				}
				if err == nil {
					legacySummary, err = json.Marshal(summary)
					if err != nil {
						return snapshot, err
					}
				}
			}
		}
	}
	if query.ResourceKind == "message" || query.ResourceKind == "conversation" {
		if err := p.mergeLegacyChatSnapshot(ctx, &snapshot, scope, query, legacyConversations, legacyMessages, legacySummary); err != nil {
			return snapshot, err
		}
		return snapshot, coordination.ValidateSnapshot(scope, snapshot)
	}
	if !query.Management && query.ResourceKind == "" {
		snapshot.RuntimeContext, err = p.ownedRuntimeContext(ctx, scope, query)
		if err != nil {
			return snapshot, err
		}
	}
	if err := p.mergeLegacyChatSnapshot(ctx, &snapshot, scope, query, legacyConversations, legacyMessages, legacySummary); err != nil {
		return snapshot, err
	}
	return snapshot, coordination.ValidateSnapshot(scope, snapshot)
}

func (p *meshLocalDataPort) mergeLegacyChatSnapshot(ctx context.Context, snapshot *coordination.DataSnapshot, scope coordination.ExecutionScope, query coordination.DataQuery, legacyConversations, legacyMessages []json.RawMessage, legacySummary json.RawMessage) error {
	if snapshot == nil || snapshot.OwnerID != p.ownerID {
		return coordination.ErrWrongOwner
	}
	appendLegacy := func(kind, id, updatedAt string, raw json.RawMessage) error {
		if id == "" || len(id) > 512 || !json.Valid(raw) {
			return coordination.ErrWrongOwner
		}
		existing, err := p.store.Get(ctx, kind, id)
		if err != nil {
			return err
		}
		if existing != nil && existing.RoleID == scope.RoleID {
			return nil
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			return err
		}
		document["id"] = id
		document["ownerId"] = p.ownerID
		document["roleId"] = scope.RoleID
		document["origin"] = "legacy"
		document["readOnly"] = true
		if kind == "message" || kind == "conversation" {
			document["characterId"] = scope.RoleID
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			return err
		}
		snapshot.Resources = append(snapshot.Resources, coordination.Resource{
			OwnerID:   p.ownerID,
			Kind:      kind,
			ID:        id,
			RoleID:    scope.RoleID,
			SourceID:  "legacy",
			Revision:  0,
			UpdatedAt: updatedAt,
			Origin:    "legacy",
			ReadOnly:  true,
			Body:      encoded,
		})
		return nil
	}
	for _, raw := range legacyConversations {
		var row struct {
			ID        string `json:"id"`
			UpdatedAt string `json:"updatedAt"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			return err
		}
		if err := appendLegacy("conversation", row.ID, row.UpdatedAt, raw); err != nil {
			return err
		}
	}
	for _, raw := range legacyMessages {
		var row struct {
			ID        string `json:"id"`
			UpdatedAt string `json:"updatedAt"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			return err
		}
		if err := appendLegacy("message", row.ID, row.UpdatedAt, raw); err != nil {
			return err
		}
	}
	if len(legacySummary) > 0 && query.ConversationID != "" {
		var document map[string]any
		if err := json.Unmarshal(legacySummary, &document); err != nil {
			return err
		}
		summaryID := query.ConversationID + "/summary"
		existing, err := p.store.Get(ctx, "summary", summaryID)
		if err != nil {
			return err
		}
		if existing == nil || existing.RoleID != scope.RoleID {
			summaryText := ""
			for _, key := range []string{"summaryText", "summary_text", "text"} {
				if value, ok := document[key].(string); ok && strings.TrimSpace(value) != "" {
					summaryText = strings.TrimSpace(value)
					break
				}
			}
			body := map[string]any{
				"id":              summaryID,
				"conversationId":  query.ConversationID,
				"ownerId":         p.ownerID,
				"roleId":          scope.RoleID,
				"origin":          "legacy",
				"readOnly":        true,
				"content":         map[string]any{"summary": summaryText},
				"summaryText":     summaryText,
				"legacySummaryId": document["id"],
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				return err
			}
			snapshot.Resources = append(snapshot.Resources, coordination.Resource{
				OwnerID:  p.ownerID,
				Kind:     "summary",
				ID:       summaryID,
				RoleID:   scope.RoleID,
				SourceID: "legacy",
				Revision: 0,
				Origin:   "legacy",
				ReadOnly: true,
				Body:     encoded,
			})
		}
	}
	return nil
}

func (p *meshLocalDataPort) listAllWorldbookResources(ctx context.Context, partition string, query coordination.DataQuery) ([]coordination.Resource, error) {
	result := make([]coordination.Resource, 0, 128)
	cursor := ""
	for {
		pageQuery := query
		pageQuery.Cursor = cursor
		pageQuery.Limit = 128
		page, next, err := p.store.ListPage(ctx, "worldbook", partition, pageQuery)
		if err != nil {
			return nil, err
		}
		result = append(result, page...)
		if len(result) > 4096 {
			return nil, coordination.ErrPendingLimit
		}
		if next == "" {
			return result, nil
		}
		if next == cursor {
			return nil, coordination.ErrWrongOwner
		}
		cursor = next
	}
}

func (p *meshLocalDataPort) HistoricalConversations(ctx context.Context, scope coordination.ExecutionScope) ([]json.RawMessage, error) {
	if !scope.Coordinated || scope.TargetDeviceID != p.ownerID || scope.ResourceOwnerID != scope.CoreID || scope.RoleOwnerID != scope.CoreID {
		return nil, coordination.ErrWrongOwner
	}
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='conversation' AND deleted=0 ORDER BY updated_at DESC,resource_id DESC LIMIT 128`, p.ownerID)
	if err != nil {
		return nil, err
	}
	result := []json.RawMessage{}
	seen := map[string]bool{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return nil, err
		}
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			_ = rows.Close()
			return nil, err
		}
		value["ownerId"] = p.ownerID
		id, _ := value["id"].(string)
		if id == "" {
			continue
		}
		seen[id] = true
		encoded, err := json.Marshal(value)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		result = append(result, encoded)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	var conversations []chat.Conversation
	if err := p.services.DB.WithContext(ctx).Where("deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default')", p.legacySpaceID).Order("updated_at DESC").Limit(128).Find(&conversations).Error; err != nil {
		return nil, err
	}
	for _, conversation := range conversations {
		if seen[conversation.ID] {
			continue
		}
		encoded, err := json.Marshal(conversation)
		if err != nil {
			return nil, err
		}
		var value map[string]any
		if err := json.Unmarshal(encoded, &value); err != nil {
			return nil, err
		}
		value["ownerId"] = p.ownerID
		encoded, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded)
	}
	return chat.EnrichConversationActivityBodies(p.services.DB, result)
}

func (p *meshLocalDataPort) HistoricalSnapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (*coordination.DataSnapshot, error) {
	if scope.TargetDeviceID != p.ownerID || !scope.Coordinated || scope.CoreID == "" || query.ConversationID == "" && query.HistoricalRoleID == "" {
		return nil, coordination.ErrWrongOwner
	}
	db := p.services.KernelContainer.DeviceRegistry.Database()
	var storedRole string
	var err error
	if query.ConversationID == "" {
		roles, err := p.HistoricalRoles(ctx, scope)
		if err != nil {
			return nil, err
		}
		for _, role := range roles {
			if role.ID == query.HistoricalRoleID {
				storedRole = role.ID
				break
			}
		}
		if storedRole == "" {
			return nil, coordination.ErrRoleRequired
		}
	} else {
		err = db.QueryRowContext(ctx, `SELECT role_id FROM kernel_device_owned_resources WHERE owner_id=? AND kind='conversation' AND resource_id=? AND deleted=0`, p.ownerID, query.ConversationID).Scan(&storedRole)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		var conversation chat.Conversation
		err = p.services.DB.WithContext(ctx).Where("id=? AND deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default')", query.ConversationID, p.legacySpaceID).First(&conversation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if query.HistoricalRoleID != "" {
				query.ConversationID = ""
				return p.HistoricalSnapshot(ctx, scope, query)
			}
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		var roleIDs []string
		if err := p.services.DB.WithContext(ctx).Model(&chat.Message{}).Where("conversation_id=? AND deleted_at IS NULL AND character_id<>''", query.ConversationID).Distinct("character_id").Limit(3).Pluck("character_id", &roleIDs).Error; err != nil {
			return nil, err
		}
		if len(roleIDs) == 0 {
			return nil, nil
		}
		storedRole = query.HistoricalRoleID
		if storedRole == "" {
			if len(roleIDs) != 1 {
				return nil, coordination.ErrRoleSelection
			}
			storedRole = roleIDs[0]
		} else {
			found := false
			for _, id := range roleIDs {
				found = found || id == storedRole
			}
			if !found {
				return nil, coordination.ErrRoleRequired
			}
		}
	}
	if query.HistoricalRoleID != "" && query.HistoricalRoleID != storedRole {
		return nil, coordination.ErrRoleRequired
	}
	historical := scope
	historical.ResourceOwnerID = p.ownerID
	historical.RoleOwnerID = p.ownerID
	historical.RoleID = storedRole
	historical.MemoryRoleID = storedRole
	historical.MemoryViewAll = false
	role := coordination.Role{ID: storedRole, Name: "设备历史角色", Revision: 1, Profile: json.RawMessage(`{}`)}
	historical.RoleRevision = role.Revision
	snapshot, err := p.snapshotWithRole(ctx, historical, query, role)
	if err != nil {
		return nil, err
	}
	filtered := make([]coordination.Resource, 0, len(snapshot.Resources))
	for _, resource := range snapshot.Resources {
		if resource.Kind != "checkpoint" && (resource.Kind != "continuity" || query.Management && query.ResourceKind == "continuity") {
			filtered = append(filtered, resource)
		}
	}
	snapshot.Resources = filtered
	snapshot.Role.Profile = json.RawMessage(`{}`)
	return &snapshot, nil
}

func (p *meshLocalDataPort) SaveInterrupted(ctx context.Context, reply coordination.InterruptedReply) error {
	return p.store.SaveInterrupted(ctx, reply)
}

func (p *meshLocalDataPort) Commit(ctx context.Context, commit coordination.Commit) (coordination.Acknowledgement, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return coordination.Acknowledgement{}, err
	}
	ctx = coordination.WithoutAdditionalGuard(ctx)
	unlock := character.LockRuntimeRole(p.services.DB)
	defer unlock()
	roles, err := p.rolesLocked(ctx, commit.Scope)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	role, err := coordination.ResolveRole(commit.Scope.RoleID, roles)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	if role.Revision != commit.Scope.RoleRevision {
		return coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	if commit.MemoryTransfer != nil && commit.MemoryTransfer.ToRoleID != coordination.GlobalMemoryRole {
		target, err := coordination.ResolveRole(commit.MemoryTransfer.ToRoleID, roles)
		if err != nil || target.Revision != commit.MemoryTransfer.TargetRoleRevision {
			return coordination.Acknowledgement{}, coordination.ErrScopeExpired
		}
	}
	var ack coordination.Acknowledgement
	err = coordination.CommitCurrent(ctx, func() error {
		var err error
		ack, err = p.store.Apply(ctx, commit)
		return err
	})
	return ack, err
}

func (p *meshLocalDataPort) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	if scope.ResourceOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	if err := coordination.ValidateRoleRevision(ctx, p, scope); err != nil {
		return nil, err
	}
	resource, err := p.store.Get(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	if resource != nil && resource.RoleID != coordination.StorageRole(scope, kind) {
		return nil, coordination.ErrWrongOwner
	}
	return resource, nil
}

func (p *meshLocalDataPort) ReadTaskResource(ctx context.Context, current coordination.ExecutionScope, proof coordination.TaskReadProof, kind, id string) (*coordination.Resource, error) {
	if proof.Scope.ResourceOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateTaskReadProof(current, proof); err != nil {
		return nil, err
	}
	if err := coordination.ValidateTaskReadResourceID(proof, kind, id); err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	resource, err := p.store.Get(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	if err := coordination.ValidateTaskReadResource(proof, kind, id, resource); err != nil {
		return nil, err
	}
	return resource, coordination.ValidateCurrent(ctx)
}
