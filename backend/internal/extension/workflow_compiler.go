package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var workflowStepIDPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
var workflowReferencePattern = regexp.MustCompile(`(?:input|config|secrets|steps|runtime)(?:\.[a-zA-Z_][a-zA-Z0-9_-]*)+`)
var forbiddenDraftPattern = regexp.MustCompile(`(?i)(?:package\s+main|func\s+(?:\([^)]*\)\s*)?\w+\s*\(|import\s*\(|function\s+\w*\s*\(|=>\s*[{(]|\b(?:eval|exec|__import__)\s*\(|\bdef\s+\w+\s*\(|\bclass\s+\w+\s*[:(]|\b(?:subprocess\.|os\.system\s*\()|<\/?[a-z][^>]*>|javascript:|\bSELECT\s+.+\bFROM\b|\b(?:INSERT\s+INTO|UPDATE\s+\w+\s+SET|DELETE\s+FROM|CREATE\s+TABLE|ALTER\s+TABLE|DROP\s+TABLE)\b|#!/bin/|\b(?:powershell|cmd\.exe|bash|sh)\b)`)
var secretPattern = regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|refresh[_-]?token|authorization|bearer|password|passwd|secret|private[_-]?key|client[_-]?secret|cookie|session|webhook[_-]?token)\s*[=:]\s*["']?[A-Za-z0-9_./+\-=]{8,}`)

var allowedWorkflowSteps = map[string]bool{"http": true, "condition": true, "transform": true, "template": true, "call_skill": true, "schedule": true, "notification": true, "memory_candidate": true, "context_contribution": true}
var reservedStepIDs = map[string]bool{"input": true, "config": true, "secrets": true, "steps": true, "runtime": true}

func DefaultWorkflowLimits() WorkflowLimits {
	return WorkflowLimits{MaxSteps: 32, MaxExecutionDurationMS: 30000, MaxStepDurationMS: 10000, MaxInputBytes: 262144, MaxOutputBytes: 262144, MaxIntermediateBytes: 524288, MaxHTTPResponseBytes: 1048576, MaxHTTPRedirects: 3, MaxSkillCallDepth: 4, MaxSkillCalls: 16, MaxArrayItems: 1000, MaxExpressionDepth: 16, MaxTemplateLength: 32768, MaxEventsEmitted: 16, MaxSchedulesCreated: 8, MaxSideEffects: 32}
}

func effectiveWorkflowLimits(requested WorkflowLimits) WorkflowLimits {
	host := DefaultWorkflowLimits()
	minInt := func(value, fallback, maximum int) int {
		if value <= 0 {
			return fallback
		}
		if value > maximum {
			return maximum
		}
		return value
	}
	minInt64 := func(value, fallback, maximum int64) int64 {
		if value <= 0 {
			return fallback
		}
		if value > maximum {
			return maximum
		}
		return value
	}
	return WorkflowLimits{MaxSteps: minInt(requested.MaxSteps, host.MaxSteps, host.MaxSteps), MaxExecutionDurationMS: minInt64(requested.MaxExecutionDurationMS, host.MaxExecutionDurationMS, host.MaxExecutionDurationMS), MaxStepDurationMS: minInt64(requested.MaxStepDurationMS, host.MaxStepDurationMS, host.MaxStepDurationMS), MaxInputBytes: minInt64(requested.MaxInputBytes, host.MaxInputBytes, host.MaxInputBytes), MaxOutputBytes: minInt64(requested.MaxOutputBytes, host.MaxOutputBytes, host.MaxOutputBytes), MaxIntermediateBytes: minInt64(requested.MaxIntermediateBytes, host.MaxIntermediateBytes, host.MaxIntermediateBytes), MaxHTTPResponseBytes: minInt64(requested.MaxHTTPResponseBytes, host.MaxHTTPResponseBytes, host.MaxHTTPResponseBytes), MaxHTTPRedirects: minInt(requested.MaxHTTPRedirects, host.MaxHTTPRedirects, host.MaxHTTPRedirects), MaxSkillCallDepth: minInt(requested.MaxSkillCallDepth, host.MaxSkillCallDepth, host.MaxSkillCallDepth), MaxSkillCalls: minInt(requested.MaxSkillCalls, host.MaxSkillCalls, host.MaxSkillCalls), MaxArrayItems: minInt(requested.MaxArrayItems, host.MaxArrayItems, host.MaxArrayItems), MaxExpressionDepth: minInt(requested.MaxExpressionDepth, host.MaxExpressionDepth, host.MaxExpressionDepth), MaxTemplateLength: minInt(requested.MaxTemplateLength, host.MaxTemplateLength, host.MaxTemplateLength), MaxEventsEmitted: minInt(requested.MaxEventsEmitted, host.MaxEventsEmitted, host.MaxEventsEmitted), MaxSchedulesCreated: minInt(requested.MaxSchedulesCreated, host.MaxSchedulesCreated, host.MaxSchedulesCreated), MaxSideEffects: minInt(requested.MaxSideEffects, host.MaxSideEffects, host.MaxSideEffects)}
}

type WorkflowCompiler struct{ registry SkillRegistry }

func NewWorkflowCompiler(registry SkillRegistry) *WorkflowCompiler {
	return &WorkflowCompiler{registry: registry}
}

func (c *WorkflowCompiler) AnalyzeDependencyCycles(ctx context.Context, currentSkillID string, dependencies []ResolvedSkillDependency) []AnalysisIssue {
	if c.registry == nil || strings.TrimSpace(currentSkillID) == "" {
		return nil
	}
	completed := map[string]bool{}
	active := map[string]int{}
	var visit func(string, []string) *AnalysisIssue
	visit = func(skillID string, path []string) *AnalysisIssue {
		if skillID == currentSkillID {
			cycle := append(append([]string(nil), path...), skillID)
			return &AnalysisIssue{Level: "error", Code: ErrWorkshopDependencyCycle, Message: "检测到 Skill 调用循环: " + strings.Join(cycle, " -> "), Path: "workflow.steps"}
		}
		if index, ok := active[skillID]; ok {
			cycle := append(append([]string(nil), path[index:]...), skillID)
			return &AnalysisIssue{Level: "error", Code: ErrWorkshopDependencyCycle, Message: "检测到 Skill 调用循环: " + strings.Join(cycle, " -> "), Path: "workflow.steps"}
		}
		if completed[skillID] {
			return nil
		}
		registered, err := c.registry.Get(ctx, skillID)
		if err != nil {
			return &AnalysisIssue{Level: "error", Code: ErrWorkshopDependencyNotFound, Message: "依赖 Skill 不存在或已卸载: " + skillID, Path: "workflow.steps"}
		}
		active[skillID] = len(path)
		nextPath := append(append([]string(nil), path...), skillID)
		next := append([]string(nil), registered.Definition.Dependencies...)
		sort.Strings(next)
		for _, dependencyID := range next {
			if issue := visit(dependencyID, nextPath); issue != nil {
				return issue
			}
		}
		delete(active, skillID)
		completed[skillID] = true
		return nil
	}
	direct := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		direct = append(direct, dependency.SkillID)
	}
	sort.Strings(direct)
	for _, skillID := range direct {
		if issue := visit(skillID, []string{currentSkillID}); issue != nil {
			return []AnalysisIssue{*issue}
		}
	}
	return nil
}

func (c *WorkflowCompiler) Compile(ctx context.Context, workflow WorkflowDefinition) (CompiledWorkflow, []AnalysisIssue, error) {
	issues := []AnalysisIssue{}
	limits := effectiveWorkflowLimits(workflow.Limits)
	if workflow.SchemaVersion == "" {
		workflow.SchemaVersion = "1.0.0"
	}
	if len(workflow.Steps) == 0 {
		issues = append(issues, AnalysisIssue{Level: "error", Code: "WORKFLOW_EMPTY", Message: "工作流不能为空", Path: "workflow.steps"})
	}
	if len(workflow.Steps) > limits.MaxSteps {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSandboxLimit, Message: "工作流步骤超过宿主限制", Path: "workflow.steps"})
	}
	if len(workflow.Output) == 0 {
		issues = append(issues, AnalysisIssue{Level: "error", Code: "WORKFLOW_OUTPUT_REQUIRED", Message: "工作流最终输出不能为空", Path: "workflow.output"})
	}
	seen := map[string]int{}
	capByStep := map[string][]string{}
	capSet := map[string]bool{}
	dependencies := []ResolvedSkillDependency{}
	hasSideEffects := false
	idempotent := true
	compiledSteps := make([]CompiledStep, 0, len(workflow.Steps))
	for index, step := range workflow.Steps {
		path := fmt.Sprintf("workflow.steps[%d]", index)
		if !workflowStepIDPattern.MatchString(step.ID) || reservedStepIDs[step.ID] {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "步骤 ID 非法或为保留名称", Path: path + ".id", StepID: step.ID})
		}
		if previous, ok := seen[step.ID]; ok {
			issues = append(issues, AnalysisIssue{Level: "error", Code: "WORKFLOW_DUPLICATE_STEP_ID", Message: fmt.Sprintf("步骤 ID 与第 %d 步重复", previous+1), Path: path + ".id", StepID: step.ID})
		}
		seen[step.ID] = index
		if !allowedWorkflowSteps[step.Type] {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "不支持的工作流步骤", Path: path + ".type", StepID: step.ID})
			continue
		}
		if !json.Valid(normalizeJSON(step.Input)) {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "步骤输入不是合法 JSON", Path: path + ".input", StepID: step.ID})
		}
		mode := step.OnError.Mode
		if mode == "" {
			mode = "fail"
		}
		if mode != "fail" && mode != "continue" && mode != "use_default" {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "不支持的错误策略", Path: path + ".onError.mode", StepID: step.ID})
		}
		if mode == "use_default" && len(step.OnError.Default) == 0 {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "use_default 必须提供默认结果", Path: path + ".onError.default", StepID: step.ID})
		}
		step.OnError.Mode = mode
		for _, ref := range workflowReferencePattern.FindAllString(string(step.Input), -1) {
			if err := validateWorkflowReference(ref, index, seen); err != nil {
				issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowReferenceInvalid, Message: err.Error(), Path: path + ".input", StepID: step.ID})
			}
		}
		if step.When != nil {
			if err := validateCondition(step.When, 0, limits.MaxExpressionDepth); err != nil {
				issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: err.Error(), Path: path + ".when", StepID: step.ID})
			}
		}
		caps, effects, stepIdempotent, dependencyIssues, dependency := c.analyzeStep(ctx, step)
		issues = append(issues, dependencyIssues...)
		if dependency != nil {
			dependencies = append(dependencies, *dependency)
		}
		capByStep[step.ID] = caps
		for _, capability := range caps {
			capSet[capability] = true
		}
		if len(effects) > 0 {
			hasSideEffects = true
		}
		if !stepIdempotent {
			idempotent = false
		}
		if mode == "continue" && len(effects) > 0 {
			issues = append(issues, AnalysisIssue{Level: "error", Code: "WORKFLOW_SIDE_EFFECT_CONTINUE", Message: "副作用步骤不能使用 continue", Path: path + ".onError", StepID: step.ID})
		}
		compiledSteps = append(compiledSteps, CompiledStep{ID: step.ID, Type: step.Type, Input: stableJSON(step.Input), When: step.When, OnError: step.OnError, TimeoutMS: compiledStepTimeout(step, limits.MaxStepDurationMS)})
	}
	for _, ref := range workflowReferencePattern.FindAllString(string(workflow.Output), -1) {
		if err := validateFinalReference(ref, seen); err != nil {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowReferenceInvalid, Message: err.Error(), Path: "workflow.output"})
		}
	}
	capabilities := sortedKeys(capSet)
	sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].SkillID < dependencies[j].SkillID })
	base := struct {
		SchemaVersion  string                    `json:"schemaVersion"`
		Steps          []CompiledStep            `json:"steps"`
		Output         json.RawMessage           `json:"output"`
		Capabilities   []string                  `json:"capabilities"`
		Dependencies   []ResolvedSkillDependency `json:"dependencies"`
		Limits         WorkflowLimits            `json:"limits"`
		HasSideEffects bool                      `json:"hasSideEffects"`
		Idempotent     bool                      `json:"idempotent"`
	}{workflow.SchemaVersion, compiledSteps, stableJSON(workflow.Output), capabilities, dependencies, limits, hasSideEffects, idempotent}
	raw, _ := json.Marshal(base)
	hash := sha256.Sum256(raw)
	compiled := CompiledWorkflow{SchemaVersion: base.SchemaVersion, Steps: base.Steps, Output: base.Output, Capabilities: capabilities, Dependencies: dependencies, Limits: limits, HasSideEffects: hasSideEffects, Idempotent: idempotent, Checksum: hex.EncodeToString(hash[:])}
	if hasErrorIssues(issues) {
		return compiled, issues, NewExtensionError(ErrWorkshopWorkflowInvalid, "工作流编译失败", "请修复静态检查错误", false, nil)
	}
	return compiled, issues, nil
}

func (c *WorkflowCompiler) analyzeStep(ctx context.Context, step WorkflowStep) ([]string, []string, bool, []AnalysisIssue, *ResolvedSkillDependency) {
	switch step.Type {
	case "http":
		var input struct {
			URL              string                 `json:"url"`
			Method           string                 `json:"method"`
			TimeoutMS        int64                  `json:"timeoutMs"`
			MaxResponseBytes int64                  `json:"maxResponseBytes"`
			Headers          map[string]interface{} `json:"headers"`
			AllowDelete      bool                   `json:"allowDelete"`
			ExpectedStatus   []int                  `json:"expectedStatus"`
			ResponseType     string                 `json:"responseType"`
		}
		_ = json.Unmarshal(step.Input, &input)
		issues := []AnalysisIssue{}
		if err := ValidateNetworkTarget(input.URL); err != nil {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopNetworkDenied, Message: err.Error(), StepID: step.ID})
		}
		for key, value := range input.Headers {
			if strings.EqualFold(key, "Host") || (strings.EqualFold(key, "Authorization") && !isSecretReference(value)) {
				issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSecretDetected, Message: "禁止明文 Host 或 Authorization Header", StepID: step.ID})
			}
		}
		method := strings.ToUpper(input.Method)
		if method == "" {
			method = "GET"
		}
		if !map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}[method] {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "HTTP Method 不在白名单中", StepID: step.ID})
		}
		if method == "DELETE" && !input.AllowDelete {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopPermissionRequired, Message: "DELETE 必须显式声明 allowDelete 高风险行为", StepID: step.ID})
		}
		if input.TimeoutMS < 0 || input.TimeoutMS > DefaultWorkflowLimits().MaxStepDurationMS {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSandboxLimit, Message: "HTTP timeoutMs 超出宿主限制", StepID: step.ID})
		}
		if input.MaxResponseBytes < 0 || input.MaxResponseBytes > DefaultWorkflowLimits().MaxHTTPResponseBytes {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSandboxLimit, Message: "HTTP maxResponseBytes 超出宿主限制", StepID: step.ID})
		}
		for _, status := range input.ExpectedStatus {
			if status < 100 || status > 599 {
				issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "HTTP expectedStatus 必须在 100 到 599 之间", StepID: step.ID})
			}
		}
		if input.ResponseType != "" && input.ResponseType != "json" && input.ResponseType != "text" {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "HTTP responseType 只能为 json 或 text", StepID: step.ID})
		}
		for _, path := range secretReferencePaths(step.Input) {
			if !strings.EqualFold(path, "headers.Authorization") {
				issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSecretDetected, Message: "Secret 只能注入 HTTP Authorization Header", Path: path, StepID: step.ID})
			}
		}
		return []string{"network.https"}, []string{"network_request"}, method == "GET", issues, nil
	case "schedule":
		var input struct {
			IdempotencyKey string `json:"idempotencyKey"`
			Timezone       string `json:"timezone"`
			DueAt          string `json:"dueAt"`
			DueTime        string `json:"due_time"`
			Cron           string `json:"cron"`
		}
		_ = json.Unmarshal(step.Input, &input)
		issues := []AnalysisIssue{}
		if strings.TrimSpace(input.IdempotencyKey) == "" {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "schedule 必须提供幂等键", StepID: step.ID})
		}
		if strings.TrimSpace(input.Timezone) == "" || strings.TrimSpace(input.DueAt) == "" && strings.TrimSpace(input.DueTime) == "" && strings.TrimSpace(input.Cron) == "" {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "schedule 必须提供时区和 dueAt 或 cron", StepID: step.ID})
		}
		return []string{"scheduler.own.manage"}, []string{"schedule_create"}, true, issues, nil
	case "notification":
		var input struct {
			Content   string `json:"content"`
			Recipient string `json:"recipient"`
		}
		_ = json.Unmarshal(step.Input, &input)
		issues := []AnalysisIssue{}
		if strings.TrimSpace(input.Content) == "" && !strings.Contains(string(step.Input), "{{") || len([]rune(input.Content)) > 4000 {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "notification 内容必须为 1 到 4000 个字符", StepID: step.ID})
		}
		if input.Recipient != "" && input.Recipient != "current_conversation" {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSessionForbidden, Message: "notification 只能发送到当前会话", StepID: step.ID})
		}
		return []string{"notification.send"}, []string{"notification_send"}, false, issues, nil
	case "memory_candidate":
		var input struct {
			Source string `json:"source"`
		}
		_ = json.Unmarshal(step.Input, &input)
		issues := []AnalysisIssue{}
		if strings.TrimSpace(input.Source) == "" {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkflowStepInvalid, Message: "memory_candidate 必须提供来源文本", StepID: step.ID})
		}
		return []string{"memory.candidate.write"}, []string{"memory_candidate_write"}, false, issues, nil
	case "context_contribution":
		var input struct {
			TokenLimit int `json:"tokenLimit"`
		}
		_ = json.Unmarshal(step.Input, &input)
		issues := []AnalysisIssue{}
		if input.TokenLimit < 1 || input.TokenLimit > 1024 {
			issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSandboxLimit, Message: "context_contribution tokenLimit 必须为 1 到 1024", StepID: step.ID})
		}
		return []string{"context.inject"}, []string{"context_injection"}, true, issues, nil
	case "transform":
		var input struct {
			Op string `json:"op"`
		}
		_ = json.Unmarshal(step.Input, &input)
		if !map[string]bool{"pick": true, "omit": true, "rename": true, "set": true, "merge": true, "flatten": true, "array_map": true, "array_filter": true, "array_take": true, "array_sort": true, "to_string": true, "to_number": true, "to_boolean": true}[input.Op] {
			return nil, nil, true, []AnalysisIssue{{Level: "error", Code: ErrWorkflowStepInvalid, Message: "transform 操作不在白名单中", StepID: step.ID}}, nil
		}
		return nil, nil, true, nil, nil
	case "call_skill":
		var input struct {
			SkillID  string `json:"skillId"`
			Optional bool   `json:"optional"`
		}
		_ = json.Unmarshal(step.Input, &input)
		if input.SkillID == "" {
			return nil, nil, false, []AnalysisIssue{{Level: "error", Code: ErrWorkshopDependencyNotFound, Message: "调用的 Skill 不存在", StepID: step.ID}}, nil
		}
		if c.registry == nil {
			if input.Optional {
				return nil, nil, true, []AnalysisIssue{{Level: "warning", Code: ErrWorkshopDependencyNotFound, Message: "可选 Skill 当前不可用", StepID: step.ID}}, nil
			}
			return nil, nil, false, []AnalysisIssue{{Level: "error", Code: ErrWorkshopDependencyNotFound, Message: "调用的 Skill 不存在", StepID: step.ID}}, nil
		}
		registered, err := c.registry.Get(ctx, input.SkillID)
		if err != nil {
			if input.Optional {
				return nil, nil, true, []AnalysisIssue{{Level: "warning", Code: ErrWorkshopDependencyNotFound, Message: "可选 Skill 当前不可用", StepID: step.ID}}, nil
			}
			return nil, nil, false, []AnalysisIssue{{Level: "error", Code: ErrWorkshopDependencyNotFound, Message: "调用的 Skill 不存在", StepID: step.ID}}, nil
		}
		dependency := &ResolvedSkillDependency{SkillID: input.SkillID, Version: registered.Definition.Version, Capabilities: append([]string(nil), registered.Definition.Capabilities...), HasSideEffects: registered.Definition.HasSideEffects, Idempotent: registered.Definition.Idempotent}
		effects := []string{}
		if dependency.HasSideEffects {
			effects = append(effects, "skill_side_effect")
		}
		return dependency.Capabilities, effects, dependency.Idempotent, nil, dependency
	default:
		return nil, nil, true, nil, nil
	}
}

func compiledStepTimeout(step WorkflowStep, maximum int64) int64 {
	if step.Type != "http" {
		return maximum
	}
	var input struct {
		TimeoutMS int64 `json:"timeoutMs"`
	}
	_ = json.Unmarshal(step.Input, &input)
	if input.TimeoutMS > 0 && input.TimeoutMS < maximum {
		return input.TimeoutMS
	}
	return maximum
}

func validateWorkflowReference(ref string, current int, seen map[string]int) error {
	parts := strings.Split(ref, ".")
	if containsForbiddenPath(parts) {
		return fmt.Errorf("引用包含禁止路径: %s", ref)
	}
	if parts[0] == "steps" {
		if len(parts) < 3 {
			return fmt.Errorf("步骤引用不完整: %s", ref)
		}
		index, ok := seen[parts[1]]
		if !ok || index >= current {
			return fmt.Errorf("步骤引用未定义或指向未来步骤: %s", ref)
		}
	}
	if parts[0] == "runtime" {
		allowed := map[string]bool{"traceId": true, "runId": true, "characterId": true, "conversationId": true, "channel": true}
		if len(parts) != 2 || !allowed[parts[1]] {
			return fmt.Errorf("运行时引用不允许: %s", ref)
		}
	}
	return nil
}

func validateFinalReference(ref string, seen map[string]int) error {
	parts := strings.Split(ref, ".")
	if len(parts) == 2 && parts[0] == "steps" {
		if _, ok := seen[parts[1]]; !ok {
			return fmt.Errorf("步骤引用不存在: %s", ref)
		}
		return nil
	}
	return validateWorkflowReference(ref, len(seen)+1, seen)
}
func containsForbiddenPath(parts []string) bool {
	for _, part := range parts {
		lower := strings.ToLower(part)
		if lower == "__proto__" || lower == "prototype" || lower == "constructor" || lower == "env" {
			return true
		}
	}
	return false
}
func stableJSON(raw json.RawMessage) json.RawMessage {
	var value interface{}
	if json.Unmarshal(normalizeJSON(raw), &value) != nil {
		return normalizeJSON(raw)
	}
	out, _ := json.Marshal(value)
	return out
}
func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func hasErrorIssues(issues []AnalysisIssue) bool {
	for _, issue := range issues {
		if issue.Level == "error" {
			return true
		}
	}
	return false
}
func isSecretReference(value interface{}) bool {
	object, ok := value.(map[string]interface{})
	if !ok {
		return false
	}
	text, ok := object["$secret"].(string)
	return ok && strings.TrimSpace(text) != ""
}

func secretReferencePaths(raw json.RawMessage) []string {
	var value interface{}
	if json.Unmarshal(normalizeJSON(raw), &value) != nil {
		return nil
	}
	paths := []string{}
	var walk func(interface{}, string)
	walk = func(current interface{}, path string) {
		switch typed := current.(type) {
		case map[string]interface{}:
			if _, ok := typed["$secret"].(string); ok {
				paths = append(paths, path)
				return
			}
			for key, item := range typed {
				next := key
				if path != "" {
					next = path + "." + key
				}
				walk(item, next)
			}
		case []interface{}:
			for index, item := range typed {
				walk(item, fmt.Sprintf("%s[%d]", path, index))
			}
		}
	}
	walk(value, "")
	sort.Strings(paths)
	return paths
}

func secretReferenceNames(raw json.RawMessage) []string {
	var value interface{}
	if json.Unmarshal(normalizeJSON(raw), &value) != nil {
		return nil
	}
	names := map[string]bool{}
	var walk func(interface{})
	walk = func(current interface{}) {
		switch typed := current.(type) {
		case map[string]interface{}:
			if name, ok := typed["$secret"].(string); ok {
				names[strings.TrimSpace(name)] = true
				return
			}
			for _, item := range typed {
				walk(item)
			}
		case []interface{}:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return sortedKeys(names)
}

func ValidateNetworkTarget(rawURL string) error {
	if strings.Contains(rawURL, "{{") {
		return fmt.Errorf("动态 URL 必须在受控域名策略中显式授权")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return fmt.Errorf("仅允许完整 HTTPS URL")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" {
		return fmt.Errorf("目标主机被网络策略拒绝")
	}
	if ip := net.ParseIP(host); ip != nil && deniedIP(ip) {
		return fmt.Errorf("目标 IP 被网络策略拒绝")
	}
	return nil
}

func deniedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	for _, raw := range []string{"169.254.169.254/32", "100.100.100.200/32", "fd00:ec2::254/128"} {
		_, block, _ := net.ParseCIDR(raw)
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func ScanWorkshopSecrets(raw []byte) []AnalysisIssue {
	issues := []AnalysisIssue{}
	text := string(raw)
	if forbiddenDraftPattern.MatchString(text) {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopGenerationOutputInvalid, Message: "草案包含禁止的源码、脚本或 SQL 内容"})
	}
	if secretPattern.MatchString(text) {
		issues = append(issues, AnalysisIssue{Level: "error", Code: ErrWorkshopSecretDetected, Message: "检测到疑似明文 Secret，请改用 Secret Reference"})
	}
	return issues
}
