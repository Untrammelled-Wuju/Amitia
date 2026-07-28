package final_acceptance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	defaultReportDir    = "../../../../../docs/extension-kernel/reports"
	defaultMarkdownName = "release-readiness-report.md"
	defaultJSONName     = "release-readiness-report.json"
)

func TestFinalAcceptanceSuite(t *testing.T) {
	suite := DefaultSuite()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	report, err := suite.Run(ctx)
	if err != nil {
		t.Fatalf("final acceptance run failed: %v", err)
	}

	if !report.ReleaseReady {
		for _, it := range report.Items {
			if it.Status != StatusPassed {
				t.Logf("item %s: status=%s err=%s", it.ItemID, it.Status, it.Error)
			}
		}
		t.Fatalf("release not ready: %d failed, blocking=%v", report.Summary.Failed, report.BlockingIssues)
	}

	reportDir := resolveReportDir(t)
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir report dir: %v", err)
	}

	jsonPath := filepath.Join(reportDir, defaultJSONName)
	if err := writeJSON(report, jsonPath); err != nil {
		t.Fatalf("write json report: %v", err)
	}
	t.Logf("wrote json report: %s", jsonPath)

	mdPath := filepath.Join(reportDir, defaultMarkdownName)
	if err := writeMarkdown(report, mdPath); err != nil {
		t.Fatalf("write markdown report: %v", err)
	}
	t.Logf("wrote markdown report: %s", mdPath)

	matrixPath := filepath.Join(reportDir, "..", "final-acceptance-matrix.md")
	if _, err := os.Stat(matrixPath); err != nil {
		t.Logf("matrix not found at %s (acceptable)", matrixPath)
	} else {
		t.Logf("matrix found at %s", matrixPath)
	}

	for _, it := range report.Items {
		if it.Required && it.Status != StatusPassed {
			t.Errorf("required item failed: %s (%s) status=%s err=%s", it.ItemID, it.Title, it.Status, it.Error)
		}
	}
}

func resolveReportDir(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("AMITIA_FINAL_ACCEPTANCE_REPORT_DIR"); v != "" {
		return v
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, defaultReportDir))
}

func writeJSON(report *FinalReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeMarkdown(report *FinalReport, path string) error {
	var b strings.Builder
	b.WriteString("# Extension Kernel Release Readiness Report\n\n")
	b.WriteString("> 由 `backend/internal/extension/kernel/final_acceptance` 在第 70 步生成。\n\n")
	b.WriteString("## 概要\n\n")
	b.WriteString("| 字段 | 值 |\n|------|----|\n")
	b.WriteString("| Report ID | " + report.ReportID + " |\n")
	b.WriteString("| 生成时间 | " + report.GeneratedAt.Format(time.RFC3339) + " |\n")
	b.WriteString("| 开始时间 | " + report.StartedAt.Format(time.RFC3339) + " |\n")
	if report.EndedAt != nil {
		b.WriteString("| 结束时间 | " + report.EndedAt.Format(time.RFC3339) + " |\n")
	}
	b.WriteString("| 总项 | " + itoa(report.Summary.Total) + " |\n")
	b.WriteString("| 通过 | " + itoa(report.Summary.Passed) + " |\n")
	b.WriteString("| 失败 | " + itoa(report.Summary.Failed) + " |\n")
	b.WriteString("| 跳过 | " + itoa(report.Summary.Skipped) + " |\n")
	b.WriteString("| 阻塞 | " + itoa(report.Summary.Blocked) + " |\n")
	b.WriteString("| Required | " + itoa(report.Summary.Required) + " |\n")
	b.WriteString("| 结果 | " + report.Outcome + " |\n")
	b.WriteString("| Release Ready | " + boolStr(report.ReleaseReady) + " |\n\n")

	b.WriteString("## 签字\n\n")
	b.WriteString("| 角色 | 状态 |\n|------|------|\n")
	b.WriteString("| Architecture | " + boolStr(report.SignOff.ArchitectureApproved) + " |\n")
	b.WriteString("| Security | " + boolStr(report.SignOff.SecurityApproved) + " |\n")
	b.WriteString("| Stability | " + boolStr(report.SignOff.StabilityApproved) + " |\n")
	b.WriteString("| DevExperience | " + boolStr(report.SignOff.DevExperienceApproved) + " |\n")
	b.WriteString("| Release | " + boolStr(report.SignOff.ReleaseApproved) + " |\n\n")

	if len(report.BlockingIssues) > 0 {
		b.WriteString("## 阻断项\n\n")
		for _, issue := range report.BlockingIssues {
			b.WriteString("- " + issue + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## 验收项明细\n\n")
	b.WriteString("| Item ID | Stage | Title | Required | Status | Evidence |\n")
	b.WriteString("|---------|-------|-------|----------|--------|----------|\n")
	for _, it := range report.Items {
		b.WriteString("| " + it.ItemID + " | " + string(it.Stage) + " | " + it.Title + " | " + boolStr(it.Required) + " | " + string(it.Status) + " | " + strings.Join(it.Evidence, "; ") + " |\n")
	}
	b.WriteString("\n")

	b.WriteString("## 建议\n\n")
	for _, rec := range report.Recommendations {
		b.WriteString("- " + rec + "\n")
	}
	b.WriteString("\n")

	b.WriteString("## 发布演练\n\n")
	b.WriteString("1. 旧版本数据准备\n2. 升级应用\n3. 迁移\n4. 切换\n5. 启动\n6. 核心扩展运行\n7. 安装新包\n8. 更新\n9. 回滚 Extension\n10. 禁用/启用\n11. 卸载\n12. 应用更新回滚\n13. 数据库恢复\n14. 诊断包\n15. 关闭\n\n")

	b.WriteString("## 协议版本基线\n\n")
	b.WriteString("- Extension Kernel v1\n- Manifest v2\n- Host API v1\n- Runtime RPC v1\n- Schema UI v1\n- UI Contract v1\n- SDK v1\n\n")

	b.WriteString("## 已知限制\n\n")
	b.WriteString("详见 `docs/extension-kernel/known-limitations.md`。\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	negative := false
	if i < 0 {
		negative = true
		i = -i
	}
	buf := make([]byte, 0, 11)
	for i > 0 {
		buf = append([]byte{digits[i%10]}, buf...)
		i /= 10
	}
	if negative {
		return "-" + string(buf)
	}
	return string(buf)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
