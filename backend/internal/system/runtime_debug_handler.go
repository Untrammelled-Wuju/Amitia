// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/pkg/util"
)

type runtimeDebugInteractionRow struct {
	ID             string    `gorm:"column:id"`
	UserID         string    `gorm:"column:user_id"`
	CharacterID    string    `gorm:"column:character_id"`
	ConversationID string    `gorm:"column:conversation_id"`
	Priority       int       `gorm:"column:priority"`
	PathType       string    `gorm:"column:path_type"`
	Status         string    `gorm:"column:status"`
	StatusVersion  int64     `gorm:"column:status_version"`
	CancelReason   string    `gorm:"column:cancel_reason"`
	DeadlineAt     time.Time `gorm:"column:deadline_at"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

type runtimeDebugDeliveryRow struct {
	Channel     string `gorm:"column:channel"`
	Status      string `gorm:"column:status"`
	RetryCount  int    `gorm:"column:retry_count"`
	LeaseOwner  string `gorm:"column:lease_owner"`
	CreatedAt   string `gorm:"column:created_at"`
	SentAt      string `gorm:"column:sent_at"`
	DeliveredAt string `gorm:"column:delivered_at"`
}

type runtimeDebugToolRow struct {
	ExtensionID    string `gorm:"column:extension_id"`
	SkillID        string `gorm:"column:skill_id"`
	Status         string `gorm:"column:status"`
	IdempotencyKey string `gorm:"column:idempotency_key"`
	FinishedAt     string `gorm:"column:finished_at"`
	CreatedAt      string `gorm:"column:created_at"`
}

// RuntimeDebugSnapshot exposes a read-only snapshot assembled from the canonical
// interaction/delivery/extension stores and the live mind-runtime registries.
// It deliberately leaves optional decision-plan fields absent when the runtime
// has not persisted a plan; the UI must not manufacture decision data.
func (h *Handler) RuntimeDebugSnapshot(c *gin.Context) {
	now := time.Now().UTC()
	runtimeExport := mindruntime.ExportRuntimeSnapshot()
	metrics := mindruntime.DefaultMetricsCollector.Snapshot()

	interactionRows := make([]runtimeDebugInteractionRow, 0)
	if h.db != nil && h.db.Migrator().HasTable("interaction_records") {
		_ = h.db.Table("interaction_records").
			Select("id,user_id,character_id,conversation_id,priority,path_type,status,status_version,cancel_reason,deadline_at,created_at,updated_at").
			Order("updated_at DESC").Limit(100).Scan(&interactionRows).Error
	}

	activeInteractions := 0
	queuedTasks := 0
	interactions := make([]map[string]interface{}, 0, len(interactionRows))
	queueDepth := make(map[string]int)
	queueOldest := make(map[string]time.Time)
	for _, row := range interactionRows {
		status := strings.ToLower(strings.TrimSpace(row.Status))
		if runtimeDebugActiveStatus(status) {
			activeInteractions++
		}
		if status == "queued" {
			queuedTasks++
			queueName := strings.TrimSpace(row.PathType)
			if queueName == "" {
				queueName = "interaction"
			}
			queueDepth[queueName]++
			created := row.CreatedAt
			if previous, ok := queueOldest[queueName]; !ok || (!created.IsZero() && created.Before(previous)) {
				queueOldest[queueName] = created
			}
		}
		scope := runtimeDebugScope(row.UserID, row.CharacterID, row.ConversationID)
		item := map[string]interface{}{
			"scope":        scope,
			"status":       row.Status,
			"priority":     fmt.Sprintf("%d", row.Priority),
			"path":         row.PathType,
			"stateVersion": row.StatusVersion,
			"cancelReason": nullableString(row.CancelReason),
		}
		if !row.DeadlineAt.IsZero() {
			item["deadlineAt"] = row.DeadlineAt.UTC().Format(time.RFC3339Nano)
		}
		interactions = append(interactions, item)
	}

	queues := make([]map[string]interface{}, 0, len(queueDepth))
	for name, depth := range queueDepth {
		oldestAge := int64(0)
		if oldest := queueOldest[name]; !oldest.IsZero() {
			oldestAge = now.Sub(oldest).Milliseconds()
			if oldestAge < 0 {
				oldestAge = 0
			}
		}
		state := "ok"
		if depth >= 20 {
			state = "busy"
		}
		queues = append(queues, map[string]interface{}{
			"name": name, "priority": "runtime", "depth": depth, "oldestAgeMs": oldestAge, "status": state,
		})
	}

	deliveryRows := make([]runtimeDebugDeliveryRow, 0)
	if h.db != nil && h.db.Migrator().HasTable("delivery_intents") {
		_ = h.db.Table("delivery_intents").
			Select("channel,status,retry_count,lease_owner,created_at,sent_at,delivered_at").
			Order("created_at DESC").Limit(100).Scan(&deliveryRows).Error
	}
	deliveries := make([]map[string]interface{}, 0, len(deliveryRows))
	for _, row := range deliveryRows {
		leaseState := "free"
		if strings.TrimSpace(row.LeaseOwner) != "" {
			leaseState = "leased"
		}
		updated := firstNonEmpty(row.DeliveredAt, row.SentAt, row.CreatedAt)
		deliveries = append(deliveries, map[string]interface{}{
			"channel": row.Channel, "leaseState": leaseState, "deliveryState": row.Status,
			"attempt": row.RetryCount + 1, "updatedAt": nullableString(updated),
		})
	}

	toolRows := make([]runtimeDebugToolRow, 0)
	if h.db != nil && h.db.Migrator().HasTable("extension_runs") {
		_ = h.db.Table("extension_runs").
			Select("extension_id,skill_id,status,idempotency_key,finished_at,created_at").
			Order("created_at DESC").Limit(100).Scan(&toolRows).Error
	}
	tools := make([]map[string]interface{}, 0, len(toolRows))
	for _, row := range toolRows {
		name := firstNonEmpty(row.SkillID, row.ExtensionID)
		tools = append(tools, map[string]interface{}{
			"tool": name, "status": row.Status, "idempotencyKey": row.IdempotencyKey,
			"updatedAt": nullableString(firstNonEmpty(row.FinishedAt, row.CreatedAt)),
		})
	}

	circuits := make([]map[string]interface{}, 0)
	for _, report := range mindruntime.DefaultCircuitBreakerRegistry.AllHealthReports() {
		if report.CircuitBreaker == nil {
			continue
		}
		cb := report.CircuitBreaker
		item := map[string]interface{}{
			"dependency": report.Name, "state": string(cb.Status()), "failures": cb.Failures,
		}
		if !cb.OpenedAt.IsZero() {
			item["openedAt"] = cb.OpenedAt.UTC().Format(time.RFC3339Nano)
		}
		circuits = append(circuits, item)
	}

	reconciliation := make([]map[string]interface{}, 0)
	reconciliationIssues := int64(0)
	if h.reconciliation != nil {
		for _, scan := range h.reconciliation.AllScans() {
			open := int64(0)
			severity := "ok"
			for _, diff := range scan.Diffs {
				if diff.Repaired {
					continue
				}
				open++
				severity = runtimeDebugMaxSeverity(severity, diff.Severity)
			}
			reconciliationIssues += open
			updated := scan.StartedAt
			if !scan.EndedAt.IsZero() {
				updated = scan.EndedAt
			}
			reconciliation = append(reconciliation, map[string]interface{}{
				"category": string(scan.Target), "severity": severity, "count": open,
				"strategy": string(scan.Strategy), "updatedAt": updated.UTC().Format(time.RFC3339Nano),
			})
		}
	}
	if metrics.ReconciliationOpenIssues > reconciliationIssues {
		reconciliationIssues = metrics.ReconciliationOpenIssues
	}

	if metrics.ActiveInteractions > int64(activeInteractions) {
		activeInteractions = int(metrics.ActiveInteractions)
	}

	response := map[string]interface{}{
		"meta": map[string]interface{}{
			"generatedAt": now.Format(time.RFC3339Nano),
			"degraded":    !runtimeExport.AllHealthy,
		},
		"summary": map[string]interface{}{
			"activeInteractions":   activeInteractions,
			"queuedTasks":          queuedTasks,
			"reconciliationIssues": reconciliationIssues,
		},
		"interactions":   interactions,
		"budgets":        []interface{}{},
		"queues":         queues,
		"deliveries":     deliveries,
		"tools":          tools,
		"circuits":       circuits,
		"reconciliation": reconciliation,
		"metrics":        metrics,
	}
	util.SuccessResponse(c, response)
}

func runtimeDebugActiveStatus(status string) bool {
	switch status {
	case "received", "normalized", "queued", "processing", "context_ready", "decided", "generated", "committed", "delivery_pending":
		return true
	default:
		return false
	}
}

func runtimeDebugScope(userID, characterID, conversationID string) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(userID) != "" {
		parts = append(parts, "user:"+userID)
	}
	if strings.TrimSpace(characterID) != "" {
		parts = append(parts, "character:"+characterID)
	}
	if strings.TrimSpace(conversationID) != "" {
		parts = append(parts, "conversation:"+conversationID)
	}
	if len(parts) == 0 {
		return "runtime"
	}
	return strings.Join(parts, " / ")
}

func runtimeDebugMaxSeverity(current, candidate string) string {
	rank := map[string]int{"ok": 0, "low": 1, "medium": 2, "high": 3, "critical": 3}
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if _, ok := rank[candidate]; !ok {
		candidate = "medium"
	}
	if rank[candidate] > rank[current] {
		if candidate == "critical" {
			return "high"
		}
		return candidate
	}
	return current
}

func nullableString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
