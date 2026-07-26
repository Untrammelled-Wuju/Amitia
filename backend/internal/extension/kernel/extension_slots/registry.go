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
	MultiplicitySingle          SlotMultiplicity = "single"
	MultiplicityMultiple        SlotMultiplicity = "multiple"
	MultiplicityOrderedMultiple SlotMultiplicity = "ordered_multiple"
	MultiplicityReplaceableSingle SlotMultiplicity = "replaceable_single"
	MultiplicityExclusive       SlotMultiplicity = "exclusive"
)

type SlotLayout string

const (
	LayoutInline   SlotLayout = "inline"
	LayoutStack    SlotLayout = "stack"
	LayoutRow      SlotLayout = "row"
	LayoutGrid     SlotLayout = "grid"
	LayoutTabs     SlotLayout = "tabs"
	LayoutPanel    SlotLayout = "panel"
	LayoutDrawer   SlotLayout = "drawer"
	LayoutModal    SlotLayout = "modal"
	LayoutHidden   SlotLayout = "hidden"
)

type FallbackPolicy string

const (
	FallbackNone     FallbackPolicy = "none"
	FallbackSkeleton FallbackPolicy = "skeleton"
	FallbackEmpty    FallbackPolicy = "empty"
	FallbackDefault  FallbackPolicy = "default"
)

type SlotDefinition struct {
	SlotID            SlotID              `json:"slotId"`
	ContractVersion   int                 `json:"contractVersion"`
	SupportedKinds    []string            `json:"supportedKinds"`
	Multiplicity      SlotMultiplicity    `json:"multiplicity"`
	Layout            SlotLayout          `json:"layout"`
	ContextSchema     json.RawMessage     `json:"contextSchema,omitempty"`
	PerformanceBudget PerformanceBudget   `json:"performanceBudget"`
	FallbackPolicy    FallbackPolicy      `json:"fallbackPolicy"`
	Description       string              `json:"description,omitempty"`
	Platform          []string            `json:"platform,omitempty"`
	OrderingPolicy    string              `json:"orderingPolicy,omitempty"`
	FailurePolicy     string              `json:"failurePolicy,omitempty"`
}

type PerformanceBudget struct {
	FirstPaint    time.Duration `json:"firstPaint"`
	BundleSize    int64         `json:"bundleSize"`
	MemoryBytes   int64         `json:"memoryBytes"`
	MessageRate   int           `json:"messageRate"`
	UpdateFrequency time.Duration `json:"updateFrequency"`
}

type SlotRegistry struct {
	mu    sync.RWMutex
	slots map[SlotID]*SlotDefinition
}

func NewSlotRegistry() *SlotRegistry {
	return &SlotRegistry{slots: make(map[SlotID]*SlotDefinition)}
}

func (r *SlotRegistry) Register(def *SlotDefinition) error {
	if def == nil || def.SlotID == "" {
		return ErrInvalidSlotDefinition
	}
	if def.ContractVersion <= 0 {
		return ErrInvalidContractVersion
	}
	if len(def.SupportedKinds) == 0 {
		return ErrNoSupportedKinds
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.slots[def.SlotID]; exists {
		return fmt.Errorf("%w: %s", ErrSlotExists, def.SlotID)
	}
	r.slots[def.SlotID] = def
	return nil
}

func (r *SlotRegistry) Get(slotID SlotID) (*SlotDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, exists := r.slots[slotID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrSlotNotFound, slotID)
	}
	return def, nil
}

func (r *SlotRegistry) List() []*SlotDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SlotDefinition, 0, len(r.slots))
	for _, def := range r.slots {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool {
		return string(out[i].SlotID) < string(out[j].SlotID)
	})
	return out
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
		{SlotID: "chat.composer.action", ContractVersion: 1, SupportedKinds: []string{"composer_action", "action"}, Multiplicity: MultiplicityOrderedMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框操作"},
		{SlotID: "chat.composer.attachment", ContractVersion: 1, SupportedKinds: []string{"action"}, Multiplicity: MultiplicityMultiple, Layout: LayoutInline, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackNone, Description: "输入框附件"},
		{SlotID: "chat.empty_state.card", ContractVersion: 1, SupportedKinds: []string{"card", "schema_page"}, Multiplicity: MultiplicityMultiple, Layout: LayoutStack, PerformanceBudget: defaultBudget, FallbackPolicy: FallbackEmpty, Description: "聊天空状态卡片"},
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
	ContributionID    string          `json:"contributionId"`
	ExtensionID       string          `json:"extensionId"`
	ModuleID          string          `json:"moduleId"`
	Kind              string          `json:"kind"`
	SlotID            SlotID          `json:"slotId"`
	ContractVersion   int             `json:"contractVersion"`
	Generation        int64           `json:"generation"`
	Title             string          `json:"title"`
	Description       string          `json:"description,omitempty"`
	Icon              string          `json:"icon,omitempty"`
	Ordering          int             `json:"ordering"`
	Visible           bool            `json:"visible"`
	Effective         bool            `json:"effective"`
	Enabled           bool            `json:"enabled"`
	RuntimeReady      bool            `json:"runtimeReady"`
	Permissions       []string        `json:"permissions,omitempty"`
	Sandbox           string          `json:"sandbox,omitempty"`
	EntryPath         string          `json:"entryPath,omitempty"`
	SchemaPath        string          `json:"schemaPath,omitempty"`
	Actions           []ActionSummary `json:"actions,omitempty"`
	HiddenReason      string          `json:"hiddenReason,omitempty"`
}

type ActionSummary struct {
	ActionID string `json:"actionId"`
	Title    string `json:"title"`
	Icon     string `json:"icon,omitempty"`
	RiskLevel string `json:"riskLevel,omitempty"`
}

type SlotSnapshot struct {
	SlotID         SlotID                 `json:"slotId"`
	ContractVersion int                   `json:"contractVersion"`
	Layout         SlotLayout             `json:"layout"`
	Multiplicity   SlotMultiplicity       `json:"multiplicity"`
	FallbackPolicy FallbackPolicy         `json:"fallbackPolicy"`
	Contributions  []*ContributionSummary `json:"contributions"`
	GeneratedAt    time.Time              `json:"generatedAt"`
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
	registry   *SlotRegistry
	resolver   SlotResolver
	mu         sync.RWMutex
	cache      *UIContributionSnapshot
	cacheTTL   time.Duration
	cacheTime  time.Time
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
	ErrInvalidSlotDefinition = errors.New("extension_slots: invalid slot definition")
	ErrInvalidContractVersion = errors.New("extension_slots: invalid contract version")
	ErrNoSupportedKinds      = errors.New("extension_slots: no supported kinds")
	ErrSlotExists            = errors.New("extension_slots: slot exists")
	ErrSlotNotFound          = errors.New("extension_slots: slot not found")
)
