package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
)

type WASMDefinitionRepository struct {
	db *sql.DB
}

func NewWASMDefinitionRepository(db *sql.DB) *WASMDefinitionRepository {
	return &WASMDefinitionRepository{db: db}
}

func (r *WASMDefinitionRepository) Put(ctx context.Context, def *wasm_runtime.WASMRuntimeDefinition) error {
	if def == nil {
		return fmt.Errorf("sqlite: nil wasm definition")
	}
	if def.RuntimeDefinitionID == "" {
		return fmt.Errorf("sqlite: missing runtime definition id")
	}

	allowedImports := ""
	if len(def.AllowedImports) > 0 {
		parts := make([]string, len(def.AllowedImports))
		for i, imp := range def.AllowedImports {
			parts[i] = string(imp)
		}
		allowedImports = strings.Join(parts, ",")
	}

	deterministic := 0
	if def.Deterministic {
		deterministic = 1
	}

	defJSON, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("sqlite: marshal wasm definition: %w", err)
	}

	now := time.Now().UTC()
	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_wasm_runtime_definitions (
			id, extension_id, module_id, module_path, module_hash, module_sha256,
			engine_type, abi, memory_limit_bytes, fuel_limit, instance_policy,
			deterministic, entry_export, max_output_bytes, max_host_calls, call_timeout_ms,
			allowed_imports, definition_hash, definition_version, generation,
			definition_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension_id, module_id) DO UPDATE SET
			id = excluded.id,
			module_path = excluded.module_path,
			module_hash = excluded.module_hash,
			module_sha256 = excluded.module_sha256,
			engine_type = excluded.engine_type,
			abi = excluded.abi,
			memory_limit_bytes = excluded.memory_limit_bytes,
			fuel_limit = excluded.fuel_limit,
			instance_policy = excluded.instance_policy,
			deterministic = excluded.deterministic,
			entry_export = excluded.entry_export,
			max_output_bytes = excluded.max_output_bytes,
			max_host_calls = excluded.max_host_calls,
			call_timeout_ms = excluded.call_timeout_ms,
			allowed_imports = excluded.allowed_imports,
			definition_hash = excluded.definition_hash,
			definition_version = excluded.definition_version,
			generation = excluded.generation,
			definition_json = excluded.definition_json,
			updated_at = excluded.updated_at
	`,
		def.RuntimeDefinitionID,
		def.ExtensionID,
		def.ModuleID,
		def.ModulePath,
		def.ModuleHash,
		def.ModuleSHA256,
		def.EngineType,
		string(def.ABI),
		def.MemoryLimitBytes,
		def.FuelLimit,
		string(def.InstancePolicy),
		deterministic,
		def.EntryExport,
		def.MaxOutputBytes,
		def.MaxHostCalls,
		def.CallTimeout.Milliseconds(),
		allowedImports,
		def.DefinitionHash,
		def.DefinitionVersion,
		def.Generation,
		string(defJSON),
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert wasm definition: %w", err)
	}
	return nil
}

func (r *WASMDefinitionRepository) Get(ctx context.Context, id string) (*wasm_runtime.WASMRuntimeDefinition, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT definition_json FROM extension_wasm_runtime_definitions WHERE id = ?
	`, id)

	var defJSON string
	if err := row.Scan(&defJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: wasm definition not found: %s", id)
		}
		return nil, fmt.Errorf("sqlite: query wasm definition: %w", err)
	}

	var def wasm_runtime.WASMRuntimeDefinition
	if err := json.Unmarshal([]byte(defJSON), &def); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal wasm definition: %w", err)
	}
	return &def, nil
}

func (r *WASMDefinitionRepository) GetByModule(ctx context.Context, extensionID, moduleID string) (*wasm_runtime.WASMRuntimeDefinition, error) {
	ex := getExecutor(ctx, r.db)
	row := ex.QueryRowContext(ctx, `
		SELECT definition_json FROM extension_wasm_runtime_definitions
		WHERE extension_id = ? AND module_id = ?
	`, extensionID, moduleID)

	var defJSON string
	if err := row.Scan(&defJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: wasm definition not found for ext=%s mod=%s", extensionID, moduleID)
		}
		return nil, fmt.Errorf("sqlite: query wasm definition: %w", err)
	}

	var def wasm_runtime.WASMRuntimeDefinition
	if err := json.Unmarshal([]byte(defJSON), &def); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal wasm definition: %w", err)
	}
	return &def, nil
}

func (r *WASMDefinitionRepository) ListByExtension(ctx context.Context, extensionID string) ([]*wasm_runtime.WASMRuntimeDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT definition_json FROM extension_wasm_runtime_definitions
		WHERE extension_id = ?
		ORDER BY module_id
	`, extensionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list wasm definitions: %w", err)
	}
	defer rows.Close()

	var out []*wasm_runtime.WASMRuntimeDefinition
	for rows.Next() {
		var defJSON string
		if err := rows.Scan(&defJSON); err != nil {
			return nil, fmt.Errorf("sqlite: scan wasm definition: %w", err)
		}
		var def wasm_runtime.WASMRuntimeDefinition
		if err := json.Unmarshal([]byte(defJSON), &def); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal wasm definition: %w", err)
		}
		out = append(out, &def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate wasm definitions: %w", err)
	}
	return out, nil
}

func (r *WASMDefinitionRepository) List(ctx context.Context) ([]*wasm_runtime.WASMRuntimeDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT definition_json FROM extension_wasm_runtime_definitions
		ORDER BY extension_id, module_id
	`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list all wasm definitions: %w", err)
	}
	defer rows.Close()

	var out []*wasm_runtime.WASMRuntimeDefinition
	for rows.Next() {
		var defJSON string
		if err := rows.Scan(&defJSON); err != nil {
			return nil, fmt.Errorf("sqlite: scan wasm definition: %w", err)
		}
		var def wasm_runtime.WASMRuntimeDefinition
		if err := json.Unmarshal([]byte(defJSON), &def); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal wasm definition: %w", err)
		}
		out = append(out, &def)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate wasm definitions: %w", err)
	}
	return out, nil
}

func (r *WASMDefinitionRepository) Delete(ctx context.Context, id string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_wasm_runtime_definitions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete wasm definition: %w", err)
	}
	return nil
}

func (r *WASMDefinitionRepository) DeleteByExtension(ctx context.Context, extensionID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_wasm_runtime_definitions WHERE extension_id = ?`, extensionID)
	if err != nil {
		return fmt.Errorf("sqlite: delete wasm definitions by extension: %w", err)
	}
	return nil
}
