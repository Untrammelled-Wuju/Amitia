package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type PermissionChecker interface {
	Check(ctx context.Context, extensionID, permission string) (bool, error)
}

type ScopeChecker interface {
	CheckScope(ctx context.Context, extensionID, scopeType, scopeID string) (bool, error)
}

type ActionExecutor interface {
	ExecuteAction(ctx context.Context, binding DesktopActionBinding, extensionID string, scope ScopeContext) (json.RawMessage, error)
}

type ScopeContext struct {
	CharacterID    string
	ConversationID string
	ExtensionID    string
	Global         bool
}

type ResourceOwner struct {
	ExtensionID    string
	ContributionID string
	ResourceType   string
	ResourceHandle string
	AcquiredAt     time.Time
}

type DesktopHost struct {
	mu               sync.RWMutex
	contracts        *DesktopContractRegistry
	conflicts        *ConflictResolver
	contributions    map[string]*ResolvedDesktopContribution
	contribsByExt    map[string][]string
	shortcutsByAccel map[string]string
	resources        []ResourceOwner
	applyReports     map[int64]DesktopApplyReport
	generation       int64
	snapshot         *DesktopSnapshot
	permChecker      PermissionChecker
	scopeChecker     ScopeChecker
	actionExecutor   ActionExecutor
	circuitOpen      atomic.Bool
	circuitFailures  atomic.Int32
	circuitThreshold int32
	circuitResetAt   time.Time
}

func NewDesktopHost() *DesktopHost {
	return &DesktopHost{
		contracts:        NewDesktopContractRegistry(),
		conflicts:        NewConflictResolver(),
		contributions:    make(map[string]*ResolvedDesktopContribution),
		contribsByExt:    make(map[string][]string),
		shortcutsByAccel: make(map[string]string),
		applyReports:     make(map[int64]DesktopApplyReport),
		circuitThreshold: 5,
	}
}

type DesktopApplyReport struct {
	Generation int64     `json:"generation"`
	Hash       string    `json:"hash"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	AppliedAt  time.Time `json:"appliedAt"`
}

func (h *DesktopHost) RecordApplyReport(report DesktopApplyReport) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if report.AppliedAt.IsZero() {
		report.AppliedAt = time.Now().UTC()
	}
	h.applyReports[report.Generation] = report
}

func (h *DesktopHost) GetApplyReport(generation int64) (DesktopApplyReport, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	report, ok := h.applyReports[generation]
	return report, ok
}

func (h *DesktopHost) SetPermissionChecker(p PermissionChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.permChecker = p
}

func (h *DesktopHost) SetScopeChecker(s ScopeChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scopeChecker = s
}

func (h *DesktopHost) SetActionExecutor(e ActionExecutor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.actionExecutor = e
}

func (h *DesktopHost) GetContracts() *DesktopContractRegistry {
	return h.contracts
}

func (h *DesktopHost) GetConflicts() *ConflictResolver {
	return h.conflicts
}

func (h *DesktopHost) RegisterContribution(ctx context.Context, def DesktopContributionDefinition) (*ResolvedDesktopContribution, error) {
	if def.ContributionID == "" || def.ExtensionID == "" || def.ModuleID == "" {
		return nil, ErrInvalidDefinition
	}
	if def.DesktopType == "" || def.ContractID == "" || def.ContractVersion <= 0 {
		return nil, ErrInvalidDefinition
	}
	if def.Target == "" {
		return nil, ErrInvalidDefinition
	}
	if err := def.Action.Validate(); err != nil {
		return nil, err
	}
	if !h.contracts.IsTargetAllowed(def.ContractID, def.ContractVersion, def.Target) {
		return nil, fmt.Errorf("%w: target %s not allowed for contract %s v%d", ErrInvalidMenuTarget, def.Target, def.ContractID, def.ContractVersion)
	}
	permissionGranted := true
	h.mu.RLock()
	permissionChecker := h.permChecker
	h.mu.RUnlock()
	if permissionChecker != nil {
		permissionID := permissionForType(def.DesktopType, def.Shortcut != nil && def.Shortcut.Global)
		granted, err := permissionChecker.Check(ctx, def.ExtensionID, permissionID)
		permissionGranted = err == nil && granted
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.contributions[def.ContributionID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrContributionExists, def.ContributionID)
	}
	maxItems := h.contracts.MaxItemsPerExtension(def.ContractID, def.ContractVersion)
	if maxItems > 0 {
		if len(h.contribsByExt[def.ExtensionID]) >= maxItems {
			return nil, fmt.Errorf("%w: extension %s has %d items, max %d", ErrTooManyItems, def.ExtensionID, len(h.contribsByExt[def.ExtensionID]), maxItems)
		}
	}
	resolved := &ResolvedDesktopContribution{
		Definition: def,
		Status:     ContributionStatusDeclared,
		Generation: atomic.AddInt64(&h.generation, 1),
		ResolvedAt: time.Now().UTC(),
	}
	if def.Shortcut != nil {
		accel := def.Shortcut.Accelerator
		vr := ValidateAccelerator(accel)
		if !vr.Valid {
			if vr.IsReserved {
				resolved.Status = ContributionStatusConflict
				resolved.ConflictReason = "reserved shortcut: " + accel
				h.conflicts.createConflict(ConflictTypeShortcut, ConflictSeverityBlock,
					def.Target, "", "", def.ContributionID, def.ExtensionID, accel)
			} else {
				resolved.Status = ContributionStatusUnsupported
				resolved.ConflictReason = vr.Reason
			}
		} else {
			normalized := vr.Normalized
			if existingID, exists := h.shortcutsByAccel[normalized]; exists {
				resolved.Status = ContributionStatusConflict
				resolved.ConflictReason = fmt.Sprintf("accelerator %s already used by %s", normalized, existingID)
				var existingDef DesktopContributionDefinition
				if existing, ok := h.contributions[existingID]; ok {
					existingDef = existing.Definition
				}
				h.conflicts.DetectShortcutConflict(&existingDef, &def)
			} else {
				h.shortcutsByAccel[normalized] = def.ContributionID
				resolved.Status = ContributionStatusRegistered
				resolved.EffectiveLabel = def.Label.Get("")
			}
		}
	} else {
		resolved.Status = ContributionStatusRegistered
		resolved.EffectiveLabel = def.Label.Get("")
	}
	if resolved.Status == ContributionStatusRegistered {
		if !permissionGranted {
			resolved.Status = ContributionStatusPendingPermission
			if def.Shortcut != nil {
				delete(h.shortcutsByAccel, NormalizeAccelerator(def.Shortcut.Accelerator))
			}
		}
	}
	if resolved.Status == ContributionStatusRegistered {
		for _, existing := range h.contributions {
			if existing.Definition.Target == def.Target {
				if conflict := h.conflicts.DetectMenuIDConflict(&existing.Definition, &def); conflict != nil {
					resolved.Status = ContributionStatusConflict
					resolved.ConflictReason = string(conflict.Type) + " conflict"
					break
				}
			}
		}
	}
	h.contributions[def.ContributionID] = resolved
	h.contribsByExt[def.ExtensionID] = append(h.contribsByExt[def.ExtensionID], def.ContributionID)
	if resolved.Status == ContributionStatusRegistered {
		h.trackResource(def.ExtensionID, def.ContributionID, string(def.DesktopType), def.ContributionID)
	}
	return resolved, nil
}

func (h *DesktopHost) UnregisterContribution(contributionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	resolved, exists := h.contributions[contributionID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrContributionNotFound, contributionID)
	}
	extID := resolved.Definition.ExtensionID
	delete(h.contributions, contributionID)
	newList := make([]string, 0, len(h.contribsByExt[extID]))
	for _, id := range h.contribsByExt[extID] {
		if id != contributionID {
			newList = append(newList, id)
		}
	}
	if len(newList) == 0 {
		delete(h.contribsByExt, extID)
	} else {
		h.contribsByExt[extID] = newList
	}
	if resolved.Definition.Shortcut != nil {
		normalized := NormalizeAccelerator(resolved.Definition.Shortcut.Accelerator)
		delete(h.shortcutsByAccel, normalized)
	}
	h.releaseResource(contributionID)
	atomic.AddInt64(&h.generation, 1)
	return nil
}

func (h *DesktopHost) UnregisterByExtension(extensionID string) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids := h.contribsByExt[extensionID]
	count := 0
	for _, id := range ids {
		if resolved, ok := h.contributions[id]; ok {
			if resolved.Definition.Shortcut != nil {
				normalized := NormalizeAccelerator(resolved.Definition.Shortcut.Accelerator)
				delete(h.shortcutsByAccel, normalized)
			}
			delete(h.contributions, id)
			h.releaseResource(id)
			count++
		}
	}
	delete(h.contribsByExt, extensionID)
	h.conflicts.ClearByExtension(extensionID)
	if count > 0 {
		atomic.AddInt64(&h.generation, 1)
	}
	return count, nil
}

func (h *DesktopHost) GetContribution(contributionID string) (*ResolvedDesktopContribution, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.contributions[contributionID]
	return c, ok
}

func (h *DesktopHost) ListContributions() []ResolvedDesktopContribution {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]ResolvedDesktopContribution, 0, len(h.contributions))
	for _, c := range h.contributions {
		result = append(result, *c)
	}
	return result
}

func (h *DesktopHost) ListByExtension(extensionID string) []ResolvedDesktopContribution {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]ResolvedDesktopContribution, 0)
	for _, id := range h.contribsByExt[extensionID] {
		if c, ok := h.contributions[id]; ok {
			result = append(result, *c)
		}
	}
	return result
}

func (h *DesktopHost) ListByTarget(target string) []ResolvedDesktopContribution {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]ResolvedDesktopContribution, 0)
	for _, c := range h.contributions {
		if c.Definition.Target == target {
			result = append(result, *c)
		}
	}
	return result
}

func (h *DesktopHost) BuildSnapshot(sortCtx SortContext) *DesktopSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	contribs := make([]ResolvedDesktopContribution, 0, len(h.contributions))
	for _, c := range h.contributions {
		contribs = append(contribs, *c)
	}
	conflicts := h.conflicts.ListAll()
	builder := NewSnapshotBuilder(atomic.LoadInt64(&h.generation))
	snapshot := builder.Build(contribs, conflicts, sortCtx)
	h.snapshot = snapshot
	return snapshot
}

func (h *DesktopHost) GetSnapshot() *DesktopSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot
}

func (h *DesktopHost) InvokeAction(ctx context.Context, contributionID string, scope ScopeContext) (json.RawMessage, error) {
	if h.circuitOpen.Load() {
		if time.Now().Before(h.circuitResetAt) {
			return nil, ErrCircuitOpen
		}
		h.circuitOpen.Store(false)
		h.circuitFailures.Store(0)
	}
	h.mu.RLock()
	resolved, exists := h.contributions[contributionID]
	executor := h.actionExecutor
	permChecker := h.permChecker
	h.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrContributionNotFound, contributionID)
	}
	if resolved.Status != ContributionStatusRegistered {
		return nil, fmt.Errorf("%w: contribution status is %s", ErrQuarantined, resolved.Status)
	}
	if permChecker != nil {
		permID := permissionForType(resolved.Definition.DesktopType, resolved.Definition.Shortcut != nil && resolved.Definition.Shortcut.Global)
		ok, err := permChecker.Check(ctx, resolved.Definition.ExtensionID, permID)
		if err != nil {
			h.recordFailure()
			return nil, err
		}
		if !ok {
			return nil, ErrPermissionDenied
		}
	}
	if executor == nil {
		return nil, fmt.Errorf("desktop: action executor not set")
	}
	result, err := executor.ExecuteAction(ctx, resolved.Definition.Action, resolved.Definition.ExtensionID, scope)
	if err != nil {
		h.recordFailure()
		return nil, err
	}
	h.circuitFailures.Store(0)
	return result, nil
}

func (h *DesktopHost) recordFailure() {
	failures := h.circuitFailures.Add(1)
	if failures >= h.circuitThreshold {
		h.circuitOpen.Store(true)
		h.circuitResetAt = time.Now().Add(30 * time.Second)
	}
}

func (h *DesktopHost) ResetCircuit() {
	h.circuitOpen.Store(false)
	h.circuitFailures.Store(0)
}

func (h *DesktopHost) trackResource(extID, contribID, resType, handle string) {
	h.resources = append(h.resources, ResourceOwner{
		ExtensionID:    extID,
		ContributionID: contribID,
		ResourceType:   resType,
		ResourceHandle: handle,
		AcquiredAt:     time.Now().UTC(),
	})
}

func (h *DesktopHost) releaseResource(contribID string) {
	newResources := make([]ResourceOwner, 0, len(h.resources))
	for _, r := range h.resources {
		if r.ContributionID != contribID {
			newResources = append(newResources, r)
		}
	}
	h.resources = newResources
}

func (h *DesktopHost) ListResources() []ResourceOwner {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]ResourceOwner, len(h.resources))
	copy(result, h.resources)
	return result
}

func (h *DesktopHost) ListResourcesByExtension(extensionID string) []ResourceOwner {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]ResourceOwner, 0)
	for _, r := range h.resources {
		if r.ExtensionID == extensionID {
			result = append(result, r)
		}
	}
	return result
}

func permissionForType(desktopType DesktopType, isGlobal bool) string {
	switch desktopType {
	case DesktopTypeMenuItem, DesktopTypeMenuSubmenu:
		return "desktop.menu.execute"
	case DesktopTypeTrayItem, DesktopTypeTraySubmenu:
		return "desktop.tray.execute"
	case DesktopTypeAppShortcut:
		return "desktop.shortcut.application.execute"
	case DesktopTypeGlobalShortcut:
		return "desktop.shortcut.global.execute"
	default:
		return "desktop.menu.execute"
	}
}

func (h *DesktopHost) EnableExtension(ctx context.Context, extensionID string) {
	permissions := make(map[string]bool)
	for _, contribution := range h.ListByExtension(extensionID) {
		granted := true
		if h.permChecker != nil {
			permissionID := permissionForType(contribution.Definition.DesktopType, contribution.Definition.Shortcut != nil && contribution.Definition.Shortcut.Global)
			ok, err := h.permChecker.Check(ctx, extensionID, permissionID)
			granted = err == nil && ok
		}
		permissions[contribution.Definition.ContributionID] = granted
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range h.contribsByExt[extensionID] {
		if resolved, ok := h.contributions[id]; ok {
			if permissions[id] {
				resolved.Status = ContributionStatusRegistered
			} else {
				resolved.Status = ContributionStatusPendingPermission
			}
			resolved.Generation = atomic.AddInt64(&h.generation, 1)
			resolved.ResolvedAt = time.Now().UTC()
		}
	}
}

func (h *DesktopHost) DisableExtension(ctx context.Context, extensionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range h.contribsByExt[extensionID] {
		if resolved, ok := h.contributions[id]; ok {
			resolved.Status = ContributionStatusDisabled
			resolved.Generation = atomic.AddInt64(&h.generation, 1)
		}
	}
}

func (h *DesktopHost) UninstallContributions(ctx context.Context, extensionID string) error {
	_, err := h.UnregisterByExtension(extensionID)
	return err
}
