//go:build legacy_migration

package package_legacy_migration

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type SourceRepository struct {
	db *gorm.DB
}

func NewSourceRepository(
	db *gorm.DB,
) (*SourceRepository, error) {
	if db == nil {
		return nil,
			fmt.Errorf(
				"legacy migration: source database unavailable",
			)
	}

	return &SourceRepository{
		db: db,
	}, nil
}

func (r *SourceRepository) BackfillOwnership(
	ctx context.Context,
) error {
	if err :=
		r.db.WithContext(ctx).
			Exec(`
				UPDATE extensions
				SET
					owner_user_id=(
						SELECT user_id
						FROM extension_agent_skill_metadata
						WHERE
							extension_agent_skill_metadata.extension_id=
								extensions.extension_id
					),
					scope_type=COALESCE((
						SELECT scope_type
						FROM extension_agent_skill_metadata
						WHERE
							extension_agent_skill_metadata.extension_id=
								extensions.extension_id
					), scope_type),
					scope_id=COALESCE((
						SELECT scope_id
						FROM extension_agent_skill_metadata
						WHERE
							extension_agent_skill_metadata.extension_id=
								extensions.extension_id
					), scope_id)
				WHERE
					source='instructions'
					AND owner_user_id=''
			`).
			Error; err != nil {
		return err
	}

	if err :=
		r.db.WithContext(ctx).
			Exec(`
				UPDATE extensions
				SET
					owner_user_id=COALESCE((
						SELECT ws.user_id
						FROM extension_artifacts ea
						JOIN extension_workshop_sessions ws
							ON ws.id=ea.session_id
						WHERE
							ea.extension_id=extensions.extension_id
							AND ea.extension_version=
								extensions.current_version
						LIMIT 1
					), owner_user_id),
					scope_type=CASE
						WHEN COALESCE((
							SELECT ws.character_id
							FROM extension_artifacts ea
							JOIN extension_workshop_sessions ws
								ON ws.id=ea.session_id
							WHERE
								ea.extension_id=extensions.extension_id
								AND ea.extension_version=
									extensions.current_version
							LIMIT 1
						), '')=''
						THEN 'global'
						ELSE 'character'
					END,
					scope_id=COALESCE((
						SELECT ws.character_id
						FROM extension_artifacts ea
						JOIN extension_workshop_sessions ws
							ON ws.id=ea.session_id
						WHERE
							ea.extension_id=extensions.extension_id
							AND ea.extension_version=
								extensions.current_version
						LIMIT 1
					), scope_id)
				WHERE
					source='workflow'
					AND owner_user_id=''
			`).
			Error; err != nil {
		return err
	}

	return r.db.WithContext(ctx).
		Exec(`
			UPDATE extension_versions
			SET
				artifact_id=COALESCE((
					SELECT artifact_id
					FROM extension_artifacts
					WHERE
						extension_artifacts.extension_id=
							extension_versions.extension_id
						AND extension_artifacts.extension_version=
							extension_versions.version
					LIMIT 1
				), artifact_id),
				artifact_hash=CASE
					WHEN artifact_hash=''
					THEN checksum
					ELSE artifact_hash
				END,
				package_hash=CASE
					WHEN package_hash=''
					THEN checksum
					ELSE package_hash
				END,
				source=CASE
					WHEN source=''
					THEN COALESCE((
						SELECT source
						FROM extension_artifacts
						WHERE
							extension_artifacts.extension_id=
								extension_versions.extension_id
							AND extension_artifacts.extension_version=
								extension_versions.version
						LIMIT 1
					), '')
					ELSE source
				END,
				compatibility_status=CASE
					WHEN compatibility_status=''
					THEN 'compatible'
					ELSE compatibility_status
				END,
				capabilities_json=CASE
					WHEN capabilities_json=''
					THEN '[]'
					ELSE capabilities_json
				END,
				validation_status=CASE
					WHEN validation_status=''
					THEN 'valid'
					ELSE validation_status
				END
		`).
		Error
}

func (r *SourceRepository) ListCandidates(
	ctx context.Context,
) ([]LegacyPackageCandidate, error) {
	var candidates []LegacyPackageCandidate

	err :=
		r.db.WithContext(ctx).
			Raw(`
				SELECT
					e.extension_id,
					e.current_version AS version,
					COALESCE(v.package_blob, X'')
						AS package_blob,
					COALESCE(e.owner_user_id, '')
						AS user_id,
					COALESCE(e.scope_type, 'global')
						AS scope_type,
					COALESCE(e.scope_id, '')
						AS scope_id
				FROM extensions e
				LEFT JOIN extension_versions v
					ON v.extension_id=e.extension_id
					AND v.version=e.current_version
				WHERE COALESCE(e.archived_at, '')=''
				ORDER BY e.extension_id
			`).
			Scan(&candidates).
			Error

	return candidates, err
}

func (r *SourceRepository) ListSigners(
	ctx context.Context,
) ([]LegacySignerCandidate, error) {
	if !r.db.Migrator().
		HasTable(
			"extension_package_signers",
		) {
		return nil, nil
	}

	var signers []LegacySignerCandidate

	err :=
		r.db.WithContext(ctx).
			Table(
				"extension_package_signers",
			).
			Select(
				"fingerprint, created_at",
			).
			Order(
				"fingerprint",
			).
			Scan(&signers).
			Error

	return signers, err
}
