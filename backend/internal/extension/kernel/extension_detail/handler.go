package extension_detail

import (
	"context"
	"encoding/json"
	amitialog "log"
	"net/http"
	"strings"
)

type HTTPHandler struct {
	service *DetailService
}

func NewHTTPHandler(service *DetailService) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/extensions/", h.handleDetail)
	mux.HandleFunc("/api/extensions/", h.handleDetail)
}

func (h *HTTPHandler) handleDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, nil)
		return
	}
	extensionID := parseExtensionID(r.URL.Path)
	if extensionID == "" {
		writeError(w, http.StatusBadRequest, nil)
		return
	}
	sub := parseSubResource(r.URL.Path, extensionID)
	switch sub {
	case "":
		detail, err := h.service.GetDetail(r.Context(), extensionID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	case "actions":
		actions, err := h.service.ListActions(r.Context(), extensionID)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, actions)
	default:
		writeError(w, http.StatusNotFound, nil)
	}
}

func parseExtensionID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/extensions/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func parseSubResource(path, extensionID string) string {
	prefix := "/api/extensions/" + extensionID + "/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
}

var _ = context.Background
