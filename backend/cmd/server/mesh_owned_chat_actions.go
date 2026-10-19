package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
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
		start := min((page-1)*size, len(items))
		return map[string]any{"items": items[start:min(start+size, len(items))], "total": len(items), "page": page, "pageSize": size}, nil
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
		encodedInput, _ := json.Marshal([]string{request.Title, request.ProjectID, request.Channel, request.PeerID})
		inputHash := sha256.Sum256(encodedInput)
		fingerprint := hex.EncodeToString(inputHash[:])
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(p.ownerID+"\x00"+role+"\x00"+request.RequestID)).String()
		for _, row := range records {
			if row.Kind == "conversation" && row.ID == id {
				if chatManagementString(row.Body, "creationRequestId") != request.RequestID || chatManagementString(row.Body, "creationFingerprint") != fingerprint {
					return nil, coordination.ErrRequestConflict
				}
				return row.Body, nil
			}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		title := strings.TrimSpace(request.Title)
		if title == "" {
			title = "新对话"
		}
		document := map[string]any{"id": id, "conversationId": id, "characterId": role, "title": title, "channel": request.Channel, "peerId": request.PeerID, "source": "agent_tool", "createdAt": now, "updatedAt": now, "creationRequestId": request.RequestID}
		document["creationFingerprint"] = fingerprint
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
		current.RequestID = "create-chat/" + hex.EncodeToString(digest[:])
		_, err = p.store.Apply(ctx, coordination.Commit{Scope: current, Mutations: []coordination.Mutation{{Kind: "conversation", ID: id, RoleID: role, Body: encoded}}})
		return document, err
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
		if target.Legacy {
			result := p.services.DB.WithContext(ctx).Model(&chat.Conversation{}).Where("id=? AND revision=? AND deleted_at IS NULL", target.ID, target.Revision).Updates(map[string]any{"title": title, "revision": target.Revision + 1, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)})
			if result.Error != nil {
				return nil, result.Error
			}
			if result.RowsAffected != 1 {
				return nil, coordination.ErrResourceVersion
			}
		} else {
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
		}
		target.Body["title"] = title
		return target.Body, nil
	}
	return nil, errors.New("未知会话操作")
}
