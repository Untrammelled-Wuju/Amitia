package permission

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PermissionSnapshot struct {
	SnapshotID          string                    `json:"snapshotId"`
	SessionID           string                    `json:"sessionId"`
	ExtensionID         string                    `json:"extensionId"`
	ModuleID            string                    `json:"moduleId"`
	Generation          int64                     `json:"generation"`
	CharacterID         string                    `json:"characterId"`
	ConversationID      string                    `json:"conversationId"`
	ResourceIDs         []string                  `json:"resourceIds,omitempty"`
	GrantedPerms        []string                  `json:"grantedPerms,omitempty"`
	GrantedScopes       []string                  `json:"grantedScopes,omitempty"`
	CreatedAt           time.Time                 `json:"createdAt"`
	ExpiresAt           *time.Time                `json:"expiresAt,omitempty"`
	RevokedAt           *time.Time                `json:"revokedAt,omitempty"`
	ExecutionPlacement  ExecutionPlacement        `json:"executionPlacement,omitempty"`
	UserID              runtimeidentity.UserID    `json:"userId,omitempty"`
	DeviceID            runtimeidentity.DeviceID  `json:"deviceId,omitempty"`
	RuntimeID           runtimeidentity.RuntimeID `json:"runtimeId,omitempty"`
	ProviderID          string                    `json:"providerId,omitempty"`
	ProviderInstanceID  string                    `json:"providerInstanceId,omitempty"`
	ExecutionBindingKey string                    `json:"executionBindingKey,omitempty"`
}

type PermissionIDValidator struct {
	registry *PermissionDefinitionRegistry
}

func NewPermissionIDValidator(registry *PermissionDefinitionRegistry) *PermissionIDValidator {
	return &PermissionIDValidator{registry: registry}
}

func (v *PermissionIDValidator) IsValidPermissionID(id string) bool {
	if v == nil || v.registry == nil {
		return false
	}
	_, ok := v.registry.Get(id)
	return ok
}

func (v *PermissionIDValidator) ValidateAll(ids []string) []string {
	invalid := make([]string, 0)
	for _, id := range ids {
		if !v.IsValidPermissionID(id) {
			invalid = append(invalid, id)
		}
	}
	return invalid
}

type PermissionSnapshotRequest struct {
	SessionID        string
	ExtensionID      string
	ModuleID         string
	Generation       int64
	CharacterID      string
	ConversationID   string
	ResourceIDs      []string
	GrantedPerms     []string
	GrantedScopes    []string
	Lifetime         time.Duration
	ExecutionContext PermissionExecutionContext
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
	if req.ExecutionContext != (PermissionExecutionContext{}) {
		snap.ExecutionPlacement = req.ExecutionContext.Placement
		snap.UserID = req.ExecutionContext.UserID
		snap.DeviceID = req.ExecutionContext.DeviceID
		snap.RuntimeID = req.ExecutionContext.RuntimeID
		snap.ProviderID = req.ExecutionContext.ProviderID
		snap.ProviderInstanceID = req.ExecutionContext.ProviderInstanceID
		snap.ExecutionBindingKey = req.ExecutionContext.StableBindingKey()
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

func (s PermissionSnapshot) VerifyExecutionContext(ctx PermissionExecutionContext) error {
	if s.ExecutionBindingKey != "" {
		if s.ExecutionBindingKey != ctx.StableBindingKey() {
			return ErrPermissionExecutionBindingMismatch
		}
		return nil
	}
	if s.ProviderID != "" && s.ProviderID != ctx.ProviderID {
		return ErrPermissionProviderBindingMismatch
	}
	if s.DeviceID != "" && s.DeviceID != ctx.DeviceID {
		return ErrPermissionDeviceBindingMismatch
	}
	if s.RuntimeID != "" && s.RuntimeID != ctx.RuntimeID {
		return ErrPermissionRuntimeBindingMismatch
	}
	return nil
}

func (s PermissionSnapshot) ValidateGrantedPerms(validator *PermissionIDValidator) error {
	if validator == nil {
		return fmt.Errorf("permission snapshot: validator not configured")
	}
	invalid := validator.ValidateAll(s.GrantedPerms)
	if len(invalid) > 0 {
		return fmt.Errorf("permission snapshot %s contains invalid permission IDs (likely action IDs): %v", s.SnapshotID, invalid)
	}
	return nil
}

func (s PermissionSnapshot) HasActionIDs(validator *PermissionIDValidator) bool {
	if validator == nil {
		return false
	}
	return len(validator.ValidateAll(s.GrantedPerms)) > 0
}

func (s PermissionSnapshot) VerifySession(sessionID string) error {
	if s.SessionID != "" && s.SessionID != sessionID {
		return fmt.Errorf("permission snapshot session %s does not match caller session %s", s.SessionID, sessionID)
	}
	return nil
}
