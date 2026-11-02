package extension_detail

import (
	"encoding/json"
	"fmt"
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
	mux.HandleFunc("/api/extension-center/extension", h.handleDetail)
}

func (h *HTTPHandler) handleDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, nil)
		return
	}
	extensionID := strings.TrimSpace(r.URL.Query().Get("id"))
	if extensionID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id query parameter required"))
		return
	}
	sub := strings.TrimSpace(r.URL.Query().Get("resource"))
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
