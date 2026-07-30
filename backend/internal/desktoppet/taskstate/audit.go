// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package taskstate

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type AuditRecord struct {
	ID              string                     `json:"id"`
	EntityType      contracts.EntityType       `json:"entityType"`
	EntityID        string                     `json:"entityId"`
	ParentTaskID    string                     `json:"parentTaskId"`
	ExecutionID     string                     `json:"executionId"`
	AttemptID       string                     `json:"attemptId"`
	FromStatus      contracts.LifecycleStatus  `json:"fromStatus"`
	ToStatus        contracts.LifecycleStatus  `json:"toStatus"`
	FromStage       contracts.Stage            `json:"fromStage"`
	ToStage         contracts.Stage            `json:"toStage"`
	ReasonCode      contracts.TransitionReason `json:"reasonCode"`
	ErrorCode       string                     `json:"errorCode"`
	ErrorMessage    string                     `json:"errorMessage"`
	FailureStage    contracts.Stage            `json:"failureStage"`
	ActorType       contracts.ActorType        `json:"actorType"`
	ActorID         string                     `json:"actorId"`
	PreviousVersion int64                      `json:"previousVersion"`
	CurrentVersion  int64                      `json:"currentVersion"`
	Metadata        map[string]any             `json:"metadata"`
	CreatedAt       string                     `json:"createdAt"`
}

func NewAuditID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "ta_" + hex.EncodeToString(b) + time.Now().Format("20060102150405")
}

func (a *AuditRecord) Sanitize() {
	if a.Metadata != nil {
		for k := range a.Metadata {
			switch k {
			case "api_key", "apiKey", "secret", "token", "password", "credential":
				delete(a.Metadata, k)
			}
		}
	}
}
