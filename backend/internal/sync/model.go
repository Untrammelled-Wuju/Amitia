// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"time"
)

type ChangeID string
type Sequence int64
type EntityType string
type EntityID string
type OperationType string
type MutationID string

const (
	OpCreate   OperationType = "create"
	OpUpdate   OperationType = "update"
	OpDelete   OperationType = "delete"
	OpSnapshot OperationType = "snapshot"
)

const (
	MutationClaimStatusPending     = "pending"
	MutationClaimStatusCommitted   = "committed"
	MutationClaimStatusRolledBack  = "rolled_back"
)

type ChangeRecord struct {
	ChangeID     ChangeID      `gorm:"column:change_id;primaryKey"`
	Sequence     Sequence      `gorm:"column:seq;not null;index"`
	UserID       string        `gorm:"column:user_id;not null;index"`
	Scope        CursorScope   `gorm:"column:scope;not null;default:device;index"`
	EntityType   EntityType    `gorm:"column:entity_type;not null;index"`
	EntityID     EntityID      `gorm:"column:entity_id;not null;index"`
	Operation    OperationType `gorm:"column:operation;not null"`
	Revision     int64         `gorm:"column:revision;not null"`
	MutationID   MutationID    `gorm:"column:mutation_id;index"`
	OriginDevice string        `gorm:"column:origin_device;index"`
	Payload      []byte        `gorm:"column:payload"`
	Checksum     string        `gorm:"column:checksum"`
	CreatedAt    time.Time     `gorm:"column:created_at;not null;index"`
}

func (ChangeRecord) TableName() string {
	return "sync_changes"
}

type MutationClaim struct {
	UserID      string       `gorm:"column:user_id;primaryKey"`
	Scope       CursorScope  `gorm:"column:scope;primaryKey;not null;default:device"`
	MutationID  MutationID   `gorm:"column:mutation_id;primaryKey"`
	Status      string       `gorm:"column:status;not null;default:pending;index"`
	CreatedAt   time.Time    `gorm:"column:created_at;not null"`
}

func (MutationClaim) TableName() string {
	return "sync_mutation_claims"
}

type CursorScope string

const (
	ScopeGlobal  CursorScope = "global"
	ScopeDevice  CursorScope = "device"
	ScopeUser    CursorScope = "user"
)

type SyncCursor struct {
	DeviceID    string      `gorm:"column:device_id;primaryKey"`
	UserID      string      `gorm:"column:user_id;primaryKey;not null"`
	Scope       CursorScope `gorm:"column:scope;primaryKey;not null"`
	LastApplied Sequence    `gorm:"column:last_applied;not null;default:0"`
	LastPushed  Sequence    `gorm:"column:last_pushed;not null;default:0"`
	UpdatedAt   time.Time   `gorm:"column:updated_at;not null"`
}

func (SyncCursor) TableName() string {
	return "sync_cursors"
}

type CursorStatus struct {
	DeviceID       string    `json:"deviceId"`
	UserID         string    `json:"userId"`
	LastApplied    Sequence  `json:"lastApplied"`
	LastPushed     Sequence  `json:"lastPushed"`
	ServerSequence Sequence  `json:"serverSequence"`
	Lag            int64     `json:"lag"`
	LastPullAt     time.Time `json:"lastPullAt,omitempty"`
}

type PullRequest struct {
	DeviceID     string `json:"deviceId" binding:"required"`
	UserID       string `json:"userId"`
	LastCursor   Sequence `json:"lastCursor"`
	Limit        int    `json:"limit,omitempty"`
	EntityType   string `json:"entityType,omitempty"`
	Scope        CursorScope `json:"scope,omitempty"`
}

type PullResult struct {
	Changes      []ChangeRecord `json:"changes"`
	NextCursor   Sequence       `json:"nextCursor"`
	HasMore      bool           `json:"hasMore"`
	ServerSequence Sequence     `json:"serverSequence"`
}

type PushRequest struct {
	DeviceID  string           `json:"deviceId" binding:"required"`
	UserID    string           `json:"userId"`
	Mutations []ClientMutation `json:"mutations" binding:"required,min=1"`
}

type ClientMutation struct {
	MutationID   MutationID    `json:"mutationId" binding:"required"`
	EntityType   EntityType    `json:"entityType" binding:"required"`
	EntityID     EntityID      `json:"entityId" binding:"required"`
	Operation    OperationType `json:"operation" binding:"required"`
	BaseRevision int64         `json:"baseRevision"`
	Payload      []byte        `json:"payload,omitempty"`
}

type PushResult struct {
	Accepted    []MutationResult `json:"accepted"`
	Rejected    []MutationResult `json:"rejected"`
	ServerSeq   Sequence         `json:"serverSequence"`
	ApplyCursor Sequence         `json:"applyCursor"`
}

type MutationResult struct {
	MutationID MutationID `json:"mutationId"`
	Success    bool       `json:"success"`
	ChangeID   ChangeID   `json:"changeId,omitempty"`
	Sequence   Sequence   `json:"sequence,omitempty"`
	Revision   int64      `json:"revision,omitempty"`
	ErrorCode  string     `json:"errorCode,omitempty"`
	Message    string     `json:"message,omitempty"`
	ServerRevision int64  `json:"serverRevision,omitempty"`
}

type Conflict struct {
	EntityID     EntityID `json:"entityId"`
	EntityType   EntityType `json:"entityType"`
	BaseRevision int64    `json:"baseRevision"`
	ServerRevision int64  `json:"serverRevision"`
	Resolution   string   `json:"resolution"`
}

type GapReport struct {
	Detected  bool     `json:"detected"`
	FromSeq   Sequence `json:"fromSeq,omitempty"`
	ToSeq     Sequence `json:"toSeq,omitempty"`
	InvalidCursor bool `json:"invalidCursor,omitempty"`
	Message   string   `json:"message,omitempty"`
}
