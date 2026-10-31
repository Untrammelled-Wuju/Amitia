package business

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	amitialog "log"
)

type MemoryToolResponse struct {
	Text            string                       `json:"text"`
	Resources       []coordination.Resource      `json:"resources,omitempty"`
	Saved           bool                         `json:"saved"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement,omitempty"`
	Metadata        map[string]interface{}       `json:"metadata,omitempty"`
}

func ExplicitGlobalMemoryRequest(message string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(message), " "))
	if normalized == "" {
		return false
	}
	targets := []string{"全局记忆", "全局的记忆", "global memory"}
	targetFound := false
	for _, target := range targets {
		if strings.Contains(normalized, target) {
			targetFound = true
			break
		}
	}
	if !targetFound {
		return false
	}
	explicit := []string{"保存", "存储", "存到", "存入", "存为", "写入", "写到", "记录到", "记住到", "记到", "记为", "设为", "提升为", "加入", "放到", "迁移到", "改为", "save", "store", "write", "remember"}
	matched := false
	for _, action := range explicit {
		if strings.Contains(normalized, action) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	for _, rejected := range []string{"不要", "别保存", "别存", "无需", "不需要", "禁止", "不能", "不得", "不是", "并非"} {
		if strings.Contains(normalized, rejected) {
			return false
		}
	}
	for _, question := range []string{"为什么", "为何", "是否", "能否", "可不可以", "要不要", "需不需要", "怎么", "如何"} {
		if strings.Contains(normalized, question) {
			return false
		}
	}
	return !strings.ContainsAny(normalized, "?？")
}

func NormalizeMemoryExpiresAt(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UTC().Format(time.RFC3339Nano), nil
		}
	}
	return "", errors.New("记忆过期时间必须为日期或时间")
}

func NormalizeMemoryEntityID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", errors.New("关联实体编号必须为 UUID")
	}
	return id.String(), nil
}

func MemoryToolString(args map[string]interface{}, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func memoryToolInt(args map[string]interface{}, key string, fallback, low, high int) int {
	value := fallback
	switch raw := args[key].(type) {
	case float64:
		value = int(raw)
	case float32:
		value = int(raw)
	case int:
		value = raw
	case int64:
		value = int(raw)
	case json.Number:
		if number, err := raw.Int64(); err == nil {
			value = int(number)
		}
	}
	return max(low, min(high, value))
}

func (e *Engine) MemoryTool(ctx context.Context, request Request, name string, args map[string]interface{}) (MemoryToolResponse, error) {
	if role := MemoryToolString(args, "character_id"); role != "" && role != request.RoleID {
		return MemoryToolResponse{}, coordination.ErrWrongOwner
	}
	switch name {
	case "save_memory", "save_profile", "save_episodic_memory":
		return e.saveMemoryTool(ctx, request, name, args)
	case "query_memory", "get_memory_by_title":
		return e.queryMemoryTool(ctx, request, name, args)
	case "summarize_memories":
		return e.summarizeMemoryTool(ctx, request, args)
	case "request_memory_consolidation":
		page, err := e.SearchOwnedMemories(ctx, request, MemoryManagementQuery{Mode: "keyword", Limit: 1})
		if err != nil {
			return MemoryToolResponse{}, err
		}
		request.ExpectedScope = &page.Scope
		result, err := e.ManageOwnedMemoryCandidates(ctx, request, MemoryCandidateInput{Action: "generate", ConversationID: request.ConversationID, Consolidate: true, MaxOutputs: memoryToolInt(args, "max_outputs", 5, 1, 10)})
		if err != nil {
			return MemoryToolResponse{}, err
		}
		return MemoryToolResponse{Text: fmt.Sprintf("记忆整理生成 %d 条候选，审核通过后写入正式记忆。", len(result.Resources)), Resources: result.Resources, Saved: result.Saved, Acknowledgement: result.Acknowledgement, Metadata: map[string]interface{}{"requiresReview": len(result.Resources)}}, nil
	default:
		return MemoryToolResponse{}, errors.New("不支持的记忆操作")
	}
}

func (e *Engine) saveMemoryTool(ctx context.Context, request Request, name string, args map[string]interface{}) (MemoryToolResponse, error) {
	scopeName := MemoryToolString(args, "scope")
	if scopeName == "" {
		scopeName = "character"
	}
	if scopeName != "character" && scopeName != "global" {
		return MemoryToolResponse{}, errors.New("记忆范围无效")
	}
	if scopeName == "global" {
		if !ExplicitGlobalMemoryRequest(request.Message) {
			return MemoryToolResponse{}, errors.New("只有用户明确要求保存到全局记忆时才能写入全局记忆")
		}
		request.MemoryView = "global"
	} else {
		request.MemoryView = ""
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryToolResponse{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryToolResponse{}, coordination.ErrWrongOwner
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00memory-tool\x00" + request.RequestID)
	defer unlock()
	receiptID := "memory-tool/" + request.RequestID
	fingerprint := memoryManagementHash([]interface{}{summaryAuthority(scope), name, args, request.ConversationID})
	if receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID); err != nil {
		return MemoryToolResponse{}, err
	} else if receipt != nil {
		var saved struct {
			Hash     string             `json:"hash"`
			Response MemoryToolResponse `json:"response"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint {
			return MemoryToolResponse{}, coordination.ErrRequestConflict
		}
		return saved.Response, e.coordination.Validate(ctx, scope)
	}
	key, value, kind := MemoryToolString(args, "key"), MemoryToolString(args, "value"), "fact"
	content := map[string]interface{}{"value": value, "memoryType": MemoryToolString(args, "memoryType"), "importance": memoryToolInt(args, "importance", 5, 1, 10), "confidence": memoryToolInt(args, "confidence", 50, 0, 100), "source": "tool", "createdAt": time.Now().UTC().Format(time.RFC3339Nano)}
	if content["memoryType"] == "" {
		content["memoryType"] = "fact"
	}
	if name == "save_profile" {
		kind, key, value = "profile", MemoryToolString(args, "attribute_name"), MemoryToolString(args, "attribute_value")
		category := MemoryToolString(args, "category")
		if category == "" {
			return MemoryToolResponse{}, errors.New("画像类别不能为空")
		}
		content["category"], content["attributeName"], content["attributeValue"], content["memoryType"] = category, key, value, category
	}
	if name == "save_episodic_memory" {
		kind, key, value = "episodic", MemoryToolString(args, "title"), MemoryToolString(args, "content")
		scene := MemoryToolString(args, "scene_type")
		if scene == "" {
			return MemoryToolResponse{}, errors.New("情景类型不能为空")
		}
		content["sceneType"], content["title"], content["sentimentScore"], content["memoryType"] = scene, key, memoryToolInt(args, "sentiment_score", 0, -10, 10), "episodic"
	}
	content["value"] = value
	entity, err := NormalizeMemoryEntityID(MemoryToolString(args, "entityId"))
	if err != nil {
		return MemoryToolResponse{}, err
	}
	if entity != "" {
		content["entityId"] = entity
	}
	if key == "" || value == "" || len(key) > 512 || len(value) > 128<<10 || !utf8.ValidString(key+value) || strings.ContainsRune(key+value, 0) {
		return MemoryToolResponse{}, errors.New("记忆内容无效")
	}
	snapshot, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return MemoryToolResponse{}, err
	}
	if kind == "profile" {
		key = MemoryToolString(args, "category") + "/" + key
		for _, row := range snapshot.Resources {
			document, previous, err := MemoryContent(MemoryConsumerEntry{Resource: row})
			if err != nil {
				return MemoryToolResponse{}, err
			}
			if MemoryText(document, "key") != key || MemoryText(document, "kind") != "profile" {
				continue
			}
			var oldConfidence int
			if err := json.Unmarshal(previous["confidence"], &oldConfidence); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			confidence := oldConfidence
			if MemoryText(previous, "value") != value {
				confidence = max(5, oldConfidence/2)
			} else if MemoryText(document, "conversationId") != request.ConversationID && oldConfidence < 95 {
				confidence = min(95, oldConfidence+max(2, 10-oldConfidence/10))
			}
			content["confidence"] = confidence
		}
	}
	for _, row := range snapshot.Resources {
		var document map[string]json.RawMessage
		if err := json.Unmarshal(row.Body, &document); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		var pinned bool
		if err := json.Unmarshal(document["pinned"], &pinned); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if pinned && MemoryText(document, "key") == key && MemoryText(document, "kind") == kind {
			return MemoryToolResponse{}, errors.New("固定记忆只能由用户在记忆管理中修改")
		}
	}
	mutations, err := e.prepareMemoryMutations(ctx, scope, request.ConversationID, []DerivedMemory{{Kind: kind, Key: key, Body: body(content)}}, &snapshot)
	if err != nil {
		return MemoryToolResponse{}, err
	}
	expires, err := NormalizeMemoryExpiresAt(MemoryToolString(args, "expiresAt"))
	if err != nil {
		return MemoryToolResponse{}, err
	}
	if expires != "" {
		parsed, err := time.Parse(time.RFC3339Nano, expires)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", expires)
		}
		if err != nil {
			return MemoryToolResponse{}, errors.New("记忆过期时间无效")
		}
		for index := range mutations {
			if mutations[index].Kind == "memory" {
				var document map[string]json.RawMessage
				if err := json.Unmarshal(mutations[index].Body, &document); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
				document["expiresAt"] = body(parsed.UTC().Format(time.RFC3339Nano))
				mutations[index].Body = body(document)
			}
		}
	}
	response := MemoryToolResponse{Text: fmt.Sprintf("已保存 %s：%s", key, value), Saved: true, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{}}}
	for _, mutation := range mutations {
		response.Resources = append(response.Resources, coordination.Resource{OwnerID: scope.ResourceOwnerID, RoleID: mutation.RoleID, Kind: mutation.Kind, ID: mutation.ID, SourceID: mutation.SourceID, Revision: mutation.ExpectedRevision + 1, Body: mutation.Body})
		response.Acknowledgement.Versions[mutation.Kind+"/"+mutation.ID] = mutation.ExpectedRevision + 1
	}
	response.Acknowledgement.Versions["checkpoint/"+receiptID] = 1
	mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]interface{}{"hash": fingerprint, "response": response})})
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, Mutations: mutations})
	if err != nil {
		return MemoryToolResponse{}, err
	}
	response.Acknowledgement = ack
	return response, nil
}

func (e *Engine) queryMemoryTool(ctx context.Context, request Request, name string, args map[string]interface{}) (MemoryToolResponse, error) {
	query := MemoryToolString(args, "query")
	exact := name == "get_memory_by_title"
	if exact {
		query = MemoryToolString(args, "title")
	}
	if query == "" {
		return MemoryToolResponse{}, errors.New("检索内容不能为空")
	}
	mode := MemoryToolString(args, "mode")
	if mode == "" {
		mode = "hybrid"
	}
	if exact {
		mode = "keyword"
	}
	page, err := e.SearchOwnedMemories(ctx, request, MemoryManagementQuery{ContextUsable: true, Query: query, Mode: mode, ExactTitle: exact, Limit: memoryToolInt(args, "limit", 10, 1, 50), Types: memoryToolStrings(args, "types"), Layers: memoryToolStrings(args, "layers"), MinimumImportance: memoryToolInt(args, "min_importance", 0, 0, 10), Time: memoryTimeFilterArgs(args)})
	if err != nil {
		return MemoryToolResponse{}, err
	}
	selected := []coordination.Resource{}
	for _, row := range page.Resources {
		if MemoryActive(MemoryConsumerEntry{Resource: row}) {
			selected = append(selected, row)
		}
	}
	if !exact {
		return MemoryToolResponse{Text: string(body(selected)), Resources: selected, Metadata: map[string]interface{}{"count": len(selected), "scores": page.Scores}}, nil
	}
	if len(selected) == 0 {
		return MemoryToolResponse{Text: "未找到该标题的记忆", Metadata: map[string]interface{}{"count": 0}}, nil
	}
	row := selected[0]
	_, value, _, _, _ := memoryManagementText(row.Body)
	text := []rune(value)
	offset := memoryToolInt(args, "offset", 0, 0, 10000000)
	needle := MemoryToolString(args, "query")
	if needle != "" {
		position := strings.Index(strings.ToLower(value), strings.ToLower(needle))
		if position < 0 {
			return MemoryToolResponse{Text: "记忆正文中未找到检索内容", Metadata: map[string]interface{}{"count": 0}}, nil
		}
		offset = len([]rune(strings.ToLower(value)[:position])) + offset
	}
	offset = min(offset, len(text))
	end := min(offset+memoryToolInt(args, "max_chars", 2000, 1, 50000), len(text))
	chunk := string(text[offset:end])
	return MemoryToolResponse{Text: chunk, Resources: []coordination.Resource{row}, Metadata: map[string]interface{}{"id": row.ID, "title": query, "offset": offset, "nextOffset": end, "totalChars": len(text), "truncated": end < len(text)}}, nil
}

func (e *Engine) summarizeMemoryTool(ctx context.Context, request Request, args map[string]interface{}) (MemoryToolResponse, error) {
	topic := MemoryToolString(args, "topic")
	if topic == "" {
		return MemoryToolResponse{}, errors.New("摘要主题不能为空")
	}
	page, err := e.SearchOwnedMemories(ctx, request, MemoryManagementQuery{ContextUsable: true, Query: topic, Mode: "keyword", Limit: memoryToolInt(args, "limit", 20, 1, 50), Sort: "importance_desc", MinimumImportance: memoryToolInt(args, "min_importance", 1, 1, 10)})
	if err != nil {
		return MemoryToolResponse{}, err
	}
	var lines []string
	var messages []SummaryMessage
	for _, resource := range page.Resources {
		key, value, _, _, importance := memoryManagementText(resource.Body)
		if importance < memoryToolInt(args, "min_importance", 1, 1, 10) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", key, value))
		messages = append(messages, SummaryMessage{ID: resource.ID, OwnerID: resource.OwnerID, Revision: resource.Revision, Role: "user", Content: key + ": " + value})
	}
	mode := MemoryToolString(args, "mode")
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "model" && mode != "deterministic" {
		return MemoryToolResponse{}, errors.New("摘要模式无效")
	}
	text := strings.Join(lines, "\n")
	if len(lines) == 0 {
		text = "未找到相关记忆"
	} else {
		text = fmt.Sprintf("找到 %d 条相关记忆：\n%s", len(lines), text)
	}
	generatedBy := "deterministic"
	if len(messages) > 0 && (mode == "model" || mode == "auto" && len(messages) > 5) {
		model, ok := e.model.(SummaryModel)
		if !ok {
			return MemoryToolResponse{}, errors.New("记忆摘要模型端口不可用")
		}
		snapshot, err := e.data.Snapshot(ctx, page.Scope, coordination.DataQuery{Management: true, ResourceKind: "memory"})
		if err != nil {
			return MemoryToolResponse{}, err
		}
		text, err = model.GenerateOwnedSummary(coordination.WithScope(ctx, page.Scope), SummaryInference{Scope: page.Scope, Role: snapshot.Role, Messages: messages, MemoryTopic: topic})
		if err != nil {
			return MemoryToolResponse{}, err
		}
		generatedBy = "model"
	}
	if err := e.coordination.Validate(ctx, page.Scope); err != nil {
		return MemoryToolResponse{}, err
	}
	return MemoryToolResponse{Text: text, Resources: page.Resources, Metadata: map[string]interface{}{"evidenceCount": len(messages), "generatedBy": generatedBy, "topic": topic}}, nil
}
