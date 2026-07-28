package repair_baseline

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBaseline_CLI_AmitiaxIsPrimaryName(t *testing.T) {
	src := readCLIMainSource(t)
	if !strings.Contains(src, "amitiax - Amitia") {
		t.Fatalf("CLI usage must show 'amitiax' as the primary binary name; Phase 7 section 11.1 requires amitiax as the official command")
	}
	if !strings.Contains(src, `amitiax <命令>`) {
		t.Fatalf("CLI usage must use 'amitiax' as the command prefix in usage text")
	}
	if !strings.Contains(src, "amitiax v%s") {
		t.Fatalf("CLI version output must use 'amitiax' as the binary name")
	}
}

func TestBaseline_CLI_AmitiaExtIsCompatibilityAlias(t *testing.T) {
	src := readCLIMainSource(t)
	if !strings.Contains(src, "amitia-ext") {
		t.Fatalf("CLI must retain 'amitia-ext' as a compatibility alias; Phase 7 section 11.2 requires short-term alias retention")
	}
	if !strings.Contains(src, "兼容别名") {
		t.Fatalf("CLI must document 'amitia-ext' explicitly as a compatibility alias")
	}
}

func TestBaseline_CLI_HasAllRequiredCommands(t *testing.T) {
	src := readCLIMainSource(t)
	requiredCommands := []string{
		`case "init"`,
		`case "validate"`,
		`case "dev"`,
		`case "inspect"`,
		`case "test"`,
		`case "pack"`,
		`case "sign"`,
		`case "verify"`,
		`case "doctor"`,
		`case "export-diagnostics"`,
	}
	missing := []string{}
	for _, cmd := range requiredCommands {
		if !strings.Contains(src, cmd) {
			missing = append(missing, cmd)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("CLI must implement all 10 required commands from Phase 7 section 11.4; missing=%v", missing)
	}
}

func TestBaseline_CLI_SourceInGoModule(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	cliDir := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "cmd", "amitia-ext")
	if _, err := os.Stat(filepath.Join(cliDir, "main.go")); err != nil {
		t.Fatalf("Go CLI source must exist at backend/cmd/amitia-ext/main.go; Phase 7 section 11.1 chooses the Go CLI as the sole production implementation: %v", err)
	}
	signPath := filepath.Join(cliDir, "cmd_sign.go")
	if _, err := os.Stat(signPath); err != nil {
		t.Fatalf("Go CLI must implement sign command (cmd_sign.go); Phase 7 forbids TypeScript CLI from implementing signing: %v", err)
	}
	verifyPath := filepath.Join(cliDir, "cmd_verify.go")
	if _, err := os.Stat(verifyPath); err != nil {
		t.Fatalf("Go CLI must implement verify command (cmd_verify.go); Phase 7 forbids TypeScript CLI from implementing verification: %v", err)
	}
	packPath := filepath.Join(cliDir, "cmd_pack.go")
	if _, err := os.Stat(packPath); err != nil {
		t.Fatalf("Go CLI must implement pack command (cmd_pack.go); Phase 7 requires Go CLI as sole packing implementation: %v", err)
	}
}

func readCLIMainSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "cmd", "amitia-ext", "main.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read CLI main.go: %v", err)
	}
	return string(data)
}
