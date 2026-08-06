// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const ExitTestFailed = 5

type testCaseResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration string `json:"duration,omitempty"`
	Error    string `json:"error,omitempty"`
}

type testRunSummary struct {
	Total    int              `json:"total"`
	Passed   int              `json:"passed"`
	Failed   int              `json:"failed"`
	Skipped  int              `json:"skipped"`
	Duration string           `json:"duration"`
	Cases    []testCaseResult `json:"cases"`
}

func runTest(args []string, output *Output) int {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	hostVersion := fs.String("host-version", "", "目标 Host 版本")
	platform := fs.String("platform", "", "目标平台（如 windows/darwin/linux）")
	runtimeVersion := fs.String("runtime-version", "", "目标运行时版本")
	fs.Parse(args)

	targetDir := "."
	if fs.NArg() >= 1 {
		targetDir = fs.Arg(0)
	}
	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		output.fail(ExitInternal, fmt.Sprintf("解析目录失败: %v", err))
	}
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		output.fail(ExitEnv, fmt.Sprintf("目录不存在: %s", absDir))
	}

	testsDir := filepath.Join(absDir, "tests")
	if info, err := os.Stat(testsDir); err != nil || !info.IsDir() {
		output.emit(Result{
			OK:      true,
			Message: fmt.Sprintf("未找到 tests/ 目录，跳过测试: %s", absDir),
			Data: map[string]any{
				"total":   0,
				"passed":  0,
				"failed":  0,
				"skipped": 0,
			},
		})
		return ExitSuccess
	}

	testFiles, err := findTestFiles(testsDir)
	if err != nil {
		output.fail(ExitFailure, fmt.Sprintf("查找测试文件失败: %v", err))
	}
	if len(testFiles) == 0 {
		output.emit(Result{
			OK:      true,
			Message: fmt.Sprintf("未找到 .test.js 测试文件: %s", testsDir),
			Data: map[string]any{
				"total":   0,
				"passed":  0,
				"failed":  0,
				"skipped": 0,
			},
		})
		return ExitSuccess
	}

	deps, err := NewRuntimeDependencies()
	if err != nil {
		output.fail(ExitEnv, fmt.Sprintf("运行时依赖初始化失败: %v", err))
	}

	env, err := deps.NodeResolver.Resolve(context.Background())
	if err != nil || env.NodeBinary == "" {
		output.fail(ExitEnv, "未找到管理 Node.js 可执行文件（扩展测试运行时需要 Node.js）")
	}

	nodePath := env.NodeBinary

	envVars := buildTestEnv(*hostVersion, *platform, *runtimeVersion)

	summary := testRunSummary{}
	overallStart := time.Now()

	for _, tf := range testFiles {
		result := runOneTestFile(nodePath, tf, envVars, output)
		summary.Cases = append(summary.Cases, result)
		summary.Total++
		switch result.Status {
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		}
	}
	summary.Duration = time.Since(overallStart).String()

	data := map[string]any{
		"total":    summary.Total,
		"passed":   summary.Passed,
		"failed":   summary.Failed,
		"skipped":  summary.Skipped,
		"duration": summary.Duration,
		"nodePath": nodePath,
		"nodeSource": string(env.Source),
		"cases":    summary.Cases,
	}
	if *hostVersion != "" || *platform != "" || *runtimeVersion != "" {
		data["env"] = map[string]any{
			"hostVersion":    *hostVersion,
			"platform":       *platform,
			"runtimeVersion": *runtimeVersion,
		}
	}

	ok := summary.Failed == 0
	result := Result{
		OK:      ok,
		Message: fmt.Sprintf("测试结果: %d 通过, %d 失败, %d 跳过 (%s)", summary.Passed, summary.Failed, summary.Skipped, summary.Duration),
		Data:    data,
	}
	if !ok {
		var errs []string
		for _, c := range summary.Cases {
			if c.Status == "failed" {
				errs = append(errs, fmt.Sprintf("%s: %s", c.Name, c.Error))
			}
		}
		result.Errors = errs
	}

	output.emit(result)
	if ok {
		return ExitSuccess
	}
	return ExitTestFailed
}

func findTestFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".test.js") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func buildTestEnv(hostVersion, platform, runtimeVersion string) []string {
	env := os.Environ()
	if hostVersion != "" {
		env = append(env, "AMITIA_HOST_VERSION="+hostVersion)
	}
	if platform != "" {
		env = append(env, "AMITIA_PLATFORM="+platform)
	}
	if runtimeVersion != "" {
		env = append(env, "AMITIA_RUNTIME_VERSION="+runtimeVersion)
	}
	return env
}

func runOneTestFile(nodePath, file string, env []string, output *Output) testCaseResult {
	name := strings.TrimSuffix(filepath.Base(file), ".test.js")
	start := time.Now()
	output.infof("运行测试: %s\n", file)

	cmd := exec.Command(nodePath, file)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	duration := time.Since(start)

	if err == nil {
		return testCaseResult{
			Name:     name,
			Status:   "passed",
			Duration: duration.String(),
		}
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ExitCode() {
		case 0:
			return testCaseResult{
				Name:     name,
				Status:   "passed",
				Duration: duration.String(),
			}
		case 77:
			return testCaseResult{
				Name:     name,
				Status:   "skipped",
				Duration: duration.String(),
				Error:    fmt.Sprintf("exit code %d", exitErr.ExitCode()),
			}
		default:
			return testCaseResult{
				Name:     name,
				Status:   "failed",
				Duration: duration.String(),
				Error:    fmt.Sprintf("exit code %d", exitErr.ExitCode()),
			}
		}
	}

	return testCaseResult{
		Name:     name,
		Status:   "failed",
		Duration: duration.String(),
		Error:    err.Error(),
	}
}
