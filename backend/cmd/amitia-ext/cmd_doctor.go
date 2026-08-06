// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
)

func runDoctor(args []string, output *Output) int {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	fs.Parse(args)

	data := map[string]any{
		"cliVersion":    CLIVersion,
		"goVersion":     runtime.Version(),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"cpuCount":      runtime.NumCPU(),
		"packageFormat": "amitiax",
	}

	deps, err := NewRuntimeDependencies()
	if err != nil {
		data["nodePath"] = "不可用"
		data["nodeSource"] = "依赖初始化失败: " + err.Error()
		warnings := []string{"运行时依赖初始化失败: " + err.Error()}
		result := Result{
			OK:       true,
			Message:  "环境检查完成（部分功能受限）",
			Data:     data,
			Warnings: warnings,
		}
		output.emit(result)
		return ExitSuccess
	}

	env, err := deps.NodeResolver.Resolve(context.Background())
	if err != nil || env.NodeBinary == "" {
		data["nodePath"] = "不可用"
		data["nodeSource"] = "Node 解析失败"
		if err != nil {
			data["nodeError"] = err.Error()
		}
		warnings := []string{"管理 Node.js 未就绪（扩展运行时可能需要）"}
		result := Result{
			OK:       true,
			Message:  "环境检查完成（部分功能受限）",
			Data:     data,
			Warnings: warnings,
		}
		output.emit(result)
		return ExitSuccess
	}

	data["nodePath"] = env.NodeBinary
	data["nodeSource"] = string(env.Source)
	if env.DistributionRoot != "" {
		data["distributionRoot"] = env.DistributionRoot
	}
	data["npmAvailable"] = env.NPMCLI != ""
	data["npxAvailable"] = env.NPXCLI != ""
	data["workDir"] = env.WorkDir

	if nodeVersion, err := exec.Command(env.NodeBinary, "--version").Output(); err == nil {
		data["nodeVersion"] = fmt.Sprintf("%s", nodeVersion)
	}

	if cwd, err := os.Getwd(); err == nil {
		data["cwd"] = cwd
	}

	data["supportedDirs"] = []string{
		amitiax.ModulesDir,
		amitiax.ResourcesDir,
		amitiax.AssetsDir,
		amitiax.MigrationsDir,
		amitiax.LicensesDir,
		amitiax.DocsDir,
	}
	data["signatureAlgorithm"] = "ed25519"

	warnings := []string{}
	if !env.PackageManagementAvailable {
		warnings = append(warnings, "npm/npx CLI 不可用，包管理功能可能受限")
	}

	result := Result{
		OK:       true,
		Message:  "环境检查完成",
		Data:     data,
		Warnings: warnings,
	}

	output.emit(result)
	return ExitSuccess
}
