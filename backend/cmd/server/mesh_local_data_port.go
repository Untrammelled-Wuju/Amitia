package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/profile"
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
	return &meshLocalDataPort{services: services, ownerID: ownerID, legacySpaceID: space.SpaceID(), roles: character.NewRepository(app.NewAppContext(services.DB, nil)), store: coordination.NewOwnershipStore(services.KernelContainer.DeviceRegistry.Database(), ownerID)}, nil
}

func (p *meshLocalDataPort) Roles(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Role, error) {
	unlock := character.LockRuntimeRole(p.services.DB)
	defer unlock()
	return p.rolesLocked(ctx, scope)
}

func (p *meshLocalDataPort) rolesLocked(ctx context.Context, scope coordination.ExecutionScope) ([]coordination.Role, error) {
	if scope.RoleOwnerID != p.ownerID || scope.ResourceOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	characters, err := p.roles.List(false)
	if err != nil {
		return nil, err
	}
	roles := make([]coordination.Role, 0, len(characters))
	for _, row := range characters {
		if row.SpaceID != "" && row.SpaceID != "default" && row.SpaceID != p.legacySpaceID {
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
		roles = append(roles, coordination.Role{ID: row.ID, Name: row.Name, Revision: row.Revision, Profile: body})
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
	snapshot := coordination.DataSnapshot{OwnerID: p.ownerID, Role: role, Resources: make([]coordination.Resource, 0), NextCursors: make(map[string]string)}
	usableMemories := make(map[string]bool)
	for _, kind := range []string{"memory", "conversation", "message", "summary", "working", "profile", "episodic", "fact", "vector", "graph", "continuity", "checkpoint", "project"} {
		if kind == "project" && query.ResourceKind != "project" {
			continue
		}
		if query.Management && query.Cursor == "" && query.LegacyCursor != "" {
			continue
		}
		if query.ResourceKind != "" && query.ResourceKind != kind {
			continue
		}
		resources, nextCursor, err := p.store.ListPage(ctx, kind, scope.RoleID, query)
		if err != nil {
			return snapshot, err
		}
		if nextCursor != "" {
			snapshot.NextCursors[kind] = nextCursor
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
				if _, found := usableMemories[resource.SourceID]; !found {
					source, err := p.store.Get(ctx, "memory", resource.SourceID)
					if err != nil {
						return snapshot, err
					}
					usableMemories[resource.SourceID] = source != nil && !source.Deleted && source.RoleID == scope.RoleID && coordination.ResourceUsable(source.Body, time.Now())
					if usableMemories[resource.SourceID] {
						snapshot.Resources = append(snapshot.Resources, *source)
					}
				}
				if !usableMemories[resource.SourceID] {
					continue
				}
			}
			if kind == "conversation" && !query.ListConversations && resource.ID != query.ConversationID {
				continue
			}
			if kind == "message" || kind == "summary" || kind == "working" {
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
	if query.ResourceKind == "" && len(query.Vector) > 0 {
		relevant, err := p.semanticSearch(ctx, scope.RoleID, query)
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
	if query.Management && !scope.Coordinated {
		if err := p.historicalMemoryManagement(ctx, scope, scope.RoleID, query, &snapshot); err != nil {
			return snapshot, err
		}
	}
	if query.Management || query.ResourceKind == "continuity" || (query.Cursor != "" && query.LegacyCursor == "") {
		return snapshot, coordination.ValidateSnapshot(scope, snapshot)
	}
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
		snapshot.LegacyConversations, err = appendMeshJSON(conversations)
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
			snapshot.LegacyMessages, err = appendMeshJSON(messages)
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
					snapshot.LegacySummary, err = json.Marshal(summary)
					if err != nil {
						return snapshot, err
					}
				}
			}
		}
	}
	if query.ResourceKind == "message" || query.ResourceKind == "conversation" {
		return snapshot, coordination.ValidateSnapshot(scope, snapshot)
	}
	if p.services.DataLifecycle == nil || !p.services.DataLifecycle.IsRetrievalBlocked(scope.RoleID) {
		var memories []memory.Memory
		if err := db.Where("character_id=? AND (space_id=? OR space_id='' OR space_id='default') AND verified_status NOT IN ('tombstone','replaced','invalidated','deleted') AND (allow_context_use IS NULL OR allow_context_use=1) AND archived_at IS NULL", scope.RoleID, p.legacySpaceID).Order("importance DESC,updated_at DESC").Limit(limit).Find(&memories).Error; err != nil {
			return snapshot, err
		}
		live := make([]memory.Memory, 0, len(memories))
		for _, item := range memories {
			if meshMemoryUsable(item, time.Now()) {
				live = append(live, item)
			}
		}
		snapshot.LegacyMemories, err = appendMeshJSON(live)
		if err != nil {
			return snapshot, err
		}
		var profiles []profile.UserProfile
		if err := db.Where("character_id=? AND (space_id=? OR space_id='' OR space_id='default') AND projection_status='active'", scope.RoleID, p.legacySpaceID).Order("confidence DESC,updated_at DESC").Limit(limit).Find(&profiles).Error; err != nil {
			return snapshot, err
		}
		snapshot.LegacyProfiles, err = appendMeshJSON(profiles)
		if err != nil {
			return snapshot, err
		}
		var episodes []episodic.EpisodicMemory
		if err := db.Where("character_id=? AND (space_id=? OR space_id='' OR space_id='default') AND archived_at IS NULL AND decay_state NOT IN ('forgotten','deleted')", scope.RoleID, p.legacySpaceID).Order("created_at DESC").Limit(limit).Find(&episodes).Error; err != nil {
			return snapshot, err
		}
		snapshot.LegacyEpisodes, err = appendMeshJSON(episodes)
		if err != nil {
			return snapshot, err
		}
	}
	return snapshot, coordination.ValidateSnapshot(scope, snapshot)
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
		var raw json.RawMessage
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
	return result, nil
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
	if query.Management && query.HistoricalRoleID != "" {
		if err := p.historicalMemoryManagement(ctx, scope, storedRole, query, &snapshot); err != nil {
			return nil, err
		}
	}
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
	if resource != nil && resource.RoleID != scope.RoleID {
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

func meshMemoryUsable(item memory.Memory, now time.Time) bool {
	if item.AllowContextUse != nil && !*item.AllowContextUse {
		return false
	}
	if item.ArchivedAt != nil && *item.ArchivedAt != "" {
		return false
	}
	if item.ExpiresAt == nil || *item.ExpiresAt == "" {
		return true
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
		if expires, err := time.ParseInLocation(layout, *item.ExpiresAt, time.Local); err == nil {
			return expires.After(now)
		}
	}
	return false
}
