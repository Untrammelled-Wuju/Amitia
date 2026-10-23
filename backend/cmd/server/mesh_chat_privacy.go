package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func privacyRecordFingerprint(row ownedChatManagementRecord) string {
	raw, _ := json.Marshal(row.Body)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (p *meshLocalDataPort) managedPrivacyMessages(ctx context.Context, scope coordination.ExecutionScope, role string, request coordination.ChatManagementRequest) (map[string]any, error) {
	records, err := p.privacyChatRecords(ctx)
	if err != nil {
		return nil, err
	}
	imported := map[string]bool{}
	for _, row := range records {
		if row.Kind == "conversation" {
			imported[fmtPrivacyKey(row.Legacy, row.ID)] = chatManagementString(row.Body, "source") == "import"
		}
	}
	fingerprint := chatManagementFingerprint(records)
	if request.PreviewID != "" && request.PreviewID != fingerprint {
		return nil, coordination.ErrResourceVersion
	}
	items := []coordination.PrivacyMessage{}
	for _, row := range records {
		if row.Kind != "message" {
			continue
		}
		raw, err := json.Marshal(row.Body)
		if err != nil {
			return nil, err
		}
		item := coordination.PrivacyMessage{ID: row.ID, RoleID: row.RoleID, OwnerID: p.ownerID, Legacy: row.Legacy, Revision: row.Revision, Fingerprint: privacyRecordFingerprint(row), Imported: chatManagementString(row.Body, "source") == "import" || imported[fmtPrivacyKey(row.Legacy, chatManagementString(row.Body, "conversationId"))], Body: raw, Scope: scope}
		if request.Operation == "privacy/messages" {
			items = append(items, item)
			continue
		}
		expected := request.PrivacyMessage
		if expected == nil || expected.ID != item.ID || expected.Legacy != item.Legacy {
			continue
		}
		if expected.OwnerID != p.ownerID || expected.RoleID != row.RoleID || expected.Revision != item.Revision || expected.Fingerprint != item.Fingerprint {
			return nil, coordination.ErrResourceVersion
		}
		if request.RequestID == "" {
			return nil, errors.New("脱敏请求编号不能为空")
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return nil, err
		}
		if row.Legacy {
			result := p.services.DB.WithContext(ctx).Model(&chat.Message{}).Where("id=? AND character_id=? AND conversation_id=? AND revision=? AND content=? AND deleted_at IS NULL", row.ID, row.RoleID, chatManagementString(row.Body, "conversationId"), row.Revision, chatManagementString(row.Body, "content")).Updates(map[string]any{"content": "[已脱敏]", "reasoning_content": "", "safety_level": "masked", "revision": row.Revision + 1})
			if result.Error != nil {
				return nil, result.Error
			}
			if result.RowsAffected != 1 {
				return nil, coordination.ErrResourceVersion
			}
		} else {
			stored, err := p.store.Get(ctx, "message", row.ID)
			if err != nil {
				return nil, err
			}
			if stored == nil || stored.Revision != row.Revision {
				return nil, coordination.ErrResourceVersion
			}
			row.Body = nil
			if err := json.Unmarshal(stored.Body, &row.Body); err != nil {
				return nil, err
			}
			row.Body["content"], row.Body["reasoningContent"], row.Body["safetyLevel"] = "[已脱敏]", "", "masked"
			row.Body["updatedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
			raw, _ = json.Marshal(row.Body)
			local := scope
			local.ResourceOwnerID, local.RoleOwnerID, local.RoleID, local.RequestID = p.ownerID, p.ownerID, row.RoleID, request.RequestID
			if _, err := p.store.Apply(ctx, coordination.Commit{Scope: local, Mutations: []coordination.Mutation{{Kind: "message", ID: row.ID, RoleID: row.RoleID, ExpectedRevision: row.Revision, Body: raw}}}); err != nil {
				return nil, err
			}
		}
		return map[string]any{"masked": true, "maskedCount": 1, "ownerId": p.ownerID}, nil
	}
	if request.Operation == "privacy/mask" {
		return nil, coordination.ErrResourceVersion
	}
	size := min(max(request.PageSize, 1), 64)
	start := min((max(request.Page, 1)-1)*size, len(items))
	end := min(start+size, len(items))
	return map[string]any{"items": items[start:end], "hasMore": end < len(items), "snapshotFingerprint": fingerprint}, nil
}

func fmtPrivacyKey(legacy bool, id string) string {
	if legacy {
		return "legacy/" + id
	}
	return "owned/" + id
}

func (p *meshLocalDataPort) privacyChatRecords(ctx context.Context) ([]ownedChatManagementRecord, error) {
	roles := map[string]bool{}
	rows, err := p.services.KernelContainer.DeviceRegistry.Database().QueryContext(ctx, `SELECT DISTINCT role_id FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND deleted=0`, p.ownerID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			rows.Close()
			return nil, err
		}
		roles[role] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var legacyRoles []string
	if err := p.services.DB.WithContext(ctx).Table("messages AS m").Distinct("m.character_id").Joins("JOIN conversations c ON c.id=m.conversation_id").Where("m.deleted_at IS NULL AND c.deleted_at IS NULL AND (c.space_id=? OR c.space_id='' OR c.space_id='default')", p.legacySpaceID).Pluck("m.character_id", &legacyRoles).Error; err != nil {
		return nil, err
	}
	for _, role := range legacyRoles {
		roles[role] = true
	}
	records := []ownedChatManagementRecord{}
	for role := range roles {
		rows, err := p.chatManagementRecords(ctx, role)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			rows[i].RoleID = role
		}
		records = append(records, rows...)
	}
	sort.Slice(records, func(i, j int) bool {
		left, right := records[i], records[j]
		if left.RoleID != right.RoleID {
			return left.RoleID < right.RoleID
		}
		if left.Legacy != right.Legacy {
			return left.Legacy
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.ID < right.ID
	})
	return records, nil
}
