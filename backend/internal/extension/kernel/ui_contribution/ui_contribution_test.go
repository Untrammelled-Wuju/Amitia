package ui_contribution

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func makeValidDefinition() *UIContributionDefinition {
	return &UIContributionDefinition{
		ContributionID:  "contrib-1",
		ExtensionID:     "ext-1",
		ModuleID:        "mod-1",
		Kind:            UIContributionSchemaPage,
		Slot:            UISlotReference{SlotID: "extension.settings.page", ContractVersion: 1},
		ContractVersion: 1,
		Display: UIDisplayMetadata{
			Title:       LocalizedText{Default: "Settings"},
			Description: LocalizedText{Default: "My extension settings"},
		},
		Entry: UIEntryDefinition{
			Type:        SandboxSchemaRenderer,
			Path:        "ui/settings.json",
			ContentHash: "sha256:abc",
		},
		Sandbox:   UISandboxPolicy{Type: SandboxSchemaRenderer},
		Lifecycle: UILifecyclePolicy{Initial: string(UIStateRegistered)},
		Integrity: ContributionIntegrity{DefinitionHash: "sha256:def", Generation: 1},
	}
}

func TestValidateDefinitionValid(t *testing.T) {
	def := makeValidDefinition()
	if err := ValidateDefinition(def); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateDefinitionMissingID(t *testing.T) {
	def := makeValidDefinition()
	def.ContributionID = ""
	if err := ValidateDefinition(def); !errors.Is(err, ErrContributionIDEmpty) {
		t.Fatalf("expected ErrContributionIDEmpty, got %v", err)
	}
}

func TestValidateDefinitionInvalidKind(t *testing.T) {
	def := makeValidDefinition()
	def.Kind = "invalid_kind"
	if err := ValidateDefinition(def); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("expected ErrInvalidKind, got %v", err)
	}
}

func TestValidateDefinitionInvalidSandbox(t *testing.T) {
	def := makeValidDefinition()
	def.Sandbox.Type = "invalid_sandbox"
	if err := ValidateDefinition(def); !errors.Is(err, ErrInvalidSandbox) {
		t.Fatalf("expected ErrInvalidSandbox, got %v", err)
	}
}

func TestValidateDefinitionMissingEntryPath(t *testing.T) {
	def := makeValidDefinition()
	def.Entry.Path = ""
	if err := ValidateDefinition(def); !errors.Is(err, ErrEntryPathEmpty) {
		t.Fatalf("expected ErrEntryPathEmpty, got %v", err)
	}
}

func TestValidateDefinitionMissingIntegrityHash(t *testing.T) {
	def := makeValidDefinition()
	def.Integrity.DefinitionHash = ""
	if err := ValidateDefinition(def); !errors.Is(err, ErrIntegrityHashEmpty) {
		t.Fatalf("expected ErrIntegrityHashEmpty, got %v", err)
	}
}

func TestValidateDefinitionInvalidRiskLevel(t *testing.T) {
	def := makeValidDefinition()
	def.Actions = []UIActionDefinition{{
		ActionID:  "act-1",
		Title:     LocalizedText{Default: "Click"},
		Target:    UIActionTarget{Type: ActionTargetHostCommand, Command: "save"},
		RiskLevel: "invalid",
	}}
	if err := ValidateDefinition(def); !errors.Is(err, ErrInvalidRiskLevel) {
		t.Fatalf("expected ErrInvalidRiskLevel, got %v", err)
	}
}

func TestValidateAgainstSlotSuccess(t *testing.T) {
	def := makeValidDefinition()
	slot := DefaultSlots["extension.settings.page"]
	if err := ValidateAgainstSlot(def, slot); err != nil {
		t.Fatalf("validate against slot: %v", err)
	}
}

func TestValidateAgainstSlotKindNotSupported(t *testing.T) {
	def := makeValidDefinition()
	def.Kind = UIContributionMenuItem
	slot := DefaultSlots["extension.settings.page"]
	err := ValidateAgainstSlot(def, slot)
	if !errors.Is(err, ErrKindNotSupportedBySlot) {
		t.Fatalf("expected ErrKindNotSupportedBySlot, got %v", err)
	}
}

func TestValidateAgainstSlotSandboxNotAllowed(t *testing.T) {
	def := makeValidDefinition()
	def.Sandbox.Type = SandboxHostNative
	def.Entry.Type = SandboxHostNative
	slot := DefaultSlots["extension.settings.page"]
	err := ValidateAgainstSlot(def, slot)
	if !errors.Is(err, ErrSandboxNotAllowedBySlot) {
		t.Fatalf("expected ErrSandboxNotAllowedBySlot, got %v", err)
	}
}

func TestKindDefaultSandbox(t *testing.T) {
	cases := []struct {
		kind     UIContributionKind
		expected UISandboxType
	}{
		{UIContributionSchemaPage, SandboxSchemaRenderer},
		{UIContributionWebPage, SandboxWebRestricted},
		{UIContributionAction, SandboxHostNative},
		{UIContributionMenuItem, SandboxHostNative},
		{UIContributionStatusItem, SandboxHostNative},
	}
	for _, c := range cases {
		if got := c.kind.DefaultSandbox(); got != c.expected {
			t.Fatalf("for %s, expected %s, got %s", c.kind, c.expected, got)
		}
	}
}

func TestLocalizedTextResolve(t *testing.T) {
	lt := LocalizedText{
		Default: "Hello",
		I18n:    map[string]string{"zh-CN": "你好", "ja-JP": "こんにちは"},
	}
	if lt.Resolve("zh-CN") != "你好" {
		t.Fatalf("expected 你好, got %s", lt.Resolve("zh-CN"))
	}
	if lt.Resolve("en-US") != "Hello" {
		t.Fatalf("expected fallback Hello, got %s", lt.Resolve("en-US"))
	}
}

func TestUIHostRegisterSlot(t *testing.T) {
	h := NewUIHost()
	slot := &UISlotContract{
		SlotID:           "custom.slot",
		Version:          1,
		SupportedKinds:   []UIContributionKind{UIContributionCard},
		AllowedSandboxes: []UISandboxType{SandboxSchemaRenderer},
		Multiplicity:     MultiplicityMultiple,
	}
	if err := h.RegisterSlot(slot); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := h.RegisterSlot(slot); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestUIHostRegisterContribution(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	if err := h.RegisterContribution(def); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := h.RegisterContribution(def); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestUIHostRegisterContributionSlotNotRegistered(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	def.Slot.SlotID = "missing.slot"
	if err := h.RegisterContribution(def); err == nil {
		t.Fatalf("expected slot unsupported error")
	}
}

func TestUIHostListBySlot(t *testing.T) {
	h := NewUIHost()
	def1 := makeValidDefinition()
	def1.ContributionID = "contrib-1"
	def2 := makeValidDefinition()
	def2.ContributionID = "contrib-2"
	def2.Kind = UIContributionSettingsSection
	_ = h.RegisterContribution(def1)
	_ = h.RegisterContribution(def2)
	list := h.ListBySlot("extension.settings.page")
	if len(list) != 2 {
		t.Fatalf("expected 2 contributions, got %d", len(list))
	}
}

func TestUIHostSingleMultiplicityReject(t *testing.T) {
	h := NewUIHost()
	slot := &UISlotContract{
		SlotID:           "exclusive.slot",
		Version:          1,
		SupportedKinds:   []UIContributionKind{UIContributionPanel},
		AllowedSandboxes: []UISandboxType{SandboxSchemaRenderer},
		Multiplicity:     MultiplicitySingle,
	}
	_ = h.RegisterSlot(slot)
	def1 := makeValidDefinition()
	def1.ContributionID = "contrib-a"
	def1.Slot.SlotID = "exclusive.slot"
	def1.Kind = UIContributionPanel
	if err := h.RegisterContribution(def1); err != nil {
		t.Fatalf("register 1: %v", err)
	}
	def2 := makeValidDefinition()
	def2.ContributionID = "contrib-b"
	def2.Slot.SlotID = "exclusive.slot"
	def2.Kind = UIContributionPanel
	if err := h.RegisterContribution(def2); err == nil {
		t.Fatalf("expected single multiplicity conflict")
	}
}

func TestUIHostMountUnmount(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	if err := h.Mount(def.ContributionID); err != nil {
		t.Fatalf("mount: %v", err)
	}
	inst, _ := h.GetInstance(def.ContributionID)
	if !inst.State.IsActive() {
		t.Fatalf("expected active state, got %s", inst.State)
	}
	if err := h.Unmount(def.ContributionID); err != nil {
		t.Fatalf("unmount: %v", err)
	}
	inst, _ = h.GetInstance(def.ContributionID)
	if !inst.State.IsTerminal() {
		t.Fatalf("expected terminal state, got %s", inst.State)
	}
}

func TestUIHostDisableExtension(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	_ = h.Mount(def.ContributionID)
	h.DisableExtension(def.ExtensionID)
	inst, _ := h.GetInstance(def.ContributionID)
	if !inst.State.IsTerminal() {
		t.Fatalf("expected terminal after disable, got %s", inst.State)
	}
}

func TestBridgeSession(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, err := h.Bridge().CreateSession(def, "amitia://ext-1", []string{"read"}, []string{"tool.invoke"}, "web", "", "", time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.SessionID == "" {
		t.Fatalf("session id empty")
	}
	if sess.ContributionID != "contrib-1" {
		t.Fatalf("contribution mismatch")
	}
}

func TestBridgeValidateSession(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	if _, err := h.Bridge().ValidateSession(sess.SessionID, "contrib-1", "amitia://ext-1", 1, sess.Token, sess.Generation, "nonce-1"); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, err := h.Bridge().ValidateSession(sess.SessionID, "wrong", "amitia://ext-1", 1, sess.Token, sess.Generation, "nonce-2"); err == nil {
		t.Fatalf("expected contribution mismatch")
	}
	if _, err := h.Bridge().ValidateSession(sess.SessionID, "contrib-1", "wrong-origin", 1, sess.Token, sess.Generation, "nonce-3"); err == nil {
		t.Fatalf("expected origin mismatch")
	}
	if _, err := h.Bridge().ValidateSession(sess.SessionID, "contrib-1", "amitia://ext-1", 2, sess.Token, sess.Generation, "nonce-4"); err == nil {
		t.Fatalf("expected version mismatch")
	}
}

func TestBridgeValidateSessionExpired(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	h.Bridge().mu.Lock()
	sess.ExpiresAt = time.Now().UTC().Add(-time.Hour)
	h.Bridge().mu.Unlock()
	_, err := h.Bridge().ValidateSession(sess.SessionID, "contrib-1", "amitia://ext-1", 1, sess.Token, sess.Generation, "nonce-expired")
	if err == nil {
		t.Fatalf("expected expired")
	}
}

func TestBridgeHandleReady(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          BridgeUIReady,
		SessionID:       sess.SessionID,
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           "nonce-ready",
	})
	if !resp.OK {
		t.Fatalf("ready failed: %v", resp.Error)
	}
}

func TestBridgeHandleLog(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	payload, _ := json.Marshal(map[string]any{"level": "info", "message": "hello"})
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          BridgeUILog,
		SessionID:       sess.SessionID,
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           "nonce-log",
		Payload:         payload,
	})
	if !resp.OK {
		t.Fatalf("log failed: %v", resp.Error)
	}
}

func TestBridgeHandleActionInvokeUnknown(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	payload, _ := json.Marshal(map[string]any{"action_id": "missing"})
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          BridgeUIActionInvoke,
		SessionID:       sess.SessionID,
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           "nonce-action-unknown",
		Payload:         payload,
	})
	if resp.OK {
		t.Fatalf("expected action not declared error")
	}
	if resp.Error.Code != UIErrActionNotDeclared {
		t.Fatalf("expected action_not_declared, got %s", resp.Error.Code)
	}
}

func TestBridgeHandleActionInvokeDeclared(t *testing.T) {
	h := NewUIHost()
	h.Bridge().SetHandlers(func(context.Context, *BridgeSession, *UIActionDefinition, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"saved":true}`), nil
	}, nil)
	def := makeValidDefinition()
	def.Actions = []UIActionDefinition{{
		ActionID:  "save",
		Title:     LocalizedText{Default: "Save"},
		Target:    UIActionTarget{Type: ActionTargetHostCommand, Command: "save"},
		RiskLevel: RiskLevelLow,
	}}
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	payload, _ := json.Marshal(map[string]any{"action_id": "save"})
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          BridgeUIActionInvoke,
		SessionID:       sess.SessionID,
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           "nonce-action-declared",
		Payload:         payload,
	})
	if !resp.OK {
		t.Fatalf("action invoke failed: %v", resp.Error)
	}
}

func TestBridgeHandleDataRequestUsesHandler(t *testing.T) {
	h := NewUIHost()
	h.Bridge().SetHandlers(nil, func(_ context.Context, _ *BridgeSession, sourceID string, _ json.RawMessage) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"source": sourceID, "value": "real"})
	})
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", []string{"profile"}, nil, "web", "", "", time.Hour)
	payload, _ := json.Marshal(map[string]any{"key": "profile"})
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          BridgeUIDataRequest,
		SessionID:       sess.SessionID,
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           "nonce-data-request",
		Payload:         payload,
	})
	if !resp.OK || !strings.Contains(string(resp.Result), `"value":"real"`) {
		t.Fatalf("data request did not use handler: %+v", resp)
	}
}

func TestBridgeHandleInvalidMethod(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          "unknown.method",
		SessionID:       sess.SessionID,
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           "nonce-invalid-method",
	})
	if resp.OK {
		t.Fatalf("expected invalid method")
	}
}

func TestBridgeHandleUnauthenticated(t *testing.T) {
	h := NewUIHost()
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          BridgeUIReady,
		SessionID:       "missing",
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
	})
	if resp.OK {
		t.Fatalf("expected auth error")
	}
	if resp.Error.Code != UIErrBridgeAuth {
		t.Fatalf("expected bridge_auth_failed, got %s", resp.Error.Code)
	}
}

func TestBridgeHandleNavigation(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	payload, _ := json.Marshal(map[string]any{"route_id": "extension.detail"})
	resp := h.Bridge().Handle(context.Background(), BridgeMessage{
		Method:          BridgeUINavigationRequest,
		SessionID:       sess.SessionID,
		ContributionID:  "contrib-1",
		Origin:          "amitia://ext-1",
		ContractVersion: 1,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           "nonce-navigation",
		Payload:         payload,
	})
	if !resp.OK {
		t.Fatalf("navigation failed: %v", resp.Error)
	}
}

func TestBridgeRevokeSession(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	sess, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)
	if h.Bridge().SessionCount() != 1 {
		t.Fatalf("expected 1 session, got %d", h.Bridge().SessionCount())
	}
	h.Bridge().RevokeSession(sess.SessionID)
	if h.Bridge().SessionCount() != 0 {
		t.Fatalf("expected 0 sessions, got %d", h.Bridge().SessionCount())
	}
}

func TestUILifecycleStateValid(t *testing.T) {
	valid := []UILifecycleState{
		UIStateRegistered, UIStateLoading, UIStateMounted,
		UIStateVisible, UIStateHidden, UIStateSuspended,
		UIStateFailed, UIStateUnmounted,
	}
	for _, s := range valid {
		if !s.Valid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}
	if !UIStateUnmounted.IsTerminal() {
		t.Fatalf("unmounted should be terminal")
	}
	if !UIStateVisible.IsActive() {
		t.Fatalf("visible should be active")
	}
	if UIStateHidden.IsActive() {
		t.Fatalf("hidden should not be active")
	}
}

func TestUIInstanceFailure(t *testing.T) {
	inst := &UIInstance{State: UIStateVisible}
	c := inst.RecordFailure("boom")
	if c != 1 {
		t.Fatalf("expected 1 failure, got %d", c)
	}
	if inst.State != UIStateFailed {
		t.Fatalf("expected failed state")
	}
}

func TestDefaultSlotsExist(t *testing.T) {
	expected := []string{
		"extension.settings.page", "chat.sidebar.panel",
		"chat.message.action", "chat.message.renderer",
		"chat.composer.action", "desktop.tray.item",
		"system.status.item", "extension.detail.tab",
	}
	for _, id := range expected {
		if _, ok := DefaultSlots[id]; !ok {
			t.Fatalf("missing default slot: %s", id)
		}
	}
}

func TestUIHostUnregister(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)
	if err := h.UnregisterContribution(def.ContributionID); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	if _, err := h.GetContribution(def.ContributionID); err == nil {
		t.Fatalf("expected not found after unregister")
	}
}

func TestRevokeSessionsByCharacterID(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)

	sess1, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-1", "conv-1", time.Hour)
	sess2, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-1", "conv-2", time.Hour)
	sess3, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-2", "conv-3", time.Hour)

	if h.Bridge().SessionCount() != 3 {
		t.Fatalf("expected 3 sessions, got %d", h.Bridge().SessionCount())
	}

	count := h.Bridge().RevokeSessionsByContext("char-1", "")
	if count != 2 {
		t.Fatalf("expected 2 sessions revoked for char-1, got %d", count)
	}
	if h.Bridge().SessionCount() != 1 {
		t.Fatalf("expected 1 remaining session, got %d", h.Bridge().SessionCount())
	}

	_, err := h.Bridge().ValidateSession(sess1.SessionID, "contrib-1", "amitia://ext-1", 1, sess1.Token, sess1.Generation, "nonce-rev-1")
	if err == nil {
		t.Fatal("expected sess1 to be revoked")
	}
	_, err = h.Bridge().ValidateSession(sess2.SessionID, "contrib-1", "amitia://ext-1", 1, sess2.Token, sess2.Generation, "nonce-rev-2")
	if err == nil {
		t.Fatal("expected sess2 to be revoked")
	}
	_, err = h.Bridge().ValidateSession(sess3.SessionID, "contrib-1", "amitia://ext-1", 1, sess3.Token, sess3.Generation, "nonce-rev-3")
	if err != nil {
		t.Fatalf("expected sess3 to still be valid: %v", err)
	}
}

func TestRevokeSessionsByConversationID(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)

	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-1", "conv-1", time.Hour)
	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-2", "conv-1", time.Hour)
	sess3, _ := h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-3", "conv-2", time.Hour)

	if h.Bridge().SessionCount() != 3 {
		t.Fatalf("expected 3 sessions, got %d", h.Bridge().SessionCount())
	}

	count := h.Bridge().RevokeSessionsByContext("", "conv-1")
	if count != 2 {
		t.Fatalf("expected 2 sessions revoked for conv-1, got %d", count)
	}
	if h.Bridge().SessionCount() != 1 {
		t.Fatalf("expected 1 remaining session, got %d", h.Bridge().SessionCount())
	}

	_, err := h.Bridge().ValidateSession(sess3.SessionID, "contrib-1", "amitia://ext-1", 1, sess3.Token, sess3.Generation, "nonce-rev-conv-3")
	if err != nil {
		t.Fatalf("expected sess3 to still be valid: %v", err)
	}
}

func TestRevokeSessionsByBothContext(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)

	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-1", "conv-1", time.Hour)
	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-1", "conv-2", time.Hour)
	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-2", "conv-1", time.Hour)
	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-2", "conv-2", time.Hour)

	if h.Bridge().SessionCount() != 4 {
		t.Fatalf("expected 4 sessions, got %d", h.Bridge().SessionCount())
	}

	count := h.Bridge().RevokeSessionsByContext("char-1", "conv-1")
	if count != 3 {
		t.Fatalf("expected 3 sessions revoked (char-1 OR conv-1), got %d", count)
	}
	if h.Bridge().SessionCount() != 1 {
		t.Fatalf("expected 1 remaining session, got %d", h.Bridge().SessionCount())
	}
}

func TestRevokeSessionsNoMatch(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)

	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-1", "conv-1", time.Hour)
	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-2", "conv-2", time.Hour)

	count := h.Bridge().RevokeSessionsByContext("char-999", "conv-999")
	if count != 0 {
		t.Fatalf("expected 0 sessions revoked, got %d", count)
	}
	if h.Bridge().SessionCount() != 2 {
		t.Fatalf("expected 2 remaining sessions, got %d", h.Bridge().SessionCount())
	}
}

func TestRevokeSessionsEmptyContext(t *testing.T) {
	h := NewUIHost()
	def := makeValidDefinition()
	_ = h.RegisterContribution(def)

	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "char-1", "conv-1", time.Hour)
	_, _ = h.Bridge().CreateSession(def, "amitia://ext-1", nil, nil, "web", "", "", time.Hour)

	count := h.Bridge().RevokeSessionsByContext("", "")
	if count != 0 {
		t.Fatalf("expected 0 sessions revoked with empty context, got %d", count)
	}
	if h.Bridge().SessionCount() != 2 {
		t.Fatalf("expected 2 remaining sessions, got %d", h.Bridge().SessionCount())
	}
}
