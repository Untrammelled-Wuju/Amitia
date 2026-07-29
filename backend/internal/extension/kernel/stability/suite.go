package stability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
)

type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "macos"
	PlatformLinux   Platform = "linux"
)

type AcceptanceStatus string

const (
	StatusPassed  AcceptanceStatus = "passed"
	StatusFailed  AcceptanceStatus = "failed"
	StatusSkipped AcceptanceStatus = "skipped"
	StatusBlocked AcceptanceStatus = "blocked"
)

type Scenario struct {
	ScenarioID  string           `json:"scenarioId"`
	Category    string           `json:"category"`
	Platform    Platform         `json:"platform"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Duration    time.Duration    `json:"duration"`
	Status      AcceptanceStatus `json:"status"`
	StartedAt   *time.Time       `json:"startedAt,omitempty"`
	CompletedAt *time.Time       `json:"completedAt,omitempty"`
	Error       string           `json:"error,omitempty"`
	Metrics     ScenarioMetrics  `json:"metrics"`
}

type ScenarioMetrics struct {
	InitialMemoryMB float64 `json:"initialMemoryMb"`
	PeakMemoryMB    float64 `json:"peakMemoryMb"`
	FinalMemoryMB   float64 `json:"finalMemoryMb"`
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
	ProcessCount    int     `json:"processCount"`
	OpenHandles     int     `json:"openHandles"`
	GoroutineCount  int     `json:"goroutineCount"`
}

type AcceptanceReport struct {
	ReportID    string     `json:"reportId"`
	GeneratedAt time.Time  `json:"generatedAt"`
	Platform    Platform   `json:"platform"`
	Scenarios   []Scenario `json:"scenarios"`
	Summary     Summary    `json:"summary"`
	Outcome     string     `json:"outcome"`
}

type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Blocked int `json:"blocked"`
}

type ScenarioFn func(ctx context.Context) (ScenarioMetrics, error)

type Suite struct {
	mu        sync.Mutex
	scenarios []Scenario
	fns       map[string]ScenarioFn
}

func NewSuite() *Suite {
	return &Suite{fns: make(map[string]ScenarioFn)}
}

func (s *Suite) Register(scenario Scenario, fn ScenarioFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scenarios = append(s.scenarios, scenario)
	if fn != nil {
		s.fns[scenario.ScenarioID] = fn
	}
}

var (
	ErrNoScenarios = errors.New("stability: no scenarios registered")
)

func (s *Suite) Run(ctx context.Context, platform Platform) (*AcceptanceReport, error) {
	s.mu.Lock()
	scenarios := make([]Scenario, len(s.scenarios))
	copy(scenarios, s.scenarios)
	fns := make(map[string]ScenarioFn, len(s.fns))
	for k, v := range s.fns {
		fns[k] = v
	}
	s.mu.Unlock()

	if len(scenarios) == 0 {
		return nil, ErrNoScenarios
	}

	for i := range scenarios {
		sc := &scenarios[i]
		if sc.Platform != "" && sc.Platform != platform {
			sc.Status = StatusSkipped
			continue
		}
		fn, ok := fns[sc.ScenarioID]
		if !ok {
			sc.Status = StatusBlocked
			sc.Error = "no runner registered for scenario"
			continue
		}
		start := time.Now().UTC()
		sc.StartedAt = &start
		metrics, err := fn(ctx)
		sc.CompletedAt = ptrTime(time.Now().UTC())
		sc.Duration = time.Since(start)
		sc.Metrics = metrics
		if err != nil {
			sc.Status = StatusFailed
			sc.Error = err.Error()
			continue
		}
		sc.Status = StatusPassed
	}

	summary := summarize(scenarios)
	outcome := "passed"
	if summary.Failed > 0 || summary.Blocked > 0 {
		outcome = "failed"
	}
	return &AcceptanceReport{
		ReportID:    fmt.Sprintf("stability-%s-%d", platform, time.Now().UnixNano()),
		GeneratedAt: time.Now().UTC(),
		Platform:    platform,
		Scenarios:   scenarios,
		Summary:     summary,
		Outcome:     outcome,
	}, nil
}

func (s *Suite) SaveReport(report *AcceptanceReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func ptrTime(t time.Time) *time.Time { return &t }

func summarize(scenarios []Scenario) Summary {
	s := Summary{Total: len(scenarios)}
	for _, sc := range scenarios {
		switch sc.Status {
		case StatusPassed:
			s.Passed++
		case StatusFailed:
			s.Failed++
		case StatusSkipped:
			s.Skipped++
		case StatusBlocked:
			s.Blocked++
		}
	}
	return s
}

func CurrentPlatform() Platform {
	switch runtime.GOOS {
	case "windows":
		return PlatformWindows
	case "darwin":
		return PlatformMacOS
	case "linux":
		return PlatformLinux
	}
	return Platform("")
}

func CaptureMetrics() ScenarioMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return ScenarioMetrics{
		InitialMemoryMB: float64(m.Alloc) / 1024 / 1024,
		PeakMemoryMB:    float64(m.TotalAlloc) / 1024 / 1024,
		FinalMemoryMB:   float64(m.Alloc) / 1024 / 1024,
		GoroutineCount:  runtime.NumGoroutine(),
	}
}

func DefaultSuite() *Suite {
	s := NewSuite()
	stabilityScenarios := []Scenario{
		{ScenarioID: "startup.cold", Category: "startup", Title: "冷启动", Description: "扩展系统冷启动无残留"},
		{ScenarioID: "startup.warm", Category: "startup", Title: "热启动", Description: "热启动恢复状态"},
		{ScenarioID: "shutdown.clean", Category: "shutdown", Title: "干净关闭", Description: "关闭流程在5s内完成"},
		{ScenarioID: "shutdown.forced", Category: "shutdown", Title: "强制关闭", Description: "强制关闭后无残留"},
		{ScenarioID: "resume.from_sleep", Category: "resume", Title: "休眠恢复", Description: "休眠唤醒后扩展恢复"},
		{ScenarioID: "memory.long_run", Category: "memory", Title: "长时运行内存", Description: "运行24小时内存增长<10%"},
		{ScenarioID: "memory.leak_detection", Category: "memory", Title: "内存泄漏检测", Description: "重复加载卸载无泄漏"},
		{ScenarioID: "process.orphan", Category: "process", Title: "孤儿进程", Description: "扩展崩溃后无孤儿进程"},
		{ScenarioID: "process.spawn_loop", Category: "process", Title: "进程拉起循环", Description: "Runtime 崩溃后不反复拉起"},
		{ScenarioID: "tray.icon_persistence", Category: "tray", Title: "托盘图标持久性", Description: "托盘图标在重启后正确"},
		{ScenarioID: "window.multi_monitor", Category: "window", Title: "多显示器", Description: "多显示器下窗口正确"},
		{ScenarioID: "upgrade.runtime_replace", Category: "upgrade", Title: "运行时替换", Description: "运行时升级不丢失状态"},
		{ScenarioID: "crash.runtime_recovery", Category: "crash", Title: "Runtime崩溃恢复", Description: "Runtime崩溃后自动恢复"},
		{ScenarioID: "crash.host_recovery", Category: "crash", Title: "Host崩溃恢复", Description: "Host崩溃后扩展恢复"},
		{ScenarioID: "high_dpi.render", Category: "render", Title: "高DPI渲染", Description: "高DPI下UI正确"},
		{ScenarioID: "long_path.windows", Category: "filesystem", Platform: PlatformWindows, Title: "长路径", Description: "Windows长路径支持"},
		{ScenarioID: "chinese_path", Category: "filesystem", Title: "中文路径", Description: "中文路径支持"},
	}
	for _, sc := range stabilityScenarios {
		s.Register(sc, nil)
	}
	return s
}
