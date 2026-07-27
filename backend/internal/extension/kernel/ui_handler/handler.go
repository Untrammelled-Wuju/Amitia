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

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/extensions/ui/slots", h.handleSlots)
	mux.HandleFunc("/api/extensions/ui/contributions", h.handleContributions)
	mux.HandleFunc("/api/extensions/ui/snapshot", h.handleSnapshot)
	mux.HandleFunc("/api/extensions/ui/sessions", h.handleSessionsCollection)
	mux.HandleFunc("/api/extensions/ui/sessions/", h.handleSessionItem)
	mux.HandleFunc("/api/extensions/ui/page-sessions/", h.handlePageSession)
	mux.HandleFunc("/api/extensions/", h.handleExtensionScoped)
	mux.HandleFunc("/api/extension/schema/", h.handleSchema)
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
		SlotID        string                                `json:"slotId"`
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
		ContributionID string   `json:"contributionId"`
		Origin         string   `json:"origin"`
		GrantedScopes  []string `json:"grantedScopes"`
		GrantedPerms   []string `json:"grantedPerms"`
		LifetimeSeconds int64   `json:"lifetimeSeconds"`
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
	sess, err := h.uiHost.Bridge().CreateSession(def, req.Origin, req.GrantedScopes, req.GrantedPerms, lifetime)
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
			"sessionId":         sess.SessionID,
			"state":             sess.State,
			"definition":        nil,
			"missingPermissions": []string{},
			"reason":            "",
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
				"extensionId":  extensionID,
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
	extensionID := segs[3]
	contributionID := segs[4]
	def, err := h.uiHost.GetContribution(ui_contribution.ContributionID(contributionID))
	if err != nil {
		writeError(w, http.StatusNotFound, "contribution_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"extensionId":    extensionID,
		"contributionId": contributionID,
		"schemaPath":     def.Entry.SchemaPath,
		"document":       buildSampleSchema(),
	})
}

func (h *HTTPHandler) handleWebUISessionCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var req struct {
		ContributionID    string                       `json:"contributionId"`
		ExtensionID       string                       `json:"extensionId"`
		ModuleID          string                       `json:"moduleId"`
		Generation        int64                        `json:"generation"`
		SlotID            string                       `json:"slotId"`
		Sandbox           string                       `json:"sandbox"`
		CSP               string                       `json:"csp"`
		AllowedActions    []string                     `json:"allowedActions"`
		AllowedDataSources []string                    `json:"allowedDataSources"`
		Theme             sandbox_webui.ThemeSnapshot  `json:"theme"`
		Locale            string                       `json:"locale"`
		BasePath          string                       `json:"basePath"`
		EntryPath         string                       `json:"entryPath"`
		GrantedScopes     []string                     `json:"grantedScopes"`
		GrantedPerms      []string                     `json:"grantedPerms"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	if h.permChecker != nil && req.ContributionID != "" {
		def, err := h.uiHost.GetContribution(ui_contribution.ContributionID(req.ContributionID))
		if err == nil && def != nil {
			if err := h.permChecker.ValidateSessionRequest(def, req.GrantedScopes, req.GrantedPerms); err != nil {
				writeError(w, http.StatusForbidden, "permission_denied", err.Error())
				return
			}
		}
	}
	sandbox := sandbox_webui.SandboxType(req.Sandbox)
	if sandbox == "" {
		sandbox = sandbox_webui.SandboxWebRestricted
	}
	sreq := sandbox_webui.CreateSessionRequest{
		ContributionID:     req.ContributionID,
		ExtensionID:        req.ExtensionID,
		ModuleID:           req.ModuleID,
		Generation:         req.Generation,
		SlotID:             req.SlotID,
		Sandbox:            sandbox,
		CSP:                req.CSP,
		AllowedActions:     req.AllowedActions,
		AllowedDataSources: req.AllowedDataSources,
		Theme:              req.Theme,
		Locale:             req.Locale,
		BasePath:           req.BasePath,
		EntryPath:          req.EntryPath,
	}
	result, err := h.sandboxHost.CreateSession(sreq)
	if err != nil {
		writeError(w, http.StatusBadRequest, "webui_session_create_failed", err.Error())
		return
	}
	resourceURL := fmt.Sprintf("/api/extension/webui/resource/%s/%s", result.SessionID, req.EntryPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId":   result.SessionID,
		"entryUrl":    result.EntryURL,
		"resourceUrl": resourceURL,
		"origin":      result.Origin,
		"nonce":       result.Nonce,
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
		preloadScript, err := h.sandboxHost.GetPreloadScript(sessionID)
		if err == nil {
			content = injectPreloadScript(content, preloadScript, sess.CSP)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", mime)
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (h *HTTPHandler) resolveExtensionBasePath(extensionID string) string {
	if h.extRoot != "" {
		candidate := filepath.Join(h.extRoot, extensionID)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func injectPreloadScript(html []byte, preload string, csp string) []byte {
	htmlStr := string(html)
	if csp == "" {
		csp = sandbox_webui.DefaultCSP
	}
	cspMeta := fmt.Sprintf(`<meta http-equiv="Content-Security-Policy" content="%s">`, csp)
	scriptTag := fmt.Sprintf("<script>%s</script>", preload)
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
	msg.Origin = sess.Origin
	msg.Nonce = sess.Nonce
	if err := sandbox_webui.ValidateBridgeMessage(&msg, sess); err != nil {
		writeError(w, http.StatusBadRequest, "bridge_invalid", err.Error())
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
		Context map[string]any    `json:"context"`
		Input   json.RawMessage   `json:"input"`
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

func buildSampleSchema() map[string]any {
	return map[string]any{
		"type":    "page",
		"version": 1,
		"title":   "Extension Schema UI",
		"nodes": []map[string]any{
			{
				"type":  "section",
				"title": "General",
				"nodes": []map[string]any{
					{"type": "text", "content": "Welcome to the extension settings page."},
					{
						"type":  "button",
						"label": "Save",
						"action": map[string]any{
							"type":      "host_command",
							"command":   "save",
							"riskLevel": "low",
						},
					},
				},
			},
			{
				"type":  "section",
				"title": "Advanced",
				"nodes": []map[string]any{
					{"type": "text", "content": "Advanced configuration options."},
					{
						"type":  "button",
						"label": "Reset",
						"action": map[string]any{
							"type":         "dialog",
							"dialogId":     "reset-confirm",
							"riskLevel":    "medium",
							"confirmation": "Reset all settings to defaults?",
						},
					},
				},
			},
		},
	}
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
