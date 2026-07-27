package wasm_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type APIService struct {
	factory *WASMRuntimeFactory
	repo    DefinitionRepository
}

type DefinitionRepository interface {
	Put(ctx context.Context, def *WASMRuntimeDefinition) error
	Get(ctx context.Context, id string) (*WASMRuntimeDefinition, error)
	GetByModule(ctx context.Context, extensionID, moduleID string) (*WASMRuntimeDefinition, error)
	ListByExtension(ctx context.Context, extensionID string) ([]*WASMRuntimeDefinition, error)
	List(ctx context.Context) ([]*WASMRuntimeDefinition, error)
	Delete(ctx context.Context, id string) error
	DeleteByExtension(ctx context.Context, extensionID string) error
}

func NewAPIService(factory *WASMRuntimeFactory, repo DefinitionRepository) *APIService {
	return &APIService{factory: factory, repo: repo}
}

type HTTPHandler struct {
	service *APIService
}

func NewHTTPHandler(service *APIService) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/wasm/definitions", h.handleDefinitions)
	mux.HandleFunc("/api/wasm/definitions/", h.handleDefinition)
	mux.HandleFunc("/api/wasm/modules", h.handleModules)
	mux.HandleFunc("/api/wasm/modules/", h.handleModule)
	mux.HandleFunc("/api/wasm/invoke", h.handleInvoke)
	mux.HandleFunc("/api/wasm/instances", h.handleInstances)
	mux.HandleFunc("/api/wasm/validate", h.handleValidate)
}

func (h *HTTPHandler) handleDefinitions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listDefinitions(w, r)
	case http.MethodPost:
		h.createDefinition(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleDefinition(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/wasm/definitions/")
	if id == "" {
		writeAPIError(w, http.StatusBadRequest, "definition id required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.getDefinition(w, r, id)
	case http.MethodDelete:
		h.deleteDefinition(w, r, id)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleModules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listModules(w, r)
	case http.MethodPost:
		h.uploadModule(w, r)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleModule(w http.ResponseWriter, r *http.Request) {
	moduleID := strings.TrimPrefix(r.URL.Path, "/api/wasm/modules/")
	if moduleID == "" {
		writeAPIError(w, http.StatusBadRequest, "module id required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.getModule(w, r, moduleID)
	case http.MethodDelete:
		h.deleteModule(w, r, moduleID)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *HTTPHandler) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ModuleID string          `json:"module_id"`
		Input    json.RawMessage `json:"input"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ModuleID == "" {
		writeAPIError(w, http.StatusBadRequest, "module_id required")
		return
	}
	result, err := h.service.factory.Invoke(r.Context(), req.ModuleID, req.Input)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{
		"output":    json.RawMessage(result.Output),
		"duration":  result.Duration.String(),
		"fuel_used": result.FuelUsed,
		"cached":    result.Cached,
	})
}

func (h *HTTPHandler) handleInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	instances := h.service.factory.ListInstances()
	out := make([]map[string]any, 0, len(instances))
	for _, inst := range instances {
		out = append(out, map[string]any{
			"instance_id": inst.instanceID,
			"identity":    inst.Identity(),
			"stats":       inst.Stats(),
		})
	}
	writeAPIJSON(w, http.StatusOK, out)
}

func (h *HTTPHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.ParseMultipartForm(32 << 20)
	file, _, err := r.FormFile("module")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "module file required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "read file failed")
		return
	}
	validator := NewModuleValidator()
	report, err := validator.ValidateBytes(data)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}

func (h *HTTPHandler) listDefinitions(w http.ResponseWriter, r *http.Request) {
	extensionID := r.URL.Query().Get("extension_id")
	var defs []*WASMRuntimeDefinition
	var err error
	if extensionID != "" {
		defs, err = h.service.repo.ListByExtension(r.Context(), extensionID)
	} else {
		defs, err = h.service.repo.List(r.Context())
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusOK, defs)
}

func (h *HTTPHandler) createDefinition(w http.ResponseWriter, r *http.Request) {
	var def WASMRuntimeDefinition
	if err := decodeJSON(r, &def); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if def.RuntimeDefinitionID == "" {
		def.RuntimeDefinitionID = uuid.NewString()
	}
	if err := ValidateDefinition(&def); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.factory.RegisterDefinition(&def); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.service.repo != nil {
		if err := h.service.repo.Put(r.Context(), &def); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeAPIJSON(w, http.StatusCreated, def)
}

func (h *HTTPHandler) getDefinition(w http.ResponseWriter, r *http.Request, id string) {
	def, err := h.service.repo.Get(r.Context(), id)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusOK, def)
}

func (h *HTTPHandler) deleteDefinition(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.service.repo.Delete(r.Context(), id); err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *HTTPHandler) listModules(w http.ResponseWriter, r *http.Request) {
	mods := h.service.factory.ModuleManager().List()
	out := make([]map[string]any, 0, len(mods))
	for _, mod := range mods {
		out = append(out, map[string]any{
			"module_id": mod.ModuleID,
			"path":      mod.Path,
			"hash":      mod.Hash,
			"size":      mod.Size,
			"valid":     mod.Report != nil && mod.Report.Valid,
		})
	}
	writeAPIJSON(w, http.StatusOK, out)
}

func (h *HTTPHandler) uploadModule(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	moduleID := r.FormValue("module_id")
	if moduleID == "" {
		writeAPIError(w, http.StatusBadRequest, "module_id required")
		return
	}
	file, _, err := r.FormFile("module")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "module file required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "read file failed")
		return
	}
	if err := h.service.factory.LoadModule(moduleID, data); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	mod, _ := h.service.factory.ModuleManager().Get(moduleID)
	writeAPIJSON(w, http.StatusCreated, map[string]any{
		"module_id": moduleID,
		"hash":      mod.Hash,
		"size":      mod.Size,
		"valid":     mod.Report.Valid,
	})
}

func (h *HTTPHandler) getModule(w http.ResponseWriter, r *http.Request, moduleID string) {
	mod, ok := h.service.factory.ModuleManager().Get(moduleID)
	if !ok {
		writeAPIError(w, http.StatusNotFound, fmt.Sprintf("module not found: %s", moduleID))
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{
		"module_id": mod.ModuleID,
		"path":      mod.Path,
		"hash":      mod.Hash,
		"size":      mod.Size,
		"valid":     mod.Report != nil && mod.Report.Valid,
	})
}

func (h *HTTPHandler) deleteModule(w http.ResponseWriter, r *http.Request, moduleID string) {
	h.service.factory.ModuleManager().Unload(moduleID)
	writeAPIJSON(w, http.StatusOK, map[string]any{"unloaded": true})
}

func decodeJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return fmt.Errorf("empty body")
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}
	return nil
}

func writeAPIJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}
