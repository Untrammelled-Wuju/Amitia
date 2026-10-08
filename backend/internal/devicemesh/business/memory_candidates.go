package business

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type MemoryCandidateModel interface {
	ExtractOwnedMemoryCandidates(context.Context, Inference, Generation) ([]DerivedMemory, error)
}

type MemoryCandidateInput struct {
	Action                   string `json:"action"`
	ID                       string `json:"id,omitempty"`
	ExpectedRevision         int64  `json:"expectedRevision,omitempty"`
	Key                      string `json:"key,omitempty"`
	Value                    string `json:"value,omitempty"`
	MemoryType               string `json:"memoryType,omitempty"`
	Importance               int    `json:"importance,omitempty"`
	ConversationID           string `json:"conversationId,omitempty"`
	ConflictID               string `json:"conflictId,omitempty"`
	ExpectedConflictRevision int64  `json:"expectedConflictRevision,omitempty"`
	Resolution               string `json:"resolution,omitempty"`
}

type OwnedMemoryCandidate struct {
	Type               string                         `json:"type"`
	ID                 string                         `json:"id"`
	Key                string                         `json:"key"`
	Value              string                         `json:"value"`
	MemoryType         string                         `json:"memoryType"`
	Importance         int                            `json:"importance"`
	State              string                         `json:"state"`
	DerivedKind        string                         `json:"derivedKind"`
	Derived            json.RawMessage                `json:"derived"`
	ConversationID     string                         `json:"conversationId"`
	ConversationOrigin *ConversationOrigin            `json:"conversationOrigin,omitempty"`
	HistoricalRoleID   string                         `json:"historicalRoleId,omitempty"`
	ProvenanceHash     string                         `json:"provenanceHash"`
	Dependencies       []coordination.ResourceVersion `json:"dependencies"`
	CreatedAt          string                         `json:"createdAt"`
	ExecutionScope     coordination.ExecutionScope    `json:"executionScope"`
}

func (e *Engine) memoryCandidateCommit(ctx context.Context, scope coordination.ExecutionScope, mutations []coordination.Mutation, dependencies []coordination.ResourceVersion, receipt string, receiptVersion int64, fingerprint string) (MemoryManagementResponse, error) {
	response := MemoryManagementResponse{Scope: scope, Saved: true, Resources: []coordination.Resource{}, Acknowledgement: coordination.Acknowledgement{OwnerID: scope.ResourceOwnerID, RequestID: scope.RequestID, Versions: map[string]int64{}}}
	for _, mutation := range mutations {
		response.Acknowledgement.Versions[mutation.Kind+"/"+mutation.ID] = mutation.ExpectedRevision + 1
		response.Resources = append(response.Resources, coordination.Resource{OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID, Kind: mutation.Kind, ID: mutation.ID, Revision: mutation.ExpectedRevision + 1, Deleted: mutation.Deleted, SourceID: mutation.SourceID, Body: mutation.Body})
	}
	response.Acknowledgement.Versions["checkpoint/"+receipt] = receiptVersion + 1
	mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: receipt, RoleID: scope.RoleID, ExpectedRevision: receiptVersion, Body: body(map[string]any{"hash": fingerprint, "response": response})})
	commit := coordination.Commit{Scope: scope, Mutations: mutations, Dependencies: dependencies}
	ack, err := e.commit(ctx, commit)
	if err != nil {
		if !errors.Is(err, coordination.ErrResourceVersion) && !errors.Is(err, coordination.ErrRequestConflict) && !errors.Is(err, coordination.ErrScopeExpired) {
			if pendingErr := e.coordination.Enqueue(ctx, commit); pendingErr != nil {
				return MemoryManagementResponse{}, errors.Join(err, pendingErr)
			}
		}
		return MemoryManagementResponse{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID {
		return MemoryManagementResponse{}, coordination.ErrWrongOwner
	}
	for key, version := range response.Acknowledgement.Versions {
		if ack.Versions[key] != version {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
	}
	response.Acknowledgement = ack
	return response, nil
}

func (e *Engine) ListOwnedMemoryCandidates(ctx context.Context, request Request, input MemoryManagementQuery) (MemoryManagementResponse, error) {
	if input.Limit < 0 || input.Limit > 128 || len(input.Cursor) > 4096 {
		return MemoryManagementResponse{}, errors.New("候选记忆分页参数无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	defer finish()
	snapshot, err := e.memoryManagementSnapshot(ctx, scope, "checkpoint", "")
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	result := MemoryManagementResponse{Scope: scope, Resources: []coordination.Resource{}}
	for _, row := range snapshot.Resources {
		if !strings.HasPrefix(row.ID, "memory-candidate/") {
			continue
		}
		var candidate OwnedMemoryCandidate
		if json.Unmarshal(row.Body, &candidate) != nil || candidate.Type != "owned-memory-candidate" || candidate.ID != row.ID || candidate.State != "pending" {
			continue
		}
		if input.Query != "" && !strings.Contains(strings.ToLower(candidate.Key+" "+candidate.Value), strings.ToLower(input.Query)) {
			continue
		}
		result.Resources = append(result.Resources, row)
	}
	sort.Slice(result.Resources, func(i, j int) bool { return result.Resources[i].ID < result.Resources[j].ID })
	catalog := []coordination.ResourceVersion{}
	for _, row := range result.Resources {
		catalog = append(catalog, coordination.ResourceVersion{Kind: row.Kind, ID: row.ID, Revision: row.Revision})
	}
	filter := input
	filter.Cursor, filter.Limit = "", 0
	page := memoryManagementCursor{Scope: memoryManagementHash(summaryAuthority(scope)), Filter: memoryManagementHash(filter), Catalog: memoryManagementHash(catalog)}
	if input.Cursor != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		var old memoryManagementCursor
		if err != nil || json.Unmarshal(encoded, &old) != nil || old.Scope != page.Scope || old.Filter != page.Filter || old.Catalog != page.Catalog || old.Offset < 1 || old.Offset >= len(result.Resources) {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		page.Offset = old.Offset
		result.Resources = result.Resources[old.Offset:]
	}
	limit := input.Limit
	if limit == 0 {
		limit = 32
	}
	if len(result.Resources) > limit {
		page.Offset += limit
		result.NextCursor = base64.RawURLEncoding.EncodeToString(body(page))
		result.Resources = result.Resources[:limit]
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return MemoryManagementResponse{}, err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return MemoryManagementResponse{}, err
	}
	return result, nil
}

func (e *Engine) ManageOwnedMemoryCandidates(ctx context.Context, request Request, input MemoryCandidateInput) (MemoryManagementResponse, error) {
	if input.Action != "generate" && input.Action != "update" && input.Action != "accept" && input.Action != "reject" || len(body(input)) > 256<<10 || len(input.Key) > 512 || len(input.Value) > 128<<10 || !utf8.ValidString(input.Key+input.Value) || strings.ContainsRune(input.Key+input.Value, 0) || input.Importance < 0 || input.Importance > 10 {
		return MemoryManagementResponse{}, errors.New("候选记忆操作参数无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, true)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryManagementResponse{}, errors.New("数据所有者不支持候选记忆")
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00memory-candidates\x00" + scope.RoleID)
	defer unlock()
	fingerprint := memoryManagementHash(struct {
		Scope            coordination.ExecutionScope
		Input            MemoryCandidateInput
		Origin           *ConversationOrigin
		HistoricalRoleID string
	}{summaryAuthority(scope), input, request.ConversationOrigin, request.HistoricalRoleID})
	receiptID := "memory-candidate-operation/" + request.RequestID
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return MemoryManagementResponse{}, err
	} else if pending != nil {
		if _, err := e.commit(ctx, *pending); err != nil {
			return MemoryManagementResponse{}, err
		}
	}
	receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	if receipt != nil {
		var saved struct {
			Hash     string                    `json:"hash"`
			Response *MemoryManagementResponse `json:"response"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint {
			return MemoryManagementResponse{}, coordination.ErrRequestConflict
		}
		if saved.Response == nil {
			return MemoryManagementResponse{}, ErrUncertainExecution
		}
		if summaryAuthority(saved.Response.Scope) != summaryAuthority(scope) {
			return MemoryManagementResponse{}, coordination.ErrScopeExpired
		}
		if err := e.coordination.Validate(ctx, scope); err != nil {
			return MemoryManagementResponse{}, err
		}
		if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
			return MemoryManagementResponse{}, err
		}
		if input.Action == "generate" {
			saved.Response.Scope = scope
		}
		return *saved.Response, nil
	}
	if input.Action == "generate" {
		return e.generateOwnedMemoryCandidates(ctx, scope, request, input, receiptID, fingerprint)
	}
	if !strings.HasPrefix(input.ID, "memory-candidate/") || input.ExpectedRevision < 1 {
		return MemoryManagementResponse{}, errors.New("请先加载原候选记录及其版本")
	}
	row, err := port.Resource(ctx, scope, "checkpoint", input.ID)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	var candidate OwnedMemoryCandidate
	if row == nil || row.Deleted || row.Revision != input.ExpectedRevision || json.Unmarshal(row.Body, &candidate) != nil || candidate.Type != "owned-memory-candidate" || candidate.ID != row.ID || candidate.State != "pending" {
		return MemoryManagementResponse{}, coordination.ErrResourceVersion
	}
	mutations := []coordination.Mutation{}
	dependencies := []coordination.ResourceVersion{}
	switch input.Action {
	case "update":
		if strings.TrimSpace(input.Key) == "" || strings.TrimSpace(input.Value) == "" {
			return MemoryManagementResponse{}, errors.New("候选记忆标题及内容不能为空")
		}
		candidate.Key, candidate.Value, candidate.MemoryType, candidate.Importance = strings.TrimSpace(input.Key), strings.TrimSpace(input.Value), input.MemoryType, input.Importance
	case "reject":
		candidate.State = "rejected"
	case "accept":
		provenanceRequest := request
		provenanceRequest.ConversationID, provenanceRequest.ConversationOrigin, provenanceRequest.HistoricalRoleID = candidate.ConversationID, candidate.ConversationOrigin, candidate.HistoricalRoleID
		messages, _, err := e.summaryHistory(ctx, provenanceRequest, scope)
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		if candidate.ProvenanceHash == "" || memoryManagementHash(messages) != candidate.ProvenanceHash {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		for _, dependency := range candidate.Dependencies {
			current, err := port.Resource(ctx, scope, dependency.Kind, dependency.ID)
			if err != nil {
				return MemoryManagementResponse{}, err
			}
			if current == nil || current.Deleted || current.Revision != dependency.Revision {
				return MemoryManagementResponse{}, coordination.ErrResourceVersion
			}
		}
		dependencies = append(dependencies, candidate.Dependencies...)
		snapshot, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		key := candidate.Key
		provisional, err := memoryMutations(scope, candidate.ConversationID, []DerivedMemory{{Kind: candidate.DerivedKind, Key: key, Body: candidate.Derived}}, snapshot)
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		var memoryID string
		for _, mutation := range provisional {
			if mutation.Kind == "memory" {
				memoryID = mutation.ID
			}
		}
		existing, err := port.Resource(ctx, scope, "memory", memoryID)
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		if existing != nil {
			if existing.Deleted || input.ConflictID != existing.ID || input.ExpectedConflictRevision != existing.Revision {
				return MemoryManagementResponse{}, MemoryManagementConflict{Resources: []coordination.Resource{*existing}}
			}
			dependencies = append(dependencies, coordination.ResourceVersion{Kind: "memory", ID: existing.ID, Revision: existing.Revision})
			switch input.Resolution {
			case "replace":
			case "merge":
				_, old, _, _, _ := memoryManagementText(existing.Body)
				candidate.Value = old + "\n" + candidate.Value
			case "keep_both":
				key += " · " + scope.RequestID
			default:
				return MemoryManagementResponse{}, errors.New("请选择候选记忆的冲突处理方式")
			}
		} else if input.ConflictID != "" {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		if len(candidate.Value) > 128<<10 || len(key) > 512 {
			return MemoryManagementResponse{}, coordination.ErrPendingLimit
		}
		var content map[string]any
		if json.Unmarshal(candidate.Derived, &content) != nil {
			return MemoryManagementResponse{}, errors.New("候选计算结果无效")
		}
		content["value"], content["memoryType"], content["importance"], content["source"] = candidate.Value, candidate.MemoryType, candidate.Importance, "reviewed"
		prepared, err := e.prepareMemoryMutations(ctx, scope, candidate.ConversationID, []DerivedMemory{{Kind: candidate.DerivedKind, Key: key, Body: body(content)}}, &snapshot)
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		mutations = append(mutations, prepared...)
		candidate.State = "accepted"
	}
	mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: candidate.ID, RoleID: scope.RoleID, ExpectedRevision: row.Revision, Body: body(candidate)})
	if input.Action == "accept" {
		provenanceRequest := request
		provenanceRequest.ConversationID, provenanceRequest.ConversationOrigin, provenanceRequest.HistoricalRoleID = candidate.ConversationID, candidate.ConversationOrigin, candidate.HistoricalRoleID
		messages, _, err := e.summaryHistory(ctx, provenanceRequest, scope)
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		if memoryManagementHash(messages) != candidate.ProvenanceHash {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
	}
	return e.memoryCandidateCommit(ctx, scope, mutations, dependencies, receiptID, 0, fingerprint)
}

func (e *Engine) generateOwnedMemoryCandidates(ctx context.Context, scope coordination.ExecutionScope, request Request, input MemoryCandidateInput, receiptID, fingerprint string) (MemoryManagementResponse, error) {
	if input.ConversationID == "" || len(input.ConversationID) > 512 {
		return MemoryManagementResponse{}, errors.New("请选择已保存的当前所有者对话")
	}
	request.ConversationID = input.ConversationID
	messages, dependencies, err := e.summaryHistory(ctx, request, scope)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	provenanceHash := memoryManagementHash(messages)
	if len(messages) == 0 || len(messages) > 4096 {
		return MemoryManagementResponse{}, errors.New("对话消息为空或超过候选提取上限")
	}
	if len(messages) > 128 {
		messages = messages[len(messages)-128:]
	}
	var user, assistant strings.Builder
	for _, message := range messages {
		if message.Role == "assistant" {
			assistant.WriteString(message.Content + "\n")
		} else if message.Role == "user" {
			user.WriteString(message.Content + "\n")
		}
	}
	if user.Len()+assistant.Len() > 256<<10 || user.Len() == 0 {
		return MemoryManagementResponse{}, errors.New("对话文本为空或超过候选提取上限")
	}
	reservation := coordination.Commit{Scope: scope, Dependencies: dependencies, Mutations: []coordination.Mutation{{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]any{"hash": fingerprint, "state": "generating"})}}}
	ack, err := e.commit(ctx, reservation)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID || ack.Versions["checkpoint/"+receiptID] != 1 {
		return MemoryManagementResponse{}, coordination.ErrWrongOwner
	}
	snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{Management: true, ResourceKind: "memory"})
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
		return MemoryManagementResponse{}, err
	}
	inference := Inference{Scope: scope, ConversationID: input.ConversationID, Snapshot: snapshot, Message: user.String()}
	var derived []DerivedMemory
	if model, ok := e.model.(MemoryCandidateModel); ok {
		derived, err = model.ExtractOwnedMemoryCandidates(ctx, inference, Generation{Text: assistant.String()})
	} else {
		derived, err = e.model.ExtractOwnedMemory(ctx, inference, Generation{Text: assistant.String()})
	}
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	if len(derived) > 80 {
		return MemoryManagementResponse{}, coordination.ErrPendingLimit
	}
	mutations := []coordination.Mutation{}
	seen := map[string]bool{}
	for _, item := range derived {
		if item.Kind != "fact" && item.Kind != "profile" && item.Kind != "episodic" {
			continue
		}
		if item.Key == "" || len(item.Key) > 512 || len(item.Body) > 128<<10 || !json.Valid(item.Body) || seen[item.Kind+"/"+item.Key] {
			return MemoryManagementResponse{}, errors.New("候选记忆计算结果无效")
		}
		seen[item.Kind+"/"+item.Key] = true
		var content struct {
			Value      string `json:"value"`
			Text       string `json:"text"`
			Type       string `json:"memoryType"`
			Importance int    `json:"importance"`
		}
		if json.Unmarshal(item.Body, &content) != nil {
			return MemoryManagementResponse{}, errors.New("候选记忆内容无效")
		}
		if content.Value == "" {
			content.Value = content.Text
		}
		if strings.TrimSpace(content.Value) == "" {
			var fields map[string]json.RawMessage
			if json.Unmarshal(item.Body, &fields) != nil {
				return MemoryManagementResponse{}, errors.New("候选记忆内容必须为对象")
			}
			for _, key := range []string{"preference", "event", "attributeValue", "description", "summary"} {
				var value string
				if json.Unmarshal(fields[key], &value) == nil && strings.TrimSpace(value) != "" {
					content.Value = value
					break
				}
			}
			if content.Value == "" {
				for _, key := range []string{"importance", "memoryType", "source", "createdAt", "value", "text"} {
					delete(fields, key)
				}
				if len(fields) > 0 {
					content.Value = string(body(fields))
				}
			}
		}
		if strings.TrimSpace(content.Value) == "" {
			return MemoryManagementResponse{}, errors.New("候选记忆内容不能为空")
		}
		if len(content.Value) > 128<<10 || !utf8.ValidString(content.Value) || len(content.Type) > 64 || content.Importance < 0 || content.Importance > 10 {
			return MemoryManagementResponse{}, errors.New("候选记忆内容或重要程度无效")
		}
		id := "memory-candidate/" + scope.RequestID + "/" + memoryManagementHash(item.Kind + "/" + item.Key)[:32]
		candidate := OwnedMemoryCandidate{Type: "owned-memory-candidate", ID: id, Key: item.Key, Value: content.Value, MemoryType: content.Type, Importance: content.Importance, State: "pending", DerivedKind: item.Kind, Derived: item.Body, ConversationID: input.ConversationID, ConversationOrigin: request.ConversationOrigin, HistoricalRoleID: request.HistoricalRoleID, ProvenanceHash: provenanceHash, Dependencies: dependencies, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), ExecutionScope: scope}
		mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: id, RoleID: scope.RoleID, Body: body(candidate)})
	}
	finalScope := scope
	currentMessages, _, err := e.summaryHistory(ctx, request, scope)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	if memoryManagementHash(currentMessages) != provenanceHash {
		return MemoryManagementResponse{}, coordination.ErrResourceVersion
	}
	finalScope.RequestID += "|candidate-result"
	ctx = coordination.WithScope(ctx, finalScope)
	response, err := e.memoryCandidateCommit(ctx, finalScope, mutations, dependencies, receiptID, 1, fingerprint)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	response.Scope = scope
	return response, nil
}
