package security_acceptance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

type SecurityStatus string

const (
	SecStatusPassed  SecurityStatus = "passed"
	SecStatusFailed  SecurityStatus = "failed"
	SecStatusSkipped SecurityStatus = "skipped"
	SecStatusBlocked SecurityStatus = "blocked"
)

type Check struct {
	CheckID     string         `json:"checkId"`
	Category    string         `json:"category"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Severity    Severity       `json:"severity"`
	Status      SecurityStatus `json:"status"`
	StartedAt   *time.Time     `json:"startedAt,omitempty"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
	Error       string         `json:"error,omitempty"`
	Evidence    []string       `json:"evidence,omitempty"`
	Remediation string         `json:"remediation,omitempty"`
}

type SecurityReport struct {
	ReportID    string     `json:"reportId"`
	GeneratedAt time.Time  `json:"generatedAt"`
	Checks      []Check    `json:"checks"`
	Summary     SecSummary `json:"summary"`
	Outcome     string     `json:"outcome"`
	Notes       []string   `json:"notes,omitempty"`
}

type SecSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Skipped  int `json:"skipped"`
	Blocked  int `json:"blocked"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

type CheckFn func(ctx context.Context) ([]string, error)

type Suite struct {
	mu     sync.Mutex
	checks []Check
	fns    map[string]CheckFn
}

func NewSuite() *Suite {
	return &Suite{fns: make(map[string]CheckFn)}
}

func (s *Suite) Register(check Check, fn CheckFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks = append(s.checks, check)
	if fn != nil {
		s.fns[check.CheckID] = fn
	}
}

var (
	ErrNoChecks = errors.New("security_acceptance: no checks registered")
)

func (s *Suite) Run(ctx context.Context) (*SecurityReport, error) {
	s.mu.Lock()
	checks := make([]Check, len(s.checks))
	copy(checks, s.checks)
	fns := make(map[string]CheckFn, len(s.fns))
	for k, v := range s.fns {
		fns[k] = v
	}
	s.mu.Unlock()
	if len(checks) == 0 {
		return nil, ErrNoChecks
	}

	for i := range checks {
		c := &checks[i]
		start := time.Now().UTC()
		c.StartedAt = &start
		fn, ok := fns[c.CheckID]
		if !ok {
			c.Status = SecStatusBlocked
			c.Error = "no runner registered for security check"
			c.CompletedAt = ptrTime(time.Now().UTC())
			continue
		}
		evidence, err := fn(ctx)
		c.CompletedAt = ptrTime(time.Now().UTC())
		if err != nil {
			c.Status = SecStatusFailed
			c.Error = err.Error()
			c.Evidence = evidence
			continue
		}
		if len(evidence) == 0 {
			c.Status = SecStatusFailed
			c.Error = "evidence is empty"
			continue
		}
		c.Evidence = evidence
		c.Status = SecStatusPassed
	}

	summary := summarizeSec(checks)
	outcome := "passed"
	if summary.Failed > 0 || summary.Blocked > 0 {
		outcome = "failed"
	}
	return &SecurityReport{
		ReportID:    fmt.Sprintf("security-%d", time.Now().UnixNano()),
		GeneratedAt: time.Now().UTC(),
		Checks:      checks,
		Summary:     summary,
		Outcome:     outcome,
	}, nil
}

func (s *Suite) SaveReport(report *SecurityReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func ptrTime(t time.Time) *time.Time { return &t }

func summarizeSec(checks []Check) SecSummary {
	s := SecSummary{Total: len(checks)}
	for _, c := range checks {
		switch c.Status {
		case SecStatusPassed:
			s.Passed++
		case SecStatusFailed:
			s.Failed++
		case SecStatusSkipped:
			s.Skipped++
		case SecStatusBlocked:
			s.Blocked++
		}
		switch c.Severity {
		case SeverityCritical:
			s.Critical++
		case SeverityHigh:
			s.High++
		case SeverityMedium:
			s.Medium++
		case SeverityLow:
			s.Low++
		}
	}
	return s
}

func DefaultSuite() *Suite {
	s := NewSuite()
	checks := []Check{
		{CheckID: "permission.broker_enforced", Category: "permission", Title: "Permission Broker 强制执行", Description: "所有 Tool 执行路径经过 Permission Broker", Severity: SeverityCritical},
		{CheckID: "permission.minimal_default", Category: "permission", Title: "默认最小权限", Description: "扩展默认无权限", Severity: SeverityHigh},
		{CheckID: "permission.explicit_grant", Category: "permission", Title: "权限显式授予", Description: "权限必须显式授予", Severity: SeverityHigh},
		{CheckID: "permission.high_risk_review", Category: "permission", Title: "高风险权限审批", Description: "高风险权限需要用户审批", Severity: SeverityCritical},
		{CheckID: "scope.character_isolation", Category: "scope", Title: "角色 Scope 隔离", Description: "角色作用域数据隔离", Severity: SeverityCritical},
		{CheckID: "scope.conversation_isolation", Category: "scope", Title: "会话 Scope 隔离", Description: "会话作用域数据隔离", Severity: SeverityHigh},
		{CheckID: "scope.no_implicit_global", Category: "scope", Title: "无隐式全局读取", Description: "Tool 不读取全局当前角色", Severity: SeverityHigh},
		{CheckID: "storage.namespace_isolation", Category: "storage", Title: "存储命名空间隔离", Description: "扩展只能访问自己命名空间", Severity: SeverityCritical},
		{CheckID: "storage.secret_excluded", Category: "storage", Title: "Storage 不存储 Secret", Description: "Storage key 拒绝 secret 关键字", Severity: SeverityCritical},
		{CheckID: "storage.quota_enforced", Category: "storage", Title: "Storage 配额", Description: "Storage 配额强制执行", Severity: SeverityMedium},
		{CheckID: "secret.reference_only", Category: "secret", Title: "Secret 只暴露引用", Description: "扩展代码不接触 Secret 明文", Severity: SeverityCritical},
		{CheckID: "secret.lease_limited", Category: "secret", Title: "Secret Lease 限时", Description: "Lease 默认 60s", Severity: SeverityHigh},
		{CheckID: "sandbox.webui_csp", Category: "sandbox", Title: "WebUI CSP", Description: "Restricted WebUI 强 CSP", Severity: SeverityCritical},
		{CheckID: "sandbox.webui_origin_isolation", Category: "sandbox", Title: "WebUI Origin 隔离", Description: "每个 WebUI 会话独立 origin", Severity: SeverityCritical},
		{CheckID: "sandbox.webui_no_node", Category: "sandbox", Title: "WebUI 无 Node 访问", Description: "WebUI 不能访问 Node API", Severity: SeverityCritical},
		{CheckID: "package.signature_required", Category: "package", Title: "包签名校验", Description: "非 dev 包必须签名", Severity: SeverityCritical},
		{CheckID: "package.integrity_check", Category: "package", Title: "完整性校验", Description: "包内容 Hash 校验", Severity: SeverityCritical},
		{CheckID: "package.path_traversal", Category: "package", Title: "路径穿越防护", Description: "包内路径不允许 .. 越界", Severity: SeverityCritical},
		{CheckID: "package.no_secret_in_bundle", Category: "package", Title: "包内不含 Secret", Description: "打包流程排除 .env、密钥", Severity: SeverityCritical},
		{CheckID: "runtime.isolated_process", Category: "runtime", Title: "Runtime 进程隔离", Description: "Runtime 在独立进程运行", Severity: SeverityHigh},
		{CheckID: "runtime.resource_limits", Category: "runtime", Title: "Runtime 资源限制", Description: "Runtime 有 CPU/内存限制", Severity: SeverityHigh},
		{CheckID: "runtime.no_global_state", Category: "runtime", Title: "Runtime 无全局状态", Description: "Runtime 不共享宿主全局状态", Severity: SeverityMedium},
		{CheckID: "hostapi.typed_only", Category: "hostapi", Title: "HostAPI 类型化", Description: "HostAPI 不暴露 host.call(any)", Severity: SeverityCritical},
		{CheckID: "hostapi.no_session_token_leak", Category: "hostapi", Title: "Session Token 不泄露", Description: "SDK 不导出 Session Token", Severity: SeverityCritical},
		{CheckID: "hostapi.no_real_path", Category: "hostapi", Title: "不暴露真实路径", Description: "SDK 不暴露宿主真实文件路径", Severity: SeverityHigh},
		{CheckID: "migration.no_data_loss", Category: "migration", Title: "迁移不丢数据", Description: "数据迁移完整性验证", Severity: SeverityCritical},
		{CheckID: "migration.rollback_safe", Category: "migration", Title: "回滚安全", Description: "迁移可回滚", Severity: SeverityHigh},
		{CheckID: "audit.all_writes", Category: "audit", Title: "审计覆盖写操作", Description: "所有写操作有审计记录", Severity: SeverityHigh},
		{CheckID: "audit.no_sensitive_in_log", Category: "audit", Title: "日志不含敏感信息", Description: "日志过滤器移除敏感字段", Severity: SeverityCritical},
	}
	for _, c := range checks {
		s.Register(c, nil)
	}
	return s
}
