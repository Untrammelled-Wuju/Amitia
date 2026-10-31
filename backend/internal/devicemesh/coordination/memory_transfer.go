package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	amitialog "log"
)

type MemoryTransfer struct {
	Kind               string `json:"kind"`
	ID                 string `json:"id"`
	ToRoleID           string `json:"toRoleId"`
	TargetRoleRevision int64  `json:"targetRoleRevision,omitempty"`
}

func (s *OwnershipStore) validateMemoryTransfer(ctx context.Context, tx *sql.Tx, commit Commit) error {
	transfer := commit.MemoryTransfer
	if transfer == nil {
		return nil
	}
	if transfer.Kind != "memory" && transfer.Kind != "continuity" || transfer.ID == "" || transfer.ToRoleID == "" || transfer.ToRoleID == StorageRole(commit.Scope, transfer.Kind) || commit.LeaseProof != nil {
		return ErrWrongOwner
	}
	if transfer.ToRoleID != GlobalMemoryRole && transfer.TargetRoleRevision < 1 {
		return ErrWrongOwner
	}
	rows, err := tx.QueryContext(ctx, `SELECT kind,resource_id,role_id,revision,source_id,CAST(body AS BLOB) FROM kernel_device_owned_resources WHERE owner_id=? AND deleted=0 AND ((kind=? AND resource_id=?) OR (?='memory' AND source_id=? AND kind IN ('fact','episodic','vector','graph')))`, s.ownerID, transfer.Kind, transfer.ID, transfer.Kind, transfer.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	expected := map[string]int64{}
	originals := map[string]json.RawMessage{}
	sources := map[string]string{}
	for rows.Next() {
		var kind, id, role, source string
		var revision int64
		var raw json.RawMessage
		if err := rows.Scan(&kind, &id, &role, &revision, &source, &raw); err != nil {
			return err
		}
		if role != StorageRole(commit.Scope, kind) {
			return ErrWrongOwner
		}
		if kind == "continuity" {
			var document struct {
				Lease *struct {
					State string `json:"state"`
				} `json:"lease"`
			}
			if json.Unmarshal(raw, &document) != nil || document.Lease != nil && (document.Lease.State == "running" || document.Lease.State == "unknown") {
				return ErrScopeExpired
			}
		}
		expected[kind+"/"+id] = revision
		originals[kind+"/"+id] = raw
		sources[kind+"/"+id] = source
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, ok := expected[transfer.Kind+"/"+transfer.ID]; !ok || len(expected) != len(commit.Mutations) {
		return ErrResourceVersion
	}
	for _, mutation := range commit.Mutations {
		key := mutation.Kind + "/" + mutation.ID
		revision, ok := expected[key]
		if !ok || mutation.ExpectedRevision != revision || mutation.Deleted || mutation.RoleID != transfer.ToRoleID || mutation.SourceID != sources[key] {
			return ErrResourceVersion
		}
		var original, changed map[string]json.RawMessage
		if json.Unmarshal(originals[key], &original) != nil || json.Unmarshal(mutation.Body, &changed) != nil {
			return ErrWrongOwner
		}
		delete(changed, "memoryPartition")
		delete(changed, "previousMemoryPartition")
		delete(original, "memoryPartition")
		delete(original, "previousMemoryPartition")
		if mutation.Kind == "continuity" {
			var before, after map[string]json.RawMessage
			if json.Unmarshal(original["thread"], &before) != nil || json.Unmarshal(changed["thread"], &after) != nil {
				return ErrWrongOwner
			}
			var nextRevision int64
			if json.Unmarshal(after["revision"], &nextRevision) != nil || nextRevision != revision+1 {
				return ErrResourceVersion
			}
			delete(before, "revision")
			delete(after, "revision")
			original["thread"], _ = json.Marshal(before)
			changed["thread"], _ = json.Marshal(after)
		}
		before, _ := json.Marshal(original)
		after, _ := json.Marshal(changed)
		if string(before) != string(after) {
			return ErrWrongOwner
		}
		delete(expected, key)
	}
	if transfer.Kind == "memory" {
		var root map[string]json.RawMessage
		if err := json.Unmarshal(originals["memory/"+transfer.ID], &root); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		var key string
		if err := json.Unmarshal(root["key"], &key); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if key != "" {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='memory' AND role_id=? AND resource_id<>? AND deleted=0 AND json_extract(CAST(body AS TEXT),'$.key')=?`, s.ownerID, transfer.ToRoleID, transfer.ID, key).Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				return ErrRequestConflict
			}
		}
	}
	return nil
}
