package ui_provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Capability string

type Mode string

type EntryType string

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

func (c Capability) Valid() bool {
	_, ok := knownCapabilities[c]
	return ok
}

func (m Mode) Valid() bool { return m == ModeReplace || m == ModeCompose || m == ModeAugment }
func (e EntryType) Valid() bool {
	switch e {
	case EntryBuiltinNative, EntryDeclarative, EntryWebModule, EntrySchemaRenderer, EntryWebRestricted, EntryWebIsolated:
		return true
	default:
		return false
	}
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
	ProviderID         string           `json:"providerId"`
	ExtensionID        string           `json:"extensionId"`
	ModuleID           string           `json:"moduleId,omitempty"`
	Capability         Capability       `json:"capability"`
	Mode               Mode             `json:"mode"`
	Priority           int              `json:"priority,omitempty"`
	Platforms          []string         `json:"platforms,omitempty"`
	Entries            map[string]Entry `json:"entries"`
	FallbackProviderID string           `json:"fallbackProviderId,omitempty"`
	TrustLevel         string           `json:"trustLevel,omitempty"`
	Permissions        []string         `json:"permissions,omitempty"`
	Generation         int64            `json:"generation,omitempty"`
	Enabled            bool             `json:"enabled"`
	Builtin            bool             `json:"builtin,omitempty"`
	Metadata           map[string]any   `json:"metadata,omitempty"`
}

func (d *ProviderDefinition) Normalize() {
	d.ProviderID = strings.TrimSpace(d.ProviderID)
	d.ExtensionID = strings.TrimSpace(d.ExtensionID)
	d.ModuleID = strings.TrimSpace(d.ModuleID)
	d.Capability = Capability(strings.TrimSpace(string(d.Capability)))
	if d.Mode == "" {
		d.Mode = ModeReplace
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
	return nil
}

type Profile struct {
	ProfileID  string                `json:"profileId"`
	Name       string                `json:"name"`
	Selections map[Capability]string `json:"selections"`
	UpdatedAt  int64                 `json:"updatedAt"`
}

type Resolution struct {
	Capability    Capability          `json:"capability"`
	Platform      string              `json:"platform"`
	Provider      *ProviderDefinition `json:"provider,omitempty"`
	FallbackChain []string            `json:"fallbackChain,omitempty"`
	Reason        string              `json:"reason,omitempty"`
}

type Snapshot struct {
	Providers []*ProviderDefinition              `json:"providers"`
	Profile   Profile                            `json:"profile"`
	Resolved  map[Capability]*ProviderDefinition `json:"resolved"`
	Version   int                                `json:"version"`
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]*ProviderDefinition
	profile   Profile
	store     ProfileStore
	version   uint64
}

func NewRegistry() *Registry {
	return &Registry{providers: map[string]*ProviderDefinition{}, profile: Profile{ProfileID: "default", Name: "Default", Selections: map[Capability]string{}}, version: 1}
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

func (r *Registry) Version() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

func (r *Registry) Register(def ProviderDefinition) error {
	def.Normalize()
	if err := def.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := def
	r.providers[def.ProviderID] = &cp
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
	profileChanged := false
	for cap, selected := range r.profile.Selections {
		if selected == providerID {
			delete(r.profile.Selections, cap)
			profileChanged = true
		}
	}
	r.version++
	profile := r.profile
	profile.Selections = cloneSelections(r.profile.Selections)
	store := r.store
	r.mu.Unlock()
	if profileChanged && store != nil {
		_ = store.Save(context.Background(), profile)
	}
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
	profileChanged := false
	for cap, selected := range r.profile.Selections {
		if removed[selected] {
			delete(r.profile.Selections, cap)
			profileChanged = true
		}
	}
	if len(removed) > 0 {
		r.version++
	}
	profile := r.profile
	profile.Selections = cloneSelections(r.profile.Selections)
	store := r.store
	r.mu.Unlock()
	if profileChanged && store != nil {
		_ = store.Save(context.Background(), profile)
	}
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
	cp := *p
	cp.Enabled = enabled
	r.providers[providerID] = &cp
	r.version++
	return nil
}

func (r *Registry) List() []*ProviderDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ProviderDefinition, 0, len(r.providers))
	for _, p := range r.providers {
		cp := *p
		out = append(out, &cp)
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
	cp := *p
	return &cp, true
}

func supportsPlatform(p *ProviderDefinition, platform string) bool {
	if p == nil || !p.Enabled {
		return false
	}
	if len(p.Platforms) > 0 {
		found := false
		for _, v := range p.Platforms {
			if v == platform || v == "*" {
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
	if _, ok := p.Entries["*"]; ok {
		return true
	}
	return false
}

func cloneProvider(p *ProviderDefinition) *ProviderDefinition {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Platforms != nil {
		cp.Platforms = append([]string(nil), p.Platforms...)
	}
	if p.Permissions != nil {
		cp.Permissions = append([]string(nil), p.Permissions...)
	}
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
	return &cp
}

func (r *Registry) resolveFallbackLocked(providerID string, capability Capability, platform string, visited map[string]struct{}, chain *[]string) *ProviderDefinition {
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
		if supportsPlatform(p, platform) {
			return cloneProvider(p)
		}
		*chain = append(*chain, id+":unsupported")
		id = strings.TrimSpace(p.FallbackProviderID)
	}
	return nil
}

func (r *Registry) Resolve(capability Capability, platform string) Resolution {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := Resolution{Capability: capability, Platform: platform}
	selected := strings.TrimSpace(r.profile.Selections[capability])
	if selected != "" {
		if p := r.resolveFallbackLocked(selected, capability, platform, map[string]struct{}{}, &res.FallbackChain); p != nil {
			res.Provider = p
			if p.ProviderID == selected {
				res.Reason = "profile_selection"
			} else {
				res.Reason = "profile_fallback"
			}
			return res
		}
	}

	// An installed replacement provider must never take over the application simply
	// because it advertises a larger priority. When the profile has no explicit
	// selection, prefer Amitia's built-in provider. This keeps installation and
	// activation separate and guarantees a deterministic recovery path.
	builtinCandidates := make([]*ProviderDefinition, 0)
	otherCandidates := make([]*ProviderDefinition, 0)
	for _, p := range r.providers {
		if p.Capability != capability || !supportsPlatform(p, platform) {
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

func (r *Registry) SetProfile(profile Profile) error {
	if profile.ProfileID == "" {
		profile.ProfileID = "default"
	}
	if profile.Name == "" {
		profile.Name = profile.ProfileID
	}
	if profile.Selections == nil {
		profile.Selections = map[Capability]string{}
	}
	r.mu.RLock()
	for capability, id := range profile.Selections {
		p, ok := r.providers[id]
		if !ok {
			r.mu.RUnlock()
			return fmt.Errorf("ui_provider: selected provider %s not found", id)
		}
		if p.Capability != capability {
			r.mu.RUnlock()
			return fmt.Errorf("ui_provider: provider %s does not provide %s", id, capability)
		}
		if !p.Enabled {
			r.mu.RUnlock()
			return fmt.Errorf("ui_provider: provider %s is disabled", id)
		}
	}
	store := r.store
	r.mu.RUnlock()
	if store != nil {
		if err := store.Save(context.Background(), profile); err != nil {
			return fmt.Errorf("ui_provider: persist profile: %w", err)
		}
	}
	r.mu.Lock()
	r.profile = profile
	r.version++
	r.mu.Unlock()
	return nil
}

func cloneSelections(in map[Capability]string) map[Capability]string {
	out := make(map[Capability]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (r *Registry) Profile() Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := r.profile
	out.Selections = cloneSelections(r.profile.Selections)
	return out
}

func (r *Registry) Snapshot(platform string) Snapshot {
	providers := r.List()
	profile := r.Profile()
	resolved := map[Capability]*ProviderDefinition{}
	for cap := range knownCapabilities {
		rr := r.Resolve(cap, platform)
		if rr.Provider != nil {
			resolved[cap] = rr.Provider
		}
	}
	return Snapshot{Providers: providers, Profile: profile, Resolved: resolved, Version: int(r.Version())}
}
