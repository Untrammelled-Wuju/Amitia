package browseragent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/agent/tool"
)

const ToolName = "browser.agent.run"

type Request struct {
	Goal         string   `json:"goal"`
	StartURL     string   `json:"startUrl,omitempty"`
	SessionID    string   `json:"sessionId,omitempty"`
	TabID        string   `json:"tabId,omitempty"`
	MaxSteps     int      `json:"maxSteps,omitempty"`
	TimeoutMS    int64    `json:"timeoutMs,omitempty"`
	AllowedHosts []string `json:"allowedHosts,omitempty"`
	KeepSession  bool     `json:"keepSession,omitempty"`
}

type Step struct {
	Index       int             `json:"index"`
	Action      string          `json:"action"`
	Reason      string          `json:"reason,omitempty"`
	ToolName    string          `json:"toolName,omitempty"`
	Input       json.RawMessage `json:"input,omitempty"`
	Status      string          `json:"status"`
	Observation string          `json:"observation,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type Result struct {
	Success    bool   `json:"success"`
	Result     string `json:"result,omitempty"`
	FinalState string `json:"finalState,omitempty"`
	SessionID  string `json:"sessionId,omitempty"`
	TabID      string `json:"tabId,omitempty"`
	Steps      []Step `json:"steps"`
	StepCount  int    `json:"stepCount"`
}

type Runner interface {
	RunBrowserAgent(context.Context, tool.ToolExecutionContext, Request) (Result, error)
}

var (
	runnerMu sync.RWMutex
	runner   Runner
)

func SetRunner(value Runner) {
	runnerMu.Lock()
	runner = value
	runnerMu.Unlock()
}

func currentRunner() Runner {
	runnerMu.RLock()
	defer runnerMu.RUnlock()
	return runner
}

func init() {
	tool.Register(tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        ToolName,
			Description: "Run a bounded browser sub-agent that observes DOM state, plans one constrained action at a time, executes only typed browser capabilities, re-observes, and replans until the goal completes.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"goal":         {Type: "string", Description: "The concrete browser goal."},
					"startUrl":     {Type: "string", Description: "Optional initial http or https URL."},
					"sessionId":    {Type: "string", Description: "Optional existing browser session."},
					"tabId":        {Type: "string", Description: "Optional existing tab, requiring sessionId."},
					"maxSteps":     {Type: "integer", Description: "Maximum autonomous steps, 1-30."},
					"timeoutMs":    {Type: "integer", Description: "Overall timeout, 5000-180000 milliseconds."},
					"allowedHosts": {Type: "array", Description: "Optional hostname allowlist for navigation."},
					"keepSession":  {Type: "boolean", Description: "Keep a session created by this run open after completion."},
				},
				Required: []string{"goal"},
			},
		},
	}, runTool)
}

func runTool(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	r := currentRunner()
	if r == nil {
		return tool.ErrorResult("browser_agent_unavailable", "browser agent runner is not configured")
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return tool.ErrorResult("browser_agent_invalid_request", err.Error())
	}
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return tool.ErrorResult("browser_agent_invalid_request", err.Error())
	}
	if err := normalizeRequest(&req); err != nil {
		return tool.ErrorResult("browser_agent_invalid_request", err.Error())
	}
	result, err := r.RunBrowserAgent(ctx, execCtx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return tool.CancelledResult(err.Error())
		}
		return tool.ErrorResult("browser_agent_failed", err.Error())
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return tool.ErrorResult("browser_agent_result_encode_failed", fmt.Sprintf("encode result: %v", err))
	}
	callResult := tool.TextResult(string(encoded))
	callResult.VisibleText = result.Result
	callResult.Audit = map[string]interface{}{"stepCount": result.StepCount, "success": result.Success}
	return callResult
}

func normalizeRequest(req *Request) error {
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		return errors.New("goal is required")
	}
	if len([]rune(req.Goal)) > 4096 {
		return errors.New("goal exceeds 4096 characters")
	}
	req.StartURL = strings.TrimSpace(req.StartURL)
	if req.StartURL != "" {
		parsed, err := url.Parse(req.StartURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || strings.TrimSpace(parsed.Hostname()) == "" {
			return errors.New("startUrl must be an absolute http or https URL")
		}
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.TabID = strings.TrimSpace(req.TabID)
	if req.TabID != "" && req.SessionID == "" {
		return errors.New("tabId requires sessionId")
	}
	if req.MaxSteps == 0 {
		req.MaxSteps = 12
	}
	if req.MaxSteps < 1 || req.MaxSteps > 30 {
		return errors.New("maxSteps must be between 1 and 30")
	}
	if req.TimeoutMS == 0 {
		req.TimeoutMS = 90_000
	}
	if req.TimeoutMS < 5_000 || req.TimeoutMS > 180_000 {
		return errors.New("timeoutMs must be between 5000 and 180000")
	}
	if len(req.AllowedHosts) > 64 {
		return errors.New("allowedHosts exceeds 64 entries")
	}
	seen := make(map[string]struct{}, len(req.AllowedHosts))
	out := make([]string, 0, len(req.AllowedHosts))
	for _, rawHost := range req.AllowedHosts {
		host := strings.ToLower(strings.TrimSpace(rawHost))
		host = strings.TrimSuffix(host, ".")
		if host == "" || strings.ContainsAny(host, "/:@?#") || len(host) > 253 {
			return errors.New("allowedHosts contains an invalid hostname")
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	req.AllowedHosts = out
	return nil
}
