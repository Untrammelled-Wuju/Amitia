package permission

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PermissionSnapshot struct {
	SnapshotID     string      `json:"snapshotId"`
	SessionID      string      `json:"sessionId"`
	ExtensionID    string      `json:"extensionId"`
	ModuleID       string      `json:"moduleId"`
	Generation     int64       `json:"generation"`
	CharacterID    string      `json:"characterId"`
	ConversationID string      `json:"conversationId"`
	ResourceIDs    []string    `json:"resourceIds,omitempty"`
	GrantedPerms   []string    `json:"grantedPerms,omitempty"`
	GrantedScopes  []string    `json:"grantedScopes,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	ExpiresAt      *time.Time  `json:"expiresAt,omitempty"`
	RevokedAt      *time.Time  `json:"revokedAt,omitempty"`
}

type PermissionSnapshotRequest struct {
	SessionID      string
	ExtensionID    string
	ModuleID       string
	Generation     int64
	CharacterID    string
	ConversationID string
	ResourceIDs    []string
	GrantedPerms   []string
	GrantedScopes  []string
	Lifetime       time.Duration
}

func NewPermissionSnapshot(req PermissionSnapshotRequest) PermissionSnapshot {
	now := time.Now().UTC()
	snap := PermissionSnapshot{
		SnapshotID:     "psnap-" + uuid.NewString(),
		SessionID:      req.SessionID,
		ExtensionID:    req.ExtensionID,
		ModuleID:       req.ModuleID,
		Generation:     req.Generation,
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		ResourceIDs:    req.ResourceIDs,
		GrantedPerms:   req.GrantedPerms,
		GrantedScopes:  req.GrantedScopes,
		CreatedAt:      now,
	}
	if req.Lifetime > 0 {
		exp := now.Add(req.Lifetime)
		snap.ExpiresAt = &exp
	}
	return snap
}

func (s PermissionSnapshot) IsExpired(now time.Time) bool {
	if s.ExpiresAt == nil {
		return false
	}
	return now.After(*s.ExpiresAt)
}

func (s PermissionSnapshot) IsRevoked() bool {
	return s.RevokedAt != nil
}

func (s PermissionSnapshot) IsValid(now time.Time) bool {
	return !s.IsExpired(now) && !s.IsRevoked()
}

func (s PermissionSnapshot) HasPermission(permID string) bool {
	for _, p := range s.GrantedPerms {
		if p == permID {
			return true
		}
	}
	return false
}

func (s PermissionSnapshot) VerifyIdentity(extID, modID string, generation int64) error {
	if s.ExtensionID != "" && s.ExtensionID != extID {
		return fmt.Errorf("permission snapshot extension %s does not match caller %s", s.ExtensionID, extID)
	}
	if modID != "" && s.ModuleID != "" && s.ModuleID != modID {
		return fmt.Errorf("permission snapshot module %s does not match caller %s", s.ModuleID, modID)
	}
	if generation > 0 && s.Generation != generation {
		return fmt.Errorf("permission snapshot generation %d does not match caller generation %d", s.Generation, generation)
	}
	return nil
}
