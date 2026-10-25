package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/worldbook"
)

type WorldbookRule struct {
	ID                     string          `json:"id"`
	Scope                  string          `json:"scope"`
	CharacterID            string          `json:"characterId,omitempty"`
	MatchType              string          `json:"matchType"`
	MatchPattern           string          `json:"matchPattern"`
	MatchScope             string          `json:"matchScope"`
	InjectContent          string          `json:"injectContent"`
	Priority               int             `json:"priority"`
	Enabled                bool            `json:"enabled"`
	CharacterBookKeys      []string        `json:"characterBookKeys,omitempty"`
	SecondaryKeys          []string        `json:"secondaryKeys,omitempty"`
	CaseSensitive          *bool           `json:"caseSensitive,omitempty"`
	Selective              *bool           `json:"selective,omitempty"`
	Constant               *bool           `json:"constant,omitempty"`
	InsertionOrder         int             `json:"insertionOrder,omitempty"`
	Position               *string         `json:"position,omitempty"`
	Extensions             map[string]any  `json:"extensions,omitempty"`
	CharacterBookEntryName string          `json:"characterBookEntryName,omitempty"`
	CharacterBookEntryID   json.RawMessage `json:"characterBookEntryId,omitempty"`
	SourceCardHash         string          `json:"sourceCardHash,omitempty"`
	SourceEntryIndex       int             `json:"sourceEntryIndex,omitempty"`
	CreatedAt              string          `json:"createdAt"`
	UpdatedAt              string          `json:"updatedAt"`
}

type WorldbookMutation struct {
	ExpectedScope    *coordination.ExecutionScope `json:"expectedExecutionScope"`
	RequestID        string                       `json:"requestId"`
	RoleID           string                       `json:"characterId"`
	TargetDeviceID   string                       `json:"targetDeviceId,omitempty"`
	Action           string                       `json:"action"`
	Rule             WorldbookRule                `json:"rule"`
	ExpectedRevision int64                        `json:"expectedRevision,omitempty"`
}

type WorldbookRuleView struct {
	WorldbookRule
	OwnerID  string `json:"ownerId"`
	RoleID   string `json:"roleId"`
	Revision int64  `json:"revision"`
}

type WorldbookMutationResponse struct {
	Rule            *WorldbookRuleView           `json:"rule,omitempty"`
	Scope           coordination.ExecutionScope  `json:"executionScope"`
	Saved           bool                         `json:"saved"`
	Acknowledgement coordination.Acknowledgement `json:"acknowledgement"`
}

type WorldbookMatch struct {
	RuleID     string `json:"ruleId"`
	Revision   int64  `json:"revision"`
	Scope      string `json:"scope"`
	MatchScope string `json:"matchScope"`
	HitText    string `json:"hitText"`
}

type WorldbookMatchResponse struct {
	Matches []WorldbookMatch            `json:"matches"`
	Scope   coordination.ExecutionScope `json:"executionScope"`
}

func normalizeWorldbookRule(input WorldbookRule, roleID string) (WorldbookRule, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Scope = strings.ToLower(strings.TrimSpace(input.Scope))
	input.CharacterID = strings.TrimSpace(input.CharacterID)
	input.MatchType = strings.ToLower(strings.TrimSpace(input.MatchType))
	input.MatchPattern = strings.TrimSpace(input.MatchPattern)
	input.MatchScope = strings.ToLower(strings.TrimSpace(input.MatchScope))
	input.InjectContent = strings.TrimSpace(input.InjectContent)
	if input.Scope != "global" && input.Scope != "role" {
		return WorldbookRule{}, errors.New("世界书作用域必须为 global 或 role")
	}
	if input.Scope == "global" {
		input.CharacterID = ""
	} else {
		input.CharacterID = roleID
	}
	if input.MatchType != "keyword" && input.MatchType != "exact" && input.MatchType != "regex" {
		return WorldbookRule{}, errors.New("世界书匹配类型无效")
	}
	if input.MatchScope == "" {
		input.MatchScope = "full_context"
	}
	if input.MatchScope != "full_context" && input.MatchScope != "user_message" && input.MatchScope != "assistant_reply" {
		return WorldbookRule{}, errors.New("世界书匹配范围无效")
	}
	if input.MatchPattern == "" || len(input.MatchPattern) > 4096 || !utf8.ValidString(input.MatchPattern) || strings.ContainsRune(input.MatchPattern, 0) {
		return WorldbookRule{}, errors.New("世界书匹配内容无效或超过上限")
	}
	if input.InjectContent == "" || len(input.InjectContent) > 128<<10 || !utf8.ValidString(input.InjectContent) || strings.ContainsRune(input.InjectContent, 0) {
		return WorldbookRule{}, errors.New("世界书注入内容无效或超过上限")
	}
	if input.Priority < -1000 || input.Priority > 1000 {
		return WorldbookRule{}, errors.New("世界书优先级超出范围")
	}
	if len(input.SecondaryKeys) > 64 {
		return WorldbookRule{}, errors.New("世界书次级关键词数量超出上限")
	}
	if len(input.CharacterBookKeys) > 64 {
		return WorldbookRule{}, errors.New("世界书原始关键词数量超出上限")
	}
	for index, key := range input.CharacterBookKeys {
		key = strings.TrimSpace(key)
		if len(key) > 256 || !utf8.ValidString(key) || strings.ContainsRune(key, 0) {
			return WorldbookRule{}, errors.New("世界书原始关键词无效")
		}
		input.CharacterBookKeys[index] = key
	}
	for index, key := range input.SecondaryKeys {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > 256 || !utf8.ValidString(key) || strings.ContainsRune(key, 0) {
			return WorldbookRule{}, errors.New("世界书次级关键词无效")
		}
		input.SecondaryKeys[index] = key
	}
	if input.SourceCardHash != "" && (len(input.SourceCardHash) != 64 || !utf8.ValidString(input.SourceCardHash)) {
		return WorldbookRule{}, errors.New("世界书来源卡片摘要无效")
	}
	if input.SourceEntryIndex < 0 || input.SourceEntryIndex > 1000000 {
		return WorldbookRule{}, errors.New("世界书来源条目序号无效")
	}
	if len(input.Extensions) > 0 {
		encoded, err := json.Marshal(input.Extensions)
		if err != nil || len(encoded) > 64<<10 {
			return WorldbookRule{}, errors.New("世界书扩展元数据无效或超过上限")
		}
	}
	if input.MatchType == "regex" {
		if _, err := regexp.Compile(input.MatchPattern); err != nil {
			return WorldbookRule{}, errors.New("世界书正则表达式无效")
		}
	}
	return input, nil
}

func worldbookRuleFromResource(resource coordination.Resource) (WorldbookRuleView, error) {
	var rule WorldbookRule
	if err := json.Unmarshal(resource.Body, &rule); err != nil {
		return WorldbookRuleView{}, errors.New("世界书资源格式无效")
	}
	rule.ID = resource.ID
	if resource.RoleID == coordination.GlobalWorldbookRole {
		rule.Scope, rule.CharacterID = "global", ""
	} else {
		rule.Scope, rule.CharacterID = "role", resource.RoleID
	}
	return WorldbookRuleView{WorldbookRule: rule, OwnerID: resource.OwnerID, RoleID: resource.RoleID, Revision: resource.Revision}, nil
}

func worldbookResourceRule(resource coordination.Resource) (worldbook.WorldBookEntry, error) {
	view, err := worldbookRuleFromResource(resource)
	if err != nil {
		return worldbook.WorldBookEntry{}, err
	}
	return worldbook.WorldBookEntry{
		ID: view.ID, MatchType: view.MatchType, MatchPattern: view.MatchPattern,
		MatchScope: view.MatchScope, InjectContent: view.InjectContent,
		Priority: view.Priority, CharacterID: view.CharacterID,
		SecondaryKeys: view.SecondaryKeys, CaseSensitive: view.CaseSensitive,
		Selective: view.Selective, Constant: view.Constant,
	}, nil
}

func worldbookScopeAuthority(scope coordination.ExecutionScope) coordination.ExecutionScope {
	scope.RequestID, scope.TurnID, scope.ExecutionID = "", "", ""
	scope.WorldbookRoleID = ""
	return scope
}

func (e *Engine) worldbookScope(ctx context.Context, request Request, input WorldbookMutation) (context.Context, coordination.ExecutionScope, func(), error) {
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, request.RequestID)
	if err != nil {
		return ctx, scope, finish, err
	}
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	role, err := coordination.ResolveRole(input.RoleID, roles)
	if err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	if err := coordination.ResolveMemoryScope(&scope, ""); err != nil {
		finish()
		return ctx, scope, func() {}, err
	}
	if input.ExpectedScope == nil || worldbookScopeAuthority(*input.ExpectedScope) != worldbookScopeAuthority(scope) {
		finish()
		return ctx, scope, func() {}, coordination.ErrScopeExpired
	}
	coordination.ResolveWorldbookScope(&scope, input.Rule.Scope == "global")
	ctx = coordination.WithScope(ctx, scope)
	return ctx, scope, finish, nil
}

func (e *Engine) ManageWorldbook(ctx context.Context, request Request, input WorldbookMutation) (WorldbookMutationResponse, error) {
	if input.RequestID == "" || len(input.RequestID) > 128 || input.ExpectedScope == nil || input.RoleID == "" || input.ExpectedRevision < 0 {
		return WorldbookMutationResponse{}, errors.New("世界书保存参数无效")
	}
	if input.Action != "create" && input.Action != "update" && input.Action != "delete" {
		return WorldbookMutationResponse{}, errors.New("世界书操作无效")
	}
	rule := input.Rule
	var err error
	if input.Action == "delete" {
		rule.ID = strings.TrimSpace(rule.ID)
		rule.Scope = strings.ToLower(strings.TrimSpace(rule.Scope))
		if rule.Scope != "global" && rule.Scope != "role" {
			return WorldbookMutationResponse{}, errors.New("世界书作用域必须为 global 或 role")
		}
	} else {
		rule, err = normalizeWorldbookRule(rule, input.RoleID)
		if err != nil {
			return WorldbookMutationResponse{}, err
		}
	}
	if input.Action != "create" && (strings.TrimSpace(rule.ID) == "" || input.ExpectedRevision < 1) {
		return WorldbookMutationResponse{}, errors.New("世界书修改缺少原记录版本")
	}
	request.TargetDeviceID = input.TargetDeviceID
	request.RoleID = input.RoleID
	request.RequestID = input.RequestID
	request.ExpectedScope = input.ExpectedScope
	ctx, scope, finish, err := e.worldbookScope(ctx, request, input)
	if err != nil {
		return WorldbookMutationResponse{}, err
	}
	defer finish()
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return WorldbookMutationResponse{}, errors.New("数据来源不支持世界书管理")
	}
	storageRole := coordination.StorageRole(scope, "worldbook")
	unlock := e.lock(scope.SpaceID + "\x00" + scope.ResourceOwnerID + "\x00worldbook\x00" + input.RequestID)
	defer unlock()
	digest := sha256.Sum256(body(struct {
		Scope coordination.ExecutionScope
		Input WorldbookMutation
	}{scope, input}))
	fingerprint := hex.EncodeToString(digest[:])
	receiptID := "worldbook/" + input.RequestID
	if pending, _, err := e.pendingMutation(ctx, scope, receiptID, fingerprint); err != nil {
		return WorldbookMutationResponse{}, err
	} else if pending != nil {
		if _, err := e.commit(ctx, *pending); err != nil {
			return WorldbookMutationResponse{}, err
		}
	}
	if receipt, err := port.Resource(ctx, scope, "checkpoint", receiptID); err != nil {
		return WorldbookMutationResponse{}, err
	} else if receipt != nil {
		var saved struct {
			Hash     string                    `json:"hash"`
			Response WorldbookMutationResponse `json:"response"`
		}
		if receipt.Deleted || json.Unmarshal(receipt.Body, &saved) != nil || saved.Hash != fingerprint || saved.Response.Acknowledgement.OwnerID != scope.ResourceOwnerID {
			return WorldbookMutationResponse{}, coordination.ErrRequestConflict
		}
		if err := e.coordination.Validate(ctx, scope); err != nil {
			return WorldbookMutationResponse{}, err
		}
		return saved.Response, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	deleted := input.Action == "delete"
	expectedRevision := int64(0)
	if input.Action == "create" {
		identity := sha256.Sum256([]byte(scope.CoreID + "\x00" + scope.InitiatorDeviceID + "\x00" + scope.ResourceOwnerID + "\x00" + input.RequestID))
		rule.ID = "worldbook-" + hex.EncodeToString(identity[:16])
		rule.CreatedAt, rule.UpdatedAt = now, now
	} else {
		existing, err := port.Resource(ctx, scope, "worldbook", rule.ID)
		if err != nil {
			return WorldbookMutationResponse{}, err
		}
		if existing == nil || existing.Deleted || existing.RoleID != storageRole || existing.Revision != input.ExpectedRevision {
			return WorldbookMutationResponse{}, coordination.ErrResourceVersion
		}
		expectedRevision = existing.Revision
		var previous WorldbookRule
		if json.Unmarshal(existing.Body, &previous) != nil {
			return WorldbookMutationResponse{}, errors.New("世界书资源格式无效")
		}
		rule.CreatedAt = previous.CreatedAt
		rule.UpdatedAt = now
	}
	resourceBody := body(rule)
	if deleted {
		resourceBody = json.RawMessage(`null`)
	}
	view := WorldbookRuleView{WorldbookRule: rule, OwnerID: scope.ResourceOwnerID, RoleID: storageRole, Revision: expectedRevision + 1}
	response := WorldbookMutationResponse{Scope: scope, Saved: true, Acknowledgement: coordination.Acknowledgement{RequestID: scope.RequestID, OwnerID: scope.ResourceOwnerID, Versions: map[string]int64{"worldbook/" + rule.ID: expectedRevision + 1, "checkpoint/" + receiptID: 1}}}
	if deleted {
		response.Rule = nil
	} else {
		response.Rule = &view
	}
	commit := coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{
		{Kind: "worldbook", ID: rule.ID, RoleID: storageRole, ExpectedRevision: expectedRevision, Deleted: deleted, Body: resourceBody},
		{Kind: "checkpoint", ID: receiptID, RoleID: scope.RoleID, Body: body(map[string]any{"hash": fingerprint, "response": response})},
	}}
	ack, err := e.commit(ctx, commit)
	if err != nil {
		if !errors.Is(err, coordination.ErrResourceVersion) && !errors.Is(err, coordination.ErrRequestConflict) && !errors.Is(err, coordination.ErrScopeExpired) {
			if queueErr := e.coordination.Enqueue(ctx, commit); queueErr != nil {
				return WorldbookMutationResponse{}, errors.Join(err, queueErr)
			}
		}
		return WorldbookMutationResponse{}, err
	}
	if ack.OwnerID != scope.ResourceOwnerID || ack.RequestID != scope.RequestID || ack.Versions["worldbook/"+rule.ID] != expectedRevision+1 || ack.Versions["checkpoint/"+receiptID] != 1 {
		return WorldbookMutationResponse{}, errors.New("世界书尚未获得数据所有者保存确认")
	}
	response.Acknowledgement = ack
	return response, nil
}

func (e *Engine) MatchWorldbook(ctx context.Context, request Request, roleID, targetDeviceID string, expected *coordination.ExecutionScope, text string) (WorldbookMatchResponse, error) {
	if expected == nil || strings.TrimSpace(roleID) == "" || strings.TrimSpace(text) == "" || len(text) > 128<<10 || !utf8.ValidString(text) {
		return WorldbookMatchResponse{}, errors.New("世界书匹配参数无效")
	}
	input := WorldbookMutation{RequestID: request.RequestID, RoleID: roleID, TargetDeviceID: targetDeviceID, ExpectedScope: expected, Action: "match", Rule: WorldbookRule{Scope: "role"}}
	ctx, scope, finish, err := e.worldbookScope(ctx, request, input)
	if err != nil {
		return WorldbookMatchResponse{}, err
	}
	defer finish()
	partitionScope := scope
	partitionScope.WorldbookRoleID = ""
	snapshot, err := e.data.Snapshot(ctx, partitionScope, coordination.DataQuery{Management: true, ResourceKind: "worldbook", Limit: 128})
	if err != nil {
		return WorldbookMatchResponse{}, err
	}
	if err := coordination.ValidateSnapshot(partitionScope, snapshot); err != nil {
		return WorldbookMatchResponse{}, err
	}
	resources := snapshot.Resources
	rules := make([]worldbook.WorldBookEntry, 0, len(resources))
	revisions := map[string]int64{}
	for _, resource := range resources {
		var stored WorldbookRule
		if json.Unmarshal(resource.Body, &stored) != nil || !stored.Enabled {
			continue
		}
		rule, err := worldbookResourceRule(resource)
		if err != nil {
			continue
		}
		rules = append(rules, rule)
		revisions[resource.ID] = resource.Revision
	}
	matched := worldbook.MatchRules(rules, text, "")
	matches := make([]WorldbookMatch, 0, len(matched))
	for _, item := range matched {
		scopeName := "role"
		if strings.TrimSpace(item.Entry.CharacterID) == "" {
			scopeName = "global"
		}
		matches = append(matches, WorldbookMatch{RuleID: item.Entry.ID, Revision: revisions[item.Entry.ID], Scope: scopeName, MatchScope: item.MatchScope, HitText: item.HitText})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].RuleID < matches[j].RuleID })
	return WorldbookMatchResponse{Matches: matches, Scope: scope}, nil
}
