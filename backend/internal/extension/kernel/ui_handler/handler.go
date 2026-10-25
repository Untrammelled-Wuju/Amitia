package ui_handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/extension_slots"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type DialogResolver interface {
	ResolveDialog(dialogID string, result string) bool
	ResolveDialogWithHost(dialogID string, hostClientID string, hostSessionID string, result string) bool
}

type ClientRuntimeCommandResolver interface {
	ResolveClientRuntimeCommand(commandID string, result map[string]interface{}, commandErr string) bool
}

type ScopedClientRuntimeCommandResolver interface {
	ResolveClientRuntimeCommandWithHost(commandID, hostClientID, hostSessionID string, result map[string]interface{}, commandErr string) bool
	ClientRuntimeSessionState(spaceID, conversationID string) map[string]interface{}
	AcknowledgeClientRuntimeSession(spaceID, conversationID string, revision int64) (map[string]interface{}, error)
}

type ClipboardResolver interface {
	ResolveClipboardRequest(requestID string, text string) bool
	FailClipboardRequest(requestID string, err error) bool
	ResolveClipboardRequestWithHost(requestID string, hostClientID string, hostSessionID string, text string) bool
	FailClipboardRequestWithHost(requestID string, hostClientID string, hostSessionID string, err error) bool
}

type ResourceLinkResolver interface {
	ResolveResourceLink(token string) (extensionID string, scope string, fullPath string, err error)
}

type HTTPHandler struct {
	uiHost                *ui_contribution.UIHost
	slotRegistry          *extension_slots.SlotRegistry
	pageHost              *extension_page_host.PageHost
	providerRegistry      *ui_provider.Registry
	providerContext       func(*http.Request, string) ui_provider.ResolveContext
	providerBroadcaster   func(string, map[string]interface{})
	authorizer            *permission.UISessionAuthorizer
	schemaLookup          func(extensionID, contributionID string) (json.RawMessage, bool)
	dialogResolver        DialogResolver
	clientRuntimeResolver ClientRuntimeCommandResolver
	clipboardResolver     ClipboardResolver
	hostRegistry          *host_registry.HostRegistry
	extRoot               string
	resourceLinks         ResourceLinkResolver
}

func NewHTTPHandler(
	uiHost *ui_contribution.UIHost,
	slotRegistry *extension_slots.SlotRegistry,
	pageHost *extension_page_host.PageHost,
) *HTTPHandler {
	return &HTTPHandler{
		uiHost:       uiHost,
		slotRegistry: slotRegistry,
		pageHost:     pageHost,
	}
}

func (h *HTTPHandler) SetProviderRegistry(registry *ui_provider.Registry) {
	h.providerRegistry = registry
}

func (h *HTTPHandler) SetProviderContextResolver(fn func(*http.Request, string) ui_provider.ResolveContext) {
	h.providerContext = fn
}

func (h *HTTPHandler) SetProviderChangeBroadcaster(fn func(string, map[string]interface{})) {
	h.providerBroadcaster = fn
}

func (h *HTTPHandler) resolveProviderContext(r *http.Request, platform string) ui_provider.ResolveContext {
	if h.providerContext != nil {
		return h.providerContext(r, platform).Normalize()
	}
	return ui_provider.ResolveContext{Platform: platform}.Normalize()
}

func (h *HTTPHandler) SetAuthorizer(a *permission.UISessionAuthorizer) {
	h.authorizer = a
}

func (h *HTTPHandler) SetExtensionRoot(root string) {
	h.extRoot = root
}

func (h *HTTPHandler) SetSchemaLookup(fn func(extensionID, contributionID string) (json.RawMessage, bool)) {
	h.schemaLookup = fn
}

func (h *HTTPHandler) SetDialogResolver(r DialogResolver) {
	h.dialogResolver = r
}

func (h *HTTPHandler) SetClientRuntimeCommandResolver(r ClientRuntimeCommandResolver) {
	h.clientRuntimeResolver = r
}

func (h *HTTPHandler) SetClipboardResolver(r ClipboardResolver) {
	h.clipboardResolver = r
}

func (h *HTTPHandler) SetHostRegistry(registry *host_registry.HostRegistry) {
	h.hostRegistry = registry
}

func (h *HTTPHandler) SetResourceLinkResolver(resolver ResourceLinkResolver) {
	h.resourceLinks = resolver
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/extensions/ui/slots", h.handleSlots)
	mux.HandleFunc("/api/extensions/ui/contributions", h.handleContributions)
	mux.HandleFunc("/api/extensions/ui/snapshot", h.handleSnapshot)
	mux.HandleFunc("/api/extensions/ui/providers", h.handleProviders)
	mux.HandleFunc("/api/extensions/ui/providers/resolve", h.handleProviderResolve)
	mux.HandleFunc("/api/extensions/ui/provider-module/", h.handleProviderModuleResource)
	mux.HandleFunc("/api/extensions/ui/profile", h.handleProfile)
	mux.HandleFunc("/api/extensions/ui/sessions", h.handleSessionsCollection)
	mux.HandleFunc("/api/extensions/ui/sessions/", h.handleSessionItem)
	mux.HandleFunc("/api/extensions/ui/page-sessions/", h.handlePageSession)
	mux.HandleFunc("/api/extensions/ui/open-page", h.handleOpenPage)
	mux.HandleFunc("/api/extensions/ui/by-extension", h.handleExtensionContributions)
	mux.HandleFunc("/api/extensions/", h.handleExtensionScoped)
	mux.HandleFunc("/api/extension/schema/", h.handleSchema)
	mux.HandleFunc("/api/extensions/ui/schema/", h.handleSchema)
	mux.HandleFunc("/api/extension/resources/", h.handleResourceLink)
	mux.HandleFunc("/api/extension/action/", h.handleAction)
	mux.HandleFunc("/api/extension/composer/action/", h.handleComposerAction)
	mux.HandleFunc("/api/extensions/ui/dialog-response", h.handleDialogResponse)
	mux.HandleFunc("/api/extensions/ui/client-runtime-response", h.handleClientRuntimeResponse)
	mux.HandleFunc("/api/extensions/ui/client-runtime-state", h.handleClientRuntimeState)
	mux.HandleFunc("/api/extensions/ui/client-runtime-session-ack", h.handleClientRuntimeSessionAck)
	mux.HandleFunc("/api/extensions/ui/clipboard-response", h.handleClipboardResponse)
	mux.HandleFunc("/api/extensions/ui/host-session", h.handleHostSession)
	mux.HandleFunc("/api/extensions/ui/host-session/heartbeat", h.handleHostSessionHeartbeat)
	mux.HandleFunc("/api/extensions/ui/host-session/disconnect", h.handleHostSessionDisconnect)
}

func (h *HTTPHandler) RegisterPublicModuleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ui/provider-module/", h.handleProviderModuleResource)
	mux.HandleFunc("/api/public/extensions/ui/provider-module/", h.handleProviderModuleResource)
}

func (h *HTTPHandler) handleResourceLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.resourceLinks == nil {
		writeError(w, http.StatusServiceUnavailable, "resource_links_unavailable", "resource link resolver not configured")
		return
	}
	const prefix = "/api/extension/resources/"
	token := strings.TrimPrefix(r.URL.Path, prefix)
	if token == "" || token == r.URL.Path {
		writeError(w, http.StatusNotFound, "resource_not_found", "resource token required")
		return
	}
	_, _, fullPath, err := h.resourceLinks.ResolveResourceLink(token)
	if err != nil {
		writeError(w, http.StatusNotFound, "resource_not_found", err.Error())
		return
	}
	file, err := os.Open(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "resource_not_found", err.Error())
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "resource_not_found", "resource not found")
		return
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(fullPath)))
	if contentType == "" {
		var header [512]byte
		n, _ := file.Read(header[:])
		contentType = http.DetectContentType(header[:n])
		_, _ = file.Seek(0, 0)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if strings.HasPrefix(contentType, "image/") || strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/") || contentType == "application/pdf" {
		w.Header().Set("Content-Disposition", "inline")
	} else {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

func (h *HTTPHandler) handleSlots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slots := h.slotRegistry.List()
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

func (h *HTTPHandler) handleContributions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	list := h.uiHost.ListAll()
	writeJSON(w, http.StatusOK, map[string]any{"contributions": list})
}

func (h *HTTPHandler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	slots := h.slotRegistry.List()
	type slotSnapshotEntry struct {
		SlotID            string                                      `json:"slotId"`
		ContractVersion   int                                         `json:"contractVersion"`
		SupportedKinds    []string                                    `json:"supportedKinds"`
		Multiplicity      extension_slots.SlotMultiplicity            `json:"multiplicity"`
		DispatchKind      extension_slots.SlotDispatchKind            `json:"kind,omitempty"`
		Layout            extension_slots.SlotLayout                  `json:"layout"`
		FallbackPolicy    extension_slots.FallbackPolicy              `json:"fallbackPolicy"`
		PerformanceBudget extension_slots.PerformanceBudget           `json:"performanceBudget"`
		Description       string                                      `json:"description,omitempty"`
		Platform          []string                                    `json:"platform,omitempty"`
		OrderingPolicy    string                                      `json:"orderingPolicy,omitempty"`
		FailurePolicy     string                                      `json:"failurePolicy,omitempty"`
		OwnerExtension    string                                      `json:"ownerExtension,omitempty"`
		ParentSlotID      extension_slots.SlotID                      `json:"parentSlotId,omitempty"`
		Dynamic           bool                                        `json:"dynamic,omitempty"`
		Scope             extension_slots.SlotScope                   `json:"scope"`
		DeclarationEpoch  uint64                                      `json:"declarationEpoch"`
		Contributions     []*ui_contribution.UIContributionDefinition `json:"contributions"`
	}
	out := make([]slotSnapshotEntry, 0, len(slots))
	for _, s := range slots {
		contribs := h.uiHost.ListActiveBySlot(string(s.SlotID))
		out = append(out, slotSnapshotEntry{
			SlotID:            string(s.SlotID),
			ContractVersion:   s.ContractVersion,
			SupportedKinds:    append([]string(nil), s.SupportedKinds...),
			Multiplicity:      s.Multiplicity,
			DispatchKind:      s.DispatchKind,
			Layout:            s.Layout,
			FallbackPolicy:    s.FallbackPolicy,
			PerformanceBudget: s.PerformanceBudget,
			Description:       s.Description,
			Platform:          append([]string(nil), s.Platform...),
			OrderingPolicy:    s.OrderingPolicy,
			FailurePolicy:     s.FailurePolicy,
			OwnerExtension:    s.OwnerExtension,
			ParentSlotID:      s.ParentSlotID,
			Dynamic:           s.Dynamic,
			Scope:             s.Scope,
			DeclarationEpoch:  s.DeclarationEpoch,
			Contributions:     contribs,
		})
	}
	payload := map[string]any{
		"slots":                out,
		"contributions":        h.uiHost.ListActiveAll(),
		"pendingContributions": h.uiHost.ListPending(),
		"timestamp":            time.Now().UTC(),
	}
	if h.providerRegistry != nil {
		platform := strings.TrimSpace(r.URL.Query().Get("platform"))
		if platform == "" {
			platform = "web"
		}
		resolveContext := h.resolveProviderContext(r, platform)
		providerSnapshot := h.providerRegistry.SnapshotWithContext(r.Context(), resolveContext)
		payload["providers"] = signProviderModuleCapabilityList(providerSnapshot.Providers)
		payload["profile"] = providerSnapshot.Profile
		payload["profileLayers"] = providerSnapshot.ProfileLayers
		payload["resolved"] = signResolvedProviderModules(providerSnapshot.Resolved)
		payload["providerContext"] = providerSnapshot.Context
		payload["providerVersion"] = providerSnapshot.Version
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *HTTPHandler) handleProviders(w http.ResponseWriter, r *http.Request) {
	if h.providerRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "ui_provider_registry_unavailable", "ui provider registry unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"providers": signProviderModuleCapabilityList(h.providerRegistry.List())})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (h *HTTPHandler) handleProviderResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.providerRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "ui_provider_registry_unavailable", "ui provider registry unavailable")
		return
	}
	capability := ui_provider.Capability(strings.TrimSpace(r.URL.Query().Get("capability")))
	if !capability.Valid() {
		writeError(w, http.StatusBadRequest, "invalid_capability", "invalid UI provider capability")
		return
	}
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	if platform == "" {
		platform = "web"
	}
	resolveContext := h.resolveProviderContext(r, platform)
	resolution := h.providerRegistry.ResolveWithContext(r.Context(), capability, resolveContext)
	resolution.Provider = signProviderModuleCapabilities(resolution.Provider)
	writeJSON(w, http.StatusOK, resolution)
}

func trustedModuleProvider(p *ui_provider.ProviderDefinition) bool {
	if p == nil || !p.Enabled || p.Builtin {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(p.TrustLevel)) {
	case "system", "official", "trusted", "user_trusted", "verified":
		return true
	default:
		return false
	}
}

func (h *HTTPHandler) handleProviderModuleResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.providerRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "ui_provider_registry_unavailable", "ui provider registry unavailable")
		return
	}
	const marker = "/ui/provider-module/"
	markerIndex := strings.Index(r.URL.Path, marker)
	if markerIndex < 0 {
		writeError(w, http.StatusNotFound, "provider_module_not_found", "provider module route not found")
		return
	}
	rest := r.URL.Path[markerIndex+len(marker):]
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusNotFound, "provider_module_not_found", "provider id and resource path required")
		return
	}
	providerID := parts[0]
	resourcePath := parts[1]
	provider, ok := h.providerRegistry.Get(providerID)
	if !ok || !trustedModuleProvider(provider) {
		writeError(w, http.StatusForbidden, "provider_module_forbidden", "trusted web module provider required")
		return
	}
	if !validProviderModuleCapability(providerID, r.Header.Get("X-Amitia-Module-Access")) {
		writeError(w, http.StatusForbidden, "provider_module_capability_invalid", "provider module capability is invalid or expired")
		return
	}
	basePath := h.resolveExtensionBasePath(provider.ExtensionID)
	if basePath == "" {
		writeError(w, http.StatusNotFound, "extension_path_not_found", "extension bundle path not found")
		return
	}
	cleanPath, err := sanitizeUIModulePath(basePath, resourcePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_path", err.Error())
		return
	}
	fullPath := filepath.Join(basePath, cleanPath)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "provider_module_resource_not_found", "provider module resource not found")
		return
	}
	mime := uiModuleMIME(cleanPath)
	if mime == "" {
		writeError(w, http.StatusUnsupportedMediaType, "mime_not_allowed", "mime type not allowed")
		return
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read_failed", err.Error())
		return
	}
	// The entry module may carry an explicit digest in the provider manifest.
	// Verify it at the serving boundary so a trusted provider cannot silently
	// execute a module that drifted after package verification. Relative support
	// modules remain covered by the extension package content-tree integrity.
	for _, entry := range provider.Entries {
		if entry.Type != ui_provider.EntryWebModule {
			continue
		}
		expected := ""
		if strings.TrimPrefix(entry.Path, "/") == cleanPath {
			expected = entry.ContentHash
		} else if strings.TrimSpace(entry.StylePath) != "" && strings.TrimPrefix(entry.StylePath, "/") == cleanPath {
			expected = entry.StyleHash
		}
		if strings.TrimSpace(expected) == "" {
			continue
		}
		sum := sha256.Sum256(content)
		actual := "sha256:" + hex.EncodeToString(sum[:])
		if strings.ToLower(strings.TrimSpace(expected)) != strings.ToLower(actual) {
			writeError(w, http.StatusConflict, "provider_module_integrity_mismatch", "provider module content hash mismatch")
			return
		}
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=3600, immutable")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *HTTPHandler) handleProfile(w http.ResponseWriter, r *http.Request) {
	if h.providerRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "ui_provider_registry_unavailable", "ui provider registry unavailable")
		return
	}
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	if platform == "" {
		platform = "web"
	}
	resolveContext := h.resolveProviderContext(r, platform)
	scopeKind := ui_provider.ProfileScopeKind(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scopeKind == "" {
		scopeKind = ui_provider.ProfileScopeSpace
	}
	if scopeKind == ui_provider.ProfileScopeGlobal && r.Method != http.MethodGet {
		actor, ok := auth.FromContext(r.Context())
		if !ok || actor == nil || (!actor.HasPermission(auth.PermSystemAdmin) && !actor.IsSystemActor()) {
			writeError(w, http.StatusForbidden, "global_profile_forbidden", "global UI profile requires administrator privileges")
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		profile, layers, err := h.providerRegistry.ProfileForContext(r.Context(), resolveContext)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "profile_load_failed", err.Error())
			return
		}
		scopeProfile, scopeExists, err := h.providerRegistry.ProfileForScope(r.Context(), resolveContext, scopeKind)
		if err != nil {
			writeError(w, http.StatusBadRequest, "profile_scope_invalid", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"profile": profile, "layers": layers, "context": resolveContext,
			"scope": scopeKind, "scopeProfile": scopeProfile, "scopeExists": scopeExists,
		})
	case http.MethodPut, http.MethodPost:
		var profile ui_provider.Profile
		if err := decodeJSON(r, &profile); err != nil {
			writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
			return
		}
		expectedRevision := profile.Revision
		profile.UpdatedAt = time.Now().UTC().UnixMilli()
		saved, err := h.providerRegistry.SetProfileForContext(r.Context(), resolveContext, scopeKind, profile, expectedRevision)
		if err != nil {
			if errors.Is(err, ui_provider.ErrRevisionConflict) {
				writeError(w, http.StatusConflict, "profile_revision_conflict", err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "profile_invalid", err.Error())
			return
		}
		if h.providerBroadcaster != nil {
			h.providerBroadcaster("ui_profile_changed", map[string]interface{}{
				// SSE extension events are broadcast to all connected UI hosts. Do not
				// expose user/device identifiers in a global event. Every client simply
				// refreshes its own authenticated/scoped snapshot after this signal.
				"scope": scopeKind, "revision": saved.Revision,
			})
		}
		writeJSON(w, http.StatusOK, saved)
	case http.MethodDelete:
		expectedRevision := int64(-1)
		if raw := strings.TrimSpace(r.URL.Query().Get("revision")); raw != "" {
			if _, err := fmt.Sscanf(raw, "%d", &expectedRevision); err != nil {
				writeError(w, http.StatusBadRequest, "revision_invalid", "revision must be an integer")
				return
			}
		}
		if err := h.providerRegistry.DeleteProfileScope(r.Context(), resolveContext, scopeKind, expectedRevision); err != nil {
			if errors.Is(err, ui_provider.ErrRevisionConflict) {
				writeError(w, http.StatusConflict, "profile_revision_conflict", err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "profile_delete_failed", err.Error())
			return
		}
		if h.providerBroadcaster != nil {
			h.providerBroadcaster("ui_profile_changed", map[string]interface{}{
				"scope": scopeKind, "deleted": true,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "scope": scopeKind})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (h *HTTPHandler) handleSessionsCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		ContributionID string `json:"contributionId"`
		Surface        string `json:"surface"`
		CharacterID    string `json:"characterId"`
		ConversationID string `json:"conversationId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	def, err := h.uiHost.GetContribution(ui_contribution.ContributionID(req.ContributionID))
	if err != nil {
		writeError(w, http.StatusNotFound, "contribution_not_found", err.Error())
		return
	}
	if req.Surface == "" {
		req.Surface = "web"
	}
	if h.authorizer == nil {
		writeError(w, http.StatusInternalServerError, "authorizer_not_configured", "session authorizer not configured (fail-closed)")
		return
	}
	sessionAuth, err := h.authorizer.AuthorizeSession(r.Context(), def, req.CharacterID, req.ConversationID)
	if err != nil {
		writeError(w, http.StatusForbidden, "permission_denied", err.Error())
		return
	}
	origin := fmt.Sprintf("amitia-extension://%s/%s", def.ExtensionID, def.ModuleID)
	lifetime := time.Hour
	sess, err := h.uiHost.Bridge().CreateSession(def, origin, sessionAuth.GrantedScopes, sessionAuth.GrantedPerms, req.Surface, req.CharacterID, req.ConversationID, lifetime)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_create_failed", err.Error())
		return
	}
	if actor, ok := auth.FromContext(r.Context()); ok && actor != nil {
		_ = h.uiHost.Bridge().SetSessionExecutionIdentity(sess.SessionID, actor.SpaceID.String(), strings.TrimSpace(r.Header.Get("X-Amitia-Device-ID")))
	}
	writeJSON(w, http.StatusOK, bridgeSessionResponse{
		SessionID:       sess.SessionID,
		ContributionID:  sess.ContributionID,
		ExtensionID:     sess.ExtensionID,
		ModuleID:        sess.ModuleID,
		Generation:      sess.Generation,
		Origin:          sess.Origin,
		ContractVersion: sess.ContractVersion,
		GrantedScopes:   sess.GrantedScopes,
		GrantedPerms:    sess.GrantedPerms,
		CreatedAt:       sess.CreatedAt,
		ExpiresAt:       sess.ExpiresAt,
		Token:           sess.Token,
	})
}

func (h *HTTPHandler) handleSessionItem(w http.ResponseWriter, r *http.Request) {
	segs := splitPath(r.URL.Path)
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "session id required")
		return
	}
	sessionID := segs[4]
	if len(segs) == 5 {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		h.uiHost.Bridge().RevokeSession(sessionID)
		writeJSON(w, http.StatusOK, map[string]any{"revoked": true, "sessionId": sessionID})
		return
	}
	if len(segs) == 6 && segs[5] == "bridge" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			Method          string          `json:"method"`
			ContributionID  string          `json:"contributionId"`
			Origin          string          `json:"origin"`
			ContractVersion int             `json:"contractVersion"`
			Token           string          `json:"token"`
			Generation      int64           `json:"generation"`
			Nonce           string          `json:"nonce"`
			Payload         json.RawMessage `json:"payload"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
			return
		}
		msg := ui_contribution.BridgeMessage{
			Method:          ui_contribution.UIBridgeMethod(req.Method),
			SessionID:       sessionID,
			ContributionID:  req.ContributionID,
			Origin:          req.Origin,
			ContractVersion: req.ContractVersion,
			Token:           req.Token,
			Generation:      req.Generation,
			Nonce:           req.Nonce,
			Payload:         req.Payload,
		}
		resp := h.uiHost.Bridge().Handle(r.Context(), msg)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "unknown path")
}

func (h *HTTPHandler) handlePageSession(w http.ResponseWriter, r *http.Request) {
	segs := splitPath(r.URL.Path)
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "session id required")
		return
	}
	sessionID := segs[4]
	if len(segs) == 6 && segs[5] == "status" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		sess, err := h.pageHost.GetSession(extension_page_host.PageSessionID(sessionID))
		if err != nil {
			writeError(w, http.StatusNotFound, "page_session_not_found", err.Error())
			return
		}
		if (sess.State == extension_page_host.PageStateRuntimeStarting || sess.State == extension_page_host.PageStateLoading) && time.Since(sess.CreatedAt) > time.Second {
			sess.SetState(extension_page_host.PageStateReady)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sessionId":          sess.SessionID,
			"generation":         sess.Generation,
			"state":              sess.State,
			"definition":         nil,
			"missingPermissions": []string{},
			"reason":             "",
		})
		return
	}
	if len(segs) == 5 {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if err := h.pageHost.ClosePage(r.Context(), extension_page_host.PageSessionID(sessionID)); err != nil {
			writeError(w, http.StatusInternalServerError, "page_session_close_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"closed": true, "sessionId": sessionID})
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "unknown path")
}

func (h *HTTPHandler) handleExtensionScoped(w http.ResponseWriter, r *http.Request) {
	segs := splitEscapedPath(r.URL.EscapedPath())
	if len(segs) < 3 {
		writeError(w, http.StatusNotFound, "not_found", "extension id required")
		return
	}
	extensionID := segs[2]
	if len(segs) >= 4 {
		switch segs[3] {
		case "ui":
			if r.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			all := h.uiHost.ListAll()
			out := make([]*ui_contribution.UIContributionDefinition, 0)
			for _, def := range all {
				if string(def.ExtensionID) == extensionID {
					out = append(out, def)
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"extensionId":   extensionID,
				"contributions": out,
			})
			return
		case "pages":
			if len(segs) >= 6 && segs[5] == "open" {
				h.openExtensionPage(w, r, extensionID, segs[4])
				return
			}
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "unknown path")
}

func (h *HTTPHandler) openExtensionPage(w http.ResponseWriter, r *http.Request, extensionID, pageID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body struct {
		Params         map[string]any `json:"params"`
		ScopeSnapshot  string         `json:"scopeSnapshot"`
		DeepLinkOrigin string         `json:"deepLinkOrigin"`
	}
	_ = decodeJSON(r, &body)
	params := map[string]string{}
	for k, v := range body.Params {
		if s, ok := v.(string); ok {
			params[k] = s
		}
	}
	req := extension_page_host.OpenPageRequest{
		ExtensionID:    extension_page_host.ExtensionID(extensionID),
		PageID:         extension_page_host.PageID(pageID),
		Params:         params,
		ScopeSnapshot:  body.ScopeSnapshot,
		DeepLinkOrigin: body.DeepLinkOrigin,
	}
	result, err := h.pageHost.OpenPage(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "page_open_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *HTTPHandler) handleExtensionContributions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	extensionID := r.URL.Query().Get("extensionId")
	if extensionID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "extensionId query parameter required")
		return
	}
	all := h.uiHost.ListAll()
	out := make([]*ui_contribution.UIContributionDefinition, 0)
	for _, def := range all {
		if string(def.ExtensionID) == extensionID {
			out = append(out, def)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"extensionId":   extensionID,
		"contributions": out,
	})
}

func (h *HTTPHandler) handleOpenPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body struct {
		ExtensionID    string         `json:"extensionId"`
		PageID         string         `json:"pageId"`
		Params         map[string]any `json:"params"`
		ScopeSnapshot  string         `json:"scopeSnapshot"`
		DeepLinkOrigin string         `json:"deepLinkOrigin"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	if body.ExtensionID == "" || body.PageID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "extensionId and pageId required")
		return
	}
	params := map[string]string{}
	for k, v := range body.Params {
		if s, ok := v.(string); ok {
			params[k] = s
		}
	}
	req := extension_page_host.OpenPageRequest{
		ExtensionID:    extension_page_host.ExtensionID(body.ExtensionID),
		PageID:         extension_page_host.PageID(body.PageID),
		Params:         params,
		ScopeSnapshot:  body.ScopeSnapshot,
		DeepLinkOrigin: body.DeepLinkOrigin,
	}
	result, err := h.pageHost.OpenPage(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "page_open_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *HTTPHandler) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	segs := splitEscapedPath(r.URL.EscapedPath())
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "extensionId and contributionId required")
		return
	}
	extensionID := segs[len(segs)-2]
	contributionID := segs[len(segs)-1]
	def, err := h.uiHost.GetContribution(ui_contribution.ContributionID(contributionID))
	if err != nil {
		writeError(w, http.StatusNotFound, "contribution_not_found", err.Error())
		return
	}
	if string(def.ExtensionID) != extensionID {
		writeError(w, http.StatusNotFound, "schema_not_found", "schema contribution does not belong to extension")
		return
	}
	if def.Entry.Type != ui_contribution.SandboxSchemaRenderer || def.Sandbox.Type != ui_contribution.SandboxSchemaRenderer || strings.TrimSpace(def.Entry.SchemaPath) == "" {
		writeError(w, http.StatusNotFound, "schema_not_found", "contribution does not declare a schema renderer")
		return
	}
	if h.schemaLookup != nil {
		if doc, ok := h.schemaLookup(extensionID, contributionID); ok {
			writeJSON(w, http.StatusOK, map[string]any{
				"extensionId":    extensionID,
				"contributionId": contributionID,
				"schemaPath":     def.Entry.SchemaPath,
				"document":       json.RawMessage(doc),
			})
			return
		}
	}
	writeError(w, http.StatusNotFound, "schema_not_loaded", "schema resource is not loaded")
}

func (h *HTTPHandler) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	segs := splitPath(r.URL.Path)
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "contributionId and actionId required")
		return
	}
	contributionID := segs[3]
	actionID := segs[4]
	var req struct {
		Context map[string]any  `json:"context"`
		Input   json.RawMessage `json:"input"`
	}
	_ = decodeJSON(r, &req)
	resp := h.invokeAction(r, contributionID, actionID, req.Context, req.Input)
	writeJSON(w, http.StatusOK, resp)
}

func (h *HTTPHandler) handleComposerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	segs := splitPath(r.URL.Path)
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "actionId required")
		return
	}
	actionID := segs[4]
	var req struct {
		Context map[string]any  `json:"context"`
		Input   json.RawMessage `json:"input"`
	}
	_ = decodeJSON(r, &req)
	var contributionID string
	for _, def := range h.uiHost.ListAll() {
		if def.Kind == ui_contribution.UIContributionComposerAction {
			contributionID = string(def.ContributionID)
			break
		}
	}
	if contributionID == "" {
		writeError(w, http.StatusNotFound, "composer_action_not_found", "no composer action contribution registered")
		return
	}
	resp := h.invokeAction(r, contributionID, actionID, req.Context, req.Input)
	writeJSON(w, http.StatusOK, resp)
}

func (h *HTTPHandler) handleHostSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.hostRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "host_registry_unavailable", "UI host registry not configured")
		return
	}
	actor, ok := auth.FromContext(r.Context())
	if !ok || actor == nil || strings.TrimSpace(actor.SpaceID.String()) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authenticated user is required")
		return
	}
	var req struct {
		HostClientID string   `json:"hostClientId"`
		DeviceID     string   `json:"deviceId"`
		Platform     string   `json:"platform"`
		WindowID     string   `json:"windowId"`
		Features     []string `json:"features"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	hostClientID := strings.TrimSpace(req.HostClientID)
	if !validHostClientID(hostClientID) {
		writeError(w, http.StatusBadRequest, "host_client_id_invalid", "hostClientId is invalid")
		return
	}
	platform, err := runtimeidentity.ParsePlatform(req.Platform)
	if err != nil {
		writeError(w, http.StatusBadRequest, "platform_invalid", err.Error())
		return
	}
	deviceID := actor.DeviceID
	if deviceID == "" {
		deviceID = runtimeidentity.ParseDeviceID(req.DeviceID)
	}
	features := normalizeUIHostFeatures(req.Features)
	if len(features) == 0 {
		features = []host_registry.EndpointFeature{
			host_registry.EndpointFeatureUINotify,
			host_registry.EndpointFeatureUIDialog,
			host_registry.EndpointFeatureUINavigate,
			host_registry.EndpointFeatureUIStyle,
		}
	}
	now := time.Now().UTC()
	hostSessionID := "uihs_" + uuid.NewString()
	entry := &host_registry.HostEntry{
		EntryID:         hostClientID,
		Kind:            host_registry.RegistryEntryKindUIHost,
		SpaceID:         actor.SpaceID,
		DeviceID:        deviceID,
		Platform:        platform,
		Features:        features,
		AuthenticatedAt: now,
		LastHeartbeat:   now,
		PresenceState:   host_registry.PresenceStateReady,
		HostClientID:    hostClientID,
		HostSessionID:   hostSessionID,
		WindowID:        strings.TrimSpace(req.WindowID),
	}
	if err := h.hostRegistry.RegisterHost(r.Context(), entry); err != nil {
		writeError(w, http.StatusInternalServerError, "host_register_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hostClientId":             hostClientID,
		"hostSessionId":            hostSessionID,
		"heartbeatIntervalSeconds": 60,
		"features":                 features,
	})
}

func (h *HTTPHandler) handleHostSessionHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.hostRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "host_registry_unavailable", "UI host registry not configured")
		return
	}
	hostClientID, _, ok := h.authorizedHostSessionRequest(w, r)
	if !ok {
		return
	}
	if err := h.hostRegistry.UpdateHeartbeat(r.Context(), hostClientID); err != nil {
		writeError(w, http.StatusNotFound, "host_session_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hostClientId": hostClientID})
}

func (h *HTTPHandler) handleHostSessionDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.hostRegistry == nil {
		writeError(w, http.StatusServiceUnavailable, "host_registry_unavailable", "UI host registry not configured")
		return
	}
	hostClientID, _, ok := h.authorizedHostSessionRequest(w, r)
	if !ok {
		return
	}
	if err := h.hostRegistry.SetDisconnected(r.Context(), hostClientID); err != nil {
		writeError(w, http.StatusNotFound, "host_session_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "hostClientId": hostClientID})
}

func (h *HTTPHandler) authorizedHostSessionRequest(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	actor, ok := auth.FromContext(r.Context())
	if !ok || actor == nil || strings.TrimSpace(actor.SpaceID.String()) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authenticated user is required")
		return "", "", false
	}
	var req struct {
		HostClientID  string `json:"hostClientId"`
		HostSessionID string `json:"hostSessionId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return "", "", false
	}
	req.HostClientID = strings.TrimSpace(req.HostClientID)
	req.HostSessionID = strings.TrimSpace(req.HostSessionID)
	if req.HostClientID == "" || req.HostSessionID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "hostClientId and hostSessionId are required")
		return "", "", false
	}
	entry, err := h.hostRegistry.GetHost(r.Context(), req.HostClientID)
	if err != nil || entry == nil {
		writeError(w, http.StatusNotFound, "host_session_not_found", "UI host session not found")
		return "", "", false
	}
	if entry.SpaceID != actor.SpaceID || entry.HostSessionID != req.HostSessionID {
		writeError(w, http.StatusForbidden, "host_session_mismatch", "UI host session does not belong to this actor")
		return "", "", false
	}
	return req.HostClientID, req.HostSessionID, true
}

func validHostClientID(value string) bool {
	if len(value) < 3 || len(value) > 160 {
		return false
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == ':' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

func normalizeUIHostFeatures(raw []string) []host_registry.EndpointFeature {
	allowed := map[host_registry.EndpointFeature]struct{}{
		host_registry.EndpointFeatureUINotify:       {},
		host_registry.EndpointFeatureUIDialog:       {},
		host_registry.EndpointFeatureUINavigate:     {},
		host_registry.EndpointFeatureUIStyle:        {},
		host_registry.EndpointFeatureClipboardRead:  {},
		host_registry.EndpointFeatureClipboardWrite: {},
		host_registry.EndpointFeatureDesktopMenu:    {},
		host_registry.EndpointFeatureDesktopTray:    {},
	}
	seen := map[host_registry.EndpointFeature]struct{}{}
	result := make([]host_registry.EndpointFeature, 0, len(raw))
	for _, item := range raw {
		feature := host_registry.EndpointFeature(strings.TrimSpace(item))
		if _, ok := allowed[feature]; !ok {
			continue
		}
		if _, duplicate := seen[feature]; duplicate {
			continue
		}
		seen[feature] = struct{}{}
		result = append(result, feature)
	}
	return result
}

func (h *HTTPHandler) handleDialogResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.dialogResolver == nil {
		writeError(w, http.StatusServiceUnavailable, "dialog_resolver_unavailable", "dialog resolver not configured")
		return
	}
	var req struct {
		DialogID      string `json:"dialogId"`
		Result        string `json:"result"`
		HostClientID  string `json:"hostClientId"`
		HostSessionID string `json:"hostSessionId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	if req.DialogID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "dialogId is required")
		return
	}
	if req.Result == "" {
		req.Result = "ok"
	}
	if !h.dialogResolver.ResolveDialogWithHost(req.DialogID, req.HostClientID, req.HostSessionID, req.Result) {
		writeError(w, http.StatusNotFound, "dialog_not_found", "no pending dialog with id: "+req.DialogID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dialogId": req.DialogID})
}

func (h *HTTPHandler) handleClientRuntimeResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.clientRuntimeResolver == nil {
		writeError(w, http.StatusServiceUnavailable, "client_runtime_resolver_unavailable", "client runtime resolver not configured")
		return
	}
	var req struct {
		CommandID     string                 `json:"commandId"`
		HostClientID  string                 `json:"hostClientId"`
		HostSessionID string                 `json:"hostSessionId"`
		Result        map[string]interface{} `json:"result"`
		Error         string                 `json:"error"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	if strings.TrimSpace(req.CommandID) == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "commandId is required")
		return
	}
	resolved := false
	if scoped, ok := h.clientRuntimeResolver.(ScopedClientRuntimeCommandResolver); ok {
		resolved = scoped.ResolveClientRuntimeCommandWithHost(req.CommandID, strings.TrimSpace(req.HostClientID), strings.TrimSpace(req.HostSessionID), req.Result, req.Error)
	} else {
		resolved = h.clientRuntimeResolver.ResolveClientRuntimeCommand(req.CommandID, req.Result, req.Error)
	}
	if !resolved {
		writeError(w, http.StatusNotFound, "client_runtime_command_not_found", "pending client runtime command/host binding not found: "+req.CommandID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "commandId": req.CommandID})
}

func (h *HTTPHandler) handleClientRuntimeState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	scoped, ok := h.clientRuntimeResolver.(ScopedClientRuntimeCommandResolver)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "client_runtime_state_unavailable", "scoped client runtime resolver not configured")
		return
	}
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversationId"))
	if conversationID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "conversationId is required")
		return
	}
	spaceID := ""
	if actor, ok := auth.FromContext(r.Context()); ok && actor != nil {
		spaceID = actor.SpaceID.String()
	}
	writeJSON(w, http.StatusOK, scoped.ClientRuntimeSessionState(spaceID, conversationID))
}

func (h *HTTPHandler) handleClientRuntimeSessionAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	scoped, ok := h.clientRuntimeResolver.(ScopedClientRuntimeCommandResolver)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "client_runtime_state_unavailable", "scoped client runtime resolver not configured")
		return
	}
	var req struct {
		ConversationID string `json:"conversationId"`
		Revision       int64  `json:"revision"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	req.ConversationID = strings.TrimSpace(req.ConversationID)
	if req.ConversationID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "conversationId is required")
		return
	}
	spaceID := ""
	if actor, ok := auth.FromContext(r.Context()); ok && actor != nil {
		spaceID = actor.SpaceID.String()
	}
	state, err := scoped.AcknowledgeClientRuntimeSession(spaceID, req.ConversationID, req.Revision)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "client_runtime_revision_conflict", "message": err.Error(), "state": state})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *HTTPHandler) handleClipboardResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.clipboardResolver == nil {
		writeError(w, http.StatusServiceUnavailable, "clipboard_resolver_unavailable", "clipboard resolver not configured")
		return
	}
	var req struct {
		RequestID     string `json:"requestId"`
		Text          string `json:"text"`
		Error         string `json:"error"`
		HostClientID  string `json:"hostClientId"`
		HostSessionID string `json:"hostSessionId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	if req.RequestID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "requestId is required")
		return
	}
	if req.Error != "" {
		if !h.clipboardResolver.FailClipboardRequestWithHost(req.RequestID, req.HostClientID, req.HostSessionID, errors.New(req.Error)) {
			writeError(w, http.StatusNotFound, "clipboard_request_not_found", "no pending clipboard request with id: "+req.RequestID)
			return
		}
	} else {
		if !h.clipboardResolver.ResolveClipboardRequestWithHost(req.RequestID, req.HostClientID, req.HostSessionID, req.Text) {
			writeError(w, http.StatusNotFound, "clipboard_request_not_found", "no pending clipboard request with id: "+req.RequestID)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "requestId": req.RequestID})
}

func (h *HTTPHandler) invokeAction(r *http.Request, contributionID, actionID string, ctxMap map[string]any, input json.RawMessage) ui_contribution.BridgeResponse {
	sessionID, _ := ctxMap["sessionId"].(string)
	if strings.TrimSpace(sessionID) == "" {
		return h.invokeHostGuardedAction(r, contributionID, actionID, ctxMap, input)
	}
	origin, _ := ctxMap["origin"].(string)
	contractVersion := 0
	switch v := ctxMap["contractVersion"].(type) {
	case float64:
		contractVersion = int(v)
	case int:
		contractVersion = v
	}
	token, _ := ctxMap["token"].(string)
	nonce, _ := ctxMap["nonce"].(string)
	var generation int64
	switch v := ctxMap["generation"].(type) {
	case float64:
		generation = int64(v)
	case int:
		generation = int64(v)
	case int64:
		generation = v
	}
	payload, _ := json.Marshal(map[string]any{
		"action_id": actionID,
		"input":     input,
	})
	msg := ui_contribution.BridgeMessage{
		Method:          ui_contribution.BridgeUIActionInvoke,
		SessionID:       sessionID,
		ContributionID:  contributionID,
		Origin:          origin,
		ContractVersion: contractVersion,
		Token:           token,
		Generation:      generation,
		Nonce:           nonce,
		Payload:         payload,
	}
	return h.uiHost.Bridge().Handle(r.Context(), msg)
}

func (h *HTTPHandler) invokeHostGuardedAction(r *http.Request, contributionID, actionID string, ctxMap map[string]any, input json.RawMessage) ui_contribution.BridgeResponse {
	if h.uiHost == nil || h.authorizer == nil {
		return ui_contribution.BridgeResponse{OK: false, Error: ui_contribution.NewUIError(ui_contribution.UIErrRuntimeUnavailable, "host action guard unavailable", nil)}
	}
	def, err := h.uiHost.GetContribution(ui_contribution.ContributionID(contributionID))
	if err != nil {
		return ui_contribution.BridgeResponse{OK: false, Error: ui_contribution.NewUIError(ui_contribution.UIErrRuntimeUnavailable, err.Error(), nil)}
	}
	characterID := mapString(ctxMap, "characterId")
	conversationID := mapString(ctxMap, "conversationId")
	sessionAuth, err := h.authorizer.AuthorizeSession(r.Context(), def, characterID, conversationID)
	if err != nil {
		return ui_contribution.BridgeResponse{OK: false, Error: ui_contribution.NewUIError(ui_contribution.UIErrPermissionDenied, err.Error(), nil)}
	}
	sess, err := h.uiHost.Bridge().CreateSession(def, "amitia-host", sessionAuth.GrantedScopes, sessionAuth.GrantedPerms, "same-page", characterID, conversationID, time.Minute)
	if err != nil {
		return ui_contribution.BridgeResponse{OK: false, Error: ui_contribution.NewUIError(ui_contribution.UIErrRuntimeUnavailable, err.Error(), nil)}
	}
	defer h.uiHost.Bridge().RevokeSession(sess.SessionID)
	if actor, ok := auth.FromContext(r.Context()); ok && actor != nil {
		_ = h.uiHost.Bridge().SetSessionExecutionIdentity(sess.SessionID, actor.SpaceID.String(), strings.TrimSpace(actor.DeviceID.String()))
	}
	payload, _ := json.Marshal(map[string]any{
		"action_id": actionID,
		"input":     input,
	})
	return h.uiHost.Bridge().Handle(r.Context(), ui_contribution.BridgeMessage{
		Method:          ui_contribution.BridgeUIActionInvoke,
		SessionID:       sess.SessionID,
		ContributionID:  contributionID,
		Origin:          sess.Origin,
		ContractVersion: sess.ContractVersion,
		Token:           sess.Token,
		Generation:      sess.Generation,
		Nonce:           uuid.NewString(),
		Payload:         payload,
	})
}

func mapString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

type bridgeSessionResponse struct {
	SessionID       string    `json:"sessionId"`
	ContributionID  string    `json:"contributionId"`
	ExtensionID     string    `json:"extensionId"`
	ModuleID        string    `json:"moduleId"`
	Generation      int64     `json:"generation"`
	Origin          string    `json:"origin"`
	ContractVersion int       `json:"contractVersion"`
	GrantedScopes   []string  `json:"grantedScopes"`
	GrantedPerms    []string  `json:"grantedPerms"`
	CreatedAt       time.Time `json:"createdAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Token           string    `json:"token"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}

func decodeJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

func splitEscapedPath(path string) []string {
	segs := splitPath(path)
	out := make([]string, len(segs))
	for i, seg := range segs {
		if decoded, err := url.PathUnescape(seg); err == nil {
			out[i] = decoded
		} else {
			out[i] = seg
		}
	}
	return out
}
