package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

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
			imported[fmtPrivacyKey(row.ID)] = chatManagementString(row.Body, "source") == "import"
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
		item := coordination.PrivacyMessage{ID: row.ID, RoleID: row.RoleID, OwnerID: p.ownerID, Revision: row.Revision, Fingerprint: privacyRecordFingerprint(row), Imported: chatManagementString(row.Body, "source") == "import" || imported[fmtPrivacyKey(chatManagementString(row.Body, "conversationId"))], Body: raw, Scope: scope}
		if request.Operation == "privacy/messages" {
			items = append(items, item)
			continue
		}
		expected := request.PrivacyMessage
		if expected == nil || expected.ID != item.ID {
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

func fmtPrivacyKey(id string) string {
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
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.ID < right.ID
	})
	return records, nil
}
