package extension_slots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	amitialog "log"
	"sort"
	"sync"
	"time"
)

type SlotID string

type SlotDispatchKind string
type SlotScope string

const (
	ScopeRoot         SlotScope = "root"
	ScopeSessionMaybe SlotScope = "session-maybe"
	ScopeSession      SlotScope = "session"
)

const (
	DispatchSingle SlotDispatchKind = "single"
	DispatchList   SlotDispatchKind = "list"
	DispatchKeyed  SlotDispatchKind = "keyed"
	DispatchChain  SlotDispatchKind = "chain"
	DispatchHoc    SlotDispatchKind = "hoc"
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
	DispatchKind      SlotDispatchKind  `json:"kind"`
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
	Scope             SlotScope         `json:"scope"`
	AllowRootChild    bool              `json:"allowRootChild,omitempty"`
	DeclarationEpoch  uint64            `json:"declarationEpoch"`
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
	epochs    map[SlotID]uint64
}

func NewSlotRegistry() *SlotRegistry {
	return &SlotRegistry{
		slots:     make(map[SlotID]*SlotDefinition),
		suspended: make(map[SlotID]*SlotDefinition),
		epochs:    make(map[SlotID]uint64),
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
	if !validDispatchKind(copyDef.DispatchKind) {
		return fmt.Errorf("%w: %s", ErrInvalidSlotDispatchKind, copyDef.DispatchKind)
	}
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
		parent, exists := r.slots[copyDef.ParentSlotID]
		if !exists {
			return fmt.Errorf("%w: %s", ErrParentSlotNotFound, copyDef.ParentSlotID)
		}
		if copyDef.Scope == "" {
			copyDef.Scope = parent.Scope
		}
		if !scopeCanContain(parent.Scope, copyDef.Scope) && !(copyDef.AllowRootChild && copyDef.Scope == ScopeRoot) {
			return fmt.Errorf("%w: %s", ErrSlotScopeEscape, copyDef.SlotID)
		}
	}
	if copyDef.Scope == "" {
		copyDef.Scope = ScopeRoot
	}
	if copyDef.Scope != ScopeRoot && copyDef.Scope != ScopeSessionMaybe && copyDef.Scope != ScopeSession {
		return fmt.Errorf("%w: %s", ErrInvalidSlotScope, copyDef.Scope)
	}
	delete(r.suspended, copyDef.SlotID)
	copyDef.DeclarationEpoch = r.nextEpochLocked(copyDef.SlotID)
	r.slots[copyDef.SlotID] = copyDef
	r.restoreSuspendedChildrenLocked(copyDef.SlotID)
	return nil
}

func validDispatchKind(kind SlotDispatchKind) bool {
	switch kind {
	case DispatchSingle, DispatchList, DispatchKeyed, DispatchChain, DispatchHoc:
		return true
	default:
		return false
	}
}

func scopeCanContain(parent SlotScope, child SlotScope) bool {
	switch parent {
	case ScopeSession:
		return child == ScopeSession
	case ScopeSessionMaybe:
		return child == ScopeSessionMaybe || child == ScopeSession
	default:
		return child == ScopeRoot || child == ScopeSessionMaybe || child == ScopeSession
	}
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
			restoredDef := cloneSlotDefinition(def)
			restoredDef.DeclarationEpoch = r.nextEpochLocked(id)
			r.slots[id] = restoredDef
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

func (r *SlotRegistry) nextEpochLocked(slotID SlotID) uint64 {
	next := r.epochs[slotID] + 1
	r.epochs[slotID] = next
	return next
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
		if err := r.Register(def); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
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
	slots := []*SlotDefinition{
		{SlotID: "root", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "UI 根组合槽位"},
		{SlotID: "provider.app.shell", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "应用 Shell Provider 组合边界"},
		{SlotID: "provider.app.navigation", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "应用导航 Provider 组合边界"},
		{SlotID: "provider.app.workspace", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "应用工作区 Provider 组合边界"},
		{SlotID: "provider.app.workspace.sidebar", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "应用工作区第二栏 Provider 组合边界"},
		{SlotID: "provider.route.registry", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "路由注册 Provider 组合边界"},
		{SlotID: "provider.page.provider", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "页面 Provider 组合边界"},
		{SlotID: "provider.conversation.shell", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话 Shell Provider 组合边界"},
		{SlotID: "provider.conversation.header", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话头部 Provider 组合边界"},
		{SlotID: "provider.conversation.messages", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话消息区 Provider 组合边界"},
		{SlotID: "provider.conversation.message_renderer", ContractVersion: 1, SupportedKinds: []string{"message_renderer", "panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "消息渲染 Provider 组合边界"},
		{SlotID: "provider.conversation.sidebar", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话侧栏 Provider 组合边界"},
		{SlotID: "provider.conversation.composer", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话输入区 Provider 组合边界"},
		{SlotID: "provider.conversation.overlay", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话浮层 Provider 组合边界"},
		{SlotID: "provider.character.shell", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "角色 Shell Provider 组合边界"},
		{SlotID: "provider.character.detail", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "角色详情 Provider 组合边界"},
		{SlotID: "provider.memory.shell", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "记忆 Shell Provider 组合边界"},
		{SlotID: "provider.memory.detail", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "记忆详情 Provider 组合边界"},
		{SlotID: "provider.settings.shell", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "设置 Shell Provider 组合边界"},
		{SlotID: "provider.settings.section", ContractVersion: 1, SupportedKinds: []string{"settings_section", "panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "设置分节 Provider 组合边界"},
		{SlotID: "provider.extension.center", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "扩展中心 Provider 组合边界"},
		{SlotID: "provider.extension.page", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "扩展页面 Provider 组合边界"},
		{SlotID: "provider.ui.theme", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "主题 Provider 组合边界"},
		{SlotID: "provider.ui.tokens", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "Token Provider 组合边界"},
		{SlotID: "provider.ui.icons", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "图标 Provider 组合边界"},
		{SlotID: "provider.ui.components", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "组件 Provider 组合边界"},
		{SlotID: "app.shell.quota_notice", ContractVersion: 1, SupportedKinds: []string{"status_item", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "应用壳全局额度通知链"},
		{SlotID: "app.workspace.toast", ContractVersion: 1, SupportedKinds: []string{"status_item", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "工作区通知按业务判别键渲染区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "app.shell.bottom", ContractVersion: 1, SupportedKinds: []string{"panel", "status_item"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "应用壳全宽底部内容"},
		{SlotID: "app.rightbar", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutPanel, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "应用右侧栏根容器", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "app.rightbar.session", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutPanel, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "应用右侧栏会话内容", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "desktop.context_menu.content", ContractVersion: 1, SupportedKinds: []string{"menu_item", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "桌面右键菜单项内容替换区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "desktop.context_menu.icon", ContractVersion: 1, SupportedKinds: []string{"menu_item", "icon"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "桌面右键菜单项图标替换区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "workbench.file.actions", ContractVersion: 1, SupportedKinds: []string{"action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "工作台文件预览操作区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "workbench.review.file.actions", ContractVersion: 1, SupportedKinds: []string{"action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "工作台变更文件审查操作区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.model.provider", ContractVersion: 1, SupportedKinds: []string{"settings_section", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "模型 Provider 卡片扩展区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.model.footer", ContractVersion: 1, SupportedKinds: []string{"settings_section", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "模型 Provider 列表底部扩展区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.shell", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutPanel, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "设置面板根容器", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.shell.header", ContractVersion: 1, SupportedKinds: []string{"status_item", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "设置面板标题", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.shell.action", ContractVersion: 1, SupportedKinds: []string{"action", "menu_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "设置面板关闭前操作区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.shell.close", ContractVersion: 1, SupportedKinds: []string{"status_item", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "设置面板关闭按钮内容", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.shell.launcher", ContractVersion: 1, SupportedKinds: []string{"action", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "侧栏设置启动器", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.shell.launcher.notifications", ContractVersion: 1, SupportedKinds: []string{"status_item", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "设置启动器通知区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.onboarding", ContractVersion: 1, SupportedKinds: []string{"settings_section", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "设置引导步骤区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.section.icon", ContractVersion: 1, SupportedKinds: []string{"icon", "status_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "设置分节导航图标", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.shell.trigger", ContractVersion: 1, SupportedKinds: []string{"action", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "侧栏设置触发行内容", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "settings.model.sign-in", ContractVersion: 1, SupportedKinds: []string{"settings_section", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "模型配置凭据编辑器登录位", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "extension.center.header.action", ContractVersion: 1, SupportedKinds: []string{"action", "menu_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "扩展中心顶部操作区"},
		{SlotID: "extension.center.card.badge", ContractVersion: 1, SupportedKinds: []string{"card", "badge"}, Layout: LayoutGrid, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "扩展中心卡片徽章"},
		{SlotID: "extension.detail.tab", ContractVersion: 1, SupportedKinds: []string{"schema_page", "web_page"}, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "扩展详情标签页"},
		{SlotID: "extension.detail.action", ContractVersion: 1, SupportedKinds: []string{"action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "扩展详情操作区"},
		{SlotID: "extension.settings.page", ContractVersion: 1, SupportedKinds: []string{"schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "扩展设置主页面"},
		{SlotID: "extension.settings.section", ContractVersion: 1, SupportedKinds: []string{"settings_section", "schema_page", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "扩展设置分节"},
		{SlotID: "extension.plugin.tab", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "插件管理页面扩展页签", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "extension.plugin.item", ContractVersion: 1, SupportedKinds: []string{"card", "panel"}, Layout: LayoutGrid, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "插件管理官方插件条目", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "extension.plugin.bundle.activation", ContractVersion: 1, SupportedKinds: []string{"action", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "插件 Bundle 启用后引导区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "extension.plugin.bundle.config", ContractVersion: 1, SupportedKinds: []string{"settings_section", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "插件 Bundle 详情配置区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "extension.plugin.row.config", ContractVersion: 1, SupportedKinds: []string{"settings_section", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "插件 Bundle 单项配置区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "extension.settings.tab", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "Settings 插件页签", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.header.action", ContractVersion: 1, SupportedKinds: []string{"action", "toolbar_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "聊天顶部操作"},
		{SlotID: "chat.sidebar.panel", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "聊天侧栏面板"},
		{SlotID: "chat.profile_summary.panel", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "聊天用户画像摘要面板"},
		{SlotID: "chat.memory_context.panel", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "聊天记忆上下文面板"},
		{SlotID: "chat.message.action", ContractVersion: 1, SupportedKinds: []string{"message_action", "action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "消息操作按钮"},
		{SlotID: "chat.message.renderer", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "消息渲染器"},
		{SlotID: "chat.message.images", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "助手消息图片组替换区"},
		{SlotID: "chat.message.user.images", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "用户消息图片组替换区"},
		{SlotID: "chat.message.user.text", ContractVersion: 1, SupportedKinds: []string{"message_renderer", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "用户消息纯文本节点渲染区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.approval.detail", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "审批关联工具详情", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.message.command", ContractVersion: 1, SupportedKinds: []string{"message_renderer", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "命令消息按命令名称渲染区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.conversation.flow", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话消息流主体", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.message.markdown.fence", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "Markdown 代码围栏按语言包装层", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.message.reasoning.body", ContractVersion: 1, SupportedKinds: []string{"message_renderer", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "推理正文渲染区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.message.text.interaction", ContractVersion: 1, SupportedKinds: []string{"message_renderer", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "消息文本区域交互扩展", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.message.trajectory.images", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "轨迹记录图片组渲染区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.message.tool.images", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "工具消息持久图片渲染区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.message.tool.cordis", ContractVersion: 1, SupportedKinds: []string{"message_renderer", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "cordis_run 工具卡片包内交互区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.conversation.node", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "会话事件投影节点"},
		{SlotID: "chat.message.custom_renderer", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "扩展消息类型渲染器"},
		{SlotID: "chat.message.attachment_renderer", ContractVersion: 1, SupportedKinds: []string{"message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "消息附件渲染器"},
		{SlotID: "chat.message.badge", ContractVersion: 1, SupportedKinds: []string{"badge", "status_item", "card"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "消息徽章"},
		{SlotID: "chat.composer.action", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框操作"},
		{SlotID: "chat.composer.attachment", ContractVersion: 1, SupportedKinds: []string{"action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框附件"},
		{SlotID: "chat.composer.hint", ContractVersion: 1, SupportedKinds: []string{"status_item", "card", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框提示"},
		{SlotID: "chat.composer.dock.top", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page", "status_item"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框上方停靠区"},
		{SlotID: "chat.composer.dock.bottom", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page", "status_item"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框下方停靠区"},
		{SlotID: "chat.composer.left", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框工具行左侧区"},
		{SlotID: "chat.composer.right", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框工具行右侧区"},
		{SlotID: "chat.composer.overlay", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框卡片内部浮层区"},
		{SlotID: "chat.composer.model", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "输入框模型选择替换区"},
		{SlotID: "chat.composer.model.label", ContractVersion: 1, SupportedKinds: []string{"status_item", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "输入框模型非交互标签替换区"},
		{SlotID: "chat.composer.model.options", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action", "menu_item"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框模型菜单扩展选项区"},
		{SlotID: "chat.composer.permission", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "输入框权限选择替换区"},
		{SlotID: "chat.header.leading", ContractVersion: 1, SupportedKinds: []string{"action", "toolbar_item", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "会话头部全局导航前导区"},
		{SlotID: "chat.header.lineage", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话头部标题替换区"},
		{SlotID: "chat.header.corner", ContractVersion: 1, SupportedKinds: []string{"action", "toolbar_item", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "会话头部最右侧控制区"},
		{SlotID: "chat.conversation.root", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话根容器", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.conversation.session", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "当前会话主体", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.conversation.header", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "会话导航头部容器", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.conversation.session.header", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "当前会话标题、操作与视图导航", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.conversation.subagent.footer", ContractVersion: 1, SupportedKinds: []string{"panel", "action"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "当前会话子代理树底部内容", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.conversation.views", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "当前会话目标视图列表", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.composer.root", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "当前会话输入器替换链", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.composer.bar", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "当前会话输入器主体", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.composer.attachments", ContractVersion: 1, SupportedKinds: []string{"panel", "action"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "输入器草稿附件轨道与投放区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.empty.brand_mark", ContractVersion: 1, SupportedKinds: []string{"icon", "status_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "空会话品牌标记", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.empty.frame", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "空会话滚动主体包装层", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.empty.options", ContractVersion: 1, SupportedKinds: []string{"action", "menu_item", "status_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "新会话紧凑选项", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.empty.workspace", ContractVersion: 1, SupportedKinds: []string{"panel", "action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "新会话工作区选择器", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.empty.workspace.directory_flow", ContractVersion: 1, SupportedKinds: []string{"panel", "action"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "新会话工作区目录选择流程", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.empty.agent_preset", ContractVersion: 1, SupportedKinds: []string{"panel", "action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "新会话代理预设选择器", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "chat.empty_state.card", ContractVersion: 1, SupportedKinds: []string{"card", "schema_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "聊天空状态卡片"},
		{SlotID: "chat.status.item", ContractVersion: 1, SupportedKinds: []string{"status_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "聊天状态项"},
		{SlotID: "character.detail.tab", ContractVersion: 1, SupportedKinds: []string{"schema_page", "web_page"}, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "角色详情标签页"},
		{SlotID: "character.detail.action", ContractVersion: 1, SupportedKinds: []string{"action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "角色详情操作"},
		{SlotID: "character.sidebar.card", ContractVersion: 1, SupportedKinds: []string{"card", "panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "角色侧栏卡片"},
		{SlotID: "memory.page.section", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "记忆核心页扩展分节"},
		{SlotID: "tasks.page.section", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "主动任务核心页扩展分节"},
		{SlotID: "system.status.item", ContractVersion: 1, SupportedKinds: []string{"status_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "系统状态项"},
		{SlotID: "system.settings.section", ContractVersion: 1, SupportedKinds: []string{"settings_section", "schema_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "系统设置分节"},
		{SlotID: "system.diagnostics.tab", ContractVersion: 1, SupportedKinds: []string{"schema_page", "panel"}, Layout: LayoutTabs, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "系统诊断标签页"},
		{SlotID: "ui.portal.overlay", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "全局浮层扩展挂载点"},
		{SlotID: "ui.portal.modal", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutModal, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "全局模态框扩展挂载点"},
		{SlotID: "ui.portal.drawer", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutDrawer, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "全局抽屉扩展挂载点"},
		{SlotID: "ui.portal.toast", ContractVersion: 1, SupportedKinds: []string{"status_item", "panel", "schema_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "全局消息提示扩展挂载点"},
		{SlotID: "ui.sidebar.right.pane.tab", ContractVersion: 1, SupportedKinds: []string{"panel", "schema_page", "web_page"}, Layout: LayoutPanel, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧资源面板内容扩展挂载点"},
		{SlotID: "ui.sidebar.right.pane.tab.title", ContractVersion: 1, SupportedKinds: []string{"panel", "status_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧资源面板标题扩展挂载点"},
		{SlotID: "ui.sidebar.right.pane.document", ContractVersion: 1, SupportedKinds: []string{"panel", "message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧文档正文渲染区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.document.action", ContractVersion: 1, SupportedKinds: []string{"action", "panel"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧文档渲染器专属操作区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.document.actions", ContractVersion: 1, SupportedKinds: []string{"action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧文档文件操作区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.document.pdf", ContractVersion: 1, SupportedKinds: []string{"panel", "message_renderer"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧 Office PDF 呈现区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.document.unpreviewable", ContractVersion: 1, SupportedKinds: []string{"action", "status_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧不可预览文件处理区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.files.actions", ContractVersion: 1, SupportedKinds: []string{"action"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧文件页目录操作区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.guide", ContractVersion: 1, SupportedKinds: []string{"panel"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "右侧 Guide 正文扩展区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.guide.actions", ContractVersion: 1, SupportedKinds: []string{"action", "menu_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧 Guide 更多菜单操作区", Platform: []string{"windows", "macos", "linux", "web"}, AllowRootChild: true},
		{SlotID: "ui.sidebar.right.pane.guide.entry", ContractVersion: 1, SupportedKinds: []string{"panel", "action"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "右侧 Guide 单个提供方卡片", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "ui.sidebar.right.pane.tab.menu.item", ContractVersion: 1, SupportedKinds: []string{"action", "menu_item"}, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "右侧标签页操作菜单扩展区", Platform: []string{"windows", "macos", "linux", "web"}},
		{SlotID: "desktop.command", ContractVersion: 1, SupportedKinds: []string{"desktop_command"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "桌面命令"},
		{SlotID: "desktop.application_menu.item", ContractVersion: 1, SupportedKinds: []string{"menu_item"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "桌面菜单项"},
		{SlotID: "desktop.context_menu.item", ContractVersion: 1, SupportedKinds: []string{"menu_item"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "桌面上下文菜单项"},
		{SlotID: "desktop.tray.item", ContractVersion: 1, SupportedKinds: []string{"menu_item", "action"}, Layout: LayoutHidden, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "托盘菜单项"},
		{SlotID: "desktop.window.page", ContractVersion: 1, SupportedKinds: []string{"schema_page", "web_page"}, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackDefault, Description: "桌面独立窗口页面"},
	}
	parents := map[SlotID]SlotID{
		"provider.app.shell":                           "root",
		"provider.app.navigation":                      "provider.app.shell",
		"provider.app.workspace":                       "provider.app.shell",
		"provider.app.workspace.sidebar":               "provider.app.shell",
		"provider.route.registry":                      "provider.app.workspace",
		"provider.page.provider":                       "provider.app.workspace",
		"provider.conversation.shell":                  "provider.app.workspace",
		"provider.conversation.header":                 "provider.conversation.shell",
		"provider.conversation.messages":               "provider.conversation.shell",
		"provider.conversation.message_renderer":       "provider.conversation.messages",
		"provider.conversation.sidebar":                "provider.conversation.shell",
		"provider.conversation.composer":               "provider.conversation.shell",
		"provider.conversation.overlay":                "provider.conversation.shell",
		"provider.character.shell":                     "provider.app.workspace",
		"provider.character.detail":                    "provider.character.shell",
		"provider.memory.shell":                        "provider.app.workspace",
		"provider.memory.detail":                       "provider.memory.shell",
		"memory.page.section":                          "provider.memory.shell",
		"tasks.page.section":                           "root",
		"provider.settings.shell":                      "provider.app.workspace",
		"provider.settings.section":                    "provider.settings.shell",
		"provider.extension.center":                    "provider.app.workspace",
		"provider.extension.page":                      "provider.extension.center",
		"provider.ui.theme":                            "root",
		"provider.ui.tokens":                           "provider.ui.theme",
		"provider.ui.icons":                            "root",
		"provider.ui.components":                       "root",
		"app.shell.quota_notice":                       "root",
		"app.shell.bottom":                             "root",
		"app.rightbar":                                 "root",
		"app.rightbar.session":                         "app.rightbar",
		"desktop.context_menu.content":                 "root",
		"desktop.context_menu.icon":                    "root",
		"workbench.file.actions":                       "provider.conversation.shell",
		"workbench.review.file.actions":                "provider.conversation.shell",
		"settings.model.provider":                      "root",
		"settings.model.footer":                        "root",
		"settings.shell":                               "root",
		"settings.shell.header":                        "settings.shell",
		"settings.shell.action":                        "settings.shell",
		"settings.shell.close":                         "settings.shell",
		"settings.shell.launcher":                      "settings.shell",
		"settings.shell.launcher.notifications":        "settings.shell.launcher",
		"settings.onboarding":                          "settings.shell",
		"settings.section.icon":                        "settings.shell",
		"settings.shell.trigger":                       "settings.shell",
		"settings.model.sign-in":                       "settings.onboarding",
		"ui.portal.overlay":                            "root",
		"ui.portal.modal":                              "root",
		"ui.portal.drawer":                             "root",
		"ui.portal.toast":                              "root",
		"ui.sidebar.right.pane.tab":                    "root",
		"ui.sidebar.right.pane.tab.title":              "root",
		"ui.sidebar.right.pane.document":               "ui.sidebar.right.pane.tab",
		"ui.sidebar.right.pane.document.action":        "ui.sidebar.right.pane.document",
		"ui.sidebar.right.pane.document.actions":       "ui.sidebar.right.pane.document",
		"ui.sidebar.right.pane.document.pdf":           "ui.sidebar.right.pane.document",
		"ui.sidebar.right.pane.document.unpreviewable": "ui.sidebar.right.pane.document",
		"ui.sidebar.right.pane.files.actions":          "ui.sidebar.right.pane.tab",
		"ui.sidebar.right.pane.guide":                  "ui.sidebar.right.pane.tab",
		"ui.sidebar.right.pane.guide.actions":          "ui.sidebar.right.pane.guide",
		"ui.sidebar.right.pane.guide.entry":            "ui.sidebar.right.pane.guide",
		"ui.sidebar.right.pane.tab.menu.item":          "ui.sidebar.right.pane.tab",
		"chat.header.action":                           "provider.conversation.header",
		"chat.sidebar.panel":                           "provider.conversation.sidebar",
		"chat.profile_summary.panel":                   "provider.conversation.shell",
		"chat.memory_context.panel":                    "provider.conversation.shell",
		"chat.message.action":                          "provider.conversation.messages",
		"chat.message.renderer":                        "provider.conversation.message_renderer",
		"chat.message.images":                          "provider.conversation.messages",
		"chat.message.user.images":                     "provider.conversation.messages",
		"chat.conversation.node":                       "provider.conversation.messages",
		"chat.message.custom_renderer":                 "provider.conversation.messages",
		"chat.message.attachment_renderer":             "provider.conversation.messages",
		"chat.message.badge":                           "provider.conversation.messages",
		"chat.composer.action":                         "provider.conversation.composer",
		"chat.composer.attachment":                     "provider.conversation.composer",
		"chat.composer.hint":                           "provider.conversation.composer",
		"chat.composer.dock.top":                       "provider.conversation.composer",
		"chat.composer.dock.bottom":                    "provider.conversation.composer",
		"chat.composer.left":                           "provider.conversation.composer",
		"chat.composer.right":                          "provider.conversation.composer",
		"chat.composer.overlay":                        "provider.conversation.composer",
		"chat.composer.model":                          "provider.conversation.composer",
		"chat.composer.model.label":                    "chat.composer.model",
		"chat.composer.model.options":                  "chat.composer.model",
		"chat.composer.permission":                     "provider.conversation.composer",
		"chat.header.leading":                          "root",
		"chat.header.lineage":                          "provider.conversation.header",
		"chat.header.corner":                           "provider.conversation.header",
		"chat.empty_state.card":                        "provider.conversation.messages",
		"chat.status.item":                             "provider.conversation.shell",
	}
	dispatchKinds := map[SlotID]SlotDispatchKind{
		"root":                                         DispatchSingle,
		"provider.app.shell":                           DispatchChain,
		"provider.app.navigation":                      DispatchChain,
		"provider.app.workspace":                       DispatchChain,
		"provider.app.workspace.sidebar":               DispatchChain,
		"provider.route.registry":                      DispatchList,
		"provider.page.provider":                       DispatchChain,
		"provider.conversation.shell":                  DispatchChain,
		"provider.conversation.header":                 DispatchChain,
		"provider.conversation.messages":               DispatchChain,
		"provider.conversation.message_renderer":       DispatchChain,
		"provider.conversation.sidebar":                DispatchChain,
		"provider.conversation.composer":               DispatchChain,
		"provider.conversation.overlay":                DispatchList,
		"provider.character.shell":                     DispatchChain,
		"provider.character.detail":                    DispatchChain,
		"provider.memory.shell":                        DispatchChain,
		"provider.memory.detail":                       DispatchChain,
		"provider.settings.shell":                      DispatchChain,
		"provider.settings.section":                    DispatchList,
		"provider.extension.center":                    DispatchChain,
		"provider.extension.page":                      DispatchChain,
		"provider.ui.theme":                            DispatchChain,
		"provider.ui.tokens":                           DispatchChain,
		"provider.ui.icons":                            DispatchChain,
		"provider.ui.components":                       DispatchChain,
		"app.shell.quota_notice":                       DispatchChain,
		"app.workspace.toast":                          DispatchKeyed,
		"app.shell.bottom":                             DispatchSingle,
		"app.rightbar":                                 DispatchSingle,
		"app.rightbar.session":                         DispatchSingle,
		"desktop.context_menu.content":                 DispatchKeyed,
		"desktop.context_menu.icon":                    DispatchKeyed,
		"workbench.file.actions":                       DispatchList,
		"workbench.review.file.actions":                DispatchList,
		"settings.model.provider":                      DispatchKeyed,
		"settings.model.footer":                        DispatchList,
		"extension.plugin.tab":                         DispatchList,
		"extension.plugin.item":                        DispatchList,
		"extension.plugin.bundle.activation":           DispatchKeyed,
		"extension.plugin.bundle.config":               DispatchKeyed,
		"extension.plugin.row.config":                  DispatchKeyed,
		"extension.settings.tab":                       DispatchList,
		"settings.shell":                               DispatchSingle,
		"settings.shell.header":                        DispatchSingle,
		"settings.shell.action":                        DispatchList,
		"settings.shell.close":                         DispatchSingle,
		"settings.shell.launcher":                      DispatchSingle,
		"settings.shell.launcher.notifications":        DispatchSingle,
		"settings.onboarding":                          DispatchList,
		"settings.section.icon":                        DispatchList,
		"settings.shell.trigger":                       DispatchSingle,
		"settings.model.sign-in":                       DispatchSingle,
		"ui.sidebar.right.pane.document":               DispatchKeyed,
		"ui.sidebar.right.pane.document.action":        DispatchKeyed,
		"ui.sidebar.right.pane.document.actions":       DispatchList,
		"ui.sidebar.right.pane.document.pdf":           DispatchKeyed,
		"ui.sidebar.right.pane.document.unpreviewable": DispatchList,
		"ui.sidebar.right.pane.files.actions":          DispatchList,
		"ui.sidebar.right.pane.guide":                  DispatchChain,
		"ui.sidebar.right.pane.guide.actions":          DispatchList,
		"ui.sidebar.right.pane.guide.entry":            DispatchKeyed,
		"ui.sidebar.right.pane.tab.menu.item":          DispatchList,
		"memory.page.section":                          DispatchList,
		"tasks.page.section":                           DispatchList,
		"chat.profile_summary.panel":                   DispatchChain,
		"chat.memory_context.panel":                    DispatchChain,
		"chat.message.renderer":                        DispatchChain,
		"chat.message.images":                          DispatchSingle,
		"chat.message.user.images":                     DispatchSingle,
		"chat.message.user.text":                       DispatchKeyed,
		"chat.approval.detail":                         DispatchSingle,
		"chat.message.command":                         DispatchKeyed,
		"chat.conversation.flow":                       DispatchSingle,
		"chat.message.markdown.fence":                  DispatchHoc,
		"chat.message.reasoning.body":                  DispatchSingle,
		"chat.message.text.interaction":                DispatchKeyed,
		"chat.message.trajectory.images":               DispatchSingle,
		"chat.message.tool.images":                     DispatchSingle,
		"chat.message.tool.cordis":                     DispatchKeyed,
		"chat.message.custom_renderer":                 DispatchChain,
		"chat.message.attachment_renderer":             DispatchChain,
		"chat.composer.model":                          DispatchSingle,
		"chat.composer.model.label":                    DispatchSingle,
		"chat.composer.model.options":                  DispatchList,
		"chat.composer.permission":                     DispatchSingle,
		"chat.header.leading":                          DispatchSingle,
		"chat.header.lineage":                          DispatchSingle,
		"chat.header.corner":                           DispatchSingle,
		"chat.conversation.root":                       DispatchSingle,
		"chat.conversation.session":                    DispatchSingle,
		"chat.conversation.header":                     DispatchSingle,
		"chat.conversation.session.header":             DispatchSingle,
		"chat.conversation.subagent.footer":            DispatchList,
		"chat.conversation.views":                      DispatchList,
		"chat.composer.root":                           DispatchChain,
		"chat.composer.bar":                            DispatchSingle,
		"chat.composer.attachments":                    DispatchSingle,
		"chat.empty.brand_mark":                        DispatchSingle,
		"chat.empty.frame":                             DispatchHoc,
		"chat.empty.options":                           DispatchList,
		"chat.empty.workspace":                         DispatchSingle,
		"chat.empty.workspace.directory_flow":          DispatchSingle,
		"chat.empty.agent_preset":                      DispatchSingle,
		"desktop.command":                              DispatchKeyed,
		"desktop.window.page":                          DispatchKeyed,
		"ui.portal.overlay":                            DispatchList,
		"ui.portal.modal":                              DispatchList,
		"ui.portal.drawer":                             DispatchList,
		"ui.portal.toast":                              DispatchList,
		"ui.sidebar.right.pane.tab":                    DispatchList,
		"ui.sidebar.right.pane.tab.title":              DispatchList,
	}
	sessionScopes := map[SlotID]SlotScope{
		"provider.conversation.shell":                  ScopeSessionMaybe,
		"provider.conversation.header":                 ScopeSessionMaybe,
		"provider.conversation.messages":               ScopeSessionMaybe,
		"provider.conversation.message_renderer":       ScopeSession,
		"provider.conversation.sidebar":                ScopeSessionMaybe,
		"provider.conversation.composer":               ScopeSessionMaybe,
		"provider.conversation.overlay":                ScopeSessionMaybe,
		"chat.header.action":                           ScopeSessionMaybe,
		"chat.sidebar.panel":                           ScopeSessionMaybe,
		"chat.profile_summary.panel":                   ScopeSessionMaybe,
		"chat.memory_context.panel":                    ScopeSessionMaybe,
		"chat.message.action":                          ScopeSession,
		"chat.message.renderer":                        ScopeSession,
		"chat.message.images":                          ScopeSession,
		"chat.message.user.images":                     ScopeSession,
		"chat.message.user.text":                       ScopeSession,
		"chat.approval.detail":                         ScopeSession,
		"chat.message.command":                         ScopeSession,
		"chat.conversation.flow":                       ScopeSession,
		"chat.message.markdown.fence":                  ScopeSession,
		"chat.message.reasoning.body":                  ScopeSession,
		"chat.message.text.interaction":                ScopeSession,
		"chat.message.trajectory.images":               ScopeSession,
		"chat.message.tool.images":                     ScopeSession,
		"chat.message.tool.cordis":                     ScopeSession,
		"chat.conversation.node":                       ScopeSession,
		"chat.message.custom_renderer":                 ScopeSession,
		"chat.message.attachment_renderer":             ScopeSession,
		"chat.message.badge":                           ScopeSession,
		"workbench.file.actions":                       ScopeSession,
		"workbench.review.file.actions":                ScopeSession,
		"chat.composer.action":                         ScopeSessionMaybe,
		"chat.composer.attachment":                     ScopeSessionMaybe,
		"chat.composer.hint":                           ScopeSessionMaybe,
		"chat.composer.dock.top":                       ScopeSession,
		"chat.composer.dock.bottom":                    ScopeSession,
		"chat.composer.left":                           ScopeSession,
		"chat.composer.right":                          ScopeSession,
		"chat.composer.overlay":                        ScopeSession,
		"chat.composer.model":                          ScopeSession,
		"chat.composer.model.label":                    ScopeSession,
		"chat.composer.model.options":                  ScopeSession,
		"chat.composer.permission":                     ScopeSession,
		"chat.header.leading":                          ScopeRoot,
		"chat.header.lineage":                          ScopeSession,
		"chat.header.corner":                           ScopeSession,
		"chat.conversation.root":                       ScopeSessionMaybe,
		"chat.conversation.session":                    ScopeSession,
		"chat.conversation.header":                     ScopeSessionMaybe,
		"chat.conversation.session.header":             ScopeSession,
		"chat.conversation.subagent.footer":            ScopeSession,
		"chat.conversation.views":                      ScopeSession,
		"chat.composer.root":                           ScopeSession,
		"chat.composer.bar":                            ScopeSessionMaybe,
		"chat.composer.attachments":                    ScopeSessionMaybe,
		"chat.empty.brand_mark":                        ScopeRoot,
		"chat.empty.frame":                             ScopeRoot,
		"chat.empty.options":                           ScopeSession,
		"chat.empty.workspace":                         ScopeRoot,
		"chat.empty.workspace.directory_flow":          ScopeRoot,
		"chat.empty.agent_preset":                      ScopeSessionMaybe,
		"chat.empty_state.card":                        ScopeSessionMaybe,
		"chat.status.item":                             ScopeSessionMaybe,
		"app.rightbar.session":                         ScopeSession,
		"ui.sidebar.right.pane.tab":                    ScopeSessionMaybe,
		"ui.sidebar.right.pane.tab.title":              ScopeSessionMaybe,
		"ui.sidebar.right.pane.document":               ScopeSession,
		"ui.sidebar.right.pane.document.action":        ScopeSession,
		"ui.sidebar.right.pane.document.actions":       ScopeSession,
		"ui.sidebar.right.pane.document.pdf":           ScopeSession,
		"ui.sidebar.right.pane.document.unpreviewable": ScopeSession,
		"ui.sidebar.right.pane.files.actions":          ScopeSession,
		"ui.sidebar.right.pane.guide":                  ScopeSession,
		"ui.sidebar.right.pane.guide.actions":          ScopeRoot,
		"ui.sidebar.right.pane.guide.entry":            ScopeSession,
		"ui.sidebar.right.pane.tab.menu.item":          ScopeSession,
	}
	for _, def := range slots {
		def.Scope = ScopeRoot
		if scope, ok := sessionScopes[def.SlotID]; ok {
			def.Scope = scope
		}
		if kind, ok := dispatchKinds[def.SlotID]; ok {
			def.DispatchKind = kind
		} else {
			def.DispatchKind = DispatchList
		}
		if parent, ok := parents[def.SlotID]; ok {
			def.ParentSlotID = parent
		} else if def.SlotID != "root" {
			def.ParentSlotID = "root"
		}
	}
	return slots
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
	SlotID            SlotID                 `json:"slotId"`
	ContractVersion   int                    `json:"contractVersion"`
	SupportedKinds    []string               `json:"supportedKinds,omitempty"`
	DispatchKind      SlotDispatchKind       `json:"kind,omitempty"`
	Layout            SlotLayout             `json:"layout"`
	FallbackPolicy    FallbackPolicy         `json:"fallbackPolicy"`
	Description       string                 `json:"description,omitempty"`
	Platform          []string               `json:"platforms,omitempty"`
	OrderingPolicy    string                 `json:"orderingPolicy,omitempty"`
	FailurePolicy     string                 `json:"failurePolicy,omitempty"`
	OwnerExtension    string                 `json:"ownerExtension,omitempty"`
	ParentSlotID      SlotID                 `json:"parentSlotId,omitempty"`
	Dynamic           bool                   `json:"dynamic,omitempty"`
	Scope             SlotScope              `json:"scope"`
	DeclarationEpoch  uint64                 `json:"declarationEpoch"`
	PerformanceBudget PerformanceBudget      `json:"performanceBudget"`
	Contributions     []*ContributionSummary `json:"contributions"`
	GeneratedAt       time.Time              `json:"generatedAt"`
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
			SlotID:            slotDef.SlotID,
			ContractVersion:   slotDef.ContractVersion,
			SupportedKinds:    append([]string(nil), slotDef.SupportedKinds...),
			DispatchKind:      slotDef.DispatchKind,
			Layout:            slotDef.Layout,
			FallbackPolicy:    slotDef.FallbackPolicy,
			Description:       slotDef.Description,
			Platform:          append([]string(nil), slotDef.Platform...),
			OrderingPolicy:    slotDef.OrderingPolicy,
			FailurePolicy:     slotDef.FailurePolicy,
			OwnerExtension:    slotDef.OwnerExtension,
			ParentSlotID:      slotDef.ParentSlotID,
			Dynamic:           slotDef.Dynamic,
			Scope:             slotDef.Scope,
			DeclarationEpoch:  slotDef.DeclarationEpoch,
			PerformanceBudget: slotDef.PerformanceBudget,
			Contributions:     contribs,
			GeneratedAt:       time.Now().UTC(),
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
	ErrInvalidSlotScope         = errors.New("extension_slots: invalid slot scope")
	ErrInvalidSlotDispatchKind  = errors.New("extension_slots: invalid slot dispatch kind")
	ErrSlotScopeEscape          = errors.New("extension_slots: session child slot cannot escape to root scope")
)
