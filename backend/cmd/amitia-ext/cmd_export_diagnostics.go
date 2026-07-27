package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
)

type diagnosticsBundle struct {
	Timestamp   string         `json:"timestamp"`
	CLIVersion  string         `json:"cliVersion"`
	Environment map[string]any `json:"environment"`
	Target      string         `json:"target"`
	TargetType  string         `json:"targetType"`
	Manifest    any            `json:"manifest,omitempty"`
	PackageInfo any            `json:"packageInfo,omitempty"`
	FileList    []string       `json:"fileList,omitempty"`
	BuildLogs   []string       `json:"buildLogs,omitempty"`
	ErrorLogs   []string       `json:"errorLogs,omitempty"`
	Diagnostics []string       `json:"diagnostics,omitempty"`
}

func runExportDiagnostics(args []string, output *Output) int {
	fs := flag.NewFlagSet("export-diagnostics", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	fs.Parse(args)

	target := "."
	if fs.NArg() >= 1 {
		target = fs.Arg(0)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析目标路径失败: %v", err))
	}

	outPath := ""
	if fs.NArg() >= 2 {
		outPath = fs.Arg(1)
	}

	bundle := diagnosticsBundle{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		CLIVersion: CLIVersion,
		Environment: map[string]any{
			"go":       runtime.Version(),
			"os":       runtime.GOOS,
			"arch":     runtime.GOARCH,
			"cpuCount": runtime.NumCPU(),
		},
		Target: absTarget,
	}

	info, err := os.Stat(absTarget)
	if err != nil {
		bundle.TargetType = "missing"
		bundle.ErrorLogs = append(bundle.ErrorLogs, fmt.Sprintf("无法访问目标: %v", err))
	} else if info.IsDir() {
		bundle.TargetType = "directory"
		collectDirectoryDiagnostics(absTarget, &bundle)
	} else if strings.HasSuffix(absTarget, ".amitiax") {
		bundle.TargetType = "amitiax"
		collectPackageDiagnostics(absTarget, &bundle)
	} else {
		bundle.TargetType = "file"
		bundle.Diagnostics = append(bundle.Diagnostics, fmt.Sprintf("目标为普通文件: %s", absTarget))
	}

	if outPath == "" {
		outPath = "diagnostics.json"
	}
	outAbs, err := filepath.Abs(outPath)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析输出路径失败: %v", err))
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("序列化诊断包失败: %v", err))
	}
	if err := os.WriteFile(outAbs, data, 0o644); err != nil {
		output.fail(ExitEnv, fmt.Sprintf("写入诊断包失败: %v", err))
	}

	output.emit(Result{
		OK:      true,
		Message: fmt.Sprintf("诊断包已导出: %s", outAbs),
		Data: map[string]any{
			"output":        outAbs,
			"target":        absTarget,
			"targetType":    bundle.TargetType,
			"fileCount":     len(bundle.FileList),
			"buildLogCount": len(bundle.BuildLogs),
			"errorLogCount": len(bundle.ErrorLogs),
			"timestamp":     bundle.Timestamp,
		},
	})
	return ExitSuccess
}

func collectDirectoryDiagnostics(dir string, bundle *diagnosticsBundle) {
	manifestPath := filepath.Join(dir, "manifest.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var raw any
		if err := json.Unmarshal(data, &raw); err == nil {
			bundle.Manifest = raw
		} else {
			bundle.ErrorLogs = append(bundle.ErrorLogs, fmt.Sprintf("解析 manifest.json 失败: %v", err))
			bundle.Manifest = string(data)
		}
	} else {
		bundle.ErrorLogs = append(bundle.ErrorLogs, fmt.Sprintf("读取 manifest.json 失败: %v", err))
	}

	files := []string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if rel != "." {
				files = append(files, rel+"/")
			}
			return nil
		}
		files = append(files, fmt.Sprintf("%s  size=%d", rel, info.Size()))
		return nil
	})
	if err != nil {
		bundle.ErrorLogs = append(bundle.ErrorLogs, fmt.Sprintf("遍历目录失败: %v", err))
	}
	sort.Strings(files)
	bundle.FileList = files

	for _, logName := range []string{"build.log", "build-logs.txt"} {
		if data, err := os.ReadFile(filepath.Join(dir, logName)); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			for _, l := range lines {
				if strings.TrimSpace(l) != "" {
					bundle.BuildLogs = append(bundle.BuildLogs, l)
				}
			}
		}
	}

	for _, logName := range []string{"error.log", "errors.log", "error-logs.txt"} {
		if data, err := os.ReadFile(filepath.Join(dir, logName)); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			for _, l := range lines {
				if strings.TrimSpace(l) != "" {
					bundle.ErrorLogs = append(bundle.ErrorLogs, l)
				}
			}
		}
	}
}

func collectPackageDiagnostics(pkgPath string, bundle *diagnosticsBundle) {
	pkg, err := amitiax.OpenArchive(pkgPath)
	if err != nil {
		bundle.ErrorLogs = append(bundle.ErrorLogs, fmt.Sprintf("打开 amitiax 包失败: %v", err))
		return
	}

	bundle.Manifest = pkg.Manifest
	bundle.PackageInfo = map[string]any{
		"extensionId": pkg.Manifest.Extension.ID,
		"version":     pkg.Manifest.Extension.Version,
		"publisher":   pkg.Manifest.Publisher.ID,
		"moduleCount": len(pkg.Manifest.Modules),
		"fileCount":   len(pkg.Integrity.Files),
		"treeHash":    pkg.Tree.TreeHash,
		"algorithm":   pkg.Tree.Algorithm,
		"signed":      pkg.Signatures != nil,
	}

	var fileList []string
	sortedFiles := make([]amitiax.FileEntry, len(pkg.Files))
	copy(sortedFiles, pkg.Files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].Path < sortedFiles[j].Path
	})
	for _, f := range sortedFiles {
		if !f.IsDir {
			fileList = append(fileList, fmt.Sprintf("%s  size=%d  sha256=%s", f.Path, f.Size, f.Hash))
		}
	}
	bundle.FileList = fileList

	if pkg.Signatures != nil {
		bundle.Diagnostics = append(bundle.Diagnostics, fmt.Sprintf("签名: keyId=%s publisher=%s algorithm=%s", pkg.Signatures.KeyID, pkg.Signatures.PublisherID, pkg.Signatures.Algorithm))
	}

	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		bundle.ErrorLogs = append(bundle.ErrorLogs, fmt.Sprintf("完整性验证失败: %v", err))
	} else {
		bundle.Diagnostics = append(bundle.Diagnostics, "完整性验证通过")
	}
}
