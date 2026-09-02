package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/browseragent"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type plannedBrowserAction struct {
	Action    string         `json:"action"`
	Reason    string         `json:"reason,omitempty"`
	URL       string         `json:"url,omitempty"`
	Target    map[string]any `json:"target,omitempty"`
	Text      string         `json:"text,omitempty"`
	Value     string         `json:"value,omitempty"`
	Direction string         `json:"direction,omitempty"`
	WaitMS    int            `json:"waitMs,omitempty"`
	Result    string         `json:"result,omitempty"`
}

func (s *service) RunBrowserAgent(ctx context.Context, execCtx tool.ToolExecutionContext, req browseragent.Request) (browseragent.Result, error) {
	if s == nil || s.toolFacade == nil {
		return browseragent.Result{}, errors.New("canonical ToolFacade is not configured")
	}
	cfg := s.getActiveModel()
	if cfg == nil {
		return browseragent.Result{}, errors.New("no active model configuration for browser agent")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(req.TimeoutMS)*time.Millisecond)
	defer cancel()

	scope := extensionkernel.LegacyScope{
		UserID:         execCtx.User,
		CharacterID:    execCtx.CharacterID,
		ConversationID: execCtx.ConversationID,
		Channel:        execCtx.Channel,
		TraceID:        execCtx.CorrelationID,
		RequestID:      execCtx.RequestID,
		ToolCallID:     execCtx.ToolCallID,
		CorrelationID:  execCtx.CorrelationID,
		CausationID:    execCtx.CausationID,
	}
	runKey := firstNonEmpty(strings.TrimSpace(execCtx.IdempotencyKey), strings.TrimSpace(execCtx.ToolCallID), strings.TrimSpace(execCtx.RequestID), strings.TrimSpace(execCtx.CorrelationID))
	if runKey == "" {
		runKey = fmt.Sprintf("browseragent:%d", time.Now().UnixNano())
	}

	result := browseragent.Result{Steps: make([]browseragent.Step, 0, req.MaxSteps)}
	sessionID := req.SessionID
	tabID := req.TabID
	createdSession := false
	createdTab := false
	if sessionID == "" {
		raw, err := s.browserAgentExecute(ctx, scope, runKey, 0, "browser_session_create", json.RawMessage(`{}`))
		if err != nil {
			return result, err
		}
		var value struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value.SessionID) == "" {
			return result, errors.New("browser session creation returned no sessionId")
		}
		sessionID = strings.TrimSpace(value.SessionID)
		createdSession = true
	}
	result.SessionID = sessionID
	if createdSession && !req.KeepSession {
		defer func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			input, _ := json.Marshal(map[string]any{"sessionId": sessionID})
			_, _ = s.browserAgentExecute(cleanupCtx, scope, runKey, req.MaxSteps+100, "browser_session_close", input)
		}()
	}

	if tabID == "" {
		input, _ := json.Marshal(map[string]any{"sessionId": sessionID})
		raw, err := s.browserAgentExecute(ctx, scope, runKey, 0, "browser_tab_create", input)
		if err != nil {
			return result, err
		}
		var value struct {
			TabID string `json:"tabId"`
		}
		if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value.TabID) == "" {
			return result, errors.New("browser tab creation returned no tabId")
		}
		tabID = strings.TrimSpace(value.TabID)
		createdTab = true
	}
	result.TabID = tabID
	if !createdSession && createdTab && !req.KeepSession {
		defer func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			input, _ := json.Marshal(map[string]any{"sessionId": sessionID, "tabId": tabID})
			_, _ = s.browserAgentExecute(cleanupCtx, scope, runKey, req.MaxSteps+101, "browser_tab_close", input)
		}()
	}

	if req.StartURL != "" {
		if err := validateBrowserAgentURL(req.StartURL, req.AllowedHosts); err != nil {
			return result, err
		}
		input, _ := json.Marshal(map[string]any{"sessionId": sessionID, "tabId": tabID, "url": req.StartURL, "waitUntil": "load", "timeoutMs": 30000})
		if _, err := s.browserAgentExecute(ctx, scope, runKey, 0, "browser_navigate", input); err != nil {
			return result, err
		}
	}

	for index := 1; index <= req.MaxSteps; index++ {
		if err := ctx.Err(); err != nil {
			result.StepCount = len(result.Steps)
			result.FinalState = "timeout"
			return result, err
		}
		observation, summary, err := s.browserAgentObservation(ctx, scope, runKey, index, sessionID, tabID)
		if err != nil {
			result.StepCount = len(result.Steps)
			result.FinalState = "observation_failed"
			return result, err
		}
		action, err := s.planBrowserAction(cfg, req, observation, result.Steps)
		if err != nil {
			result.Steps = append(result.Steps, browseragent.Step{Index: index, Action: "planner", Status: "failed", Observation: summary, Error: err.Error()})
			result.StepCount = len(result.Steps)
			result.FinalState = "planner_failed"
			return result, err
		}
		step := browseragent.Step{Index: index, Action: action.Action, Reason: strings.TrimSpace(action.Reason), Observation: summary}
		switch action.Action {
		case "done":
			step.Status = "completed"
			result.Steps = append(result.Steps, step)
			result.Success = true
			result.Result = strings.TrimSpace(action.Result)
			if result.Result == "" {
				result.Result = "Browser goal completed"
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
			result.Steps = append(result.Steps, step)
			continue
		}

		toolID, input, policyErr := mapBrowserAction(req, sessionID, tabID, action)
		if policyErr != nil {
			step.Status = "rejected"
			step.Error = policyErr.Error()
			result.Steps = append(result.Steps, step)
			continue
		}
		step.ToolName = toolID
		step.Input = input
		if _, err := s.browserAgentExecute(ctx, scope, runKey, index, toolID, input); err != nil {
			step.Status = "failed"
			step.Error = err.Error()
			result.Steps = append(result.Steps, step)
			continue
		}
		step.Status = "success"
		result.Steps = append(result.Steps, step)
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	result.StepCount = len(result.Steps)
	result.FinalState = "step_limit"
	result.Result = fmt.Sprintf("Browser agent reached the %d-step limit without declaring the goal complete", req.MaxSteps)
	return result, nil
}

func (s *service) browserAgentExecute(ctx context.Context, scope extensionkernel.LegacyScope, runKey string, index int, toolID string, input json.RawMessage) (json.RawMessage, error) {
	callID := fmt.Sprintf("%s:browser:%s:%d", runKey, toolID, index)
	idempotency := fmt.Sprintf("browser-agent:%s:%s:%d", runKey, toolID, index)
	toolResult, found := s.toolFacade.ExecuteTool(ctx, capability.CapabilityID(toolID), input, scope, callID, idempotency)
	if !found {
		return nil, errors.New("typed browser tool is unavailable: " + toolID)
	}
	if !legacyToolSucceeded(toolResult) {
		message := strings.TrimSpace(toolResult.VisibleText)
		if toolResult.Error != nil {
			if message == "" {
				message = strings.TrimSpace(toolResult.Error.Message)
			}
			if message == "" {
				message = strings.TrimSpace(toolResult.Error.Code)
			}
		}
		if message == "" {
			message = toolID + " failed"
		}
		return nil, errors.New(message)
	}
	if len(toolResult.Output) > 0 {
		return toolResult.Output, nil
	}
	if strings.TrimSpace(toolResult.VisibleText) != "" {
		return json.RawMessage(toolResult.VisibleText), nil
	}
	return json.RawMessage(`{}`), nil
}

func (s *service) browserAgentObservation(ctx context.Context, scope extensionkernel.LegacyScope, runKey string, index int, sessionID, tabID string) (json.RawMessage, string, error) {
	input, _ := json.Marshal(map[string]any{"sessionId": sessionID, "tabId": tabID, "maxDepth": 28})
	raw, err := s.browserAgentExecute(ctx, scope, runKey, index, "browser_dom_snapshot", input)
	if err != nil {
		return nil, "", err
	}
	bounded := raw
	if len(bounded) > 48*1024 {
		var value map[string]any
		if json.Unmarshal(raw, &value) == nil {
			if content, ok := value["content"].(string); ok && len(content) > 42*1024 {
				value["content"] = content[:42*1024]
				value["agentTruncated"] = true
				if encoded, encodeErr := json.Marshal(value); encodeErr == nil {
					bounded = encoded
				}
			}
		}
	}
	summary := string(bounded)
	if len(summary) > 2048 {
		summary = summary[:2048] + "…"
	}
	return bounded, summary, nil
}

func (s *service) planBrowserAction(cfg map[string]string, req browseragent.Request, observation json.RawMessage, history []browseragent.Step) (plannedBrowserAction, error) {
	historyJSON, _ := json.Marshal(browserHistoryForPlanner(history, 6))
	allowedHosts, _ := json.Marshal(req.AllowedHosts)
	system := `You are a bounded browser sub-agent planner. Return exactly one JSON object and nothing else.
You may choose ONLY these actions: navigate, click, input, select, scroll, back, forward, wait, done, needs_user, fail.
Never request shell commands, arbitrary code execution, downloads, uploads, credential entry, CAPTCHA solving, payments or purchases, sending messages/posts/forms with external impact, destructive deletion, permission grants, or account/security changes. Return needs_user for those actions.
Use stableId values from the current DOM snapshot for every element interaction. Never emit a selector-only target.
One action per turn. Never claim completion unless the current observation shows the goal is achieved.
JSON shape: {"action":"...","reason":"short","url":"https://...","target":{"stableId":"...","selector":"...","runtimeGeneration":0,"documentGeneration":0,"frameId":"..."},"text":"...","value":"...","direction":"up|down|left|right","waitMs":500,"result":"..."}.`
	user := fmt.Sprintf("Goal:\n%s\n\nAllowed navigation hosts (empty means normal browser policy only):\n%s\n\nRecent action history:\n%s\n\nCurrent DOM observation:\n%s", req.Goal, allowedHosts, historyJSON, observation)
	content, _, err := s.callLLM(cfg, []map[string]interface{}{{"role": "system", "content": system}, {"role": "user", "content": user}})
	if err != nil {
		return plannedBrowserAction{}, err
	}
	var action plannedBrowserAction
	if err := decodeSingleJSONObject(content, &action); err != nil {
		return plannedBrowserAction{}, fmt.Errorf("browser planner returned invalid action: %w", err)
	}
	action.Action = strings.ToLower(strings.TrimSpace(action.Action))
	if action.Action == "" {
		return plannedBrowserAction{}, errors.New("browser planner action is empty")
	}
	return action, nil
}

func mapBrowserAction(req browseragent.Request, sessionID, tabID string, action plannedBrowserAction) (string, json.RawMessage, error) {
	base := map[string]any{"sessionId": sessionID, "tabId": tabID}
	switch action.Action {
	case "navigate":
		if err := validateBrowserAgentURL(action.URL, req.AllowedHosts); err != nil {
			return "", nil, err
		}
		base["url"] = strings.TrimSpace(action.URL)
		base["waitUntil"] = "load"
		base["timeoutMs"] = 30000
		return marshalBrowserAction("browser_navigate", base)
	case "click":
		target, err := normalizeBrowserTarget(action.Target)
		if err != nil {
			return "", nil, err
		}
		base["element"] = target
		return marshalBrowserAction("browser_interact_click", base)
	case "input":
		target, err := normalizeBrowserTarget(action.Target)
		if err != nil {
			return "", nil, err
		}
		if len([]rune(action.Text)) > 8192 {
			return "", nil, errors.New("browser input text exceeds 8192 characters")
		}
		base["element"] = target
		base["inputText"] = action.Text
		return marshalBrowserAction("browser_interact_input", base)
	case "select":
		target, err := normalizeBrowserTarget(action.Target)
		if err != nil {
			return "", nil, err
		}
		base["element"] = target
		base["value"] = action.Value
		return marshalBrowserAction("browser_interact_select", base)
	case "scroll":
		direction := strings.ToLower(strings.TrimSpace(action.Direction))
		if direction != "up" && direction != "down" && direction != "left" && direction != "right" {
			return "", nil, errors.New("browser scroll direction must be up, down, left, or right")
		}
		base["direction"] = direction
		return marshalBrowserAction("browser_interact_scroll", base)
	case "back":
		return marshalBrowserAction("browser_navigate_back", base)
	case "forward":
		return marshalBrowserAction("browser_navigate_forward", base)
	default:
		return "", nil, fmt.Errorf("browser action %q is not allowed", action.Action)
	}
}

func marshalBrowserAction(toolID string, value map[string]any) (string, json.RawMessage, error) {
	raw, err := json.Marshal(value)
	return toolID, raw, err
}

func normalizeBrowserTarget(target map[string]any) (map[string]any, error) {
	if len(target) == 0 {
		return nil, errors.New("browser action requires a target")
	}
	stableID := strings.TrimSpace(fmt.Sprint(target["stableId"]))
	selector := strings.TrimSpace(fmt.Sprint(target["selector"]))
	if stableID == "<nil>" {
		stableID = ""
	}
	if selector == "<nil>" {
		selector = ""
	}
	if stableID == "" {
		return nil, errors.New("browser target requires stableId from the current DOM snapshot")
	}
	out := map[string]any{}
	if stableID != "" {
		out["stableId"] = stableID
	}
	if selector != "" {
		out["selector"] = selector
	}
	for _, key := range []string{"runtimeGeneration", "documentGeneration"} {
		if value, ok := target[key]; ok {
			if number, ok := value.(float64); ok && number >= 0 {
				out[key] = uint64(number)
			}
		}
	}
	if frameID := strings.TrimSpace(fmt.Sprint(target["frameId"])); frameID != "" && frameID != "<nil>" {
		out["frameId"] = frameID
	}
	return out, nil
}

func validateBrowserAgentURL(rawURL string, allowedHosts []string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || strings.TrimSpace(parsed.Hostname()) == "" {
		return errors.New("browser navigation requires an absolute http or https URL")
	}
	if len(allowedHosts) == 0 {
		return nil
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	for _, allowed := range allowedHosts {
		allowed = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(allowed), "."))
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("browser navigation host %s is outside allowedHosts", host)
}

func browserHistoryForPlanner(history []browseragent.Step, limit int) []browseragent.Step {
	if limit <= 0 || len(history) <= limit {
		return history
	}
	return history[len(history)-limit:]
}
