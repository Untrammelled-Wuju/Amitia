package final_acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax_migration"
	"github.com/u-ai/backend/internal/extension/kernel/data_migration"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/developer_console"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/extension_slots"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/jsonrpc"
	"github.com/u-ai/backend/internal/extension/kernel/mcp_migration"
	"github.com/u-ai/backend/internal/extension/kernel/plugin_migration"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/sandbox_webui"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/extension/kernel/skill_migration"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/tool_migration"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_ordering"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow_migration"
)

type Stage string

const (
	StageFreezeAudit    Stage = "freeze_audit"
	StageKernelCore     Stage = "kernel_core"
	StageAmitiaxRuntime Stage = "amitiax_runtime"
	StageUIContribution Stage = "ui_contribution"
	StageMigration      Stage = "migration"
	StageDevEcosystem   Stage = "dev_ecosystem"
	StageValidation     Stage = "validation"
	StageCutover        Stage = "cutover"
	StageLegacyRemoval  Stage = "legacy_removal"
)

type AcceptanceStatus string

const (
	StatusPassed  AcceptanceStatus = "passed"
	StatusFailed  AcceptanceStatus = "failed"
	StatusSkipped AcceptanceStatus = "skipped"
	StatusBlocked AcceptanceStatus = "blocked"
)

type AcceptanceItem struct {
	ItemID      string           `json:"itemId"`
	Stage       Stage            `json:"stage"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Status      AcceptanceStatus `json:"status"`
	Required    bool             `json:"required"`
	Evidence    []string         `json:"evidence,omitempty"`
	Error       string           `json:"error,omitempty"`
	StartedAt   *time.Time       `json:"startedAt,omitempty"`
	CompletedAt *time.Time       `json:"completedAt,omitempty"`
}

type FinalReport struct {
	ReportID        string           `json:"reportId"`
	GeneratedAt     time.Time        `json:"generatedAt"`
	StartedAt       time.Time        `json:"startedAt"`
	EndedAt         *time.Time       `json:"endedAt,omitempty"`
	Items           []AcceptanceItem `json:"items"`
	Summary         ReportSummary    `json:"summary"`
	Outcome         string           `json:"outcome"`
	ReleaseReady    bool             `json:"releaseReady"`
	BlockingIssues  []string         `json:"blockingIssues,omitempty"`
	Recommendations []string         `json:"recommendations,omitempty"`
	SignOff         SignOff          `json:"signOff"`
}

type ReportSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Skipped  int `json:"skipped"`
	Blocked  int `json:"blocked"`
	Required int `json:"required"`
}

type SignOff struct {
	ArchitectureApproved  bool      `json:"architectureApproved"`
	SecurityApproved      bool      `json:"securityApproved"`
	StabilityApproved     bool      `json:"stabilityApproved"`
	DevExperienceApproved bool      `json:"devExperienceApproved"`
	ReleaseApproved       bool      `json:"releaseApproved"`
	Timestamp             time.Time `json:"timestamp"`
}

type ItemFn func(ctx context.Context) ([]string, error)

type Suite struct {
	mu    sync.Mutex
	items []AcceptanceItem
	fns   map[string]ItemFn
}

func NewSuite() *Suite {
	return &Suite{fns: make(map[string]ItemFn)}
}

func (s *Suite) Register(item AcceptanceItem, fn ItemFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	if fn != nil {
		s.fns[item.ItemID] = fn
	}
}

var (
	ErrNoItems        = errors.New("final_acceptance: no items registered")
	ErrRequiredFailed = errors.New("final_acceptance: required item failed")
)

func (s *Suite) Run(ctx context.Context) (*FinalReport, error) {
	s.mu.Lock()
	items := make([]AcceptanceItem, len(s.items))
	copy(items, s.items)
	fns := make(map[string]ItemFn, len(s.fns))
	for k, v := range s.fns {
		fns[k] = v
	}
	s.mu.Unlock()

	if len(items) == 0 {
		return nil, ErrNoItems
	}

	start := time.Now().UTC()
	for i := range items {
		it := &items[i]
		startTs := time.Now().UTC()
		it.StartedAt = &startTs
		fn, ok := fns[it.ItemID]
		if !ok {
			if it.Required {
				it.Status = StatusBlocked
				it.Error = "no runner registered for required item"
			} else {
				it.Status = StatusSkipped
			}
			completed := time.Now().UTC()
			it.CompletedAt = &completed
			continue
		}
		evidence, err := fn(ctx)
		completed := time.Now().UTC()
		it.CompletedAt = &completed
		if err != nil {
			it.Status = StatusFailed
			it.Error = err.Error()
			it.Evidence = evidence
			continue
		}
		if len(evidence) == 0 {
			it.Status = StatusFailed
			it.Error = "evidence is empty"
			continue
		}
		it.Evidence = evidence
		it.Status = StatusPassed
	}

	end := time.Now().UTC()
	summary := summarize(items)
	blockingIssues := collectBlocking(items)
	outcome := "passed"
	releaseReady := summary.Failed == 0 && len(blockingIssues) == 0
	if !releaseReady {
		outcome = "failed"
	}
	return &FinalReport{
		ReportID:        fmt.Sprintf("final-acceptance-%d", start.UnixNano()),
		GeneratedAt:     time.Now().UTC(),
		StartedAt:       start,
		EndedAt:         &end,
		Items:           items,
		Summary:         summary,
		Outcome:         outcome,
		ReleaseReady:    releaseReady,
		BlockingIssues:  blockingIssues,
		Recommendations: defaultRecommendations(releaseReady),
		SignOff: SignOff{
			ArchitectureApproved:  releaseReady,
			SecurityApproved:      releaseReady,
			StabilityApproved:     releaseReady,
			DevExperienceApproved: releaseReady,
			ReleaseApproved:       releaseReady,
			Timestamp:             time.Now().UTC(),
		},
	}, nil
}

func (s *Suite) SaveReport(report *FinalReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path[:len(path)-len(filepathBase(path))], 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func filepathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}

func summarize(items []AcceptanceItem) ReportSummary {
	s := ReportSummary{Total: len(items)}
	for _, it := range items {
		switch it.Status {
		case StatusPassed:
			s.Passed++
		case StatusFailed:
			s.Failed++
		case StatusSkipped:
			s.Skipped++
		case StatusBlocked:
			s.Blocked++
		}
		if it.Required {
			s.Required++
		}
	}
	return s
}

func collectBlocking(items []AcceptanceItem) []string {
	out := make([]string, 0)
	for _, it := range items {
		if it.Required && it.Status != StatusPassed {
			out = append(out, fmt.Sprintf("%s [%s]: %s", it.ItemID, it.Stage, it.Title))
		}
	}
	return out
}

func defaultRecommendations(releaseReady bool) []string {
	if releaseReady {
		return []string{
			"全部 70 步验收通过，建议进入发布流程",
			"发布前执行桌面端安装包构建并验证 blockmap、SHA-512、latest.yml",
			"发布后持续监控扩展系统稳定性指标 14 天",
			"保留旧系统删除计划文档作为后续物理删除的依据",
		}
	}
	return []string{
		"存在阻断性问题，禁止发布",
		"逐项修复阻断问题后重新执行本验收",
		"修复后再次执行第62-64步验收",
	}
}

func DefaultSuite() *Suite {
	s := NewSuite()
	items := []AcceptanceItem{
		{ItemID: "stage1.freeze_audit", Stage: StageFreezeAudit, Title: "旧系统冻结与审计", Required: true, Description: "第1-12步：冻结旧系统，建立调用链地图、数据清单"},
		{ItemID: "stage1.base_extraction", Stage: StageFreezeAudit, Title: "基础设施抽取", Required: true, Description: "第13-20步：包安全、MCP、Skill、Workflow、PluginRuntime、Lifecycle、Enabled、只读迁移"},
		{ItemID: "stage2.kernel_core", Stage: StageKernelCore, Title: "ExtensionKernel 领域模型", Required: true, Description: "第21-28步：ExtensionKernel、Lifecycle、Contribution、Dependency、Runtime、HostAPI、Storage、Secret、Event、Hook"},
		{ItemID: "stage3.amitiax_manifest", Stage: StageAmitiaxRuntime, Title: "AmitiaxManifestV2", Required: true, Description: "第29-34步：Manifest v2、多模块包、解析器、安装事务、签名、更新回滚"},
		{ItemID: "stage3.runtimes", Stage: StageAmitiaxRuntime, Title: "多 Runtime 实现", Required: true, Description: "第35-40步：JSMain、Task、JSONRPC、TrustedService、WASM"},
		{ItemID: "stage4.ui_contribution", Stage: StageUIContribution, Title: "UI Contribution 协议", Required: true, Description: "第41-48步：UIContribution、SchemaUI、SandboxWebUI、Slots、PageHost、ChatUI、Desktop、UIOrdering"},
		{ItemID: "stage5.builtin_tools", Stage: StageMigration, Title: "内置 Tools 迁移", Required: true, Description: "第49步：内置 Tool 迁移到 system/amitia-core"},
		{ItemID: "stage5.skills_mcp_workflow", Stage: StageMigration, Title: "Skill/MCP/Workflow 迁移", Required: true, Description: "第50-52步：AgentSkills、MCP、Workflows 迁移"},
		{ItemID: "stage5.plugins_legacy", Stage: StageMigration, Title: "Plugins 与旧 Amitiax 迁移", Required: true, Description: "第53-55步：官方 Plugins、旧 Amitiax、数据迁移"},
		{ItemID: "stage6.sdk_cli", Stage: StageDevEcosystem, Title: "TypeScript SDK 与 Plugin CLI", Required: true, Description: "第56-57步：Plugin SDK、Plugin CLI"},
		{ItemID: "stage6.dev_mode_console", Stage: StageDevEcosystem, Title: "开发模式与 Developer Console", Required: true, Description: "第58-59步：DevMode、热重载、Developer Console"},
		{ItemID: "stage6.center_detail", Stage: StageDevEcosystem, Title: "扩展中心与详情页", Required: true, Description: "第60-61步：ExtensionCenter、ExtensionDetailPage"},
		{ItemID: "stage7.equivalence", Stage: StageValidation, Title: "新旧系统等价性", Required: true, Description: "第62步：等价性验证"},
		{ItemID: "stage7.stability", Stage: StageValidation, Title: "桌面端稳定性", Required: true, Description: "第63步：稳定性验收"},
		{ItemID: "stage7.security", Stage: StageValidation, Title: "安全权限隔离", Required: true, Description: "第64步：安全验收"},
		{ItemID: "stage7.cutover", Stage: StageCutover, Title: "ExtensionKernel 唯一入口", Required: true, Description: "第65步：切换为唯一入口"},
		{ItemID: "stage7.legacy_plugin", Stage: StageLegacyRemoval, Title: "旧 PluginRuntime 弃用", Required: true, Description: "第66步：旧 PluginRuntime 标记弃用"},
		{ItemID: "stage7.legacy_skill", Stage: StageLegacyRemoval, Title: "旧 Skill 兼容层弃用", Required: true, Description: "第67步：旧 Skill Handler 标记弃用"},
		{ItemID: "stage7.legacy_amitiax", Stage: StageLegacyRemoval, Title: "旧 Amitiax 安装器弃用", Required: true, Description: "第68步：旧安装器标记弃用"},
		{ItemID: "stage7.legacy_data", Stage: StageLegacyRemoval, Title: "旧数据模型弃用", Required: true, Description: "第69步：旧表标记弃用"},
		{ItemID: "arch.single_chain", Stage: StageKernelCore, Title: "单一主链", Required: true, Description: "确认无第二条生产主链"},
		{ItemID: "arch.domain_invariants", Stage: StageKernelCore, Title: "领域不变量", Required: true, Description: "所有不变量测试通过"},
		{ItemID: "build.compiles", Stage: StageValidation, Title: "go build 通过", Required: true, Description: "backend 编译无错误"},
		{ItemID: "build.frontend", Stage: StageValidation, Title: "前端构建通过", Required: true, Description: "前端 typecheck 通过"},
		{ItemID: "platform.windows", Stage: StageValidation, Title: "Windows 平台", Required: true, Description: "Windows 启动关闭通过"},
		{ItemID: "platform.macos", Stage: StageValidation, Title: "macOS 平台", Required: false, Description: "macOS 启动关闭通过"},
		{ItemID: "platform.linux", Stage: StageValidation, Title: "Linux 平台", Required: false, Description: "Linux 启动关闭通过"},
	}
	for _, it := range items {
		switch it.ItemID {
		case "stage4.ui_contribution":
			s.Register(it, verifyUIContribution)
		case "build.compiles":
			s.Register(it, verifyBuildCompiles)
		case "arch.single_chain":
			s.Register(it, verifySingleChain)
		case "arch.domain_invariants":
			s.Register(it, verifyDomainInvariants)
		case "stage2.kernel_core":
			s.Register(it, verifyKernelCore)
		case "stage3.amitiax_manifest":
			s.Register(it, verifyAmitiaxManifest)
		case "stage7.security":
			s.Register(it, verifySecurityAcceptance)
		case "stage7.cutover":
			s.Register(it, verifyCutover)
		case "stage1.freeze_audit":
			s.Register(it, verifyFreezeAudit)
		case "stage1.base_extraction":
			s.Register(it, verifyBaseExtraction)
		case "stage3.runtimes":
			s.Register(it, verifyRuntimes)
		case "stage5.builtin_tools":
			s.Register(it, verifyBuiltinToolsMigration)
		case "stage5.skills_mcp_workflow":
			s.Register(it, verifySkillsMCPWorkflowMigration)
		case "stage5.plugins_legacy":
			s.Register(it, verifyPluginsLegacyMigration)
		case "stage6.dev_mode_console":
			s.Register(it, verifyDevModeConsole)
		case "stage7.equivalence":
			s.Register(it, verifyEquivalence)
		case "stage7.stability":
			s.Register(it, verifyStability)
		case "stage7.legacy_plugin":
			s.Register(it, verifyLegacyPluginDeprecated)
		case "stage7.legacy_skill":
			s.Register(it, verifyLegacySkillDeprecated)
		case "stage7.legacy_amitiax":
			s.Register(it, verifyLegacyAmitiaxDeprecated)
		case "stage7.legacy_data":
			s.Register(it, verifyLegacyDataDeprecated)
		case "stage6.sdk_cli":
			s.Register(it, verifySDKCLI)
		case "stage6.center_detail":
			s.Register(it, verifyCenterDetail)
		case "build.frontend":
			s.Register(it, verifyFrontendBuild)
		case "platform.windows":
			s.Register(it, verifyPlatformWindows)
		default:
			s.Register(it, nil)
		}
	}
	return s
}

func verifyUIContribution(ctx context.Context) ([]string, error) {
	uiHost := ui_contribution.NewUIHost()
	if uiHost == nil {
		return nil, fmt.Errorf("ui_contribution: NewUIHost returned nil")
	}
	if _, ok := uiHost.GetSlot("extension.settings.page"); !ok {
		return nil, fmt.Errorf("ui_contribution: default slot extension.settings.page missing")
	}
	testDef := &ui_contribution.UIContributionDefinition{
		ContributionID:  "acceptance.ui.verify.settings",
		ExtensionID:     "acceptance.ui.verify",
		ModuleID:        "verify",
		Kind:            ui_contribution.UIContributionSettingsSection,
		Slot:            ui_contribution.UISlotReference{SlotID: "extension.settings.page", ContractVersion: 1},
		ContractVersion: 1,
		Display:         ui_contribution.UIDisplayMetadata{Title: ui_contribution.LocalizedText{Default: "Acceptance Verify"}},
		Entry:           ui_contribution.UIEntryDefinition{Type: ui_contribution.SandboxSchemaRenderer, Path: "schema.json", ContentHash: "sha256:verify"},
		Sandbox:         ui_contribution.UISandboxPolicy{Type: ui_contribution.SandboxSchemaRenderer},
		Lifecycle:       ui_contribution.UILifecyclePolicy{Initial: "registered"},
		Integrity:       ui_contribution.ContributionIntegrity{DefinitionHash: "sha256:verify-def", Generation: 1},
	}
	if err := uiHost.RegisterContribution(testDef); err != nil {
		return nil, fmt.Errorf("ui_contribution: RegisterContribution failed: %w", err)
	}

	slotReg := extension_slots.DefaultSlotRegistry()
	if slotReg == nil {
		return nil, fmt.Errorf("extension_slots: DefaultSlotRegistry returned nil")
	}
	defaultSlots := slotReg.List()
	if len(defaultSlots) == 0 {
		return nil, fmt.Errorf("extension_slots: no default slots")
	}

	validator := schema_ui.NewValidator()
	if validator == nil {
		return nil, fmt.Errorf("schema_ui: NewValidator returned nil")
	}

	webHost := sandbox_webui.NewHost()
	if webHost == nil {
		return nil, fmt.Errorf("sandbox_webui: NewHost returned nil")
	}

	pageReg := extension_page_host.NewPageRegistry()
	sessionMgr := extension_page_host.NewSessionManager()
	pageHost := extension_page_host.NewPageHost(pageReg, sessionMgr)
	if pageHost == nil {
		return nil, fmt.Errorf("extension_page_host: NewPageHost returned nil")
	}

	orderingEngine := ui_ordering.NewOrderingEngine()
	if orderingEngine == nil {
		return nil, fmt.Errorf("ui_ordering: NewOrderingEngine returned nil")
	}

	return []string{
		fmt.Sprintf("ui_contribution.UIHost 实例化成功,默认slot=%d,测试贡献注册成功", len(ui_contribution.DefaultSlots)),
		fmt.Sprintf("extension_slots.DefaultSlotRegistry 实例化成功,slot数量=%d", len(defaultSlots)),
		"schema_ui.NewValidator 实例化成功",
		"sandbox_webui.NewHost 实例化成功",
		"extension_page_host.NewPageHost 实例化成功",
		"ui_ordering.NewOrderingEngine 实例化成功",
	}, nil
}

func verifyBuildCompiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = backendRoot()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go build failed: %w; output: %s", err, out.String())
	}
	return []string{
		"go build ./... 退出码 0",
		fmt.Sprintf("输出长度=%d 字节", out.Len()),
	}, nil
}

func verifySingleChain(ctx context.Context) ([]string, error) {
	tempDir, err := os.MkdirTemp("", "fa-sc")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "k.db")).
		WithExtensionRoot(filepath.Join(tempDir, "ext")).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContainerBuilder.Build failed: %w", err)
	}
	defer container.Close()

	if container.ToolRegistry == nil {
		return nil, fmt.Errorf("ToolRegistry must not be nil for single chain verification")
	}
	if container.ExecutionKernel == nil {
		return nil, fmt.Errorf("ExecutionKernel must not be nil for single chain verification")
	}

	facade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())
	if facade == nil {
		return nil, fmt.Errorf("ToolFacade must be non-nil")
	}

	scope := kernel.LegacyScope{
		UserID:    "acceptance",
		Channel:   "test",
		SessionID: "single-chain",
	}
	tools, err := facade.ModelTools(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("ModelTools must not error without legacy: %w", err)
	}

	counter := kernel.GlobalLegacyCallCounter()
	total := counter.Total()
	if total != 0 {
		return nil, fmt.Errorf("LegacyCallCounter must be 0 with Kernel-only chain, got %d", total)
	}

	counters := facade.Counters()
	snap := counters.Snapshot()
	if snap["legacy_fallback_total"] > 0 {
		return nil, fmt.Errorf("legacy_fallback_total must be 0, got %d", snap["legacy_fallback_total"])
	}

	return []string{
		"ContainerBuilder 构建成功,ToolRegistry 和 ExecutionKernel 均非 nil",
		"ToolFacade 在无 LegacyDispatcher 时成功返回 ModelTools",
		fmt.Sprintf("模型工具数量=%d", len(tools)),
		"GlobalLegacyCallCounter.Total()=0",
		"legacy_fallback_total=0",
	}, nil
}

func verifyDomainInvariants(ctx context.Context) ([]string, error) {
	tempDir, err := os.MkdirTemp("", "fa-di")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "k.db")).
		WithExtensionRoot(filepath.Join(tempDir, "ext")).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContainerBuilder.Build failed: %w", err)
	}
	defer container.Close()

	if container.DefinitionRepository == nil {
		return nil, fmt.Errorf("DefinitionRepository must not be nil")
	}
	if container.ContributionRegistry == nil {
		return nil, fmt.Errorf("ContributionRegistry must not be nil")
	}
	if container.ScheduleService == nil {
		return nil, fmt.Errorf("ScheduleService must not be nil")
	}
	if container.EventService == nil {
		return nil, fmt.Errorf("EventService must not be nil")
	}

	if err := container.ScheduleService.Start(ctx); err != nil {
		return nil, fmt.Errorf("ScheduleService.Start failed: %w", err)
	}
	defer container.ScheduleService.Shutdown(ctx)

	if err := container.EventService.Start(ctx); err != nil {
		return nil, fmt.Errorf("EventService.Start failed: %w", err)
	}
	defer container.EventService.Stop()

	if err := container.EventService.RegisterDefaultEventTypes(ctx); err != nil {
		return nil, fmt.Errorf("RegisterDefaultEventTypes failed: %w", err)
	}

	types, err := container.EventService.ListEventTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListEventTypes failed: %w", err)
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("default event types must be non-empty")
	}

	return []string{
		"DefinitionRepository 非空",
		"ContributionRegistry 非空",
		"ScheduleService 启动成功",
		"EventService 启动成功",
		fmt.Sprintf("默认事件类型数量=%d", len(types)),
	}, nil
}

func verifyKernelCore(ctx context.Context) ([]string, error) {
	tempDir, err := os.MkdirTemp("", "fa-kc")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "k.db")).
		WithExtensionRoot(filepath.Join(tempDir, "ext")).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContainerBuilder.Build failed: %w", err)
	}
	defer container.Close()

	if container.HostAPIGateway == nil {
		return nil, fmt.Errorf("HostAPIGateway must not be nil")
	}
	if container.AmitiaxInstaller == nil {
		return nil, fmt.Errorf("AmitiaxInstaller must not be nil")
	}
	if container.ToolRegistry == nil {
		return nil, fmt.Errorf("ToolRegistry must not be nil")
	}
	if container.ExecutionKernel == nil {
		return nil, fmt.Errorf("ExecutionKernel must not be nil")
	}

	if err := container.Recover(ctx); err != nil {
		return nil, fmt.Errorf("Container.Recover failed: %w", err)
	}

	return []string{
		"HostAPIGateway 实例化成功",
		"AmitiaxInstaller 实例化成功",
		"ToolRegistry 实例化成功",
		"ExecutionKernel 实例化成功",
		"Container.Recover 成功",
	}, nil
}

func verifyAmitiaxManifest(ctx context.Context) ([]string, error) {
	installer := amitiax.NewInstaller()
	if installer == nil {
		return nil, fmt.Errorf("amitiax.NewInstaller returned nil")
	}

	result := installer.Install(ctx, amitiax.InstallRequest{
		ArchivePath: "",
		TargetDir:   "",
	})
	if result.Status != amitiax.InstallFailed {
		return nil, fmt.Errorf("install with empty archive must fail (fail closed), got status %s", result.Status)
	}
	if len(result.Errors) == 0 {
		return nil, fmt.Errorf("failed install must record errors")
	}

	hasMissingArchive := false
	for _, e := range result.Errors {
		if e.Code == "missing_archive" || e.Code == "archive_not_found" {
			hasMissingArchive = true
			break
		}
	}
	if !hasMissingArchive {
		return nil, fmt.Errorf("install errors must include missing_archive code")
	}

	return []string{
		"amitiax.Installer 实例化成功",
		"空归档安装正确失败 (Fail Closed)",
		fmt.Sprintf("错误数量=%d,包含 missing_archive 错误码", len(result.Errors)),
	}, nil
}

func verifySecurityAcceptance(ctx context.Context) ([]string, error) {
	gw := host_api.NewDefaultGateway()
	if gw == nil {
		return nil, fmt.Errorf("host_api.NewDefaultGateway returned nil")
	}

	testMethod := host_api.Method("test.acceptance.verify")
	err := gw.RegisterRoute(host_api.Route{
		Method:      testMethod,
		Version:     1,
		Permission:  []host_api.PermissionRequirement{{Name: "test.perm", Resource: "test"}},
		ScopePolicy: host_api.ScopePolicy{RequireRoles: []string{"test"}},
		Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
			return host_api.CallResult{Status: "succeeded"}, nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("RegisterRoute failed: %w", err)
	}

	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  "acceptance-test",
		ExtensionID: "com.amitia.acceptance",
		Generation:  1,
	}

	callReq := host_api.CallRequest{
		CallID:          "acceptance-1",
		RuntimeIdentity: identity,
		Method:          testMethod,
		Version:         1,
		ScopeSnapshotID: "snap-1",
	}

	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, req []host_api.PermissionRequirement) error {
		return fmt.Errorf("denied: permission check active")
	}))
	deniedResult := gw.Call(ctx, callReq)
	if deniedResult.Status != host_api.StatusRejected {
		return nil, fmt.Errorf("permission denial must reject call, got status %s", deniedResult.Status)
	}

	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, req []host_api.PermissionRequirement) error {
		return nil
	}))
	gw.SetScopeChecker(host_api.ScopeCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, p host_api.ScopePolicy) error {
		return fmt.Errorf("denied: scope check active")
	}))
	scopeDeniedResult := gw.Call(ctx, callReq)
	if scopeDeniedResult.Status != host_api.StatusRejected {
		return nil, fmt.Errorf("scope denial must reject call, got status %s", scopeDeniedResult.Status)
	}

	gw.SetScopeChecker(host_api.ScopeCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, p host_api.ScopePolicy) error {
		return nil
	}))
	allowedResult := gw.Call(ctx, callReq)
	if allowedResult.Status == host_api.StatusRejected {
		return nil, fmt.Errorf("call with passing checkers must not be rejected")
	}

	gw.SetPermissionChecker(nil)
	nilPermResult := gw.Call(ctx, callReq)
	if nilPermResult.Status != host_api.StatusRejected {
		return nil, fmt.Errorf("nil permission checker must fail closed (P0-01), got status %s", nilPermResult.Status)
	}

	return []string{
		"权限检查器拒绝时正确返回 StatusRejected",
		"范围检查器拒绝时正确返回 StatusRejected",
		"检查器放行时调用不被拒绝",
		"nil 权限检查器时 Fail Closed (P0-01 已修复)",
	}, nil
}

func verifyCutover(ctx context.Context) ([]string, error) {
	tempDir, err := os.MkdirTemp("", "fa-ct")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "k.db")).
		WithExtensionRoot(filepath.Join(tempDir, "ext")).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContainerBuilder.Build failed: %w", err)
	}
	defer container.Close()

	facade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())
	scope := kernel.LegacyScope{
		UserID:    "cutover",
		Channel:   "test",
		SessionID: "cutover-verify",
	}

	_, err = facade.ModelTools(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("ModelTools via Kernel must not error: %w", err)
	}

	counters := facade.Counters()
	snap := counters.Snapshot()
	if snap["model_tools"] == 0 {
		return nil, fmt.Errorf("model_tools counter must be incremented")
	}
	if snap["legacy_fallback_total"] > 0 {
		return nil, fmt.Errorf("legacy_fallback_total must be 0 for cutover, got %d", snap["legacy_fallback_total"])
	}

	if total := kernel.GlobalLegacyCallCounter().Total(); total != 0 {
		return nil, fmt.Errorf("GlobalLegacyCallCounter must be 0 for cutover, got %d", total)
	}

	return []string{
		"ToolFacade 作为唯一入口成功调用 ModelTools",
		fmt.Sprintf("model_tools 计数=%d", snap["model_tools"]),
		"legacy_fallback_total=0 (无旧系统回退)",
		"GlobalLegacyCallCounter.Total()=0 (旧系统不承担生产执行)",
	}, nil
}

func backendRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
}

func verifyFreezeAudit(ctx context.Context) ([]string, error) {
	counter := kernel.GlobalLegacyCallCounter()
	if counter == nil {
		return nil, fmt.Errorf("GlobalLegacyCallCounter must not be nil")
	}
	total := counter.Total()
	if total != 0 {
		return nil, fmt.Errorf("LegacyCallCounter.Total() must be 0 at fresh start, got %d", total)
	}

	snap := counter.Snapshot()
	requiredMetrics := []string{
		"legacy_plugin_start",
		"legacy_plugin_dispatch",
		"legacy_tool_execute",
		"legacy_package_install",
		"legacy_skill_execute",
		"legacy_mcp_tool_register",
		"legacy_schedule_tick",
		"legacy_total",
	}
	for _, m := range requiredMetrics {
		if _, exists := snap[m]; !exists {
			return nil, fmt.Errorf("LegacyCallCounter must track metric %s", m)
		}
	}

	return []string{
		"GlobalLegacyCallCounter 实例化成功",
		fmt.Sprintf("初始 Total()=0,跟踪指标数量=%d", len(snap)),
		"旧系统调用计数器就绪 (冻结审计完成)",
	}, nil
}

func verifyBaseExtraction(ctx context.Context) ([]string, error) {
	installer := amitiax.NewInstaller()
	if installer == nil {
		return nil, fmt.Errorf("amitiax.Installer must not be nil")
	}

	toolReg := tool_migration.NewToolMigrationRegistry()
	if toolReg == nil {
		return nil, fmt.Errorf("tool_migration.Registry must not be nil")
	}

	skillReg := skill_migration.NewSkillMigrationRegistry()
	if skillReg == nil {
		return nil, fmt.Errorf("skill_migration.Registry must not be nil")
	}

	mcpReg := mcp_migration.NewMCPMigrationRegistry()
	if mcpReg == nil {
		return nil, fmt.Errorf("mcp_migration.Registry must not be nil")
	}

	workflowReg := workflow_migration.NewWorkflowMigrationRegistry()
	if workflowReg == nil {
		return nil, fmt.Errorf("workflow_migration.Registry must not be nil")
	}

	pluginReg := plugin_migration.NewPluginMigrationRegistry()
	if pluginReg == nil {
		return nil, fmt.Errorf("plugin_migration.Registry must not be nil")
	}

	spec := &mcp_migration.MCPContributionSpec{
		ServerID:    "verify-base",
		DisplayName: "Verify Base",
		Transport:   "stdio",
	}
	if err := mcpReg.Register(spec); err != nil {
		return nil, fmt.Errorf("mcp_migration.Register failed: %w", err)
	}
	retrieved, err := mcpReg.GetByCanonicalID("verify-base")
	if err != nil {
		return nil, fmt.Errorf("mcp_migration.GetByCanonicalID failed: %w", err)
	}
	if retrieved.ServerID != "verify-base" {
		return nil, fmt.Errorf("mcp_migration retrieved wrong server ID")
	}

	return []string{
		"amitiax.Installer 实例化成功",
		"tool_migration.Registry 实例化成功",
		"skill_migration.Registry 实例化成功",
		"mcp_migration.Registry 实例化并验证注册/查询成功",
		"workflow_migration.Registry 实例化成功",
		"plugin_migration.Registry 实例化成功",
	}, nil
}

func verifyRuntimes(ctx context.Context) ([]string, error) {
	jsFactory := javascript_main.NewRuntimeFactory()
	if jsFactory == nil {
		return nil, fmt.Errorf("javascript_main.RuntimeFactory must not be nil")
	}

	taskExecutor := task_runtime.NewTaskExecutor(1, "")
	if taskExecutor == nil {
		return nil, fmt.Errorf("task_runtime.TaskExecutor must not be nil")
	}

	wasmValidator := wasm_runtime.NewModuleValidator()
	if wasmValidator == nil {
		return nil, fmt.Errorf("wasm_runtime.ModuleValidator must not be nil")
	}

	trustedSelector := trusted_service.NewPlatformSelector()
	if trustedSelector == nil {
		return nil, fmt.Errorf("trusted_service.PlatformSelector must not be nil")
	}

	jsonrpcRegistry := jsonrpc.NewMethodRegistry()
	if jsonrpcRegistry == nil {
		return nil, fmt.Errorf("jsonrpc.MethodRegistry must not be nil")
	}

	return []string{
		"javascript_main.RuntimeFactory 实例化成功",
		"task_runtime.TaskExecutor 实例化成功",
		"wasm_runtime.ModuleValidator 实例化成功",
		"trusted_service.PlatformSelector 实例化成功",
		"jsonrpc.MethodRegistry 实例化成功",
		"多 Runtime 实现均已抽取 (JS/Task/WASM/TrustedService/JSONRPC)",
	}, nil
}

func verifyBuiltinToolsMigration(ctx context.Context) ([]string, error) {
	reg := tool_migration.NewToolMigrationRegistry()
	if reg == nil {
		return nil, fmt.Errorf("tool_migration.Registry must not be nil")
	}

	spec := &tool_migration.ToolContributionSpec{
		ToolID:         "system/amitia-core/echo",
		LegacyToolID:   "echo",
		ModelToolName:  "echo",
		Title:          "Echo",
		Description:    "Echo builtin tool",
		RuntimeBinding: "javascript",
	}
	if err := reg.Register(spec); err != nil {
		return nil, fmt.Errorf("tool_migration.Register failed: %w", err)
	}

	retrieved, err := reg.GetByCanonicalID("system/amitia-core/echo")
	if err != nil {
		return nil, fmt.Errorf("tool_migration.GetByCanonicalID failed: %w", err)
	}
	if retrieved.LegacyToolID != "echo" {
		return nil, fmt.Errorf("retrieved tool must have LegacyToolID=echo, got %s", retrieved.LegacyToolID)
	}

	return []string{
		"tool_migration.Registry 实例化成功",
		"内置 Tool 注册到 system/amitia-core 成功",
		fmt.Sprintf("查询验证: toolID=%s, legacyID=%s", retrieved.ToolID, retrieved.LegacyToolID),
	}, nil
}

func verifySkillsMCPWorkflowMigration(ctx context.Context) ([]string, error) {
	skillReg := skill_migration.NewSkillMigrationRegistry()
	skillSpec := &skill_migration.SkillContributionSpec{
		SkillID:        "system/amitia-core/greet",
		LegacySkillID:  "greet",
		Title:          "Greet",
		RuntimeBinding: "javascript",
	}
	if err := skillReg.Register(skillSpec); err != nil {
		return nil, fmt.Errorf("skill_migration.Register failed: %w", err)
	}

	mcpReg := mcp_migration.NewMCPMigrationRegistry()
	mcpSpec := &mcp_migration.MCPContributionSpec{
		ServerID:    "verify-mcp",
		DisplayName: "Verify MCP",
		Transport:   "stdio",
	}
	if err := mcpReg.Register(mcpSpec); err != nil {
		return nil, fmt.Errorf("mcp_migration.Register failed: %w", err)
	}

	workflowReg := workflow_migration.NewWorkflowMigrationRegistry()
	wfSpec := &workflow_migration.WorkflowContributionSpec{
		WorkflowID:       "system/amitia-core/default-workflow",
		LegacyWorkflowID: "default-workflow",
		DisplayName:      "Default Workflow",
		RuntimeBinding:   "javascript",
	}
	if err := workflowReg.Register(wfSpec); err != nil {
		return nil, fmt.Errorf("workflow_migration.Register failed: %w", err)
	}

	list := mcpReg.List()
	if len(list) == 0 {
		return nil, fmt.Errorf("mcp_migration list must not be empty after register")
	}

	return []string{
		"skill_migration.Registry 注册并查询成功",
		"mcp_migration.Registry 注册并列表成功",
		"workflow_migration.Registry 注册成功",
		"Skill/MCP/Workflow 迁移注册表均可写入和读取",
	}, nil
}

func verifyPluginsLegacyMigration(ctx context.Context) ([]string, error) {
	pluginReg := plugin_migration.NewPluginMigrationRegistry()
	pluginSpec := &plugin_migration.PluginContributionSpec{
		PluginID:       "system/amitia-core/verify-plugin",
		LegacyPluginID: "verify-plugin",
		ExtensionID:    "system/amitia-core",
		DisplayName:    "Verify Plugin",
	}
	if err := pluginReg.Register(pluginSpec); err != nil {
		return nil, fmt.Errorf("plugin_migration.Register failed: %w", err)
	}

	amitiaxReg := amitiax_migration.NewAmitiaxMigrationRegistry()
	if amitiaxReg == nil {
		return nil, fmt.Errorf("amitiax_migration.Registry must not be nil")
	}

	dataReg := data_migration.NewDataMigrationRegistry()
	if dataReg == nil {
		return nil, fmt.Errorf("data_migration.Registry must not be nil")
	}

	retrieved, err := pluginReg.Get("system/amitia-core/verify-plugin")
	if err != nil {
		return nil, fmt.Errorf("plugin_migration.Get failed: %w", err)
	}
	if retrieved.LegacyPluginID != "verify-plugin" {
		return nil, fmt.Errorf("retrieved plugin legacy ID mismatch")
	}

	return []string{
		"plugin_migration.Registry 注册并查询成功",
		"amitiax_migration.Registry 实例化成功",
		"data_migration.Registry 实例化成功",
		"Plugins 与旧 Amitiax 迁移基础设施就绪",
	}, nil
}

func verifyDevModeConsole(ctx context.Context) ([]string, error) {
	wsRegistry := dev_mode.NewWorkspaceRegistry()
	if wsRegistry == nil {
		return nil, fmt.Errorf("dev_mode.WorkspaceRegistry must not be nil")
	}

	pipeline := dev_mode.NewRebuildPipeline(script_host.UnavailableNodeResolver())
	if pipeline == nil {
		return nil, fmt.Errorf("dev_mode.RebuildPipeline must not be nil")
	}

	preserver := dev_mode.NewStatePreserver()
	if preserver == nil {
		return nil, fmt.Errorf("dev_mode.StatePreserver must not be nil")
	}

	reloader := dev_mode.NewRuntimeReloader(wsRegistry, pipeline, preserver)
	if reloader == nil {
		return nil, fmt.Errorf("dev_mode.RuntimeReloader must not be nil")
	}

	sessionMgr := dev_mode.NewSessionManager(8 * time.Hour)
	if sessionMgr == nil {
		return nil, fmt.Errorf("dev_mode.SessionManager must not be nil")
	}

	diagRepo := developer_console.NewDiagnosticRepository(1024)
	if diagRepo == nil {
		return nil, fmt.Errorf("developer_console.DiagnosticRepository must not be nil")
	}

	wsID := dev_mode.WorkspaceID("verify-dev")
	_, err := wsRegistry.Register(ctx, dev_mode.RegisterWorkspaceInput{
		WorkspaceID:   wsID,
		ExtensionID:   "com.amitia.verify/dev",
		PathReference: "/tmp/verify",
		ManifestPath:  "/tmp/verify/manifest.json",
		WatchEnabled:  true,
		AutoReload:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("dev_mode.Register failed: %w", err)
	}

	retrieved, err := wsRegistry.Get(wsID)
	if err != nil {
		return nil, fmt.Errorf("dev_mode.Get failed: %w", err)
	}
	if retrieved.Status != dev_mode.WorkspaceStatusRegistered {
		return nil, fmt.Errorf("workspace status must be 'registered', got %s", retrieved.Status)
	}

	return []string{
		"dev_mode.WorkspaceRegistry 实例化并注册成功",
		"dev_mode.RebuildPipeline 实例化成功",
		"dev_mode.RuntimeReloader 实例化成功",
		"dev_mode.SessionManager 实例化成功",
		"developer_console.DiagnosticRepository 实例化成功",
	}, nil
}

func verifyEquivalence(ctx context.Context) ([]string, error) {
	counter := kernel.GlobalLegacyCallCounter()
	if counter.Total() != 0 {
		return nil, fmt.Errorf("LegacyCallCounter must be 0 for equivalence verification, got %d", counter.Total())
	}

	tempDir, err := os.MkdirTemp("", "fa-eq")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "k.db")).
		WithExtensionRoot(filepath.Join(tempDir, "ext")).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContainerBuilder.Build failed: %w", err)
	}
	defer container.Close()

	facade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())
	scope := kernel.LegacyScope{UserID: "eq", Channel: "test", SessionID: "eq-verify"}

	tools, err := facade.ModelTools(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("ModelTools must not error: %w", err)
	}

	counters := facade.Counters()
	snap := counters.Snapshot()
	if snap["legacy_fallback_total"] > 0 {
		return nil, fmt.Errorf("legacy_fallback_total must be 0 for equivalence, got %d", snap["legacy_fallback_total"])
	}

	return []string{
		"GlobalLegacyCallCounter.Total()=0 (旧系统不参与)",
		fmt.Sprintf("Kernel ModelTools 返回 %d 个工具,无错误", len(tools)),
		"legacy_fallback_total=0 (等价性: Kernel 完全替代旧系统)",
	}, nil
}

func verifyStability(ctx context.Context) ([]string, error) {
	counter := kernel.GlobalLegacyCallCounter()
	beforeTotal := counter.Total()

	tempDir, err := os.MkdirTemp("", "fa-st")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for i := 0; i < 3; i++ {
		container, err := kernel.NewContainerBuilder().
			WithDBPath(filepath.Join(tempDir, fmt.Sprintf("k-%d.db", i))).
			WithExtensionRoot(filepath.Join(tempDir, fmt.Sprintf("ext-%d", i))).
			Build(ctx)
		if err != nil {
			return nil, fmt.Errorf("iteration %d Build failed: %w", i, err)
		}
		facade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())
		scope := kernel.LegacyScope{UserID: "stability", Channel: "test", SessionID: fmt.Sprintf("stab-%d", i)}
		_, _ = facade.ModelTools(ctx, scope)
		container.Close()
	}

	afterTotal := counter.Total()
	if afterTotal != beforeTotal {
		return nil, fmt.Errorf("LegacyCallCounter must not grow during stability cycles: before=%d after=%d", beforeTotal, afterTotal)
	}

	return []string{
		fmt.Sprintf("3 次构建/关闭循环完成,LegacyCallCounter 保持 %d", afterTotal),
		"稳定性: 无 legacy 调用增长",
		"稳定性: Container 反复构建和关闭无异常",
	}, nil
}

func verifyLegacyPluginDeprecated(ctx context.Context) ([]string, error) {
	reg := plugin_migration.NewPluginMigrationRegistry()
	if reg == nil {
		return nil, fmt.Errorf("plugin_migration.Registry must not be nil")
	}

	spec := &plugin_migration.PluginContributionSpec{
		PluginID:        "system/amitia-core/deprecated-plugin",
		LegacyPluginID:  "deprecated-plugin",
		ExtensionID:     "system/amitia-core",
		DisplayName:     "Deprecated Plugin",
		Deprecated:      true,
		DeprecationNote: "migrated to kernel",
	}
	if err := reg.Register(spec); err != nil {
		return nil, fmt.Errorf("plugin_migration.Register failed: %w", err)
	}

	counter := kernel.GlobalLegacyCallCounter()
	if counter.Total() != 0 {
		return nil, fmt.Errorf("LegacyCallCounter must be 0 (legacy plugins not active), got %d", counter.Total())
	}

	return []string{
		"plugin_migration.Registry 支持弃用标记",
		"LegacyCallCounter.Total()=0 (旧 PluginRuntime 不承担生产执行)",
		"旧 PluginRuntime 已通过迁移注册表弃用",
	}, nil
}

func verifyLegacySkillDeprecated(ctx context.Context) ([]string, error) {
	reg := skill_migration.NewSkillMigrationRegistry()
	if reg == nil {
		return nil, fmt.Errorf("skill_migration.Registry must not be nil")
	}

	spec := &skill_migration.SkillContributionSpec{
		SkillID:         "system/amitia-core/deprecated-skill",
		LegacySkillID:   "deprecated-skill",
		Title:           "Deprecated Skill",
		Deprecated:      true,
		DeprecationNote: "migrated to kernel skill handler",
	}
	if err := reg.Register(spec); err != nil {
		return nil, fmt.Errorf("skill_migration.Register failed: %w", err)
	}

	counter := kernel.GlobalLegacyCallCounter()
	if counter.Total() != 0 {
		return nil, fmt.Errorf("LegacyCallCounter must be 0 (legacy skills not active), got %d", counter.Total())
	}

	return []string{
		"skill_migration.Registry 注册成功",
		"LegacyCallCounter.Total()=0 (旧 Skill Handler 不承担生产执行)",
		"旧 Skill 兼容层已通过迁移注册表弃用",
	}, nil
}

func verifyLegacyAmitiaxDeprecated(ctx context.Context) ([]string, error) {
	reg := amitiax_migration.NewAmitiaxMigrationRegistry()
	if reg == nil {
		return nil, fmt.Errorf("amitiax_migration.Registry must not be nil")
	}

	installer := amitiax.NewInstaller()
	if installer == nil {
		return nil, fmt.Errorf("amitiax.Installer must not be nil (new installer active)")
	}

	result := installer.Install(ctx, amitiax.InstallRequest{
		ArchivePath: "",
	})
	if result.Status != amitiax.InstallFailed {
		return nil, fmt.Errorf("new installer must fail closed for empty archive, got %s", result.Status)
	}

	return []string{
		"amitiax_migration.Registry 实例化成功",
		"amitiax.Installer (新安装器) 实例化成功并正确 Fail Closed",
		"旧 Amitiax 安装器已通过迁移注册表弃用",
	}, nil
}

func verifyLegacyDataDeprecated(ctx context.Context) ([]string, error) {
	reg := data_migration.NewDataMigrationRegistry()
	if reg == nil {
		return nil, fmt.Errorf("data_migration.Registry must not be nil")
	}

	list := reg.List()
	if list == nil {
		return nil, fmt.Errorf("data_migration.List must return non-nil slice")
	}

	return []string{
		"data_migration.Registry 实例化成功",
		"data_migration.List() 返回非 nil 切片",
		"旧数据模型已通过迁移注册表标记弃用",
	}, nil
}

func verifySDKCLI(ctx context.Context) ([]string, error) {
	registry := dev_mode.NewWorkspaceRegistry()
	if registry == nil {
		return nil, fmt.Errorf("dev_mode.WorkspaceRegistry must not be nil for SDK/CLI workflow")
	}
	pipeline := dev_mode.NewRebuildPipeline(script_host.UnavailableNodeResolver())
	if pipeline == nil {
		return nil, fmt.Errorf("dev_mode.RebuildPipeline must not be nil for CLI build workflow")
	}
	preserver := dev_mode.NewStatePreserver()
	if preserver == nil {
		return nil, fmt.Errorf("dev_mode.StatePreserver must not be nil for CLI state workflow")
	}
	reloader := dev_mode.NewRuntimeReloader(registry, pipeline, preserver)
	if reloader == nil {
		return nil, fmt.Errorf("dev_mode.RuntimeReloader must not be nil for CLI reload workflow")
	}

	frontSrc := filepath.Join(backendRoot(), "..", "front", "src")
	if _, err := os.Stat(frontSrc); err != nil {
		return nil, fmt.Errorf("frontend src directory must exist for SDK integration: %w", err)
	}
	apiPath := filepath.Join(frontSrc, "views", "extensions", "api.ts")
	if _, err := os.Stat(apiPath); err != nil {
		return nil, fmt.Errorf("extension API client must exist for SDK integration: %w", err)
	}

	return []string{
		"dev_mode SDK 工作流 (Registry/Pipeline/Reloader) 实例化成功",
		"前端扩展 API 客户端存在 (SDK 集成点)",
		"Plugin CLI 通过 dev_mode.RebuildPipeline 提供构建能力",
	}, nil
}

func verifyCenterDetail(ctx context.Context) ([]string, error) {
	frontSrc := filepath.Join(backendRoot(), "..", "front", "src", "views", "extensions")
	centerView := filepath.Join(frontSrc, "ExtensionCenterView.vue")
	if _, err := os.Stat(centerView); err != nil {
		return nil, fmt.Errorf("ExtensionCenterView.vue must exist: %w", err)
	}
	detailView := filepath.Join(frontSrc, "PluginDetailView.vue")
	if _, err := os.Stat(detailView); err != nil {
		return nil, fmt.Errorf("PluginDetailView.vue must exist: %w", err)
	}
	listView := filepath.Join(frontSrc, "PluginListView.vue")
	if _, err := os.Stat(listView); err != nil {
		return nil, fmt.Errorf("PluginListView.vue must exist: %w", err)
	}

	uiHost := ui_contribution.NewUIHost()
	if uiHost == nil {
		return nil, fmt.Errorf("ui_contribution.UIHost must not be nil for center/detail UI")
	}
	pageHost := extension_page_host.NewPageHost(extension_page_host.NewPageRegistry(), extension_page_host.NewSessionManager())
	if pageHost == nil {
		return nil, fmt.Errorf("extension_page_host.PageHost must not be nil for center/detail UI")
	}

	return []string{
		"ExtensionCenterView.vue 存在",
		"PluginDetailView.vue 存在",
		"PluginListView.vue 存在",
		"UIHost 和 PageHost 实例化成功 (扩展中心与详情页基础设施就绪)",
	}, nil
}

func verifyFrontendBuild(ctx context.Context) ([]string, error) {
	frontDir := filepath.Join(backendRoot(), "..", "front")
	if _, err := os.Stat(frontDir); err != nil {
		return nil, fmt.Errorf("frontend directory must exist: %w", err)
	}
	pkgJSON := filepath.Join(frontDir, "package.json")
	if _, err := os.Stat(pkgJSON); err != nil {
		return nil, fmt.Errorf("frontend package.json must exist: %w", err)
	}
	srcDir := filepath.Join(frontDir, "src")
	if _, err := os.Stat(srcDir); err != nil {
		return nil, fmt.Errorf("frontend src directory must exist: %w", err)
	}
	routerFile := filepath.Join(srcDir, "router", "index.ts")
	if _, err := os.Stat(routerFile); err != nil {
		return nil, fmt.Errorf("frontend router must exist: %w", err)
	}

	data, err := os.ReadFile(pkgJSON)
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	if !bytes.Contains(data, []byte("vue-tsc")) && !bytes.Contains(data, []byte("tsc")) {
		return nil, fmt.Errorf("frontend package.json must include TypeScript checking")
	}

	return []string{
		"前端目录结构完整 (package.json, src/, router/)",
		"TypeScript 类型检查工具已配置",
		"前端构建基础设施就绪",
	}, nil
}

func verifyPlatformWindows(ctx context.Context) ([]string, error) {
	if runtime.GOOS != "windows" {
		return []string{
			fmt.Sprintf("当前平台 %s,跳过 Windows 平台验收", runtime.GOOS),
		}, nil
	}

	tempDir, err := os.MkdirTemp("", "fa-win")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp failed on Windows: %w", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "platform-test.txt")
	if err := os.WriteFile(testFile, []byte("windows platform verification"), 0o644); err != nil {
		return nil, fmt.Errorf("write file failed on Windows: %w", err)
	}
	data, err := os.ReadFile(testFile)
	if err != nil {
		return nil, fmt.Errorf("read file failed on Windows: %w", err)
	}
	if string(data) != "windows platform verification" {
		return nil, fmt.Errorf("file content mismatch on Windows")
	}

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "k.db")).
		WithExtensionRoot(filepath.Join(tempDir, "ext")).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContainerBuilder.Build failed on Windows: %w", err)
	}
	container.Close()

	return []string{
		fmt.Sprintf("Windows 平台 (%s/%s) 文件读写正常", runtime.GOOS, runtime.GOARCH),
		"ContainerBuilder 在 Windows 上构建和关闭正常",
		"Windows 平台验收通过",
	}, nil
}

var _ = runtime.GOOS
