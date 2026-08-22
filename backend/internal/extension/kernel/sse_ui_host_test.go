package kernel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/pkg/sse"
)

func TestDefaultUIHostNotifier_FailClosed(t *testing.T) {
	n := NewDefaultUIHostNotifier()

	if err := n.Notify(context.Background(), "ext-1", "title", "body", "info"); !errors.Is(err, ErrUIHostUnavailable) {
		t.Errorf("Notify() error = %v, want ErrUIHostUnavailable", err)
	}

	if _, err := n.Dialog(context.Background(), "ext-1", "dlg-1", "msg", []string{"OK"}); !errors.Is(err, ErrDialogHostUnavailable) {
		t.Errorf("Dialog() error = %v, want ErrDialogHostUnavailable", err)
	}

	if err := n.Navigate(context.Background(), "ext-1", "/target"); !errors.Is(err, ErrNavigationHostUnavailable) {
		t.Errorf("Navigate() error = %v, want ErrNavigationHostUnavailable", err)
	}
}

func TestSSEUIHostNotifier_HostUnavailableWithoutClients(t *testing.T) {
	hub := &sse.Hub{}
	n := NewSSEUIHostNotifier(hub)

	if err := n.Notify(context.Background(), "ext-1", "title", "body", "info"); !errors.Is(err, ErrUIHostUnavailable) {
		t.Errorf("Notify() error = %v, want ErrUIHostUnavailable", err)
	}

	if _, err := n.Dialog(context.Background(), "ext-1", "dlg-1", "msg", []string{"OK"}); !errors.Is(err, ErrDialogHostUnavailable) {
		t.Errorf("Dialog() error = %v, want ErrDialogHostUnavailable", err)
	}

	if err := n.Navigate(context.Background(), "ext-1", "/target"); !errors.Is(err, ErrNavigationHostUnavailable) {
		t.Errorf("Navigate() error = %v, want ErrNavigationHostUnavailable", err)
	}
}

func TestSSEUIHostNotifier_NilHub(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)

	if err := n.Notify(context.Background(), "ext-1", "title", "body", "info"); !errors.Is(err, ErrUIHostUnavailable) {
		t.Errorf("Notify() error = %v, want ErrUIHostUnavailable", err)
	}

	if _, err := n.Dialog(context.Background(), "ext-1", "dlg-1", "msg", []string{"OK"}); !errors.Is(err, ErrDialogHostUnavailable) {
		t.Errorf("Dialog() error = %v, want ErrDialogHostUnavailable", err)
	}

	if err := n.Navigate(context.Background(), "ext-1", "/target"); !errors.Is(err, ErrNavigationHostUnavailable) {
		t.Errorf("Navigate() error = %v, want ErrNavigationHostUnavailable", err)
	}
}

func TestSSEUIHostNotifier_NotifyWithClients(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-notify-" + t.Name())
	defer hub.Unsubscribe("test-notify-" + t.Name())

	n := NewSSEUIHostNotifier(hub)
	if err := n.Notify(context.Background(), "ext-1", "Test Title", "Test Body", "info"); err != nil {
		t.Fatalf("Notify() error = %v, want nil", err)
	}

	select {
	case msg := <-client.Events:
		event, ok := msg["event"].(string)
		if !ok || event != "ui_notify" {
			t.Errorf("expected event ui_notify, got %v", msg["event"])
		}
		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data to be map[string]interface{}")
		}
		if data["eventType"] != "ui_notify" {
			t.Errorf("expected eventType 'ui_notify', got %v", data["eventType"])
		}
		if data["extensionId"] != "ext-1" {
			t.Errorf("expected extensionId 'ext-1', got %v", data["extensionId"])
		}
		if data["requestId"] == nil || data["requestId"] == "" {
			t.Error("expected requestId to be non-empty")
		}
		if data["sessionId"] != "ui-host" {
			t.Errorf("expected sessionId 'ui-host', got %v", data["sessionId"])
		}
		if data["expiresAt"] == nil || data["expiresAt"] == "" {
			t.Error("expected expiresAt to be non-empty")
		}
		if data["timestamp"] == nil || data["timestamp"] == "" {
			t.Error("expected timestamp to be non-empty")
		}
		payload, ok := data["payload"].(map[string]interface{})
		if !ok {
			t.Fatal("expected payload to be map[string]interface{}")
		}
		if payload["title"] != "Test Title" {
			t.Errorf("expected payload title 'Test Title', got %v", payload["title"])
		}
		if payload["body"] != "Test Body" {
			t.Errorf("expected payload body 'Test Body', got %v", payload["body"])
		}
		if payload["severity"] != "info" {
			t.Errorf("expected payload severity 'info', got %v", payload["severity"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestSSEUIHostNotifier_NavigateWithClients(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-navigate-" + t.Name())
	defer hub.Unsubscribe("test-navigate-" + t.Name())

	n := NewSSEUIHostNotifier(hub)
	if err := n.Navigate(context.Background(), "ext-1", "/chat"); err != nil {
		t.Fatalf("Navigate() error = %v, want nil", err)
	}

	select {
	case msg := <-client.Events:
		event, ok := msg["event"].(string)
		if !ok || event != "ui_navigate" {
			t.Errorf("expected event ui_navigate, got %v", msg["event"])
		}
		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data to be map[string]interface{}")
		}
		if data["eventType"] != "ui_navigate" {
			t.Errorf("expected eventType 'ui_navigate', got %v", data["eventType"])
		}
		if data["requestId"] == nil || data["requestId"] == "" {
			t.Error("expected requestId to be non-empty")
		}
		if data["expiresAt"] == nil || data["expiresAt"] == "" {
			t.Error("expected expiresAt to be non-empty")
		}
		payload, ok := data["payload"].(map[string]interface{})
		if !ok {
			t.Fatal("expected payload to be map[string]interface{}")
		}
		if payload["target"] != "/chat" {
			t.Errorf("expected payload target '/chat', got %v", payload["target"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestSSEUIHostNotifier_DialogWithResolution(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-dialog-" + t.Name())
	defer hub.Unsubscribe("test-dialog-" + t.Name())

	n := NewSSEUIHostNotifier(hub)

	dialogID := "test-dlg-resolve"
	resultCh := make(chan struct {
		result string
		err    error
	}, 1)

	go func() {
		result, err := n.Dialog(context.Background(), "ext-1", dialogID, "Choose", []string{"Yes", "No"})
		resultCh <- struct {
			result string
			err    error
		}{result, err}
	}()

	select {
	case msg := <-client.Events:
		event, ok := msg["event"].(string)
		if !ok || event != "ui_dialog" {
			t.Errorf("expected event ui_dialog, got %v", msg["event"])
		}
		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			t.Fatal("expected data to be map[string]interface{}")
		}
		if data["eventType"] != "ui_dialog" {
			t.Errorf("expected eventType 'ui_dialog', got %v", data["eventType"])
		}
		if data["requestId"] == nil || data["requestId"] == "" {
			t.Error("expected requestId to be non-empty")
		}
		if data["expiresAt"] == nil || data["expiresAt"] == "" {
			t.Error("expected expiresAt to be non-empty")
		}
		payload, ok := data["payload"].(map[string]interface{})
		if !ok {
			t.Fatal("expected payload to be map[string]interface{}")
		}
		if payload["dialogId"] != dialogID {
			t.Errorf("expected payload dialogId '%s', got %v", dialogID, payload["dialogId"])
		}
		if payload["message"] != "Choose" {
			t.Errorf("expected payload message 'Choose', got %v", payload["message"])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE event")
	}

	if !n.HasPendingDialog(dialogID) {
		t.Fatal("expected pending dialog to exist")
	}

	if !n.ResolveDialog(dialogID, "Yes") {
		t.Fatal("ResolveDialog returned false, expected true")
	}

	select {
	case r := <-resultCh:
		if r.err != nil {
			t.Fatalf("Dialog() error = %v, want nil", r.err)
		}
		if r.result != "Yes" {
			t.Errorf("Dialog() result = %v, want 'Yes'", r.result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dialog result")
	}

	if n.HasPendingDialog(dialogID) {
		t.Fatal("expected pending dialog to be removed after resolution")
	}
}

func TestSSEUIHostNotifier_DialogTimeout(t *testing.T) {
	hub := sse.Global
	client := hub.Subscribe("test-dialog-timeout-" + t.Name())
	defer hub.Unsubscribe("test-dialog-timeout-" + t.Name())

	n := NewSSEUIHostNotifier(hub)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := n.Dialog(ctx, "ext-1", "test-dlg-timeout", "msg", []string{"OK"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	go func() {
		<-client.Events
	}()
}

func TestSSEUIHostNotifier_ResolveDialogNotFound(t *testing.T) {
	hub := &sse.Hub{}
	n := NewSSEUIHostNotifier(hub)

	if n.ResolveDialog("nonexistent", "ok") {
		t.Fatal("ResolveDialog returned true for non-existent dialog, expected false")
	}

	if n.FailDialog("nonexistent", errors.New("test")) {
		t.Fatal("FailDialog returned true for non-existent dialog, expected false")
	}
}

func TestSSEEventEnvelope_Fields(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	envelope := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)

	if envelope.EventType != "ui_notify" {
		t.Errorf("expected EventType 'ui_notify', got %v", envelope.EventType)
	}
	if envelope.ExtensionID != "ext-1" {
		t.Errorf("expected ExtensionID 'ext-1', got %v", envelope.ExtensionID)
	}
	if envelope.SessionID != "ui-host" {
		t.Errorf("expected SessionID 'ui-host', got %v", envelope.SessionID)
	}
	if envelope.RequestID == "" {
		t.Error("expected RequestID to be non-empty")
	}
	if envelope.Timestamp == "" {
		t.Error("expected Timestamp to be non-empty")
	}
	if envelope.ExpiresAt == "" {
		t.Error("expected ExpiresAt to be non-empty")
	}

	expiresAt, err := time.Parse(time.RFC3339, envelope.ExpiresAt)
	if err != nil {
		t.Fatalf("failed to parse ExpiresAt: %v", err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Error("expected ExpiresAt to be in the future")
	}
}

func TestSSEEventEnvelope_UniqueRequestIDs(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	e1 := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)
	e2 := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)
	if e1.RequestID == e2.RequestID {
		t.Error("expected RequestIDs to be unique")
	}
}

func TestSSEEventEnvelope_ToMap(t *testing.T) {
	payload := map[string]interface{}{"title": "test"}
	envelope := NewEventEnvelope("ui_notify", "ext-1", payload, 5*time.Minute)
	m := envelope.ToMap()

	if m["eventType"] != "ui_notify" {
		t.Errorf("expected eventType 'ui_notify', got %v", m["eventType"])
	}
	if m["extensionId"] != "ext-1" {
		t.Errorf("expected extensionId 'ext-1', got %v", m["extensionId"])
	}
	if m["requestId"] == "" {
		t.Error("expected requestId to be non-empty")
	}
	if m["payload"] == nil {
		t.Error("expected payload to be non-nil")
	}
	if m["expiresAt"] == "" {
		t.Error("expected expiresAt to be non-empty")
	}
	if m["timestamp"] == "" {
		t.Error("expected timestamp to be non-empty")
	}
}

func TestSSEUIHostNotifier_ClientRuntimeSessionRevisionIsAuthoritative(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	scope := map[string]interface{}{
		"userId":         "user-1",
		"conversationId": "conversation-1",
		"requestId":      "request-1",
	}
	definePayload := map[string]interface{}{
		"_runtimeScope": scope,
		"package": map[string]interface{}{
			"id":            "demo",
			"version":       "1.0.0",
			"contributions": []interface{}{},
		},
	}
	defined, err := n.ExecuteClientRuntimeCommand(context.Background(), "define", definePayload)
	if err != nil {
		t.Fatalf("define failed: %v", err)
	}
	definedState, ok := defined["serverState"].(map[string]interface{})
	if !ok {
		t.Fatalf("define serverState type = %T", defined["serverState"])
	}
	if got := definedState["revision"]; got != int64(1) {
		t.Fatalf("define revision = %v, want 1", got)
	}
	if got := defined["delivery"]; got != "deferred" {
		t.Fatalf("define delivery = %v, want deferred", got)
	}

	runPayload := map[string]interface{}{"_runtimeScope": scope, "id": "demo", "version": "1.0.0"}
	run, err := n.ExecuteClientRuntimeCommand(context.Background(), "run", runPayload)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	runState := run["serverState"].(map[string]interface{})
	if got := runState["revision"]; got != int64(3) {
		t.Fatalf("run revision = %v, want 3 (approval + activation)", got)
	}

	inspect, err := n.ExecuteClientRuntimeCommand(context.Background(), "inspect", map[string]interface{}{"_runtimeScope": scope})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	inspectState := inspect["serverState"].(map[string]interface{})
	if got := inspectState["revision"]; got != int64(3) {
		t.Fatalf("inspect revision = %v, want 3", got)
	}

	stop, err := n.ExecuteClientRuntimeCommand(context.Background(), "stop", map[string]interface{}{"_runtimeScope": scope, "id": "demo"})
	if err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	stopState := stop["serverState"].(map[string]interface{})
	if got := stopState["revision"]; got != int64(4) {
		t.Fatalf("stop revision = %v, want 4", got)
	}
}

func TestSSEUIHostNotifier_ClientRuntimeResponseIsBoundAndDeduplicated(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	pending := &pendingClientRuntimeCommand{
		responseCh: make(chan clientRuntimeHostResponse, 2),
		allowedHosts: map[string]string{
			"host-a": "session-a",
			"host-b": "session-b",
		},
		responded: make(map[string]struct{}),
	}
	n.pendingClientRuntimeCommands["cmd-1"] = pending

	if n.ResolveClientRuntimeCommandWithHost("cmd-1", "host-a", "wrong", nil, "") {
		t.Fatal("response with wrong host session must be rejected")
	}
	if !n.ResolveClientRuntimeCommandWithHost("cmd-1", "host-a", "session-a", map[string]interface{}{"ok": true}, "") {
		t.Fatal("first valid host response should be accepted")
	}
	if n.ResolveClientRuntimeCommandWithHost("cmd-1", "host-a", "session-a", map[string]interface{}{"ok": true}, "") {
		t.Fatal("duplicate host response must be rejected")
	}
	if !n.ResolveClientRuntimeCommandWithHost("cmd-1", "host-b", "session-b", nil, "boom") {
		t.Fatal("second host error response should still be accepted as an acknowledgement")
	}

	first := <-pending.responseCh
	second := <-pending.responseCh
	if first.HostClientID == second.HostClientID {
		t.Fatalf("expected two distinct host acknowledgements, got %q twice", first.HostClientID)
	}
}

func TestSSEUIHostNotifier_ClientRuntimeDeferredResponseDoesNotFailRevisionGate(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	pending := &pendingClientRuntimeCommand{
		responseCh:     make(chan clientRuntimeHostResponse, 1),
		allowedHosts:   map[string]string{"host-a": "session-a"},
		responded:      make(map[string]struct{}),
		targetRevision: 7,
	}
	n.pendingClientRuntimeCommands["cmd-deferred"] = pending

	if !n.ResolveClientRuntimeCommandWithHost("cmd-deferred", "host-a", "session-a", map[string]interface{}{
		"ok":    true,
		"state": "deferred",
	}, "") {
		t.Fatal("deferred host acknowledgement should be accepted without applying the active revision gate")
	}
	response := <-pending.responseCh
	if response.Error != "" {
		t.Fatalf("deferred acknowledgement error = %q, want empty", response.Error)
	}
}

func TestSSEUIHostNotifier_ClientRuntimeActiveResponseEnforcesRevisionGate(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	pending := &pendingClientRuntimeCommand{
		responseCh:     make(chan clientRuntimeHostResponse, 1),
		allowedHosts:   map[string]string{"host-a": "session-a"},
		responded:      make(map[string]struct{}),
		targetRevision: 7,
	}
	n.pendingClientRuntimeCommands["cmd-active"] = pending

	if !n.ResolveClientRuntimeCommandWithHost("cmd-active", "host-a", "session-a", map[string]interface{}{
		"ok":       true,
		"state":    "reconciled",
		"revision": int64(6),
	}, "") {
		t.Fatal("active host acknowledgement should be accepted into the response channel")
	}
	response := <-pending.responseCh
	if !strings.Contains(response.Error, "behind target revision 7") {
		t.Fatalf("active stale acknowledgement error = %q, want revision gate failure", response.Error)
	}
}

func TestSSEUIHostNotifier_ClientRuntimeRollbackRestoresCommittedStateWithNewRevision(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	userID, conversationID := "user-rollback", "conversation-rollback"
	definePayload := map[string]interface{}{
		"package": map[string]interface{}{"id": "demo", "version": "1", "contributions": []interface{}{}},
	}
	if err := n.recordClientRuntimeDefinition(userID, conversationID, definePayload); err != nil {
		t.Fatalf("define: %v", err)
	}
	committed := n.cloneClientRuntimeSessionForScope(userID, conversationID)
	if _, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "1"}, nil); err != nil {
		t.Fatalf("run mutation: %v", err)
	}
	mutated := n.ClientRuntimeSessionState(userID, conversationID)
	if got := mutated["revision"]; got != int64(2) {
		t.Fatalf("mutated revision = %v, want 2", got)
	}

	restored, err := n.restoreClientRuntimeSession(userID, conversationID, committed)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := restored["revision"]; got != int64(3) {
		t.Fatalf("rollback revision = %v, want 3", got)
	}
	packages := restored["packages"].([]map[string]interface{})
	if len(packages) != 1 || packages[0]["running"] != false {
		t.Fatalf("rollback package state = %#v, want stopped committed state", packages)
	}
}

func TestValidateClientRuntimePackageDefinitionAcceptsContributionOwnedChildSlots(t *testing.T) {
	pkg := map[string]interface{}{
		"id":      "demo",
		"version": "1.0.0",
		"contributions": []interface{}{
			map[string]interface{}{
				"slotId":               "chat.sidebar.panel",
				"key":                  "demo-panel",
				"sourceExtensionId":    "demo.ext",
				"sourceContributionId": "demo.schema",
				"children": []interface{}{
					map[string]interface{}{
						"slotId":         "demo.panel.toolbar",
						"supportedKinds": []interface{}{"action"},
					},
				},
			},
		},
	}
	if err := validateClientRuntimePackageDefinition(pkg); err != nil {
		t.Fatalf("valid dynamic package rejected: %v", err)
	}
}

func TestValidateClientRuntimePackageDefinitionRejectsDuplicateChildSlotDeclaration(t *testing.T) {
	child := func() map[string]interface{} {
		return map[string]interface{}{"slotId": "demo.child", "supportedKinds": []interface{}{"panel"}}
	}
	pkg := map[string]interface{}{
		"id":      "demo",
		"version": "1.0.0",
		"contributions": []interface{}{
			map[string]interface{}{"slotId": "chat.sidebar.panel", "key": "a", "sourceExtensionId": "demo", "sourceContributionId": "a", "children": []interface{}{child()}},
			map[string]interface{}{"slotId": "chat.sidebar.panel", "key": "b", "sourceExtensionId": "demo", "sourceContributionId": "b", "children": []interface{}{child()}},
		},
	}
	if err := validateClientRuntimePackageDefinition(pkg); err == nil || !strings.Contains(err.Error(), "declared more than once") {
		t.Fatalf("duplicate child slot validation error = %v", err)
	}
}

func TestSSEUIHostNotifier_ClientRuntimeCurrentAdvancesOnlyAfterBrowserAck(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	userID, conversationID := "user-current-next", "conversation-current-next"
	definePayload := map[string]interface{}{
		"package": map[string]interface{}{"id": "demo", "version": "1", "contributions": []interface{}{}},
	}
	if err := n.recordClientRuntimeDefinition(userID, conversationID, definePayload); err != nil {
		t.Fatalf("define: %v", err)
	}
	result, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "1"}, nil)
	if err != nil {
		t.Fatalf("begin run: %v", err)
	}
	runID := runtimeMapString(result, "pluginRunId")
	if runID == "" {
		t.Fatal("run did not allocate pluginRunId")
	}
	state := n.ClientRuntimeSessionState(userID, conversationID)
	pkg := state["packages"].([]map[string]interface{})[0]
	if got := pkg["activeVersion"]; got != "" {
		t.Fatalf("activeVersion before activation = %v, want empty", got)
	}
	if got := pkg["targetVersion"]; got != "1" {
		t.Fatalf("targetVersion = %v, want 1", got)
	}
	if got := pkg["transitionState"]; got != "starting" {
		t.Fatalf("transitionState = %v, want starting", got)
	}

	awaiting, err := n.markClientRuntimeTransitionAwaiting(userID, conversationID, "demo", runID)
	if err != nil {
		t.Fatalf("mark awaiting: %v", err)
	}
	pkg = awaiting["packages"].([]map[string]interface{})[0]
	if got := pkg["activeVersion"]; got != "" {
		t.Fatalf("activeVersion while awaiting = %v, want empty", got)
	}
	if got := pkg["transitionState"]; got != "awaiting_client" {
		t.Fatalf("transitionState = %v, want awaiting_client", got)
	}

	revision := awaiting["revision"].(int64)
	committed, err := n.AcknowledgeClientRuntimeSession(userID, conversationID, revision)
	if err != nil {
		t.Fatalf("browser ack: %v", err)
	}
	pkg = committed["packages"].([]map[string]interface{})[0]
	if got := pkg["activeVersion"]; got != "1" {
		t.Fatalf("activeVersion after ack = %v, want 1", got)
	}
	if got := pkg["targetVersion"]; got != "" {
		t.Fatalf("targetVersion after ack = %v, want empty", got)
	}
	if got := pkg["transitionState"]; got != "" {
		t.Fatalf("transitionState after ack = %v, want empty", got)
	}
	if running, _ := pkg["running"].(bool); !running {
		t.Fatal("package should be running after browser ack")
	}
}

func TestSSEUIHostNotifier_ClientRuntimeFailedUpdateKeepsCurrentAndNextDiagnostics(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	userID, conversationID := "user-failed-next", "conversation-failed-next"
	for _, version := range []string{"1", "2"} {
		if err := n.recordClientRuntimeDefinition(userID, conversationID, map[string]interface{}{
			"package": map[string]interface{}{"id": "demo", "version": version, "contributions": []interface{}{}},
		}); err != nil {
			t.Fatalf("define %s: %v", version, err)
		}
	}
	first, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "1"}, nil)
	if err != nil {
		t.Fatalf("run v1: %v", err)
	}
	committed, err := n.commitClientRuntimeTransition(userID, conversationID, "demo", runtimeMapString(first, "pluginRunId"))
	if err != nil {
		t.Fatalf("commit v1: %v", err)
	}
	if got := committed["packages"].([]map[string]interface{})[0]["activeVersion"]; got != "1" {
		t.Fatalf("current after v1 = %v, want 1", got)
	}

	previous := n.cloneClientRuntimeSessionForScope(userID, conversationID)
	second, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "2", "mode": "update"}, nil)
	if err != nil {
		t.Fatalf("run v2: %v", err)
	}
	failed, err := n.failClientRuntimeTransition(userID, conversationID, "demo", runtimeMapString(second, "pluginRunId"), "synthetic activation failure", previous)
	if err != nil {
		t.Fatalf("fail v2: %v", err)
	}
	pkg := failed["packages"].([]map[string]interface{})[0]
	if got := pkg["activeVersion"]; got != "1" {
		t.Fatalf("current after failed v2 = %v, want 1", got)
	}
	if got := pkg["targetVersion"]; got != "2" {
		t.Fatalf("next after failed v2 = %v, want 2", got)
	}
	if got := pkg["transitionState"]; got != "failed" {
		t.Fatalf("transitionState = %v, want failed", got)
	}
	if got := pkg["lastError"]; got != "synthetic activation failure" {
		t.Fatalf("lastError = %v", got)
	}
}

func TestSSEUIHostNotifier_ClientRuntimeRunModeRestartsCurrentAndRequiresUpdateForVersionSwitch(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	userID, conversationID := "user-run-mode", "conversation-run-mode"
	for _, version := range []string{"1", "2"} {
		if err := n.recordClientRuntimeDefinition(userID, conversationID, map[string]interface{}{
			"package": map[string]interface{}{"id": "demo", "version": version, "contributions": []interface{}{}},
		}); err != nil {
			t.Fatalf("define %s: %v", version, err)
		}
	}
	first, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "1", "mode": "run"}, nil)
	if err != nil {
		t.Fatalf("initial run v1: %v", err)
	}
	if _, err := n.commitClientRuntimeTransition(userID, conversationID, "demo", runtimeMapString(first, "pluginRunId")); err != nil {
		t.Fatalf("commit v1: %v", err)
	}

	restart, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "1", "mode": "run"}, nil)
	if err != nil {
		t.Fatalf("restart current v1: %v", err)
	}
	state := n.ClientRuntimeSessionState(userID, conversationID)
	pkg := state["packages"].([]map[string]interface{})[0]
	if got := pkg["activeVersion"]; got != "1" {
		t.Fatalf("current during restart = %v, want 1", got)
	}
	if got := pkg["targetVersion"]; got != "1" {
		t.Fatalf("target during restart = %v, want 1", got)
	}
	if got := pkg["transitionMode"]; got != "run" {
		t.Fatalf("transition mode = %v, want run", got)
	}
	if runtimeMapString(restart, "pluginRunId") == "" {
		t.Fatal("restart did not allocate a new pluginRunId")
	}

	if _, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "2", "mode": "run"}, nil); err == nil || !strings.Contains(err.Error(), "use update mode") {
		t.Fatalf("cross-version run error = %v, want explicit update-mode rejection", err)
	}
	updated, err := n.applyClientRuntimeState(userID, conversationID, "run", map[string]interface{}{"id": "demo", "version": "2", "mode": "update"}, nil)
	if err != nil {
		t.Fatalf("update v2: %v", err)
	}
	if got := runtimeMapString(updated, "mode"); got != "update" {
		t.Fatalf("update mode = %q, want update", got)
	}
}

func TestSSEUIHostNotifier_ClientRuntimeFutureVersionApprovalCoversLaterImmutablePackages(t *testing.T) {
	n := NewSSEUIHostNotifier(nil)
	userID, conversationID := "user-future-approval", "conversation-future-approval"
	for _, version := range []string{"1", "2"} {
		if err := n.recordClientRuntimeDefinition(userID, conversationID, map[string]interface{}{
			"package": map[string]interface{}{"id": "demo", "version": version, "contributions": []interface{}{}},
		}); err != nil {
			t.Fatalf("define %s: %v", version, err)
		}
	}
	if err := n.approveClientRuntimeVersion(userID, conversationID, "demo", "1", true); err != nil {
		t.Fatalf("approve future versions: %v", err)
	}
	selected, approved, err := n.clientRuntimeSelectedVersion(userID, conversationID, "demo", "2")
	if err != nil {
		t.Fatalf("select v2: %v", err)
	}
	if selected != "2" || !approved {
		t.Fatalf("future-version approval = (%q, %v), want (2, true)", selected, approved)
	}
	state := n.ClientRuntimeSessionState(userID, conversationID)
	pkg := state["packages"].([]map[string]interface{})[0]
	if pkg["approveFutureVersions"] != true {
		t.Fatalf("approveFutureVersions = %v, want true", pkg["approveFutureVersions"])
	}
}

func TestValidateClientRuntimePackageDefinitionRejectsInvalidChildSlotScope(t *testing.T) {
	pkg := map[string]interface{}{
		"id":      "demo",
		"version": "1.0.0",
		"contributions": []interface{}{
			map[string]interface{}{
				"slotId":               "chat.sidebar.panel",
				"key":                  "demo-panel",
				"sourceExtensionId":    "demo.ext",
				"sourceContributionId": "demo.schema",
				"children": []interface{}{
					map[string]interface{}{
						"slotId":         "demo.panel.toolbar",
						"supportedKinds": []interface{}{"action"},
						"scope":          "device",
					},
				},
			},
		},
	}
	if err := validateClientRuntimePackageDefinition(pkg); err == nil || !strings.Contains(err.Error(), "invalid scope") {
		t.Fatalf("invalid child scope validation error = %v", err)
	}
}

func TestValidateClientRuntimePackageDefinitionAcceptsSandboxedClientCodeWithoutSchemaSource(t *testing.T) {
	pkg := map[string]interface{}{
		"id":      "sandbox-demo",
		"version": "1.0.0",
		"contributions": []interface{}{
			map[string]interface{}{
				"slotId": "chat.sidebar.panel",
				"key":    "sandbox-panel",
				"clientCode": map[string]interface{}{
					"html":      "<button id=demo>Demo</button>",
					"css":       "button{font:inherit}",
					"script":    "document.getElementById('demo')?.addEventListener('click',()=>{});",
					"minHeight": float64(80),
					"maxHeight": float64(320),
				},
			},
		},
	}
	if err := validateClientRuntimePackageDefinition(pkg); err != nil {
		t.Fatalf("sandboxed clientCode should be accepted: %v", err)
	}
}

func TestValidateClientRuntimePackageDefinitionRejectsUnboundedSandboxedClientCode(t *testing.T) {
	pkg := map[string]interface{}{
		"id":      "sandbox-too-large",
		"version": "1.0.0",
		"contributions": []interface{}{
			map[string]interface{}{
				"slotId": "chat.sidebar.panel",
				"key":    "sandbox-panel",
				"clientCode": map[string]interface{}{
					"script": strings.Repeat("x", maxClientRuntimeScriptBytes+1),
				},
			},
		},
	}
	if err := validateClientRuntimePackageDefinition(pkg); err == nil || !strings.Contains(err.Error(), "script exceeds") {
		t.Fatalf("oversized sandbox script validation error = %v", err)
	}
}

func TestValidateClientRuntimePackageDefinitionRejectsIncompleteSchemaSourceWithoutClientCode(t *testing.T) {
	pkg := map[string]interface{}{
		"id":      "bad-source",
		"version": "1.0.0",
		"contributions": []interface{}{
			map[string]interface{}{
				"slotId":            "chat.sidebar.panel",
				"key":               "bad-panel",
				"sourceExtensionId": "demo.ext",
			},
		},
	}
	if err := validateClientRuntimePackageDefinition(pkg); err == nil || !strings.Contains(err.Error(), "both sourceExtensionId and sourceContributionId") {
		t.Fatalf("incomplete schema source validation error = %v", err)
	}
}
