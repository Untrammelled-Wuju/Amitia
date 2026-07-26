package ui_contribution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type UIHost struct {
	mu            sync.RWMutex
	slots         map[string]*UISlotContract
	contributions map[string]*UIContributionDefinition
	instances     map[string]*UIInstance
	bridge        *UIBridge
	logger        func(level, msg string, fields map[string]any)
}

type UIInstance struct {
	Definition   *UIContributionDefinition
	State        UILifecycleState
	MountedAt    *time.Time
	LastActiveAt *time.Time
	Failures     int
	LastError    string
	mu           sync.RWMutex
}

func (i *UIInstance) SetState(s UILifecycleState) {
	i.mu.Lock()
	i.State = s
	now := time.Now().UTC()
	i.LastActiveAt = &now
	if s == UIStateMounted || s == UIStateVisible {
		if i.MountedAt == nil {
			i.MountedAt = &now
		}
	}
	i.mu.Unlock()
}

func (i *UIInstance) RecordFailure(err string) int {
	i.mu.Lock()
	i.Failures++
	i.LastError = err
	i.State = UIStateFailed
	count := i.Failures
	i.mu.Unlock()
	return count
}

func (i *UIInstance) Snapshot() UIInstance {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return UIInstance{
		Definition:   i.Definition,
		State:        i.State,
		MountedAt:    i.MountedAt,
		LastActiveAt: i.LastActiveAt,
		Failures:     i.Failures,
		LastError:    i.LastError,
	}
}

func NewUIHost() *UIHost {
	h := &UIHost{
		slots:         make(map[string]*UISlotContract),
		contributions: make(map[string]*UIContributionDefinition),
		instances:     make(map[string]*UIInstance),
		logger:        func(level, msg string, fields map[string]any) {},
	}
	h.bridge = NewUIBridge(h)
	for id, slot := range DefaultSlots {
		h.slots[id] = slot
	}
	return h
}

func (h *UIHost) SetLogger(l func(level, msg string, fields map[string]any)) {
	h.logger = l
}

func (h *UIHost) Bridge() *UIBridge { return h.bridge }

func (h *UIHost) RegisterSlot(slot *UISlotContract) error {
	if slot == nil || slot.SlotID == "" {
		return errors.New("ui_contribution: invalid slot")
	}
	if !slot.Multiplicity.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidMultiplicity, slot.Multiplicity)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.slots[slot.SlotID]; exists {
		return fmt.Errorf("ui_contribution: slot %s already registered", slot.SlotID)
	}
	h.slots[slot.SlotID] = slot
	return nil
}

func (h *UIHost) GetSlot(slotID string) (*UISlotContract, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.slots[slotID]
	return s, ok
}

func (h *UIHost) RegisterContribution(def *UIContributionDefinition) error {
	if err := ValidateDefinition(def); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	slot, ok := h.slots[def.Slot.SlotID]
	if !ok {
		return NewUIError(UIErrSlotUnsupported, "slot not registered: "+def.Slot.SlotID, nil)
	}
	if err := ValidateAgainstSlot(def, slot); err != nil {
		return err
	}
	if slot.Multiplicity == MultiplicitySingle || slot.Multiplicity == MultiplicityExclusive {
		for _, existing := range h.contributions {
			if existing.Slot.SlotID == def.Slot.SlotID {
				return fmt.Errorf("ui_contribution: slot %s is %s, already occupied by %s",
					def.Slot.SlotID, slot.Multiplicity, existing.ContributionID)
			}
		}
	}
	if _, exists := h.contributions[string(def.ContributionID)]; exists {
		return fmt.Errorf("ui_contribution: contribution %s already registered", def.ContributionID)
	}
	h.contributions[string(def.ContributionID)] = def
	h.instances[string(def.ContributionID)] = &UIInstance{
		Definition: def,
		State:      UIStateRegistered,
	}
	return nil
}

func (h *UIHost) UnregisterContribution(id ContributionID) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.contributions[string(id)]; !exists {
		return fmt.Errorf("ui_contribution: contribution %s not found", id)
	}
	if inst, ok := h.instances[string(id)]; ok && inst.State.IsActive() {
		inst.SetState(UIStateUnmounted)
	}
	delete(h.contributions, string(id))
	delete(h.instances, string(id))
	return nil
}

func (h *UIHost) GetContribution(id ContributionID) (*UIContributionDefinition, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	def, ok := h.contributions[string(id)]
	if !ok {
		return nil, fmt.Errorf("ui_contribution: contribution %s not found", id)
	}
	return def, nil
}

func (h *UIHost) ListBySlot(slotID string) []*UIContributionDefinition {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*UIContributionDefinition, 0)
	for _, def := range h.contributions {
		if def.Slot.SlotID == slotID {
			out = append(out, def)
		}
	}
	return out
}

func (h *UIHost) ListAll() []*UIContributionDefinition {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*UIContributionDefinition, 0, len(h.contributions))
	for _, def := range h.contributions {
		out = append(out, def)
	}
	return out
}

func (h *UIHost) GetInstance(id ContributionID) (*UIInstance, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	inst, ok := h.instances[string(id)]
	if !ok {
		return nil, fmt.Errorf("ui_contribution: instance %s not found", id)
	}
	return inst, nil
}

func (h *UIHost) Mount(id ContributionID) error {
	h.mu.Lock()
	inst, ok := h.instances[string(id)]
	h.mu.Unlock()
	if !ok {
		return fmt.Errorf("ui_contribution: instance %s not found", id)
	}
	inst.SetState(UIStateLoading)
	inst.SetState(UIStateMounted)
	inst.SetState(UIStateVisible)
	return nil
}

func (h *UIHost) Unmount(id ContributionID) error {
	h.mu.Lock()
	inst, ok := h.instances[string(id)]
	h.mu.Unlock()
	if !ok {
		return fmt.Errorf("ui_contribution: instance %s not found", id)
	}
	inst.SetState(UIStateHidden)
	inst.SetState(UIStateUnmounted)
	return nil
}

func (h *UIHost) DisableExtension(extensionID ExtensionID) {
	h.mu.RLock()
	ids := make([]string, 0)
	for id, def := range h.contributions {
		if def.ExtensionID == extensionID {
			ids = append(ids, id)
		}
	}
	h.mu.RUnlock()
	for _, id := range ids {
		_ = h.Unmount(ContributionID(id))
	}
}

type BridgeSession struct {
	SessionID       string
	ContributionID  string
	ExtensionID     string
	ModuleID        string
	Generation      int64
	Origin          string
	ContractVersion int
	GrantedScopes   []string
	GrantedPerms    []string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

type UIBridge struct {
	host    *UIHost
	mu      sync.RWMutex
	sessions map[string]*BridgeSession
}

func NewUIBridge(host *UIHost) *UIBridge {
	return &UIBridge{host: host, sessions: make(map[string]*BridgeSession)}
}

func (b *UIBridge) CreateSession(def *UIContributionDefinition, origin string, grantedScopes, grantedPerms []string, lifetime time.Duration) (*BridgeSession, error) {
	if def == nil {
		return nil, errors.New("ui_contribution: nil definition")
	}
	if lifetime <= 0 {
		lifetime = time.Hour
	}
	now := time.Now().UTC()
	sess := &BridgeSession{
		SessionID:       newBridgeSessionID(),
		ContributionID:  string(def.ContributionID),
		ExtensionID:     string(def.ExtensionID),
		ModuleID:        string(def.ModuleID),
		Generation:      def.Integrity.Generation,
		Origin:          origin,
		ContractVersion: def.ContractVersion,
		GrantedScopes:   grantedScopes,
		GrantedPerms:    grantedPerms,
		CreatedAt:       now,
		ExpiresAt:       now.Add(lifetime),
	}
	b.mu.Lock()
	b.sessions[sess.SessionID] = sess
	b.mu.Unlock()
	return sess, nil
}

func (b *UIBridge) ValidateSession(sessionID, contributionID, origin string, contractVersion int) (*BridgeSession, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	sess, ok := b.sessions[sessionID]
	if !ok {
		return nil, NewUIError(UIErrBridgeAuth, "session not found", nil)
	}
	if sess.ContributionID != contributionID {
		return nil, NewUIError(UIErrBridgeAuth, "contribution mismatch", nil)
	}
	if sess.Origin != origin {
		return nil, NewUIError(UIErrBridgeAuth, "origin mismatch", nil)
	}
	if sess.ContractVersion != contractVersion {
		return nil, NewUIError(UIErrBridgeAuth, "contract version mismatch", nil)
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		return nil, NewUIError(UIErrBridgeAuth, "session expired", nil)
	}
	return sess, nil
}

func (b *UIBridge) RevokeSession(sessionID string) {
	b.mu.Lock()
	delete(b.sessions, sessionID)
	b.mu.Unlock()
}

func (b *UIBridge) SessionCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.sessions)
}

type BridgeMessage struct {
	Method      UIBridgeMethod   `json:"method"`
	SessionID   string           `json:"session_id"`
	ContributionID string        `json:"contribution_id"`
	Origin      string           `json:"origin"`
	ContractVersion int          `json:"contract_version"`
	Payload     json.RawMessage  `json:"payload,omitempty"`
}

type BridgeResponse struct {
	OK      bool             `json:"ok"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *UIError         `json:"error,omitempty"`
}

func (b *UIBridge) Handle(ctx context.Context, msg BridgeMessage) BridgeResponse {
	if !msg.Method.Valid() {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, "invalid method", nil)}
	}
	sess, err := b.ValidateSession(msg.SessionID, msg.ContributionID, msg.Origin, msg.ContractVersion)
	if err != nil {
		return BridgeResponse{OK: false, Error: err.(*UIError)}
	}
	switch msg.Method {
	case BridgeUIReady:
		return b.handleReady(ctx, sess)
	case BridgeUILog:
		return b.handleLog(ctx, sess, msg.Payload)
	case BridgeUIActionInvoke:
		return b.handleActionInvoke(ctx, sess, msg.Payload)
	case BridgeUIDataRequest:
		return b.handleDataRequest(ctx, sess, msg.Payload)
	case BridgeUIResizeRequest:
		return BridgeResponse{OK: true, Result: json.RawMessage(`{}`)}
	case BridgeUINavigationRequest:
		return b.handleNavigation(ctx, sess, msg.Payload)
	case BridgeUIDialogRequest:
		return b.handleDialog(ctx, sess, msg.Payload)
	case BridgeUIResourceOpen:
		return b.handleResourceOpen(ctx, sess, msg.Payload)
	case BridgeUIDataSubscribe:
		return BridgeResponse{OK: true, Result: json.RawMessage(`{"subscribed":true}`)}
	}
	return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, "method not implemented", nil)}
}

func (b *UIBridge) handleReady(ctx context.Context, sess *BridgeSession) BridgeResponse {
	return BridgeResponse{
		OK: true,
		Result: mustMarshal(map[string]any{
			"session_id":      sess.SessionID,
			"contribution_id": sess.ContributionID,
			"ready":           true,
		}),
	}
}

func (b *UIBridge) handleLog(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(payload, &p)
	b.host.logger(p.Level, "ui bridge log", map[string]any{
		"contribution": sess.ContributionID, "message": p.Message,
	})
	return BridgeResponse{OK: true}
}

func (b *UIBridge) handleActionInvoke(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		ActionID string          `json:"action_id"`
		Input    json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, err.Error(), nil)}
	}
	def, err := b.host.GetContribution(ContributionID(sess.ContributionID))
	if err != nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrRuntimeUnavailable, err.Error(), nil)}
	}
	action := findAction(def, p.ActionID)
	if action == nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrActionNotDeclared, "action "+p.ActionID+" not declared", nil)}
	}
	return BridgeResponse{
		OK: true,
		Result: mustMarshal(map[string]any{
			"action_id":   action.ActionID,
			"target_type": action.Target.Type,
			"accepted":    true,
		}),
	}
}

func (b *UIBridge) handleDataRequest(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		Key string `json:"key"`
	}
	_ = json.Unmarshal(payload, &p)
	return BridgeResponse{
		OK: true,
		Result: mustMarshal(map[string]any{
			"key":   p.Key,
			"value": nil,
		}),
	}
}

func (b *UIBridge) handleNavigation(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		RouteID  string `json:"route_id"`
		Params   map[string]any `json:"params,omitempty"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, err.Error(), nil)}
	}
	if p.RouteID == "" {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, "route_id required", nil)}
	}
	return BridgeResponse{
		OK: true,
		Result: mustMarshal(map[string]any{
			"navigated": true,
			"route_id":  p.RouteID,
		}),
	}
}

func (b *UIBridge) handleDialog(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		DialogID string `json:"dialog_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, err.Error(), nil)}
	}
	return BridgeResponse{
		OK: true,
		Result: mustMarshal(map[string]any{
			"opened":    true,
			"dialog_id": p.DialogID,
		}),
	}
}

func (b *UIBridge) handleResourceOpen(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		Resource string `json:"resource"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, err.Error(), nil)}
	}
	if p.Resource == "" {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, "resource required", nil)}
	}
	return BridgeResponse{
		OK: true,
		Result: mustMarshal(map[string]any{
			"opened":    true,
			"resource":  p.Resource,
		}),
	}
}

func findAction(def *UIContributionDefinition, actionID string) *UIActionDefinition {
	for i := range def.Actions {
		if def.Actions[i].ActionID == actionID {
			return &def.Actions[i]
		}
	}
	return nil
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func newBridgeSessionID() string {
	return fmt.Sprintf("ui-sess-%d", time.Now().UnixNano())
}
