package main

import (
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
		"cliVersion":   CLIVersion,
		"goVersion":    runtime.Version(),
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"cpuCount":     runtime.NumCPU(),
		"packageFormat": "amitiax",
	}

	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeVersion := ""
		if out, err := exec.Command(nodePath, "--version").Output(); err == nil {
			nodeVersion = fmt.Sprintf("%s", out)
		}
		data["nodePath"] = nodePath
		if nodeVersion != "" {
			data["nodeVersion"] = nodeVersion
		}
	} else {
		data["nodePath"] = "未找到"
	}

	if cwd, err := os.Getwd(); err == nil {
		data["workDir"] = cwd
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
	if data["nodePath"] == "未找到" {
		warnings = append(warnings, "Node.js 未在 PATH 中找到（扩展运行时可能需要）")
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
