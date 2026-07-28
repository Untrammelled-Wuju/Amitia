package scope

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestScopeRef_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ref     ScopeRef
		wantErr bool
	}{
		{"valid global", NewGlobalScope(), false},
		{"valid character", NewCharacterScope("char-1"), false},
		{"valid conversation", NewConversationScope("conv-1"), false},
		{"valid extension", NewExtensionScope("ext-1"), false},
		{"valid module", NewModuleScope("ext-1", "mod-1"), false},
		{"valid resource", NewResourceScope("file", "res-1"), false},
		{"valid invocation", NewInvocationScope("inv-1"), false},
		{"valid session", NewSessionScope("sess-1"), false},
		{"empty type", ScopeRef{}, true},
		{"character without id", ScopeRef{Type: ScopeCharacter}, true},
		{"conversation without id", ScopeRef{Type: ScopeConversation}, true},
		{"extension without id", ScopeRef{Type: ScopeExtension}, true},
		{"module missing ext", ScopeRef{Type: ScopeModule, ModuleID: "mod-1"}, true},
		{"module missing mod", ScopeRef{Type: ScopeModule, ExtensionID: "ext-1"}, true},
		{"resource without type", ScopeRef{Type: ScopeResource, ResourceID: "res-1"}, true},
		{"unknown type", ScopeRef{Type: "unknown"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScopeRef_Contains(t *testing.T) {
	global := NewGlobalScope()

	tests := []struct {
		name   string
		parent ScopeRef
		child  ScopeRef
		want   bool
	}{
		{"global contains character", global, NewCharacterScope("c1"), true},
		{"global contains conversation", global, NewConversationScope("c1"), true},
		{"same character", NewCharacterScope("c1"), NewCharacterScope("c1"), true},
		{"different character", NewCharacterScope("c1"), NewCharacterScope("c2"), false},
		{"different type", NewCharacterScope("c1"), NewConversationScope("c1"), false},
		{"same module", NewModuleScope("e1", "m1"), NewModuleScope("e1", "m1"), true},
		{"different module", NewModuleScope("e1", "m1"), NewModuleScope("e1", "m2"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.parent.Contains(tt.child); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScopeBinding(t *testing.T) {
	scope := NewCharacterScope("char-1")
	binding, err := NewBinding(SubjectTool, "tool-1", scope, SourceSystem)
	if err != nil {
		t.Fatalf("NewBinding() error = %v", err)
	}

	if binding.BindingID == "" {
		t.Error("binding ID should not be empty")
	}
	if !binding.IsActive() {
		t.Error("new binding should be active")
	}

	binding.Revoke()
	if binding.IsActive() {
		t.Error("revoked binding should not be active")
	}

	binding.Activate()
	if !binding.IsActive() {
		t.Error("reactivated binding should be active")
	}

	expired := time.Now().Add(-time.Hour)
	binding.SetExpiry(expired)
	if binding.IsActive() {
		t.Error("expired binding should not be active")
	}
}

func TestScopeBinding_Invalid(t *testing.T) {
	_, err := NewBinding("", "id", NewGlobalScope(), SourceSystem)
	if err == nil {
		t.Error("expected error for empty subject type")
	}

	_, err = NewBinding(SubjectTool, "", NewGlobalScope(), SourceSystem)
	if err == nil {
		t.Error("expected error for empty subject ID")
	}

	_, err = NewBinding(SubjectTool, "id", ScopeRef{}, SourceSystem)
	if err == nil {
		t.Error("expected error for invalid scope")
	}
}

func TestScopeExpression(t *testing.T) {
	single := SingleScope(NewCharacterScope("c1"))
	if err := single.Validate(); err != nil {
		t.Errorf("single expression should be valid: %v", err)
	}

	all := AllOf(NewCharacterScope("c1"), NewExtensionScope("e1"))
	if err := all.Validate(); err != nil {
		t.Errorf("all expression should be valid: %v", err)
	}

	any := AnyOf(NewCharacterScope("c1"), NewCharacterScope("c2"))
	if err := any.Validate(); err != nil {
		t.Errorf("any expression should be valid: %v", err)
	}

	invalid := ScopeExpression{Operator: "INVALID"}
	if err := invalid.Validate(); err == nil {
		t.Error("invalid operator should return error")
	}
}

func TestScopeResolution(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		expr    ScopeExpression
		req     ScopeResolveRequest
		wantErr bool
		check   func(t *testing.T, scopes []ScopeRef)
	}{
		{
			name: "resolve current character",
			expr: WithPlaceholder(ScopeExpression{Operator: OpAND}, PHCurrentCharacter),
			req:  ScopeResolveRequest{CharacterID: "char-1"},
			check: func(t *testing.T, scopes []ScopeRef) {
				if len(scopes) != 1 || scopes[0].CharacterID != "char-1" {
					t.Errorf("expected char-1, got %+v", scopes)
				}
			},
		},
		{
			name:    "resolve current character without id",
			expr:    WithPlaceholder(ScopeExpression{Operator: OpAND}, PHCurrentCharacter),
			req:     ScopeResolveRequest{},
			wantErr: true,
		},
		{
			name: "resolve global scope",
			expr: SingleScope(NewGlobalScope()),
			req:  ScopeResolveRequest{},
			check: func(t *testing.T, scopes []ScopeRef) {
				if len(scopes) != 1 || !scopes[0].IsGlobal() {
					t.Error("expected global scope")
				}
			},
		},
		{
			name: "resolve owner extension",
			expr: WithPlaceholder(ScopeExpression{Operator: OpAND}, PHOwnerExtension),
			req:  ScopeResolveRequest{ExtensionID: "ext-1"},
			check: func(t *testing.T, scopes []ScopeRef) {
				if len(scopes) != 1 || scopes[0].ExtensionID != "ext-1" {
					t.Errorf("expected ext-1, got %+v", scopes)
				}
			},
		},
		{
			name:    "resolve with invalid expression",
			expr:    ScopeExpression{Operator: "INVALID"},
			req:     ScopeResolveRequest{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopes, err := ResolveScope(ctx, ScopeResolveRequest{
				Expression:     tt.expr,
				CharacterID:    tt.req.CharacterID,
				ConversationID: tt.req.ConversationID,
				ExtensionID:    tt.req.ExtensionID,
				ModuleID:       tt.req.ModuleID,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveScope() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.check != nil {
				tt.check(t, scopes)
			}
		})
	}
}

func TestScopeManager_BindAndEvaluate(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)

	binding, err := manager.Bind(ctx, ScopeBindRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		Scope:       NewCharacterScope("char-1"),
		Source:      SourceSystem,
	})
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if binding.BindingID == "" {
		t.Error("binding should have an ID")
	}

	decision := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		CharacterID: "char-1",
	})
	if !decision.Allowed {
		t.Errorf("expected allowed for matching character, got reasons: %+v", decision.Reasons)
	}

	decision2 := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		CharacterID: "char-2",
	})
	if decision2.Allowed {
		t.Error("expected denied for non-matching character")
	}

	if err := manager.Unbind(ctx, binding.BindingID); err != nil {
		t.Fatalf("Unbind() error = %v", err)
	}

	decision3 := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		CharacterID: "char-1",
	})
	if decision3.Allowed {
		t.Error("expected denied after unbind")
	}
}

func TestScopeManager_GlobalScope(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)

	_, err := manager.Bind(ctx, ScopeBindRequest{
		SubjectType: SubjectTool,
		SubjectID:   "global-tool",
		Scope:       NewGlobalScope(),
		Source:      SourceSystem,
	})
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}

	decision := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "global-tool",
		CharacterID: "any-char",
	})
	if !decision.Allowed {
		t.Error("global scope should allow any character")
	}
}

func TestScopeManager_Snapshot(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)

	snapshot, err := manager.Snapshot(ctx, ScopeResolveRequest{
		Expression:   SingleScope(NewCharacterScope("char-1")),
		CharacterID:  "char-1",
		InvocationID: "inv-1",
	})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.InvocationID != "inv-1" {
		t.Errorf("expected inv-1, got %s", snapshot.InvocationID)
	}
	if !snapshot.Contains(NewCharacterScope("char-1")) {
		t.Error("snapshot should contain character scope")
	}
	if snapshot.Contains(NewCharacterScope("char-2")) {
		t.Error("snapshot should not contain different character")
	}
}

func TestScopeManager_Invalidation(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)

	_, err := manager.Bind(ctx, ScopeBindRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		Scope:       NewCharacterScope("char-1"),
		Source:      SourceSystem,
	})
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}

	decision := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		CharacterID: "char-1",
	})
	if !decision.Allowed {
		t.Fatal("expected allowed before invalidation")
	}

	if err := manager.Invalidate(ctx, ScopeInvalidationFilter{CharacterID: "char-1"}); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}

	bindings, _ := manager.ListBindings(ctx, ScopeBindingFilter{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
	})
	for _, b := range bindings {
		if b.State != StateRevoked {
			t.Errorf("expected revoked state, got %s", b.State)
		}
	}
}

func TestScopeManager_Cache(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)

	_, err := manager.Bind(ctx, ScopeBindRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		Scope:       NewCharacterScope("char-1"),
		Source:      SourceSystem,
	})
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}

	_ = manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		CharacterID: "char-1",
	})

	bindings, err := manager.ListBindings(ctx, ScopeBindingFilter{SubjectType: SubjectTool})
	if err != nil {
		t.Fatalf("ListBindings() error = %v", err)
	}

	manager.cache.InvalidateSubject(SubjectTool, "tool-1")
	_ = manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		CharacterID: "char-1",
	})

	_ = bindings
}

func TestScopeManager_NoBinding(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)

	decision := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType: SubjectTool,
		SubjectID:   "nonexistent",
	})
	if decision.Allowed {
		t.Error("expected denied for nonexistent tool")
	}
}

func TestScopeEvaluator_StrictContextRelations(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &strictRelationChecker{
		conversationOwned: true,
		resourceOwned:     true,
		invocationExists:  true,
		invocationChild:   true,
		sessionValid:      true,
	}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)
	tests := []struct {
		name    string
		scope   ScopeRef
		request ScopeEvaluationRequest
		allowed bool
	}{
		{name: "resource missing id", scope: ScopeRef{Type: ScopeResource, ExtensionID: "ext-1", ResourceType: "file", ResourceID: "res-1"}, request: ScopeEvaluationRequest{ExtensionID: "ext-1", ResourceType: "file"}},
		{name: "resource exact", scope: ScopeRef{Type: ScopeResource, ExtensionID: "ext-1", ResourceType: "file", ResourceID: "res-1"}, request: ScopeEvaluationRequest{ExtensionID: "ext-1", ResourceType: "file", ResourceID: "res-1"}, allowed: true},
		{name: "invocation missing id", scope: ScopeRef{Type: ScopeInvocation, ExtensionID: "ext-1", InvocationID: "inv-1"}, request: ScopeEvaluationRequest{ExtensionID: "ext-1"}},
		{name: "invocation exact", scope: ScopeRef{Type: ScopeInvocation, ExtensionID: "ext-1", InvocationID: "inv-1"}, request: ScopeEvaluationRequest{ExtensionID: "ext-1", InvocationID: "inv-1"}, allowed: true},
		{name: "session missing id", scope: ScopeRef{Type: ScopeSession, ExtensionID: "ext-1", SessionID: "sess-1"}, request: ScopeEvaluationRequest{ExtensionID: "ext-1", Generation: 2}},
		{name: "session exact generation", scope: ScopeRef{Type: ScopeSession, ExtensionID: "ext-1", SessionID: "sess-1"}, request: ScopeEvaluationRequest{ExtensionID: "ext-1", SessionID: "sess-1", Generation: 2}, allowed: true},
	}
	for index, item := range tests {
		subjectID := fmt.Sprintf("strict-%d", index)
		if _, err := manager.Bind(ctx, ScopeBindRequest{SubjectType: SubjectTool, SubjectID: subjectID, Scope: item.scope, Source: SourceSystem}); err != nil {
			t.Fatalf("bind %s: %v", item.name, err)
		}
		item.request.SubjectType = SubjectTool
		item.request.SubjectID = subjectID
		decision := manager.Evaluate(ctx, item.request)
		if decision.Allowed != item.allowed {
			t.Fatalf("%s allowed=%v reasons=%v", item.name, decision.Allowed, decision.Reasons)
		}
	}
}

func TestScopeEvaluator_RejectsInvalidRelationsAndAcceptsFutureExpiry(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &strictRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)
	binding, err := manager.Bind(ctx, ScopeBindRequest{SubjectType: SubjectTool, SubjectID: "conversation-tool", Scope: NewGlobalScope(), Source: SourceSystem})
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	binding.ExpiresAt = &future
	if err := store.SaveBinding(ctx, binding); err != nil {
		t.Fatal(err)
	}
	decision := manager.Evaluate(ctx, ScopeEvaluationRequest{SubjectType: SubjectTool, SubjectID: "conversation-tool", CharacterID: "char-1", ConversationID: "conv-1"})
	if decision.Allowed {
		t.Fatal("conversation outside character must be rejected")
	}
	checker.conversationOwned = true
	manager.cache.Clear()
	decision = manager.Evaluate(ctx, ScopeEvaluationRequest{SubjectType: SubjectTool, SubjectID: "conversation-tool", CharacterID: "char-1", ConversationID: "conv-1"})
	if !decision.Allowed {
		t.Fatalf("future expiry must remain active: %v", decision.Reasons)
	}
}

func TestScopeManager_ParentInheritance(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)

	_, err := manager.Bind(ctx, ScopeBindRequest{
		SubjectType: SubjectTool,
		SubjectID:   "child-tool",
		Scope:       NewCharacterScope("char-1"),
		Source:      SourceSystem,
	})
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}

	parentSnapshot := ScopeSnapshot{
		SnapshotID:     "snap-parent",
		ResolvedScopes: []ScopeRef{NewCharacterScope("char-1")},
	}

	decision := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType:    SubjectTool,
		SubjectID:      "child-tool",
		CharacterID:    "char-1",
		ParentSnapshot: &parentSnapshot,
	})
	if !decision.Allowed {
		t.Errorf("expected allowed when parent has same scope, got: %+v", decision.Reasons)
	}

	narrowParent := ScopeSnapshot{
		SnapshotID:     "snap-narrow",
		ResolvedScopes: []ScopeRef{NewCharacterScope("char-2")},
	}

	decision2 := manager.Evaluate(ctx, ScopeEvaluationRequest{
		SubjectType:    SubjectTool,
		SubjectID:      "child-tool",
		CharacterID:    "char-1",
		ParentSnapshot: &narrowParent,
	})
	if decision2.Allowed {
		t.Error("expected denied when parent has narrower scope")
	}
}

func TestMemoryScopeStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()

	binding := ScopeBinding{
		BindingID:   "b-1",
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		Scope:       NewGlobalScope(),
		State:       StateActive,
	}

	if err := store.SaveBinding(ctx, binding); err != nil {
		t.Fatalf("SaveBinding() error = %v", err)
	}

	got, err := store.GetBinding(ctx, "b-1")
	if err != nil {
		t.Fatalf("GetBinding() error = %v", err)
	}
	if got.BindingID != "b-1" {
		t.Errorf("expected b-1, got %s", got.BindingID)
	}

	_, err = store.GetBinding(ctx, "nonexistent")
	if err != ErrBindingNotFound {
		t.Errorf("expected ErrBindingNotFound, got %v", err)
	}

	bindings, err := store.ListBindings(ctx, ScopeBindingFilter{SubjectType: SubjectTool})
	if err != nil {
		t.Fatalf("ListBindings() error = %v", err)
	}
	if len(bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(bindings))
	}

	if err := store.DeleteBinding(ctx, "b-1"); err != nil {
		t.Fatalf("DeleteBinding() error = %v", err)
	}

	bindings, _ = store.ListBindings(ctx, ScopeBindingFilter{})
	if len(bindings) != 0 {
		t.Errorf("expected 0 bindings after delete, got %d", len(bindings))
	}
}

func TestCleanupHandler(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryScopeStore()
	checker := &mockRelationChecker{}
	evaluator := NewScopeEvaluator(store, checker)
	manager := NewScopeManager(store, evaluator)
	handler := NewCleanupHandler(manager, store)

	_, err := manager.Bind(ctx, ScopeBindRequest{
		SubjectType: SubjectTool,
		SubjectID:   "tool-1",
		Scope:       NewCharacterScope("char-1"),
		Source:      SourceSystem,
	})
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}

	if err := handler.OnCharacterDeleted(ctx, "char-1"); err != nil {
		t.Fatalf("OnCharacterDeleted() error = %v", err)
	}

	bindings, _ := manager.ListBindings(ctx, ScopeBindingFilter{SubjectType: SubjectTool})
	for _, b := range bindings {
		if b.State != StateRevoked {
			t.Errorf("expected revoked state after character deletion, got %s", b.State)
		}
	}
}

func TestScopeCache(t *testing.T) {
	cache := NewScopeCacheWithTTL(100 * time.Millisecond)

	decision := ScopeDecision{Allowed: true}
	cache.Set("key1", decision)

	got, ok := cache.Get("key1")
	if !ok || !got.Allowed {
		t.Error("cache should return stored decision")
	}

	_, ok = cache.Get("key2")
	if ok {
		t.Error("cache should not return nonexistent key")
	}

	time.Sleep(150 * time.Millisecond)
	_, ok = cache.Get("key1")
	if ok {
		t.Error("cache should expire entries")
	}

	cache.Set("key-to-invalidate", decision)
	cache.InvalidateSubject(SubjectTool, "tool-1")

	cache.Set("key-tool-2", decision)
	_, _ = cache.Get("key-tool-2")

	cache.Clear()
	_, ok = cache.Get("key-tool-2")
	if ok {
		t.Error("cache should be empty after clear")
	}
}

type mockRelationChecker struct{}

func (m *mockRelationChecker) ConversationBelongsToCharacter(context.Context, string, string) bool {
	return true
}

func (m *mockRelationChecker) IsCharacterDeleted(context.Context, string) bool {
	return false
}

func (m *mockRelationChecker) IsConversationDeleted(context.Context, string) bool {
	return false
}

func (m *mockRelationChecker) ResourceOwnedBy(context.Context, string, string, string, string) bool {
	return true
}
func (m *mockRelationChecker) InvocationOwnedBy(context.Context, string, string, string) bool {
	return true
}
func (m *mockRelationChecker) InvocationIsChildOf(context.Context, string, string) bool { return true }
func (m *mockRelationChecker) SessionValid(context.Context, string, string, string, int64) bool {
	return true
}

type strictRelationChecker struct {
	conversationOwned bool
	resourceOwned     bool
	invocationExists  bool
	invocationChild   bool
	sessionValid      bool
}

func (c *strictRelationChecker) ConversationBelongsToCharacter(context.Context, string, string) bool {
	return c.conversationOwned
}
func (c *strictRelationChecker) IsCharacterDeleted(context.Context, string) bool    { return false }
func (c *strictRelationChecker) IsConversationDeleted(context.Context, string) bool { return false }
func (c *strictRelationChecker) ResourceOwnedBy(context.Context, string, string, string, string) bool {
	return c.resourceOwned
}
func (c *strictRelationChecker) InvocationOwnedBy(context.Context, string, string, string) bool {
	return c.invocationExists
}
func (c *strictRelationChecker) InvocationIsChildOf(context.Context, string, string) bool {
	return c.invocationChild
}
func (c *strictRelationChecker) SessionValid(context.Context, string, string, string, int64) bool {
	return c.sessionValid
}
