package extension_center

import (
	"encoding/json"
	amitialog "log"
	"net/http"
	"strings"
)

type HTTPHandler struct {
	service *CenterService
}

func NewHTTPHandler(service *CenterService) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/extension-center/view", h.handleView)
}

func (h *HTTPHandler) handleView(w http.ResponseWriter, r *http.Request) {
	filter := parseFilter(r)
	sortKey := parseSort(r)
	view, err := h.service.GetView(r.Context(), filter, sortKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func parseFilter(r *http.Request) CenterFilter {
	q := r.URL.Query()
	filter := CenterFilter{
		Search:     q.Get("search"),
		Platform:   q.Get("platform"),
		UpdateOnly: q.Get("updateOnly") == "1",
	}
	if v := q.Get("status"); v != "" {
		for _, s := range strings.Split(v, ",") {
			filter.Status = append(filter.Status, ExtensionStatus(strings.TrimSpace(s)))
		}
	}
	if v := q.Get("trust"); v != "" {
		for _, s := range strings.Split(v, ",") {
			filter.Trust = append(filter.Trust, TrustLevel(strings.TrimSpace(s)))
		}
	}
	if v := q.Get("tags"); v != "" {
		for _, s := range strings.Split(v, ",") {
			filter.Tags = append(filter.Tags, ContributionTag(strings.TrimSpace(s)))
		}
	}
	if v := q.Get("enabled"); v != "" {
		b := v == "1" || v == "true"
		filter.Enabled = &b
	}
	if v := q.Get("dev"); v != "" {
		b := v == "1" || v == "true"
		filter.DevMode = &b
	}
	return filter
}

func parseSort(r *http.Request) CenterSortKey {
	v := r.URL.Query().Get("sort")
	switch v {
	case "name":
		return SortByName
	case "recent":
		return SortByRecent
	case "trust":
		return SortByTrust
	case "status":
		return SortByStatus
	}
	return SortByName
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
