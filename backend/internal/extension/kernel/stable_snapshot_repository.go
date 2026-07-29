package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type StableSnapshotRepository struct {
	db *sql.DB
}

func NewStableSnapshotRepository(db *sql.DB) *StableSnapshotRepository {
	return &StableSnapshotRepository{db: db}
}

func (r *StableSnapshotRepository) SaveSnapshot(ctx context.Context, candidateID string, extID domain.ExtensionID, snap *StableSnapshot) error {
	if r == nil || r.db == nil {
		return nil
	}
	if snap == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	capturedAt := snap.CapturedAt.Format(time.RFC3339Nano)

	for _, contrib := range snap.Contributions {
		snapshotID := "snap-" + uuid.NewString()
		defJSON, err := json.Marshal(contrib.Definition)
		if err != nil {
			return fmt.Errorf("stable-snapshot-repo: marshal definition for %s: %w", contrib.ID, err)
		}
		var runtimeBindingJSON []byte
		if contrib.RuntimeBinding != nil {
			runtimeBindingJSON, err = json.Marshal(contrib.RuntimeBinding)
			if err != nil {
				return fmt.Errorf("stable-snapshot-repo: marshal runtime binding for %s: %w", contrib.ID, err)
			}
		} else {
			runtimeBindingJSON = []byte("{}")
		}
		permJSON, err := json.Marshal(contrib.RequiredPermissions)
		if err != nil {
			return fmt.Errorf("stable-snapshot-repo: marshal permissions for %s: %w", contrib.ID, err)
		}
		scopeJSON, err := json.Marshal(contrib.RequiredScope)
		if err != nil {
			return fmt.Errorf("stable-snapshot-repo: marshal scope for %s: %w", contrib.ID, err)
		}
		_, err = r.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO kernel_candidate_stable_snapshots
			(snapshot_id, candidate_id, extension_id, contribution_id, contribution_kind,
			 stable_generation, stable_definition_json, stable_definition_hash,
			 stable_runtime_binding_json, stable_permission_json, stable_scope_json,
			 enablement_state, generation_id, captured_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			snapshotID,
			candidateID,
			string(extID),
			string(contrib.ID),
			string(contrib.Kind),
			snap.Generation,
			string(defJSON),
			snap.DefinitionHash,
			string(runtimeBindingJSON),
			string(permJSON),
			string(scopeJSON),
			string(snap.EnablementState),
			snap.GenerationID,
			capturedAt,
			now,
		)
		if err != nil {
			return fmt.Errorf("stable-snapshot-repo: save snapshot for candidate %s: %w", candidateID, err)
		}
	}
	return nil
}

func (r *StableSnapshotRepository) ListSnapshotsByCandidate(ctx context.Context, candidateID string) (*StableSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT contribution_id, contribution_kind, stable_generation, stable_definition_json,
		       stable_definition_hash, stable_runtime_binding_json, stable_permission_json,
		       stable_scope_json, enablement_state, generation_id, captured_at
		FROM kernel_candidate_stable_snapshots
		WHERE candidate_id = ?`, candidateID)
	if err != nil {
		return nil, fmt.Errorf("stable-snapshot-repo: list by candidate %s: %w", candidateID, err)
	}
	defer rows.Close()

	snap := &StableSnapshot{}
	first := true
	for rows.Next() {
		var (
			contribID      string
			contribKind    string
			defJSON        string
			defHash        string
			runtimeBindStr string
			permJSON       string
			scopeJSON      string
			enablementStr  string
			genID          string
			capturedAtStr  string
		)
		if err := rows.Scan(
			&contribID,
			&contribKind,
			&snap.Generation,
			&defJSON,
			&defHash,
			&runtimeBindStr,
			&permJSON,
			&scopeJSON,
			&enablementStr,
			&genID,
			&capturedAtStr,
		); err != nil {
			return nil, fmt.Errorf("stable-snapshot-repo: scan row: %w", err)
		}
		if first {
			snap.DefinitionHash = defHash
			snap.GenerationID = genID
			snap.EnablementState = domain.EnablementState(enablementStr)
			if capturedAtStr != "" {
				if t, pErr := time.Parse(time.RFC3339Nano, capturedAtStr); pErr == nil {
					snap.CapturedAt = t
				}
			}
			first = false
		}
		contrib := domain.ContributionDefinition{
			ID:   domain.ContributionID(contribID),
			Kind: domain.ContributionKind(contribKind),
		}
		if defJSON != "" && defJSON != "{}" {
			_ = json.Unmarshal([]byte(defJSON), &contrib.Definition)
		}
		if runtimeBindStr != "" && runtimeBindStr != "{}" {
			var rb domain.RuntimeBinding
			if jErr := json.Unmarshal([]byte(runtimeBindStr), &rb); jErr == nil {
				contrib.RuntimeBinding = &rb
			}
		}
		if permJSON != "" && permJSON != "[]" {
			_ = json.Unmarshal([]byte(permJSON), &contrib.RequiredPermissions)
		}
		if scopeJSON != "" && scopeJSON != "{}" {
			_ = json.Unmarshal([]byte(scopeJSON), &contrib.RequiredScope)
		}
		snap.Contributions = append(snap.Contributions, contrib)
	}
	if first {
		return nil, nil
	}
	return snap, rows.Err()
}

func (r *StableSnapshotRepository) DeleteSnapshotsByCandidate(ctx context.Context, candidateID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM kernel_candidate_stable_snapshots WHERE candidate_id = ?`, candidateID)
	if err != nil {
		return fmt.Errorf("stable-snapshot-repo: delete by candidate %s: %w", candidateID, err)
	}
	return nil
}
