package legacy_deprecation

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func ValidatePackageLegacyBoundary(root string) error {
	fileSet := token.NewFileSet()
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			if strings.Contains(imported.Path.Value, "/package_legacy_migration") && filepath.Base(filepath.Dir(path)) != "package_legacy_migration" {
				return fmt.Errorf("legacy_deprecation: production package imports migration adapter: %s", path)
			}
		}
		parsed, err = parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		var callPath string
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.SelectorExpr)
			if ok && identifier.Sel.Name == "installLegacyPackage" {
				callPath = path
				return false
			}
			if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "installLegacyPackage" {
				callPath = path
				return false
			}
			return true
		})
		if callPath != "" {
			return fmt.Errorf("legacy_deprecation: production legacy installer call: %s", callPath)
		}
		return nil
	})
}

type DeprecationStatus string

const (
	StatusMarked     DeprecationStatus = "marked"
	StatusFrozen     DeprecationStatus = "frozen"
	StatusRedirected DeprecationStatus = "redirected"
	StatusScheduled  DeprecationStatus = "scheduled_for_deletion"
	StatusDeleted    DeprecationStatus = "deleted"
)

type LegacyFile struct {
	FilePath         string            `json:"filePath"`
	Package          string            `json:"package"`
	Step             int               `json:"step"`
	Reason           string            `json:"reason"`
	Replacement      string            `json:"replacement"`
	Status           DeprecationStatus `json:"status"`
	MarkedAt         time.Time         `json:"markedAt"`
	ProductionRefs   []string          `json:"productionRefs"`
	TestRefs         []string          `json:"testRefs"`
	BlockingDeletion bool              `json:"blockingDeletion"`
	Remediation      string            `json:"remediation"`
}

type DeprecationRegistry struct {
	mu    sync.RWMutex
	files map[string]*LegacyFile
}

func NewDeprecationRegistry() *DeprecationRegistry {
	return &DeprecationRegistry{files: make(map[string]*LegacyFile)}
}

func (r *DeprecationRegistry) Mark(file LegacyFile) error {
	if file.FilePath == "" {
		return fmt.Errorf("legacy_deprecation: file path required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.files[file.FilePath]; ok {
		existing.Status = file.Status
		existing.Reason = file.Reason
		existing.Remediation = file.Remediation
		existing.ProductionRefs = file.ProductionRefs
		existing.TestRefs = file.TestRefs
		return nil
	}
	if file.MarkedAt.IsZero() {
		file.MarkedAt = time.Now().UTC()
	}
	if file.Status == "" {
		file.Status = StatusMarked
	}
	r.files[file.FilePath] = &file
	return nil
}

func (r *DeprecationRegistry) Get(filePath string) (*LegacyFile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.files[filePath]
	return f, ok
}

func (r *DeprecationRegistry) List() []LegacyFile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]LegacyFile, 0, len(r.files))
	for _, f := range r.files {
		out = append(out, *f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Step != out[j].Step {
			return out[i].Step < out[j].Step
		}
		return out[i].FilePath < out[j].FilePath
	})
	return out
}

func (r *DeprecationRegistry) ListByStep(step int) []LegacyFile {
	all := r.List()
	out := make([]LegacyFile, 0)
	for _, f := range all {
		if f.Step == step {
			out = append(out, f)
		}
	}
	return out
}

func (r *DeprecationRegistry) UpdateStatus(filePath string, status DeprecationStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[filePath]
	if !ok {
		return fmt.Errorf("legacy_deprecation: file not registered: %s", filePath)
	}
	f.Status = status
	return nil
}

func (r *DeprecationRegistry) Save(path string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := struct {
		GeneratedAt time.Time    `json:"generatedAt"`
		Files       []LegacyFile `json:"files"`
	}{
		GeneratedAt: time.Now().UTC(),
		Files:       make([]LegacyFile, 0, len(r.files)),
	}
	for _, f := range r.files {
		out.Files = append(out.Files, *f)
	}
	sort.SliceStable(out.Files, func(i, j int) bool {
		if out.Files[i].Step != out.Files[j].Step {
			return out.Files[i].Step < out.Files[j].Step
		}
		return out.Files[i].FilePath < out.Files[j].FilePath
	})
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

const deprecatedHeader = `// Deprecated: %s
// 替代: %s
// 步骤: 第%d步
// 本文件已标记为弃用，cutover 后将通过 legacy_deprecation 框架逐步删除。
// 新代码禁止直接引用本文件，旧入口由 cutover manager 重定向到 extension/kernel。
// 详见: docs/amitiax/Amitia_扩展系统重构_第%d步_*.md

`

func (r *DeprecationRegistry) ApplyDeprecatedHeader(filePath, reason, replacement string, step int) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("legacy_deprecation: file not found: %s", filePath)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	str := string(content)
	if strings.HasPrefix(str, "// Deprecated:") {
		return nil
	}
	header := fmt.Sprintf(deprecatedHeader, reason, replacement, step, step)
	updated := header + str
	if err := os.WriteFile(filePath, []byte(updated), 0o644); err != nil {
		return err
	}
	return r.Mark(LegacyFile{
		FilePath:    filePath,
		Step:        step,
		Reason:      reason,
		Replacement: replacement,
		Status:      StatusMarked,
	})
}

func DefaultRegistry() *DeprecationRegistry {
	r := NewDeprecationRegistry()
	step68Files := []LegacyFile{
		{
			FilePath:         "backend/internal/extension/package_installer.go",
			Package:          "extension",
			Step:             68,
			Reason:           "旧 .amitiax 包安装器，由 extension/kernel/amitiax 替代",
			Replacement:      "extension/kernel/amitiax",
			Status:           StatusMarked,
			BlockingDeletion: true,
			ProductionRefs:   []string{"package_handler.go:128", "package_lifecycle.go:320"},
			TestRefs: []string{
				"package_manager_test.go",
				"package_baseline_test.go",
			},
			Remediation: "package_handler.go 改为调用 extension/kernel/amitiax 的 Install，旧 Install 保留为 internal 包装但不再新增逻辑",
		},
	}
	for _, f := range step68Files {
		_ = r.Mark(f)
	}

	step70Files := []LegacyFile{
		{
			FilePath:         "backend/internal/extension/kernel/event_bus/bus.go",
			Package:          "event_bus",
			Step:             70,
			Reason:           "旧 EventBus 实现，由 extension/kernel/event 替代",
			Replacement:      "extension/kernel/event",
			Status:           StatusFrozen,
			BlockingDeletion: false,
			ProductionRefs:   []string{},
			TestRefs:         []string{"event_bus/bus_test.go"},
			Remediation:      "零调用，可直接删除；保留 bus_test.go 至新 Event 系统测试覆盖率达标后一并删除",
		},
		{
			FilePath:         "backend/internal/extension/kernel/event_bus/pipeline.go",
			Package:          "event_bus",
			Step:             70,
			Reason:           "旧 Event Pipeline，由 extension/kernel/event.Dispatcher 替代",
			Replacement:      "extension/kernel/event.Dispatcher",
			Status:           StatusFrozen,
			BlockingDeletion: false,
			ProductionRefs:   []string{},
			TestRefs:         []string{},
			Remediation:      "零调用，可直接删除",
		},
	}
	for _, f := range step70Files {
		_ = r.Mark(f)
	}

	return r
}
