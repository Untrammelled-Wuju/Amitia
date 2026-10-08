package business

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type MemoryManagementInput struct {
	Action                   string `json:"action"`
	Key                      string `json:"key"`
	Value                    string `json:"value"`
	MemoryType               string `json:"memoryType"`
	Importance               int    `json:"importance"`
	ConflictID               string `json:"conflictId,omitempty"`
	ExpectedConflictRevision int64  `json:"expectedConflictRevision,omitempty"`
	Resolution               string `json:"resolution,omitempty"`
}

type MemoryManagementQuery struct {
	Query      string `json:"query"`
	Mode       string `json:"mode"`
	MemoryType string `json:"memoryType,omitempty"`
	Source     string `json:"source,omitempty"`
	Sort       string `json:"sort,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type MemoryManagementResponse struct {
	Resources       []coordination.Resource      `json:"resources"`
	Scope           coordination.ExecutionScope  `json:"executionScope"`
	Saved           bool                         `json:"saved"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
	NextCursor      string                       `json:"nextCursor,omitempty"`
	Scores          map[string]float64           `json:"scores,omitempty"`
}

type MemoryManagementConflict struct {
	Resources []coordination.Resource `json:"conflicts"`
}

type memoryManagementCursor struct {
	Scope   string `json:"scope"`
	Filter  string `json:"filter"`
	Catalog string `json:"catalog"`
	Offset  int    `json:"offset"`
}

func memoryManagementHash(value any) string {
	digest := sha256.Sum256(body(value))
	return hex.EncodeToString(digest[:])
}

func (e MemoryManagementConflict) Error() string {
	return "同一角色已有同键记忆，请确认原记录版本后选择冲突处理方式"
}

func (e *Engine) memoryManagementScope(ctx context.Context, request Request, write bool) (context.Context, coordination.ExecutionScope, func(), error) {
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return ctx, scope, finish, err
	}
	if write && (request.ExpectedScope == nil || summaryAuthority(*request.ExpectedScope) != summaryAuthority(scope)) {
		finish()
		return ctx, scope, func() {}, coordination.ErrScopeExpired
	}
	return ctx, scope, finish, nil
}

func (e *Engine) memoryManagementSnapshot(ctx context.Context, scope coordination.ExecutionScope, kind, conversation string) (coordination.DataSnapshot, error) {
	combined := coordination.DataSnapshot{OwnerID: scope.ResourceOwnerID}
	seen := map[string]bool{}
	cursor := ""
	bytes := 0
	for page := 0; page < 256; page++ {
		snapshot, err := e.data.Snapshot(ctx, scope, coordination.DataQuery{Management: true, ResourceKind: kind, ConversationID: conversation, Cursor: cursor, Limit: 128})
		if err != nil {
			return combined, err
		}
		if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
			return combined, err
		}
		combined.Role = snapshot.Role
		for _, resource := range snapshot.Resources {
			if resource.Kind != kind || resource.Deleted {
				continue
			}
			key := resource.Kind + "/" + resource.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			bytes += len(resource.Body)
			combined.Resources = append(combined.Resources, resource)
		}
		if len(combined.Resources) > 32768 || bytes > 32<<20 {
			return combined, coordination.ErrPendingLimit
		}
		next := snapshot.NextCursors[kind]
		if next == "" {
			return combined, nil
		}
		if seen["cursor/"+next] || next == cursor {
			return combined, coordination.ErrRequestConflict
		}
		seen["cursor/"+next] = true
		cursor = next
	}
	return combined, coordination.ErrPendingLimit
}

func memoryManagementText(raw json.RawMessage) (string, string, string, string, int) {
	var outer struct {
		Key     string          `json:"key"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &outer) != nil {
		return "", "", "", "", 0
	}
	var content struct {
		Value      string `json:"value"`
		Text       string `json:"text"`
		Type       string `json:"memoryType"`
		Source     string `json:"source"`
		Importance int    `json:"importance"`
	}
	if json.Unmarshal(outer.Content, &content) != nil {
		var text string
		_ = json.Unmarshal(outer.Content, &text)
		return outer.Key, text, "", "", 0
	}
	if content.Value == "" {
		content.Value = content.Text
	}
	return outer.Key, content.Value, content.Type, content.Source, content.Importance
}

func (e *Engine) SearchOwnedMemories(ctx context.Context, request Request, input MemoryManagementQuery) (MemoryManagementResponse, error) {
	if len(input.Query) > 1024 || !utf8.ValidString(input.Query) || strings.ContainsRune(input.Query, 0) || input.Limit < 0 || input.Limit > 128 || len(input.Cursor) > 4096 || input.Mode != "" && input.Mode != "keyword" && input.Mode != "hybrid" && input.Mode != "vector" {
		return MemoryManagementResponse{}, errors.New("记忆检索参数无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, false)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	defer finish()
	snapshot, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	result := MemoryManagementResponse{Scope: scope, Resources: []coordination.Resource{}, Scores: map[string]float64{}}
	query := strings.ToLower(strings.TrimSpace(input.Query))
	semantic := map[string]float64{}
	if input.Mode == "hybrid" || input.Mode == "vector" {
		if query == "" {
			return MemoryManagementResponse{}, errors.New("语义检索内容不能为空")
		}
		model, ok := e.model.(SemanticModel)
		if !ok {
			return MemoryManagementResponse{}, errors.New("当前 Core 尚未配置记忆检索模型")
		}
		values, fingerprint, err := model.OwnedQueryVector(ctx, input.Query)
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		if err := coordination.ValidateQueryVector(coordination.DataQuery{Vector: values, VectorModel: fingerprint}); err != nil || len(values) == 0 {
			return MemoryManagementResponse{}, errors.New("当前 Core 的检索向量无效")
		}
		vectors, err := e.memoryManagementSnapshot(ctx, scope, "vector", "")
		if err != nil {
			return MemoryManagementResponse{}, err
		}
		usable := map[string]bool{}
		for _, resource := range snapshot.Resources {
			usable[resource.ID] = coordination.ResourceUsable(resource.Body, time.Now())
		}
		for _, resource := range vectors.Resources {
			if !usable[resource.SourceID] || !coordination.ResourceUsable(resource.Body, time.Now()) {
				continue
			}
			var vector struct {
				Content struct {
					Values []float32 `json:"values"`
					Model  string    `json:"modelFingerprint"`
				} `json:"content"`
			}
			if json.Unmarshal(resource.Body, &vector) != nil || vector.Content.Model != fingerprint || len(vector.Content.Values) != len(values) {
				continue
			}
			var dot, left, right float64
			valid := true
			for index, value := range vector.Content.Values {
				x, y := float64(value), float64(values[index])
				if math.IsNaN(x) || math.IsInf(x, 0) {
					valid = false
					break
				}
				dot += x * y
				left += x * x
				right += y * y
			}
			if valid && left > 0 && right > 0 {
				score := dot / math.Sqrt(left*right)
				if score > semantic[resource.SourceID] {
					semantic[resource.SourceID] = score
				}
			}
		}
	}
	for _, resource := range snapshot.Resources {
		key, value, kind, source, importance := memoryManagementText(resource.Body)
		if input.MemoryType != "" && input.MemoryType != kind || input.Source != "" && input.Source != source {
			continue
		}
		text := strings.ToLower(key + " " + value)
		keyword := query == "" || strings.Contains(text, query)
		if input.Mode == "vector" && semantic[resource.ID] <= 0 || input.Mode != "vector" && !keyword && semantic[resource.ID] <= 0 {
			continue
		}
		result.Resources = append(result.Resources, resource)
		if input.Mode == "hybrid" || input.Mode == "vector" {
			result.Scores[resource.ID] = semantic[resource.ID]
			if input.Mode == "hybrid" && keyword {
				result.Scores[resource.ID] += 1
			}
		} else {
			result.Scores[resource.ID] = float64(importance)
		}
	}
	sort.Slice(result.Resources, func(i, j int) bool {
		if (input.Sort == "importance_desc" || input.Mode == "hybrid" || input.Mode == "vector") && result.Scores[result.Resources[i].ID] != result.Scores[result.Resources[j].ID] {
			return result.Scores[result.Resources[i].ID] > result.Scores[result.Resources[j].ID]
		}
		return result.Resources[i].ID < result.Resources[j].ID
	})
	catalog := []coordination.ResourceVersion{}
	for _, resource := range result.Resources {
		catalog = append(catalog, coordination.ResourceVersion{Kind: resource.Kind, ID: resource.ID, Revision: resource.Revision})
	}
	filter := input
	filter.Cursor, filter.Limit = "", 0
	page := memoryManagementCursor{Scope: memoryManagementHash(summaryAuthority(scope)), Filter: memoryManagementHash(filter), Catalog: memoryManagementHash(catalog)}
	if input.Cursor != "" {
		encoded, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		var previous memoryManagementCursor
		if err != nil || json.Unmarshal(encoded, &previous) != nil || previous.Scope != page.Scope || previous.Filter != page.Filter || previous.Catalog != page.Catalog || previous.Offset < 1 || previous.Offset >= len(result.Resources) {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		page.Offset = previous.Offset
		result.Resources = result.Resources[page.Offset:]
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

func (e *Engine) ManageOwnedMemory(ctx context.Context, request Request, input MemoryManagementInput) (MemoryManagementResponse, error) {
	input.Key, input.Value = strings.TrimSpace(input.Key), strings.TrimSpace(input.Value)
	if input.Action != "create" && input.Action != "resolve" || input.Key == "" || input.Value == "" || len(input.Key) > 512 || len(input.Value) > 128<<10 || !utf8.ValidString(input.Key+input.Value) || strings.ContainsRune(input.Key+input.Value, 0) || input.Importance < 0 || input.Importance > 10 || len(input.MemoryType) > 64 {
		return MemoryManagementResponse{}, errors.New("记忆内容或管理操作参数无效")
	}
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, true)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return MemoryManagementResponse{}, errors.New("数据所有者不支持记忆管理")
	}
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00memory-management\x00" + scope.RoleID)
	defer unlock()
	digest := sha256.Sum256(body(struct {
		Scope coordination.ExecutionScope
		Input MemoryManagementInput
	}{summaryAuthority(scope), input}))
	fingerprint := hex.EncodeToString(digest[:])
	receiptID := "memory-management/" + request.RequestID
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return MemoryManagementResponse{}, err
	} else if pending != nil {
		if _, err := e.commit(ctx, *pending); err != nil {
			return MemoryManagementResponse{}, err
		}
	}
	if receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID); err != nil {
		return MemoryManagementResponse{}, err
	} else if receipt != nil {
		var saved struct {
			Hash     string                   `json:"hash"`
			Response MemoryManagementResponse `json:"response"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint || summaryAuthority(saved.Response.Scope) != summaryAuthority(scope) {
			return MemoryManagementResponse{}, coordination.ErrRequestConflict
		}
		if err := e.coordination.Validate(ctx, scope); err != nil {
			return MemoryManagementResponse{}, err
		}
		if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
			return MemoryManagementResponse{}, err
		}
		return saved.Response, nil
	}
	snapshot, err := e.memoryManagementSnapshot(ctx, scope, "memory", "")
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	identity := sha256.Sum256([]byte(scope.RoleID + "\x00memory\x00fact/" + input.Key))
	memoryID := hex.EncodeToString(identity[:16])
	existing, err := port.Resource(ctx, scope, "memory", memoryID)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	dependencies := []coordination.ResourceVersion{}
	if existing != nil {
		if existing.Deleted {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		if input.Action != "resolve" {
			return MemoryManagementResponse{}, MemoryManagementConflict{Resources: []coordination.Resource{*existing}}
		}
		if input.ConflictID != existing.ID || input.ExpectedConflictRevision != existing.Revision {
			return MemoryManagementResponse{}, coordination.ErrResourceVersion
		}
		dependencies = append(dependencies, coordination.ResourceVersion{Kind: "memory", ID: existing.ID, Revision: existing.Revision})
		switch input.Resolution {
		case "replace":
		case "merge":
			_, previous, _, _, _ := memoryManagementText(existing.Body)
			input.Value = previous + "\n" + input.Value
			if len(input.Value) > 128<<10 {
				return MemoryManagementResponse{}, coordination.ErrPendingLimit
			}
		case "keep_both":
			input.Key += " · " + request.RequestID
			if len(input.Key) > 512 {
				return MemoryManagementResponse{}, errors.New("同时保留后的记忆标题过长，请缩短标题")
			}
		default:
			return MemoryManagementResponse{}, errors.New("请选择替换、合并或同时保留")
		}
	} else if input.Action == "resolve" {
		return MemoryManagementResponse{}, coordination.ErrResourceVersion
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	derived := []DerivedMemory{{Kind: "fact", Key: input.Key, Body: body(map[string]any{"value": input.Value, "memoryType": input.MemoryType, "importance": input.Importance, "source": "manual", "createdAt": now})}}
	mutations, err := e.prepareMemoryMutations(ctx, scope, "", derived, &snapshot)
	if err != nil {
		return MemoryManagementResponse{}, err
	}
	response := MemoryManagementResponse{Scope: scope, Saved: true, Resources: []coordination.Resource{}, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{}}}
	for _, mutation := range mutations {
		response.Acknowledgement.Versions[mutation.Kind+"/"+mutation.ID] = mutation.ExpectedRevision + 1
		response.Resources = append(response.Resources, coordination.Resource{OwnerID: scope.ResourceOwnerID, RoleID: scope.RoleID, Kind: mutation.Kind, ID: mutation.ID, SourceID: mutation.SourceID, Revision: mutation.ExpectedRevision + 1, Body: mutation.Body})
	}
	response.Acknowledgement.Versions["checkpoint/"+receiptID] = 1
	mutations = append(mutations, coordination.Mutation{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]any{"hash": fingerprint, "response": response})})
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
