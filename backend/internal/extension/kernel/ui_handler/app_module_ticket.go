package ui_handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

type appModuleTicketRequest struct {
	ExtensionID    string `json:"extensionId"`
	ContributionID string `json:"contributionId"`
}

func (h *HTTPHandler) handleAppModuleTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	var body appModuleTicketRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "payload_invalid", err.Error())
		return
	}
	body.ExtensionID = strings.TrimSpace(body.ExtensionID)
	body.ContributionID = strings.TrimSpace(body.ContributionID)
	if body.ExtensionID == "" || body.ContributionID == "" {
		writeError(w, http.StatusBadRequest, "payload_invalid", "extensionId and contributionId are required")
		return
	}
	definition, err := h.uiHost.GetContribution(ui_contribution.ContributionID(body.ContributionID))
	if err != nil || string(definition.ExtensionID) != body.ExtensionID {
		writeError(w, http.StatusNotFound, "app_module_not_found", "app contribution not found")
		return
	}
	if string(definition.Sandbox.Type) != "client_module" {
		writeError(w, http.StatusUnprocessableEntity, "app_module_not_supported", "contribution sandbox is not client_module")
		return
	}
	if strings.TrimSpace(string(definition.Entry.Path)) == "" {
		writeError(w, http.StatusUnprocessableEntity, "app_module_not_supported", "contribution entry path is empty")
		return
	}
	if !uiContributionDeclaresAction(definition, "command") {
		writeError(w, http.StatusForbidden, "app_module_forbidden", "contribution does not declare command")
		return
	}
	expiresAt := time.Now().Add(time.Hour).Unix()
	writeJSON(w, http.StatusOK, map[string]any{
		"extensionId":    body.ExtensionID,
		"contributionId": body.ContributionID,
		"entryPath":      strings.TrimSpace(string(definition.Entry.Path)),
		"entryHash":      strings.TrimSpace(string(definition.Entry.ContentHash)),
		"generation":     definition.Integrity.Generation,
		"accessToken":    pageModuleCapability(body.ExtensionID, body.ContributionID, expiresAt),
		"expiresAt":      time.Unix(expiresAt, 0).UTC().Format(time.RFC3339),
	})
}
