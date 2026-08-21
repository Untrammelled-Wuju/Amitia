package extension_slots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type SlotID string

type SlotMultiplicity string

const (
	MultiplicitySingle            SlotMultiplicity = "single"
	MultiplicityMultiple          SlotMultiplicity = "multiple"
	MultiplicityOrderedMultiple   SlotMultiplicity = "ordered_multiple"
	MultiplicityReplaceableSingle SlotMultiplicity = "replaceable_single"
	MultiplicityExclusive         SlotMultiplicity = "exclusive"
)

type SlotLayout string

const (
	LayoutInline SlotLayout = "inline"
	LayoutStack  SlotLayout = "stack"
	LayoutRow    SlotLayout = "row"
	LayoutGrid   SlotLayout = "grid"
	LayoutTabs   SlotLayout = "tabs"
	LayoutPanel  SlotLayout = "panel"
	LayoutDrawer SlotLayout = "drawer"
	LayoutModal  SlotLayout = "modal"
	LayoutHidden SlotLayout = "hidden"
)

type FallbackPolicy string

const (
	FallbackNone     FallbackPolicy = "none"
	FallbackSkeleton FallbackPolicy = "skeleton"
	FallbackEmpty    FallbackPolicy = "empty"
	FallbackDefault  FallbackPolicy = "default"
)

type SlotDefinition struct {
	SlotID            SlotID            `json:"slotId"`
	ContractVersion   int               `json:"contractVersion"`
	SupportedKinds    []string          `json:"supportedKinds"`
	Multiplicity      SlotMultiplicity  `json:"multiplicity"`
	Layout            SlotLayout        `json:"layout"`
	ContextSchema     json.RawMessage   `json:"contextSchema,omitempty"`
	PerformanceBudget PerformanceBudget `json:"performanceBudget"`
	FallbackPolicy    FallbackPolicy    `json:"fallbackPolicy"`
	Description       string            `json:"description,omitempty"`
	Platform          []string          `json:"platform,omitempty"`
	OrderingPolicy    string            `json:"orderingPolicy,omitempty"`
	FailurePolicy     string            `json:"failurePolicy,omitempty"`
	OwnerExtension    string            `json:"ownerExtension,omitempty"`
	ParentSlotID      SlotID            `json:"parentSlotId,omitempty"`
	Dynamic           bool              `json:"dynamic,omitempty"`
}

type PerformanceBudget struct {
	FirstPaint      time.Duration `json:"firstPaint"`
	BundleSize      int64         `json:"bundleSize"`
	MemoryBytes     int64         `json:"memoryBytes"`
	MessageRate     int           `json:"messageRate"`
	UpdateFrequency time.Duration `json:"updateFrequency"`
}

type SlotRegistry struct {
	mu        sync.RWMutex
	slots     map[SlotID]*SlotDefinition
	suspended map[SlotID]*SlotDefinition
}

func NewSlotRegistry() *SlotRegistry {
	return &SlotRegistry{
		slots:     make(map[SlotID]*SlotDefinition),
		suspended: make(map[SlotID]*SlotDefinition),
	}
}

func (r *SlotRegistry) Register(def *SlotDefinition) error {
	return r.register("", def, false)
}

// RegisterOwned registers a slot owned by an extension. Owned slots are dynamic
// lifecycle resources: the registry can later remove every slot owned by the
// extension (including descendants) without affecting built-in slots.
func (r *SlotRegistry) RegisterOwned(ownerExtension string, def *SlotDefinition) error {
	if ownerExtension == "" {
		return ErrDynamicSlotOwnerRequired
	}
	return r.register(ownerExtension, def, true)
}

// RegisterChild registers an extension-owned child slot beneath parentSlotID.
// The parent must already exist. This gives extensions the same compositional
// primitive as host-defined slots instead of requiring all future surfaces to be
// hard-coded in DefaultSlots.
func (r *SlotRegistry) RegisterChild(ownerExtension string, parentSlotID SlotID, def *SlotDefinition) error {
	if def == nil {
		return ErrInvalidSlotDefinition
	}
	copyDef := cloneSlotDefinition(def)
	copyDef.ParentSlotID = parentSlotID
	return r.RegisterOwned(ownerExtension, copyDef)
}

func (r *SlotRegistry) register(ownerExtension string, def *SlotDefinition, dynamic bool) error {
	if def == nil || def.SlotID == "" {
		return ErrInvalidSlotDefinition
	}
	if def.ContractVersion <= 0 {
		return ErrInvalidContractVersion
	}
	if len(def.SupportedKinds) == 0 {
		return ErrNoSupportedKinds
	}
	copyDef := cloneSlotDefinition(def)
	if dynamic {
		copyDef.Dynamic = true
		copyDef.OwnerExtension = ownerExtension
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.slots[copyDef.SlotID]; exists {
		return fmt.Errorf("%w: %s", ErrSlotExists, copyDef.SlotID)
	}
	if copyDef.ParentSlotID != "" {
		if copyDef.ParentSlotID == copyDef.SlotID {
			return ErrSlotParentCycle
		}
		if _, exists := r.slots[copyDef.ParentSlotID]; !exists {
			return fmt.Errorf("%w: %s", ErrParentSlotNotFound, copyDef.ParentSlotID)
		}
	}
	delete(r.suspended, copyDef.SlotID)
	r.slots[copyDef.SlotID] = copyDef
	r.restoreSuspendedChildrenLocked(copyDef.SlotID)
	return nil
}

func (r *SlotRegistry) Get(slotID SlotID) (*SlotDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, exists := r.slots[slotID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSlotNotFound, slotID)
	}
	return cloneSlotDefinition(def), nil
}

func (r *SlotRegistry) List() []*SlotDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SlotDefinition, 0, len(r.slots))
	for _, def := range r.slots {
		out = append(out, cloneSlotDefinition(def))
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i].SlotID) < string(out[j].SlotID)
	})
	return out
}

// Children returns direct child slots in stable lexical order.
func (r *SlotRegistry) Children(parent SlotID) []*SlotDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SlotDefinition, 0)
	for _, def := range r.slots {
		if def.ParentSlotID == parent {
			out = append(out, cloneSlotDefinition(def))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SlotID < out[j].SlotID })
	return out
}

// Unregister removes one dynamic slot from the active graph. Descendant slots
// owned by other extensions are suspended rather than destroyed, so they can
// automatically return if their parent surface is declared again.
func (r *SlotRegistry) Unregister(slotID SlotID) ([]SlotID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	def, exists := r.slots[slotID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSlotNotFound, slotID)
	}
	if !def.Dynamic {
		return nil, fmt.Errorf("%w: %s", ErrStaticSlotImmutable, slotID)
	}
	removed := r.collectDescendantsLocked(slotID)
	for _, id := range removed {
		current, ok := r.slots[id]
		if !ok {
			continue
		}
		delete(r.slots, id)
		if id != slotID {
			r.suspended[id] = cloneSlotDefinition(current)
		}
	}
	delete(r.suspended, slotID)
	return removed, nil
}

// UnregisterOwned removes every slot declaration owned by an extension. Child
// slots owned by other extensions are kept suspended so the dependency graph
// can be reconstructed automatically when the missing parent comes back.
func (r *SlotRegistry) UnregisterOwned(ownerExtension string) []SlotID {
	r.mu.Lock()
	defer r.mu.Unlock()
	ownedRoots := make([]SlotID, 0)
	for id, def := range r.slots {
		if def.Dynamic && def.OwnerExtension == ownerExtension {
			ownedRoots = append(ownedRoots, id)
		}
	}
	set := make(map[SlotID]struct{})
	for _, root := range ownedRoots {
		for _, id := range r.collectDescendantsLocked(root) {
			set[id] = struct{}{}
		}
	}
	for id := range set {
		def, ok := r.slots[id]
		if !ok {
			continue
		}
		delete(r.slots, id)
		if def.OwnerExtension != ownerExtension {
			r.suspended[id] = cloneSlotDefinition(def)
		} else {
			delete(r.suspended, id)
		}
	}
	// A disabled/uninstalled extension must also lose declarations that were
	// already suspended because one of their parents was unavailable.
	for id, def := range r.suspended {
		if def.OwnerExtension == ownerExtension {
			delete(r.suspended, id)
		}
	}
	removed := make([]SlotID, 0, len(set))
	for id := range set {
		removed = append(removed, id)
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
	return removed
}

// Subtree returns an active slot and all currently active descendants. It is
// used to synchronize UIHost after a parent declaration restores suspended
// child slots.
func (r *SlotRegistry) Subtree(root SlotID) []*SlotDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.slots[root]; !ok {
		return nil
	}
	ids := r.collectDescendantsReadLocked(root)
	out := make([]*SlotDefinition, 0, len(ids))
	for _, id := range ids {
		if def, ok := r.slots[id]; ok {
			out = append(out, cloneSlotDefinition(def))
		}
	}
	return out
}

// Suspended returns dynamic declarations that are waiting for an ancestor.
// This is intentionally separate from List(), because suspended slots are not
// valid injection targets until their parent chain is active again.
func (r *SlotRegistry) Suspended() []*SlotDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SlotDefinition, 0, len(r.suspended))
	for _, def := range r.suspended {
		out = append(out, cloneSlotDefinition(def))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SlotID < out[j].SlotID })
	return out
}

func (r *SlotRegistry) restoreSuspendedChildrenLocked(parent SlotID) {
	for {
		restored := false
		ids := make([]SlotID, 0)
		for id, def := range r.suspended {
			if def.ParentSlotID == parent {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			def := r.suspended[id]
			if def == nil {
				continue
			}
			if _, parentExists := r.slots[def.ParentSlotID]; !parentExists {
				continue
			}
			r.slots[id] = cloneSlotDefinition(def)
			delete(r.suspended, id)
			restored = true
		}
		if !restored {
			return
		}
		// Continue until grandchildren whose parents were just restored are also
		// active. The loop is bounded by the number of suspended declarations.
		for _, id := range ids {
			if _, ok := r.slots[id]; ok {
				r.restoreSuspendedChildrenLocked(id)
			}
		}
		return
	}
}

func (r *SlotRegistry) collectDescendantsLocked(root SlotID) []SlotID {
	return r.collectDescendantsReadLocked(root)
}

func (r *SlotRegistry) collectDescendantsReadLocked(root SlotID) []SlotID {
	seen := make(map[SlotID]struct{})
	queue := []SlotID{root}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		for childID, def := range r.slots {
			if def.ParentSlotID == id {
				queue = append(queue, childID)
			}
		}
	}
	out := make([]SlotID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	// Children first is useful to consumers that mirror lifecycle teardown.
	sort.Slice(out, func(i, j int) bool {
		if out[i] == root {
			return false
		}
		if out[j] == root {
			return true
		}
		return out[i] < out[j]
	})
	return out
}

func cloneSlotDefinition(def *SlotDefinition) *SlotDefinition {
	if def == nil {
		return nil
	}
	copyDef := *def
	copyDef.SupportedKinds = append([]string(nil), def.SupportedKinds...)
	copyDef.ContextSchema = append(json.RawMessage(nil), def.ContextSchema...)
	copyDef.Platform = append([]string(nil), def.Platform...)
	return &copyDef
}

func (r *SlotRegistry) SupportsKind(slotID SlotID, kind string) bool {
	def, err := r.Get(slotID)
	if err != nil {
		return false
	}
	for _, k := range def.SupportedKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func DefaultSlotRegistry() *SlotRegistry {
	r := NewSlotRegistry()
	for _, def := range DefaultSlots() {
		_ = r.Register(def)
	}
	return r
}

func DefaultSlots() []*SlotDefinition {
	defaultBudget := PerformanceBudget{
		FirstPaint:      500 * time.Millisecond,
		BundleSize:      5 * 1024 * 1024,
		MemoryBytes:     128 * 1024 * 1024,
		MessageRate:     60,
		UpdateFrequency: 5 * time.Second,
	}
	return []*SlotDefinition{
		{SlotID: "extension.center.header.action", ContractVersion: 1, SupportedKinds: []string{"action", "menu_item"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "扩展中心顶部操作区"},
		{SlotID: "extension.center.card.badge", ContractVersion: 1, SupportedKinds: []string{"card", "badge"}, Multiplicity: MultiplicityMultiple, Layout: LayoutGrid, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "扩展中心卡片徽章"},
		{SlotID: "extension.detail.tab", ContractVersion: 1, SupportedKinds: []string{"schema_page", "web_page"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "扩展详情标签页"},
		{SlotID: "extension.detail.action", ContractVersion: 1, SupportedKinds: []string{"action"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "扩展详情操作区"},
		{SlotID: "extension.settings.page", ContractVersion: 1, SupportedKinds: []string{"schema_page"}, Multiplicity: MultiplicitySingle, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "扩展设置主页面"},
		{SlotID: "extension.settings.section", ContractVersion: 1, SupportedKinds: []string{"schema_page", "panel"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "扩展设置分节"},
		{SlotID: "chat.header.action", ContractVersion: 1, SupportedKinds: []string{"action", "toolbar_item"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "聊天顶部操作"},
		{SlotID: "chat.sidebar.panel", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "聊天侧栏面板"},
		{SlotID: "chat.message.action", ContractVersion: 1, SupportedKinds: []string{"message_action", "action"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "消息操作按钮"},
		{SlotID: "chat.message.renderer", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Multiplicity: MultiplicityReplaceableSingle, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "消息渲染器"},
		{SlotID: "chat.conversation.node", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "会话事件投影节点"},
		{SlotID: "chat.message.custom_renderer", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "扩展消息类型渲染器"},
		{SlotID: "chat.message.attachment_renderer", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "消息附件渲染器"},
		{SlotID: "chat.message.badge", ContractVersion: 1, SupportedKinds: []string{"badge", "status_item", "card"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "消息徽章"},
		{SlotID: "chat.composer.action", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框操作"},
		{SlotID: "chat.composer.attachment", ContractVersion: 1, SupportedKinds: []string{"action"}, Multiplicity: MultiplicityMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框附件"},
		{SlotID: "chat.composer.hint", ContractVersion: 1, SupportedKinds: []string{"status_item", "card", "panel"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框提示"},
		{SlotID: "chat.empty_state.card", ContractVersion: 1, SupportedKinds: []string{"card", "schema_page"}, Multiplicity: MultiplicityMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "聊天空状态卡片"},
		{SlotID: "chat.status.item", ContractVersion: 1, SupportedKinds: []string{"status_item"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "聊天状态项"},
		{SlotID: "character.detail.tab", ContractVersion: 1, SupportedKinds: []string{"schema_page", "web_page"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "角色详情标签页"},
		{SlotID: "character.detail.action", ContractVersion: 1, SupportedKinds: []string{"action"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "角色详情操作"},
		{SlotID: "character.sidebar.card", ContractVersion: 1, SupportedKinds: []string{"card", "panel"}, Multiplicity: MultiplicityMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "角色侧栏卡片"},
		{SlotID: "system.status.item", ContractVersion: 1, SupportedKinds: []string{"status_item"}, Multiplicity: MultiplicityMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "系统状态项"},
		{SlotID: "system.settings.section", ContractVersion: 1, SupportedKinds: []string{"settings_section", "schema_page"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "系统设置分节"},
		{SlotID: "system.diagnostics.tab", ContractVersion: 1, SupportedKinds: []string{"schema_page", "panel"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "系统诊断标签页"},
		{SlotID: "desktop.command", ContractVersion: 1, SupportedKinds: []string{"desktop_command"}, Multiplicity: MultiplicityMultiple, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "桌面命令"},
		{SlotID: "desktop.menu.item", ContractVersion: 1, SupportedKinds: []string{"menu_item"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "桌面菜单项"},
		{SlotID: "desktop.tray.item", ContractVersion: 1, SupportedKinds: []string{"menu_item", "action"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "托盘菜单项"},
		{SlotID: "desktop.window.page", ContractVersion: 1, SupportedKinds: []string{"schema_page", "web_page"}, Multiplicity: MultiplicitySingle, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "桌面独立窗口页面"},
	}
}

type ContributionSummary struct {
	ContributionID  string          `json:"contributionId"`
	ExtensionID     string          `json:"extensionId"`
	ModuleID        string          `json:"moduleId"`
	Kind            string          `json:"kind"`
	SlotID          SlotID          `json:"slotId"`
	ContractVersion int             `json:"contractVersion"`
	Generation      int64           `json:"generation"`
	Title           string          `json:"title"`
	Description     string          `json:"description,omitempty"`
	Icon            string          `json:"icon,omitempty"`
	Ordering        int             `json:"ordering"`
	Visible         bool            `json:"visible"`
	Effective       bool            `json:"effective"`
	Enabled         bool            `json:"enabled"`
	RuntimeReady    bool            `json:"runtimeReady"`
	Permissions     []string        `json:"permissions,omitempty"`
	Sandbox         string          `json:"sandbox,omitempty"`
	EntryPath       string          `json:"entryPath,omitempty"`
	SchemaPath      string          `json:"schemaPath,omitempty"`
	Actions         []ActionSummary `json:"actions,omitempty"`
	HiddenReason    string          `json:"hiddenReason,omitempty"`
}

type ActionSummary struct {
	ActionID  string `json:"actionId"`
	Title     string `json:"title"`
	Icon      string `json:"icon,omitempty"`
	RiskLevel string `json:"riskLevel,omitempty"`
}

type SlotSnapshot struct {
	SlotID          SlotID                 `json:"slotId"`
	ContractVersion int                    `json:"contractVersion"`
	Layout          SlotLayout             `json:"layout"`
	Multiplicity    SlotMultiplicity       `json:"multiplicity"`
	FallbackPolicy  FallbackPolicy         `json:"fallbackPolicy"`
	Contributions   []*ContributionSummary `json:"contributions"`
	GeneratedAt     time.Time              `json:"generatedAt"`
}

type UIContributionSnapshot struct {
	Slots       []*SlotSnapshot `json:"slots"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Version     int             `json:"version"`
}

type SnapshotProvider interface {
	GetSnapshot(ctx context.Context) (*UIContributionSnapshot, error)
}

type SnapshotService struct {
	registry  *SlotRegistry
	resolver  SlotResolver
	mu        sync.RWMutex
	cache     *UIContributionSnapshot
	cacheTTL  time.Duration
	cacheTime time.Time
}

func NewSnapshotService(registry *SlotRegistry, resolver SlotResolver) *SnapshotService {
	return &SnapshotService{
		registry: registry,
		resolver: resolver,
		cacheTTL: 5 * time.Second,
	}
}

func (s *SnapshotService) GetSnapshot(ctx context.Context) (*UIContributionSnapshot, error) {
	s.mu.RLock()
	if s.cache != nil && time.Since(s.cacheTime) < s.cacheTTL {
		snapshot := s.cache
		s.mu.RUnlock()
		return snapshot, nil
	}
	s.mu.RUnlock()

	slots := s.registry.List()
	snap := &UIContributionSnapshot{
		GeneratedAt: time.Now().UTC(),
		Version:     1,
		Slots:       make([]*SlotSnapshot, 0, len(slots)),
	}
	for _, slotDef := range slots {
		contribs, err := s.resolver.Resolve(ctx, slotDef.SlotID)
		if err != nil {
			continue
		}
		snap.Slots = append(snap.Slots, &SlotSnapshot{
			SlotID:          slotDef.SlotID,
			ContractVersion: slotDef.ContractVersion,
			Layout:          slotDef.Layout,
			Multiplicity:    slotDef.Multiplicity,
			FallbackPolicy:  slotDef.FallbackPolicy,
			Contributions:   contribs,
			GeneratedAt:     time.Now().UTC(),
		})
	}
	s.mu.Lock()
	s.cache = snap
	s.cacheTime = time.Now()
	s.mu.Unlock()
	return snap, nil
}

func (s *SnapshotService) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = nil
}

type SlotResolver interface {
	Resolve(ctx context.Context, slotID SlotID) ([]*ContributionSummary, error)
}

type ContextBuilder interface {
	BuildContext(slotID SlotID, pageState map[string]any) (map[string]any, error)
}

type SlotContextBuilder struct {
	builders map[SlotID]ContextBuilder
}

func NewSlotContextBuilder() *SlotContextBuilder {
	return &SlotContextBuilder{
		builders: make(map[SlotID]ContextBuilder),
	}
}

func (b *SlotContextBuilder) Register(slotID SlotID, builder ContextBuilder) {
	b.builders[slotID] = builder
}

func (b *SlotContextBuilder) BuildContext(slotID SlotID, pageState map[string]any) (map[string]any, error) {
	if builder, ok := b.builders[slotID]; ok {
		return builder.BuildContext(slotID, pageState)
	}
	return reduceToSafeContext(pageState), nil
}

func reduceToSafeContext(state map[string]any) map[string]any {
	safeKeys := map[string]bool{
		"messageId": true, "messageType": true, "direction": true,
		"characterId": true, "conversationId": true, "capabilities": true,
		"extensionId": true, "moduleId": true, "platform": true,
		"locale": true, "theme": true, "slotId": true,
	}
	out := make(map[string]any)
	for k, v := range state {
		if safeKeys[k] {
			out[k] = v
		}
	}
	return out
}

type SlotCacheKey struct {
	ContributionID string
	Generation     int64
	SlotContextKey string
}

type SlotCache struct {
	mu      sync.RWMutex
	entries map[SlotCacheKey]any
}

func NewSlotCache() *SlotCache {
	return &SlotCache{entries: make(map[SlotCacheKey]any)}
}

func (c *SlotCache) Get(key SlotCacheKey) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.entries[key]
	return v, ok
}

func (c *SlotCache) Set(key SlotCacheKey, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = value
}

func (c *SlotCache) Invalidate(contributionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if k.ContributionID == contributionID {
			delete(c.entries, k)
		}
	}
}

func (c *SlotCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[SlotCacheKey]any)
}

var (
	ErrInvalidSlotDefinition    = errors.New("extension_slots: invalid slot definition")
	ErrInvalidContractVersion   = errors.New("extension_slots: invalid contract version")
	ErrNoSupportedKinds         = errors.New("extension_slots: no supported kinds")
	ErrSlotExists               = errors.New("extension_slots: slot exists")
	ErrSlotNotFound             = errors.New("extension_slots: slot not found")
	ErrDynamicSlotOwnerRequired = errors.New("extension_slots: dynamic slot owner required")
	ErrParentSlotNotFound       = errors.New("extension_slots: parent slot not found")
	ErrSlotParentCycle          = errors.New("extension_slots: slot cannot be its own parent")
	ErrStaticSlotImmutable      = errors.New("extension_slots: built-in slot is immutable")
)
