package permission

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type mockPermissionBroker struct {
	evalResult PermissionEvaluationResult
	evalCalled bool
	evalReq    PermissionEvaluationRequest
}

func (m *mockPermissionBroker) Evaluate(_ context.Context, req PermissionEvaluationRequest) PermissionEvaluationResult {
	m.evalCalled = true
	m.evalReq = req
	return m.evalResult
}

func (m *mockPermissionBroker) Grant(_ context.Context, _ PermissionGrantRequest) (PermissionGrant, error) {
	return PermissionGrant{}, nil
}

func (m *mockPermissionBroker) Revoke(_ context.Context, _ string) error { return nil }

func (m *mockPermissionBroker) RevokeBySubject(_ context.Context, _ PermissionSubject) (int, error) {
	return 0, nil
}

func (m *mockPermissionBroker) RevokeByExtension(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockPermissionBroker) ListGrants(_ context.Context, _ PermissionGrantFilter) ([]PermissionGrant, error) {
	return nil, nil
}

func (m *mockPermissionBroker) Explain(_ context.Context, _ PermissionEvaluationRequest) PermissionExplanation {
	return PermissionExplanation{}
}

func (m *mockPermissionBroker) DetectUpgrade(_ context.Context, _, _ []PermissionRequirement) []PermissionUpgrade {
	return nil
}

type mockScopeManager struct {
	evalResult scope.ScopeDecision
	evalCalled bool
	evalReq    scope.ScopeEvaluationRequest
}

func (m *mockScopeManager) Bind(_ context.Context, _ scope.ScopeBindRequest) (scope.ScopeBinding, error) {
	return scope.ScopeBinding{}, nil
}

func (m *mockScopeManager) Unbind(_ context.Context, _ string) error { return nil }

func (m *mockScopeManager) Evaluate(_ context.Context, req scope.ScopeEvaluationRequest) scope.ScopeDecision {
	m.evalCalled = true
	m.evalReq = req
	return m.evalResult
}

func (m *mockScopeManager) Snapshot(_ context.Context, _ scope.ScopeResolveRequest) (scope.ScopeSnapshot, error) {
	return scope.ScopeSnapshot{}, nil
}

func (m *mockScopeManager) Invalidate(_ context.Context, _ scope.ScopeInvalidationFilter) error {
	return nil
}

func (m *mockScopeManager) ListBindings(_ context.Context, _ scope.ScopeBindingFilter) ([]scope.ScopeBinding, error) {
	return nil, nil
}

func makeAuthTestDefinition() *ui_contribution.UIContributionDefinition {
	return &ui_contribution.UIContributionDefinition{
		ContributionID:  "contrib-auth-1",
		ExtensionID:     "ext-auth-1",
		ModuleID:        "mod-auth-1",
		Kind:            ui_contribution.UIContributionSchemaPage,
		Slot:            ui_contribution.UISlotReference{SlotID: "extension.settings.page", ContractVersion: 1},
		ContractVersion: 1,
		Display: ui_contribution.UIDisplayMetadata{
			Title:       ui_contribution.LocalizedText{Default: "Test"},
			Description: ui_contribution.LocalizedText{Default: "Test extension"},
		},
		Entry: ui_contribution.UIEntryDefinition{
			Type:        ui_contribution.SandboxSchemaRenderer,
			Path:        "ui/settings.json",
			ContentHash: "sha256:abc",
		},
		Sandbox:   ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxSchemaRenderer},
		Lifecycle: ui_contribution.UILifecyclePolicy{Initial: string(ui_contribution.UIStateRegistered)},
		Integrity: ui_contribution.ContributionIntegrity{DefinitionHash: "sha256:def", Generation: 1},
	}
}

func TestAuthorizeSessionNoRequirements(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()

	result, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.GrantedPerms) != 0 {
		t.Fatalf("expected no granted perms, got %v", result.GrantedPerms)
	}
	if len(result.GrantedScopes) != 0 {
		t.Fatalf("expected no granted scopes, got %v", result.GrantedScopes)
	}
}

func TestAuthorizeSessionWithPermissionsAllowed(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{
			Decision: DecisionAllow,
			Missing:  []PermissionRequirement{},
		},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Scope: "extension", Required: true},
		{Name: "data.read", Scope: "character", Required: true},
	}

	result, err := auth.AuthorizeSession(context.Background(), def, "char-1", "conv-1")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(result.GrantedPerms) != 2 {
		t.Fatalf("expected 2 granted perms, got %d", len(result.GrantedPerms))
	}
	if !containsString(result.GrantedPerms, "tool.invoke") {
		t.Fatalf("expected tool.invoke in granted perms, got %v", result.GrantedPerms)
	}
	if !containsString(result.GrantedPerms, "data.read") {
		t.Fatalf("expected data.read in granted perms, got %v", result.GrantedPerms)
	}
	if !broker.evalCalled {
		t.Fatal("expected broker.Evaluate to be called")
	}
	if len(broker.evalReq.Requirements) != 2 {
		t.Fatalf("expected 2 requirements in eval request, got %d", len(broker.evalReq.Requirements))
	}
}

func TestAuthorizeSessionPermissionDenied(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{
			Decision: DecisionDeny,
			Missing: []PermissionRequirement{
				{PermissionID: "tool.invoke"},
			},
		},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Scope: "extension", Required: true},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err == nil {
		t.Fatal("expected permission denied error, got nil")
	}
	if !containsStr(err.Error(), "permission denied") {
		t.Fatalf("expected 'permission denied' in error, got: %s", err.Error())
	}
}

func TestAuthorizeSessionRequireApprovalDenied(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{
			Decision: DecisionRequireApproval,
			Missing: []PermissionRequirement{
				{PermissionID: "tool.invoke"},
			},
		},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Required: true},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err == nil {
		t.Fatal("expected permission denied error for require_approval, got nil")
	}
}

func TestAuthorizeSessionNilBrokerFailClosed(t *testing.T) {
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(nil, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Required: true},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err == nil {
		t.Fatal("expected fail-closed error when broker is nil, got nil")
	}
	if !containsStr(err.Error(), "broker not configured") {
		t.Fatalf("expected 'broker not configured' in error, got: %s", err.Error())
	}
}

func TestAuthorizeSessionScopeDenied(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{
			Allowed: false,
			Reasons: []scope.ScopeReason{{Code: "no_binding"}},
		},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.ScopeRule = ui_contribution.ScopeRule{
		RequiredScopes: []string{"character.profile"},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "char-1", "conv-1")
	if err == nil {
		t.Fatal("expected scope denied error, got nil")
	}
	if !containsStr(err.Error(), "scope denied") {
		t.Fatalf("expected 'scope denied' in error, got: %s", err.Error())
	}
}

func TestAuthorizeSessionNilScopeManagerFailClosed(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	auth := NewUISessionAuthorizer(broker, nil)
	def := makeAuthTestDefinition()
	def.ScopeRule = ui_contribution.ScopeRule{
		RequiredScopes: []string{"character.profile"},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "char-1", "conv-1")
	if err == nil {
		t.Fatal("expected fail-closed error when scope manager is nil, got nil")
	}
	if !containsStr(err.Error(), "scope manager not configured") {
		t.Fatalf("expected 'scope manager not configured' in error, got: %s", err.Error())
	}
}

func TestAuthorizeSessionScopeAllowedWithCharacterContext(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.ScopeRule = ui_contribution.ScopeRule{
		RequiredScopes: []string{"character.profile"},
	}

	result, err := auth.AuthorizeSession(context.Background(), def, "char-1", "conv-1")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !containsString(result.GrantedScopes, "character.profile") {
		t.Fatalf("expected character.profile in granted scopes, got %v", result.GrantedScopes)
	}
	if !scopeMgr.evalCalled {
		t.Fatal("expected scope manager.Evaluate to be called")
	}
	if scopeMgr.evalReq.CharacterID != "char-1" {
		t.Fatalf("expected CharacterID=char-1, got %s", scopeMgr.evalReq.CharacterID)
	}
	if scopeMgr.evalReq.ConversationID != "conv-1" {
		t.Fatalf("expected ConversationID=conv-1, got %s", scopeMgr.evalReq.ConversationID)
	}
	if scopeMgr.evalReq.ExtensionID != "ext-auth-1" {
		t.Fatalf("expected ExtensionID=ext-auth-1, got %s", scopeMgr.evalReq.ExtensionID)
	}
}

func TestAuthorizeSessionNilDefinition(t *testing.T) {
	broker := &mockPermissionBroker{}
	scopeMgr := &mockScopeManager{}
	auth := NewUISessionAuthorizer(broker, scopeMgr)

	_, err := auth.AuthorizeSession(context.Background(), nil, "", "")
	if err == nil {
		t.Fatal("expected error for nil definition, got nil")
	}
	if !containsStr(err.Error(), "nil definition") {
		t.Fatalf("expected 'nil definition' in error, got: %s", err.Error())
	}
}

func TestAuthorizeSessionNilAuthorizer(t *testing.T) {
	var auth *UISessionAuthorizer
	def := makeAuthTestDefinition()

	_, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err == nil {
		t.Fatal("expected error for nil authorizer, got nil")
	}
	if !containsStr(err.Error(), "not configured") {
		t.Fatalf("expected 'not configured' in error, got: %s", err.Error())
	}
}

func TestAuthorizeSessionOptionalPermissionsNotRequired(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Required: true},
		{Name: "data.export", Required: false},
	}

	result, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !containsString(result.GrantedPerms, "tool.invoke") {
		t.Fatalf("expected tool.invoke in granted perms, got %v", result.GrantedPerms)
	}
	if containsString(result.GrantedPerms, "data.export") {
		t.Fatalf("expected data.export NOT in granted perms (not required), got %v", result.GrantedPerms)
	}
	if len(broker.evalReq.Requirements) != 1 {
		t.Fatalf("expected 1 requirement in eval request (only required), got %d", len(broker.evalReq.Requirements))
	}
}

func TestAuthorizeSessionAllowOnceDecision(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllowOnce},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Required: true},
	}

	result, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err != nil {
		t.Fatalf("expected success with allow_once, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestAuthorizeSessionScopeEvaluationWithoutRequiredScopes(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: false},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()

	result, err := auth.AuthorizeSession(context.Background(), def, "char-1", "")
	if err != nil {
		t.Fatalf("expected success when no required scopes and scope denied but no required scopes, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestResolvePermScope(t *testing.T) {
	cases := []struct {
		scopeStr      string
		characterID   string
		conversationID string
		extID         string
		expectedType  ScopeType
	}{
		{"character", "char-1", "", "ext-1", ScopeCharacter},
		{"Character", "char-1", "", "ext-1", ScopeCharacter},
		{"conversation", "", "conv-1", "ext-1", ScopeConversation},
		{"Conversation", "", "conv-1", "ext-1", ScopeConversation},
		{"extension", "char-1", "conv-1", "ext-1", ScopeExtension},
		{"Extension", "char-1", "conv-1", "ext-1", ScopeExtension},
		{"unknown", "char-1", "conv-1", "ext-1", ScopeGlobal},
		{"", "char-1", "conv-1", "ext-1", ScopeGlobal},
	}
	for _, c := range cases {
		s := resolvePermScope(c.scopeStr, c.characterID, c.conversationID, c.extID)
		if s.Type != c.expectedType {
			t.Fatalf("for scopeStr=%q, expected %s, got %s", c.scopeStr, c.expectedType, s.Type)
		}
		switch c.expectedType {
		case ScopeCharacter:
			if s.CharacterID != c.characterID {
				t.Fatalf("expected CharacterID=%s, got %s", c.characterID, s.CharacterID)
			}
		case ScopeConversation:
			if s.ConversationID != c.conversationID {
				t.Fatalf("expected ConversationID=%s, got %s", c.conversationID, s.ConversationID)
			}
		case ScopeExtension:
			if s.ExtensionID != c.extID {
				t.Fatalf("expected ExtensionID=%s, got %s", c.extID, s.ExtensionID)
			}
		}
	}
}

func TestAuthorizeSessionBrokerSubjectIsExtension(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Required: true},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if broker.evalReq.Subject.Type != SubjectExtension {
		t.Fatalf("expected subject type=extension, got %s", broker.evalReq.Subject.Type)
	}
	if broker.evalReq.Subject.ID != "ext-auth-1" {
		t.Fatalf("expected subject id=ext-auth-1, got %s", broker.evalReq.Subject.ID)
	}
	if broker.evalReq.Subject.ExtensionID != "ext-auth-1" {
		t.Fatalf("expected subject extensionID=ext-auth-1, got %s", broker.evalReq.Subject.ExtensionID)
	}
}

func TestAuthorizeSessionScopeSubjectIsUIContribution(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{Decision: DecisionAllow},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.ScopeRule = ui_contribution.ScopeRule{
		RequiredScopes: []string{"character.profile"},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "char-1", "conv-1")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if scopeMgr.evalReq.SubjectType != scope.SubjectUIContribution {
		t.Fatalf("expected subject type=ui_contribution, got %s", scopeMgr.evalReq.SubjectType)
	}
	if scopeMgr.evalReq.SubjectID != "ext-auth-1" {
		t.Fatalf("expected subject id=ext-auth-1, got %s", scopeMgr.evalReq.SubjectID)
	}
	if scopeMgr.evalReq.Generation != 1 {
		t.Fatalf("expected generation=1, got %d", scopeMgr.evalReq.Generation)
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestAuthorizeSessionClientFakePermissionsIgnored(t *testing.T) {
	broker := &mockPermissionBroker{
		evalResult: PermissionEvaluationResult{
			Decision: DecisionDeny,
			Missing: []PermissionRequirement{
				{PermissionID: "tool.invoke"},
			},
		},
	}
	scopeMgr := &mockScopeManager{
		evalResult: scope.ScopeDecision{Allowed: true},
	}
	auth := NewUISessionAuthorizer(broker, scopeMgr)
	def := makeAuthTestDefinition()
	def.Permissions = []ui_contribution.PermissionRequirement{
		{Name: "tool.invoke", Required: true},
	}

	_, err := auth.AuthorizeSession(context.Background(), def, "", "")
	if err == nil {
		t.Fatal("expected denial even though client might have faked permissions")
	}
}
