package business

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
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type SummaryMessage struct {
	ID        string `json:"id"`
	OwnerID   string `json:"ownerId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	Revision  int64  `json:"revision"`
}

type SummaryInference struct {
	Scope          coordination.ExecutionScope
	Role           coordination.Role
	ConversationID string
	Messages       []SummaryMessage
}

type SummaryModel interface {
	GenerateOwnedSummary(context.Context, SummaryInference) (string, error)
}

type SummaryResponse struct {
	Text            string                       `json:"summaryText"`
	ConversationID  string                       `json:"conversationId"`
	ResourceID      string                       `json:"sourceResourceId"`
	Revision        int64                        `json:"sourceRevision"`
	OwnerID         string                       `json:"sourceOwnerId"`
	Scope           coordination.ExecutionScope  `json:"executionScope"`
	Saved           bool                         `json:"saved"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
}

type summaryReceipt struct {
	Hash         string           `json:"hash"`
	Status       string           `json:"status"`
	RoleRevision int64            `json:"roleRevision"`
	Response     *SummaryResponse `json:"response,omitempty"`
}

func summaryAuthority(scope coordination.ExecutionScope) coordination.ExecutionScope {
	scope.RequestID, scope.TurnID, scope.ExecutionID = "", "", ""
	return scope
}

func (e *Engine) summaryHistory(ctx context.Context, request Request, authority coordination.ExecutionScope) ([]SummaryMessage, []coordination.ResourceVersion, error) {
	rows := map[string]SummaryMessage{}
	dependencies := map[string]coordination.ResourceVersion{}
	query := coordination.DataQuery{ConversationID: request.ConversationID, ResourceKind: "message", Limit: 128}
	visited := map[string]bool{}
	currentDone, historicalDone := false, false
	bytes := 0
	for {
		result, err := e.Query(ctx, request, query)
		if err != nil {
			return nil, nil, err
		}
		if summaryAuthority(result.Scope) != summaryAuthority(authority) {
			return nil, nil, coordination.ErrScopeExpired
		}
		for index, snapshot := range []*coordination.DataSnapshot{&result.Snapshot, result.HistoricalSnapshot} {
			if index == 0 && currentDone || index == 1 && historicalDone {
				continue
			}
			if snapshot == nil {
				continue
			}
			expectedOwner := authority.ResourceOwnerID
			if index == 1 {
				expectedOwner = authority.TargetDeviceID
			}
			if snapshot.OwnerID == "" || snapshot.OwnerID != expectedOwner {
				return nil, nil, coordination.ErrWrongOwner
			}
			conversationID := result.ConversationID
			if index == 1 && result.ConversationOrigin != nil {
				conversationID = result.ConversationOrigin.ID
			}
			appendMessage := func(raw json.RawMessage, id string, revision int64) error {
				var item struct {
					ID, Role, Content, CreatedAt, Status string
					ConversationID                       string `json:"conversationId"`
					Transcription                        string `json:"transcription"`
					TranscriptionSourceContent           string `json:"transcriptionSourceContent"`
				}
				if json.Unmarshal(raw, &item) != nil {
					return errors.New("摘要历史消息格式无效")
				}
				if item.ConversationID != conversationID || conversationID == "" {
					return coordination.ErrWrongOwner
				}
				if item.Role != "user" && item.Role != "assistant" || item.Status == "failed" || item.Status == "sending" {
					return nil
				}
				if id == "" {
					id = item.ID
				}
				if id == "" || len(id) > 512 {
					return errors.New("摘要历史消息缺少编号")
				}
				if item.Transcription != "" && item.Content == item.TranscriptionSourceContent {
					item.Content = item.Transcription
				}
				message := SummaryMessage{ID: id, OwnerID: snapshot.OwnerID, Role: item.Role, Content: item.Content, CreatedAt: item.CreatedAt, Revision: revision}
				key := snapshot.OwnerID + "\x00" + id
				if previous, ok := rows[key]; ok {
					if previous != message {
						return coordination.ErrResourceVersion
					}
				} else {
					bytes += len(body(message))
				}
				if bytes > 8<<20 || len(rows) >= 4096 && rows[key].ID == "" {
					return coordination.ErrPendingLimit
				}
				rows[key] = message
				return nil
			}
			for _, raw := range snapshot.LegacyMessages {
				if err := appendMessage(raw, "", 0); err != nil {
					return nil, nil, err
				}
			}
			for _, resource := range snapshot.Resources {
				if resource.Kind != "message" || resource.Deleted {
					continue
				}
				if resource.OwnerID != snapshot.OwnerID || resource.RoleID != snapshot.Role.ID || resource.Revision < 1 {
					return nil, nil, coordination.ErrWrongOwner
				}
				if !coordination.ResourceUsable(resource.Body, time.Now()) {
					continue
				}
				if err := appendMessage(resource.Body, resource.ID, resource.Revision); err != nil {
					return nil, nil, err
				}
				if resource.OwnerID == authority.ResourceOwnerID {
					dependencies["message/"+resource.ID] = coordination.ResourceVersion{Kind: "message", ID: resource.ID, Revision: resource.Revision}
				}
			}
		}
		if len(rows) > 4096 {
			return nil, nil, coordination.ErrPendingLimit
		}
		current := result.Snapshot.NextCursors
		var historical map[string]string
		if result.HistoricalSnapshot != nil {
			historical = result.HistoricalSnapshot.NextCursors
		}
		if !currentDone {
			query.Cursor, query.LegacyCursor = current["message"], current["legacyMessage"]
		}
		if !historicalDone {
			query.HistoricalCursor, query.HistoricalLegacyCursor = historical["message"], historical["legacyMessage"]
		}
		currentDone = currentDone || query.Cursor == "" && query.LegacyCursor == ""
		historicalDone = historicalDone || query.HistoricalCursor == "" && query.HistoricalLegacyCursor == ""
		key := string(body([]string{query.Cursor, query.LegacyCursor, query.HistoricalCursor, query.HistoricalLegacyCursor}))
		if query.Cursor == "" && query.LegacyCursor == "" && query.HistoricalCursor == "" && query.HistoricalLegacyCursor == "" {
			break
		}
		if visited[key] {
			return nil, nil, errors.New("摘要历史分页游标重复")
		}
		visited[key] = true
	}
	messages := make([]SummaryMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, row)
	}
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].CreatedAt == messages[j].CreatedAt {
			return messages[i].OwnerID+"\x00"+messages[i].ID < messages[j].OwnerID+"\x00"+messages[j].ID
		}
		return messages[i].CreatedAt < messages[j].CreatedAt
	})
	if len(body(messages)) > 8<<20 {
		return nil, nil, coordination.ErrPendingLimit
	}
	versions := make([]coordination.ResourceVersion, 0, len(dependencies))
	for _, row := range dependencies {
		versions = append(versions, row)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].ID < versions[j].ID })
	return messages, versions, nil
}

func (e *Engine) GenerateSummary(ctx context.Context, request Request, expectedRevision int64) (SummaryResponse, error) {
	model, ok := e.model.(SummaryModel)
	if !ok {
		return SummaryResponse{}, errors.New("Core 摘要计算服务不可用")
	}
	if request.ConversationID == "" || request.RequestID == "" || len(request.RequestID) > 128 || request.ExpectedScope == nil || expectedRevision < 0 {
		return SummaryResponse{}, errors.New("摘要生成缺少会话、请求编号或页面版本")
	}
	if request.TargetDeviceID == "" {
		request.TargetDeviceID = request.DeviceID
	}
	unlock := e.lock(request.SpaceID + "\x00" + request.TargetDeviceID + "\x00" + request.ConversationID)
	defer unlock()
	first, err := e.Query(ctx, request, coordination.DataQuery{ConversationID: request.ConversationID, Limit: 128})
	if err != nil {
		return SummaryResponse{}, err
	}
	if summaryAuthority(first.Scope) != summaryAuthority(*request.ExpectedScope) {
		return SummaryResponse{}, coordination.ErrScopeExpired
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, first.Scope.RoleID, request.RequestID)
	if err != nil {
		return SummaryResponse{}, err
	}
	defer finish()
	scope.RoleRevision = first.Scope.RoleRevision
	if summaryAuthority(scope) != summaryAuthority(first.Scope) {
		return SummaryResponse{}, coordination.ErrScopeExpired
	}
	scope.TurnID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(scope.CoreID+"\x00summary\x00"+scope.InitiatorDeviceID+"\x00"+request.RequestID)).String()
	scope.ExecutionID = scope.TurnID
	ctx = coordination.WithScope(ctx, scope)
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return SummaryResponse{}, err
	}
	route, err := e.resolveStoredConversationRoute(ctx, scope, request.ConversationID, request.ConversationOrigin)
	if err != nil {
		return SummaryResponse{}, err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return SummaryResponse{}, errors.New("摘要所有者记录端口不可用")
	}
	resourceID := route.CurrentID + "/summary"
	digest := sha256.Sum256(body(map[string]any{"scope": summaryAuthority(scope), "conversationId": route.CurrentID, "origin": route.Origin, "revision": expectedRevision}))
	fingerprint := hex.EncodeToString(digest[:])
	receiptID := "summary/" + request.RequestID
	receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID)
	if err != nil {
		return SummaryResponse{}, err
	}
	if receipt != nil {
		var proof summaryReceipt
		if receipt.Deleted || json.Unmarshal(receipt.Body, &proof) != nil || proof.Hash != fingerprint || proof.RoleRevision != scope.RoleRevision {
			return SummaryResponse{}, coordination.ErrRequestConflict
		}
		if proof.Status == "completed" && proof.Response != nil {
			current, err := port.Resource(ctx, scope, "summary", resourceID)
			if err != nil {
				return SummaryResponse{}, err
			}
			if current == nil || current.Deleted || current.Revision != proof.Response.Revision {
				return SummaryResponse{}, coordination.ErrResourceVersion
			}
			if err := coordination.ValidateCurrent(ctx); err != nil {
				return SummaryResponse{}, err
			}
			if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
				return SummaryResponse{}, err
			}
			return *proof.Response, nil
		}
		return SummaryResponse{}, ErrUncertainExecution
	}
	current, err := port.Resource(ctx, scope, "summary", resourceID)
	if err != nil {
		return SummaryResponse{}, err
	}
	actual := int64(0)
	if current != nil {
		if current.ID != resourceID || current.OwnerID != scope.ResourceOwnerID || current.RoleID != scope.RoleID {
			return SummaryResponse{}, coordination.ErrWrongOwner
		}
		actual = current.Revision
	}
	if actual != expectedRevision {
		return SummaryResponse{}, coordination.ErrResourceVersion
	}
	messages, dependencies, err := e.summaryHistory(ctx, request, scope)
	if err != nil {
		return SummaryResponse{}, err
	}
	if len(messages) == 0 {
		return SummaryResponse{}, errors.New("会话没有可总结的消息")
	}
	conversation, err := port.Resource(ctx, scope, "conversation", route.CurrentID)
	if err != nil {
		return SummaryResponse{}, err
	}
	conversationRevision := int64(0)
	if conversation != nil {
		if conversation.Deleted || conversation.OwnerID != scope.ResourceOwnerID || conversation.RoleID != scope.RoleID {
			return SummaryResponse{}, coordination.ErrWrongOwner
		}
		conversationRevision = conversation.Revision
	}
	if conversationRevision > 0 {
		dependencies = append(dependencies, coordination.ResourceVersion{Kind: "conversation", ID: route.CurrentID, Revision: conversationRevision})
	}
	if len(dependencies) > 4096 {
		return SummaryResponse{}, coordination.ErrPendingLimit
	}
	startScope := scope
	startScope.RequestID += "|summary-start"
	claim, err := e.commit(ctx, coordination.Commit{Scope: startScope, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(summaryReceipt{Hash: fingerprint, Status: "started", RoleRevision: scope.RoleRevision})}}})
	if err != nil {
		return SummaryResponse{}, err
	}
	if claim.OwnerID != scope.ResourceOwnerID || claim.RequestID != startScope.RequestID || claim.Versions["checkpoint/"+receiptID] != 1 {
		return SummaryResponse{}, errors.New("摘要执行所有者尚未确认请求")
	}
	ctx, cancel := context.WithCancelCause(ctx)
	key := scope.SpaceID + "\x00" + scope.InitiatorDeviceID + "\x00" + scope.RequestID
	e.mu.Lock()
	e.active[key] = cancel
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.active, key); e.mu.Unlock(); cancel(nil) }()
	text, err := model.GenerateOwnedSummary(ctx, SummaryInference{Scope: scope, Role: first.Snapshot.Role, ConversationID: route.CurrentID, Messages: messages})
	if err != nil {
		return SummaryResponse{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 64<<10 {
		return SummaryResponse{}, errors.New("Core 摘要结果无效或超过上限")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return SummaryResponse{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return SummaryResponse{}, err
	}
	latest, _, err := e.summaryHistory(ctx, request, scope)
	if err != nil {
		return SummaryResponse{}, err
	}
	if string(body(latest)) != string(body(messages)) {
		return SummaryResponse{}, coordination.ErrResourceVersion
	}
	resultScope := scope
	resultScope.RequestID += "|summary-result"
	response := SummaryResponse{Text: text, ConversationID: route.CurrentID, ResourceID: resourceID, Revision: expectedRevision + 1, OwnerID: scope.ResourceOwnerID, Scope: scope, Saved: true, Acknowledgement: coordination.Acknowledgement{RequestID: resultScope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"summary/" + resourceID: expectedRevision + 1, "checkpoint/" + receiptID: 2}}}
	commit := coordination.Commit{Scope: resultScope, Dependencies: dependencies, Mutations: []coordination.Mutation{
		{Kind: "summary", ID: resourceID, RoleID: scope.RoleID, ExpectedRevision: expectedRevision, Body: body(map[string]any{"conversationId": route.CurrentID, "content": map[string]string{"summary": text}, "sourceMessages": messagesToSummarySources(messages), "updatedAt": time.Now().UTC().Format(time.RFC3339Nano), "executionScope": scope})},
		{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, ExpectedRevision: 1, Body: body(summaryReceipt{Hash: fingerprint, Status: "completed", RoleRevision: scope.RoleRevision, Response: &response})},
	}}
	if conversation == nil {
		commit.Mutations = append(commit.Mutations, coordination.Mutation{Kind: "conversation", ID: route.CurrentID, RoleID: scope.RoleID, Body: body(map[string]any{"id": route.CurrentID, "conversationId": route.CurrentID, "title": "历史对话", "characterId": scope.RoleID, "ownerId": scope.ResourceOwnerID, "conversationOrigin": route.Origin, "createdAt": time.Now().UTC().Format(time.RFC3339Nano)})})
	}
	ack, err := e.commit(ctx, commit)
	if err != nil {
		if !errors.Is(err, coordination.ErrResourceVersion) && !errors.Is(err, coordination.ErrRequestConflict) && !errors.Is(err, coordination.ErrScopeExpired) {
			if queueErr := e.coordination.Enqueue(ctx, commit); queueErr != nil {
				return SummaryResponse{}, errors.Join(err, queueErr)
			}
		}
		return SummaryResponse{}, err
	}
	if ack.OwnerID != response.OwnerID || ack.RequestID != resultScope.RequestID || ack.Versions["summary/"+resourceID] != response.Revision || ack.Versions["checkpoint/"+receiptID] != 2 {
		return SummaryResponse{}, errors.New("摘要所有者尚未确认保存")
	}
	response.Acknowledgement = ack
	return response, nil
}

func messagesToSummarySources(messages []SummaryMessage) []map[string]any {
	sources := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		digest := sha256.Sum256(body(message))
		sources = append(sources, map[string]any{"id": message.ID, "ownerId": message.OwnerID, "revision": message.Revision, "sha256": hex.EncodeToString(digest[:])})
	}
	return sources
}
