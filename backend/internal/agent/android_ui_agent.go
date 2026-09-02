package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/androiduiagent"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	workflowmetrics "github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type plannedAndroidUIAction struct {
	Action      string         `json:"action"`
	Reason      string         `json:"reason,omitempty"`
	Target      map[string]any `json:"target,omitempty"`
	Text        string         `json:"text,omitempty"`
	Direction   string         `json:"direction,omitempty"`
	Amount      string         `json:"amount,omitempty"`
	StartX      int            `json:"startX,omitempty"`
	StartY      int            `json:"startY,omitempty"`
	EndX        int            `json:"endX,omitempty"`
	EndY        int            `json:"endY,omitempty"`
	DurationMS  int            `json:"durationMs,omitempty"`
	PackageName string         `json:"packageName,omitempty"`
	Description string         `json:"description,omitempty"`
	Role        string         `json:"role,omitempty"`
	WaitMS      int            `json:"waitMs,omitempty"`
	Result      string         `json:"result,omitempty"`
}

func (s *service) RunAndroidUIAgent(ctx context.Context, execCtx tool.ToolExecutionContext, req androiduiagent.Request) (androiduiagent.Result, error) {
	if s == nil || s.toolFacade == nil {
		return androiduiagent.Result{}, errors.New("canonical ToolFacade is not configured")
	}
	cfg := s.getActiveModel()
	if cfg == nil {
		return androiduiagent.Result{}, errors.New("no active model configuration for Android UI agent")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	scope := extensionkernel.LegacyScope{
		UserID: execCtx.User, CharacterID: execCtx.CharacterID, ConversationID: execCtx.ConversationID,
		Channel: execCtx.Channel, TraceID: execCtx.CorrelationID, RequestID: execCtx.RequestID,
		ToolCallID: execCtx.ToolCallID, CorrelationID: execCtx.CorrelationID, CausationID: execCtx.CausationID,
	}

	result := androiduiagent.Result{Steps: make([]androiduiagent.Step, 0, req.MaxSteps)}
	runKey := firstNonEmpty(strings.TrimSpace(execCtx.IdempotencyKey), strings.TrimSpace(execCtx.ToolCallID), strings.TrimSpace(execCtx.RequestID), strings.TrimSpace(execCtx.CorrelationID))
	if runKey == "" {
		runKey = fmt.Sprintf("uiagent:%d", time.Now().UnixNano())
	}
	loopBlocks := 0
	noProgressSteps := 0
	lastSemanticHash := ""

	for index := 1; index <= req.MaxSteps; index++ {
		if err := ctx.Err(); err != nil {
			result.StepCount = len(result.Steps)
			result.FinalState = "timeout"
			return result, err
		}

		observationRaw, observationSummary, quality, tree, err := s.androidUIObservation(ctx, scope, runKey, index)
		if err != nil {
			result.StepCount = len(result.Steps)
			result.FinalState = "observation_failed"
			return result, err
		}
		beforeHash := androidUISemanticHash(tree)
		action, err := s.planAndroidUIAction(cfg, req, observationRaw, result.Steps)
		if err != nil {
			result.Steps = append(result.Steps, androiduiagent.Step{
				Index: index, Action: "planner", Status: "failed", Observation: observationSummary,
				ObservationQuality: quality.Level, BeforeHash: beforeHash, Error: err.Error(),
			})
			result.StepCount = len(result.Steps)
			result.FinalState = "planner_failed"
			return result, err
		}

		fingerprint := androidUIActionFingerprint(action)
		views := make([]androiduiagentStepView, 0, len(result.Steps))
		for _, item := range result.Steps {
			views = append(views, androiduiagentStepView{ActionFingerprint: item.ActionFingerprint, AfterHash: item.AfterHash})
		}
		repeatCount := repeatedAndroidUIAction(views, fingerprint, beforeHash)
		step := androiduiagent.Step{
			Index: index, Action: action.Action, Reason: strings.TrimSpace(action.Reason), Observation: observationSummary,
			ObservationQuality: quality.Level, ActionFingerprint: fingerprint, BeforeHash: beforeHash,
		}
		if repeatCount >= 2 {
			loopBlocks++
			workflowmetrics.DefaultWorkflowReliabilityMetrics.Inc(workflowmetrics.MetricUIAgentLoopTotal)
			step.Status = "loop_blocked"
			step.VerificationResult = "REPEATED_ACTION_NO_PROGRESS"
			step.Error = "repeated action on an unchanged observation was blocked; choose a different strategy"
			result.Steps = append(result.Steps, step)
			if loopBlocks >= 2 {
				result.Success = false
				result.Result = "Android UI agent stopped after repeated no-progress loops"
				result.FinalState = "needs_user"
				result.StepCount = len(result.Steps)
				return result, nil
			}
			continue
		}

		switch action.Action {
		case "done":
			step.Status = "completed"
			step.VerificationResult = "GOAL_OBSERVED"
			result.Steps = append(result.Steps, step)
			result.Success = true
			result.Result = strings.TrimSpace(action.Result)
			if result.Result == "" {
				result.Result = "Android UI goal completed"
			}
			result.FinalState = "completed"
			result.StepCount = len(result.Steps)
			return result, nil
		case "needs_user":
			step.Status = "needs_user"
			step.Error = strings.TrimSpace(action.Result)
			result.Steps = append(result.Steps, step)
			result.Result = strings.TrimSpace(action.Result)
			result.FinalState = "needs_user"
			result.StepCount = len(result.Steps)
			return result, nil
		case "fail":
			step.Status = "failed"
			step.Error = strings.TrimSpace(action.Result)
			result.Steps = append(result.Steps, step)
			result.Result = strings.TrimSpace(action.Result)
			result.FinalState = "failed"
			result.StepCount = len(result.Steps)
			return result, nil
		case "wait":
			wait := time.Duration(action.WaitMS) * time.Millisecond
			if wait <= 0 {
				wait = 500 * time.Millisecond
			}
			if wait > 5*time.Second {
				wait = 5 * time.Second
			}
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(wait):
			}
			step.Status = "success"
			step.VerificationResult = "WAIT_COMPLETED"
			step.LatencyMS = wait.Milliseconds()
			result.Steps = append(result.Steps, step)
			continue
		}

		action.Target = enrichTargetFromObservation(tree, action.Target)
		workflowmetrics.DefaultWorkflowReliabilityMetrics.Inc(workflowmetrics.MetricUIAgentActionTotal)
		startedAt := time.Now()
		toolID, input, toolResult, found, fallbackLevel := s.executeAndroidUIActionWithRecovery(ctx, scope, req, action, tree, runKey, index)
		step.ToolName = toolID
		step.Input = input
		step.FallbackLevel = fallbackLevel
		step.LatencyMS = time.Since(startedAt).Milliseconds()
		workflowmetrics.DefaultWorkflowReliabilityMetrics.Observe(workflowmetrics.MetricUIAgentActionLatencyMS, float64(step.LatencyMS))
		if fallbackLevel > 0 {
			workflowmetrics.DefaultWorkflowReliabilityMetrics.Inc(workflowmetrics.MetricUIAgentFallbackTotal)
		}
		if !found {
			step.Status = "failed"
			step.Error = "typed Android tool is unavailable: " + toolID
			result.Steps = append(result.Steps, step)
			continue
		}
		if !legacyToolSucceeded(toolResult) {
			step.Status = "failed"
			step.Error = strings.TrimSpace(toolResult.VisibleText)
			if toolResult.Error != nil {
				if step.Error == "" {
					step.Error = toolResult.Error.Message
				}
				if step.Error == "" {
					step.Error = toolResult.Error.Code
				}
			}
			if blocked, reason := androidUIInteractionBlocked(toolResult); blocked {
				step.Status = "needs_user"
				step.VerificationResult = "DEVICE_INTERACTION_BLOCKED"
				if reason != "" {
					step.Error = reason
				}
				result.Steps = append(result.Steps, step)
				result.Success = false
				result.Result = step.Error
				result.FinalState = "needs_user"
				result.StepCount = len(result.Steps)
				return result, nil
			}
			result.Steps = append(result.Steps, step)
			continue
		}

		_, _, afterQuality, afterTree, settleErr := s.waitForAndroidUISettle(ctx, scope, runKey, index)
		if settleErr != nil && !errors.Is(settleErr, context.Canceled) && !errors.Is(settleErr, context.DeadlineExceeded) {
			step.Status = "success"
			step.VerificationResult = "ACTION_ACCEPTED_VERIFICATION_UNAVAILABLE"
			result.Steps = append(result.Steps, step)
			continue
		}
		afterHash := androidUISemanticHash(afterTree)
		step.AfterHash = afterHash
		if afterQuality.Level != "" {
			step.ObservationQuality = quality.Level + "->" + afterQuality.Level
		}
		if afterHash != "" && afterHash != beforeHash {
			step.Status = "success"
			step.VerificationResult = "ACTION_EFFECT_OBSERVED"
			noProgressSteps = 0
		} else {
			step.Status = "no_effect"
			step.VerificationResult = "ACTION_NO_EFFECT"
			workflowmetrics.DefaultWorkflowReliabilityMetrics.Inc(workflowmetrics.MetricUIAgentNoEffectTotal)
			step.Error = "typed action returned success but no stable UI effect was observed"
			noProgressSteps++
		}
		if lastSemanticHash == afterHash && afterHash != "" {
			noProgressSteps++
		}
		lastSemanticHash = afterHash
		result.Steps = append(result.Steps, step)
		if noProgressSteps >= 4 {
			result.Success = false
			result.Result = "Android UI agent stopped after repeated actions produced no observable progress"
			result.FinalState = "needs_user"
			result.StepCount = len(result.Steps)
			return result, nil
		}
	}

	result.Success = false
	result.StepCount = len(result.Steps)
	result.FinalState = "step_limit"
	result.Result = fmt.Sprintf("Android UI agent reached the %d-step limit without declaring the goal complete", req.MaxSteps)
	return result, nil
}

func (s *service) executeAndroidUIActionWithRecovery(
	ctx context.Context,
	scope extensionkernel.LegacyScope,
	req androiduiagent.Request,
	action plannedAndroidUIAction,
	baseline androidUITreeEnvelope,
	runKey string,
	index int,
) (string, json.RawMessage, extensionkernel.LegacyToolResult, bool, int) {
	toolID, input, err := mapAndroidUIAction(req, action)
	if err != nil {
		return "", nil, extensionkernel.LegacyToolResult{Status: "FAILED", VisibleText: err.Error()}, true, 0
	}
	execute := func(callSuffix string, id string, in json.RawMessage) (extensionkernel.LegacyToolResult, bool) {
		return s.toolFacade.ExecuteTool(ctx, capability.CapabilityID(id), in, scope,
			fmt.Sprintf("%s:android-ui:%d:%s", runKey, index, callSuffix),
			fmt.Sprintf("android-ui-agent:%s:%d:%s", runKey, index, callSuffix))
	}
	result, found := execute("primary", toolID, input)
	if !found || legacyToolSucceeded(result) {
		return toolID, input, result, found, 0
	}
	if blocked, _ := androidUIInteractionBlocked(result); blocked {
		return toolID, input, result, found, 0
	}

	code := ""
	if result.Error != nil {
		code = strings.ToUpper(strings.TrimSpace(result.Error.Code))
	}
	message := strings.ToUpper(strings.TrimSpace(result.VisibleText))
	if result.Error != nil {
		message += " " + strings.ToUpper(result.Error.Message)
	}
	if strings.Contains(code, "STALE") || strings.Contains(message, "STALE") {
		_, _, _, freshTree, observeErr := s.androidUIObservation(ctx, scope, runKey, index*100+1)
		if observeErr == nil {
			if rematched, _, ok := semanticRematchTarget(freshTree, action.Target); ok {
				retryAction := action
				retryAction.Target = rematched
				if retryTool, retryInput, retryErr := mapAndroidUIAction(req, retryAction); retryErr == nil {
					retryResult, retryFound := execute("semantic-rematch", retryTool, retryInput)
					if retryFound && legacyToolSucceeded(retryResult) {
						return retryTool, retryInput, retryResult, true, 1
					}
					result, found, toolID, input = retryResult, retryFound, retryTool, retryInput
				}
			}
		}
	}

	if visualAction, ok := visualFallbackAction(action, enrichTargetFromObservation(baseline, action.Target)); ok {
		if visualTool, visualInput, visualErr := mapAndroidUIAction(req, visualAction); visualErr == nil {
			visualResult, visualFound := execute("visual-fallback", visualTool, visualInput)
			if visualFound {
				return visualTool, visualInput, visualResult, true, 4
			}
		}
	}
	return toolID, input, result, found, 0
}

func (s *service) waitForAndroidUISettle(ctx context.Context, scope extensionkernel.LegacyScope, runKey string, index int) (json.RawMessage, string, androidUIObservationQuality, androidUITreeEnvelope, error) {
	delays := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}
	var previousHash string
	var lastRaw json.RawMessage
	var lastSummary string
	var lastQuality androidUIObservationQuality
	var lastTree androidUITreeEnvelope
	for sample, delay := range delays {
		select {
		case <-ctx.Done():
			return nil, "", lastQuality, lastTree, ctx.Err()
		case <-time.After(delay):
		}
		raw, summary, quality, tree, err := s.androidUIObservation(ctx, scope, runKey, index*1000+sample+1)
		if err != nil {
			return lastRaw, lastSummary, lastQuality, lastTree, err
		}
		currentHash := androidUISemanticHash(tree)
		lastRaw, lastSummary, lastQuality, lastTree = raw, summary, quality, tree
		if previousHash != "" && currentHash == previousHash {
			return lastRaw, lastSummary, lastQuality, lastTree, nil
		}
		previousHash = currentHash
	}
	return lastRaw, lastSummary, lastQuality, lastTree, nil
}

func (s *service) androidUIObservation(ctx context.Context, scope extensionkernel.LegacyScope, runKey string, index int) (json.RawMessage, string, androidUIObservationQuality, androidUITreeEnvelope, error) {
	input := json.RawMessage(`{"source":"auto","includeAllWindows":true,"includeInvisible":false,"maxDepth":32,"excludeOwnPackage":true,"allowRootFallback":false}`)
	result, found := s.toolFacade.ExecuteTool(ctx, "android.ui_tree.snapshot", input, scope, fmt.Sprintf("%s:uiagent-observe:%d", runKey, index), fmt.Sprintf("%s:uiagent-observe:%d", runKey, index))
	if !found {
		return nil, "", androidUIObservationQuality{}, androidUITreeEnvelope{}, errors.New("android.ui_tree.snapshot is unavailable")
	}
	if !legacyToolSucceeded(result) {
		message := strings.TrimSpace(result.VisibleText)
		if result.Error != nil && message == "" {
			message = result.Error.Message
		}
		if message == "" {
			message = "UI tree snapshot failed"
		}
		return nil, "", androidUIObservationQuality{}, androidUITreeEnvelope{}, errors.New(message)
	}
	raw := result.Output
	if len(raw) == 0 && result.VisibleText != "" {
		raw = json.RawMessage(result.VisibleText)
	}
	if len(raw) == 0 {
		return nil, "", androidUIObservationQuality{}, androidUITreeEnvelope{}, errors.New("UI tree snapshot returned no structured observation")
	}
	quality, tree := analyzeAndroidUIObservation(raw)
	wrapped := wrapAndroidUIObservation(raw, quality)
	bounded := compactAndroidObservation(wrapped, 36*1024)
	summary := string(bounded)
	if len(summary) > 2048 {
		summary = summary[:2048] + "…"
	}
	return bounded, summary, quality, tree, nil
}

func (s *service) planAndroidUIAction(cfg map[string]string, req androiduiagent.Request, observation json.RawMessage, history []androiduiagent.Step) (plannedAndroidUIAction, error) {
	historyJSON, _ := json.Marshal(historyForPlanner(history, 6))
	allowedApps, _ := json.Marshal(req.AllowedApps)
	system := `You are a bounded Android UI sub-agent planner. Return exactly one JSON object and nothing else.
You may choose ONLY these actions: click, long_click, input_text, clear_text, scroll, swipe, visual_click, back, home, recents, open_app, wait, done, needs_user, fail.
Never emit shell, ADB, root commands, arbitrary intents, package installation/uninstallation, permission grants, security-setting changes, purchases/payments, destructive deletion, or message submission with legal/financial impact. For those, return needs_user.
Prefer stable UI-tree nodeId/snapshotId targets from the current observation. Respect quality.visualRecommended: use visual_click when the structured tree is EMPTY/LOW_INFORMATION or custom-drawn/WebView content prevents reliable semantic targeting.
Never repeat an action that recent history marks no_effect, failed, or loop_blocked unless the observation changed materially.
One action per turn. Never claim success unless the current observation itself shows the goal is achieved.
JSON shape: {"action":"...","reason":"short","target":{...},"text":"...","direction":"forward|backward|up|down|left|right","amount":"small|medium|large","startX":0,"startY":0,"endX":0,"endY":0,"durationMs":300,"packageName":"...","description":"...","role":"...","waitMs":500,"result":"..."}.`
	user := fmt.Sprintf("Goal:\n%s\n\nAllowed app packages (empty means no explicit open_app allowlist):\n%s\n\nRecent action history:\n%s\n\nCurrent structured Android UI observation:\n%s", req.Goal, allowedApps, historyJSON, observation)
	content, _, err := s.callLLM(cfg, []map[string]interface{}{
		{"role": "system", "content": system},
		{"role": "user", "content": user},
	})
	if err != nil {
		return plannedAndroidUIAction{}, err
	}
	var action plannedAndroidUIAction
	if err := decodeSingleJSONObject(content, &action); err != nil {
		return plannedAndroidUIAction{}, fmt.Errorf("Android UI planner returned invalid action: %w", err)
	}
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	if action.Action == "" {
		return plannedAndroidUIAction{}, errors.New("planner action is empty")
	}
	return action, nil
}

func mapAndroidUIAction(req androiduiagent.Request, action plannedAndroidUIAction) (string, json.RawMessage, error) {
	target := sanitizeUITarget(action.Target)
	switch action.Action {
	case "click":
		if len(target) == 0 {
			return "", nil, errors.New("click requires a structured target")
		}
		raw, _ := json.Marshal(map[string]any{"target": target, "allowCoordinateFallback": true, "allowVisualFallback": true, "allowRootFallback": req.AllowRootFallback, "allowAdbFallback": req.AllowADBFallback, "verify": true})
		return "android.interaction.click", raw, nil
	case "long_click":
		if len(target) == 0 {
			return "", nil, errors.New("long_click requires a structured target")
		}
		duration := action.DurationMS
		if duration < 300 {
			duration = 700
		}
		if duration > 3000 {
			duration = 3000
		}
		raw, _ := json.Marshal(map[string]any{"target": target, "durationMs": duration, "allowCoordinateFallback": true, "allowVisualFallback": true, "allowRootFallback": req.AllowRootFallback, "allowAdbFallback": req.AllowADBFallback, "verify": true})
		return "android.interaction.long_click", raw, nil
	case "input_text":
		if target["snapshotId"] == nil || target["nodeId"] == nil {
			return "", nil, errors.New("input_text requires snapshotId and nodeId")
		}
		if len([]rune(action.Text)) > 10000 {
			return "", nil, errors.New("input text exceeds 10000 characters")
		}
		raw, _ := json.Marshal(map[string]any{"target": map[string]any{"snapshotId": target["snapshotId"], "nodeId": target["nodeId"]}, "text": action.Text, "allowAdbFallback": req.AllowADBFallback, "verify": true})
		return "android.interaction.input_text", raw, nil
	case "clear_text":
		if target["snapshotId"] == nil || target["nodeId"] == nil {
			return "", nil, errors.New("clear_text requires snapshotId and nodeId")
		}
		raw, _ := json.Marshal(map[string]any{"target": map[string]any{"snapshotId": target["snapshotId"], "nodeId": target["nodeId"]}, "verify": true})
		return "android.interaction.clear_text", raw, nil
	case "scroll":
		if target["snapshotId"] == nil || target["nodeId"] == nil {
			return "", nil, errors.New("scroll requires snapshotId and nodeId")
		}
		direction := strings.ToLower(strings.TrimSpace(action.Direction))
		if !oneOf(direction, "forward", "backward", "up", "down", "left", "right") {
			return "", nil, errors.New("scroll direction is invalid")
		}
		amount := strings.ToLower(strings.TrimSpace(action.Amount))
		if !oneOf(amount, "small", "medium", "large") {
			amount = "medium"
		}
		raw, _ := json.Marshal(map[string]any{"target": map[string]any{"snapshotId": target["snapshotId"], "nodeId": target["nodeId"]}, "direction": direction, "amount": amount, "verify": true})
		return "android.interaction.scroll", raw, nil
	case "swipe":
		if action.StartX < 0 || action.StartY < 0 || action.EndX < 0 || action.EndY < 0 {
			return "", nil, errors.New("swipe coordinates must be non-negative")
		}
		if action.StartX == action.EndX && action.StartY == action.EndY {
			return "", nil, errors.New("swipe start and end coordinates must differ")
		}
		duration := action.DurationMS
		if duration < 100 {
			duration = 350
		}
		if duration > 3000 {
			duration = 3000
		}
		raw, _ := json.Marshal(map[string]any{"startX": action.StartX, "startY": action.StartY, "endX": action.EndX, "endY": action.EndY, "durationMs": duration})
		return "android.interaction.swipe", raw, nil
	case "visual_click":
		description := strings.TrimSpace(action.Description)
		if description == "" {
			description = strings.TrimSpace(action.Text)
		}
		if description == "" {
			return "", nil, errors.New("visual_click requires description or text")
		}
		raw, _ := json.Marshal(map[string]any{"description": description, "text": strings.TrimSpace(action.Text), "role": strings.TrimSpace(action.Role), "ocrFirst": true, "verify": true})
		return "android.interaction.visual_click", raw, nil
	case "back", "home", "recents":
		raw, _ := json.Marshal(map[string]any{"action": action.Action})
		return "android.input.global_action", raw, nil
	case "open_app":
		packageName := strings.TrimSpace(action.PackageName)
		if packageName == "" {
			return "", nil, errors.New("open_app requires packageName")
		}
		if len(req.AllowedApps) > 0 && !containsString(req.AllowedApps, packageName) {
			return "", nil, fmt.Errorf("package %s is outside allowedApps", packageName)
		}
		raw, _ := json.Marshal(map[string]any{"packageName": packageName})
		return "android.app.open", raw, nil
	default:
		return "", nil, fmt.Errorf("planner action %q is not permitted", action.Action)
	}
}

func sanitizeUITarget(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	allowed := map[string]struct{}{"snapshotId": {}, "nodeId": {}, "x": {}, "y": {}, "text": {}, "resourceId": {}, "role": {}, "description": {}}
	out := make(map[string]any)
	for key, value := range input {
		if _, ok := allowed[key]; !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" && len(trimmed) <= 1024 {
				out[key] = trimmed
			}
		case float64:
			if key == "x" || key == "y" {
				out[key] = int(typed)
			}
		case int:
			if key == "x" || key == "y" {
				out[key] = typed
			}
		}
	}
	return out
}

func compactAndroidObservation(raw json.RawMessage, maxBytes int) json.RawMessage {
	if len(raw) <= maxBytes {
		return append(json.RawMessage(nil), raw...)
	}
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil {
		encoded, _ := json.Marshal(map[string]any{"agentObservationTruncated": true, "rawBytes": len(raw)})
		return encoded
	}
	if nodes, ok := root["nodes"].([]any); ok && len(nodes) > 80 {
		root["nodes"] = nodes[:80]
		root["agentObservationTruncated"] = true
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return append(json.RawMessage(nil), raw[:maxBytes]...)
	}
	if len(encoded) > maxBytes {
		root["nodes"] = []any{}
		root["agentObservationTruncated"] = true
		encoded, _ = json.Marshal(root)
	}
	if len(encoded) > maxBytes {
		minimal := map[string]any{"agentObservationTruncated": true}
		for _, key := range []string{"packageName", "currentPackage", "activity", "windowTitle", "width", "height", "orientation", "keyboardVisible"} {
			if value, ok := root[key]; ok {
				minimal[key] = value
			}
		}
		encoded, _ = json.Marshal(minimal)
	}
	return encoded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func historyForPlanner(steps []androiduiagent.Step, limit int) []map[string]any {
	if len(steps) > limit {
		steps = steps[len(steps)-limit:]
	}
	out := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		out = append(out, map[string]any{
			"index":  step.Index,
			"action": step.Action,
			"status": step.Status,
			"error":  step.Error,
		})
	}
	return out
}

func decodeSingleJSONObject(content string, target any) error {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return errors.New("response does not contain a JSON object")
	}
	decoder := json.NewDecoder(strings.NewReader(trimmed[start : end+1]))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func androidUIInteractionBlocked(result extensionkernel.LegacyToolResult) (bool, string) {
	code := ""
	message := strings.TrimSpace(result.VisibleText)
	if result.Error != nil {
		code = strings.ToUpper(strings.TrimSpace(result.Error.Code))
		if message == "" {
			message = strings.TrimSpace(result.Error.Message)
		}
	}
	switch code {
	case "DEVICE_WAITING_UNLOCK", "DEVICE_WAITING_SCREEN", "DEVICE_BACKGROUND_RESTRICTED":
		if message == "" {
			message = code
		}
		return true, message
	}
	upper := strings.ToUpper(message)
	if strings.Contains(upper, "DEVICE_WAITING_UNLOCK") ||
		strings.Contains(upper, "DEVICE_WAITING_SCREEN") ||
		strings.Contains(upper, "DEVICE_BACKGROUND_RESTRICTED") {
		return true, message
	}
	return false, ""
}

func legacyToolSucceeded(result extensionkernel.LegacyToolResult) bool {
	status := strings.ToUpper(strings.TrimSpace(result.Status))
	return status == "SUCCESS" || status == "SUCCEEDED" || status == "OK"
}

func oneOf(value string, values ...string) bool {
	for _, item := range values {
		if value == item {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
