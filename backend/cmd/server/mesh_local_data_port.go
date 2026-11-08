package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/spaceidentity"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/pkg/app"
	amitialog "log"
)

type meshLocalDataPort struct {
	services *AppServices
	ownerID  string
	spaceID  string
	roles    character.Repository
	store    *coordination.OwnershipStore
}

func (p *meshLocalDataPort) ownsScope(scope coordination.ExecutionScope) bool {
	return scope.ResourceOwnerID == p.ownerID && (scope.RoleOwnerID == p.ownerID || scope.RoleOwnerID == scope.CoreID)
}

func projectGroupResourceForSnapshot(resource coordination.Resource) (coordination.Resource, error) {
	if resource.Kind != "group-round" && resource.Kind != "group-delivery" && resource.Kind != "conversation" {
		return resource, nil
	}
	var document map[string]any
	if err := json.Unmarshal(resource.Body, &document); err != nil {
		return resource, err
	}
	if resource.Kind == "conversation" {
		if document["conversationType"] != "group" {
			return resource, nil
		}
		members, _ := document["members"].([]any)
		for _, raw := range members {
			member, _ := raw.(map[string]any)
			delete(member, "characterAvatar")
		}
	} else {
		delete(document, "characterAvatar")
	}
	if resource.Kind == "group-round" {
		deliveries, _ := document["deliveries"].([]any)
		for _, raw := range deliveries {
			delivery, _ := raw.(map[string]any)
			delete(delivery, "characterAvatar")
		}
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return resource, err
	}
	resource.Body = encoded
	return resource, nil
}

func newMeshLocalDataPort(services *AppServices, dataDir, ownerID string) (*meshLocalDataPort, error) {
	if services == nil || services.DB == nil || services.KernelContainer == nil || services.KernelContainer.DeviceRegistry == nil || ownerID == "" {
		return nil, errors.New("设备业务数据服务不可用")
	}
	space, err := spaceidentity.Open(dataDir)
	if err != nil {
		return nil, err
	}
	return &meshLocalDataPort{services: services, ownerID: ownerID, spaceID: space.SpaceID(), roles: character.NewRepository(app.NewAppContext(services.DB, nil)), store: coordination.NewOwnershipStore(services.KernelContainer.DeviceRegistry.Database(), ownerID)}, nil
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
	if strings.TrimSpace(conversationID) == "" || !p.ownsScope(scope) || scope.RoleID == "" {
		return coordination.GroupState{}, fmt.Errorf("会话分组范围无效 resourceOwner=%q roleOwner=%q owner=%q role=%q: %w", scope.ResourceOwnerID, scope.RoleOwnerID, p.ownerID, scope.RoleID, coordination.ErrWrongOwner)
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
	state.RoleOwnerID = scope.RoleOwnerID
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
	if !p.ownsScope(scope) || scope.RoleID == "" {
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
	if (scope.RoleOwnerID != p.ownerID && scope.RoleOwnerID != scope.CoreID) || scope.ResourceOwnerID == "" {
		return nil, fmt.Errorf("角色目录所有者不匹配 roleOwner=%q expectedRoleOwner=%q resourceOwner=%q: %w", scope.RoleOwnerID, p.ownerID, scope.ResourceOwnerID, coordination.ErrWrongOwner)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	characters, err := p.roles.List(includeDisabled)
	if err != nil {
		return nil, err
	}
	var agents []agentplatform.AgentProfile
	if err := p.services.DB.WithContext(ctx).Where("space_id = ? AND deleted_at IS NULL", p.spaceID).Find(&agents).Error; err != nil {
		return nil, err
	}
	agentStatus := make(map[string]string, len(agents))
	for _, agent := range agents {
		agentStatus[agent.CharacterID] = agent.Status
	}
	roles := make([]coordination.Role, 0, len(characters))
	for _, row := range characters {
		if row.SpaceID != p.spaceID {
			continue
		}
		status, exists := agentStatus[row.ID]
		if !exists || row.Status == "enabled" && status != "active" || row.Status != "enabled" && status != "disabled" {
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
		roles = append(roles, coordination.Role{
			ID: row.ID, Name: row.Name, Avatar: row.Avatar, Description: row.Description,
			Disabled: row.Status != "enabled", Revision: row.Revision, Profile: body,
			ModelConfigID: runtime.ModelConfigID, ReasoningEffort: runtime.ReasoningEffort,
			ReasoningEnabled: runtime.ReasoningEnabled,
		})
	}
	return roles, nil
}

func (p *meshLocalDataPort) WithSourceRole(ctx context.Context, scope coordination.ExecutionScope, execute func() error) error {
	if scope.RoleOwnerID == scope.CoreID && scope.CoreID != p.ownerID {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		return execute()
	}
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
	if query.Role != nil {
		return p.snapshotWithRole(ctx, scope, query, *query.Role)
	}
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

func (p *meshLocalDataPort) SnapshotWithRole(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery, role coordination.Role) (coordination.DataSnapshot, error) {
	snapshot, err := p.snapshotWithRole(ctx, scope, query, role)
	if err != nil {
		return snapshot, fmt.Errorf("按中心角色读取本机快照失败: %w", err)
	}
	return snapshot, nil
}

func (p *meshLocalDataPort) snapshotWithRole(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery, role coordination.Role) (coordination.DataSnapshot, error) {
	search, searchErr := coordination.NormalizeConversationSearch(query.SearchQuery)
	if searchErr != nil {
		return coordination.DataSnapshot{}, fmt.Errorf("校验本机快照搜索条件失败: %w", searchErr)
	}
	if search != "" {
		if !query.ListConversations || query.ConversationID != "" || query.ResourceKind != "" && query.ResourceKind != "conversation" {
			return coordination.DataSnapshot{}, fmt.Errorf("本机快照搜索范围无效: %w", coordination.ErrWrongOwner)
		}
		query.SearchQuery, query.ResourceKind = search, "conversation"
	}
	if err := coordination.ValidateQueryVector(query); err != nil {
		return coordination.DataSnapshot{}, fmt.Errorf("校验本机快照向量条件失败: %w", err)
	}
	var err error
	if !p.ownsScope(scope) || scope.RoleID != role.ID || role.Revision != scope.RoleRevision {
		return coordination.DataSnapshot{}, fmt.Errorf("本机快照范围无效: %w", coordination.ErrWrongOwner)
	}
	if query.ConversationID != "" {
		group, err := p.GroupState(ctx, scope, query.ConversationID)
		if err != nil {
			return coordination.DataSnapshot{}, fmt.Errorf("读取本机会话分组失败 resourceOwner=%q roleOwner=%q owner=%q: %w", scope.ResourceOwnerID, scope.RoleOwnerID, p.ownerID, err)
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
	deviceOnly := scope.Coordinated && scope.ResourceOwnerID != scope.CoreID
	validationScope := scope
	if deviceOnly {
		validationScope.Coordinated = false
	}
	if snapshot.Group != nil {
		snapshot.GroupRoles[snapshot.Group.ConversationID] = snapshot.Group.ConversationRoleID
	}
	usableMemories := make(map[string]bool)
	for _, kind := range []string{"memory", "conversation", "message", "summary", "working", "profile", "episodic", "fact", "vector", "graph", "continuity", business.EmotionSnapshotKind, "checkpoint", "project", "tool-result", "worldbook", "group-round", "group-delivery", "group-invitation", coordination.RuntimeStateKindPsyche, coordination.RuntimeStateKindRelationship, coordination.RuntimeStateKindNeeds} {
		if deviceOnly && coordination.CoreOwnedKind(kind) {
			continue
		}
		if query.Management && query.RuntimeTurns && query.ResourceKind == "" && kind != "conversation" && kind != "message" && kind != "checkpoint" {
			continue
		}
		if kind == "tool-result" && !query.RuntimeTurns {
			continue
		}
		if kind == "project" && query.ResourceKind != "project" {
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
				return snapshot, fmt.Errorf("读取本机 %s/%s 资源失败: %w", kind, currentPartition, err)
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
			projected, err := projectGroupResourceForSnapshot(resource)
			if err != nil {
				return snapshot, err
			}
			resource = projected
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
			return snapshot, fmt.Errorf("读取本机会话列表失败: %w", err)
		}
		seen := make(map[string]bool)
		for _, resource := range snapshot.Resources {
			seen[resource.Kind+"/"+resource.ID] = true
		}
		for _, resource := range groupResources {
			if seen[resource.Kind+"/"+resource.ID] {
				continue
			}
			projected, err := projectGroupResourceForSnapshot(resource)
			if err != nil {
				return snapshot, err
			}
			resource = projected
			snapshot.Resources = append(snapshot.Resources, resource)
			snapshot.GroupRoles[resource.ID] = groupRoles[resource.ID]
		}
	}
	if (query.ResourceKind == "" || query.ResourceKind == "worldbook") && scope.WorldbookRoleID != coordination.GlobalWorldbookRole {
		resources, err := p.listAllWorldbookResources(ctx, coordination.GlobalWorldbookRole, coordination.DataQuery{ResourceKind: "worldbook"})
		if err != nil {
			return snapshot, fmt.Errorf("读取本机世界书失败: %w", err)
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
			resource := &snapshot.Resources[resourceIndex]
			resource.Body = enriched[index]
			if query.ListConversations {
				active := business.RuntimeManager().RuntimeSnapshot(business.RuntimeKey(scope, resource.ID)).ActiveTurn
				if active != nil {
					var document map[string]any
					if err := json.Unmarshal(resource.Body, &document); err != nil {
						return snapshot, err
					}
					document["isGenerating"] = true
					label := "正在生成回复"
					for i := len(active.Blocks) - 1; i >= 0; i-- {
						block := active.Blocks[i]
						switch block.Type {
						case "reasoning":
							label = "正在思考"
						case "text":
							if content := strings.TrimSpace(block.Content); content != "" {
								label = content
							}
						case "tool_call":
							label = "正在使用工具"
							if name, ok := block.Payload["toolName"].(string); ok && strings.TrimSpace(name) != "" {
								label = "正在使用 " + strings.TrimSpace(name) + " 工具"
							}
						default:
							continue
						}
						break
					}
					document["activityLabel"] = label
					encoded, err := json.Marshal(document)
					if err != nil {
						return snapshot, err
					}
					resource.Body = encoded
				}
			}
		}
	}
	if query.ResourceKind == "" && len(query.Vector) > 0 && !deviceOnly {
		relevant, err := p.semanticSearch(ctx, coordination.StorageRole(scope, "memory"), query)
		if err != nil {
			return snapshot, err
		}
		seen := map[string]bool{}
		for _, resource := range snapshot.Resources {
			seen[resource.Kind+"/"+resource.ID] = true
		}
		prioritized := make([]coordination.Resource, 0, len(snapshot.Resources)+len(relevant))
		added := map[string]bool{}
		for _, resource := range relevant {
			key := resource.Kind + "/" + resource.ID
			if resource.Kind == "memory" && !seen["recall/"+key] {
				snapshot.RecallEvidence = append(snapshot.RecallEvidence, coordination.ResourceVersion{Kind: "memory", ID: resource.ID, Revision: resource.Revision})
				seen["recall/"+key] = true
			}
			if !added[key] {
				prioritized = append(prioritized, resource)
				added[key] = true
			}
		}
		for _, resource := range snapshot.Resources {
			key := resource.Kind + "/" + resource.ID
			if !added[key] {
				prioritized = append(prioritized, resource)
				added[key] = true
			}
		}
		snapshot.Resources = prioritized
	}
	if query.Management || query.ResourceKind == "continuity" || query.Cursor != "" {
		return snapshot, coordination.ValidateSnapshot(validationScope, snapshot)
	}
	if !query.Management && query.ResourceKind == "" {
		snapshot.RuntimeContext, err = p.ownedRuntimeContext(ctx, scope, query)
		if err != nil {
			return snapshot, fmt.Errorf("读取本机运行时上下文失败: %w", err)
		}
	}
	if err := coordination.ValidateSnapshot(validationScope, snapshot); err != nil {
		return snapshot, fmt.Errorf("校验本机数据快照失败: %w", err)
	}
	return snapshot, nil
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
	if !scope.Coordinated || scope.TargetDeviceID != p.ownerID || scope.ResourceOwnerID != p.ownerID || (scope.RoleOwnerID != p.ownerID && scope.RoleOwnerID != scope.CoreID) {
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
			if err := rows.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, err
		}
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			if err := rows.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
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
			if err := rows.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, err
		}
		result = append(result, encoded)
	}
	err = rows.Err()
	if err := rows.Close(); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if err != nil {
		return nil, err
	}
	return chat.EnrichConversationActivityBodies(p.services.DB, result)
}

func (p *meshLocalDataPort) HistoricalSnapshot(ctx context.Context, scope coordination.ExecutionScope, query coordination.DataQuery) (*coordination.DataSnapshot, error) {
	if scope.TargetDeviceID != p.ownerID || !scope.Coordinated || scope.CoreID == "" || query.ConversationID == "" && query.HistoricalRoleID == "" {
		return nil, fmt.Errorf("历史快照范围无效 target=%q owner=%q coordinated=%t: %w", scope.TargetDeviceID, p.ownerID, scope.Coordinated, coordination.ErrWrongOwner)
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
		return nil, nil
	}
	if query.HistoricalRoleID != "" && query.HistoricalRoleID != storedRole {
		return nil, coordination.ErrRoleRequired
	}
	historical := scope
	historical.ResourceOwnerID = p.ownerID
	historical.RoleOwnerID = scope.RoleOwnerID
	historical.RoleID = storedRole
	historical.MemoryRoleID = storedRole
	historical.MemoryViewAll = false
	role := coordination.Role{ID: storedRole, Name: "设备历史角色", Revision: 1, Profile: json.RawMessage(`{}`)}
	historical.RoleRevision = role.Revision
	snapshotScope := historical
	snapshotScope.Coordinated = false
	snapshot, err := p.snapshotWithRole(ctx, snapshotScope, query, role)
	if err != nil {
		return nil, fmt.Errorf("读取历史角色快照失败 role=%q roleOwner=%q resourceOwner=%q: %w", historical.RoleID, historical.RoleOwnerID, historical.ResourceOwnerID, err)
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
		return coordination.Acknowledgement{}, fmt.Errorf("校验设备资源写入权限失败: %w", err)
	}
	ctx = coordination.WithoutAdditionalGuard(ctx)
	unlock := character.LockRuntimeRole(p.services.DB)
	defer unlock()
	if commit.Scope.RoleOwnerID == commit.Scope.CoreID && commit.Scope.CoreID != p.ownerID {
		if commit.MemoryTransfer != nil && commit.MemoryTransfer.ToRoleID != coordination.GlobalMemoryRole {
			return coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
	} else {
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
	}
	var ack coordination.Acknowledgement
	err := coordination.CommitCurrent(ctx, func() error {
		var err error
		ack, err = p.store.Apply(ctx, commit)
		if err != nil {
			return fmt.Errorf("写入设备资源存储失败: %w", err)
		}
		return err
	})
	if err != nil {
		return ack, fmt.Errorf("提交设备资源事务失败: %w", err)
	}
	return ack, err
}

func (p *meshLocalDataPort) Resource(ctx context.Context, scope coordination.ExecutionScope, kind, id string) (*coordination.Resource, error) {
	if scope.ResourceOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	if !(scope.RoleOwnerID == scope.CoreID && scope.CoreID != p.ownerID) {
		if err := coordination.ValidateRoleRevision(ctx, p, scope); err != nil {
			return nil, err
		}
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
