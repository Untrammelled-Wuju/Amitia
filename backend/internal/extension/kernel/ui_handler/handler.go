package ui_handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/chat_ui_extension"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/extension_slots"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/sandbox_webui"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type HTTPHandler struct {
	uiHost       *ui_contribution.UIHost
	slotRegistry *extension_slots.SlotRegistry
	pageHost     *extension_page_host.PageHost
	sandboxHost  *sandbox_webui.Host
	chatRegistry *chat_ui_extension.ChatExtensionRegistry
	permChecker  *permission.UIPermissionChecker
	schemaLookup func(extensionID, contributionID string) (json.RawMessage, bool)
	extRoot      string
}

func NewHTTPHandler(
	uiHost *ui_contribution.UIHost,
	slotRegistry *extension_slots.SlotRegistry,
	pageHost *extension_page_host.PageHost,
	sandboxHost *sandbox_webui.Host,
	chatRegistry *chat_ui_extension.ChatExtensionRegistry,
) *HTTPHandler {
	return &HTTPHandler{
		uiHost:       uiHost,
		slotRegistry: slotRegistry,
		pageHost:     pageHost,
		sandboxHost:  sandboxHost,
		chatRegistry: chatRegistry,
		permChecker:  permission.NewUIPermissionChecker(),
	}
}

func (h *HTTPHandler) SetExtensionRoot(root string) {
	h.extRoot = root
}

func (h *HTTPHandler) SetSchemaLookup(fn func(extensionID, contributionID string) (json.RawMessage, bool)) {
	h.schemaLookup = fn
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/extensions/ui/slots", h.handleSlots)
	mux.HandleFunc("/api/extensions/ui/contributions", h.handleContributions)
	mux.HandleFunc("/api/extensions/ui/snapshot", h.handleSnapshot)
	mux.HandleFunc("/api/extensions/ui/sessions", h.handleSessionsCollection)
	mux.HandleFunc("/api/extensions/ui/sessions/", h.handleSessionItem)
	mux.HandleFunc("/api/extensions/ui/page-sessions/", h.handlePageSession)
	mux.HandleFunc("/api/extensions/ui/open-page", h.handleOpenPage)
	mux.HandleFunc("/api/extensions/ui/by-extension", h.handleExtensionContributions)
	mux.HandleFunc("/api/extensions/", h.handleExtensionScoped)
	mux.HandleFunc("/api/extension/schema/", h.handleSchema)
	mux.HandleFunc("/api/extensions/ui/schema/", h.handleSchema)
	mux.HandleFunc("/api/extension/webui/session", h.handleWebUISessionCollection)
	mux.HandleFunc("/api/extension/webui/session/", h.handleWebUISessionItem)
	mux.HandleFunc("/api/extension/webui/bridge/", h.handleWebUIBridge)
	mux.HandleFunc("/api/extension/webui/preload/", h.handleWebUIPreload)
	mux.HandleFunc("/api/extension/webui/resource/", h.handleWebUIResource)
	mux.HandleFunc("/api/extension/webui/stats", h.handleWebUIStats)
	mux.HandleFunc("/api/extension/action/", h.handleAction)
	mux.HandleFunc("/api/extension/composer/action/", h.handleComposerAction)
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
		SlotID        string                                      `json:"slotId"`
		Contributions []*ui_contribution.UIContributionDefinition `json:"contributions"`
	}
	out := make([]slotSnapshotEntry, 0, len(slots))
	for _, s := range slots {
		contribs := h.uiHost.ListBySlot(string(s.SlotID))
		out = append(out, slotSnapshotEntry{
			SlotID:        string(s.SlotID),
			Contributions: contribs,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"slots":     out,
		"timestamp": time.Now().UTC(),
	})
}

func (h *HTTPHandler) handleSessionsCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		ContributionID  string   `json:"contributionId"`
		Origin          string   `json:"origin"`
		GrantedScopes   []string `json:"grantedScopes"`
		GrantedPerms    []string `json:"grantedPerms"`
		LifetimeSeconds int64    `json:"lifetimeSeconds"`
		Surface         string   `json:"surface"`
		CharacterID     string   `json:"characterId"`
		ConversationID  string   `json:"conversationId"`
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
	if h.permChecker != nil {
		if err := h.permChecker.ValidateSessionRequest(def, req.GrantedScopes, req.GrantedPerms); err != nil {
			writeError(w, http.StatusForbidden, "permission_denied", err.Error())
			return
		}
	}
	lifetime := time.Hour
	if req.LifetimeSeconds > 0 {
		lifetime = time.Duration(req.LifetimeSeconds) * time.Second
	}
	sess, err := h.uiHost.Bridge().CreateSession(def, req.Origin, req.GrantedScopes, req.GrantedPerms, req.Surface, req.CharacterID, req.ConversationID, lifetime)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_create_failed", err.Error())
		return
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
	segs := splitPath(r.URL.Path)
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
	segs := splitPath(r.URL.Path)
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
	if string(def.ExtensionID) != extensionID || def.Kind != ui_contribution.UIContributionSchemaPage {
		writeError(w, http.StatusNotFound, "schema_not_found", "schema contribution does not belong to extension")
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

func (h *HTTPHandler) handleWebUISessionCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		ContributionID     string                      `json:"contributionId"`
		ExtensionID        string                      `json:"extensionId"`
		ModuleID           string                      `json:"moduleId"`
		Generation         int64                       `json:"generation"`
		SlotID             string                      `json:"slotId"`
		Sandbox            string                      `json:"sandbox"`
		CSP                string                      `json:"csp"`
		AllowedActions     []string                    `json:"allowedActions"`
		AllowedDataSources []string                    `json:"allowedDataSources"`
		Theme              sandbox_webui.ThemeSnapshot `json:"theme"`
		Locale             string                      `json:"locale"`
		BasePath           string                      `json:"basePath"`
		EntryPath          string                      `json:"entryPath"`
		GrantedScopes      []string                    `json:"grantedScopes"`
		GrantedPerms       []string                    `json:"grantedPerms"`
		Surface            string                      `json:"surface"`
		CharacterID        string                      `json:"characterId"`
		ConversationID     string                      `json:"conversationId"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	def, err := h.uiHost.GetContribution(ui_contribution.ContributionID(req.ContributionID))
	if err != nil || def == nil {
		writeError(w, http.StatusNotFound, "contribution_not_found", "web ui contribution not found")
		return
	}
	if def.Sandbox.Type != ui_contribution.SandboxWebRestricted && def.Sandbox.Type != ui_contribution.SandboxWebIsolated {
		writeError(w, http.StatusBadRequest, "sandbox_invalid", "contribution is not a web ui")
		return
	}
	if req.ExtensionID != string(def.ExtensionID) || req.ModuleID != string(def.ModuleID) || req.SlotID != def.Slot.SlotID || req.Generation != def.Integrity.Generation {
		writeError(w, http.StatusForbidden, "session_identity_mismatch", "session request does not match contribution")
		return
	}
	if req.Surface == "" {
		req.Surface = "web"
	}
	if h.permChecker != nil {
		if err := h.permChecker.ValidateSessionRequest(def, req.GrantedScopes, req.GrantedPerms); err != nil {
			writeError(w, http.StatusForbidden, "permission_denied", err.Error())
			return
		}
	}
	sandbox := sandbox_webui.SandboxType(def.Sandbox.Type)
	basePath := h.resolveExtensionBasePath(req.ExtensionID)
	if basePath == "" {
		writeError(w, http.StatusNotFound, "extension_path_not_found", "extension bundle path not found")
		return
	}
	sreq := sandbox_webui.CreateSessionRequest{
		ContributionID:     req.ContributionID,
		ExtensionID:        req.ExtensionID,
		ModuleID:           req.ModuleID,
		Generation:         req.Generation,
		SlotID:             req.SlotID,
		Sandbox:            sandbox,
		CSP:                sandbox_webui.RestrictedCSP,
		AllowedActions:     contributionActionIDs(def),
		AllowedDataSources: contributionDataSourceIDs(def),
		Theme:              req.Theme,
		Locale:             req.Locale,
		BasePath:           basePath,
		EntryPath:          def.Entry.Path,
		Surface:            req.Surface,
		CharacterID:        req.CharacterID,
		ConversationID:     req.ConversationID,
	}
	result, err := h.sandboxHost.CreateSession(sreq)
	if err != nil {
		writeError(w, http.StatusBadRequest, "webui_session_create_failed", err.Error())
		return
	}
	resourceURL := fmt.Sprintf("/api/extension/webui/resource/%s/%s", result.SessionID, def.Entry.Path)
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId":   result.SessionID,
		"entryUrl":    result.EntryURL,
		"resourceUrl": resourceURL,
		"origin":      result.Origin,
		"nonce":       result.Nonce,
		"token":       result.Token,
		"csp":         result.CSP,
	})
}

func (h *HTTPHandler) handleWebUISessionItem(w http.ResponseWriter, r *http.Request) {
	segs := splitPath(r.URL.Path)
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "session id required")
		return
	}
	sessionID := segs[4]
	if len(segs) == 5 {
		switch r.Method {
		case http.MethodDelete:
			if err := h.sandboxHost.CloseSession(sessionID, "api_request"); err != nil {
				writeError(w, http.StatusNotFound, "webui_session_not_found", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"closed": true, "sessionId": sessionID})
		case http.MethodGet:
			info, err := h.sandboxHost.GetSessionInfo(sessionID)
			if err != nil {
				writeError(w, http.StatusNotFound, "webui_session_not_found", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, info)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
		return
	}
	if len(segs) == 6 && segs[5] == "preload" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		script, err := h.sandboxHost.GetPreloadScript(sessionID)
		if err != nil {
			writeError(w, http.StatusNotFound, "preload_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(script))
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "unknown path")
}

func (h *HTTPHandler) handleWebUIPreload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	segs := splitPath(r.URL.Path)
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "session id required")
		return
	}
	sessionID := segs[4]
	script, err := h.sandboxHost.GetPreloadScript(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "preload_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(script))
}

func (h *HTTPHandler) handleWebUIStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	stats := h.sandboxHost.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *HTTPHandler) handleWebUIResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	segs := splitPath(r.URL.Path)
	if len(segs) < 6 {
		writeError(w, http.StatusNotFound, "not_found", "session id and resource path required")
		return
	}
	sessionID := segs[4]
	resourcePath := strings.Join(segs[5:], "/")
	if resourcePath == "" {
		resourcePath = "index.html"
	}
	sess, err := h.sandboxHost.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "webui_session_not_found", err.Error())
		return
	}
	if resourcePath == "__bridge__/bridge.js" {
		script, err := h.sandboxHost.GetPreloadScript(sessionID)
		if err != nil {
			writeError(w, http.StatusNotFound, "bridge_not_found", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Content-Security-Policy", sess.CSP)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(script))
		return
	}
	basePath := h.resolveExtensionBasePath(sess.ExtensionID)
	if basePath == "" {
		writeError(w, http.StatusNotFound, "extension_path_not_found", "extension bundle path not found")
		return
	}
	cleanPath, err := sandbox_webui.NewProtocolHandler().SanitizePath(basePath, resourcePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_path", err.Error())
		return
	}
	fullPath := filepath.Join(basePath, cleanPath)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "resource_not_found", "resource not found")
		return
	}
	mime := sandbox_webui.LookupMIME(cleanPath)
	if mime == "" {
		writeError(w, http.StatusUnsupportedMediaType, "mime_not_allowed", "mime type not allowed")
		return
	}
	if !sandbox_webui.IsMIMEAllowed(mime) {
		writeError(w, http.StatusUnsupportedMediaType, "mime_not_allowed", "mime type not allowed")
		return
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read_failed", err.Error())
		return
	}
	if strings.HasSuffix(cleanPath, ".html") || strings.HasSuffix(cleanPath, ".htm") {
		content = injectBridgeScript(content, sess.CSP, sessionID)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", sess.CSP)
	} else {
		w.Header().Set("Content-Type", mime)
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *HTTPHandler) resolveExtensionBasePath(extensionID string) string {
	if h.extRoot == "" || extensionID == "" {
		return ""
	}
	safeID := strings.NewReplacer("/", "__", "\\", "__", ":", "_", "..", "_").Replace(extensionID)
	installedRoot := filepath.Join(h.extRoot, "installed", safeID)
	entries, err := os.ReadDir(installedRoot)
	if err != nil {
		return ""
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}
		versionDir := filepath.Join(installedRoot, entries[i].Name())
		subEntries, err := os.ReadDir(versionDir)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			candidate := filepath.Join(versionDir, sub.Name())
			if _, err := os.Stat(filepath.Join(candidate, "manifest.json")); err == nil {
				return candidate
			}
		}
	}
	return ""
}

func injectBridgeScript(html []byte, csp, sessionID string) []byte {
	htmlStr := string(html)
	if csp == "" {
		csp = sandbox_webui.DefaultCSP
	}
	cspMeta := fmt.Sprintf(`<meta http-equiv="Content-Security-Policy" content="%s">`, csp)
	scriptTag := fmt.Sprintf(`<script src="/api/extension/webui/resource/%s/__bridge__/bridge.js"></script>`, sessionID)
	if idx := strings.LastIndex(htmlStr, "</head>"); idx >= 0 {
		htmlStr = htmlStr[:idx] + cspMeta + "\n" + scriptTag + "\n" + htmlStr[idx:]
	} else if idx := strings.LastIndex(htmlStr, "</body>"); idx >= 0 {
		htmlStr = htmlStr[:idx] + cspMeta + "\n" + scriptTag + "\n" + htmlStr[idx:]
	} else {
		htmlStr = cspMeta + "\n" + scriptTag + "\n" + htmlStr
	}
	return []byte(htmlStr)
}

func (h *HTTPHandler) handleWebUIBridge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	segs := splitPath(r.URL.Path)
	if len(segs) < 5 {
		writeError(w, http.StatusNotFound, "not_found", "session id required")
		return
	}
	sessionID := segs[4]
	sess, err := h.sandboxHost.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "webui_session_not_found", err.Error())
		return
	}
	var msg sandbox_webui.BridgeMessage
	if err := decodeJSON(r, &msg); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	msg.Session = sessionID
	if err := sandbox_webui.ValidateBridgeMessage(&msg, sess); err != nil {
		writeError(w, http.StatusForbidden, "bridge_invalid", err.Error())
		return
	}
	bridge := h.sandboxHost.GetBridge()
	if bridge == nil {
		writeError(w, http.StatusInternalServerError, "bridge_unavailable", "bridge not initialized")
		return
	}
	result, err := bridge.HandleMessage(r.Context(), sandbox_webui.InvokeRequest{
		SessionID: sessionID,
		Message:   msg,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     false,
			"method": msg.Method,
			"id":     msg.ID,
			"error":  err.Error(),
		})
		return
	}
	response := map[string]any{
		"ok":     true,
		"method": msg.Method,
		"id":     msg.ID,
	}
	if result.Output != nil {
		response["output"] = json.RawMessage(result.Output)
	}
	if result.Error != "" {
		response["ok"] = false
		response["error"] = result.Error
	}
	writeJSON(w, http.StatusOK, response)
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

func (h *HTTPHandler) invokeAction(r *http.Request, contributionID, actionID string, ctxMap map[string]any, input json.RawMessage) ui_contribution.BridgeResponse {
	sessionID, _ := ctxMap["sessionId"].(string)
	origin, _ := ctxMap["origin"].(string)
	contractVersion := 0
	switch v := ctxMap["contractVersion"].(type) {
	case float64:
		contractVersion = int(v)
	case int:
		contractVersion = v
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
		Payload:         payload,
	}
	return h.uiHost.Bridge().Handle(r.Context(), msg)
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
}

func contributionActionIDs(def *ui_contribution.UIContributionDefinition) []string {
	ids := make([]string, 0, len(def.Actions))
	for _, action := range def.Actions {
		ids = append(ids, action.ActionID)
	}
	return ids
}

func contributionDataSourceIDs(def *ui_contribution.UIContributionDefinition) []string {
	if def == nil || len(def.DataSources) == 0 {
		return nil
	}
	ids := make([]string, 0, len(def.DataSources))
	for _, ds := range def.DataSources {
		ids = append(ids, ds.SourceID)
	}
	return ids
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
