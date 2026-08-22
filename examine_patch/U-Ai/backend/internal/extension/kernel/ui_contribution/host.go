// Package ui_contribution provides UI contribution lifecycle management for the Extension Kernel.
//
// IMPORTANT: UIHost (sometimes called ContributionHost) represents in-process UI contribution
// slots, definitions, and lifecycle projections. It does NOT represent connected
// desktop/mobile UI hosts. Connected UI endpoint presence is owned by
// host_registry.Registry, not by UIHost.
//
// Boundaries:
//   - UIHost manages: UI contributions, their definitions, instances, bridge sessions
//   - host_registry.Registry manages: connected device/host transport endpoints
//
// BridgeSession is a UI sandbox capability session and is NOT:
//   - The Device Runtime protocol session (deviceruntime.Service)
//   - The host_registry Host Session (host_registry.Registry)
package ui_contribution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// UIHost owns in-process UI contribution slots, definitions, and lifecycle projections.
//
// It does NOT represent connected desktop/mobile UI hosts; connected endpoint presence
// is owned by host_registry.Registry.
//
// Use this type for UI contribution registration, mounting, and UI bridge session management.
// Use host_registry.Registry for finding and communicating with connected UI endpoints.
type UIHost struct {
	mu                   sync.RWMutex
	slots                map[string]*UISlotContract
	contributions        map[string]*UIContributionDefinition
	pendingContributions map[string]*UIContributionDefinition
	instances            map[string]*UIInstance
	bridge               *UIBridge
	logger               func(level, msg string, fields map[string]any)
}

// UIInstance represents the lifecycle state of a registered UI contribution.
// It is a domain-specific lifecycle model and should NOT be confused with:
//   - capability.ProviderInstance (kernel provider instance)
//   - runtimeorchestrator provider/infrastructure instance
type UIInstance struct {
	Definition     *UIContributionDefinition
	State          UILifecycleState
	MountedAt      *time.Time
	LastActiveAt   *time.Time
	Failures       int
	LastError      string
	DesiredMounted bool
	mu             sync.RWMutex
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
		Definition:     i.Definition,
		State:          i.State,
		MountedAt:      i.MountedAt,
		LastActiveAt:   i.LastActiveAt,
		Failures:       i.Failures,
		LastError:      i.LastError,
		DesiredMounted: i.DesiredMounted,
	}
}

func NewUIHost() *UIHost {
	h := &UIHost{
		slots:                make(map[string]*UISlotContract),
		contributions:        make(map[string]*UIContributionDefinition),
		pendingContributions: make(map[string]*UIContributionDefinition),
		instances:            make(map[string]*UIInstance),
		logger:               func(level, msg string, fields map[string]any) {},
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
	h.slots[slot.SlotID] = cloneSlotContract(slot)
	h.attachPendingForSlotLocked(slot.SlotID)
	return nil
}

// UpsertSlot synchronizes a slot contract from the canonical slot registry.
// Built-in startup synchronization uses this method so UIHost cannot drift from
// extension_slots when new host surfaces are added.
func (h *UIHost) UpsertSlot(slot *UISlotContract) error {
	if slot == nil || slot.SlotID == "" {
		return errors.New("ui_contribution: invalid slot")
	}
	if !slot.Multiplicity.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidMultiplicity, slot.Multiplicity)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.slots[slot.SlotID] = cloneSlotContract(slot)
	h.attachPendingForSlotLocked(slot.SlotID)
	return nil
}

// UnregisterSlot removes a dynamic surface from the active graph. Contributions
// targeting the surface are not destroyed; they move into a pending state and
// are automatically reattached if the same slot contract is declared again.
// This mirrors inject-style lifecycle semantics and avoids install-order races.
func (h *UIHost) UnregisterSlot(slotID string) []ContributionID {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.slots[slotID]; !ok {
		return nil
	}
	delete(h.slots, slotID)
	moved := make([]ContributionID, 0)
	for id, def := range h.contributions {
		if def.Slot.SlotID != slotID {
			continue
		}
		h.pendingContributions[id] = def
		delete(h.contributions, id)
		if inst, ok := h.instances[id]; ok {
			inst.SetState(UIStateUnmounted)
		}
		moved = append(moved, ContributionID(id))
	}
	return moved
}

func (h *UIHost) GetSlot(slotID string) (*UISlotContract, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.slots[slotID]
	return cloneSlotContract(s), ok
}

func (h *UIHost) RegisterContribution(def *UIContributionDefinition) error {
	if err := ValidateDefinition(def); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.contributions[string(def.ContributionID)]; exists {
		return fmt.Errorf("ui_contribution: contribution %s already registered", def.ContributionID)
	}
	if _, exists := h.pendingContributions[string(def.ContributionID)]; exists {
		return fmt.Errorf("ui_contribution: contribution %s already registered", def.ContributionID)
	}
	slot, ok := h.slots[def.Slot.SlotID]
	if !ok {
		h.pendingContributions[string(def.ContributionID)] = def
		h.instances[string(def.ContributionID)] = &UIInstance{Definition: def, State: UIStateRegistered}
		h.logger("debug", "ui contribution waiting for slot declaration", map[string]any{
			"contributionId": def.ContributionID,
			"slotId":         def.Slot.SlotID,
		})
		return nil
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
	_, active := h.contributions[string(id)]
	_, pending := h.pendingContributions[string(id)]
	if !active && !pending {
		return fmt.Errorf("ui_contribution: contribution %s not found", id)
	}
	if inst, ok := h.instances[string(id)]; ok && inst.State.IsActive() {
		inst.SetState(UIStateUnmounted)
	}
	delete(h.contributions, string(id))
	delete(h.pendingContributions, string(id))
	delete(h.instances, string(id))
	return nil
}

func (h *UIHost) GetContribution(id ContributionID) (*UIContributionDefinition, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	def, ok := h.contributions[string(id)]
	if !ok {
		def, ok = h.pendingContributions[string(id)]
	}
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

// UnregisterByExtension removes both active and inject-waiting contributions.
// Pending registrations are extension resources too and must not survive an
// uninstall simply because their target slot was absent at the time.
func (h *UIHost) UnregisterByExtension(extensionID ExtensionID) []ContributionID {
	h.mu.Lock()
	defer h.mu.Unlock()
	removed := make([]ContributionID, 0)
	remove := func(source map[string]*UIContributionDefinition) {
		for id, def := range source {
			if def.ExtensionID != extensionID {
				continue
			}
			if inst, ok := h.instances[id]; ok {
				inst.SetState(UIStateUnmounted)
			}
			delete(source, id)
			delete(h.instances, id)
			removed = append(removed, ContributionID(id))
		}
	}
	remove(h.contributions)
	remove(h.pendingContributions)
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
	return removed
}

// ListPending returns contributions whose target slot is currently absent.
// They remain registered with their extension and are reactivated when the
// owning UI surface returns.
func (h *UIHost) ListPending() []*UIContributionDefinition {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*UIContributionDefinition, 0, len(h.pendingContributions))
	for _, def := range h.pendingContributions {
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
	_, pending := h.pendingContributions[string(id)]
	if ok {
		inst.mu.Lock()
		inst.DesiredMounted = true
		inst.mu.Unlock()
	}
	h.mu.Unlock()
	if !ok {
		return fmt.Errorf("ui_contribution: instance %s not found", id)
	}
	// inject-style contributions may be activated before their target slot is
	// declared. Record the desired state now and mount automatically when the
	// slot becomes available.
	if pending {
		inst.SetState(UIStateRegistered)
		return nil
	}
	inst.SetState(UIStateLoading)
	inst.SetState(UIStateMounted)
	inst.SetState(UIStateVisible)
	return nil
}

func (h *UIHost) Unmount(id ContributionID) error {
	h.mu.Lock()
	inst, ok := h.instances[string(id)]
	if ok {
		inst.mu.Lock()
		inst.DesiredMounted = false
		inst.mu.Unlock()
	}
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

func (h *UIHost) attachPendingForSlotLocked(slotID string) {
	slot, ok := h.slots[slotID]
	if !ok {
		return
	}
	ids := make([]string, 0)
	for id, def := range h.pendingContributions {
		if def.Slot.SlotID == slotID {
			ids = append(ids, id)
		}
	}
	// Stable ordering prevents nondeterministic winner selection for replaceable
	// or exclusive slots. Explicit contribution ordering is applied by the
	// frontend ordering engine after attachment.
	sort.Strings(ids)
	for _, id := range ids {
		def := h.pendingContributions[id]
		if err := ValidateAgainstSlot(def, slot); err != nil {
			h.logger("warn", "pending ui contribution rejected by restored slot", map[string]any{
				"contributionId": def.ContributionID,
				"slotId":         slotID,
				"error":          err.Error(),
			})
			continue
		}
		if slot.Multiplicity == MultiplicitySingle || slot.Multiplicity == MultiplicityExclusive {
			occupied := false
			for _, existing := range h.contributions {
				if existing.Slot.SlotID == slotID {
					occupied = true
					break
				}
			}
			if occupied {
				continue
			}
		}
		h.contributions[id] = def
		delete(h.pendingContributions, id)
		if inst, ok := h.instances[id]; ok {
			inst.mu.RLock()
			desiredMounted := inst.DesiredMounted
			inst.mu.RUnlock()
			if desiredMounted {
				inst.SetState(UIStateLoading)
				inst.SetState(UIStateMounted)
				inst.SetState(UIStateVisible)
			} else {
				inst.SetState(UIStateRegistered)
			}
		}
	}
}

func cloneSlotContract(slot *UISlotContract) *UISlotContract {
	if slot == nil {
		return nil
	}
	copySlot := *slot
	copySlot.SupportedKinds = append([]UIContributionKind(nil), slot.SupportedKinds...)
	copySlot.InputSchema = append(json.RawMessage(nil), slot.InputSchema...)
	copySlot.OutputSchema = append(json.RawMessage(nil), slot.OutputSchema...)
	copySlot.AllowedActions = append([]string(nil), slot.AllowedActions...)
	copySlot.AllowedSandboxes = append([]UISandboxType(nil), slot.AllowedSandboxes...)
	return &copySlot
}

// BridgeSession represents a UI sandbox/bridge capability session.
//
// It is NOT a Device Runtime protocol session (deviceruntime.Service) and
// NOT a host_registry Host/Transport session. It is a UI bridge security boundary
// for a single contribution.
type BridgeSession struct {
	SessionID            string
	ContributionID       string
	ExtensionID          string
	ModuleID             string
	Generation           int64
	Origin               string
	ContractVersion      int
	GrantedScopes        []string
	GrantedPerms         []string
	CreatedAt            time.Time
	ExpiresAt            time.Time
	Surface              string
	CharacterID          string
	ConversationID       string
	ScopeSnapshotID      string
	PermissionSnapshotID string
	Token                string
	UsedNonces           map[string]bool
}

type PermissionSnapshotFactory func(sessionID, extensionID, moduleID string, generation int64, characterID, conversationID string, grantedPerms []string, expiresAt time.Time) (string, error)
type SnapshotReleaser func(scopeSnapshotID, permissionSnapshotID string) error

type UIBridge struct {
	host                      *UIHost
	mu                        sync.RWMutex
	sessions                  map[string]*BridgeSession
	scopeSnapshotFactory      func(extensionID, moduleID string, generation int64, characterID, conversationID string) (string, error)
	permissionSnapshotFactory PermissionSnapshotFactory
	snapshotReleaser          SnapshotReleaser
	actionHandler             func(context.Context, *BridgeSession, *UIActionDefinition, json.RawMessage) (json.RawMessage, error)
	dataHandler               func(context.Context, *BridgeSession, string, json.RawMessage) (json.RawMessage, error)
}

func NewUIBridge(host *UIHost) *UIBridge {
	return &UIBridge{host: host, sessions: make(map[string]*BridgeSession)}
}

func (b *UIBridge) SetPermissionSnapshotFactory(fn PermissionSnapshotFactory) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.permissionSnapshotFactory = fn
}

func (b *UIBridge) SetSnapshotReleaser(fn SnapshotReleaser) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.snapshotReleaser = fn
}

func (b *UIBridge) SetScopeSnapshotFactory(fn func(extensionID, moduleID string, generation int64, characterID, conversationID string) (string, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.scopeSnapshotFactory = fn
}

func (b *UIBridge) SetHandlers(
	actionHandler func(context.Context, *BridgeSession, *UIActionDefinition, json.RawMessage) (json.RawMessage, error),
	dataHandler func(context.Context, *BridgeSession, string, json.RawMessage) (json.RawMessage, error),
) {
	b.mu.Lock()
	b.actionHandler = actionHandler
	b.dataHandler = dataHandler
	b.mu.Unlock()
}

func (b *UIBridge) CreateSession(def *UIContributionDefinition, origin string, grantedScopes, grantedPerms []string, surface, characterID, conversationID string, lifetime time.Duration) (*BridgeSession, error) {
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
		Surface:         surface,
		CharacterID:     characterID,
		ConversationID:  conversationID,
		Token:           newBridgeToken(),
		UsedNonces:      make(map[string]bool),
	}

	b.mu.RLock()
	scopeFactory := b.scopeSnapshotFactory
	permFactory := b.permissionSnapshotFactory
	releaser := b.snapshotReleaser
	b.mu.RUnlock()

	if scopeFactory != nil {
		scopeSnapID, err := scopeFactory(string(def.ExtensionID), string(def.ModuleID), def.Integrity.Generation, characterID, conversationID)
		if err != nil {
			return nil, fmt.Errorf("ui_contribution: create scope snapshot: %w", err)
		}
		sess.ScopeSnapshotID = scopeSnapID
	}

	if permFactory != nil {
		permSnapID, err := permFactory(sess.SessionID, sess.ExtensionID, sess.ModuleID, sess.Generation, characterID, conversationID, grantedPerms, sess.ExpiresAt)
		if err != nil {
			if scopeFactory != nil && releaser != nil {
				_ = releaser(sess.ScopeSnapshotID, "")
			}
			return nil, fmt.Errorf("ui_contribution: create permission snapshot: %w", err)
		}
		sess.PermissionSnapshotID = permSnapID
	}

	b.mu.Lock()
	b.sessions[sess.SessionID] = sess
	b.mu.Unlock()
	return sess, nil
}

func (b *UIBridge) ValidateSession(sessionID, contributionID, origin string, contractVersion int, token string, generation int64, nonce string) (*BridgeSession, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	sess, ok := b.sessions[sessionID]
	if !ok {
		return nil, NewUIError(UIErrBridgeAuth, "session not found", nil)
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		return nil, NewUIError(UIErrBridgeAuth, "session expired", nil)
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
	if sess.Token != token {
		return nil, NewUIError(UIErrBridgeAuth, "token mismatch", nil)
	}
	if sess.Generation != generation {
		return nil, NewUIError(UIErrBridgeAuth, "generation mismatch", nil)
	}
	if nonce == "" {
		return nil, NewUIError(UIErrBridgeAuth, "nonce required", nil)
	}
	if sess.UsedNonces[nonce] {
		return nil, NewUIError(UIErrBridgeAuth, "nonce replay detected", nil)
	}
	sess.UsedNonces[nonce] = true
	return sess, nil
}

func (b *UIBridge) RevokeSession(sessionID string) {
	b.mu.Lock()
	sess, ok := b.sessions[sessionID]
	if ok {
		delete(b.sessions, sessionID)
	}
	releaser := b.snapshotReleaser
	b.mu.Unlock()
	if ok && releaser != nil && (sess.ScopeSnapshotID != "" || sess.PermissionSnapshotID != "") {
		_ = releaser(sess.ScopeSnapshotID, sess.PermissionSnapshotID)
	}
}

func (b *UIBridge) RevokeSessionsByContext(characterID, conversationID string) int {
	b.mu.Lock()
	releaser := b.snapshotReleaser
	type snapPair struct{ scope, perm string }
	pairs := make([]snapPair, 0)
	count := 0
	for id, sess := range b.sessions {
		if (characterID != "" && sess.CharacterID == characterID) ||
			(conversationID != "" && sess.ConversationID == conversationID) {
			pairs = append(pairs, snapPair{scope: sess.ScopeSnapshotID, perm: sess.PermissionSnapshotID})
			delete(b.sessions, id)
			count++
		}
	}
	b.mu.Unlock()
	if releaser != nil {
		for _, p := range pairs {
			if p.scope != "" || p.perm != "" {
				_ = releaser(p.scope, p.perm)
			}
		}
	}
	return count
}

func (b *UIBridge) SessionCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.sessions)
}

type BridgeMessage struct {
	Method          UIBridgeMethod  `json:"method"`
	SessionID       string          `json:"session_id"`
	ContributionID  string          `json:"contribution_id"`
	Origin          string          `json:"origin"`
	ContractVersion int             `json:"contract_version"`
	Nonce           string          `json:"nonce,omitempty"`
	Token           string          `json:"token,omitempty"`
	Generation      int64           `json:"generation"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

type BridgeResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *UIError        `json:"error,omitempty"`
}

func (b *UIBridge) Handle(ctx context.Context, msg BridgeMessage) BridgeResponse {
	if !msg.Method.Valid() {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrPayloadInvalid, "invalid method", nil)}
	}
	sess, err := b.ValidateSession(msg.SessionID, msg.ContributionID, msg.Origin, msg.ContractVersion, msg.Token, msg.Generation, msg.Nonce)
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
		return b.handleResize(ctx, sess, msg.Payload)
	case BridgeUINavigationRequest:
		return b.handleNavigation(ctx, sess, msg.Payload)
	case BridgeUIDialogRequest:
		return b.handleDialog(ctx, sess, msg.Payload)
	case BridgeUIResourceOpen:
		return b.handleResourceOpen(ctx, sess, msg.Payload)
	case BridgeUIDataSubscribe:
		return b.handleDataSubscribe(ctx, sess, msg.Payload)
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
	b.mu.RLock()
	handler := b.actionHandler
	b.mu.RUnlock()
	if handler == nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrRuntimeUnavailable, "action handler unavailable", nil)}
	}
	result, err := handler(ctx, sess, action, p.Input)
	if err != nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrRuntimeUnavailable, err.Error(), nil)}
	}
	return BridgeResponse{OK: true, Result: result}
}

func (b *UIBridge) handleDataRequest(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		Key    string          `json:"key"`
		Params json.RawMessage `json:"params,omitempty"`
	}
	_ = json.Unmarshal(payload, &p)
	if !hasScope(sess.GrantedScopes, p.Key) {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrScopeDenied, "data source scope not granted: "+p.Key, nil)}
	}
	b.mu.RLock()
	handler := b.dataHandler
	b.mu.RUnlock()
	if handler == nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrRuntimeUnavailable, "data handler unavailable", nil)}
	}
	result, err := handler(ctx, sess, p.Key, p.Params)
	if err != nil {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrRuntimeUnavailable, err.Error(), nil)}
	}
	return BridgeResponse{OK: true, Result: result}
}

func (b *UIBridge) handleNavigation(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		RouteID string         `json:"route_id"`
		Params  map[string]any `json:"params,omitempty"`
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
			"opened":   true,
			"resource": p.Resource,
		}),
	}
}

func (b *UIBridge) handleDataSubscribe(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	var p struct {
		SourceID string `json:"source_id"`
	}
	_ = json.Unmarshal(payload, &p)
	if !hasScope(sess.GrantedScopes, p.SourceID) {
		return BridgeResponse{OK: false, Error: NewUIError(UIErrScopeDenied, "data source scope not granted: "+p.SourceID, nil)}
	}
	return BridgeResponse{
		OK: true,
		Result: mustMarshal(map[string]any{
			"subscribed": true,
			"source_id":  p.SourceID,
		}),
	}
}

func (b *UIBridge) handleResize(ctx context.Context, sess *BridgeSession, payload json.RawMessage) BridgeResponse {
	return BridgeResponse{OK: true, Result: json.RawMessage(`{"ok":true,"applied":true}`)}
}

func hasScope(scopes []string, target string) bool {
	for _, s := range scopes {
		if s == target {
			return true
		}
	}
	return false
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
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func newBridgeToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
