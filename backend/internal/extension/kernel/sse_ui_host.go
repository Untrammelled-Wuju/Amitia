package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/pkg/sse"
)

const (
	dialogResponseTimeout       = 25 * time.Second
	clientRuntimeCommandTimeout = 20 * time.Second
)

type pendingDialog struct {
	resultCh      chan string
	errCh         chan error
	hostClientID  string
	hostSessionID string
}

type clientRuntimeHostResponse struct {
	HostClientID  string                 `json:"hostClientId"`
	HostSessionID string                 `json:"hostSessionId"`
	Result        map[string]interface{} `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

type pendingClientRuntimeCommand struct {
	responseCh     chan clientRuntimeHostResponse
	allowedHosts   map[string]string // hostClientID -> hostSessionID
	responded      map[string]struct{}
	targetRevision int64
}

type clientRuntimePackageState struct {
	Versions              map[string]map[string]interface{} `json:"versions"`
	Order                 []string                          `json:"order"`
	ApprovedVersions      map[string]bool                   `json:"approvedVersions,omitempty"`
	ApproveFutureVersions bool                              `json:"approveFutureVersions,omitempty"`
	ActiveVersion         string                            `json:"activeVersion,omitempty"`
	Running               bool                              `json:"running"`
	TargetVersion         string                            `json:"targetVersion,omitempty"`
	TransitionState       string                            `json:"transitionState,omitempty"`
	TransitionMode        string                            `json:"transitionMode,omitempty"`
	TransitionRunID       string                            `json:"runId,omitempty"`
	LastError             string                            `json:"lastError,omitempty"`
	LastTransitionAt      string                            `json:"lastTransitionAt,omitempty"`
}

type clientRuntimeSessionState struct {
	UserID         string                                `json:"userId"`
	ConversationID string                                `json:"conversationId"`
	Revision       int64                                 `json:"revision"`
	Packages       map[string]*clientRuntimePackageState `json:"packages"`
}

// SSEUIHostNotifier sends UI notifications/dialogs/navigate actions to connected
// UI endpoints via SSE transport.
//
// Production wiring MUST use NewSSEUIHostNotifierWithRegistry (the With-Registry variant).
// The no-registry fallback (NewSSEUIHostNotifier) is for test/standalone use only.
//
// The notifier uses G6 host_registry.Registry (the authoritative connected-device registry)
// to determine the target UI endpoint. Hub verifies transport connection existence only;
// it does NOT replace Registry for target selection.
type SSEUIHostNotifier struct {
	hub                          *sse.Hub
	hostRegistry                 *host_registry.HostRegistry
	mu                           sync.Mutex
	pendingDialogs               map[string]*pendingDialog
	pendingClientRuntimeCommands map[string]*pendingClientRuntimeCommand
	clientRuntimeSessions        map[string]*clientRuntimeSessionState
	clientRuntimeDB              *sql.DB
}

// NewSSEUIHostNotifier creates an SSE UI notifier WITHOUT G6 registry injection.
// Using this constructor in production causes broadcast fallback, which is NOT allowed.
// Production wiring MUST use NewSSEUIHostNotifierWithRegistry.
//
// Deprecated: Production wiring must inject the shared host_registry.Registry.
// This constructor is retained only for test/standalone composition.
func NewSSEUIHostNotifier(hub *sse.Hub) *SSEUIHostNotifier {
	return &SSEUIHostNotifier{
		hub:                          hub,
		pendingDialogs:               make(map[string]*pendingDialog),
		pendingClientRuntimeCommands: make(map[string]*pendingClientRuntimeCommand),
		clientRuntimeSessions:        make(map[string]*clientRuntimeSessionState),
	}
}

// NewSSEUIHostNotifierWithRegistry creates an SSE UI notifier with the G6
// host_registry.Registry for target endpoint selection.
// This is the production constructor. The registry must be the G18-shared instance.
func NewSSEUIHostNotifierWithRegistry(hub *sse.Hub, registry *host_registry.HostRegistry) *SSEUIHostNotifier {
	return &SSEUIHostNotifier{
		hub:                          hub,
		hostRegistry:                 registry,
		pendingDialogs:               make(map[string]*pendingDialog),
		pendingClientRuntimeCommands: make(map[string]*pendingClientRuntimeCommand),
		clientRuntimeSessions:        make(map[string]*clientRuntimeSessionState),
	}
}

func (n *SSEUIHostNotifier) SetClientRuntimeDatabase(db *sql.DB) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.clientRuntimeDB = db
}

func (n *SSEUIHostNotifier) Notify(ctx context.Context, extensionID string, title string, body string, severity string) error {
	if n.hostRegistry != nil {
		target, err := n.hostRegistry.FindTargetHostString(ctx, "", host_registry.CapUINotify, "", "")
		if err != nil {
			return err
		}
		if target == nil {
			return ErrUIHostUnavailable
		}
		if n.hub == nil || !n.hub.ClientExists(target.HostClientID) {
			return ErrUIHostUnavailable
		}
		payload := map[string]interface{}{
			"title":    title,
			"body":     body,
			"severity": severity,
		}
		envelope := NewSSEEventEnvelope("ui_notify", extensionID, payload, defaultEventTTL)
		envelopeMap := envelope.ToMap()
		envelopeMap["hostClientId"] = target.HostClientID
		envelopeMap["hostSessionId"] = target.HostSessionID
		n.hub.SendToClient(target.HostClientID, "ui_notify", envelopeMap)
		return nil
	}

	if n.hub == nil || !n.hub.HasClients() {
		return ErrUIHostUnavailable
	}
	payload := map[string]interface{}{
		"title":    title,
		"body":     body,
		"severity": severity,
	}
	envelope := NewSSEEventEnvelope("ui_notify", extensionID, payload, defaultEventTTL)
	n.hub.Broadcast("ui_notify", envelope.ToMap())
	return nil
}

func (n *SSEUIHostNotifier) Dialog(ctx context.Context, extensionID string, dialogID string, message string, buttons []string) (string, error) {
	if dialogID == "" {
		dialogID = fmt.Sprintf("dialog-%s", uuid.NewString())
	}

	pd := &pendingDialog{
		resultCh: make(chan string, 1),
		errCh:    make(chan error, 1),
	}

	if n.hostRegistry != nil {
		target, err := n.hostRegistry.FindTargetHostString(ctx, "", host_registry.CapUIDialog, "", "")
		if err != nil {
			return "", err
		}
		if target == nil {
			return "", ErrDialogHostUnavailable
		}
		if n.hub == nil || !n.hub.ClientExists(target.HostClientID) {
			return "", ErrDialogHostUnavailable
		}
		pd.hostClientID = target.HostClientID
		pd.hostSessionID = target.HostSessionID

		n.mu.Lock()
		n.pendingDialogs[dialogID] = pd
		n.mu.Unlock()

		payload := map[string]interface{}{
			"dialogId": dialogID,
			"message":  message,
			"buttons":  buttons,
		}
		envelope := NewSSEEventEnvelope("ui_dialog", extensionID, payload, dialogEventTTL)
		envelopeMap := envelope.ToMap()
		envelopeMap["hostClientId"] = target.HostClientID
		envelopeMap["hostSessionId"] = target.HostSessionID
		n.hub.SendToClient(target.HostClientID, "ui_dialog", envelopeMap)
	} else {
		if n.hub == nil || !n.hub.HasClients() {
			return "", ErrDialogHostUnavailable
		}
		n.mu.Lock()
		n.pendingDialogs[dialogID] = pd
		n.mu.Unlock()

		payload := map[string]interface{}{
			"dialogId": dialogID,
			"message":  message,
			"buttons":  buttons,
		}
		envelope := NewSSEEventEnvelope("ui_dialog", extensionID, payload, dialogEventTTL)
		n.hub.Broadcast("ui_dialog", envelope.ToMap())
	}

	defer func() {
		n.mu.Lock()
		delete(n.pendingDialogs, dialogID)
		n.mu.Unlock()
	}()

	select {
	case result := <-pd.resultCh:
		return result, nil
	case err := <-pd.errCh:
		return "", err
	case <-time.After(dialogResponseTimeout):
		return "", fmt.Errorf("dialog %s timed out waiting for host response", dialogID)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (n *SSEUIHostNotifier) Navigate(ctx context.Context, extensionID string, target string) error {
	if n.hostRegistry != nil {
		host, err := n.hostRegistry.FindTargetHostString(ctx, "", host_registry.CapUINavigate, "", "")
		if err != nil {
			return err
		}
		if host == nil {
			return ErrNavigationHostUnavailable
		}
		if n.hub == nil || !n.hub.ClientExists(host.HostClientID) {
			return ErrNavigationHostUnavailable
		}
		payload := map[string]interface{}{
			"target": target,
		}
		envelope := NewSSEEventEnvelope("ui_navigate", extensionID, payload, defaultEventTTL)
		envelopeMap := envelope.ToMap()
		envelopeMap["hostClientId"] = host.HostClientID
		envelopeMap["hostSessionId"] = host.HostSessionID
		n.hub.SendToClient(host.HostClientID, "ui_navigate", envelopeMap)
		return nil
	}

	if n.hub == nil || !n.hub.HasClients() {
		return ErrNavigationHostUnavailable
	}
	payload := map[string]interface{}{
		"target": target,
	}
	envelope := NewSSEEventEnvelope("ui_navigate", extensionID, payload, defaultEventTTL)
	n.hub.Broadcast("ui_navigate", envelope.ToMap())
	return nil
}

func (n *SSEUIHostNotifier) ResolveDialog(dialogID string, result string) bool {
	return n.ResolveDialogWithHost(dialogID, "", "", result)
}

func (n *SSEUIHostNotifier) ResolveDialogWithHost(dialogID string, hostClientID string, hostSessionID string, result string) bool {
	n.mu.Lock()
	pd, ok := n.pendingDialogs[dialogID]
	n.mu.Unlock()
	if !ok {
		return false
	}
	if pd.hostSessionID != "" && pd.hostSessionID != hostSessionID {
		return false
	}
	if pd.hostClientID != "" && pd.hostClientID != hostClientID {
		return false
	}
	select {
	case pd.resultCh <- result:
		return true
	default:
		return false
	}
}

func (n *SSEUIHostNotifier) FailDialog(dialogID string, err error) bool {
	return n.FailDialogWithHost(dialogID, "", "", err)
}

func (n *SSEUIHostNotifier) FailDialogWithHost(dialogID string, hostClientID string, hostSessionID string, err error) bool {
	n.mu.Lock()
	pd, ok := n.pendingDialogs[dialogID]
	n.mu.Unlock()
	if !ok {
		return false
	}
	if pd.hostSessionID != "" && pd.hostSessionID != hostSessionID {
		return false
	}
	if pd.hostClientID != "" && pd.hostClientID != hostClientID {
		return false
	}
	select {
	case pd.errCh <- err:
		return true
	default:
		return false
	}
}

func (n *SSEUIHostNotifier) FailAllPendingDialogs(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, pd := range n.pendingDialogs {
		select {
		case pd.errCh <- err:
		default:
		}
	}
}

func (n *SSEUIHostNotifier) HasPendingDialog(dialogID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := n.pendingDialogs[dialogID]
	return ok
}

func (n *SSEUIHostNotifier) cloneClientRuntimeSessionForScope(userID, conversationID string) *clientRuntimeSessionState {
	n.mu.Lock()
	defer n.mu.Unlock()
	return cloneClientRuntimeSessionState(n.runtimeSessionLocked(userID, conversationID))
}

func (n *SSEUIHostNotifier) restoreClientRuntimeSession(userID, conversationID string, previous *clientRuntimeSessionState) (map[string]interface{}, error) {
	if previous == nil {
		return nil, fmt.Errorf("previous client runtime state is unavailable")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	key := clientRuntimeScopeKey(userID, conversationID)
	current := n.runtimeSessionLocked(userID, conversationID)
	currentClone := cloneClientRuntimeSessionState(current)
	restored := cloneClientRuntimeSessionState(previous)
	if restored == nil {
		return nil, fmt.Errorf("previous client runtime state is invalid")
	}
	restored.UserID, restored.ConversationID = userID, conversationID
	if restored.Revision <= current.Revision {
		restored.Revision = current.Revision + 1
	}
	n.clientRuntimeSessions[key] = restored
	if err := n.persistClientRuntimeSessionLocked(restored); err != nil {
		if currentClone != nil {
			n.clientRuntimeSessions[key] = currentClone
		}
		return nil, err
	}
	return snapshotClientRuntimeState(restored), nil
}

func clientRuntimeActionIsActivation(action string) bool {
	switch strings.TrimSpace(action) {
	case "run", "rollback":
		return true
	default:
		return false
	}
}

func (n *SSEUIHostNotifier) markClientRuntimeTransitionAwaiting(userID, conversationID, packageID, runID string) (map[string]interface{}, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	original := cloneClientRuntimeSessionState(state)
	record := state.Packages[strings.TrimSpace(packageID)]
	if record == nil || record.TransitionRunID == "" || record.TransitionRunID != strings.TrimSpace(runID) {
		return snapshotClientRuntimeState(state), nil
	}
	record.TransitionState = "awaiting_client"
	record.LastTransitionAt = time.Now().UTC().Format(time.RFC3339Nano)
	state.Revision++
	if err := n.persistClientRuntimeSessionLocked(state); err != nil {
		if original != nil {
			n.clientRuntimeSessions[clientRuntimeScopeKey(userID, conversationID)] = original
		}
		return nil, err
	}
	return snapshotClientRuntimeState(state), nil
}

func (n *SSEUIHostNotifier) commitClientRuntimeTransition(userID, conversationID, packageID, runID string) (map[string]interface{}, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	original := cloneClientRuntimeSessionState(state)
	record := state.Packages[strings.TrimSpace(packageID)]
	if record == nil {
		return nil, fmt.Errorf("client package %s is not defined", packageID)
	}
	if record.TransitionRunID == "" || record.TransitionRunID != strings.TrimSpace(runID) {
		return nil, fmt.Errorf("client package %s activation attempt is no longer current", packageID)
	}
	if record.TargetVersion == "" {
		return nil, fmt.Errorf("client package %s activation has no target package", packageID)
	}
	record.ActiveVersion = record.TargetVersion
	record.Running = true
	clearClientRuntimeTransition(record)
	state.Revision++
	if err := n.persistClientRuntimeSessionLocked(state); err != nil {
		if original != nil {
			n.clientRuntimeSessions[clientRuntimeScopeKey(userID, conversationID)] = original
		}
		return nil, err
	}
	return snapshotClientRuntimeState(state), nil
}

func (n *SSEUIHostNotifier) failClientRuntimeTransition(
	userID, conversationID, packageID, runID, reason string,
	previous *clientRuntimeSessionState,
) (map[string]interface{}, error) {
	if previous == nil {
		return nil, fmt.Errorf("previous client runtime state is unavailable")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	key := clientRuntimeScopeKey(userID, conversationID)
	current := n.runtimeSessionLocked(userID, conversationID)
	currentClone := cloneClientRuntimeSessionState(current)
	currentRecord := current.Packages[strings.TrimSpace(packageID)]
	restored := cloneClientRuntimeSessionState(previous)
	if restored == nil {
		return nil, fmt.Errorf("previous client runtime state is invalid")
	}
	restored.UserID, restored.ConversationID = userID, conversationID
	if restored.Packages == nil {
		restored.Packages = make(map[string]*clientRuntimePackageState)
	}
	if currentRecord != nil && currentRecord.TransitionRunID == strings.TrimSpace(runID) {
		record := restored.Packages[strings.TrimSpace(packageID)]
		if record == nil {
			record = cloneClientRuntimePackageState(currentRecord)
			record.ActiveVersion = ""
			record.Running = false
			restored.Packages[strings.TrimSpace(packageID)] = record
		}
		record.TargetVersion = currentRecord.TargetVersion
		record.TransitionRunID = currentRecord.TransitionRunID
		record.TransitionMode = currentRecord.TransitionMode
		record.TransitionState = "failed"
		record.LastError = strings.TrimSpace(reason)
		record.LastTransitionAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if restored.Revision <= current.Revision {
		restored.Revision = current.Revision + 1
	}
	n.clientRuntimeSessions[key] = restored
	if err := n.persistClientRuntimeSessionLocked(restored); err != nil {
		if currentClone != nil {
			n.clientRuntimeSessions[key] = currentClone
		}
		return nil, err
	}
	return snapshotClientRuntimeState(restored), nil
}

func cloneClientRuntimePackageState(record *clientRuntimePackageState) *clientRuntimePackageState {
	if record == nil {
		return nil
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return &clientRuntimePackageState{
			Versions:         make(map[string]map[string]interface{}),
			ApprovedVersions: make(map[string]bool),
		}
	}
	var clone clientRuntimePackageState
	if json.Unmarshal(raw, &clone) != nil {
		return &clientRuntimePackageState{
			Versions:         make(map[string]map[string]interface{}),
			ApprovedVersions: make(map[string]bool),
		}
	}
	if clone.Versions == nil {
		clone.Versions = make(map[string]map[string]interface{})
	}
	if clone.ApprovedVersions == nil {
		clone.ApprovedVersions = make(map[string]bool)
	}
	return &clone
}

func clientRuntimeActionRequiresRollback(action string) bool {
	switch strings.TrimSpace(action) {
	case "run", "stop", "rollback", "undefine":
		return true
	default:
		return false
	}
}

func (n *SSEUIHostNotifier) clientRuntimeSelectedVersion(userID, conversationID, packageID, requested string) (string, bool, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	record := state.Packages[strings.TrimSpace(packageID)]
	if record == nil {
		return "", false, fmt.Errorf("client package %s is not defined in this conversation", packageID)
	}
	version := strings.TrimSpace(requested)
	if version == "" {
		if len(record.Order) == 0 {
			return "", false, fmt.Errorf("client package %s has no versions", packageID)
		}
		version = record.Order[len(record.Order)-1]
	}
	if _, ok := record.Versions[version]; !ok {
		return "", false, fmt.Errorf("client package %s@%s is not defined", packageID, version)
	}
	approved := record.ApproveFutureVersions || (record.ApprovedVersions != nil && record.ApprovedVersions[version])
	return version, approved, nil
}

func (n *SSEUIHostNotifier) approveClientRuntimeVersion(userID, conversationID, packageID, version string, futureVersions bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	original := cloneClientRuntimeSessionState(state)
	record := state.Packages[packageID]
	if record == nil || record.Versions[version] == nil {
		return fmt.Errorf("client package %s@%s is not defined", packageID, version)
	}
	if record.ApprovedVersions == nil {
		record.ApprovedVersions = make(map[string]bool)
	}
	if record.ApprovedVersions[version] && (!futureVersions || record.ApproveFutureVersions) {
		return nil
	}
	record.ApprovedVersions[version] = true
	if futureVersions {
		record.ApproveFutureVersions = true
	}
	state.Revision++
	if err := n.persistClientRuntimeSessionLocked(state); err != nil {
		if original != nil {
			n.clientRuntimeSessions[clientRuntimeScopeKey(userID, conversationID)] = original
		}
		return err
	}
	return nil
}

func (n *SSEUIHostNotifier) requestClientRuntimeApproval(ctx context.Context, userID, conversationID, packageID, version string) (string, error) {
	if n.hostRegistry == nil {
		return "version", nil
	}
	target, err := n.hostRegistry.FindTargetHostString(ctx, userID, host_registry.CapUIDialog, "", "")
	if err != nil {
		return "", err
	}
	if target == nil || n.hub == nil || !n.hub.ClientExists(target.HostClientID) {
		return "", ErrDialogHostUnavailable
	}
	dialogID := "client-runtime-approval-" + uuid.NewString()
	pd := &pendingDialog{
		resultCh:      make(chan string, 1),
		errCh:         make(chan error, 1),
		hostClientID:  target.HostClientID,
		hostSessionID: target.HostSessionID,
	}
	n.mu.Lock()
	n.pendingDialogs[dialogID] = pd
	n.mu.Unlock()
	defer func() {
		n.mu.Lock()
		delete(n.pendingDialogs, dialogID)
		n.mu.Unlock()
	}()
	payload := map[string]interface{}{
		"dialogId":       dialogID,
		"message":        fmt.Sprintf("Allow this conversation to activate dynamic UI package %s@%s?", packageID, version),
		"buttons":        []string{"Allow this version", "Allow future versions", "Deny"},
		"kind":           "client_runtime_approval",
		"conversationId": conversationID,
		"packageId":      packageID,
		"version":        version,
	}
	envelope := NewSSEEventEnvelope("ui_dialog", "com.amitia.builtin.uiagent", payload, dialogEventTTL)
	envelopeMap := envelope.ToMap()
	envelopeMap["hostClientId"] = target.HostClientID
	envelopeMap["hostSessionId"] = target.HostSessionID
	n.hub.SendToClient(target.HostClientID, "ui_dialog", envelopeMap)

	select {
	case result := <-pd.resultCh:
		normalized := strings.ToLower(strings.TrimSpace(result))
		switch normalized {
		case "allow this version", "allow", "允许本版本", "允许":
			return "version", nil
		case "allow future versions", "允许后续版本", "允许未来版本":
			return "future", nil
		default:
			return "deny", nil
		}
	case err := <-pd.errCh:
		return "", err
	case <-time.After(dialogResponseTimeout):
		return "", fmt.Errorf("dynamic client package approval timed out")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (n *SSEUIHostNotifier) ExecuteClientRuntimeCommand(ctx context.Context, action string, payload map[string]interface{}) (map[string]interface{}, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return nil, fmt.Errorf("client runtime command action is required")
	}
	userID, conversationID := clientRuntimeScopeFromPayload(payload)
	if conversationID == "" {
		return nil, fmt.Errorf("client runtime command conversation scope is required")
	}

	if action == "run" {
		packageID := strings.TrimSpace(fmt.Sprint(payload["id"]))
		requestedVersion := strings.TrimSpace(fmt.Sprint(payload["version"]))
		if requestedVersion == "<nil>" {
			requestedVersion = ""
		}
		version, approved, approvalErr := n.clientRuntimeSelectedVersion(userID, conversationID, packageID, requestedVersion)
		if approvalErr != nil {
			return nil, approvalErr
		}
		if !approved {
			decision, approvalErr := n.requestClientRuntimeApproval(ctx, userID, conversationID, packageID, version)
			if approvalErr != nil {
				return nil, approvalErr
			}
			if decision == "deny" {
				return nil, fmt.Errorf("dynamic client package %s@%s was denied by the user", packageID, version)
			}
			if approvalErr := n.approveClientRuntimeVersion(userID, conversationID, packageID, version, decision == "future"); approvalErr != nil {
				return nil, approvalErr
			}
			payload["version"] = version
		}
	}

	previousState := n.cloneClientRuntimeSessionForScope(userID, conversationID)

	var serverResult map[string]interface{}
	var err error
	if action == "define" {
		if err = n.recordClientRuntimeDefinition(userID, conversationID, payload); err != nil {
			return nil, err
		}
		serverResult, err = n.applyClientRuntimeState(userID, conversationID, action, payload, nil)
	} else {
		serverResult, err = n.applyClientRuntimeState(userID, conversationID, action, payload, nil)
	}
	if err != nil {
		return nil, err
	}
	authoritativeState := n.ClientRuntimeSessionState(userID, conversationID)
	serverResult["serverState"] = authoritativeState

	targets, err := n.clientRuntimeTargets(ctx, userID)
	if err != nil {
		return nil, err
	}
	transitionPackageID := strings.TrimSpace(fmt.Sprint(payload["id"]))
	if transitionPackageID == "<nil>" {
		transitionPackageID = ""
	}
	transitionRunID := runtimeMapString(serverResult, "pluginRunId", "runId")
	if len(targets) == 0 {
		if clientRuntimeActionIsActivation(action) && transitionRunID != "" {
			awaitingState, awaitingErr := n.markClientRuntimeTransitionAwaiting(userID, conversationID, transitionPackageID, transitionRunID)
			if awaitingErr != nil {
				return nil, awaitingErr
			}
			serverResult["serverState"] = awaitingState
			serverResult["state"] = "awaiting_client"
		}
		serverResult["delivery"] = "deferred"
		serverResult["hostCount"] = 0
		serverResult["acknowledgedHostCount"] = 0
		return serverResult, nil
	}

	commandID := "client-runtime-" + uuid.NewString()
	allowedHosts := make(map[string]string, len(targets))
	for _, target := range targets {
		allowedHosts[target.HostClientID] = target.HostSessionID
	}
	targetRevision := int64(0)
	if value, ok := authoritativeState["revision"].(int64); ok {
		targetRevision = value
	}
	pending := &pendingClientRuntimeCommand{
		responseCh:     make(chan clientRuntimeHostResponse, len(targets)),
		allowedHosts:   allowedHosts,
		responded:      make(map[string]struct{}, len(targets)),
		targetRevision: targetRevision,
	}
	n.mu.Lock()
	n.pendingClientRuntimeCommands[commandID] = pending
	n.mu.Unlock()
	defer func() {
		n.mu.Lock()
		delete(n.pendingClientRuntimeCommands, commandID)
		n.mu.Unlock()
	}()

	for _, target := range targets {
		commandPayload := map[string]interface{}{
			"commandId":      commandID,
			"action":         action,
			"payload":        payload,
			"reconcileOnly":  true,
			"hostClientId":   target.HostClientID,
			"hostSessionId":  target.HostSessionID,
			"userId":         userID,
			"conversationId": conversationID,
			"sessionState":   authoritativeState,
			"expectResponse": true,
		}
		envelope := NewSSEEventEnvelope("ui_client_runtime_command", "com.amitia.builtin.uiagent", commandPayload, defaultEventTTL)
		n.hub.SendToClient(target.HostClientID, "ui_client_runtime_command", envelope.ToMap())
	}

	hostResults := make([]map[string]interface{}, 0, len(targets))
	acknowledged := make(map[string]struct{}, len(targets))
	timer := time.NewTimer(clientRuntimeCommandTimeout)
	defer timer.Stop()
collect:
	for len(acknowledged) < len(targets) {
		select {
		case response := <-pending.responseCh:
			acknowledged[response.HostClientID] = struct{}{}
			entry := map[string]interface{}{
				"hostClientId":  response.HostClientID,
				"hostSessionId": response.HostSessionID,
			}
			if response.Result != nil {
				entry["result"] = response.Result
			}
			if response.Error != "" {
				entry["error"] = response.Error
			}
			hostResults = append(hostResults, entry)
		case <-timer.C:
			break collect
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	missing := make([]string, 0)
	hostErrors := 0
	activeSuccesses := 0
	deferredHosts := 0
	for _, target := range targets {
		if _, ok := acknowledged[target.HostClientID]; !ok {
			missing = append(missing, target.HostClientID)
		}
	}
	for _, item := range hostResults {
		itemError := strings.TrimSpace(fmt.Sprint(item["error"]))
		if itemError != "" && itemError != "<nil>" {
			hostErrors++
			continue
		}
		result, _ := item["result"].(map[string]interface{})
		if strings.EqualFold(runtimeMapString(result, "state"), "deferred") {
			deferredHosts++
			continue
		}
		activeSuccesses++
	}
	sort.Strings(missing)
	serverResult["hostCount"] = len(targets)
	serverResult["acknowledgedHostCount"] = len(acknowledged)
	serverResult["activeHostSuccessCount"] = activeSuccesses
	serverResult["deferredHostCount"] = deferredHosts
	serverResult["hostResults"] = hostResults
	if len(missing) > 0 {
		serverResult["missingHosts"] = missing
	}

	if clientRuntimeActionIsActivation(action) && transitionRunID != "" {
		if hostErrors > 0 {
			reason := fmt.Sprintf("%d active host(s) rejected runtime activation", hostErrors)
			failedState, failErr := n.failClientRuntimeTransition(userID, conversationID, transitionPackageID, transitionRunID, reason, previousState)
			if failErr != nil {
				return nil, fmt.Errorf("client runtime activation failed and failure state could not be persisted: %w", failErr)
			}
			serverResult["serverState"] = failedState
			serverResult["delivery"] = "failed"
			serverResult["state"] = "failed"
			serverResult["lastError"] = reason
			n.broadcastClientRuntimeSnapshot(targets, userID, conversationID, failedState)
			return serverResult, fmt.Errorf("client runtime activation failed on %d host(s)", hostErrors)
		}
		if activeSuccesses == 0 {
			awaitingState, awaitingErr := n.markClientRuntimeTransitionAwaiting(userID, conversationID, transitionPackageID, transitionRunID)
			if awaitingErr != nil {
				return nil, awaitingErr
			}
			serverResult["serverState"] = awaitingState
			serverResult["delivery"] = "deferred"
			serverResult["state"] = "awaiting_client"
			n.broadcastClientRuntimeSnapshot(targets, userID, conversationID, awaitingState)
			return serverResult, nil
		}

		committedState, commitErr := n.commitClientRuntimeTransition(userID, conversationID, transitionPackageID, transitionRunID)
		if commitErr != nil {
			return nil, commitErr
		}
		serverResult["serverState"] = committedState
		serverResult["state"] = "running"
		serverResult["currentPackageId"] = runtimeMapString(serverResult, "nextPackageId", "version")
		serverResult["nextPackageId"] = ""
		switch {
		case len(missing) == 0 && deferredHosts == 0:
			serverResult["delivery"] = "live"
		default:
			serverResult["delivery"] = "partial"
		}
		n.broadcastClientRuntimeSnapshot(targets, userID, conversationID, committedState)
		return serverResult, nil
	}

	if hostErrors > 0 && clientRuntimeActionRequiresRollback(action) {
		rollbackState, rollbackErr := n.restoreClientRuntimeSession(userID, conversationID, previousState)
		if rollbackErr != nil {
			return nil, fmt.Errorf("client runtime reconciliation failed on %d host(s); rollback failed: %w", hostErrors, rollbackErr)
		}
		serverResult["serverState"] = rollbackState
		serverResult["delivery"] = "rolled_back"
		serverResult["rollbackReason"] = fmt.Sprintf("%d active host(s) rejected runtime revision", hostErrors)
		n.broadcastClientRuntimeSnapshot(targets, userID, conversationID, rollbackState)
		return serverResult, fmt.Errorf("client runtime transition rolled back after %d host reconciliation error(s)", hostErrors)
	}
	switch {
	case len(acknowledged) == len(targets) && hostErrors == 0:
		serverResult["delivery"] = "live"
	case len(acknowledged) == 0:
		serverResult["delivery"] = "pending"
	default:
		serverResult["delivery"] = "partial"
	}
	return serverResult, nil
}

func (n *SSEUIHostNotifier) broadcastClientRuntimeSnapshot(
	targets []*host_registry.HostEntry,
	userID, conversationID string,
	state map[string]interface{},
) {
	if n.hub == nil || len(targets) == 0 || state == nil {
		return
	}
	commandID := "client-runtime-sync-" + uuid.NewString()
	for _, target := range targets {
		if target == nil || strings.TrimSpace(target.HostClientID) == "" {
			continue
		}
		commandPayload := map[string]interface{}{
			"commandId":      commandID,
			"action":         "inspect",
			"payload":        map[string]interface{}{},
			"reconcileOnly":  true,
			"expectResponse": false,
			"hostClientId":   target.HostClientID,
			"hostSessionId":  target.HostSessionID,
			"userId":         userID,
			"conversationId": conversationID,
			"sessionState":   state,
		}
		envelope := NewSSEEventEnvelope("ui_client_runtime_command", "com.amitia.builtin.uiagent", commandPayload, defaultEventTTL)
		n.hub.SendToClient(target.HostClientID, "ui_client_runtime_command", envelope.ToMap())
	}
}

func (n *SSEUIHostNotifier) clientRuntimeTargets(ctx context.Context, userID string) ([]*host_registry.HostEntry, error) {
	if n.hub == nil || n.hostRegistry == nil {
		return nil, nil
	}
	hosts, err := n.hostRegistry.ListReadyHostsString(ctx, userID, host_registry.CapUINotify)
	if err != nil {
		return nil, err
	}
	result := make([]*host_registry.HostEntry, 0, len(hosts))
	for _, host := range hosts {
		if host != nil && n.hub.ClientExists(host.HostClientID) {
			result = append(result, host)
		}
	}
	return result, nil
}

func clientRuntimeScopeFromPayload(payload map[string]interface{}) (string, string) {
	raw, _ := payload["_runtimeScope"].(map[string]interface{})
	if raw == nil {
		return "", ""
	}
	userID, conversationID := strings.TrimSpace(fmt.Sprint(raw["userId"])), strings.TrimSpace(fmt.Sprint(raw["conversationId"]))
	if userID == "<nil>" {
		userID = ""
	}
	if conversationID == "<nil>" {
		conversationID = ""
	}
	return userID, conversationID
}

func clientRuntimeScopeKey(userID, conversationID string) string {
	return strings.TrimSpace(userID) + "\x00" + strings.TrimSpace(conversationID)
}

func (n *SSEUIHostNotifier) runtimeSessionLocked(userID, conversationID string) *clientRuntimeSessionState {
	key := clientRuntimeScopeKey(userID, conversationID)
	state := n.clientRuntimeSessions[key]
	if state != nil {
		return state
	}
	if n.clientRuntimeDB != nil {
		var raw string
		err := n.clientRuntimeDB.QueryRow(`SELECT state_json FROM extension_client_runtime_sessions WHERE user_id = ? AND conversation_id = ?`, userID, conversationID).Scan(&raw)
		if err == nil {
			var loaded clientRuntimeSessionState
			if json.Unmarshal([]byte(raw), &loaded) == nil {
				if loaded.Packages == nil {
					loaded.Packages = make(map[string]*clientRuntimePackageState)
				}
				for _, record := range loaded.Packages {
					if record == nil {
						continue
					}
					if record.Versions == nil {
						record.Versions = make(map[string]map[string]interface{})
					}
					if record.ApprovedVersions == nil {
						record.ApprovedVersions = make(map[string]bool)
					}
				}
				loaded.UserID, loaded.ConversationID = userID, conversationID
				state = &loaded
			}
		}
	}
	if state == nil {
		state = &clientRuntimeSessionState{UserID: userID, ConversationID: conversationID, Packages: make(map[string]*clientRuntimePackageState)}
	}
	n.clientRuntimeSessions[key] = state
	return state
}

func (n *SSEUIHostNotifier) persistClientRuntimeSessionLocked(state *clientRuntimeSessionState) error {
	if n.clientRuntimeDB == nil || state == nil {
		return nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = n.clientRuntimeDB.Exec(`
		INSERT INTO extension_client_runtime_sessions(user_id, conversation_id, state_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, conversation_id) DO UPDATE SET state_json = excluded.state_json, updated_at = excluded.updated_at
	`, state.UserID, state.ConversationID, string(raw), time.Now().UTC())
	return err
}

func (n *SSEUIHostNotifier) recordClientRuntimeDefinition(userID, conversationID string, payload map[string]interface{}) error {
	pkg, ok := payload["package"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("define requires package")
	}
	id, version := strings.TrimSpace(fmt.Sprint(pkg["id"])), strings.TrimSpace(fmt.Sprint(pkg["version"]))
	if id == "" || version == "" || id == "<nil>" || version == "<nil>" {
		return fmt.Errorf("define requires package id/version")
	}
	if err := validateClientRuntimePackageDefinition(pkg); err != nil {
		return err
	}
	copyValue := cloneInterfaceMap(pkg)
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	original := cloneClientRuntimeSessionState(state)
	record := state.Packages[id]
	if record == nil {
		record = &clientRuntimePackageState{Versions: make(map[string]map[string]interface{}), ApprovedVersions: make(map[string]bool)}
		state.Packages[id] = record
	}
	if record.ApprovedVersions == nil {
		record.ApprovedVersions = make(map[string]bool)
	}
	if existing, exists := record.Versions[version]; exists {
		a, _ := json.Marshal(existing)
		b, _ := json.Marshal(copyValue)
		if string(a) != string(b) {
			return fmt.Errorf("client package %s@%s is immutable", id, version)
		}
		return nil
	}
	record.Versions[version] = copyValue
	record.Order = append(record.Order, version)
	state.Revision++
	if err := n.persistClientRuntimeSessionLocked(state); err != nil {
		if original != nil {
			n.clientRuntimeSessions[clientRuntimeScopeKey(userID, conversationID)] = original
		}
		return err
	}
	return nil
}

const (
	maxClientRuntimeContributions     = 128
	maxClientRuntimeConversationNodes = 64
	maxClientRuntimeChildSlots        = 64
	maxClientRuntimeHTMLBytes         = 64 * 1024
	maxClientRuntimeCSSBytes          = 64 * 1024
	maxClientRuntimeScriptBytes       = 128 * 1024
	maxClientRuntimeSandboxHeight     = 2400
)

func validateClientRuntimePackageDefinition(pkg map[string]interface{}) error {
	id := strings.TrimSpace(fmt.Sprint(pkg["id"]))
	version := strings.TrimSpace(fmt.Sprint(pkg["version"]))
	if id == "" || id == "<nil>" || len(id) > 160 {
		return fmt.Errorf("client runtime package id is invalid")
	}
	if version == "" || version == "<nil>" || len(version) > 80 {
		return fmt.Errorf("client runtime package version is invalid")
	}
	contributions, err := runtimeObjectSlice(pkg["contributions"])
	if err != nil {
		return fmt.Errorf("client runtime package contributions: %w", err)
	}
	if len(contributions) > maxClientRuntimeContributions {
		return fmt.Errorf("client runtime package contributions exceed %d", maxClientRuntimeContributions)
	}
	declaredChildren := make(map[string]struct{})
	for index, item := range contributions {
		slotID := runtimeMapString(item, "slotId", "slot_id")
		key := runtimeMapString(item, "key")
		sourceExtensionID := runtimeMapString(item, "sourceExtensionId", "source_extension_id")
		sourceContributionID := runtimeMapString(item, "sourceContributionId", "source_contribution_id")
		clientCode, hasClientCode, codeErr := validateClientRuntimeSandboxCode(item["clientCode"])
		if codeErr != nil {
			return fmt.Errorf("client runtime contribution %d clientCode: %w", index, codeErr)
		}
		_ = clientCode
		if slotID == "" || key == "" {
			return fmt.Errorf("client runtime contribution %d requires slotId/key", index)
		}
		if (sourceExtensionID == "") != (sourceContributionID == "") {
			return fmt.Errorf("client runtime contribution %d requires both sourceExtensionId and sourceContributionId", index)
		}
		if sourceExtensionID == "" && !hasClientCode {
			return fmt.Errorf("client runtime contribution %d requires a published schema source or clientCode", index)
		}
		children, err := runtimeObjectSlice(item["children"])
		if err != nil {
			return fmt.Errorf("client runtime contribution %d children: %w", index, err)
		}
		if len(children) > maxClientRuntimeChildSlots {
			return fmt.Errorf("client runtime contribution %d child slots exceed %d", index, maxClientRuntimeChildSlots)
		}
		for childIndex, child := range children {
			childID := runtimeMapString(child, "slotId", "slot_id")
			if childID == "" || childID == slotID {
				return fmt.Errorf("client runtime contribution %d child %d has invalid slotId", index, childIndex)
			}
			scope := runtimeMapString(child, "scope")
			if scope != "" && scope != "root" && scope != "session-maybe" && scope != "session" {
				return fmt.Errorf("client runtime child slot %s has invalid scope %s", childID, scope)
			}
			if _, duplicate := declaredChildren[childID]; duplicate {
				return fmt.Errorf("client runtime child slot %s is declared more than once", childID)
			}
			declaredChildren[childID] = struct{}{}
			kinds, err := runtimeStringSlice(child["supportedKinds"])
			if err != nil || len(kinds) == 0 {
				return fmt.Errorf("client runtime child slot %s requires supportedKinds", childID)
			}
		}
	}
	conversationNodes, err := runtimeObjectSlice(pkg["conversationNodes"])
	if err != nil {
		return fmt.Errorf("client runtime package conversationNodes: %w", err)
	}
	if len(conversationNodes) > maxClientRuntimeConversationNodes {
		return fmt.Errorf("client runtime package conversationNodes exceed %d", maxClientRuntimeConversationNodes)
	}
	for index, item := range conversationNodes {
		key := runtimeMapString(item, "key")
		sourceExtensionID := runtimeMapString(item, "sourceExtensionId", "source_extension_id")
		sourceContributionID := runtimeMapString(item, "sourceContributionId", "source_contribution_id")
		projection, ok := item["projection"].(map[string]interface{})
		if key == "" || sourceExtensionID == "" || sourceContributionID == "" || !ok {
			return fmt.Errorf("client runtime conversation node %d is invalid", index)
		}
		eventTypes, eventErr := runtimeStringSlice(projection["eventTypes"])
		startEvents, startErr := runtimeStringSlice(projection["startEvents"])
		if eventErr != nil || startErr != nil || len(eventTypes) == 0 || len(startEvents) == 0 {
			return fmt.Errorf("client runtime conversation node %s requires eventTypes and startEvents", key)
		}
	}
	return nil
}

func validateClientRuntimeSandboxCode(value interface{}) (map[string]interface{}, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	code, ok := value.(map[string]interface{})
	if !ok {
		return nil, false, fmt.Errorf("must be an object")
	}
	html := runtimeMapString(code, "html")
	css := runtimeMapString(code, "css")
	script := runtimeMapString(code, "script")
	if html == "" && css == "" && script == "" {
		return nil, false, fmt.Errorf("requires html, css, or script")
	}
	if len([]byte(html)) > maxClientRuntimeHTMLBytes {
		return nil, false, fmt.Errorf("html exceeds %d bytes", maxClientRuntimeHTMLBytes)
	}
	if len([]byte(css)) > maxClientRuntimeCSSBytes {
		return nil, false, fmt.Errorf("css exceeds %d bytes", maxClientRuntimeCSSBytes)
	}
	if len([]byte(script)) > maxClientRuntimeScriptBytes {
		return nil, false, fmt.Errorf("script exceeds %d bytes", maxClientRuntimeScriptBytes)
	}
	minHeight, minSet, err := runtimeOptionalPositiveInt(code["minHeight"], 32, 1200)
	if err != nil {
		return nil, false, fmt.Errorf("minHeight: %w", err)
	}
	maxHeight, maxSet, err := runtimeOptionalPositiveInt(code["maxHeight"], 32, maxClientRuntimeSandboxHeight)
	if err != nil {
		return nil, false, fmt.Errorf("maxHeight: %w", err)
	}
	if minSet && maxSet && maxHeight < minHeight {
		return nil, false, fmt.Errorf("maxHeight must be greater than or equal to minHeight")
	}
	return code, true, nil
}

func runtimeOptionalPositiveInt(value interface{}, minimum, maximum int) (int, bool, error) {
	if value == nil {
		return 0, false, nil
	}
	var numeric float64
	switch typed := value.(type) {
	case float64:
		numeric = typed
	case float32:
		numeric = float64(typed)
	case int:
		numeric = float64(typed)
	case int64:
		numeric = float64(typed)
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false, fmt.Errorf("must be numeric")
		}
		numeric = parsed
	default:
		return 0, false, fmt.Errorf("must be numeric")
	}
	if math.Trunc(numeric) != numeric || numeric < float64(minimum) || numeric > float64(maximum) {
		return 0, false, fmt.Errorf("must be an integer between %d and %d", minimum, maximum)
	}
	return int(numeric), true, nil
}

func runtimeObjectSlice(value interface{}) ([]map[string]interface{}, error) {
	if value == nil {
		return nil, nil
	}
	raw, ok := value.([]interface{})
	if !ok {
		if typed, ok := value.([]map[string]interface{}); ok {
			return typed, nil
		}
		return nil, fmt.Errorf("must be an array")
	}
	result := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("must contain objects")
		}
		result = append(result, row)
	}
	return result, nil
}

func runtimeStringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	if typed, ok := value.([]string); ok {
		return typed, nil
	}
	raw, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("must be an array")
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text == "" || text == "<nil>" {
			return nil, fmt.Errorf("must contain non-empty strings")
		}
		result = append(result, text)
	}
	return result, nil
}

func runtimeMapString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func cloneInterfaceMap(input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func cloneClientRuntimeSessionState(state *clientRuntimeSessionState) *clientRuntimeSessionState {
	if state == nil {
		return nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil
	}
	var clone clientRuntimeSessionState
	if json.Unmarshal(raw, &clone) != nil {
		return nil
	}
	if clone.Packages == nil {
		clone.Packages = make(map[string]*clientRuntimePackageState)
	}
	return &clone
}

func beginClientRuntimeTransition(record *clientRuntimePackageState, version, mode string) string {
	runID := "client-run-" + uuid.NewString()
	record.TargetVersion = strings.TrimSpace(version)
	record.TransitionState = "starting"
	record.TransitionMode = strings.TrimSpace(mode)
	record.TransitionRunID = runID
	record.LastError = ""
	record.LastTransitionAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.Running = true
	return runID
}

func clearClientRuntimeTransition(record *clientRuntimePackageState) {
	if record == nil {
		return
	}
	record.TargetVersion = ""
	record.TransitionState = ""
	record.TransitionMode = ""
	record.TransitionRunID = ""
	record.LastError = ""
	record.LastTransitionAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func (n *SSEUIHostNotifier) applyClientRuntimeState(userID, conversationID, action string, payload, browserResult map[string]interface{}) (map[string]interface{}, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	original := cloneClientRuntimeSessionState(state)
	id := strings.TrimSpace(fmt.Sprint(payload["id"]))
	if id == "<nil>" {
		id = ""
	}
	result := cloneInterfaceMap(browserResult)
	result["scope"] = map[string]interface{}{"userId": userID, "conversationId": conversationID}
	mutated := false
	switch action {
	case "inspect":
		result["serverState"] = snapshotClientRuntimeState(state)
		return result, nil
	case "define":
		pkg, _ := payload["package"].(map[string]interface{})
		result["ok"] = true
		result["id"] = strings.TrimSpace(fmt.Sprint(pkg["id"]))
		result["version"] = strings.TrimSpace(fmt.Sprint(pkg["version"]))
		result["state"] = "defined"
	case "run":
		record := state.Packages[id]
		if record == nil {
			return nil, fmt.Errorf("client package %s is not defined in this conversation", id)
		}
		version := strings.TrimSpace(fmt.Sprint(payload["version"]))
		if version == "" || version == "<nil>" {
			if len(record.Order) == 0 {
				return nil, fmt.Errorf("client package %s has no versions", id)
			}
			version = record.Order[len(record.Order)-1]
		}
		if _, ok := record.Versions[version]; !ok {
			return nil, fmt.Errorf("client package %s@%s is not defined", id, version)
		}
		mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(payload["mode"])))
		if mode == "" || mode == "<nil>" {
			if record.ActiveVersion != "" && record.ActiveVersion != version {
				mode = "update"
			} else {
				mode = "run"
			}
		}
		if mode != "run" && mode != "update" {
			return nil, fmt.Errorf("client runtime run mode must be run or update")
		}
		if mode == "update" && record.ActiveVersion == "" {
			return nil, fmt.Errorf("client package %s has no current package; use run mode", id)
		}
		if mode == "run" && record.ActiveVersion != "" && record.ActiveVersion != version {
			return nil, fmt.Errorf("client package %s current package is %s; use update mode for %s", id, record.ActiveVersion, version)
		}
		runID := beginClientRuntimeTransition(record, version, mode)
		mutated = true
		result["ok"], result["id"], result["version"], result["state"] = true, id, version, "starting"
		result["mode"] = mode
		result["currentPackageId"], result["nextPackageId"], result["pluginRunId"] = record.ActiveVersion, version, runID
	case "stop":
		if record := state.Packages[id]; record != nil {
			record.Running = false
			clearClientRuntimeTransition(record)
			mutated = true
			result["currentPackageId"] = record.ActiveVersion
		}
		result["ok"], result["id"], result["state"] = true, id, "stopped"
	case "rollback":
		record := state.Packages[id]
		if record == nil {
			return nil, fmt.Errorf("client package %s is not defined", id)
		}
		index := len(record.Order)
		for i, version := range record.Order {
			if version == record.ActiveVersion {
				index = i
				break
			}
		}
		if index <= 0 {
			return nil, fmt.Errorf("client package %s has no rollback version", id)
		}
		version := record.Order[index-1]
		runID := beginClientRuntimeTransition(record, version, "update")
		mutated = true
		result["ok"], result["id"], result["version"], result["state"] = true, id, version, "starting"
		result["mode"] = "update"
		result["currentPackageId"], result["nextPackageId"], result["pluginRunId"] = record.ActiveVersion, version, runID
	case "undefine":
		record := state.Packages[id]
		if record == nil {
			break
		}
		mutated = true
		version := strings.TrimSpace(fmt.Sprint(payload["version"]))
		if version == "" || version == "<nil>" {
			version = record.ActiveVersion
			if version == "" && len(record.Order) > 0 {
				version = record.Order[len(record.Order)-1]
			}
		}
		delete(record.Versions, version)
		delete(record.ApprovedVersions, version)
		filtered := record.Order[:0]
		for _, item := range record.Order {
			if item != version {
				filtered = append(filtered, item)
			}
		}
		record.Order = filtered
		if record.TargetVersion == version {
			clearClientRuntimeTransition(record)
		}
		if record.ActiveVersion == version {
			record.ActiveVersion = ""
			record.Running = false
		}
		if len(record.Versions) == 0 {
			delete(state.Packages, id)
		}
		result["ok"], result["id"], result["version"], result["state"] = true, id, version, "undefined"
	default:
		return nil, fmt.Errorf("unsupported client runtime action: %s", action)
	}
	if mutated {
		state.Revision++
	}
	if err := n.persistClientRuntimeSessionLocked(state); err != nil {
		if original != nil {
			n.clientRuntimeSessions[clientRuntimeScopeKey(userID, conversationID)] = original
		}
		return nil, err
	}
	return result, nil
}

func snapshotClientRuntimeState(state *clientRuntimeSessionState) map[string]interface{} {
	ids := make([]string, 0, len(state.Packages))
	for id := range state.Packages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	packages := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		record := state.Packages[id]
		versions := make([]map[string]interface{}, 0, len(record.Order))
		for _, version := range record.Order {
			if pkg := record.Versions[version]; pkg != nil {
				versions = append(versions, cloneInterfaceMap(pkg))
			}
		}
		approved := make([]string, 0, len(record.ApprovedVersions))
		for _, version := range record.Order {
			if record.ApprovedVersions[version] {
				approved = append(approved, version)
			}
		}
		packages = append(packages, map[string]interface{}{
			"id":                    id,
			"versions":              versions,
			"approvedVersions":      approved,
			"approveFutureVersions": record.ApproveFutureVersions,
			"activeVersion":         record.ActiveVersion,
			"currentPackageId":      record.ActiveVersion,
			"targetVersion":         record.TargetVersion,
			"nextPackageId":         record.TargetVersion,
			"transitionState":       record.TransitionState,
			"transitionMode":        record.TransitionMode,
			"runId":                 record.TransitionRunID,
			"pluginRunId":           record.TransitionRunID,
			"lastError":             record.LastError,
			"lastTransitionAt":      record.LastTransitionAt,
			"running":               record.Running,
		})
	}
	return map[string]interface{}{"userId": state.UserID, "conversationId": state.ConversationID, "revision": state.Revision, "packages": packages}
}

func (n *SSEUIHostNotifier) ClientRuntimeSessionState(userID, conversationID string) map[string]interface{} {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	return snapshotClientRuntimeState(state)
}

func (n *SSEUIHostNotifier) AcknowledgeClientRuntimeSession(userID, conversationID string, revision int64) (map[string]interface{}, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.runtimeSessionLocked(userID, conversationID)
	if revision > 0 && state.Revision != revision {
		return snapshotClientRuntimeState(state), fmt.Errorf("client runtime session revision changed from %d to %d", revision, state.Revision)
	}
	original := cloneClientRuntimeSessionState(state)
	changed := false
	for _, record := range state.Packages {
		if record == nil || !record.Running || record.TargetVersion == "" {
			continue
		}
		transition := strings.TrimSpace(record.TransitionState)
		if transition != "awaiting_client" && transition != "starting" {
			continue
		}
		record.ActiveVersion = record.TargetVersion
		clearClientRuntimeTransition(record)
		changed = true
	}
	if !changed {
		return snapshotClientRuntimeState(state), nil
	}
	state.Revision++
	if err := n.persistClientRuntimeSessionLocked(state); err != nil {
		if original != nil {
			n.clientRuntimeSessions[clientRuntimeScopeKey(userID, conversationID)] = original
		}
		return nil, err
	}
	return snapshotClientRuntimeState(state), nil
}

func (n *SSEUIHostNotifier) ResolveClientRuntimeCommand(commandID string, result map[string]interface{}, commandErr string) bool {
	return n.ResolveClientRuntimeCommandWithHost(commandID, "", "", result, commandErr)
}

func (n *SSEUIHostNotifier) ResolveClientRuntimeCommandWithHost(commandID, hostClientID, hostSessionID string, result map[string]interface{}, commandErr string) bool {
	n.mu.Lock()
	pending, ok := n.pendingClientRuntimeCommands[commandID]
	if !ok {
		n.mu.Unlock()
		return false
	}
	if len(pending.allowedHosts) > 0 {
		expectedSession, allowed := pending.allowedHosts[hostClientID]
		if !allowed || (expectedSession != "" && expectedSession != hostSessionID) {
			n.mu.Unlock()
			return false
		}
	}
	if _, duplicate := pending.responded[hostClientID]; duplicate {
		n.mu.Unlock()
		return false
	}
	pending.responded[hostClientID] = struct{}{}
	targetRevision := pending.targetRevision
	n.mu.Unlock()

	if result == nil {
		result = map[string]interface{}{}
	}
	deferred := strings.EqualFold(strings.TrimSpace(fmt.Sprint(result["state"])), "deferred")
	if strings.TrimSpace(commandErr) == "" && targetRevision > 0 && !deferred {
		ackRevision := int64(0)
		switch value := result["revision"].(type) {
		case int64:
			ackRevision = value
		case int:
			ackRevision = int64(value)
		case float64:
			ackRevision = int64(value)
		case json.Number:
			ackRevision, _ = value.Int64()
		default:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(value)), 10, 64); err == nil {
				ackRevision = parsed
			}
		}
		if ackRevision < targetRevision {
			commandErr = fmt.Sprintf("host runtime revision %d is behind target revision %d", ackRevision, targetRevision)
		}
	}
	response := clientRuntimeHostResponse{
		HostClientID:  hostClientID,
		HostSessionID: hostSessionID,
		Result:        result,
		Error:         strings.TrimSpace(commandErr),
	}
	select {
	case pending.responseCh <- response:
		return true
	default:
		return false
	}
}

func (n *SSEUIHostNotifier) BroadcastExtensionChange(eventType string, extensionID string, extra map[string]interface{}) {
	if n.hub == nil || !n.hub.HasClients() {
		return
	}
	payload := map[string]interface{}{
		"extensionId": extensionID,
		"timestamp":   time.Now().UTC().Format(time.RFC3339Nano),
	}
	for k, v := range extra {
		payload[k] = v
	}
	envelope := NewSSEEventEnvelope(eventType, extensionID, payload, defaultEventTTL)
	n.hub.Broadcast(eventType, envelope.ToMap())
}
