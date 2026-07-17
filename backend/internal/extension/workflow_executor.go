package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type WorkflowExecutionMode string

const (
	WorkflowProduction     WorkflowExecutionMode = "production"
	WorkflowDryRun         WorkflowExecutionMode = "dry_run"
	WorkflowMocked         WorkflowExecutionMode = "mocked"
	WorkflowControlledLive WorkflowExecutionMode = "controlled_live"
)

type WorkflowExecutionRequest struct {
	Workflow   CompiledWorkflow
	Input      json.RawMessage
	Config     json.RawMessage
	Secrets    map[string]string
	Scope      ExecutionScope
	Mode       WorkflowExecutionMode
	HTTPMocks  []HTTPMock
	SkillMocks []SkillMock
}
type WorkflowExecutionResult struct {
	Output      json.RawMessage
	Steps       []WorkflowStepResult
	SideEffects []SideEffectRecord
}
type WorkflowAdapterRequest struct {
	Step       CompiledStep
	Input      json.RawMessage
	Context    map[string]interface{}
	Scope      ExecutionScope
	Mode       WorkflowExecutionMode
	HTTPMocks  []HTTPMock
	SkillMocks []SkillMock
	Limits     WorkflowLimits
}
type WorkflowAdapterResult struct {
	Output      json.RawMessage
	SideEffects []SideEffectRecord
	Mocked      bool
}
type WorkflowStepAdapter interface {
	Execute(context.Context, WorkflowAdapterRequest) (WorkflowAdapterResult, error)
}

type WorkflowAdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]WorkflowStepAdapter
}

func NewWorkflowAdapterRegistry() *WorkflowAdapterRegistry {
	return &WorkflowAdapterRegistry{adapters: map[string]WorkflowStepAdapter{}}
}
func (r *WorkflowAdapterRegistry) Register(kind string, adapter WorkflowStepAdapter) error {
	if !allowedWorkflowSteps[kind] || adapter == nil {
		return fmt.Errorf("invalid workflow adapter: %s", kind)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.adapters[kind]; ok {
		return fmt.Errorf("duplicate workflow adapter: %s", kind)
	}
	r.adapters[kind] = adapter
	return nil
}
func (r *WorkflowAdapterRegistry) Get(kind string) (WorkflowStepAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[kind]
	return adapter, ok
}

type WorkflowExecutor struct {
	adapters  *WorkflowAdapterRegistry
	validator *SchemaValidator
}

func NewWorkflowExecutor(adapters *WorkflowAdapterRegistry, validator *SchemaValidator) *WorkflowExecutor {
	return &WorkflowExecutor{adapters: adapters, validator: validator}
}

func (e *WorkflowExecutor) Execute(ctx context.Context, request WorkflowExecutionRequest, outputSchema json.RawMessage) (WorkflowExecutionResult, error) {
	if int64(len(request.Input)) > request.Workflow.Limits.MaxInputBytes {
		return WorkflowExecutionResult{}, NewExtensionError(ErrWorkshopSandboxLimit, "工作流输入超过限制", "", false, nil)
	}
	var input interface{}
	var config interface{}
	_ = json.Unmarshal(normalizeJSON(request.Input), &input)
	_ = json.Unmarshal(normalizeJSON(request.Config), &config)
	secrets := map[string]interface{}{}
	for key, value := range request.Secrets {
		secrets[key] = value
	}
	values := map[string]interface{}{"input": input, "config": config, "secrets": secrets, "steps": map[string]interface{}{}, "runtime": map[string]interface{}{"traceId": request.Scope.TraceID, "runId": request.Scope.RequestID, "characterId": request.Scope.CharacterID, "conversationId": request.Scope.ConversationID, "channel": request.Scope.Channel}}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(request.Workflow.Limits.MaxExecutionDurationMS)*time.Millisecond)
	defer cancel()
	results := []WorkflowStepResult{}
	effects := []SideEffectRecord{}
	for _, step := range request.Workflow.Steps {
		incrementWorkshopMetric(WorkshopMetricWorkflowStep)
		if err := execCtx.Err(); err != nil {
			return WorkflowExecutionResult{Steps: results, SideEffects: effects}, NewExtensionError(ErrWorkflowStepTimeout, "工作流执行超时或取消", err.Error(), true, err)
		}
		started := time.Now()
		item := WorkflowStepResult{StepID: step.ID, Type: step.Type, Status: "running"}
		if step.When != nil {
			allowed, err := evalCondition(step.When, values, request.Workflow.Limits.MaxExpressionDepth)
			if err != nil {
				incrementWorkshopMetric(WorkshopMetricWorkflowStepFailure)
				item.Status = "failed"
				item.Error = NewExtensionError(ErrWorkflowStepInvalid, "条件计算失败", err.Error(), false, err)
				results = append(results, item)
				return WorkflowExecutionResult{Steps: results, SideEffects: effects}, item.Error
			}
			if !allowed {
				item.Status = "skipped"
				item.DurationMS = time.Since(started).Milliseconds()
				results = append(results, item)
				continue
			}
		}
		resolved, err := resolveJSON(step.Input, values, request.Workflow.Limits.MaxTemplateLength, step.Type == "http")
		if err == nil && int64(len(resolved)) > request.Workflow.Limits.MaxIntermediateBytes {
			err = NewExtensionError(ErrWorkshopSandboxLimit, "步骤输入超过限制", step.ID, false, nil)
		}
		item.InputSummary = compactSensitiveJSON(step.Input)
		var adapterResult WorkflowAdapterResult
		if err == nil {
			adapter, ok := e.adapters.Get(step.Type)
			if !ok {
				err = NewExtensionError(ErrWorkflowStepInvalid, "工作流适配器不可用", step.Type, false, nil)
			} else {
				stepCtx, stepCancel := context.WithTimeout(execCtx, time.Duration(step.TimeoutMS)*time.Millisecond)
				adapterResult, err = executeAdapterSafe(stepCtx, adapter, WorkflowAdapterRequest{Step: step, Input: resolved, Context: values, Scope: request.Scope, Mode: request.Mode, HTTPMocks: request.HTTPMocks, SkillMocks: request.SkillMocks, Limits: request.Workflow.Limits})
				stepCancel()
			}
		}
		if err != nil {
			incrementWorkshopMetric(WorkshopMetricWorkflowStepFailure)
			recordWorkshopErrorMetric(err)
			item.Status = "failed"
			item.Error = asExtensionError(err)
			item.DurationMS = time.Since(started).Milliseconds()
			results = append(results, item)
			if step.OnError.Mode == "use_default" {
				var fallback interface{}
				if json.Unmarshal(step.OnError.Default, &fallback) != nil {
					return WorkflowExecutionResult{Steps: results, SideEffects: effects}, item.Error
				}
				values["steps"].(map[string]interface{})[step.ID] = fallback
				continue
			}
			if step.OnError.Mode == "continue" {
				values["steps"].(map[string]interface{})[step.ID] = nil
				continue
			}
			return WorkflowExecutionResult{Steps: results, SideEffects: effects}, item.Error
		}
		if int64(len(adapterResult.Output)) > request.Workflow.Limits.MaxIntermediateBytes {
			err := NewExtensionError(ErrWorkshopSandboxLimit, "步骤输出超过限制", step.ID, false, nil)
			incrementWorkshopMetric(WorkshopMetricWorkflowStepFailure)
			recordWorkshopErrorMetric(err)
			return WorkflowExecutionResult{Steps: results, SideEffects: effects}, err
		}
		if len(effects)+len(adapterResult.SideEffects) > request.Workflow.Limits.MaxSideEffects {
			err := NewExtensionError(ErrWorkshopSandboxLimit, "副作用数量超过限制", step.ID, false, nil)
			incrementWorkshopMetric(WorkshopMetricWorkflowStepFailure)
			recordWorkshopErrorMetric(err)
			return WorkflowExecutionResult{Steps: results, SideEffects: effects}, err
		}
		var output interface{}
		_ = json.Unmarshal(normalizeJSON(adapterResult.Output), &output)
		values["steps"].(map[string]interface{})[step.ID] = output
		effects = append(effects, adapterResult.SideEffects...)
		item.Status = "succeeded"
		item.OutputSummary = compactSensitiveJSON(adapterResult.Output)
		item.DurationMS = time.Since(started).Milliseconds()
		item.Mocked = adapterResult.Mocked
		results = append(results, item)
	}
	output, err := resolveJSON(request.Workflow.Output, values, request.Workflow.Limits.MaxTemplateLength, false)
	if err != nil {
		return WorkflowExecutionResult{Steps: results, SideEffects: effects}, NewExtensionError(ErrWorkflowOutputInvalid, "工作流输出解析失败", err.Error(), false, err)
	}
	if int64(len(output)) > request.Workflow.Limits.MaxOutputBytes {
		return WorkflowExecutionResult{Steps: results, SideEffects: effects}, NewExtensionError(ErrWorkshopSandboxLimit, "工作流输出超过限制", "", false, nil)
	}
	if e.validator != nil && len(outputSchema) > 0 {
		if err := e.validator.Validate("workflow-output", outputSchema, output); err != nil {
			return WorkflowExecutionResult{Steps: results, SideEffects: effects}, NewExtensionError(ErrWorkflowOutputInvalid, "工作流输出不符合 Schema", err.Error(), false, err)
		}
	}
	return WorkflowExecutionResult{Output: output, Steps: results, SideEffects: effects}, nil
}

func executeAdapterSafe(ctx context.Context, adapter WorkflowStepAdapter, request WorkflowAdapterRequest) (result WorkflowAdapterResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = NewExtensionError(ErrWorkflowStepInvalid, "工作流步骤异常", fmt.Sprint(recovered), false, nil)
		}
	}()
	return adapter.Execute(ctx, request)
}

type ValueStepAdapter struct{ kind string }

func (a ValueStepAdapter) Execute(_ context.Context, request WorkflowAdapterRequest) (WorkflowAdapterResult, error) {
	switch a.kind {
	case "condition":
		var expression ConditionExpression
		if err := json.Unmarshal(request.Input, &expression); err != nil {
			return WorkflowAdapterResult{}, err
		}
		result, err := evalCondition(&expression, request.Context, request.Limits.MaxExpressionDepth)
		raw, _ := json.Marshal(map[string]bool{"result": result})
		return WorkflowAdapterResult{Output: raw}, err
	case "template":
		var input struct {
			Template string `json:"template"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return WorkflowAdapterResult{}, err
		}
		raw, _ := json.Marshal(map[string]string{"text": input.Template})
		return WorkflowAdapterResult{Output: raw}, nil
	case "transform":
		var input map[string]interface{}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return WorkflowAdapterResult{}, err
		}
		result, err := transformJSON(input, request.Context, request.Limits.MaxArrayItems)
		raw, _ := json.Marshal(result)
		return WorkflowAdapterResult{Output: raw}, err
	default:
		return WorkflowAdapterResult{}, fmt.Errorf("unsupported value step")
	}
}

type HTTPWorkflowAdapter struct {
	client   *http.Client
	resolver *net.Resolver
}

func NewHTTPWorkflowAdapter() *HTTPWorkflowAdapter {
	return &HTTPWorkflowAdapter{resolver: net.DefaultResolver}
}
func (a *HTTPWorkflowAdapter) Execute(ctx context.Context, request WorkflowAdapterRequest) (WorkflowAdapterResult, error) {
	var input struct {
		URL              string                 `json:"url"`
		Method           string                 `json:"method"`
		Headers          map[string]interface{} `json:"headers"`
		Query            map[string]interface{} `json:"query"`
		Body             interface{}            `json:"body"`
		ContentType      string                 `json:"contentType"`
		ExpectedStatus   []int                  `json:"expectedStatus"`
		ResponseType     string                 `json:"responseType"`
		MaxResponseBytes int64                  `json:"maxResponseBytes"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return WorkflowAdapterResult{}, err
	}
	input.Method = strings.ToUpper(input.Method)
	if input.Method == "" {
		input.Method = "GET"
	}
	if !map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}[input.Method] {
		return WorkflowAdapterResult{}, fmt.Errorf("HTTP Method 不在白名单中")
	}
	if request.Mode == WorkflowDryRun {
		raw, _ := json.Marshal(map[string]interface{}{"planned": true, "method": input.Method, "url": input.URL})
		return WorkflowAdapterResult{Output: raw, SideEffects: []SideEffectRecord{{Type: "network_request", TargetID: safeURL(input.URL), Confirmed: false}}}, nil
	}
	if request.Mode == WorkflowMocked || request.Mode == WorkflowControlledLive {
		for _, mock := range request.HTTPMocks {
			if strings.EqualFold(mock.Method, input.Method) && mock.URL == input.URL && matchesMockJSON(mock.Query, input.Query) && matchesMockJSON(mock.Headers, input.Headers) && matchesMockJSON(mock.Body, input.Body) {
				if mock.DelayMS > 0 {
					select {
					case <-ctx.Done():
						return WorkflowAdapterResult{}, ctx.Err()
					case <-time.After(time.Duration(mock.DelayMS) * time.Millisecond):
					}
				}
				if mock.Error != "" {
					return WorkflowAdapterResult{}, fmt.Errorf("mock http: %s", mock.Error)
				}
				if !expectedHTTPStatus(mock.Status, input.ExpectedStatus) {
					return WorkflowAdapterResult{}, fmt.Errorf("HTTP 状态码 %d 不符合 expectedStatus", mock.Status)
				}
				raw, _ := json.Marshal(map[string]interface{}{"status": mock.Status, "headers": jsonObject(mock.ResponseHeaders), "body": jsonObject(mock.ResponseBody)})
				return WorkflowAdapterResult{Output: raw, Mocked: true, SideEffects: []SideEffectRecord{{Type: "network_request", TargetID: safeURL(input.URL), Confirmed: false}}}, nil
			}
		}
		return WorkflowAdapterResult{}, fmt.Errorf("HTTP Mock 未命中")
	}
	if err := ValidateNetworkTarget(input.URL); err != nil {
		return WorkflowAdapterResult{}, NewExtensionError(ErrWorkshopNetworkDenied, "网络目标被拒绝", err.Error(), false, err)
	}
	parsed, _ := url.Parse(input.URL)
	query := parsed.Query()
	for key, value := range input.Query {
		query.Set(key, fmt.Sprint(value))
	}
	parsed.RawQuery = query.Encode()
	body := io.Reader(nil)
	if input.Body != nil {
		raw, _ := json.Marshal(input.Body)
		if int64(len(raw)) > request.Limits.MaxInputBytes {
			return WorkflowAdapterResult{}, fmt.Errorf("HTTP 请求体超过限制")
		}
		body = bytes.NewReader(raw)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, input.Method, parsed.String(), body)
	if err != nil {
		return WorkflowAdapterResult{}, err
	}
	if input.ContentType != "" {
		httpRequest.Header.Set("Content-Type", input.ContentType)
	}
	if len(input.Headers) > 32 {
		return WorkflowAdapterResult{}, fmt.Errorf("HTTP Header 数量超过限制")
	}
	for key, value := range input.Headers {
		if strings.EqualFold(key, "Host") {
			return WorkflowAdapterResult{}, fmt.Errorf("禁止覆盖 Host")
		}
		text := fmt.Sprint(value)
		if len(key)+len(text) > 8192 {
			return WorkflowAdapterResult{}, fmt.Errorf("HTTP Header 超过限制")
		}
		httpRequest.Header.Set(key, text)
	}
	client := a.secureClient(request.Limits.MaxHTTPRedirects)
	response, err := client.Do(httpRequest)
	if err != nil {
		return WorkflowAdapterResult{}, err
	}
	defer response.Body.Close()
	responseLimit := request.Limits.MaxHTTPResponseBytes
	if input.MaxResponseBytes > 0 && input.MaxResponseBytes < responseLimit {
		responseLimit = input.MaxResponseBytes
	}
	limited := io.LimitReader(response.Body, responseLimit+1)
	responseBody, err := io.ReadAll(limited)
	if err != nil {
		return WorkflowAdapterResult{}, err
	}
	if int64(len(responseBody)) > responseLimit {
		return WorkflowAdapterResult{}, fmt.Errorf("HTTP 响应体超过限制")
	}
	if !expectedHTTPStatus(response.StatusCode, input.ExpectedStatus) {
		return WorkflowAdapterResult{}, fmt.Errorf("HTTP 状态码 %d 不符合 expectedStatus", response.StatusCode)
	}
	contentType := response.Header.Get("Content-Type")
	var value interface{}
	isJSON := strings.Contains(strings.ToLower(contentType), "json")
	if input.ResponseType == "text" {
		value = string(responseBody)
	} else if !isJSON {
		return WorkflowAdapterResult{}, fmt.Errorf("非 JSON 响应必须显式声明 responseType=text")
	} else {
		if json.Unmarshal(responseBody, &value) != nil {
			return WorkflowAdapterResult{}, fmt.Errorf("HTTP JSON 响应无效")
		}
	}
	raw, _ := json.Marshal(map[string]interface{}{"status": response.StatusCode, "headers": safeHeaders(response.Header), "body": value})
	return WorkflowAdapterResult{Output: raw, SideEffects: []SideEffectRecord{{Type: "network_request", TargetID: safeURL(input.URL), Confirmed: true}}}, nil
}

func expectedHTTPStatus(status int, expected []int) bool {
	if len(expected) == 0 {
		return status >= 200 && status < 300
	}
	for _, value := range expected {
		if status == value {
			return true
		}
	}
	return false
}

func (a *HTTPWorkflowAdapter) secureClient(maxRedirects int) *http.Client {
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := a.resolver.LookupIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("DNS 解析失败")
		}
		for _, ip := range ips {
			if deniedIP(ip) {
				return nil, NewExtensionError(ErrWorkshopNetworkDenied, "DNS 解析结果被拒绝", ip.String(), false, nil)
			}
		}
		dialer := net.Dialer{Timeout: 10 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}}
	redirects := 0
	return &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		redirects++
		if redirects > maxRedirects {
			return fmt.Errorf("HTTP 重定向次数超过限制")
		}
		return ValidateNetworkTarget(req.URL.String())
	}}
}

type SkillWorkflowAdapter struct{ executor SkillExecutor }
type workflowCallStateKey struct{}
type workflowCallState struct {
	depth int
	calls int
}

func (a SkillWorkflowAdapter) Execute(ctx context.Context, request WorkflowAdapterRequest) (WorkflowAdapterResult, error) {
	var input struct {
		SkillID   string          `json:"skillId"`
		Input     json.RawMessage `json:"input"`
		Optional  bool            `json:"optional"`
		TimeoutMS int64           `json:"timeoutMs"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return WorkflowAdapterResult{}, err
	}
	if request.Mode == WorkflowDryRun {
		raw, _ := json.Marshal(map[string]interface{}{"planned": true, "skillId": input.SkillID})
		return WorkflowAdapterResult{Output: raw}, nil
	}
	if request.Mode == WorkflowMocked || request.Mode == WorkflowControlledLive {
		for _, mock := range request.SkillMocks {
			if mock.SkillID == input.SkillID && matchesMockJSON(mock.Input, jsonObject(input.Input)) {
				if mock.DelayMS > 0 {
					select {
					case <-ctx.Done():
						return WorkflowAdapterResult{}, ctx.Err()
					case <-time.After(time.Duration(mock.DelayMS) * time.Millisecond):
					}
				}
				if mock.Error != nil {
					return WorkflowAdapterResult{}, mock.Error
				}
				if mock.Status == RunFailed || mock.Status == RunTimedOut || mock.Status == RunCancelled {
					return WorkflowAdapterResult{}, NewExtensionError(ErrWorkshopTestFailed, "Skill Mock 返回失败状态", string(mock.Status), false, nil)
				}
				return WorkflowAdapterResult{Output: normalizeJSON(mock.Output), SideEffects: mock.SideEffects, Mocked: true}, nil
			}
		}
		return WorkflowAdapterResult{}, fmt.Errorf("受控测试中的 Skill Mock 未命中")
	}
	if a.executor == nil {
		return WorkflowAdapterResult{}, fmt.Errorf("Skill Executor 不可用")
	}
	state, _ := ctx.Value(workflowCallStateKey{}).(*workflowCallState)
	if state == nil {
		state = &workflowCallState{}
	}
	if state.depth >= request.Limits.MaxSkillCallDepth || state.calls >= request.Limits.MaxSkillCalls {
		return WorkflowAdapterResult{}, NewExtensionError(ErrWorkshopSandboxLimit, "Skill 调用深度或次数超过限制", input.SkillID, false, nil)
	}
	next := &workflowCallState{depth: state.depth + 1, calls: state.calls + 1}
	result, err := a.executor.Execute(context.WithValue(ctx, workflowCallStateKey{}, next), ExecuteSkillRequest{SkillID: input.SkillID, Input: normalizeJSON(input.Input), Scope: request.Scope})
	if err != nil && input.Optional && asExtensionError(err).Code == ErrSkillNotFound {
		return WorkflowAdapterResult{Output: json.RawMessage(`{"skipped":true,"reason":"optional_dependency_unavailable"}`)}, nil
	}
	return WorkflowAdapterResult{Output: result.Output, SideEffects: result.SideEffects}, err
}

type SideEffectHost interface {
	ExecuteWorkflowSideEffect(context.Context, string, json.RawMessage, ExecutionScope) (json.RawMessage, []SideEffectRecord, error)
}
type WorkflowHostAdapter struct {
	Schedule            func(context.Context, json.RawMessage, ExecutionScope) (json.RawMessage, []SideEffectRecord, error)
	Notification        func(context.Context, json.RawMessage, ExecutionScope) (json.RawMessage, []SideEffectRecord, error)
	MemoryCandidate     func(context.Context, json.RawMessage, ExecutionScope) (json.RawMessage, []SideEffectRecord, error)
	ContextContribution func(context.Context, json.RawMessage, ExecutionScope) (json.RawMessage, []SideEffectRecord, error)
}

func (h *WorkflowHostAdapter) ExecuteWorkflowSideEffect(ctx context.Context, kind string, input json.RawMessage, scope ExecutionScope) (json.RawMessage, []SideEffectRecord, error) {
	var execute func(context.Context, json.RawMessage, ExecutionScope) (json.RawMessage, []SideEffectRecord, error)
	switch kind {
	case "schedule":
		execute = h.Schedule
	case "notification":
		execute = h.Notification
	case "memory_candidate":
		execute = h.MemoryCandidate
	case "context_contribution":
		execute = h.ContextContribution
	}
	if execute == nil {
		return nil, nil, fmt.Errorf("宿主未配置 %s 适配器", kind)
	}
	return execute(ctx, input, scope)
}

type SideEffectWorkflowAdapter struct {
	kind string
	host SideEffectHost
}

func (a SideEffectWorkflowAdapter) Execute(ctx context.Context, request WorkflowAdapterRequest) (WorkflowAdapterResult, error) {
	effectType := map[string]string{"schedule": "schedule_create", "notification": "notification_send", "memory_candidate": "memory_candidate_write", "context_contribution": "context_injection"}[a.kind]
	if request.Mode != WorkflowProduction {
		raw, _ := json.Marshal(map[string]interface{}{"planned": true, "type": a.kind})
		return WorkflowAdapterResult{Output: raw, SideEffects: []SideEffectRecord{{Type: effectType, Confirmed: false}}, Mocked: request.Mode == WorkflowMocked}, nil
	}
	if a.host == nil {
		return WorkflowAdapterResult{}, fmt.Errorf("宿主未配置 %s 适配器", a.kind)
	}
	output, effects, err := a.host.ExecuteWorkflowSideEffect(ctx, a.kind, request.Input, request.Scope)
	return WorkflowAdapterResult{Output: output, SideEffects: effects}, err
}

func BuildWorkflowAdapters(executor SkillExecutor, host SideEffectHost) *WorkflowAdapterRegistry {
	registry := NewWorkflowAdapterRegistry()
	for _, kind := range []string{"condition", "transform", "template"} {
		_ = registry.Register(kind, ValueStepAdapter{kind: kind})
	}
	_ = registry.Register("http", NewHTTPWorkflowAdapter())
	_ = registry.Register("call_skill", SkillWorkflowAdapter{executor: executor})
	for _, kind := range []string{"schedule", "notification", "memory_candidate", "context_contribution"} {
		_ = registry.Register(kind, SideEffectWorkflowAdapter{kind: kind, host: host})
	}
	return registry
}
func safeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[invalid-url]"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
func safeHeaders(headers http.Header) map[string]string {
	result := map[string]string{}
	for key, values := range headers {
		if isSensitiveKey(key) {
			result[key] = "[REDACTED]"
		} else if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

func matchesMockJSON(expected json.RawMessage, actual interface{}) bool {
	if len(bytes.TrimSpace(expected)) == 0 || string(bytes.TrimSpace(expected)) == "null" {
		return true
	}
	raw, err := json.Marshal(actual)
	if err != nil {
		return false
	}
	return jsonEqual(expected, raw)
}
func jsonObject(raw json.RawMessage) interface{} {
	var value interface{}
	if json.Unmarshal(normalizeJSON(raw), &value) != nil {
		return nil
	}
	return value
}
