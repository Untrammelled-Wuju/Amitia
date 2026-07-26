package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type ModuleRepository interface {
	PutModule(ctx context.Context, mod domain.ModuleDefinition) error
	GetModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) (domain.ModuleDefinition, error)
	ListModules(ctx context.Context, extensionID domain.ExtensionID) ([]domain.ModuleDefinition, error)
	DeleteModules(ctx context.Context, extensionID domain.ExtensionID) error
	DeleteModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) error
}

type SQLiteModuleRepository struct {
	db *sql.DB
}

func NewModuleRepository(db *sql.DB) *SQLiteModuleRepository {
	return &SQLiteModuleRepository{db: db}
}

func (r *SQLiteModuleRepository) PutModule(ctx context.Context, mod domain.ModuleDefinition) error {
	data, err := json.Marshal(mod)
	if err != nil {
		return fmt.Errorf("sqlite: marshal module: %w", err)
	}

	id := moduleKey(mod.ExtensionID, mod.ID)

	runtimeType := ""
	entryPath := ""
	if mod.Runtime != nil {
		runtimeType = string(mod.Runtime.Type)
		entryPath = mod.Runtime.EntryPoint
	}

	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_modules (id, extension_id, module_id, module_type, runtime_type, entry_path, enabled, definition_json, definition_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			module_type = excluded.module_type,
			runtime_type = excluded.runtime_type,
			entry_path = excluded.entry_path,
			definition_json = excluded.definition_json
	`,
		id,
		string(mod.ExtensionID),
		string(mod.ID),
		string(mod.Type),
		runtimeType,
		entryPath,
		1,
		string(data),
		"",
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert module: %w", err)
	}

	if mod.Runtime != nil {
		rtData, rtErr := json.Marshal(mod.Runtime)
		if rtErr != nil {
			return fmt.Errorf("sqlite: marshal runtime definition: %w", rtErr)
		}
		_, err = ex.ExecContext(ctx, `
			INSERT INTO extension_runtime_definitions (id, extension_id, module_id, runtime_type, entry_point, definition_json)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				runtime_type = excluded.runtime_type,
				entry_point = excluded.entry_point,
				definition_json = excluded.definition_json
		`,
			id,
			string(mod.ExtensionID),
			string(mod.ID),
			runtimeType,
			entryPath,
			string(rtData),
		)
		if err != nil {
			return fmt.Errorf("sqlite: upsert runtime definition: %w", err)
		}
	}

	if _, err := ex.ExecContext(ctx, `DELETE FROM extension_dependencies WHERE extension_id = ? AND module_id = ?`, string(mod.ExtensionID), string(mod.ID)); err != nil {
		return fmt.Errorf("sqlite: clear module dependencies: %w", err)
	}

	for _, dep := range mod.Dependencies {
		depID := id + "/" + string(dep.Type) + "/" + dep.ID
		_, err = ex.ExecContext(ctx, `
			INSERT INTO extension_dependencies (id, extension_id, module_id, dependency_type, dependency_id, version_required, optional)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			depID,
			string(mod.ExtensionID),
			string(mod.ID),
			string(dep.Type),
			dep.ID,
			dep.Version,
			dep.Optional,
		)
		if err != nil {
			return fmt.Errorf("sqlite: insert dependency: %w", err)
		}
	}

	return nil
}

func (r *SQLiteModuleRepository) GetModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) (domain.ModuleDefinition, error) {
	ex := getExecutor(ctx, r.db)
	key := moduleKey(extensionID, moduleID)

	var data string
	err := ex.QueryRowContext(ctx, `SELECT definition_json FROM extension_modules WHERE id = ?`, key).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ModuleDefinition{}, domain.ErrInvalidModuleID
		}
		return domain.ModuleDefinition{}, fmt.Errorf("sqlite: query module: %w", err)
	}

	var mod domain.ModuleDefinition
	if err := json.Unmarshal([]byte(data), &mod); err != nil {
		return domain.ModuleDefinition{}, fmt.Errorf("sqlite: unmarshal module: %w", err)
	}

	return mod, nil
}

func (r *SQLiteModuleRepository) ListModules(ctx context.Context, extensionID domain.ExtensionID) ([]domain.ModuleDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_modules WHERE extension_id = ? ORDER BY module_id`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list modules: %w", err)
	}
	defer rows.Close()

	var out []domain.ModuleDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan module: %w", err)
		}
		var mod domain.ModuleDefinition
		if err := json.Unmarshal([]byte(data), &mod); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal module: %w", err)
		}
		out = append(out, mod)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate modules: %w", err)
	}

	return out, nil
}

func (r *SQLiteModuleRepository) DeleteModules(ctx context.Context, extensionID domain.ExtensionID) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_modules WHERE extension_id = ?`, string(extensionID))
	if err != nil {
		return fmt.Errorf("sqlite: delete modules: %w", err)
	}
	_, err = ex.ExecContext(ctx, `DELETE FROM extension_runtime_definitions WHERE extension_id = ?`, string(extensionID))
	if err != nil {
		return fmt.Errorf("sqlite: delete runtime definitions: %w", err)
	}
	_, err = ex.ExecContext(ctx, `DELETE FROM extension_dependencies WHERE extension_id = ?`, string(extensionID))
	if err != nil {
		return fmt.Errorf("sqlite: delete dependencies: %w", err)
	}
	return nil
}

func (r *SQLiteModuleRepository) DeleteModule(ctx context.Context, extensionID domain.ExtensionID, moduleID domain.ModuleID) error {
	ex := getExecutor(ctx, r.db)
	key := moduleKey(extensionID, moduleID)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_modules WHERE id = ?`, key)
	if err != nil {
		return fmt.Errorf("sqlite: delete module: %w", err)
	}
	_, err = ex.ExecContext(ctx, `DELETE FROM extension_runtime_definitions WHERE id = ?`, key)
	if err != nil {
		return fmt.Errorf("sqlite: delete runtime definition: %w", err)
	}
	_, err = ex.ExecContext(ctx, `DELETE FROM extension_dependencies WHERE extension_id = ? AND module_id = ?`, string(extensionID), string(moduleID))
	if err != nil {
		return fmt.Errorf("sqlite: delete module dependencies: %w", err)
	}
	return nil
}

func moduleKey(extensionID domain.ExtensionID, moduleID domain.ModuleID) string {
	return string(extensionID) + "/" + string(moduleID)
}

var _ ModuleRepository = (*SQLiteModuleRepository)(nil)

func (r *SQLiteModuleRepository) ListAllModules(ctx context.Context) ([]domain.ModuleDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_modules ORDER BY extension_id, module_id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list all modules: %w", err)
	}
	defer rows.Close()

	var out []domain.ModuleDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan module: %w", err)
		}
		var mod domain.ModuleDefinition
		if err := json.Unmarshal([]byte(data), &mod); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal module: %w", err)
		}
		out = append(out, mod)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate modules: %w", err)
	}

	return out, nil
}
