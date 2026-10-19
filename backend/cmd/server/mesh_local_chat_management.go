package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

type ownedChatManagementRecord struct {
	ID       string
	Kind     string
	RoleID   string
	Legacy   bool
	Revision int64
	Body     map[string]any
}

type ownedChatCleanupPlan struct {
	Scope           coordination.ExecutionScope
	Owner           string
	Role            string
	Records         []ownedChatManagementRecord
	IncludeMemories bool
	Expires         time.Time
	Fingerprint     string
}

var ownedChatCleanupPlans = struct {
	sync.Mutex
	Plans map[string]ownedChatCleanupPlan
}{Plans: make(map[string]ownedChatCleanupPlan)}

func chatManagementString(row map[string]any, key string) string {
	value, _ := row[key].(string)
	return value
}

func (p *meshLocalDataPort) chatManagementRecords(ctx context.Context, role string) ([]ownedChatManagementRecord, error) {
	db := p.services.KernelContainer.DeviceRegistry.Database()
	rows, err := db.QueryContext(ctx, `SELECT kind,resource_id,revision,body,role_id FROM kernel_device_owned_resources WHERE owner_id=? AND (role_id=? OR (kind='memory' AND json_extract(CAST(body AS TEXT),'$.executionScope.roleId')=?)) AND deleted=0 AND kind IN ('conversation','message','summary','memory','working') ORDER BY kind,resource_id`, p.ownerID, role, role)
	if err != nil {
		return nil, err
	}
	records := []ownedChatManagementRecord{}
	for rows.Next() {
		var row ownedChatManagementRecord
		var raw []byte
		if err := rows.Scan(&row.Kind, &row.ID, &row.Revision, &raw, &row.RoleID); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal(raw, &row.Body); err != nil {
			rows.Close()
			return nil, err
		}
		row.Body["id"] = row.ID
		records = append(records, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	legacyDB := p.services.DB.WithContext(ctx)
	var conversations []chat.Conversation
	if err := legacyDB.Where("deleted_at IS NULL AND (space_id=? OR space_id='' OR space_id='default') AND EXISTS (SELECT 1 FROM messages WHERE messages.conversation_id=conversations.id AND messages.character_id=? AND messages.deleted_at IS NULL)", p.legacySpaceID, role).Order("id").Find(&conversations).Error; err != nil {
		return nil, err
	}
	for _, conversation := range conversations {
		raw, _ := json.Marshal(conversation)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		records = append(records, ownedChatManagementRecord{ID: conversation.ID, Kind: "conversation", Legacy: true, Revision: conversation.Revision, Body: body})
		var messages []chat.Message
		if err := legacyDB.Where("conversation_id=? AND character_id=? AND deleted_at IS NULL", conversation.ID, role).Order("id").Find(&messages).Error; err != nil {
			return nil, err
		}
		for _, message := range messages {
			raw, _ := json.Marshal(message)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			records = append(records, ownedChatManagementRecord{ID: message.ID, Kind: "message", Legacy: true, Revision: message.Revision, Body: body})
		}
		var otherRoles int64
		if err := legacyDB.Model(&chat.Message{}).Where("conversation_id=? AND character_id<>? AND deleted_at IS NULL", conversation.ID, role).Count(&otherRoles).Error; err != nil {
			return nil, err
		}
		if otherRoles == 0 {
			var summaries []chat.ConversationSummary
			if err := legacyDB.Where("conversation_id=?", conversation.ID).Order("id").Find(&summaries).Error; err != nil {
				return nil, err
			}
			for _, summary := range summaries {
				raw, _ := json.Marshal(summary)
				var body map[string]any
				_ = json.Unmarshal(raw, &body)
				records = append(records, ownedChatManagementRecord{ID: fmt.Sprint(summary.ID), Kind: "summary", Legacy: true, Body: body})
			}
		}
	}
	return records, nil
}

func chatManagementFingerprint(records []ownedChatManagementRecord) string {
	raw, _ := json.Marshal(records)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func chatManagementSelected(records []ownedChatManagementRecord, request coordination.ChatManagementRequest, now time.Time) ([]ownedChatManagementRecord, error) {
	threshold := time.Time{}
	if request.BeforeDate != "" {
		var err error
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
			threshold, err = time.ParseInLocation(layout, request.BeforeDate, time.Local)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, errors.New("清理截止日期无效")
		}
	}
	if request.OlderThanDays > 0 {
		candidate := now.AddDate(0, 0, -request.OlderThanDays)
		if threshold.IsZero() || candidate.Before(threshold) {
			threshold = candidate
		}
	}
	contains := func(values []string, value string) bool {
		if len(values) == 0 {
			return true
		}
		for _, v := range values {
			if v == value {
				return true
			}
		}
		return false
	}
	ids := map[string]bool{}
	for _, row := range records {
		if row.Kind != "conversation" {
			continue
		}
		if len(request.ConversationIDs) > 0 && !contains(request.ConversationIDs, row.ID) {
			continue
		}
		if request.ConversationID != "" && row.ID != request.ConversationID {
			continue
		}
		if !contains(request.Channels, chatManagementString(row.Body, "channel")) || !contains(request.Sources, chatManagementString(row.Body, "source")) {
			continue
		}
		if !threshold.IsZero() {
			created := chatManagementString(row.Body, "createdAt")
			var parsed time.Time
			var err error
			for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
				parsed, err = time.ParseInLocation(layout, created, time.Local)
				if err == nil {
					break
				}
			}
			if err != nil || !parsed.Before(threshold) {
				continue
			}
		}
		ids[row.ID] = true
	}
	selected := []ownedChatManagementRecord{}
	for _, row := range records {
		if row.Kind == "conversation" && ids[row.ID] || row.Kind != "conversation" && ids[chatManagementString(row.Body, "conversationId")] {
			selected = append(selected, row)
		}
	}
	return selected, nil
}

func chatManagementCounts(records []ownedChatManagementRecord, includeMemories bool) map[string]any {
	conversations, messages, memories := 0, 0, 0
	ids := []string{}
	for _, row := range records {
		switch row.Kind {
		case "conversation":
			conversations++
			ids = append(ids, row.ID)
		case "message":
			messages++
		case "memory":
			if includeMemories {
				memories++
			}
		}
	}
	return map[string]any{"conversationCount": conversations, "messageCount": messages, "memoryCount": memories, "conversationIds": ids}
}

func (p *meshLocalDataPort) ManageChat(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChatManagementRequest) (map[string]any, error) {
	if request.ConversationOwnerID != p.ownerID {
		return nil, coordination.ErrWrongOwner
	}
	historical := scope.Coordinated && p.ownerID == scope.TargetDeviceID && p.ownerID != scope.ResourceOwnerID
	if !historical && (scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID) {
		return nil, coordination.ErrWrongOwner
	}
	role := scope.RoleID
	if historical {
		role = request.HistoricalRoleID
		if role == "" && request.ConversationID != "" {
			snapshot, err := p.HistoricalSnapshot(ctx, scope, coordination.DataQuery{ConversationID: request.ConversationID, Management: true, ResourceKind: "conversation", Limit: 1})
			if err != nil {
				return nil, err
			}
			if snapshot != nil {
				role = snapshot.Role.ID
			}
		}
		if role == "" {
			return nil, coordination.ErrRoleSelection
		}
	}
	var result map[string]any
	run := func() error { var err error; result, err = p.manageChatForRole(ctx, scope, role, request); return err }
	if historical {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return nil, err
		}
		if err := run(); err != nil {
			return nil, err
		}
	} else if err := p.WithSourceRole(ctx, scope, run); err != nil {
		return nil, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *meshLocalDataPort) manageChatForRole(ctx context.Context, scope coordination.ExecutionScope, role string, request coordination.ChatManagementRequest) (map[string]any, error) {
	records, err := p.chatManagementRecords(ctx, role)
	if err != nil {
		return nil, err
	}
	selected, err := chatManagementSelected(records, request, time.Now())
	if err != nil {
		return nil, err
	}
	if request.ConversationID != "" {
		found := false
		for _, row := range selected {
			found = found || row.Kind == "conversation" && row.ID == request.ConversationID
		}
		if !found {
			return nil, errors.New("会话不存在或不属于当前角色")
		}
	}
	switch request.Operation {
	case "feedback-stats", "feedback/list", "feedback/create", "feedback/delete":
		return p.managedChatFeedback(ctx, scope, request, records)
	case "conversations", "conversation/create", "conversation/read", "conversation/rename", "conversation/delete":
		return p.managedConversationAction(ctx, scope, role, request, records, selected)
	case "from-import":
		if request.ConversationID == "" || request.RequestID == "" {
			return nil, errors.New("导入会话和请求编号不能为空")
		}
		messages := []map[string]any{}
		var conversation map[string]any
		for _, row := range selected {
			if row.Kind == "conversation" && row.ID == request.ConversationID && row.Legacy && chatManagementString(row.Body, "source") == "import" {
				conversation = row.Body
			}
			if row.Kind == "message" && row.Legacy {
				messages = append(messages, row.Body)
			}
		}
		if conversation == nil {
			return nil, errors.New("未找到属于当前角色的导入会话")
		}
		sort.Slice(messages, func(i, j int) bool {
			a, _ := messages[i]["sequence"].(float64)
			b, _ := messages[j]["sequence"].(float64)
			if a == b {
				return chatManagementString(messages[i], "id") < chatManagementString(messages[j], "id")
			}
			return a < b
		})
		result := map[string]any{"conversation": conversation, "messages": messages, "sourceOwnerId": p.ownerID, "sourceRoleId": role}
		raw, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		if len(raw) > 3<<20 {
			return nil, coordination.ErrPendingLimit
		}
		return result, nil
	case "stats":
		counts := chatManagementCounts(records, true)
		today := 0
		for _, row := range records {
			if row.Kind == "message" && strings.HasPrefix(chatManagementString(row.Body, "createdAt"), time.Now().Format("2006-01-02")) {
				today++
			}
		}
		bytes, err := p.chatManagementDatabaseSize(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"totalConversations": counts["conversationCount"], "totalMessages": counts["messageCount"], "todayMessages": today, "dbSize": bytes, "dbSizeFormatted": fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))}, nil
	case "cleanup/preview":
		id := uuid.NewString()
		expires := time.Now().Add(10 * time.Minute)
		plan := ownedChatCleanupPlan{Scope: scope, Owner: p.ownerID, Role: role, Records: selected, IncludeMemories: request.IncludeMemories, Expires: expires, Fingerprint: chatManagementFingerprint(selected)}
		ownedChatCleanupPlans.Lock()
		for key, value := range ownedChatCleanupPlans.Plans {
			if time.Now().After(value.Expires) {
				delete(ownedChatCleanupPlans.Plans, key)
			}
		}
		ownedChatCleanupPlans.Plans[id] = plan
		ownedChatCleanupPlans.Unlock()
		result := chatManagementCounts(selected, request.IncludeMemories)
		result["previewId"] = id
		result["expiresAt"] = expires.Format(time.RFC3339)
		return result, nil
	case "cleanup/confirm":
		if strings.TrimSpace(request.ConfirmText) != "确认清理" {
			return nil, errors.New("确认文本不匹配")
		}
		ownedChatCleanupPlans.Lock()
		plan, ok := ownedChatCleanupPlans.Plans[request.PreviewID]
		if ok && plan.Owner == p.ownerID && plan.Role == role && sameChatManagementAuthority(plan.Scope, scope) {
			delete(ownedChatCleanupPlans.Plans, request.PreviewID)
		}
		ownedChatCleanupPlans.Unlock()
		if !ok || time.Now().After(plan.Expires) || plan.Owner != p.ownerID || plan.Role != role || !sameChatManagementAuthority(plan.Scope, scope) {
			return nil, errors.New("清理预览不存在、已过期或归属已变化")
		}
		ids := []string{}
		for _, row := range plan.Records {
			if row.Kind == "conversation" {
				ids = append(ids, row.ID)
			}
		}
		current, err := chatManagementSelected(records, coordination.ChatManagementRequest{ConversationIDs: ids}, time.Now())
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			current = nil
		}
		if chatManagementFingerprint(current) != plan.Fingerprint {
			return nil, errors.New("预览后会话数据已变化，请重新预览")
		}
		if err := p.deleteManagedChat(ctx, scope, role, plan); err != nil {
			return nil, err
		}
		return chatManagementCounts(plan.Records, plan.IncludeMemories), nil
	case "cleanup/vacuum":
		before, err := p.chatManagementDatabaseSize(ctx)
		if err != nil {
			return nil, err
		}
		if err := coordination.CommitCurrent(ctx, func() error {
			if err := p.services.DB.WithContext(ctx).Exec("VACUUM").Error; err != nil {
				return err
			}
			_, err := p.services.KernelContainer.DeviceRegistry.Database().ExecContext(ctx, "VACUUM")
			return err
		}); err != nil {
			return nil, err
		}
		after, err := p.chatManagementDatabaseSize(ctx)
		if err != nil {
			return nil, err
		}
		freed := max(int64(0), before-after)
		return map[string]any{"vacuumed": true, "freedBytes": freed, "freedFormatted": fmt.Sprintf("%.2f MB", float64(freed)/(1024*1024))}, nil
	case "message-range":
		items := []map[string]any{}
		for _, row := range selected {
			if row.Kind == "message" {
				items = append(items, row.Body)
			}
		}
		sort.SliceStable(items, func(i, j int) bool {
			return chatManagementString(items[i], "createdAt") < chatManagementString(items[j], "createdAt")
		})
		result := make([]map[string]any, 0)
		for index, item := range items {
			sequence, _ := item["sequence"].(float64)
			if sequence <= 0 {
				sequence = float64(index + 1)
			}
			if request.StartSequence > 0 && sequence < float64(request.StartSequence) || request.EndSequence > 0 && sequence > float64(request.EndSequence) {
				continue
			}
			result = append(result, item)
			if len(result) >= min(max(request.Limit, 1), 500) && request.Limit > 0 {
				break
			}
		}
		if request.Limit <= 0 && len(result) > 100 {
			result = result[:100]
		}
		return map[string]any{"items": result, "count": len(result)}, nil
	case "search":
		items := []map[string]any{}
		for _, row := range selected {
			if row.Kind == "message" && strings.Contains(strings.ToLower(chatManagementString(row.Body, "content")), strings.ToLower(request.Keyword)) {
				item := row.Body
				item["ownerId"] = p.ownerID
				items = append(items, item)
			}
		}
		sort.Slice(items, func(i, j int) bool {
			return chatManagementString(items[i], "createdAt") > chatManagementString(items[j], "createdAt")
		})
		page, size := max(request.Page, 1), request.PageSize
		if size < 1 || size > 128 {
			size = 50
		}
		start := min((page-1)*size, len(items))
		end := min(start+size, len(items))
		return map[string]any{"items": items[start:end], "total": len(items), "page": page, "pageSize": size}, nil
	case "export":
		if request.ConversationID == "" && len(request.ConversationIDs) == 0 {
			return nil, errors.New("请选择导出的会话")
		}
		messages := []map[string]any{}
		conversations := []map[string]any{}
		for _, row := range selected {
			if row.Kind == "conversation" {
				conversations = append(conversations, row.Body)
			}
			if row.Kind == "message" {
				messages = append(messages, row.Body)
			}
		}
		sort.Slice(messages, func(i, j int) bool {
			return chatManagementString(messages[i], "createdAt") < chatManagementString(messages[j], "createdAt")
		})
		content := ""
		mime, extension := "application/json; charset=utf-8", "json"
		if request.Format == "json" {
			raw, err := json.MarshalIndent(map[string]any{"ownerId": p.ownerID, "conversations": conversations, "messages": messages}, "", "  ")
			if err != nil {
				return nil, err
			}
			content = string(raw)
		} else {
			mime, extension = "text/markdown; charset=utf-8", "md"
			var builder strings.Builder
			for _, conversation := range conversations {
				builder.WriteString("# " + chatManagementString(conversation, "title") + "\n\n")
				for _, message := range messages {
					if chatManagementString(message, "conversationId") == chatManagementString(conversation, "id") {
						builder.WriteString("**" + chatManagementString(message, "role") + "**: " + chatManagementString(message, "content") + "\n\n")
					}
				}
			}
			content = builder.String()
		}
		if len(content) > 3<<20 {
			return nil, coordination.ErrPendingLimit
		}
		return map[string]any{"content": content, "fileName": "amitia-conversations-" + time.Now().Format("20060102-150405") + "." + extension, "mimeType": mime}, nil
	case "compression-status", "context-preview":
		if request.ConversationID == "" {
			return nil, errors.New("请选择会话")
		}
		return p.managedChatContext(ctx, scope, role, request, selected), nil
	default:
		return nil, errors.New("聊天管理操作无效")
	}
}

func sameChatManagementAuthority(a, b coordination.ExecutionScope) bool {
	a.RequestID, a.TurnID, a.ExecutionID = "", "", ""
	b.RequestID, b.TurnID, b.ExecutionID = "", "", ""
	return a == b
}

func (p *meshLocalDataPort) chatManagementDatabaseSize(ctx context.Context) (int64, error) {
	databases := []*gorm.DB{p.services.DB}
	total := int64(0)
	for _, db := range databases {
		var pages, size int64
		if err := db.WithContext(ctx).Raw("PRAGMA page_count").Scan(&pages).Error; err != nil {
			return 0, err
		}
		if err := db.WithContext(ctx).Raw("PRAGMA page_size").Scan(&size).Error; err != nil {
			return 0, err
		}
		total += pages * size
	}
	db := p.services.KernelContainer.DeviceRegistry.Database()
	var pages, size int64
	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pages); err != nil {
		return 0, err
	}
	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&size); err != nil {
		return 0, err
	}
	return total + pages*size, nil
}

func (p *meshLocalDataPort) managedChatContext(ctx context.Context, scope coordination.ExecutionScope, role string, request coordination.ChatManagementRequest, records []ownedChatManagementRecord) map[string]any {
	messages := []map[string]any{}
	summary := ""
	summaryCount, compressed := 0, 0
	title, lastAt := "", ""
	for _, row := range records {
		switch row.Kind {
		case "conversation":
			title = chatManagementString(row.Body, "title")
		case "message":
			messages = append(messages, row.Body)
		case "summary":
			summaryCount++
			text := chatManagementString(row.Body, "text")
			if text == "" {
				text = chatManagementString(row.Body, "summaryText")
			}
			summary = text
			lastAt = chatManagementString(row.Body, "compressedAt")
			if count, ok := row.Body["messageCount"].(float64); ok {
				compressed += int(count)
			} else {
				start, _ := row.Body["roundStart"].(float64)
				end, _ := row.Body["roundEnd"].(float64)
				if end >= start {
					compressed += int(end - start + 1)
				}
			}
		}
	}
	sort.Slice(messages, func(i, j int) bool {
		return chatManagementString(messages[i], "createdAt") < chatManagementString(messages[j], "createdAt")
	})
	if request.Operation == "compression-status" {
		return map[string]any{"totalRounds": len(messages), "compressedRounds": compressed, "lastCompressedAt": lastAt, "latestSummary": summary, "summaryCount": summaryCount}
	}
	tokens := len(summary) / 2
	for _, message := range messages {
		tokens += len(chatManagementString(message, "content")) / 2
	}
	recent := messages[max(0, len(messages)-10):]
	prompt := ""
	roles, err := p.rolesLocked(ctx, scope)
	if err == nil {
		for _, available := range roles {
			if available.ID == role {
				prompt = string(available.Profile)
				tokens += len(prompt) / 2
			}
		}
	}
	return map[string]any{"conversationId": request.ConversationID, "title": title, "characterId": role, "systemPromptPreview": prompt, "messageCount": len(messages), "recentMessages": recent, "estimatedTokens": tokens, "summary": summary}
}

func (p *meshLocalDataPort) deleteManagedChat(ctx context.Context, scope coordination.ExecutionScope, role string, plan ownedChatCleanupPlan) error {
	return coordination.CommitCurrent(ctx, func() error {
		adapted := scope
		adapted.ResourceOwnerID = p.ownerID
		adapted.RoleOwnerID = p.ownerID
		adapted.RoleID = role
		adapted.MemoryRoleID = role
		for _, row := range plan.Records {
			if row.Legacy || row.Kind != "conversation" {
				continue
			}
			adapted.RequestID = "chat-cleanup/" + plan.Fingerprint + "/" + row.ID
			_, err := p.store.Apply(ctx, coordination.Commit{Scope: adapted, Mutations: []coordination.Mutation{{Kind: "conversation", ID: row.ID, RoleID: role, ExpectedRevision: row.Revision, Deleted: true, KeepConversationMemories: !plan.IncludeMemories, Body: json.RawMessage(`null`)}}})
			if err != nil {
				return err
			}
		}
		if plan.IncludeMemories {
			for _, row := range plan.Records {
				if row.Legacy || row.Kind != "memory" {
					continue
				}
				current, err := p.store.Get(ctx, "memory", row.ID)
				if err != nil {
					return err
				}
				if current == nil || current.Deleted {
					continue
				}
				adapted.RequestID = "chat-cleanup-memory/" + plan.Fingerprint + "/" + row.ID
				adapted.MemoryRoleID = row.RoleID
				_, err = p.store.Apply(ctx, coordination.Commit{Scope: adapted, Mutations: []coordination.Mutation{{Kind: "memory", ID: row.ID, RoleID: row.RoleID, ExpectedRevision: row.Revision, Deleted: true, Body: json.RawMessage(`null`)}}})
				if err != nil {
					return err
				}
			}
		}
		selection := chat.ManagedHistorySelection{ConversationRevisions: map[string]int64{}, MessageRevisions: map[string]int64{}}
		feedbackKeys := []string{}
		for _, row := range plan.Records {
			if !row.Legacy {
				if row.Kind == "message" {
					feedbackKeys = append(feedbackKeys, ownedFeedbackMessageKey(p.ownerID, role, row.ID))
				}
				continue
			}
			if row.Kind == "conversation" {
				selection.ConversationRevisions[row.ID] = row.Revision
			}
			if row.Kind == "message" {
				selection.MessageRevisions[row.ID] = row.Revision
			}
		}
		if len(feedbackKeys) > 0 {
			if err := p.services.DB.WithContext(ctx).Exec("DELETE FROM message_feedback WHERE message_id IN ?", feedbackKeys).Error; err != nil {
				return err
			}
		}
		return chat.DeleteOwnedManagedHistory(ctx, p.services.DB, p.services.Chat, p.legacySpaceID, role, selection)
	})
}
