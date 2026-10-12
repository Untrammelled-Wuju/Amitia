package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/memory"
	"gorm.io/gorm"
	"time"
)

func (p *meshLocalDataPort) TransferLegacyMemory(ctx context.Context, scope coordination.ExecutionScope, input coordination.LegacyMemoryTransferRequest) (coordination.Acknowledgement, error) {
	if scope.ResourceOwnerID != p.ownerID || input.Kind != "memory" && input.Kind != "episodic" || input.ID == "" || !json.Valid(input.ExpectedRecord) {
		return coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	unlock := character.LockRuntimeRole(p.services.DB)
	defer unlock()
	roles, err := p.rolesLocked(ctx, scope)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	actor, err := coordination.ResolveRole(scope.RoleID, roles)
	if err != nil || actor.Revision != scope.RoleRevision {
		return coordination.Acknowledgement{}, coordination.ErrScopeExpired
	}
	if input.TargetRoleID != coordination.GlobalMemoryRole {
		if _, err := coordination.ResolveRole(input.TargetRoleID, roles); err != nil {
			return coordination.Acknowledgement{}, err
		}
	}
	encoded, _ := json.Marshal(input)
	digest := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(digest[:])
	ack := coordination.Acknowledgement{OwnerID: p.ownerID, RequestID: scope.RequestID, Versions: map[string]int64{input.Kind + "/legacy/" + input.ID: 1}}
	err = p.services.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt struct {
			PayloadHash string
			Ack         string
		}
		result := tx.Table("memory_scope_transfers").Where("owner_id=? AND request_id=?", p.ownerID, scope.RequestID).Take(&receipt)
		if result.Error == nil {
			if receipt.PayloadHash != fingerprint {
				return coordination.ErrRequestConflict
			}
			return json.Unmarshal([]byte(receipt.Ack), &ack)
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		var original []byte
		var table string
		var updates map[string]any
		currentPartition := ""
		if input.Kind == "memory" {
			var row memory.Memory
			if err := tx.Where("id=? AND (space_id=? OR space_id='' OR space_id='default')", input.ID, p.legacySpaceID).Take(&row).Error; err != nil {
				return err
			}
			original, _ = json.Marshal(row)
			currentPartition = row.CharacterID
			if row.Scope == "user" || row.Scope == "user_global" {
				currentPartition = coordination.GlobalMemoryRole
			}
			memoryScope := "character"
			characterID := input.TargetRoleID
			if input.TargetRoleID == coordination.GlobalMemoryRole {
				memoryScope = "user"
				characterID = row.CharacterID
			}
			table = "memories"
			updates = map[string]any{"scope": memoryScope, "character_id": characterID, "version": row.Version + 1, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
		} else {
			var row episodic.EpisodicMemory
			if err := tx.Where("id=? AND (space_id=? OR space_id='' OR space_id='default')", input.ID, p.legacySpaceID).Take(&row).Error; err != nil {
				return err
			}
			original, _ = json.Marshal(row)
			currentPartition = row.CharacterID
			if currentPartition == "" {
				currentPartition = coordination.GlobalMemoryRole
			}
			characterID := input.TargetRoleID
			if characterID == coordination.GlobalMemoryRole {
				characterID = ""
			}
			table = "episodic_memories"
			updates = map[string]any{"character_id": characterID, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
		}
		if currentPartition != coordination.StorageRole(scope, input.Kind) || currentPartition == input.TargetRoleID {
			return coordination.ErrScopeExpired
		}
		var before, expected map[string]json.RawMessage
		if json.Unmarshal(original, &before) != nil || json.Unmarshal(input.ExpectedRecord, &expected) != nil {
			return coordination.ErrWrongOwner
		}
		normalized, _ := json.Marshal(before)
		expectedJSON, _ := json.Marshal(expected)
		if string(normalized) != string(expectedJSON) {
			return coordination.ErrResourceVersion
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		if err := tx.Table(table).Where("id=?", input.ID).Updates(updates).Error; err != nil {
			return err
		}
		proof, _ := json.Marshal(ack)
		return tx.Table("memory_scope_transfers").Create(map[string]any{"owner_id": p.ownerID, "request_id": scope.RequestID, "payload_hash": fingerprint, "ack": string(proof), "record_kind": input.Kind, "record_id": input.ID, "original_record": string(original), "created_at": time.Now().UTC().Format(time.RFC3339Nano)}).Error
	})
	if err == nil && input.Kind == "memory" && p.services.Memory != nil {
		p.services.Memory.SyncGraphMemory(input.ID)
		go p.services.Memory.SyncEmbedding(input.ID, "", "", "", "")
	}
	if err == nil && input.Kind == "episodic" && p.services.Episodic != nil {
		p.services.Episodic.SyncGraphEpisodic(input.ID)
	}
	return ack, err
}
