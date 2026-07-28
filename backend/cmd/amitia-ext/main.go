package main

import (
	"fmt"
	"os"
	"strings"
)

func usage() {
	fmt.Fprintf(os.Stderr, `amitiax - Amitia 扩展 CLI 工具 (v%s)

用法:
  amitiax <命令> [选项] [参数]

别名:
  amitia-ext  (兼容别名，指向同一二进制)

命令:
  init               创建新扩展项目
  validate           校验 manifest.json 或 .amitiax 包
  pack               将目录打包成 .amitiax
  sign               用 ed25519 私钥签名 .amitiax 包
  verify             验证 .amitiax 包的完整性和签名
  inspect            显示 .amitiax 包的详细信息
  doctor             检查环境
  keys               密钥管理 (create, export-public)
  dev                启动开发模式（连接 Developer Host）
  test               运行扩展测试（查找 tests/ 下的 .test.js）
  export-diagnostics 导出诊断包（JSON 格式）
  version            显示版本信息
  help               显示帮助信息

全局选项:
  --json        以 JSON 格式输出

示例:
  amitiax init --name com.example.my-ext --dir ./my-ext
  amitiax validate manifest.json
  amitiax validate my-ext.amitiax
  amitiax pack --dir ./my-ext -o my-ext.amitiax
  amitiax sign my-ext.amitiax --key private.key --key-id my-key --publisher com.example
  amitiax verify my-ext.amitiax --pubkey public.key
  amitiax inspect my-ext.amitiax --files
  amitiax doctor
  amitiax keys create --output ./keys
  amitiax keys export-public --key private.key
  amitiax dev ./my-ext --host localhost:18899
  amitiax test ./my-ext --host-version 0.1.0 --platform windows
  amitiax export-diagnostics ./my-ext diagnostics.json
`, CLIVersion)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(ExitConfig)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	jsonMode := false
	var filtered []string
	for _, a := range args {
		if a == "--json" {
			jsonMode = true
		} else {
			filtered = append(filtered, a)
		}
	}
	args = rearrangeArgs(filtered)

	output := newOutput(jsonMode)

	switch cmd {
	case "init":
		os.Exit(runInit(args, output))
	case "validate":
		os.Exit(runValidate(args, output))
	case "pack":
		os.Exit(runPack(args, output))
	case "sign":
		os.Exit(runSign(args, output))
	case "verify":
		os.Exit(runVerify(args, output))
	case "inspect":
		os.Exit(runInspect(args, output))
	case "doctor":
		os.Exit(runDoctor(args, output))
	case "keys":
		os.Exit(runKeys(args, output))
	case "dev":
		os.Exit(runDev(args, output))
	case "test":
		os.Exit(runTest(args, output))
	case "export-diagnostics":
		os.Exit(runExportDiagnostics(args, output))
	case "version", "--version", "-v":
		output.emit(Result{OK: true, Message: fmt.Sprintf("amitiax v%s", CLIVersion), Data: map[string]any{"version": CLIVersion}})
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		usage()
		os.Exit(ExitConfig)
	}
}

func rearrangeArgs(args []string) []string {
	var flags []string
	var positional []string
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) {
				next := args[i+1]
				if next != "-" && !strings.HasPrefix(next, "-") {
					flags = append(flags, next)
					i++
				}
			}
		} else {
			positional = append(positional, a)
		}
		i++
	}
	return append(flags, positional...)
}
