package androiduiagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/agent/tool"
)

const ToolName = "android.ui.agent.run"

type Request struct {
	Goal              string   `json:"goal"`
	MaxSteps          int      `json:"maxSteps,omitempty"`
	TimeoutMS         int64    `json:"timeoutMs,omitempty"`
	AllowedApps       []string `json:"allowedApps,omitempty"`
	AllowADBFallback  bool     `json:"allowAdbFallback,omitempty"`
	AllowRootFallback bool     `json:"allowRootFallback,omitempty"`
}

type Step struct {
	Index              int             `json:"index"`
	Action             string          `json:"action"`
	Reason             string          `json:"reason,omitempty"`
	ToolName           string          `json:"toolName,omitempty"`
	Input              json.RawMessage `json:"input,omitempty"`
	Status             string          `json:"status"`
	Observation        string          `json:"observation,omitempty"`
	ObservationQuality string          `json:"observationQuality,omitempty"`
	ActionFingerprint  string          `json:"actionFingerprint,omitempty"`
	FallbackLevel      int             `json:"fallbackLevel,omitempty"`
	VerificationResult string          `json:"verificationResult,omitempty"`
	BeforeHash         string          `json:"beforeHash,omitempty"`
	AfterHash          string          `json:"afterHash,omitempty"`
	LatencyMS          int64           `json:"latencyMs,omitempty"`
	Error              string          `json:"error,omitempty"`
}

type Result struct {
	Success    bool   `json:"success"`
	Result     string `json:"result,omitempty"`
	FinalState string `json:"finalState,omitempty"`
	Steps      []Step `json:"steps"`
	StepCount  int    `json:"stepCount"`
}

type Runner interface {
	RunAndroidUIAgent(context.Context, tool.ToolExecutionContext, Request) (Result, error)
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
			Description: "Run a bounded autonomous Android UI sub-agent that observes the current UI tree, plans one safe action at a time, executes through typed Android capabilities, verifies by re-observing, and replans until the goal is complete.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"goal":              {Type: "string", Description: "The concrete Android UI goal to accomplish."},
					"maxSteps":          {Type: "integer", Description: "Maximum autonomous action steps, 1-30."},
					"timeoutMs":         {Type: "integer", Description: "Overall timeout in milliseconds, 5000-180000."},
					"allowedApps":       {Type: "array", Description: "Optional package-name allowlist the sub-agent may open."},
					"allowAdbFallback":  {Type: "boolean", Description: "Allow already-authorized ADB provider fallback. Never grants ADB permission."},
					"allowRootFallback": {Type: "boolean", Description: "Allow already-authorized root provider fallback. Never elevates privilege."},
				},
				Required: []string{"goal"},
			},
		},
	}, runTool)
}

func runTool(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	r := currentRunner()
	if r == nil {
		return tool.ErrorResult("android_ui_agent_unavailable", "Android UI agent runner is not configured")
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return tool.ErrorResult("android_ui_agent_invalid_request", err.Error())
	}
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return tool.ErrorResult("android_ui_agent_invalid_request", err.Error())
	}
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		return tool.ErrorResult("android_ui_agent_invalid_request", "goal is required")
	}
	if len([]rune(req.Goal)) > 4096 {
		return tool.ErrorResult("android_ui_agent_invalid_request", "goal exceeds 4096 characters")
	}
	if req.MaxSteps == 0 {
		req.MaxSteps = 12
	}
	if req.MaxSteps < 1 || req.MaxSteps > 30 {
		return tool.ErrorResult("android_ui_agent_invalid_request", "maxSteps must be between 1 and 30")
	}
	if req.TimeoutMS == 0 {
		req.TimeoutMS = 90_000
	}
	if req.TimeoutMS < 5_000 || req.TimeoutMS > 180_000 {
		return tool.ErrorResult("android_ui_agent_invalid_request", "timeoutMs must be between 5000 and 180000")
	}
	if len(req.AllowedApps) > 32 {
		return tool.ErrorResult("android_ui_agent_invalid_request", "allowedApps exceeds 32 entries")
	}
	seen := make(map[string]struct{}, len(req.AllowedApps))
	for index := range req.AllowedApps {
		value := strings.TrimSpace(req.AllowedApps[index])
		if value == "" || len(value) > 255 {
			return tool.ErrorResult("android_ui_agent_invalid_request", "allowedApps contains an invalid package name")
		}
		if _, ok := seen[value]; ok {
			return tool.ErrorResult("android_ui_agent_invalid_request", "allowedApps contains duplicate package names")
		}
		seen[value] = struct{}{}
		req.AllowedApps[index] = value
	}

	result, err := r.RunAndroidUIAgent(ctx, execCtx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return tool.CancelledResult(err.Error())
		}
		return tool.ErrorResult("android_ui_agent_failed", err.Error())
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return tool.ErrorResult("android_ui_agent_result_encode_failed", fmt.Sprintf("encode result: %v", err))
	}
	callResult := tool.TextResult(string(encoded))
	callResult.VisibleText = result.Result
	callResult.Audit = map[string]interface{}{
		"stepCount": result.StepCount,
		"success":   result.Success,
	}
	return callResult
}
