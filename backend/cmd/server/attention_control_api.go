package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/agentpermission"
	"github.com/u-ai/backend/internal/attention"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/interaction"
)

type attentionTimelineEvent struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Title      string         `json:"title"`
	Summary    string         `json:"summary,omitempty"`
	Status     string         `json:"status,omitempty"`
	Source     string         `json:"source"`
	SourceID   string         `json:"sourceId,omitempty"`
	RequestID  string         `json:"requestId,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
	Details    map[string]any `json:"details,omitempty"`
}

func registerAttentionControlRoutes(group *gin.RouterGroup, services *AppServices, defaultSpaceID string) {
	group.GET("/attention/items", func(c *gin.Context) {
		spaceID, actor, err := attentionActor(c, defaultSpaceID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		items, err := collectAttentionItems(c.Request.Context(), services, spaceID, actor.DeviceID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": items})
	})
	group.GET("/attention/budget", func(c *gin.Context) {
		spaceID, _, err := attentionActor(c, defaultSpaceID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		stats, err := services.Attention.Stats(c.Request.Context(), spaceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": stats})
	})
	group.PUT("/attention/budget", func(c *gin.Context) {
		spaceID, _, err := attentionActor(c, defaultSpaceID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		var policy attention.BudgetPolicy
		if err := c.ShouldBindJSON(&policy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
			return
		}
		policy.SpaceID = spaceID
		saved, err := services.Attention.PutPolicy(c.Request.Context(), policy)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": saved})
	})
	group.GET("/attention/timeline", func(c *gin.Context) {
		spaceID, _, err := attentionActor(c, defaultSpaceID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		events, err := collectAttentionTimeline(c.Request.Context(), services, spaceID, c.Query("requestId"), c.Query("threadId"), c.Query("coordinationId"), c.Query("approvalId"), c.Query("workflowRunId"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": events})
	})
	group.GET("/agent-access/policies", func(c *gin.Context) {
		spaceID, _, err := attentionActor(c, defaultSpaceID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		policies, err := services.AccessPolicy.List(c.Request.Context(), spaceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": policies})
	})
	group.PUT("/agent-access/policies", func(c *gin.Context) {
		spaceID, _, err := attentionActor(c, defaultSpaceID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		var policy agentpermission.SourcePolicy
		if err := c.ShouldBindJSON(&policy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
			return
		}
		policy.SpaceID = spaceID
		saved, err := services.AccessPolicy.Upsert(c.Request.Context(), policy)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": saved})
	})
	group.DELETE("/agent-access/policies/:id", func(c *gin.Context) {
		spaceID, _, err := attentionActor(c, defaultSpaceID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": err.Error()})
			return
		}
		revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
		if err != nil || revision < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "revision must be a positive integer"})
			return
		}
		if err := services.AccessPolicy.Delete(c.Request.Context(), spaceID, c.Param("id"), revision); err != nil {
			c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict, "msg": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "msg": "ok", "data": gin.H{"id": c.Param("id")}})
	})
}

func attentionActor(c *gin.Context, defaultSpaceID string) (string, *auth.ActorContext, error) {
	actor, ok := auth.FromContext(c.Request.Context())
	if !ok || actor == nil {
		return "", nil, errors.New("unauthorized: missing actor context")
	}
	spaceID := actor.SpaceID.String()
	if spaceID == "" {
		spaceID = strings.TrimSpace(defaultSpaceID)
	}
	if spaceID == "" {
		return "", nil, errors.New("unauthorized: missing space identity")
	}
	return spaceID, actor, nil
}

func collectAttentionItems(ctx context.Context, services *AppServices, spaceID, deviceID string) ([]attention.Item, error) {
	items := make([]attention.Item, 0)
	if services.KernelContainer != nil && services.KernelContainer.ApprovalBroker != nil {
		for _, approval := range services.KernelContainer.ApprovalBroker.List(spaceID, "") {
			expires, _ := time.Parse(time.RFC3339Nano, approval.ExpiresAt)
			created, _ := time.Parse(time.RFC3339Nano, approval.CreatedAt)
			items = append(items, attention.Item{
				ID: "agent-approval:" + approval.ID, Kind: "agent_approval", Decision: "pending", Priority: 100, RequiresAction: true,
				Title: "工具执行需要批准", Summary: approval.ToolName, Reason: "敏感工具调用正在等待你的决定",
				SpaceID: spaceID, ConversationID: approval.ConversationID, RequestID: approval.RequestID, ApprovalID: approval.ID, TurnID: approval.TurnID,
				Route: "/chat?conversationId=" + approval.ConversationID, CreatedAt: created, ExpiresAt: &expires,
				Payload: map[string]any{"toolName": approval.ToolName, "arguments": approval.Arguments, "riskLevel": approval.RiskLevel},
			})
		}
	}
	if services.KernelContainer != nil && services.KernelContainer.TaskRuntimeService != nil {
		for _, approval := range services.KernelContainer.TaskRuntimeService.ListSourceTaskApprovals() {
			if approval.Status != "pending" || approval.Binding.Scope.SpaceID != spaceID {
				continue
			}
			expires := approval.ExpiresAt
			conversationID := ""
			roleID := ""
			if services.DB != nil && approval.Binding.Scope.RequestID != "" {
				var interactionRow interaction.InteractionRecordModel
				if lookupErr := services.DB.WithContext(ctx).Where("space_id = ? AND request_id = ?", spaceID, approval.Binding.Scope.RequestID).Order("created_at DESC").First(&interactionRow).Error; lookupErr == nil {
					conversationID = interactionRow.ConversationID
					roleID = interactionRow.CharacterID
				}
			}
			items = append(items, attention.Item{
				ID: "task-approval:" + approval.ID, Kind: "task_approval", Decision: "pending", Priority: 98, RequiresAction: true,
				Title: "设备任务需要批准", Summary: approval.Binding.Target.TaskID, Reason: "设备资源审批正在等待你的决定",
				SpaceID: spaceID, RoleID: roleID, ConversationID: conversationID, RequestID: approval.Binding.Scope.RequestID, ApprovalID: approval.ID,
				Route: "/kernel/tasks", CreatedAt: approval.CreatedAt, ExpiresAt: &expires,
				Payload: map[string]any{"binding": approval.Binding, "revision": approval.Revision},
			})
		}
	}
	if services.OwnedBusiness != nil {
		requestID := "attention-list-" + uuid.NewString()
		targetDevice := deviceID
		if targetDevice == "" {
			targetDevice = services.LocalDeviceID
		}
		baseRequest := business.Request{SpaceID: spaceID, DeviceID: targetDevice, TargetDeviceID: targetDevice, CoreID: spaceID, RequestID: requestID}
		roles, err := services.OwnedBusiness.ContinuityRoleIDs(ctx, baseRequest)
		if err != nil {
			items = append(items, attention.Item{ID: "source-error:continuity", Kind: "source_error", Decision: "degraded", Priority: 10, Title: "持续事项读取失败", Summary: err.Error(), Reason: "原始错误保留在摘要中", SpaceID: spaceID, CreatedAt: time.Now().UTC()})
		} else {
			seenDocuments := map[string]bool{}
			for _, roleID := range roles {
				roleRequest := baseRequest
				roleRequest.RoleID = roleID
				roleRequest.RequestID = "attention-role-" + uuid.NewString()
				documents, _, listErr := services.OwnedBusiness.ListContinuity(ctx, roleRequest, true)
				if listErr != nil {
					items = append(items, attention.Item{ID: "source-error:continuity:" + roleID, Kind: "source_error", Decision: "degraded", Priority: 10, Title: "持续事项读取失败", Summary: listErr.Error(), Reason: "角色 " + roleID + " 的原始错误保留在摘要中", SpaceID: spaceID, RoleID: roleID, CreatedAt: time.Now().UTC()})
					continue
				}
				for _, document := range documents {
					if seenDocuments[document.Thread.ID] {
						continue
					}
					seenDocuments[document.Thread.ID] = true
					if document.Thread.Status == "waiting" || document.Thread.Status == "blocked" || document.Thread.Status == "paused" {
						summary := document.Thread.NextAction
						if summary == "" {
							summary = document.Thread.CurrentState
						}
						conversationID := ""
						for _, binding := range document.Bindings {
							if binding.BindingType == "conversation" {
								conversationID = binding.BindingID
								break
							}
						}
						items = append(items, attention.Item{
							ID: "continuity:" + document.Thread.ID, Kind: "continuity", Decision: string(document.Thread.Status), Priority: 85, RequiresAction: true,
							Title: firstNonEmptyText(document.Thread.Title, "持续事项需要处理"), Summary: summary, Reason: "持续事项当前处于" + string(document.Thread.Status),
							SpaceID: spaceID, RoleID: document.Thread.CharacterID, ConversationID: conversationID, ThreadID: document.Thread.ID, Route: "/continuity?threadId=" + document.Thread.ID,
							CreatedAt: document.Thread.CreatedAt, Payload: map[string]any{"waits": document.Waits},
						})
					}
				}
			}
		}
	}
	if services.DB != nil {
		var rows []interaction.InteractionRecordModel
		if err := services.DB.WithContext(ctx).Where("space_id = ? AND recovery_descriptor_json <> ''", spaceID).Order("updated_at DESC").Limit(256).Find(&rows).Error; err != nil {
			items = append(items, attention.Item{ID: "source-error:tasks", Kind: "source_error", Decision: "degraded", Priority: 9, Title: "任务读取失败", Summary: err.Error(), Reason: "原始错误保留在摘要中", SpaceID: spaceID, CreatedAt: time.Now().UTC()})
			rows = nil
		}
		for _, row := range rows {
			descriptor, err := interaction.DescriptorFromJSON([]byte(row.RecoveryDescriptorJSON))
			if err != nil || descriptor == nil || descriptor.MultiAgent == nil {
				continue
			}
			ref := descriptor.MultiAgent
			kind := "task_result"
			priority := 30
			requiresAction := false
			reason := "任务已有可查看结果"
			if ref.Status == "failed" || ref.Status == "cancelled" || row.Status == string(interaction.InteractionStatusFailed) || row.Status == string(interaction.InteractionStatusInterrupted) {
				kind, priority, requiresAction, reason = "task_failed", 80, true, "任务执行失败或中断，需要检查完整错误和恢复信息"
			} else if ref.Status == "waiting" || ref.Status == "paused" || row.Status == string(interaction.InteractionStatusPaused) {
				kind, priority, requiresAction, reason = "task_blocked", 75, true, "任务正在等待人工处理或恢复"
			} else if ref.Status != "succeeded" || time.Since(row.UpdatedAt) > 7*24*time.Hour {
				continue
			}
			items = append(items, attention.Item{
				ID: "task:" + ref.CoordinationID, Kind: kind, Decision: ref.Status, Priority: priority, RequiresAction: requiresAction,
				Title: firstNonEmptyText(taskAttentionTitle(kind), "Agent 任务"), Summary: firstAssignmentObjective(ref), Reason: reason,
				SpaceID: spaceID, RoleID: row.CharacterID, ConversationID: row.ConversationID, RequestID: row.RequestID,
				CoordinationID: ref.CoordinationID, Route: "/kernel/tasks", CreatedAt: row.UpdatedAt,
				Payload: map[string]any{"interactionId": row.ID, "status": row.Status, "errorCode": row.ErrorCode, "errorMessage": row.ErrorMessage, "resultRef": row.ResultRef, "recovery": ref},
			})
		}
	}
	if services.KernelContainer != nil && services.KernelContainer.WorkflowExecRepo != nil {
		runs, err := services.KernelContainer.WorkflowExecRepo.ListAttentionRuns(ctx, spaceID, time.Now().UTC().Add(-7*24*time.Hour), 50)
		if err != nil {
			items = append(items, attention.Item{ID: "source-error:workflows", Kind: "source_error", Decision: "degraded", Priority: 9, Title: "自动化任务读取失败", Summary: err.Error(), Reason: "原始错误保留在摘要中", SpaceID: spaceID, CreatedAt: time.Now().UTC()})
		} else {
			for _, run := range runs {
				summary := strings.TrimSpace(run.Error)
				for _, step := range run.Steps {
					if strings.TrimSpace(step.Error) != "" {
						summary = step.Error
						break
					}
				}
				kind, title, reason, priority := "automation_failed", "自动化任务需要处理", "自动化运行失败或需要人工接管", 82
				if string(run.Status) == "waiting_confirmation" {
					kind, title, reason, priority = "automation_approval", "自动化任务等待确认", "受控自动化正在等待你的确认", 96
				}
				items = append(items, attention.Item{
					ID: "automation:" + run.ExecutionID, Kind: kind, Decision: string(run.Status), Priority: priority, RequiresAction: true,
					Title: title, Summary: summary, Reason: reason,
					SpaceID: spaceID, RoleID: run.Context.CharacterID, ConversationID: run.Context.ConversationID,
					RequestID: run.Context.InvocationID, WorkflowID: run.WorkflowID, WorkflowRunID: run.ExecutionID,
					Route:     "/extensions/workflows/" + url.PathEscape(run.WorkflowID) + "?runId=" + url.QueryEscape(run.ExecutionID),
					CreatedAt: run.UpdatedAt,
					Payload:   map[string]any{"deviceId": run.Context.DeviceID, "installationId": run.Context.InstallationID},
				})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].Priority > items[j].Priority
	})
	seen := map[string]bool{}
	result := items[:0]
	for _, item := range items {
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		result = append(result, item)
	}
	return result, nil
}

func taskAttentionTitle(kind string) string {
	switch kind {
	case "task_failed":
		return "Agent 任务需要恢复"
	case "task_blocked":
		return "Agent 任务正在等待"
	default:
		return "Agent 任务已有结果"
	}
}

func firstAssignmentObjective(ref *interaction.MultiAgentRecoveryRef) string {
	if ref == nil || len(ref.AssignmentRefs) == 0 {
		return ""
	}
	return ref.AssignmentRefs[0].Objective
}

func firstNonEmptyText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func collectAttentionTimeline(ctx context.Context, services *AppServices, spaceID, requestID, threadID, coordinationID, approvalID, workflowRunID string) ([]attentionTimelineEvent, error) {
	events := make([]attentionTimelineEvent, 0)
	if services.DB != nil && requestID != "" {
		var decisions []agentpermission.DecisionRecord
		if err := services.DB.WithContext(ctx).Where("space_id = ? AND request_id = ?", spaceID, requestID).Find(&decisions).Error; err != nil {
			return nil, fmt.Errorf("读取来源准入记录: %w", err)
		}
		for _, decision := range decisions {
			events = append(events, attentionTimelineEvent{ID: decision.ID, Type: "access.decision", Title: "来源准入", Summary: decision.Reason, Status: decision.Decision, Source: "agent-access", SourceID: decision.PolicyID, RequestID: decision.RequestID, OccurredAt: decision.CreatedAt, Details: map[string]any{"strength": decision.Strength, "triggerMode": decision.TriggerMode, "permissionMode": decision.PermissionMode, "policyRevision": decision.PolicyRevision}})
		}
		var intents []struct {
			ID        string
			ToolName  string
			Status    string
			CreatedAt time.Time
			UpdatedAt time.Time
		}
		if err := services.DB.WithContext(ctx).Table("tool_call_intents").Select("id, tool_name, status, created_at, updated_at").Where("request_id = ?", requestID).Order("created_at").Scan(&intents).Error; err != nil {
			return nil, fmt.Errorf("读取工具调用记录: %w", err)
		}
		for _, intent := range intents {
			events = append(events, attentionTimelineEvent{ID: intent.ID, Type: "tool.intent", Title: "工具调用", Summary: intent.ToolName, Status: intent.Status, Source: "tool-ledger", SourceID: intent.ID, RequestID: requestID, OccurredAt: intent.CreatedAt})
		}
		var results []struct {
			ID        string
			ToolName  string
			Status    string
			Content   string
			ErrorCode string
			CreatedAt time.Time
		}
		if err := services.DB.WithContext(ctx).Table("tool_call_results").Select("id, tool_name, status, content, error_code, created_at").Where("request_id = ?", requestID).Order("created_at").Scan(&results).Error; err != nil {
			return nil, fmt.Errorf("读取工具调用结果: %w", err)
		}
		for _, result := range results {
			events = append(events, attentionTimelineEvent{ID: result.ID, Type: "tool.result", Title: "工具结果", Summary: firstNonEmptyText(result.ErrorCode, result.Content), Status: result.Status, Source: "tool-ledger", SourceID: result.ID, RequestID: requestID, OccurredAt: result.CreatedAt, Details: map[string]any{"toolName": result.ToolName}})
		}
	}
	if services.DB != nil && (coordinationID != "" || requestID != "") {
		var rows []interaction.InteractionRecordModel
		query := services.DB.WithContext(ctx).Where("space_id = ?", spaceID)
		if requestID != "" {
			query = query.Where("request_id = ? OR correlation_id = ? OR causation_id = ?", requestID, coordinationID, requestID)
		} else if coordinationID != "" {
			query = query.Where("correlation_id = ?", coordinationID)
		}
		if err := query.Order("created_at").Limit(256).Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("读取交互记录: %w", err)
		}
		for _, row := range rows {
			events = append(events, attentionTimelineEvent{ID: "interaction:" + row.ID, Type: "interaction", Title: "Agent 执行", Summary: firstNonEmptyText(row.ErrorMessage, row.ResultRef), Status: row.Status, Source: row.Source, SourceID: row.ID, RequestID: row.RequestID, OccurredAt: row.UpdatedAt, Details: map[string]any{"interactionId": row.ID, "conversationId": row.ConversationID, "threadId": row.ThreadID, "errorCode": row.ErrorCode, "cancelReason": row.CancelReason}})
		}
	}
	if threadID != "" && services.OwnedBusiness != nil {
		targetDevice := services.LocalDeviceID
		baseRequest := business.Request{SpaceID: spaceID, DeviceID: targetDevice, TargetDeviceID: targetDevice, CoreID: spaceID, RequestID: "attention-timeline-roles-" + uuid.NewString()}
		roles, err := services.OwnedBusiness.ContinuityRoleIDs(ctx, baseRequest)
		if err != nil {
			return nil, fmt.Errorf("读取持续事项时间线: %w", err)
		}
		for _, roleID := range roles {
			roleRequest := baseRequest
			roleRequest.RoleID = roleID
			roleRequest.RequestID = "attention-timeline-" + uuid.NewString()
			documents, _, listErr := services.OwnedBusiness.ListContinuity(ctx, roleRequest, true)
			if listErr != nil {
				return nil, fmt.Errorf("读取持续事项时间线: %w", listErr)
			}
			for _, document := range documents {
				if document.Thread.ID != threadID {
					continue
				}
				for _, event := range document.Events {
					events = append(events, attentionTimelineEvent{ID: event.ID, Type: "continuity." + event.EventType, Title: "持续事项变化", Summary: event.PayloadJSON, Status: string(document.Thread.Status), Source: event.SourceType, SourceID: event.SourceID, RequestID: event.RequestID, OccurredAt: event.OccurredAt})
				}
				for _, wait := range document.Waits {
					events = append(events, attentionTimelineEvent{ID: "wait:" + wait.ID, Type: "continuity.wait", Title: "等待条件", Summary: wait.Description, Status: string(wait.Status), Source: "continuity", SourceID: wait.ID, OccurredAt: wait.CreatedAt})
				}
				break
			}
		}
	}
	if approvalID != "" && services.KernelContainer != nil && services.KernelContainer.ApprovalBroker != nil {
		for _, approval := range services.KernelContainer.ApprovalBroker.List(spaceID, "") {
			if approval.ID != approvalID {
				continue
			}
			created, _ := time.Parse(time.RFC3339Nano, approval.CreatedAt)
			events = append(events, attentionTimelineEvent{ID: "approval:" + approval.ID, Type: "approval", Title: "工具审批", Summary: approval.ToolName, Status: string(approval.Status), Source: "approval-broker", SourceID: approval.ID, RequestID: approval.RequestID, OccurredAt: created, Details: map[string]any{"arguments": approval.Arguments, "riskLevel": approval.RiskLevel}})
		}
	}
	if approvalID != "" && services.KernelContainer != nil && services.KernelContainer.ObservabilityStore != nil {
		audits, _, auditErr := services.KernelContainer.ObservabilityStore.ListAuditEvents(ctx, observability.AuditFilter{
			ApprovalID: approvalID, Action: "approval.decide", ListOptions: observability.ListOptions{Limit: 64},
		})
		if auditErr != nil {
			return nil, fmt.Errorf("读取审批授权记录: %w", auditErr)
		}
		for _, audit := range audits {
			events = append(events, attentionTimelineEvent{
				ID: audit.AuditID, Type: "approval.decision", Title: "人工授权", Summary: firstNonEmptyText(audit.ActorID, "系统"),
				Status: audit.Decision, Source: "kernel-audit", SourceID: audit.ApprovalID, RequestID: stringMetadata(audit.Metadata, "requestId"),
				OccurredAt: audit.CreatedAt, Details: map[string]any{
					"authorizedBy": audit.ActorID, "authorizedByType": string(audit.ActorType), "principalType": audit.Metadata["principalType"],
					"authMethod": audit.Metadata["authMethod"], "sessionId": audit.Metadata["sessionId"], "riskLevel": audit.RiskLevel,
					"toolName": audit.SubjectID, "resolvedAt": audit.Metadata["resolvedAt"],
				},
			})
		}
	}
	if workflowRunID != "" && services.KernelContainer != nil && services.KernelContainer.WorkflowExecRepo != nil {
		run, err := services.KernelContainer.WorkflowExecRepo.Get(ctx, workflowRunID)
		if err != nil {
			return nil, fmt.Errorf("读取自动化运行: %w", err)
		}
		if run != nil && run.Context.SpaceID == spaceID {
			events = append(events, attentionTimelineEvent{
				ID: "workflow-run:" + run.ExecutionID, Type: "workflow.run", Title: "自动化任务",
				Summary: firstNonEmptyText(run.Error, string(run.Output)), Status: string(run.Status), Source: "workflow",
				SourceID: run.ExecutionID, RequestID: run.Context.InvocationID, OccurredAt: run.UpdatedAt,
				Details: map[string]any{"workflowId": run.WorkflowID, "attempt": run.Attempt, "generation": run.Generation, "deviceId": run.Context.DeviceID, "scheduleId": run.Context.ScheduleID, "triggerId": run.Context.TriggerID},
			})
			steps, stepErr := services.KernelContainer.WorkflowExecRepo.ListStepRuns(ctx, workflowRunID)
			if stepErr != nil {
				return nil, fmt.Errorf("读取自动化步骤: %w", stepErr)
			}
			for _, step := range steps {
				events = append(events, attentionTimelineEvent{
					ID: "workflow-step:" + step.ExecutionID + ":" + step.NodeID, Type: "workflow.step", Title: "自动化步骤",
					Summary: firstNonEmptyText(step.Error, string(step.Output)), Status: step.Status, Source: "workflow",
					SourceID: step.NodeID, OccurredAt: step.StartedAt,
					Details: map[string]any{"attempt": step.Attempt, "deviceId": step.DeviceID, "toolCallId": step.ToolCallID},
				})
			}
			attempts, attemptErr := services.KernelContainer.WorkflowExecRepo.ListStepAttempts(ctx, workflowRunID)
			if attemptErr != nil {
				return nil, fmt.Errorf("读取自动化步骤尝试: %w", attemptErr)
			}
			for _, attempt := range attempts {
				events = append(events, attentionTimelineEvent{
					ID: "workflow-attempt:" + attempt.ExecutionID + ":" + attempt.NodeID + ":" + strconv.Itoa(attempt.Attempt), Type: "workflow.attempt", Title: "自动化步骤尝试",
					Summary: firstNonEmptyText(attempt.Error, string(attempt.Output)), Status: attempt.Status, Source: "workflow",
					SourceID: attempt.NodeID, OccurredAt: attempt.FinishedAt,
					Details: map[string]any{"attempt": attempt.Attempt, "nextBackoffMs": attempt.NextBackoffMS, "deviceId": attempt.DeviceID},
				})
			}
			compensations, compensationErr := services.KernelContainer.WorkflowExecRepo.ListCompensations(ctx, workflowRunID)
			if compensationErr != nil {
				return nil, fmt.Errorf("读取自动化补偿记录: %w", compensationErr)
			}
			for _, compensation := range compensations {
				occurredAt := compensation.UpdatedAt
				if compensation.CompletedAt != nil {
					occurredAt = *compensation.CompletedAt
				}
				events = append(events, attentionTimelineEvent{
					ID: "workflow-compensation:" + workflowRunID + ":" + compensation.NodeID, Type: "workflow.compensation", Title: "自动化补偿",
					Summary: compensation.Error, Status: compensation.Status, Source: "workflow",
					SourceID: compensation.NodeID, OccurredAt: occurredAt,
				})
			}
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].OccurredAt.Before(events[j].OccurredAt) })
	return events, nil
}
