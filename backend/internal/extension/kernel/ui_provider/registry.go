package ui_provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Capability string
type Mode string
type EntryType string
type Placement string
type ProfileScopeKind string

const (
	ModeReplace Mode = "replace"
	ModeCompose Mode = "compose"
	ModeAugment Mode = "augment"
)

const (
	EntryBuiltinNative  EntryType = "builtin_native"
	EntryDeclarative    EntryType = "declarative"
	EntryWebModule      EntryType = "web_module"
	EntrySchemaRenderer EntryType = "schema_renderer"
	EntryWebRestricted  EntryType = "web_restricted"
	EntryWebIsolated    EntryType = "web_isolated"
)

const (
	PlacementAny    Placement = "any"
	PlacementCloud  Placement = "cloud"
	PlacementDevice Placement = "device"
	PlacementHybrid Placement = "hybrid"
)

const (
	ProfileScopeGlobal         ProfileScopeKind = "global"
	ProfileScopeUser           ProfileScopeKind = "user"
	ProfileScopePlatform       ProfileScopeKind = "platform"
	ProfileScopeDevice         ProfileScopeKind = "device"
	ProfileScopeDevicePlatform ProfileScopeKind = "device_platform"
	ProfileScopeRuntime        ProfileScopeKind = "runtime"
)

const (
	CapabilityAppShell                    Capability = "app.shell"
	CapabilityAppNavigation               Capability = "app.navigation"
	CapabilityAppWorkspace                Capability = "app.workspace"
	CapabilityRouteRegistry               Capability = "route.registry"
	CapabilityPageProvider                Capability = "page.provider"
	CapabilityConversationShell           Capability = "conversation.shell"
	CapabilityConversationHeader          Capability = "conversation.header"
	CapabilityConversationMessages        Capability = "conversation.messages"
	CapabilityConversationMessageRenderer Capability = "conversation.message_renderer"
	CapabilityConversationSidebar         Capability = "conversation.sidebar"
	CapabilityConversationComposer        Capability = "conversation.composer"
	CapabilityConversationOverlay         Capability = "conversation.overlay"
	CapabilityCharacterShell              Capability = "character.shell"
	CapabilityCharacterDetail             Capability = "character.detail"
	CapabilityMemoryShell                 Capability = "memory.shell"
	CapabilityMemoryDetail                Capability = "memory.detail"
	CapabilitySettingsShell               Capability = "settings.shell"
	CapabilitySettingsSection             Capability = "settings.section"
	CapabilityExtensionCenter             Capability = "extension.center"
	CapabilityExtensionPage               Capability = "extension.page"
	CapabilityTheme                       Capability = "ui.theme"
	CapabilityTokens                      Capability = "ui.tokens"
	CapabilityIcons                       Capability = "ui.icons"
	CapabilityComponents                  Capability = "ui.components"
)

var knownCapabilities = map[Capability]struct{}{
	CapabilityAppShell: {}, CapabilityAppNavigation: {}, CapabilityAppWorkspace: {},
	CapabilityRouteRegistry: {}, CapabilityPageProvider: {},
	CapabilityConversationShell: {}, CapabilityConversationHeader: {}, CapabilityConversationMessages: {},
	CapabilityConversationMessageRenderer: {}, CapabilityConversationSidebar: {}, CapabilityConversationComposer: {}, CapabilityConversationOverlay: {},
	CapabilityCharacterShell: {}, CapabilityCharacterDetail: {}, CapabilityMemoryShell: {}, CapabilityMemoryDetail: {},
	CapabilitySettingsShell: {}, CapabilitySettingsSection: {}, CapabilityExtensionCenter: {}, CapabilityExtensionPage: {},
	CapabilityTheme: {}, CapabilityTokens: {}, CapabilityIcons: {}, CapabilityComponents: {},
}

func (c Capability) Valid() bool { _, ok := knownCapabilities[c]; return ok }
func (m Mode) Valid() bool       { return m == ModeReplace || m == ModeCompose || m == ModeAugment }
func (e EntryType) Valid() bool {
	switch e {
	case EntryBuiltinNative, EntryDeclarative, EntryWebModule, EntrySchemaRenderer, EntryWebRestricted, EntryWebIsolated:
		return true
	default:
		return false
	}
}
func (p Placement) Valid() bool {
	return p == PlacementAny || p == PlacementCloud || p == PlacementDevice || p == PlacementHybrid
}

// DeviceRequirements mirrors manifest module deviceRequirements so UI resolution
// can reject a provider before the host tries to render an incompatible entry.
type DeviceRequirements struct {
	Platforms         []string `json:"platforms,omitempty"`
	Architectures     []string `json:"architectures,omitempty"`
	MinAppVersion     string   `json:"minAppVersion,omitempty"`
	MinRuntimeVersion string   `json:"minRuntimeVersion,omitempty"`
	RequiredFeatures  []string `json:"requiredFeatures,omitempty"`
}

type Entry struct {
	ContributionID string    `json:"contributionId,omitempty"`
	Type           EntryType `json:"type"`
	Path           string    `json:"path,omitempty"`
	SchemaPath     string    `json:"schemaPath,omitempty"`
	ExportName     string    `json:"exportName,omitempty"`
	ContentHash    string    `json:"contentHash,omitempty"`
}

type ProviderDefinition struct {
	ProviderID         string              `json:"providerId"`
	ExtensionID        string              `json:"extensionId"`
	ModuleID           string              `json:"moduleId,omitempty"`
	Capability         Capability          `json:"capability"`
	Mode               Mode                `json:"mode"`
	Priority           int                 `json:"priority,omitempty"`
	Platforms          []string            `json:"platforms,omitempty"`
	Entries            map[string]Entry    `json:"entries"`
	FallbackProviderID string              `json:"fallbackProviderId,omitempty"`
	TrustLevel         string              `json:"trustLevel,omitempty"`
	Permissions        []string            `json:"permissions,omitempty"`
	Placement          Placement           `json:"placement,omitempty"`
	DeviceRequirements *DeviceRequirements `json:"deviceRequirements,omitempty"`
	Generation         int64               `json:"generation,omitempty"`
	Enabled            bool                `json:"enabled"`
	Builtin            bool                `json:"builtin,omitempty"`
	Metadata           map[string]any      `json:"metadata,omitempty"`
}

func (d *ProviderDefinition) Normalize() {
	d.ProviderID = strings.TrimSpace(d.ProviderID)
	d.ExtensionID = strings.TrimSpace(d.ExtensionID)
	d.ModuleID = strings.TrimSpace(d.ModuleID)
	d.Capability = Capability(strings.TrimSpace(string(d.Capability)))
	if d.Mode == "" {
		d.Mode = ModeReplace
	}
	if d.Placement == "" {
		if d.Builtin {
			d.Placement = PlacementAny
		} else {
			d.Placement = PlacementCloud
		}
	}
	if d.Entries == nil {
		d.Entries = map[string]Entry{}
	}
	if !d.Enabled && d.Builtin {
		d.Enabled = true
	}
}

func requiresTrustedRootProvider(capability Capability) bool {
	switch capability {
	case CapabilityAppShell, CapabilityAppNavigation, CapabilityAppWorkspace, CapabilityRouteRegistry, CapabilityPageProvider:
		return true
	default:
		return false
	}
}
func supportsDeclarativeEntry(capability Capability) bool {
	switch capability {
	case CapabilityAppNavigation, CapabilityRouteRegistry, CapabilityTheme, CapabilityTokens, CapabilityIcons, CapabilityComponents:
		return true
	default:
		return false
	}
}
func trustedRootLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "system", "official", "trusted", "user_trusted":
		return true
	default:
		return false
	}
}

func (d ProviderDefinition) Validate() error {
	if d.ProviderID == "" {
		return errors.New("ui_provider: providerId required")
	}
	if strings.ContainsAny(d.ProviderID, "/\\") || strings.Contains(d.ProviderID, "..") {
		return errors.New("ui_provider: providerId must be a safe path segment")
	}
	if d.ExtensionID == "" {
		return errors.New("ui_provider: extensionId required")
	}
	if !d.Capability.Valid() {
		return fmt.Errorf("ui_provider: unsupported capability %q", d.Capability)
	}
	if !d.Mode.Valid() {
		return fmt.Errorf("ui_provider: invalid mode %q", d.Mode)
	}
	if !d.Placement.Valid() {
		return fmt.Errorf("ui_provider: invalid placement %q", d.Placement)
	}
	if !d.Builtin && requiresTrustedRootProvider(d.Capability) && !trustedRootLevel(d.TrustLevel) {
		return fmt.Errorf("ui_provider: capability %s requires an explicitly trusted publisher", d.Capability)
	}
	if len(d.Entries) == 0 {
		return errors.New("ui_provider: entries required")
	}
	for platform, entry := range d.Entries {
		if strings.TrimSpace(platform) == "" {
			return errors.New("ui_provider: empty platform entry")
		}
		if !entry.Type.Valid() {
			return fmt.Errorf("ui_provider: invalid entry type %q for %s", entry.Type, platform)
		}
		switch entry.Type {
		case EntryBuiltinNative:
			if !d.Builtin {
				return fmt.Errorf("ui_provider: builtin_native is reserved for built-in providers (%s)", platform)
			}
		case EntryDeclarative:
			if !supportsDeclarativeEntry(d.Capability) {
				return fmt.Errorf("ui_provider: declarative entry is not supported for capability %s", d.Capability)
			}
		case EntryWebModule:
			platformKey := strings.ToLower(strings.TrimSpace(platform))
			if platformKey == "android" || platformKey == "ios" || platformKey == "mobile" {
				return fmt.Errorf("ui_provider: web_module cannot target Flutter AOT platform %s; use schema_renderer or sandbox web", platform)
			}
			if strings.TrimSpace(entry.Path) == "" {
				return fmt.Errorf("ui_provider: web_module path required for %s", platform)
			}
		case EntrySchemaRenderer:
			if strings.TrimSpace(entry.ContributionID) == "" {
				return fmt.Errorf("ui_provider: schema_renderer contributionId required for %s", platform)
			}
		case EntryWebRestricted, EntryWebIsolated:
			if strings.TrimSpace(entry.ContributionID) == "" {
				return fmt.Errorf("ui_provider: sandbox web entry contributionId required for %s", platform)
			}
		}
	}
	if err := validateProviderMetadata(d); err != nil {
		return err
	}
	return nil
}

func validateProviderMetadata(d ProviderDefinition) error {
	if d.Metadata == nil {
		return nil
	}
	objectField := func(name string) error {
		value, ok := d.Metadata[name]
		if !ok || value == nil {
			return nil
		}
		switch value.(type) {
		case map[string]any, map[string]string:
			return nil
		default:
			return fmt.Errorf("ui_provider: metadata.%s must be an object", name)
		}
	}
	arrayField := func(name string) ([]any, error) {
		value, ok := d.Metadata[name]
		if !ok || value == nil {
			return nil, nil
		}
		rows, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("ui_provider: metadata.%s must be an array", name)
		}
		return rows, nil
	}

	if d.Capability == CapabilityRouteRegistry {
		routes, err := arrayField("routes")
		if err != nil {
			return err
		}
		for _, raw := range routes {
			row, ok := raw.(map[string]any)
			if !ok {
				return errors.New("ui_provider: route.registry metadata.routes entries must be objects")
			}
			field := func(name string) string {
				value, exists := row[name]
				if !exists || value == nil {
					return ""
				}
				return strings.TrimSpace(fmt.Sprint(value))
			}
			if field("id") == "" || field("path") == "" || field("providerId") == "" {
				return errors.New("ui_provider: route.registry routes require id, path and providerId")
			}
		}
	}
	if d.Capability == CapabilityAppNavigation || d.Capability == CapabilityRouteRegistry {
		items, err := arrayField("navigationItems")
		if err != nil {
			return err
		}
		for _, raw := range items {
			row, ok := raw.(map[string]any)
			if !ok {
				return errors.New("ui_provider: navigationItems entries must be objects")
			}
			field := func(name string) string {
				value, exists := row[name]
				if !exists || value == nil {
					return ""
				}
				return strings.TrimSpace(fmt.Sprint(value))
			}
			if field("id") == "" || field("label") == "" || !strings.HasPrefix(field("route"), "/") {
				return errors.New("ui_provider: navigationItems require id, label and absolute route")
			}
		}
	}
	if d.Capability == CapabilityConversationMessageRenderer {
		for _, name := range []string{"messageTypes", "roles", "mimeTypes", "extensionTypes"} {
			if _, err := arrayField(name); err != nil {
				return err
			}
		}
	}
	if d.Capability == CapabilityComponents {
		if err := objectField("componentVariants"); err != nil {
			return err
		}
	}
	if d.Capability == CapabilityIcons {
		for _, name := range []string{"iconAliases", "iconExports", "iconGlyphs"} {
			if err := objectField(name); err != nil {
				return err
			}
		}
	}
	return nil
}

type ProfileScope struct {
	UserID         string `json:"userId,omitempty"`
	DeviceID       string `json:"deviceId,omitempty"`
	Platform       string `json:"platform,omitempty"`
	RuntimeProfile string `json:"runtimeProfile,omitempty"`
}

func (s ProfileScope) Normalize() ProfileScope {
	return ProfileScope{
		UserID: strings.TrimSpace(s.UserID), DeviceID: strings.TrimSpace(s.DeviceID),
		Platform: strings.ToLower(strings.TrimSpace(s.Platform)), RuntimeProfile: strings.ToLower(strings.TrimSpace(s.RuntimeProfile)),
	}
}
func globalProfileScope() ProfileScope { return ProfileScope{} }
func (s ProfileScope) Key() string {
	s = s.Normalize()
	// Scope keys are persisted as a single primary key. Escape each component so
	// an externally supplied identifier cannot smuggle the field delimiters into
	// the key and collide with another user's/device's profile scope.
	return "u=" + url.QueryEscape(s.UserID) + "|d=" + url.QueryEscape(s.DeviceID) + "|p=" + url.QueryEscape(s.Platform) + "|r=" + url.QueryEscape(s.RuntimeProfile)
}
func (s ProfileScope) LayerKeys() []string {
	s = s.Normalize()
	keys := []string{globalProfileScope().Key()}
	add := func(v ProfileScope) {
		key := v.Normalize().Key()
		for _, existing := range keys {
			if existing == key {
				return
			}
		}
		keys = append(keys, key)
	}
	if s.UserID == "" {
		return keys
	}
	add(ProfileScope{UserID: s.UserID})
	if s.Platform != "" {
		add(ProfileScope{UserID: s.UserID, Platform: s.Platform})
	}
	if s.DeviceID != "" {
		add(ProfileScope{UserID: s.UserID, DeviceID: s.DeviceID})
	}
	if s.DeviceID != "" && s.Platform != "" {
		add(ProfileScope{UserID: s.UserID, DeviceID: s.DeviceID, Platform: s.Platform})
	}
	if s.RuntimeProfile != "" {
		// Runtime is an explicit user-level override layer. Keep it independent
		// from device/platform so the same runtime profile behaves consistently
		// across every device using it. Device/platform layers still apply before
		// this final runtime override.
		add(ProfileScope{UserID: s.UserID, RuntimeProfile: s.RuntimeProfile})
	}
	return keys
}

func (s ProfileScope) ForKind(kind ProfileScopeKind) (ProfileScope, error) {
	s = s.Normalize()
	switch kind {
	case "", ProfileScopeUser:
		if s.UserID == "" {
			return ProfileScope{}, errors.New("ui_provider: user scope requires authenticated user")
		}
		return ProfileScope{UserID: s.UserID}, nil
	case ProfileScopeGlobal:
		return globalProfileScope(), nil
	case ProfileScopePlatform:
		if s.UserID == "" || s.Platform == "" {
			return ProfileScope{}, errors.New("ui_provider: platform scope requires user and platform")
		}
		return ProfileScope{UserID: s.UserID, Platform: s.Platform}, nil
	case ProfileScopeDevice:
		if s.UserID == "" || s.DeviceID == "" {
			return ProfileScope{}, errors.New("ui_provider: device scope requires user and device")
		}
		return ProfileScope{UserID: s.UserID, DeviceID: s.DeviceID}, nil
	case ProfileScopeDevicePlatform:
		if s.UserID == "" || s.DeviceID == "" || s.Platform == "" {
			return ProfileScope{}, errors.New("ui_provider: device_platform scope requires user, device and platform")
		}
		return ProfileScope{UserID: s.UserID, DeviceID: s.DeviceID, Platform: s.Platform}, nil
	case ProfileScopeRuntime:
		if s.UserID == "" || s.RuntimeProfile == "" {
			return ProfileScope{}, errors.New("ui_provider: runtime scope requires user and runtime profile")
		}
		return ProfileScope{UserID: s.UserID, RuntimeProfile: s.RuntimeProfile}, nil
	default:
		return ProfileScope{}, fmt.Errorf("ui_provider: invalid profile scope %q", kind)
	}
}

type Profile struct {
	ProfileID  string                `json:"profileId"`
	Name       string                `json:"name"`
	Selections map[Capability]string `json:"selections"`
	Scope      ProfileScope          `json:"scope,omitempty"`
	Revision   int64                 `json:"revision,omitempty"`
	UpdatedAt  int64                 `json:"updatedAt"`
}

type ResolveContext struct {
	UserID             string   `json:"userId,omitempty"`
	DeviceID           string   `json:"deviceId,omitempty"`
	Platform           string   `json:"platform"`
	Architecture       string   `json:"architecture,omitempty"`
	RuntimeProfile     string   `json:"runtimeProfile,omitempty"`
	AppVersion         string   `json:"appVersion,omitempty"`
	RuntimeVersion     string   `json:"runtimeVersion,omitempty"`
	DeviceOnline       bool     `json:"deviceOnline,omitempty"`
	LocalRuntime       bool     `json:"localRuntime,omitempty"`
	DeviceCapabilities []string `json:"deviceCapabilities,omitempty"`
}

func (c ResolveContext) Normalize() ResolveContext {
	c.UserID = strings.TrimSpace(c.UserID)
	c.DeviceID = strings.TrimSpace(c.DeviceID)
	c.Platform = strings.ToLower(strings.TrimSpace(c.Platform))
	if c.Platform == "" {
		c.Platform = "web"
	}
	c.Architecture = strings.ToLower(strings.TrimSpace(c.Architecture))
	c.RuntimeProfile = strings.ToLower(strings.TrimSpace(c.RuntimeProfile))
	c.AppVersion = strings.TrimSpace(c.AppVersion)
	c.RuntimeVersion = strings.TrimSpace(c.RuntimeVersion)
	caps := make([]string, 0, len(c.DeviceCapabilities))
	seen := map[string]struct{}{}
	for _, raw := range c.DeviceCapabilities {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v != "" {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				caps = append(caps, v)
			}
		}
	}
	sort.Strings(caps)
	c.DeviceCapabilities = caps
	return c
}
func (c ResolveContext) ProfileScope() ProfileScope {
	c = c.Normalize()
	return ProfileScope{UserID: c.UserID, DeviceID: c.DeviceID, Platform: c.Platform, RuntimeProfile: c.RuntimeProfile}
}

type Resolution struct {
	Capability    Capability          `json:"capability"`
	Platform      string              `json:"platform"`
	Context       ResolveContext      `json:"context"`
	Provider      *ProviderDefinition `json:"provider,omitempty"`
	FallbackChain []string            `json:"fallbackChain,omitempty"`
	Reason        string              `json:"reason,omitempty"`
}

type Snapshot struct {
	Providers     []*ProviderDefinition              `json:"providers"`
	Profile       Profile                            `json:"profile"`
	ProfileLayers []Profile                          `json:"profileLayers,omitempty"`
	Resolved      map[Capability]*ProviderDefinition `json:"resolved"`
	Context       ResolveContext                     `json:"context"`
	Version       int                                `json:"version"`
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]*ProviderDefinition
	profile   Profile // legacy/global fallback when no persistent store is attached
	store     ProfileStore
	version   uint64
}

func NewRegistry() *Registry {
	return &Registry{providers: map[string]*ProviderDefinition{}, profile: Profile{ProfileID: "default", Name: "Default", Selections: map[Capability]string{}, Scope: globalProfileScope()}, version: 1}
}
func NewRegistryWithBuiltins() *Registry {
	r := NewRegistry()
	for _, d := range BuiltinProviders() {
		_ = r.Register(d)
	}
	return r
}

func (r *Registry) AttachStore(ctx context.Context, store ProfileStore) error {
	if store == nil {
		return nil
	}
	p, ok, err := store.Load(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.store = store
	if ok {
		if p.Selections == nil {
			p.Selections = map[Capability]string{}
		}
		r.profile = p
	}
	r.version++
	r.mu.Unlock()
	return nil
}
func (r *Registry) Version() uint64 { r.mu.RLock(); defer r.mu.RUnlock(); return r.version }
func (r *Registry) Register(def ProviderDefinition) error {
	def.Normalize()
	if err := def.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneProvider(&def)
	r.providers[def.ProviderID] = cp
	r.version++
	return nil
}
func (r *Registry) Unregister(providerID string) {
	r.mu.Lock()
	p, ok := r.providers[providerID]
	if ok && p.Builtin {
		r.mu.Unlock()
		return
	}
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.providers, providerID)
	for cap, selected := range r.profile.Selections {
		if selected == providerID {
			delete(r.profile.Selections, cap)
		}
	}
	r.version++
	r.mu.Unlock()
}
func (r *Registry) UnregisterExtension(extensionID string) {
	r.mu.Lock()
	removed := map[string]bool{}
	for id, p := range r.providers {
		if p.ExtensionID == extensionID && !p.Builtin {
			delete(r.providers, id)
			removed[id] = true
		}
	}
	for cap, selected := range r.profile.Selections {
		if removed[selected] {
			delete(r.profile.Selections, cap)
		}
	}
	if len(removed) > 0 {
		r.version++
	}
	r.mu.Unlock()
}
func (r *Registry) SetEnabled(providerID string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.providers[providerID]
	if !ok {
		return fmt.Errorf("ui_provider: provider %s not found", providerID)
	}
	if p.Builtin && !enabled {
		return errors.New("ui_provider: builtin provider cannot be disabled")
	}
	if p.Enabled == enabled {
		return nil
	}
	cp := cloneProvider(p)
	cp.Enabled = enabled
	r.providers[providerID] = cp
	r.version++
	return nil
}
func (r *Registry) List() []*ProviderDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ProviderDefinition, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, cloneProvider(p))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Capability == out[j].Capability {
			if out[i].Priority == out[j].Priority {
				return out[i].ProviderID < out[j].ProviderID
			}
			return out[i].Priority > out[j].Priority
		}
		return out[i].Capability < out[j].Capability
	})
	return out
}
func (r *Registry) Get(providerID string) (*ProviderDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	if !ok {
		return nil, false
	}
	return cloneProvider(p), true
}

func supportsPlatform(p *ProviderDefinition, platform string) bool {
	if p == nil || !p.Enabled {
		return false
	}
	if len(p.Platforms) > 0 {
		found := false
		for _, v := range p.Platforms {
			if strings.EqualFold(v, platform) || v == "*" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if _, ok := p.Entries[platform]; ok {
		return true
	}
	if _, ok := p.Entries["mobile"]; ok && (platform == "android" || platform == "ios") {
		return true
	}
	if _, ok := p.Entries["desktop"]; ok && (platform == "windows" || platform == "macos" || platform == "linux") {
		return true
	}
	_, ok := p.Entries["*"]
	return ok
}

func cloneProvider(p *ProviderDefinition) *ProviderDefinition {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Platforms = append([]string(nil), p.Platforms...)
	cp.Permissions = append([]string(nil), p.Permissions...)
	if p.Entries != nil {
		cp.Entries = make(map[string]Entry, len(p.Entries))
		for k, v := range p.Entries {
			cp.Entries[k] = v
		}
	}
	if p.Metadata != nil {
		cp.Metadata = make(map[string]any, len(p.Metadata))
		for k, v := range p.Metadata {
			cp.Metadata[k] = v
		}
	}
	if p.DeviceRequirements != nil {
		req := *p.DeviceRequirements
		req.Platforms = append([]string(nil), p.DeviceRequirements.Platforms...)
		req.Architectures = append([]string(nil), p.DeviceRequirements.Architectures...)
		req.RequiredFeatures = append([]string(nil), p.DeviceRequirements.RequiredFeatures...)
		cp.DeviceRequirements = &req
	}
	return &cp
}

func providerCompatible(p *ProviderDefinition, ctx ResolveContext) (bool, string) {
	ctx = ctx.Normalize()
	if !supportsPlatform(p, ctx.Platform) {
		return false, "platform_unsupported"
	}
	if p.Builtin || p.Placement == PlacementAny {
		return true, ""
	}
	if p.Placement == PlacementDevice {
		// A local/device-agent core is itself the execution device. Cloud-core
		// resolution, however, must only select device providers for an online,
		// authenticated target device.
		if !ctx.LocalRuntime {
			if ctx.DeviceID == "" {
				return false, "device_identity_missing"
			}
			if !ctx.DeviceOnline {
				return false, "device_offline"
			}
		}
	}
	if req := p.DeviceRequirements; req != nil {
		if len(req.Platforms) > 0 && !containsFold(req.Platforms, ctx.Platform) {
			return false, "device_platform_mismatch"
		}
		if len(req.Architectures) > 0 {
			if ctx.Architecture == "" {
				return false, "device_architecture_missing"
			}
			if !containsFold(req.Architectures, ctx.Architecture) {
				return false, "device_architecture_mismatch"
			}
		}
		if req.MinAppVersion != "" {
			if ctx.AppVersion == "" {
				return false, "app_version_missing"
			}
			if compareVersion(ctx.AppVersion, req.MinAppVersion) < 0 {
				return false, "app_version_too_old"
			}
		}
		if req.MinRuntimeVersion != "" {
			if ctx.RuntimeVersion == "" {
				return false, "runtime_version_missing"
			}
			if compareVersion(ctx.RuntimeVersion, req.MinRuntimeVersion) < 0 {
				return false, "runtime_version_too_old"
			}
		}
		for _, feature := range req.RequiredFeatures {
			if !containsFold(ctx.DeviceCapabilities, feature) {
				return false, "device_feature_missing:" + feature
			}
		}
	}
	return true, ""
}
func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}
func compareVersion(a, b string) int {
	parse := func(raw string) []int {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
		raw = strings.SplitN(raw, "-", 2)[0]
		parts := strings.Split(raw, ".")
		out := make([]int, 4)
		for i := 0; i < len(parts) && i < len(out); i++ {
			n, _ := strconv.Atoi(parts[i])
			out[i] = n
		}
		return out
	}
	av, bv := parse(a), parse(b)
	for i := range av {
		if av[i] < bv[i] {
			return -1
		}
		if av[i] > bv[i] {
			return 1
		}
	}
	return 0
}

func (r *Registry) effectiveProfile(ctx context.Context, rc ResolveContext) (Profile, []Profile, error) {
	r.mu.RLock()
	store := r.store
	legacy := r.profile
	r.mu.RUnlock()
	if store == nil {
		legacy.Selections = cloneSelections(legacy.Selections)
		legacy.Scope = rc.ProfileScope()
		return legacy, nil, nil
	}
	layers, err := store.LoadLayers(ctx, rc.ProfileScope())
	if err != nil {
		return Profile{}, nil, err
	}
	effective := Profile{ProfileID: "default", Name: "Default", Selections: map[Capability]string{}, Scope: rc.ProfileScope()}
	var userRevision int64
	for _, layer := range layers {
		if layer.ProfileID != "" {
			effective.ProfileID = layer.ProfileID
		}
		if layer.Name != "" {
			effective.Name = layer.Name
		}
		for cap, id := range layer.Selections {
			if strings.TrimSpace(id) == "" {
				delete(effective.Selections, cap)
			} else {
				effective.Selections[cap] = id
			}
		}
		if layer.UpdatedAt > effective.UpdatedAt {
			effective.UpdatedAt = layer.UpdatedAt
		}
		if layer.Scope.UserID == rc.UserID && layer.Scope.DeviceID == "" && layer.Scope.Platform == "" && layer.Scope.RuntimeProfile == "" {
			userRevision = layer.Revision
		}
	}
	effective.Revision = userRevision
	return effective, layers, nil
}

func (r *Registry) resolveFallbackLocked(providerID string, capability Capability, rc ResolveContext, visited map[string]struct{}, chain *[]string) *ProviderDefinition {
	id := strings.TrimSpace(providerID)
	for id != "" {
		if _, seen := visited[id]; seen {
			*chain = append(*chain, id+":cycle")
			return nil
		}
		visited[id] = struct{}{}
		p := r.providers[id]
		if p == nil {
			*chain = append(*chain, id+":missing")
			return nil
		}
		if p.Capability != capability {
			*chain = append(*chain, id+":capability_mismatch")
			return nil
		}
		if ok, reason := providerCompatible(p, rc); ok {
			return cloneProvider(p)
		} else {
			*chain = append(*chain, id+":"+reason)
		}
		id = strings.TrimSpace(p.FallbackProviderID)
	}
	return nil
}

func (r *Registry) ResolveWithContext(ctx context.Context, capability Capability, rc ResolveContext) Resolution {
	rc = rc.Normalize()
	res := Resolution{Capability: capability, Platform: rc.Platform, Context: rc}
	profile, _, err := r.effectiveProfile(ctx, rc)
	if err != nil {
		res.Reason = "profile_load_failed"
		return res
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	selected := strings.TrimSpace(profile.Selections[capability])
	if selected != "" {
		if p := r.resolveFallbackLocked(selected, capability, rc, map[string]struct{}{}, &res.FallbackChain); p != nil {
			res.Provider = p
			if p.ProviderID == selected {
				res.Reason = "profile_selection"
			} else {
				res.Reason = "profile_fallback"
			}
			return res
		}
	}
	builtinCandidates, otherCandidates := []*ProviderDefinition{}, []*ProviderDefinition{}
	for _, p := range r.providers {
		if p.Capability != capability {
			continue
		}
		if ok, _ := providerCompatible(p, rc); !ok {
			continue
		}
		if p.Builtin {
			builtinCandidates = append(builtinCandidates, p)
		} else {
			otherCandidates = append(otherCandidates, p)
		}
	}
	sortProviders := func(items []*ProviderDefinition) {
		sort.Slice(items, func(i, j int) bool {
			if items[i].Priority == items[j].Priority {
				return items[i].ProviderID < items[j].ProviderID
			}
			return items[i].Priority > items[j].Priority
		})
	}
	sortProviders(builtinCandidates)
	if len(builtinCandidates) > 0 {
		res.Provider = cloneProvider(builtinCandidates[0])
		res.Reason = "builtin_default"
		return res
	}
	sortProviders(otherCandidates)
	if len(otherCandidates) > 0 {
		res.Provider = cloneProvider(otherCandidates[0])
		res.Reason = "priority_no_builtin"
		return res
	}
	res.Reason = "no_provider"
	return res
}
func (r *Registry) Resolve(capability Capability, platform string) Resolution {
	return r.ResolveWithContext(context.Background(), capability, ResolveContext{Platform: platform})
}

func validateSelections(providers map[string]*ProviderDefinition, selections map[Capability]string) error {
	for capability, id := range selections {
		p, ok := providers[id]
		if !ok {
			return fmt.Errorf("ui_provider: selected provider %s not found", id)
		}
		if p.Capability != capability {
			return fmt.Errorf("ui_provider: provider %s does not provide %s", id, capability)
		}
		if !p.Enabled {
			return fmt.Errorf("ui_provider: provider %s is disabled", id)
		}
	}
	return nil
}

func (r *Registry) SetProfileForContext(ctx context.Context, rc ResolveContext, kind ProfileScopeKind, profile Profile, expectedRevision int64) (Profile, error) {
	rc = rc.Normalize()
	scope, err := rc.ProfileScope().ForKind(kind)
	if err != nil {
		return Profile{}, err
	}
	if profile.Selections == nil {
		profile.Selections = map[Capability]string{}
	}
	r.mu.RLock()
	if err := validateSelections(r.providers, profile.Selections); err != nil {
		r.mu.RUnlock()
		return Profile{}, err
	}
	store := r.store
	r.mu.RUnlock()
	profile.Scope = scope
	if profile.ProfileID == "" {
		profile.ProfileID = "default"
	}
	if profile.Name == "" {
		profile.Name = profile.ProfileID
	}
	if store == nil {
		r.mu.Lock()
		profile.Revision = r.profile.Revision + 1
		r.profile = profile
		r.version++
		r.mu.Unlock()
		return profile, nil
	}
	saved, err := store.SaveScoped(ctx, profile, expectedRevision)
	if err != nil {
		return Profile{}, err
	}
	r.mu.Lock()
	r.version++
	r.mu.Unlock()
	return saved, nil
}

func (r *Registry) DeleteProfileScope(ctx context.Context, rc ResolveContext, kind ProfileScopeKind, expectedRevision int64) error {
	scope, err := rc.Normalize().ProfileScope().ForKind(kind)
	if err != nil {
		return err
	}
	r.mu.RLock()
	store := r.store
	r.mu.RUnlock()
	if store == nil {
		return nil
	}
	if err := store.DeleteScope(ctx, scope, expectedRevision); err != nil {
		return err
	}
	r.mu.Lock()
	r.version++
	r.mu.Unlock()
	return nil
}

func (r *Registry) SetProfile(profile Profile) error {
	_, err := r.SetProfileForContext(context.Background(), ResolveContext{UserID: "default", Platform: "web"}, ProfileScopeGlobal, profile, -1)
	return err
}
func cloneSelections(in map[Capability]string) map[Capability]string {
	out := make(map[Capability]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (r *Registry) ProfileForContext(ctx context.Context, rc ResolveContext) (Profile, []Profile, error) {
	return r.effectiveProfile(ctx, rc.Normalize())
}

// ProfileForScope returns the exact editable layer for the requested scope.
// A missing layer is represented as an empty revision-0 profile rather than by
// copying the effective profile; this preserves inheritance when a user edits
// only one device/platform override.
func (r *Registry) ProfileForScope(ctx context.Context, rc ResolveContext, kind ProfileScopeKind) (Profile, bool, error) {
	rc = rc.Normalize()
	scope, err := rc.ProfileScope().ForKind(kind)
	if err != nil {
		return Profile{}, false, err
	}
	r.mu.RLock()
	store := r.store
	legacy := r.profile
	r.mu.RUnlock()
	if store == nil {
		if kind == ProfileScopeGlobal || kind == "" || kind == ProfileScopeUser {
			legacy.Scope = scope
			legacy.Selections = cloneSelections(legacy.Selections)
			return legacy, true, nil
		}
		return Profile{ProfileID: "default", Name: "Default", Selections: map[Capability]string{}, Scope: scope}, false, nil
	}
	p, ok, err := store.LoadExact(ctx, scope)
	if err != nil {
		return Profile{}, false, err
	}
	if !ok {
		return Profile{ProfileID: "default", Name: "Default", Selections: map[Capability]string{}, Scope: scope, Revision: 0}, false, nil
	}
	return p, true, nil
}
func (r *Registry) Profile() Profile {
	p, _, err := r.effectiveProfile(context.Background(), ResolveContext{Platform: "web"})
	if err != nil {
		return Profile{ProfileID: "default", Name: "Default", Selections: map[Capability]string{}}
	}
	return p
}
func (r *Registry) SnapshotWithContext(ctx context.Context, rc ResolveContext) Snapshot {
	rc = rc.Normalize()
	providers := r.List()
	profile, layers, _ := r.effectiveProfile(ctx, rc)
	resolved := map[Capability]*ProviderDefinition{}
	for cap := range knownCapabilities {
		rr := r.ResolveWithContext(ctx, cap, rc)
		if rr.Provider != nil {
			resolved[cap] = rr.Provider
		}
	}
	return Snapshot{Providers: providers, Profile: profile, ProfileLayers: layers, Resolved: resolved, Context: rc, Version: int(r.Version())}
}
func (r *Registry) Snapshot(platform string) Snapshot {
	return r.SnapshotWithContext(context.Background(), ResolveContext{Platform: platform})
}
