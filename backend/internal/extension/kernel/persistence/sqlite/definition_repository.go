package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type DefinitionRepository struct {
	db *sql.DB
}

func NewDefinitionRepository(db *sql.DB) *DefinitionRepository {
	return &DefinitionRepository{db: db}
}

func (r *DefinitionRepository) PutExtension(ctx context.Context, def domain.ExtensionDefinition) error {
	if err := def.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("sqlite: marshal definition: %w", err)
	}

	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	id := definitionKey(def.ID, def.Version)
	now := time.Now().UTC()

	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_definitions (id, extension_id, version, manifest_version, definition_json, definition_hash, publisher_id, trust_level, source_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			definition_json = excluded.definition_json,
			definition_hash = excluded.definition_hash,
			manifest_version = excluded.manifest_version,
			publisher_id = excluded.publisher_id,
			trust_level = excluded.trust_level,
			source_type = excluded.source_type
	`,
		id,
		string(def.ID),
		def.Version.String(),
		def.ManifestVersion,
		string(data),
		hashHex,
		def.Publisher.PublisherID,
		def.Publisher.TrustLevel,
		"local",
		now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert extension definition: %w", err)
	}

	return nil
}

func (r *DefinitionRepository) GetExtension(ctx context.Context, id domain.ExtensionID, version domain.SemanticVersion) (domain.ExtensionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	key := definitionKey(id, version)

	var data string
	err := ex.QueryRowContext(ctx, `SELECT definition_json FROM extension_definitions WHERE id = ?`, key).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ExtensionDefinition{}, domain.ErrInvalidExtensionID
		}
		return domain.ExtensionDefinition{}, fmt.Errorf("sqlite: query extension definition: %w", err)
	}

	var def domain.ExtensionDefinition
	if err := json.Unmarshal([]byte(data), &def); err != nil {
		return domain.ExtensionDefinition{}, fmt.Errorf("sqlite: unmarshal definition: %w", err)
	}

	return def, nil
}

func (r *DefinitionRepository) ListExtensions(ctx context.Context) ([]domain.ExtensionDefinition, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `SELECT definition_json FROM extension_definitions ORDER BY extension_id, version`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list extension definitions: %w", err)
	}
	defer rows.Close()

	var out []domain.ExtensionDefinition
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("sqlite: scan definition: %w", err)
		}
		var def domain.ExtensionDefinition
		if err := json.Unmarshal([]byte(data), &def); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal definition: %w", err)
		}
		out = append(out, def)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate definitions: %w", err)
	}

	return out, nil
}

func (r *DefinitionRepository) DeleteExtension(ctx context.Context, id domain.ExtensionID, version domain.SemanticVersion) error {
	ex := getExecutor(ctx, r.db)
	key := definitionKey(id, version)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_definitions WHERE id = ?`, key)
	if err != nil {
		return fmt.Errorf("sqlite: delete extension definition: %w", err)
	}
	return nil
}

func definitionKey(id domain.ExtensionID, version domain.SemanticVersion) string {
	return string(id) + "@" + version.String()
}

var _ domain.DefinitionRepository = (*DefinitionRepository)(nil)
