package task_runtime

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type TaskHandler struct {
	service *TaskRuntimeService
}

func NewTaskHandler(service *TaskRuntimeService) *TaskHandler {
	return &TaskHandler{service: service}
}

func (h *TaskHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/extensions/tasks", h.handleTasks)
	mux.HandleFunc("/api/extensions/tasks/", h.handleTaskDetail)
	mux.HandleFunc("/api/extensions/task-definitions", h.handleTaskDefinitions)
	mux.HandleFunc("/api/extensions/task-definitions/", h.handleTaskDefinitionDetail)
}

func (h *TaskHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTasks(w, r)
	case http.MethodPost:
		h.createTask(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TaskHandler) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/extensions/tasks/")
	parts := strings.SplitN(path, "/", 2)
	taskRunID := parts[0]
	if taskRunID == "" {
		http.Error(w, "task run id required", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.getTask(w, r, taskRunID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	action := parts[1]

	switch action {
	case "progress", "result", "checkpoint":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	switch action {
	case "cancel":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.cancelTask(w, r, taskRunID)
	case "retry", "recover":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if action == "retry" {
			h.retryTask(w, r, taskRunID)
		} else {
			h.recoverTask(w, r, taskRunID)
		}
	case "pause":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.pauseTask(w, r, taskRunID)
	case "resume":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.resumeTask(w, r, taskRunID)
	case "progress":
		h.getProgress(w, r, taskRunID)
	case "result":
		h.getResult(w, r, taskRunID)
	case "checkpoint":
		h.getCheckpoint(w, r, taskRunID)
	default:
		http.Error(w, "unknown action", http.StatusNotFound)
	}
}

func (h *TaskHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	filter := ListTasksFilter{
		ExtensionID: r.URL.Query().Get("extensionId"),
		Status:      r.URL.Query().Get("status"),
	}
	runs, err := h.service.ListTaskRuns(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if runs == nil {
		runs = []*TaskRun{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": runs,
		"total": len(runs),
	})
}

func (h *TaskHandler) createTask(w http.ResponseWriter, r *http.Request) {
	var req EnqueueTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "task_input_invalid", err.Error())
		return
	}
	if req.TaskDefinitionID == "" {
		writeError(w, http.StatusBadRequest, "task_definition_invalid", "taskDefinitionId required")
		return
	}
	if req.OperationID == "" {
		req.OperationID = "op-" + uuid.NewString()
	}

	def, err := h.service.GetTaskDefinition(r.Context(), req.TaskDefinitionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "task_definition_invalid", err.Error())
		return
	}

	result, err := h.service.Enqueue(r.Context(), req, def)
	if err != nil {
		if te, ok := err.(*TaskError); ok {
			writeError(w, HTTPStatusForErrorCode(te.Code), string(te.Code), te.Message)
		} else {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *TaskHandler) getTask(w http.ResponseWriter, r *http.Request, taskRunID string) {
	run, err := h.service.GetTaskRun(r.Context(), taskRunID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *TaskHandler) cancelTask(w http.ResponseWriter, r *http.Request, taskRunID string) {
	var body struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if body.Reason == "" {
		body.Reason = "user_requested"
	}
	if err := h.service.Cancel(r.Context(), taskRunID, body.Reason); err != nil {
		if te, ok := err.(*TaskError); ok {
			writeError(w, HTTPStatusForErrorCode(te.Code), string(te.Code), te.Message)
		} else {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"taskRunId": taskRunID, "status": "cancelling"})
}

func (h *TaskHandler) retryTask(w http.ResponseWriter, r *http.Request, taskRunID string) {
	run, err := h.service.Retry(r.Context(), taskRunID)
	if err != nil {
		if te, ok := err.(*TaskError); ok {
			writeError(w, HTTPStatusForErrorCode(te.Code), string(te.Code), te.Message)
		} else {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (h *TaskHandler) pauseTask(w http.ResponseWriter, r *http.Request, taskRunID string) {
	var body struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if body.Reason == "" {
		body.Reason = "user_requested"
	}
	run, err := h.service.Pause(r.Context(), taskRunID, body.Reason)
	if err != nil {
		if te, ok := err.(*TaskError); ok {
			writeError(w, HTTPStatusForErrorCode(te.Code), string(te.Code), te.Message)
		} else {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *TaskHandler) resumeTask(w http.ResponseWriter, r *http.Request, taskRunID string) {
	run, err := h.service.Resume(r.Context(), taskRunID)
	if err != nil {
		if te, ok := err.(*TaskError); ok {
			writeError(w, HTTPStatusForErrorCode(te.Code), string(te.Code), te.Message)
		} else {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *TaskHandler) recoverTask(w http.ResponseWriter, r *http.Request, taskRunID string) {
	run, err := h.service.Recover(r.Context(), taskRunID)
	if err != nil {
		if te, ok := err.(*TaskError); ok {
			writeError(w, HTTPStatusForErrorCode(te.Code), string(te.Code), te.Message)
		} else {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *TaskHandler) getProgress(w http.ResponseWriter, r *http.Request, taskRunID string) {
	prog, err := h.service.GetProgress(r.Context(), taskRunID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if prog == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"taskRunId": taskRunID})
		return
	}
	writeJSON(w, http.StatusOK, prog)
}

func (h *TaskHandler) getResult(w http.ResponseWriter, r *http.Request, taskRunID string) {
	result, err := h.service.GetResult(r.Context(), taskRunID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if result == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"taskRunId": taskRunID})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *TaskHandler) getCheckpoint(w http.ResponseWriter, r *http.Request, taskRunID string) {
	cp, err := h.service.GetLatestCheckpoint(r.Context(), taskRunID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if cp == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"taskRunId": taskRunID})
		return
	}
	writeJSON(w, http.StatusOK, cp)
}

func (h *TaskHandler) handleTaskDefinitions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTaskDefinitions(w, r)
	case http.MethodPost:
		h.createTaskDefinition(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TaskHandler) handleTaskDefinitionDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/extensions/task-definitions/")
	defID := strings.TrimSuffix(path, "/")
	if defID == "" {
		http.Error(w, "task definition id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTaskDefinition(w, r, defID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TaskHandler) listTaskDefinitions(w http.ResponseWriter, r *http.Request) {
	extensionID := r.URL.Query().Get("extensionId")
	defs, err := h.service.ListTaskDefinitions(r.Context(), extensionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if defs == nil {
		defs = []*TaskDefinition{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": defs,
		"total": len(defs),
	})
}

func (h *TaskHandler) createTaskDefinition(w http.ResponseWriter, r *http.Request) {
	var def TaskDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		writeError(w, http.StatusBadRequest, "task_definition_invalid", err.Error())
		return
	}
	if def.TaskID == "" {
		writeError(w, http.StatusBadRequest, "task_definition_invalid", "taskId required")
		return
	}
	if def.ExtensionID == "" {
		writeError(w, http.StatusBadRequest, "task_definition_invalid", "extensionId required")
		return
	}
	if def.Entry == "" {
		writeError(w, http.StatusBadRequest, "task_definition_invalid", "entry required")
		return
	}
	if def.RuntimeType == "" {
		def.RuntimeType = "task_javascript"
	}
	if err := h.service.PutTaskDefinition(r.Context(), &def); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, def)
}

func (h *TaskHandler) getTaskDefinition(w http.ResponseWriter, r *http.Request, defID string) {
	def, err := h.service.GetTaskDefinition(r.Context(), defID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task_definition_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, def)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}
