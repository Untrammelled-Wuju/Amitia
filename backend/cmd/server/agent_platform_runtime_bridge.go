package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/workspace"
)

type agentPlatformRuntimeBridge struct {
	platform  *agentplatform.Service
	workspace *workspace.Service
	entry     *interaction.UnifiedEntry
	broker    *execution.ApprovalBroker

	mu       sync.Mutex
	leases   map[string][]agentplatform.ResourceLease
	acquired map[string]bool
}

type agentPlatformWorkspaceRegistrar struct {
	workspace *workspace.Service
}

func newAgentPlatformRuntimeBridge(platform *agentplatform.Service, workspaceService *workspace.Service, entry *interaction.UnifiedEntry, broker *execution.ApprovalBroker) *agentPlatformRuntimeBridge {
	return &agentPlatformRuntimeBridge{
		platform: platform, workspace: workspaceService, entry: entry, broker: broker,
		leases: map[string][]agentplatform.ResourceLease{}, acquired: map[string]bool{},
	}
}

func (b *agentPlatformRuntimeBridge) ExecuteEvaluationCase(ctx context.Context, spaceID, agentID, revisionID string, evaluationCase agentplatform.EvaluationCase) (agentplatform.SubmitEvaluationResult, error) {
	detail, err := b.platform.GetAgent(ctx, spaceID, agentID)
	if err != nil {
		return agentplatform.SubmitEvaluationResult{}, err
	}
	if detail.Profile.Status != "active" || detail.Revision.ID != revisionID {
		return agentplatform.SubmitEvaluationResult{}, errors.New("自动评估只能运行当前已发布的 Agent 版本")
	}
	if b.entry == nil {
		return agentplatform.SubmitEvaluationResult{}, errors.New("Agent 评估入口不可用")
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(evaluationCase.InputJSON), &input); err != nil {
		return agentplatform.SubmitEvaluationResult{}, fmt.Errorf("解析评估输入: %w", err)
	}
	message := runtimeFirstNonEmpty(stringAny(input["message"]), stringAny(input["input"]))
	if message == "" {
		encoded, marshalErr := json.Marshal(input)
		if marshalErr != nil {
			return agentplatform.SubmitEvaluationResult{}, marshalErr
		}
		message = string(encoded)
	}
	startedAt := time.Now()
	runID := capability.NewInvocationID()
	result, runErr := b.entry.Handle(ctx, &interaction.UnifiedEntryRequest{
		Channel: "evaluation", Source: "runtime", SpaceID: spaceID,
		CharacterID: detail.Profile.CharacterID, AgentID: agentID,
		ConversationID: "agent-evaluation-" + runID, RequestID: runID,
		Message: message, PermissionMode: "request_approval",
	})
	completedAt := time.Now().UTC()
	latency := completedAt.Sub(startedAt).Milliseconds()
	if runErr != nil {
		return agentplatform.SubmitEvaluationResult{
			CaseID: evaluationCase.ID, Passed: false, Score: 0, LatencyMS: latency,
			ToolTrace: map[string]any{"error": runErr.Error()}, Failure: runErr.Error(),
		}, nil
	}
	reply := ""
	outcome := ""
	interactionID := ""
	if result != nil {
		outcome = string(result.Outcome)
		interactionID = result.InteractionID
		if result.Response != nil {
			reply = result.Response.Reply
		}
	}
	var expected map[string]any
	if err := json.Unmarshal([]byte(evaluationCase.ExpectedJSON), &expected); err != nil {
		return agentplatform.SubmitEvaluationResult{}, fmt.Errorf("解析评估期望: %w", err)
	}
	passed, score, failure := scoreEvaluationReply(reply, expected)
	usageTotals, err := b.platform.ModelUsageTotalsByWindow(ctx, spaceID, detail.Profile.CharacterID, startedAt, completedAt)
	if err != nil {
		return agentplatform.SubmitEvaluationResult{}, err
	}
	return agentplatform.SubmitEvaluationResult{
		CaseID: evaluationCase.ID, Passed: passed, Score: score, LatencyMS: latency,
		Tokens: usageTotals.TotalTokens, CostMicroUSD: usageTotals.CostMicroUSD,
		ToolTrace: map[string]any{"interactionId": interactionID, "outcome": outcome, "reply": reply},
		Failure:   failure,
	}, nil
}

func (r agentPlatformWorkspaceRegistrar) RegisterRuntimeWorkspace(ctx context.Context, agentID, name, localRoot string) (agentplatform.RuntimeWorkspace, error) {
	if r.workspace == nil {
		return agentplatform.RuntimeWorkspace{}, errors.New("工作区服务不可用")
	}
	mount, err := r.workspace.RegisterLocalMount(ctx, name, localRoot, false)
	if err != nil {
		return agentplatform.RuntimeWorkspace{}, err
	}
	return agentplatform.RuntimeWorkspace{ID: string(mount.ID), Name: mount.Name, RootURI: mount.RootURI}, nil
}

func (b *agentPlatformRuntimeBridge) ResolveAgentRuntime(ctx context.Context, spaceID, characterID string) (interaction.AgentRuntimeProfile, bool, error) {
	profile, found, err := b.platform.ResolveAgentRuntime(ctx, spaceID, characterID)
	if err != nil || !found {
		return interaction.AgentRuntimeProfile{}, found, err
	}
	return interaction.AgentRuntimeProfile{
		AgentID: profile.AgentID, TeamID: profile.TeamID, RuntimeID: profile.RuntimeID,
		ModelConfigID: profile.ModelConfigID, ReasoningEffort: profile.ReasoningEffort,
		PermissionMode: profile.PermissionMode, WorkspaceID: profile.WorkspaceID,
		WorkspaceRootURI: profile.WorkspaceRootURI, Skills: profile.Skills, Tools: profile.Tools,
	}, true, nil
}

func (b *agentPlatformRuntimeBridge) BeforeToolExecution(ctx context.Context, inv capability.ToolInvocationContext, tool capability.ToolDefinition, input []byte) error {
	profile, found, err := b.platform.ResolveAgentRuntime(ctx, inv.SpaceID, inv.CharacterID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if len(profile.Tools) > 0 && !agentAllowsTool(profile.Tools, tool) {
		return fmt.Errorf("Agent 工具白名单不允许执行 %s", runtimeFirstNonEmpty(tool.ModelName, tool.Name, tool.ID))
	}
	if len(profile.Skills) > 0 && (tool.ModelName == "agent_skill_activate" || strings.Contains(tool.ID, "agent_skill")) {
		var payload map[string]any
		if err := json.Unmarshal(input, &payload); err != nil {
			return fmt.Errorf("解析 Agent 技能激活参数: %w", err)
		}
		skillName := runtimeFirstNonEmpty(stringAny(payload["name"]), stringAny(payload["skill"]), stringAny(payload["skillId"]))
		if skillName != "" && !containsFold(profile.Skills, skillName) {
			return fmt.Errorf("Agent 技能白名单不允许激活 %s", skillName)
		}
	}
	if err := b.acquireResourceLeases(ctx, inv, tool, input, profile); err != nil {
		return err
	}
	if err := b.enforceAutonomy(ctx, inv, tool, input, profile); err != nil {
		return err
	}
	return nil
}

func (b *agentPlatformRuntimeBridge) AfterToolExecution(ctx context.Context, inv capability.ToolInvocationContext, tool capability.ToolDefinition, result capability.UnifiedToolResult, durationMS int64) {
	b.releaseRuntimeLeases(ctx, inv.InvocationID)
	profile, found, err := b.platform.ResolveAgentRuntime(ctx, inv.SpaceID, inv.CharacterID)
	if err != nil {
		log.Printf("[original-error] %v", err)
		return
	}
	state := "completed"
	if result.Status == capability.ToolResultStatusFailed {
		state = "failed"
	} else if result.Status == capability.ToolResultStatusCancelled {
		state = "cancelled"
	} else if result.Status == capability.ToolResultStatusTimedOut {
		state = "timed_out"
	}
	detail := ""
	if result.Error != nil {
		detail = result.Error.Error()
	}
	event := agentplatform.ActivityEvent{
		SpaceID: inv.SpaceID, ConversationID: inv.ConversationID, Kind: "tool.execution",
		State: state, Title: runtimeFirstNonEmpty(tool.ModelName, tool.Name), Detail: detail,
		DurationMS: durationMS, OccurredAt: time.Now().UTC(), PayloadJSON: resultActivityJSON(inv, result),
	}
	if found {
		event.AgentID = profile.AgentID
		event.TeamID = profile.TeamID
		event.RuntimeID = profile.RuntimeID
	}
	if err := b.platform.RecordExecutionActivity(ctx, event); err != nil {
		log.Printf("[original-error] %v", err)
	}
}

func (b *agentPlatformRuntimeBridge) acquireResourceLeases(ctx context.Context, inv capability.ToolInvocationContext, tool capability.ToolDefinition, input []byte, profile agentplatform.ResolvedAgentRuntime) error {
	if !tool.HasSideEffects {
		return nil
	}
	resourceType, resourceID := resourceIdentity(tool, input)
	if resourceType == "" || resourceID == "" {
		return nil
	}
	lease, err := b.platform.AcquireResourceLease(ctx, inv.SpaceID, agentplatform.AcquireResourceLeaseInput{
		AgentID: profile.AgentID, ResourceType: resourceType, ResourceID: resourceID,
		TaskID: runtimeFirstNonEmpty(inv.ExternalCallID, inv.InvocationID), Mode: "exclusive", TTLSeconds: 300,
	})
	if err != nil {
		return fmt.Errorf("Agent 资源锁失败: %w", err)
	}
	b.mu.Lock()
	b.leases[inv.InvocationID] = append(b.leases[inv.InvocationID], lease)
	b.mu.Unlock()
	return nil
}

func (b *agentPlatformRuntimeBridge) enforceAutonomy(ctx context.Context, inv capability.ToolInvocationContext, tool capability.ToolDefinition, input []byte, profile agentplatform.ResolvedAgentRuntime) error {
	if inv.Source != capability.InvocationSourceScheduledTask {
		return nil
	}
	policy, err := b.platform.GetAutonomyPolicy(ctx, inv.SpaceID, profile.AgentID)
	if err != nil {
		return err
	}
	hasSideEffect := tool.HasSideEffects || tool.SideEffect != capability.SideEffectNone && tool.SideEffect != capability.SideEffectReadOnly
	if hasSideEffect && withinQuietHours(time.Now(), policy.QuietHours) {
		return fmt.Errorf("Agent 处于安静时段 %s，禁止自动执行有副作用的动作", policy.QuietHours)
	}
	if policy.WatchOnly && hasSideEffect {
		return errors.New("Agent 自治策略当前为仅观察，禁止执行有副作用的动作")
	}
	var todayCount int64
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	count, err := b.platform.CountActivity(ctx, inv.SpaceID, profile.AgentID, "tool.execution", dayStart)
	if err != nil {
		return err
	}
	todayCount = count
	if todayCount >= int64(policy.MaxActionsPerDay) {
		return fmt.Errorf("Agent 今日自动动作已达到上限 %d", policy.MaxActionsPerDay)
	}
	if policy.ExternalApproval && (tool.SideEffect == capability.SideEffectExternal || tool.SideEffect == capability.SideEffectFinancial || tool.SideEffect == capability.SideEffectDestructive) {
		approved, err := b.broker.Await(ctx, execution.ApprovalRequest{
			SpaceID: inv.SpaceID, AgentID: profile.AgentID, TeamID: profile.TeamID,
			CharacterID: inv.CharacterID, ConversationID: inv.ConversationID,
			ToolCallID: runtimeFirstNonEmpty(inv.ExternalCallID, inv.InvocationID) + ":autonomy",
			ToolName:   tool.Name, Arguments: string(input), RiskLevel: string(tool.RiskLevel),
		}, 5*time.Minute)
		if err != nil {
			return err
		}
		if !approved {
			return errors.New("用户拒绝 Agent 自主外部动作")
		}
	}
	return nil
}

func withinQuietHours(now time.Time, value string) bool {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 {
		return false
	}
	parse := func(raw string) (int, bool) {
		parsed, err := time.Parse("15:04", strings.TrimSpace(raw))
		if err != nil {
			return 0, false
		}
		return parsed.Hour()*60 + parsed.Minute(), true
	}
	start, okStart := parse(parts[0])
	end, okEnd := parse(parts[1])
	if !okStart || !okEnd {
		return false
	}
	current := now.Hour()*60 + now.Minute()
	if start == end {
		return false
	}
	if start < end {
		return current >= start && current < end
	}
	return current >= start || current < end
}

func (b *agentPlatformRuntimeBridge) releaseRuntimeLeases(ctx context.Context, invocationID string) {
	b.mu.Lock()
	resourceLeases := append([]agentplatform.ResourceLease(nil), b.leases[invocationID]...)
	delete(b.leases, invocationID)
	b.mu.Unlock()
	for _, lease := range resourceLeases {
		if _, err := b.platform.ReleaseResourceLease(ctx, lease.SpaceID, lease.ID, lease.HolderAgentID); err != nil {
			log.Printf("[original-error] %v", err)
		}
	}
}

func resourceIdentity(tool capability.ToolDefinition, input []byte) (string, string) {
	var payload map[string]any
	if err := json.Unmarshal(input, &payload); err != nil {
		return "", ""
	}
	resourceType := runtimeFirstNonEmpty(stringAny(payload["resourceType"]), agentStringMetadata(tool.Metadata, "resourceType"))
	resourceID := runtimeFirstNonEmpty(stringAny(payload["resourceId"]), stringAny(payload["resourceID"]), agentStringMetadata(tool.Metadata, "resourceId"))
	if resourceType == "" {
		if credentialRef := stringAny(payload["credentialRef"]); credentialRef != "" {
			resourceType = "credential"
			resourceID = credentialRef
		}
	}
	return resourceType, resourceID
}

func agentAllowsTool(allowed []string, tool capability.ToolDefinition) bool {
	for _, item := range allowed {
		item = strings.TrimSpace(item)
		if item == "*" || item == tool.ID || item == tool.Name || item == tool.ModelName || item == string(tool.CapabilityID) {
			return true
		}
	}
	return false
}

func containsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}

func resultActivityJSON(inv capability.ToolInvocationContext, result capability.UnifiedToolResult) string {
	body, err := json.Marshal(map[string]any{
		"invocationId": inv.InvocationID, "toolId": result.ToolID,
		"status": result.Status, "runtimeSessionId": result.RuntimeSessionID,
		"error": result.Error,
	})
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(body)
}

func scoreEvaluationReply(reply string, expected map[string]any) (bool, float64, string) {
	checks := 0
	passed := 0
	failures := make([]string, 0)
	check := func(name string, ok bool) {
		checks++
		if ok {
			passed++
		} else {
			failures = append(failures, name)
		}
	}
	if value := stringAny(expected["equals"]); value != "" {
		check("equals", strings.TrimSpace(reply) == strings.TrimSpace(value))
	}
	if value := stringAny(expected["contains"]); value != "" {
		check("contains", strings.Contains(reply, value))
	}
	if value := stringAny(expected["notContains"]); value != "" {
		check("notContains", !strings.Contains(reply, value))
	}
	if patterns, ok := expected["containsAll"].([]any); ok {
		for _, item := range patterns {
			value := stringAny(item)
			if value != "" {
				check("containsAll:"+value, strings.Contains(reply, value))
			}
		}
	}
	if checks == 0 {
		ok := strings.TrimSpace(reply) != ""
		if ok {
			return true, 1, ""
		}
		return false, 0, "Agent 未返回文本"
	}
	score := float64(passed) / float64(checks)
	return passed == checks, score, strings.Join(failures, ", ")
}

func stringAny(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func agentStringMetadata(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	return stringAny(values[key])
}

func runtimeFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
