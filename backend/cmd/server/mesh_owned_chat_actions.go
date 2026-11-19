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
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/pkg/util"
)

func (p *meshLocalDataPort) managedConversationAction(ctx context.Context, scope coordination.ExecutionScope, role string, request coordination.ChatManagementRequest, records, selected []ownedChatManagementRecord) (map[string]any, error) {
	if request.Operation == "conversations" {
		items := make([]map[string]any, 0)
		for _, row := range selected {
			if row.Kind == "conversation" && (request.ProjectID == "" || chatManagementString(row.Body, "projectId") == request.ProjectID) && (request.Channel == "" || chatManagementString(row.Body, "channel") == request.Channel) {
				items = append(items, row.Body)
			}
		}
		sort.SliceStable(items, func(i, j int) bool {
			return chatManagementString(items[i], "updatedAt") > chatManagementString(items[j], "updatedAt")
		})
		page, size := max(request.Page, 1), min(max(request.PageSize, 1), 128)
		if request.PageSize == 0 {
			size = 50
		}
		start, end := util.PageBounds(len(items), page, size)
		return map[string]any{"items": items[start:end], "total": len(items), "page": page, "pageSize": size}, nil
	}
	if request.RequestID == "" || len(request.RequestID) > 128 || len(request.Title) > 1024 || strings.ContainsRune(request.Title, 0) {
		return nil, errors.New("会话管理请求参数无效")
	}
	var target *ownedChatManagementRecord
	for index := range selected {
		if selected[index].Kind == "conversation" && selected[index].ID == request.ConversationID {
			target = &selected[index]
			break
		}
	}
	if request.Operation == "conversation/create" {
		return p.createManagedConversation(ctx, scope, role, request, records, request.Draft)
	}
	if target == nil {
		return nil, errors.New("会话不存在或不属于当前角色")
	}
	switch request.Operation {
	case "conversation/read":
		return target.Body, nil
	case "conversation/delete":
		plan := ownedChatCleanupPlan{Scope: scope, Owner: p.ownerID, Role: role, Records: selected, IncludeMemories: true}
		if err := p.deleteManagedChat(ctx, scope, role, plan); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true, "conversationId": target.ID}, nil
	case "conversation/rename":
		title := strings.TrimSpace(request.Title)
		if title == "" {
			return nil, errors.New("会话名称不能为空")
		}
		if chatManagementString(target.Body, "title") == title {
			return target.Body, nil
		}
		target.Body["title"], target.Body["updatedAt"] = title, time.Now().UTC().Format(time.RFC3339Nano)
		encoded, err := json.Marshal(target.Body)
		if err != nil {
			return nil, err
		}
		current := scope
		digest := sha256.Sum256([]byte(request.RequestID))
		current.RequestID = "rename-chat/" + hex.EncodeToString(digest[:])
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "conversation", ID: target.ID, RoleID: role, ExpectedRevision: target.Revision, Body: encoded}}}); err != nil {
			return nil, err
		}
		target.Body["title"] = title
		return target.Body, nil
	case "conversation/archive":
		if request.Archived == nil {
			return nil, errors.New("会话归档参数无效")
		}
		archivedAt := ""
		if *request.Archived {
			archivedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		target.Body["archived"], target.Body["archivedAt"], target.Body["updatedAt"] = *request.Archived, archivedAt, time.Now().UTC().Format(time.RFC3339Nano)
		encoded, err := json.Marshal(target.Body)
		if err != nil {
			return nil, err
		}
		current := scope
		digest := sha256.Sum256([]byte(request.RequestID))
		current.RequestID = "archive-chat/" + hex.EncodeToString(digest[:])
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "conversation", ID: target.ID, RoleID: role, ExpectedRevision: target.Revision, Body: encoded}}}); err != nil {
			return nil, err
		}
		return target.Body, nil
	case "conversation/pin":
		if request.Pinned == nil {
			return nil, errors.New("会话置顶参数无效")
		}
		pinnedAt := ""
		if *request.Pinned {
			pinnedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		target.Body["pinned"], target.Body["pinnedAt"], target.Body["updatedAt"] = *request.Pinned, pinnedAt, time.Now().UTC().Format(time.RFC3339Nano)
		encoded, err := json.Marshal(target.Body)
		if err != nil {
			return nil, err
		}
		current := scope
		digest := sha256.Sum256([]byte(request.RequestID))
		current.RequestID = "pin-chat/" + hex.EncodeToString(digest[:])
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "conversation", ID: target.ID, RoleID: role, ExpectedRevision: target.Revision, Body: encoded}}}); err != nil {
			return nil, err
		}
		return target.Body, nil
	case "conversation/reorder":
		return p.reorderManagedConversation(ctx, scope, role, request, records)
	case "conversation/fork":
		rows, err := p.store.List(ctx, "message", role, false)
		if err != nil {
			return nil, err
		}
		forkID := uuid.NewString()
		now := time.Now().UTC().Format(time.RFC3339Nano)
		mutations := make([]coordination.Mutation, 0, len(rows)+1)
		index := 0
		for _, row := range rows {
			source := map[string]any{}
			if err := json.Unmarshal(row.Body, &source); err != nil {
				return nil, err
			}
			if chatManagementString(source, "conversationId") != target.ID {
				continue
			}
			body := map[string]any{}
			for key, value := range source {
				body[key] = value
			}
			messageID := fmt.Sprintf("%s-message-%06d", forkID, index)
			body["id"], body["conversationId"], body["characterId"], body["ownerId"] = messageID, forkID, role, p.ownerID
			body["forkOrigin"] = map[string]any{"conversationId": target.ID, "messageId": chatManagementString(source, "id")}
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			mutations = append(mutations, coordination.Mutation{Kind: "message", ID: messageID, RoleID: role, Body: encoded})
			index++
		}
		conversation := map[string]any{}
		for key, value := range target.Body {
			conversation[key] = value
		}
		title := strings.TrimSpace(request.Title)
		if title == "" {
			title = chatManagementString(target.Body, "title")
			if title == "" {
				title = "对话"
			}
			title += " (fork)"
		}
		conversation["id"], conversation["conversationId"], conversation["characterId"], conversation["ownerId"] = forkID, forkID, role, p.ownerID
		conversation["title"], conversation["createdAt"], conversation["updatedAt"] = title, now, now
		conversation["messageCount"] = index
		conversation["forkOrigin"] = map[string]any{"conversationId": target.ID}
		encoded, err := json.Marshal(conversation)
		if err != nil {
			return nil, err
		}
		mutations = append(mutations, coordination.Mutation{Kind: "conversation", ID: forkID, RoleID: role, Body: encoded})
		current := scope
		digest := sha256.Sum256([]byte(request.RequestID))
		current.RequestID = "fork-chat/" + hex.EncodeToString(digest[:])
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: mutations}); err != nil {
			return nil, err
		}
		return map[string]any{"conversationId": forkID, "forkedFrom": target.ID, "title": title, "messageCount": index}, nil
	}
	return nil, errors.New("未知会话操作")
}

func (p *meshLocalDataPort) nextConversationOrder(ctx context.Context, role string) (int64, error) {
	rows, err := p.store.List(ctx, "conversation", role, false)
	if err != nil {
		return 0, fmt.Errorf("读取会话排序失败: %w", err)
	}
	maxOrder := int64(0)
	for _, row := range rows {
		if row.Deleted {
			continue
		}
		var document map[string]any
		if err := json.Unmarshal(row.Body, &document); err != nil {
			return 0, fmt.Errorf("会话数据格式无效: %w", err)
		}
		order, _ := document["order"].(float64)
		if candidate := conversationSortOrder(int64(order), chatManagementString(document, "createdAt")); candidate > maxOrder {
			maxOrder = candidate
		}
	}
	return maxOrder + 1024, nil
}

func conversationSortOrder(order int64, createdAt string) int64 {
	if order > 0 {
		return order
	}
	if parsed, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		return parsed.UnixMilli()
	}
	return 1
}

func (p *meshLocalDataPort) createManagedConversation(ctx context.Context, scope coordination.ExecutionScope, role string, request coordination.ChatManagementRequest, records []ownedChatManagementRecord, draft bool) (map[string]any, error) {
	encodedInput, _ := json.Marshal([]string{request.Title, request.ProjectID, request.Channel, request.PeerID})
	inputHash := sha256.Sum256(encodedInput)
	fingerprint := hex.EncodeToString(inputHash[:])
	id := strings.TrimSpace(request.ConversationID)
	if id == "" {
		id = uuid.NewSHA1(uuid.NameSpaceOID, []byte(p.ownerID+"\x00"+role+"\x00"+request.RequestID)).String()
	}
	if strings.ContainsRune(id, 0) || len(id) > 512 {
		return nil, coordination.ErrWrongOwner
	}
	for _, row := range records {
		if row.Kind != "conversation" || row.ID != id {
			continue
		}
		if chatManagementString(row.Body, "creationRequestId") != request.RequestID || chatManagementString(row.Body, "creationFingerprint") != fingerprint {
			return nil, coordination.ErrRequestConflict
		}
		if draft || !chatManagementBool(row.Body, "draft") {
			return row.Body, nil
		}
		body := map[string]any{}
		for key, value := range row.Body {
			body[key] = value
		}
		delete(body, "draft")
		now := time.Now().UTC().Format(time.RFC3339Nano)
		body["publishedAt"], body["updatedAt"] = now, now
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		current := scope
		digest := sha256.Sum256([]byte(request.RequestID))
		current.RequestID = "publish-chat/" + hex.EncodeToString(digest[:])
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "conversation", ID: id, RoleID: role, ExpectedRevision: row.Revision, Body: encoded}}}); err != nil {
			return nil, err
		}
		return body, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = "新对话"
	}
	nextOrder, err := p.nextConversationOrder(ctx, role)
	if err != nil {
		return nil, err
	}
	document := map[string]any{"id": id, "conversationId": id, "characterId": role, "title": title, "channel": request.Channel, "peerId": request.PeerID, "source": "agent_tool", "createdAt": now, "updatedAt": now, "order": nextOrder, "creationRequestId": request.RequestID}
	document["creationFingerprint"] = fingerprint
	if draft {
		document["draft"], document["preparedAt"] = true, now
	}
	if request.ProjectID != "" {
		project, err := p.store.Get(ctx, "project", request.ProjectID)
		if err != nil {
			return nil, err
		}
		if project == nil || project.Deleted || project.RoleID != role {
			return nil, errors.New("项目不存在或不属于当前角色")
		}
		document["projectId"] = request.ProjectID
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	current := scope
	digest := sha256.Sum256([]byte(request.RequestID))
	prefix := "create-chat/"
	if draft {
		prefix = "draft-chat/"
	}
	current.RequestID = prefix + hex.EncodeToString(digest[:])
	_, err = p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "conversation", ID: id, RoleID: role, Body: encoded}}})
	return document, err
}

func conversationOrderOf(row map[string]any) int64 {
	var order int64
	switch value := row["order"].(type) {
	case float64:
		order = int64(value)
	case int64:
		order = value
	case int:
		order = int64(value)
	}
	return conversationSortOrder(order, chatManagementString(row, "createdAt"))
}

func (p *meshLocalDataPort) reorderManagedConversation(ctx context.Context, scope coordination.ExecutionScope, role string, request coordination.ChatManagementRequest, records []ownedChatManagementRecord) (map[string]any, error) {
	projectID := strings.TrimSpace(request.ProjectID)
	byID := map[string]*ownedChatManagementRecord{}
	entries := make([]orderPlanEntry, 0, len(records))
	found := false
	for index := range records {
		row := &records[index]
		if row.Kind != "conversation" {
			continue
		}
		byID[row.ID] = row
		if chatManagementString(row.Body, "projectId") != projectID {
			continue
		}
		if chatManagementBool(row.Body, "draft") || chatManagementBool(row.Body, "archived") || chatManagementString(row.Body, "archivedAt") != "" {
			continue
		}
		entries = append(entries, orderPlanEntry{ID: row.ID, Order: conversationOrderOf(row.Body)})
		found = found || row.ID == request.ConversationID
	}
	if !found {
		return nil, errors.New("会话不在可排序范围")
	}
	plan, err := planSparseOrder(entries, request.ConversationID, strings.TrimSpace(request.BeforeConversationID), orderPlanStep)
	if err != nil {
		return nil, err
	}
	for index, change := range plan {
		row := byID[change.ID]
		if row == nil {
			return nil, fmt.Errorf("会话 %q 不在当前角色内", change.ID)
		}
		if chatManagementString(row.Body, "projectId") != projectID {
			return nil, fmt.Errorf("会话 %q 不属于当前工作区", change.ID)
		}
		document := make(map[string]any, len(row.Body)+1)
		for key, value := range row.Body {
			document[key] = value
		}
		document["order"] = change.Order
		encoded, err := json.Marshal(document)
		if err != nil {
			return nil, err
		}
		current := scope
		digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d", request.RequestID, index)))
		current.RequestID = "reorder-chat/" + hex.EncodeToString(digest[:])
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "conversation", ID: row.ID, RoleID: role, ExpectedRevision: row.Revision, Body: encoded}}}); err != nil {
			return nil, err
		}
		row.Revision++
		row.Body = document
	}
	ordered := make([]orderPlanEntry, 0, len(entries))
	for _, entry := range entries {
		if row := byID[entry.ID]; row != nil {
			entry.Order = conversationOrderOf(row.Body)
		}
		ordered = append(ordered, entry)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Order != ordered[j].Order {
			return ordered[i].Order < ordered[j].Order
		}
		return ordered[i].ID < ordered[j].ID
	})
	ids := make([]string, 0, len(ordered))
	for _, entry := range ordered {
		ids = append(ids, entry.ID)
	}
	return map[string]any{"conversationIds": ids, "order": conversationOrderOf(byID[request.ConversationID].Body), "projectId": projectID}, nil
}
