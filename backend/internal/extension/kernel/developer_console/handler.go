package developer_console

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type HTTPHandler struct {
	service    *ConsoleService
	repository *DiagnosticRepository
}

func NewHTTPHandler(service *ConsoleService, repository *DiagnosticRepository) *HTTPHandler {
	return &HTTPHandler{service: service, repository: repository}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/dev-console/overview", h.handleOverview)
	mux.HandleFunc("/api/dev-console/sessions", h.handleSessions)
	mux.HandleFunc("/api/dev-console/invocations", h.handleInvocations)
	mux.HandleFunc("/api/dev-console/events", h.handleEvents)
	mux.HandleFunc("/api/dev-console/hooks", h.handleHooks)
	mux.HandleFunc("/api/dev-console/tasks", h.handleTasks)
	mux.HandleFunc("/api/dev-console/ui-sessions", h.handleUISessions)
	mux.HandleFunc("/api/dev-console/storage", h.handleStorage)
	mux.HandleFunc("/api/dev-console/permissions", h.handlePermissions)
	mux.HandleFunc("/api/dev-console/scopes", h.handleScopes)
	mux.HandleFunc("/api/dev-console/resources", h.handleResources)
	mux.HandleFunc("/api/dev-console/lifecycle", h.handleLifecycle)
	mux.HandleFunc("/api/dev-console/logs", h.handleLogs)
	mux.HandleFunc("/api/dev-console/performance", h.handlePerformance)
	mux.HandleFunc("/api/dev-console/migration", h.handleMigration)
	mux.HandleFunc("/api/dev-console/compatibility", h.handleCompatibility)
	mux.HandleFunc("/api/dev-console/stream", h.handleStream)
	mux.HandleFunc("/api/dev-console/export-diagnostics", h.handleExportDiagnostics)
}

func (h *HTTPHandler) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	overview, err := h.service.BuildOverview(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *HTTPHandler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ttl, _ := time.ParseDuration(r.URL.Query().Get("ttl"))
		sess, err := h.service.OpenSession(r.Context(), r.URL.Query().Get("workspace"), ttl)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, sess)
	case http.MethodDelete:
		id := ConsoleSessionID(r.URL.Query().Get("id"))
		if err := h.service.CloseSession(id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"closed": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, nil)
	}
}

func (h *HTTPHandler) handleInvocations(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListInvocations(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleHooks(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListHooks(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListTasks(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleUISessions(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListUISessions(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleStorage(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListStorage(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handlePermissions(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListPermissions(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleScopes(w http.ResponseWriter, r *http.Request) {
	recs, err := h.repository.ListScopes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleResources(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListResources(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleLifecycle(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListLifecycle(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListLogs(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handlePerformance(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	recs, err := h.repository.ListPerformance(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleMigration(w http.ResponseWriter, r *http.Request) {
	recs, err := h.repository.ListMigration(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleCompatibility(w http.ResponseWriter, r *http.Request) {
	recs, err := h.repository.ListCompatibility(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (h *HTTPHandler) handleStream(w http.ResponseWriter, r *http.Request) {
	id := ConsoleSessionID(r.URL.Query().Get("session"))
	if id == "" {
		writeError(w, http.StatusBadRequest, nil)
		return
	}
	ch, err := h.service.Subscribe(id, nil)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, nil)
		return
	}
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			data, _ := json.Marshal(ev)
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

func (h *HTTPHandler) handleExportDiagnostics(w http.ResponseWriter, r *http.Request) {
	filter := parseFilters(r)
	export, err := h.repository.ExportDiagnostics(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	overview, err := h.service.BuildOverview(r.Context())
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="amitia-diagnostics.json"`)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"overview":  overview,
			"diagnostics": export,
		})
		return
	}
	writeJSON(w, http.StatusOK, export)
}

func parseFilters(r *http.Request) ConsoleFilters {
	q := r.URL.Query()
	filter := ConsoleFilters{
		ExtensionID: q.Get("extension"),
		ModuleID:    q.Get("module"),
		Severity:    q.Get("severity"),
		Stage:       q.Get("stage"),
		Search:      q.Get("search"),
	}
	if start := q.Get("start"); start != "" {
		if ts, err := strconv.ParseInt(start, 10, 64); err == nil {
			t := time.Unix(ts, 0)
			filter.StartTime = &t
		}
	}
	if end := q.Get("end"); end != "" {
		if ts, err := strconv.ParseInt(end, 10, 64); err == nil {
			t := time.Unix(ts, 0)
			filter.EndTime = &t
		}
	}
	return filter
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "internal error"
	if err != nil {
		msg = err.Error()
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
