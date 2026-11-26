package capability

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

type TaskDomain string

const (
	TaskDomainDocument      TaskDomain = "document"
	TaskDomainData          TaskDomain = "data"
	TaskDomainCommunication TaskDomain = "communication"
	TaskDomainAutomation    TaskDomain = "automation"
	TaskDomainMedia         TaskDomain = "media"
	TaskDomainSystem        TaskDomain = "system"
	TaskDomainCode          TaskDomain = "code"
	TaskDomainSearch        TaskDomain = "search"
	TaskDomainUnknown       TaskDomain = "unknown"
)

type TaskAction string

const (
	TaskActionRead      TaskAction = "read"
	TaskActionWrite     TaskAction = "write"
	TaskActionTransform TaskAction = "transform"
	TaskActionExecute   TaskAction = "execute"
	TaskActionSend      TaskAction = "send"
	TaskActionDelete    TaskAction = "delete"
	TaskActionUnknown   TaskAction = "unknown"
)

var ErrTaskSelectorInvalid = errors.New("task selector is invalid")
var ErrTaskContractInvalid = errors.New("task contract declaration is invalid")

type taskDomainEntry struct {
	domain TaskDomain
	terms  []string
}

var taskDomainLexicon = []taskDomainEntry{
	{TaskDomainDocument, []string{"文档", "word", "docx", "pdf", "excel", "xlsx", "幻灯片", "ppt", "pptx", "报告", "排版"}},
	{TaskDomainData, []string{"数据", "sql", "数据库", "表结构", "统计", "图表", "csv", "指标"}},
	{TaskDomainCommunication, []string{"消息", "邮件", "发送", "回复", "群聊", "通知", "email", "message"}},
	{TaskDomainAutomation, []string{"定时", "自动化", "任务", "计划", "轮询", "schedule", "cron", "automation"}},
	{TaskDomainMedia, []string{"图片", "音频", "视频", "转写", "截图", "image", "audio", "video", "transcribe"}},
	{TaskDomainSystem, []string{"系统", "桌面", "窗口", "进程", "权限", "设备", "desktop", "process", "window"}},
	{TaskDomainCode, []string{"代码", "编译", "构建", "仓库", "测试", "code", "build", "compile", "git"}},
	{TaskDomainSearch, []string{"搜索", "检索", "查找", "网页", "search", "query", "lookup"}},
}

type taskActionEntry struct {
	action TaskAction
	terms  []string
}

var taskActionLexicon = []taskActionEntry{
	{TaskActionDelete, []string{"删除", "移除", "清理", "delete", "remove", "drop"}},
	{TaskActionSend, []string{"发送", "发布", "推送", "send", "publish", "post"}},
	{TaskActionTransform, []string{"转换", "整理", "汇总", "翻译", "convert", "transform", "summarize"}},
	{TaskActionWrite, []string{"写入", "创建", "新增", "修改", "write", "create", "update", "insert"}},
	{TaskActionExecute, []string{"执行", "运行", "启动", "run", "execute", "start"}},
	{TaskActionRead, []string{"读取", "查看", "列出", "打开", "搜索", "检索", "查找", "read", "get", "list", "open", "search"}},
}

type TaskProfile struct {
	Domain  TaskDomain
	Action  TaskAction
	Matched []string
}

func ClassifyTask(text string) TaskProfile {
	lowered := strings.ToLower(text)
	profile := TaskProfile{Domain: TaskDomainUnknown, Action: TaskActionUnknown, Matched: []string{}}
	best := 0
	for _, entry := range taskDomainLexicon {
		score := 0
		for _, term := range entry.terms {
			if strings.Contains(lowered, term) {
				score += len([]rune(term))
			}
		}
		if score > best {
			best = score
			profile.Domain = entry.domain
		}
	}
	best = 0
	for _, entry := range taskActionLexicon {
		score := 0
		for _, term := range entry.terms {
			if strings.Contains(lowered, term) {
				score += len([]rune(term))
			}
		}
		if score > best {
			best = score
			profile.Action = entry.action
		}
	}
	for _, entry := range taskDomainLexicon {
		for _, term := range entry.terms {
			if strings.Contains(lowered, term) {
				profile.Matched = append(profile.Matched, term)
			}
		}
	}
	return profile
}

type ContractResolver interface {
	Resolve(ctx context.Context, name string) (major string, found bool, err error)
}

type GrantResolver interface {
	Granted(ctx context.Context, tool ToolDefinition, invocation ToolInvocationContext) (bool, error)
}

type TaskSelector struct {
	registry     *ToolRegistry
	availability AvailabilityEvaluator
	grants       GrantResolver
	contracts    ContractResolver
	platform     string
	ranking      *RankingPolicy
	exact        *ExactRouteIndex
}

func NewTaskSelector(registry *ToolRegistry, availability AvailabilityEvaluator) *TaskSelector {
	return &TaskSelector{registry: registry, availability: availability}
}

func (s *TaskSelector) WithGrantResolver(resolver GrantResolver) *TaskSelector {
	s.grants = resolver
	return s
}

func (s *TaskSelector) WithContractResolver(resolver ContractResolver) *TaskSelector {
	s.contracts = resolver
	return s
}

func (s *TaskSelector) WithPlatform(platform string) *TaskSelector {
	s.platform = platform
	return s
}

func (s *TaskSelector) WithRanking(policy *RankingPolicy) *TaskSelector {
	s.ranking = policy
	return s
}

type SelectedTool struct {
	Tool    ToolDefinition
	Score   int
	Rank    float64
	Matched []string
	Domains []TaskDomain
	Signals map[string]float64
}

type RejectedTool struct {
	ToolID    string
	ModelName string
	Reasons   []AvailabilityReason
	Cause     error
}

type SelectionReport struct {
	Profile  TaskProfile
	Strategy StrategySnapshot
	Selected []SelectedTool
	Rejected []RejectedTool
}

func (r SelectionReport) SelectedIDs() []string {
	ids := make([]string, 0, len(r.Selected))
	for _, selected := range r.Selected {
		ids = append(ids, selected.Tool.ID)
	}
	return ids
}

func (r SelectionReport) RejectedIDs() []string {
	ids := make([]string, 0, len(r.Rejected))
	for _, rejected := range r.Rejected {
		ids = append(ids, rejected.ToolID)
	}
	return ids
}

func (r SelectionReport) Reasons(toolID string) []AvailabilityReason {
	for _, rejected := range r.Rejected {
		if rejected.ToolID == toolID {
			return append([]AvailabilityReason(nil), rejected.Reasons...)
		}
	}
	return nil
}

func (r SelectionReport) Claimed(toolID string) bool {
	for _, selected := range r.Selected {
		if selected.Tool.ID == toolID {
			return true
		}
	}
	return false
}

func (s *TaskSelector) Select(ctx context.Context, invocation ToolInvocationContext, query string, limit int) (SelectionReport, error) {
	report := SelectionReport{Profile: ClassifyTask(query), Selected: []SelectedTool{}, Rejected: []RejectedTool{}}
	if s == nil || s.registry == nil {
		return report, fmt.Errorf("%w: registry is required", ErrTaskSelectorInvalid)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if ids, ok, err := s.exactLookup(ctx, query); err != nil {
		return SelectionReport{Profile: report.Profile}, err
	} else if ok {
		return s.selectExact(ctx, report.Profile, invocation, ids, limit)
	}
	lowered := strings.ToLower(query)
	candidates := make([]RankCandidate, 0, 8)
	for _, tool := range s.registry.List(ctx, ToolFilter{}) {
		reasons, cause, err := s.hardFilter(ctx, tool, invocation)
		if err != nil {
			return SelectionReport{Profile: report.Profile}, err
		}
		if len(reasons) > 0 {
			report.Rejected = append(report.Rejected, RejectedTool{
				ToolID:    tool.ID,
				ModelName: tool.ModelName,
				Reasons:   reasons,
				Cause:     cause,
			})
			continue
		}
		score, matched, domains := scoreTool(tool, lowered)
		candidates = append(candidates, RankCandidate{
			Tool:    tool,
			Lexical: score,
			Matched: matched,
			Domains: domains,
		})
	}
	if s.ranking != nil {
		ranked, snapshot, err := s.ranking.Rank(ctx, report.Profile, query, candidates)
		if err != nil {
			return SelectionReport{Profile: report.Profile, Rejected: report.Rejected}, err
		}
		report.Strategy = snapshot
		for _, item := range ranked {
			report.Selected = append(report.Selected, SelectedTool{
				Tool:    item.Tool,
				Score:   int(math.Round(item.Score)),
				Rank:    item.Score,
				Matched: item.Matched,
				Domains: item.Domains,
				Signals: item.Signals,
			})
		}
	} else {
		for _, candidate := range candidates {
			report.Selected = append(report.Selected, SelectedTool{
				Tool:    candidate.Tool,
				Score:   candidate.Lexical,
				Rank:    float64(candidate.Lexical),
				Matched: candidate.Matched,
				Domains: candidate.Domains,
				Signals: map[string]float64{"lexical": float64(candidate.Lexical)},
			})
		}
		sort.SliceStable(report.Selected, func(i, j int) bool {
			if report.Selected[i].Score != report.Selected[j].Score {
				return report.Selected[i].Score > report.Selected[j].Score
			}
			return report.Selected[i].Tool.ID < report.Selected[j].Tool.ID
		})
	}
	if limit > 0 && len(report.Selected) > limit {
		report.Selected = report.Selected[:limit]
	}
	return report, nil
}

func (s *TaskSelector) hardFilter(ctx context.Context, tool ToolDefinition, invocation ToolInvocationContext) ([]AvailabilityReason, error, error) {
	reasons := make([]AvailabilityReason, 0, 4)
	var cause error

	if !tool.Enabled {
		reasons = appendUniqueReason(reasons, ReasonToolDisabled)
	}

	state := tool.ComputedState()
	if !state.ScopeAllowed {
		reasons = appendUniqueReason(reasons, ReasonScopeDenied)
	}
	if !scopeAllows(tool.Scope, invocation) {
		reasons = appendUniqueReason(reasons, ReasonScopeDenied)
	}
	if !state.PermissionGranted {
		reasons = appendUniqueReason(reasons, ReasonPermissionMissing)
	}
	if !state.RuntimeReady {
		reasons = appendUniqueReason(reasons, ReasonRuntimeNotReady)
	}
	if !state.DependencyReady {
		reasons = appendUniqueReason(reasons, ReasonDependencyMissing)
	}
	if state.Health != HealthReady {
		reasons = appendUniqueReason(reasons, ReasonHealthUnavailable)
	}

	if s.availability != nil {
		avail := s.availability.Evaluate(ctx, tool, invocation)
		if !avail.Visible {
			reasons = appendUniqueReason(reasons, ReasonNotInstalled)
		}
		for _, reason := range avail.Reasons {
			reasons = appendUniqueReason(reasons, reason)
		}
		if !avail.Executable && len(avail.Reasons) == 0 {
			reasons = appendUniqueReason(reasons, ReasonRuntimeNotReady)
		}
	}

	if s.grants != nil {
		granted, err := s.grants.Granted(ctx, tool, invocation)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve grant for tool %s: %w", tool.ID, err)
		}
		if !granted {
			reasons = appendUniqueReason(reasons, ReasonPermissionMissing)
		}
	}

	if !s.platformSupported(tool) {
		reasons = appendUniqueReason(reasons, ReasonPlatformUnsupported)
	}

	contractReasons, contractCause, err := s.contractProblems(ctx, tool)
	if err != nil {
		return nil, nil, err
	}
	for _, reason := range contractReasons {
		reasons = appendUniqueReason(reasons, reason)
	}
	if contractCause != nil {
		cause = contractCause
	}

	return reasons, cause, nil
}

func (s *TaskSelector) platformSupported(tool ToolDefinition) bool {
	if strings.TrimSpace(s.platform) == "" {
		return true
	}
	declared := toolMetadataStrings(tool.Metadata, "platforms", "platform")
	if len(declared) == 0 {
		return true
	}
	platform := strings.ToLower(strings.TrimSpace(s.platform))
	hostOS := platform
	if index := strings.Index(platform, "/"); index >= 0 {
		hostOS = platform[:index]
	}
	for _, item := range declared {
		candidate := strings.ToLower(strings.TrimSpace(item))
		switch candidate {
		case "", "*", "any", "all":
			return true
		case platform, hostOS:
			return true
		}
	}
	return false
}

func (s *TaskSelector) contractProblems(ctx context.Context, tool ToolDefinition) ([]AvailabilityReason, error, error) {
	declared := toolMetadataStrings(tool.Metadata, "contract", "contracts")
	if len(declared) == 0 {
		return nil, nil, nil
	}
	name, major, ok := strings.Cut(declared[0], "@")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(major) == "" {
		return []AvailabilityReason{ReasonContractIncompatible}, fmt.Errorf("%w: tool=%s contract=%s", ErrTaskContractInvalid, tool.ID, declared[0]), nil
	}
	if s.contracts == nil {
		return []AvailabilityReason{ReasonContractIncompatible}, fmt.Errorf("%w: tool=%s contract=%s has no resolver", ErrTaskContractInvalid, tool.ID, declared[0]), nil
	}
	available, found, err := s.contracts.Resolve(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve contract %s for tool %s: %w", name, tool.ID, err)
	}
	if !found || strings.TrimSpace(available) != strings.TrimSpace(major) {
		return []AvailabilityReason{ReasonContractIncompatible}, nil, nil
	}
	return nil, nil, nil
}

func scopeAllows(rule ScopeRule, invocation ToolInvocationContext) bool {
	scopeType := strings.ToLower(strings.TrimSpace(rule.Type))
	scopeID := strings.TrimSpace(rule.ID)
	switch scopeType {
	case "", "global":
		return true
	case "character":
		return scopeID == "" || (scopeID == invocation.CharacterID && invocation.CharacterID != "")
	case "conversation":
		return scopeID == "" || (scopeID == invocation.ConversationID && invocation.ConversationID != "")
	case "extension":
		return scopeID == "" || (scopeID == invocation.ExtensionID && invocation.ExtensionID != "")
	case "module":
		return scopeID == "" || (scopeID == invocation.ModuleID && invocation.ModuleID != "")
	case "session":
		return scopeID == "" || (scopeID == invocation.SessionID && invocation.SessionID != "")
	default:
		return false
	}
}

func appendUniqueReason(reasons []AvailabilityReason, reason AvailabilityReason) []AvailabilityReason {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func toolMetadataStrings(metadata map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return append([]string(nil), typed...)
		case []any:
			collected := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					collected = append(collected, text)
				}
			}
			return collected
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
			return strings.FieldsFunc(typed, func(r rune) bool {
				return r == ',' || r == ';' || unicode.IsSpace(r)
			})
		}
	}
	return nil
}

func toolDomains(tool ToolDefinition) []TaskDomain {
	declared := toolMetadataStrings(tool.Metadata, "domains", "domain")
	declared = append(declared, tool.ModelExposure.Categories...)
	domains := make([]TaskDomain, 0, len(declared))
	seen := map[TaskDomain]bool{}
	for _, item := range declared {
		domain := TaskDomain(strings.ToLower(strings.TrimSpace(item)))
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		domains = append(domains, domain)
	}
	return domains
}

func scoreTool(tool ToolDefinition, loweredQuery string) (int, []string, []TaskDomain) {
	score := 0
	matched := make([]string, 0, 4)
	terms := append(toolMetadataStrings(tool.Metadata, "keywords"), toolMetadataStrings(tool.Metadata, "tags")...)
	domains := toolDomains(tool)
	for _, domain := range domains {
		terms = append(terms, string(domain))
	}
	for _, term := range terms {
		candidate := strings.ToLower(strings.TrimSpace(term))
		if candidate == "" {
			continue
		}
		if strings.Contains(loweredQuery, candidate) {
			score += 4 + len([]rune(candidate))
			matched = append(matched, candidate)
		}
	}
	name := strings.ToLower(tool.Name)
	if name != "" && strings.Contains(loweredQuery, name) {
		score += 10 + len([]rune(name))
		matched = append(matched, name)
	}
	nameTokens := strings.ToLower(tool.Name + " " + tool.ModelName)
	for _, word := range strings.FieldsFunc(loweredQuery, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(word)) < 2 {
			continue
		}
		if strings.Contains(nameTokens, word) {
			score += 2 * len([]rune(word))
			matched = append(matched, word)
		}
	}
	return score, matched, domains
}

func estimateToolTokens(tool ToolDefinition) int {
	estimated := len(tool.Description)/4 + len(tool.ModelName)/4
	if tool.InputSchema != nil {
		estimated += len(tool.InputSchema) / 4
	}
	return estimated
}
