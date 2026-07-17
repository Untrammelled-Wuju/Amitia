package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type WorkshopModelGenerator interface {
	GenerateWorkshopJSON(context.Context, string, string) (string, string, string, error)
}
type WorkshopGenerator struct {
	model       WorkshopModelGenerator
	registry    SkillRegistry
	maxAttempts int
}

type WorkshopInstructionDraft struct {
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Body             string            `json:"body"`
	References       map[string]string `json:"references"`
	Assets           map[string]string `json:"assets"`
	DisplayName      string            `json:"displayName"`
	ShortDescription string            `json:"shortDescription"`
}

func (g *WorkshopGenerator) GenerateInstruction(ctx context.Context, requirement string) (WorkshopInstructionDraft, error) {
	if g.model == nil {
		return WorkshopInstructionDraft{}, NewExtensionError(ErrWorkshopGenerationFailed, "未配置可用的模型 Provider", "请先在模型设置中配置并启用模型", false, nil)
	}
	if issues := ScanWorkshopSecrets([]byte(requirement)); hasErrorIssues(issues) {
		return WorkshopInstructionDraft{}, NewExtensionError(ErrWorkshopSecretDetected, "需求中包含疑似明文 Secret", "请使用 Secret 引用名称，不要粘贴密钥", false, nil)
	}
	if forbiddenDraftPattern.MatchString(requirement) {
		return WorkshopInstructionDraft{}, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "instructions Skill 不得包含代码或脚本执行需求", "", false, nil)
	}
	prompt := `你是 Amitia Agent Skill 工坊。只返回 JSON，字段必须且只能是 name、description、body、references、assets、displayName、shortDescription。name 必须是小写短横线格式，description 必须说明功能和适用触发场景，body 是清晰的 Markdown 命令式流程。references 和 assets 是相对文件名到纯文本内容的对象，只能放知识材料和模板。禁止 scripts、源码、Shell、Python、Node、PowerShell、MCP、allowed-tools、Secret、安装说明、README 和 CHANGELOG。`
	var last error
	for attempt := 0; attempt < g.maxAttempts; attempt++ {
		raw, _, _, err := g.model.GenerateWorkshopJSON(ctx, prompt, requirement)
		if err != nil {
			last = err
			continue
		}
		var draft WorkshopInstructionDraft
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&draft); err != nil {
			last = err
			continue
		}
		if !agentSkillNamePattern.MatchString(draft.Name) || strings.TrimSpace(draft.Body) == "" || validateAgentSkillDescription(draft.Description) != nil {
			last = fmt.Errorf("生成内容不符合 Agent Skill 结构")
			continue
		}
		invalid := false
		for name := range draft.References {
			if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "" {
				last = fmt.Errorf("references 文件名非法")
				invalid = true
			}
		}
		for name := range draft.Assets {
			if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "" {
				last = fmt.Errorf("assets 文件名非法")
				invalid = true
			}
		}
		if invalid {
			continue
		}
		return draft, nil
	}
	return WorkshopInstructionDraft{}, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "模型多次返回无效 instructions Skill", sanitizeGenerationError(last), false, last)
}

func NewWorkshopGenerator(model WorkshopModelGenerator, registry SkillRegistry) *WorkshopGenerator {
	return &WorkshopGenerator{model: model, registry: registry, maxAttempts: 3}
}
func (g *WorkshopGenerator) SetModel(model WorkshopModelGenerator) { g.model = model }

func (g *WorkshopGenerator) Generate(ctx context.Context, requirement string) (ExtensionDraft, WorkshopPlan, string, string, string, error) {
	if g.model == nil {
		return ExtensionDraft{}, WorkshopPlan{}, "", "", "", NewExtensionError(ErrWorkshopGenerationFailed, "未配置可用的模型 Provider", "请先在模型设置中配置并启用模型", false, nil)
	}
	if forbiddenDraftPattern.MatchString(requirement) {
		return ExtensionDraft{}, WorkshopPlan{}, "", "", "", NewExtensionError(ErrWorkshopGenerationOutputInvalid, "需求包含首版工坊禁止的代码或脚本执行内容", "首版只能生成声明式 Skill", false, nil)
	}
	if issues := ScanWorkshopSecrets([]byte(requirement)); hasIssueCode(issues, ErrWorkshopSecretDetected) {
		return ExtensionDraft{}, WorkshopPlan{}, "", "", "", NewExtensionError(ErrWorkshopSecretDetected, "需求中包含疑似明文 Secret", "请使用 Secret 引用名称，不要粘贴密钥", false, nil)
	}
	available := []map[string]interface{}{}
	availableIDs := map[string]bool{}
	if g.registry != nil {
		if items, err := g.registry.List(ctx, SkillFilter{}); err == nil {
			for _, item := range items {
				availableIDs[item.Definition.ID] = true
				available = append(available, map[string]interface{}{"id": item.Definition.ID, "version": item.Definition.Version, "enabled": item.Definition.Enabled, "inputSchema": json.RawMessage(item.Definition.InputSchema), "outputSchema": json.RawMessage(item.Definition.OutputSchema), "capabilities": item.Definition.Capabilities, "hasSideEffects": item.Definition.HasSideEffects, "idempotent": item.Definition.Idempotent})
			}
		}
	}
	contextRaw, _ := json.Marshal(map[string]interface{}{"requirement": requirement, "allowedSteps": sortedAllowedSteps(), "availableSkills": available, "capabilities": Capabilities(), "hostLimits": DefaultWorkflowLimits(), "protocolVersion": "extensions.amitia.dev/v1alpha1", "engineVersion": "1.0.0"})
	plan, planRaw, provider, model, err := g.generatePlan(ctx, string(contextRaw), availableIDs)
	if err != nil {
		return ExtensionDraft{}, WorkshopPlan{}, planRaw, provider, model, err
	}
	planJSON, _ := json.Marshal(plan)
	draftContext, _ := json.Marshal(map[string]interface{}{"requirement": requirement, "plan": json.RawMessage(planJSON), "allowedSteps": sortedAllowedSteps(), "availableSkills": available, "capabilities": Capabilities(), "hostLimits": DefaultWorkflowLimits(), "protocolVersion": "extensions.amitia.dev/v1alpha1", "engineVersion": "1.0.0"})
	draft, raw, draftProvider, draftModel, err := g.generateDraft(ctx, string(draftContext))
	if draftProvider != "" {
		provider = draftProvider
	}
	if draftModel != "" {
		model = draftModel
	}
	return draft, plan, raw, provider, model, err
}

func (g *WorkshopGenerator) generatePlan(ctx context.Context, prompt string, availableIDs map[string]bool) (WorkshopPlan, string, string, string, error) {
	userPrompt := prompt
	var lastError error
	var raw, provider, model string
	for attempt := 0; attempt < g.maxAttempts; attempt++ {
		if attempt > 0 {
			userPrompt = prompt + fmt.Sprintf("\n上次规划输出无效：%s。只返回修正后的 JSON 对象。", sanitizeGenerationError(lastError))
		}
		var err error
		raw, provider, model, err = g.model.GenerateWorkshopJSON(ctx, workshopPlannerPrompt(), userPrompt)
		if err != nil {
			lastError = err
			continue
		}
		if strings.Contains(raw, "```") {
			lastError = fmt.Errorf("禁止 Markdown 代码块")
			continue
		}
		if issues := ScanWorkshopSecrets([]byte(raw)); hasErrorIssues(issues) {
			lastError = fmt.Errorf("模型输出包含禁止内容或 Secret")
			continue
		}
		var plan WorkshopPlan
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&plan); err != nil {
			lastError = err
			continue
		}
		if err := validateWorkshopPlan(plan, availableIDs); err != nil {
			lastError = err
			continue
		}
		plan = normalizeWorkshopPlan(plan)
		return plan, raw, provider, model, nil
	}
	return WorkshopPlan{}, raw, provider, model, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "模型多次返回无效规划", sanitizeGenerationError(lastError), false, lastError)
}

func (g *WorkshopGenerator) generateDraft(ctx context.Context, prompt string) (ExtensionDraft, string, string, string, error) {
	userPrompt := prompt
	var lastError error
	var raw, provider, model string
	for attempt := 0; attempt < g.maxAttempts; attempt++ {
		if attempt > 0 {
			userPrompt = prompt + fmt.Sprintf("\n上次 Draft 输出无效：%s。只返回修正后的 JSON 对象。", sanitizeGenerationError(lastError))
		}
		var err error
		raw, provider, model, err = g.model.GenerateWorkshopJSON(ctx, workshopSystemPrompt(), userPrompt)
		if err != nil {
			lastError = err
			continue
		}
		if strings.Contains(raw, "```") {
			lastError = fmt.Errorf("禁止 Markdown 代码块")
			continue
		}
		if issues := ScanWorkshopSecrets([]byte(raw)); hasErrorIssues(issues) {
			lastError = fmt.Errorf("模型输出包含禁止内容或 Secret")
			continue
		}
		var draft ExtensionDraft
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&draft); err != nil {
			lastError = err
			continue
		}
		if err := validateDraftStructure(draft); err != nil {
			lastError = err
			continue
		}
		return draft, raw, provider, model, nil
	}
	return ExtensionDraft{}, raw, provider, model, NewExtensionError(ErrWorkshopGenerationOutputInvalid, "模型多次返回无效 Draft", sanitizeGenerationError(lastError), false, lastError)
}

func workshopPlannerPrompt() string {
	return `你是 Amitia Extension Workshop 的需求规划器。只返回一个 JSON 对象，顶层字段只能是 goal、inputs、outputs、configs、steps、dependencies、capabilities、sideEffects、risks、missingDetails、assumptions，所有字段都必须存在。inputs、outputs、configs 的每个元素只能包含 name、type、required、description；steps 的每个元素只能包含 id、type、purpose，禁止使用 description 或其他字段。dependencies、capabilities、sideEffects、risks、missingDetails、assumptions 必须是字符串数组，即使为空也必须返回 []，禁止返回布尔值、null 或对象。严格参照此形状：{"goal":"目标","inputs":[{"name":"name","type":"string","required":true,"description":"说明"}],"outputs":[],"configs":[],"steps":[{"id":"result","type":"transform","purpose":"生成结果"}],"dependencies":[],"capabilities":[],"sideEffects":[],"risks":[],"missingDetails":[],"assumptions":[]}。steps.type 只能使用宿主提供的白名单类型，dependencies 只能引用 availableSkills，capabilities 只能引用能力目录。不得输出 Draft、Manifest、源码、Markdown、Secret、系统提示词或安装授权决定。`
}

func workshopSystemPrompt() string {
	return `你是 Amitia Extension Workshop 的结构化规划器。只返回一个符合 ExtensionDraft 的 JSON 对象，不得输出 Markdown、解释或源码。顶层只能包含 draftVersion、metadata、intent、manifest、inputSchema、outputSchema、configSchema、defaultConfig、workflow、capabilities、dependencies、testCases、assumptions、warnings，禁止在顶层输出 kind、entry、execution 或其他字段。严格参照此最小形状：{"draftVersion":"1.0.0","metadata":{"id":"dev.user.skill","name":"名称","version":"1.0.0","description":"描述","author":"Local User","license":"LicenseRef-Amitia-Local"},"intent":{"goal":"目标","triggers":["manual"],"sideEffects":[]},"manifest":{},"inputSchema":{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}},"outputSchema":{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}},"configSchema":{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}},"defaultConfig":{},"workflow":{"schemaVersion":"1.0.0","steps":[{"id":"result","type":"transform","input":{"op":"pick","value":{"message":"hello"},"fields":["message"]},"onError":{"mode":"fail"}}],"output":{"$ref":"steps.result"},"limits":{}},"capabilities":[],"dependencies":[],"testCases":[{"id":"dry","name":"Dry Run","mode":"dry_run","input":{},"config":{},"secretRefs":[],"httpMocks":[],"skillMocks":[],"assertions":[{"type":"status_is","expected":"succeeded"}]}],"assumptions":[{"message":"假设内容"}],"warnings":[{"code":"WARNING_CODE","message":"警告内容","path":"可选路径"}]}。manifest 必须是空对象，最终 Skill Manifest、kind、entry、execution 和 Capability 由宿主编译分析后生成，模型不得自行添加。assumptions 的元素只能包含 message；warnings 的元素只能包含 code、message、path。数组为空时必须返回 []，对象为空时必须返回 {}，禁止用 null 或布尔值代替。只允许 http、condition、transform、template、call_skill、schedule、notification、memory_candidate、context_contribution。禁止 Go、JavaScript、Shell、SQL、HTML、函数、import、文件路径、数据库 DSN、Secret 明文、系统提示词和渠道凭证。不得决定授权、安装或启用，不得引用不存在的 Skill，不得使用 dev.amitia.* 命名空间，不得扩大宿主限制。Schema 必须是 JSON Schema 2020-12，工作流必须有最终 output，测试断言只能使用 equals、not_equals、exists、not_exists、contains、matches_schema、status_is、step_succeeded、step_failed、side_effect_count、duration_less_than。默认生成 dry_run 测试，网络工作流同时生成 mocked 测试。所有不确定内容写入 assumptions 或 warnings。`
}

func validateWorkshopPlan(plan WorkshopPlan, availableIDs map[string]bool) error {
	if strings.TrimSpace(plan.Goal) == "" || len(plan.Steps) == 0 {
		return fmt.Errorf("Plan 目标或步骤缺失")
	}
	for _, step := range plan.Steps {
		if !allowedWorkflowSteps[step.Type] || !workflowStepIDPattern.MatchString(step.ID) {
			return fmt.Errorf("Plan 包含未知步骤或非法 ID: %s", step.Type)
		}
	}
	for _, dependency := range plan.Dependencies {
		if !availableIDs[dependency] {
			return fmt.Errorf("Plan 引用了不存在的 Skill: %s", dependency)
		}
	}
	for _, capability := range plan.Capabilities {
		if _, ok := capabilityCatalog[capability]; !ok {
			return fmt.Errorf("Plan 引用了未知 Capability: %s", capability)
		}
	}
	raw, _ := json.Marshal(plan)
	if issues := ScanWorkshopSecrets(raw); hasErrorIssues(issues) {
		return fmt.Errorf("Plan 包含禁止内容")
	}
	return nil
}

func normalizeWorkshopPlan(plan WorkshopPlan) WorkshopPlan {
	plan.Goal = strings.TrimSpace(plan.Goal)
	if plan.Inputs == nil {
		plan.Inputs = []PlannedField{}
	}
	if plan.Outputs == nil {
		plan.Outputs = []PlannedField{}
	}
	if plan.Configs == nil {
		plan.Configs = []PlannedField{}
	}
	if plan.Steps == nil {
		plan.Steps = []PlannedStep{}
	}
	for index := range plan.Steps {
		plan.Steps[index].Purpose = strings.TrimSpace(plan.Steps[index].Purpose)
		if plan.Steps[index].Purpose == "" {
			plan.Steps[index].Purpose = plan.Steps[index].Type
		}
	}
	if plan.Dependencies == nil {
		plan.Dependencies = []string{}
	}
	if plan.Capabilities == nil {
		plan.Capabilities = []string{}
	}
	if plan.SideEffects == nil {
		plan.SideEffects = []string{}
	}
	if plan.Risks == nil {
		plan.Risks = []string{}
	}
	if plan.MissingDetails == nil {
		plan.MissingDetails = []string{}
	}
	if plan.Assumptions == nil {
		plan.Assumptions = []string{}
	}
	return plan
}

func planFromDraft(draft ExtensionDraft) WorkshopPlan {
	steps := make([]PlannedStep, 0, len(draft.Workflow.Steps))
	for _, step := range draft.Workflow.Steps {
		steps = append(steps, PlannedStep{ID: step.ID, Type: step.Type, Purpose: step.Type})
	}
	dependencies := make([]string, 0, len(draft.Dependencies))
	for _, dependency := range draft.Dependencies {
		dependencies = append(dependencies, dependency.SkillID)
	}
	assumptions := make([]string, 0, len(draft.Assumptions))
	for _, assumption := range draft.Assumptions {
		assumptions = append(assumptions, assumption.Message)
	}
	return normalizeWorkshopPlan(WorkshopPlan{Goal: draft.Intent.Goal, Steps: steps, Dependencies: dependencies, Capabilities: append([]string{}, draft.Capabilities...), SideEffects: append([]string{}, draft.Intent.SideEffects...), Assumptions: assumptions})
}
func sortedAllowedSteps() []string {
	values := make([]string, 0, len(allowedWorkflowSteps))
	for value := range allowedWorkflowSteps {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
func sanitizeGenerationError(err error) string {
	if err == nil {
		return "输出不符合结构"
	}
	text := err.Error()
	if len(text) > 300 {
		text = text[:300]
	}
	return text
}
func hasIssueCode(issues []AnalysisIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func validateDraftStructure(draft ExtensionDraft) error {
	if draft.DraftVersion == "" || draft.Metadata.Name == "" || draft.Metadata.Description == "" {
		return fmt.Errorf("Draft 基础字段缺失")
	}
	if len(draft.InputSchema) == 0 || len(draft.OutputSchema) == 0 || len(draft.Workflow.Steps) == 0 || len(draft.Workflow.Output) == 0 {
		return fmt.Errorf("Draft Schema 或 Workflow 缺失")
	}
	raw, _ := json.Marshal(draft)
	if issues := ScanWorkshopSecrets(raw); hasErrorIssues(issues) {
		return fmt.Errorf("Draft 包含禁止内容")
	}
	for _, step := range draft.Workflow.Steps {
		if !allowedWorkflowSteps[step.Type] {
			return fmt.Errorf("未知步骤: %s", step.Type)
		}
	}
	return nil
}

var nonIDPattern = regexp.MustCompile(`[^a-z0-9.-]+`)

func normalizeWorkshopDraft(draft ExtensionDraft, userID string) (ExtensionDraft, []DraftWarning) {
	warnings := append([]DraftWarning(nil), draft.Warnings...)
	draft.DraftVersion = "1.0.0"
	draft.Metadata.Name = strings.TrimSpace(draft.Metadata.Name)
	draft.Metadata.Description = strings.TrimSpace(draft.Metadata.Description)
	draft.Metadata.Version = strings.TrimSpace(draft.Metadata.Version)
	if !semverPattern.MatchString(draft.Metadata.Version) {
		draft.Metadata.Version = "1.0.0"
		warnings = append(warnings, DraftWarning{Code: "VERSION_NORMALIZED", Message: "版本已规范化为 1.0.0", Path: "metadata.version"})
	}
	namespace := "dev.user"
	cleanUser := nonIDPattern.ReplaceAllString(strings.ToLower(userID), "-")
	cleanUser = strings.Trim(cleanUser, ".-")
	if cleanUser != "" {
		namespace += "." + cleanUser
	}
	id := strings.ToLower(strings.TrimSpace(draft.Metadata.ID))
	id = nonIDPattern.ReplaceAllString(id, "-")
	id = strings.Trim(id, ".-")
	if id == "" || strings.HasPrefix(id, "dev.amitia.") || !strings.HasPrefix(id, namespace+".") {
		slug := nonIDPattern.ReplaceAllString(strings.ToLower(draft.Metadata.Name), "-")
		slug = strings.Trim(slug, ".-")
		if slug == "" {
			slug = "skill"
		}
		id = namespace + "." + slug
		warnings = append(warnings, DraftWarning{Code: "ID_NORMALIZED", Message: "Skill ID 已调整到当前用户命名空间", Path: "metadata.id"})
	}
	draft.Metadata.ID = id
	if draft.Metadata.Author == "" {
		draft.Metadata.Author = "Local User"
	}
	if draft.Metadata.License == "" {
		draft.Metadata.License = "LicenseRef-Amitia-Local"
	}
	if len(draft.Intent.Triggers) == 0 {
		draft.Intent.Triggers = []SkillTrigger{TriggerManual}
	}
	draft.Intent.Triggers = uniqueSortedTriggers(draft.Intent.Triggers)
	for index := range draft.Workflow.Steps {
		draft.Workflow.Steps[index].ID = strings.ToLower(strings.TrimSpace(draft.Workflow.Steps[index].ID))
		draft.Workflow.Steps[index].Type = strings.ToLower(strings.TrimSpace(draft.Workflow.Steps[index].Type))
		if draft.Workflow.Steps[index].OnError.Mode == "" {
			draft.Workflow.Steps[index].OnError.Mode = "fail"
		}
	}
	draft.Workflow.SchemaVersion = "1.0.0"
	draft.Workflow.Limits = effectiveWorkflowLimits(draft.Workflow.Limits)
	draft.InputSchema = normalizeSchema(draft.InputSchema)
	draft.OutputSchema = normalizeSchema(draft.OutputSchema)
	draft.ConfigSchema = normalizeSchema(draft.ConfigSchema)
	draft.DefaultConfig = stableJSON(draft.DefaultConfig)
	draft.Warnings = warnings
	return draft, warnings
}
func normalizeSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	}
	var object map[string]interface{}
	if json.Unmarshal(raw, &object) != nil {
		return raw
	}
	if _, ok := object["$schema"]; !ok {
		object["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	}
	if object["type"] == "object" {
		if _, ok := object["additionalProperties"]; !ok {
			object["additionalProperties"] = false
		}
	}
	result, _ := json.Marshal(object)
	return result
}
func uniqueSortedTriggers(values []SkillTrigger) []SkillTrigger {
	set := map[SkillTrigger]bool{}
	for _, value := range values {
		switch value {
		case TriggerLLM, TriggerManual, TriggerSchedule, TriggerSystemEvent:
			set[value] = true
		}
	}
	result := make([]SkillTrigger, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
