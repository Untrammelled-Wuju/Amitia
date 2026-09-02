package tool

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	maxSandboxScriptBytes = 128 * 1024
	maxSandboxInputBytes  = 64 * 1024
	maxSandboxOutputBytes = 1024 * 1024
)

var (
	sandboxEnvNameRE        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
	sandboxNodeResolverMu   sync.RWMutex
	sandboxNodePathResolver func(context.Context) (string, error)
)

func SetSandboxNodePathResolver(resolver func(context.Context) (string, error)) {
	sandboxNodeResolverMu.Lock()
	sandboxNodePathResolver = resolver
	sandboxNodeResolverMu.Unlock()
}

func init() {
	readParams, _ := ParseParametersSchema(json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string","pattern":"^[A-Za-z_][A-Za-z0-9_]{0,127}$"}}}`))
	Register(Tool{Type: "function", Function: Function{Name: "read_environment_variable", Description: "Read one sandbox-package environment variable, or list variable names when name is omitted. Values are scoped to the current user/character and are not process-global environment variables.", Parameters: readParams}}, readEnvironmentVariable)

	writeParams, _ := ParseParametersSchema(json.RawMessage(`{"type":"object","required":["name"],"additionalProperties":false,"properties":{"name":{"type":"string","pattern":"^[A-Za-z_][A-Za-z0-9_]{0,127}$"},"value":{"type":"string","maxLength":65536},"delete":{"type":"boolean"}}}`))
	Register(Tool{Type: "function", Function: Function{Name: "write_environment_variable", Description: "Create, update, or delete one sandbox-package environment variable in the current user/character scope. This never mutates the backend process environment.", Parameters: writeParams}}, writeEnvironmentVariable)

	scriptParams, _ := ParseParametersSchema(json.RawMessage(`{"type":"object","required":["script"],"additionalProperties":false,"properties":{"script":{"type":"string","minLength":1,"maxLength":131072},"input":{},"timeout_ms":{"type":"integer","minimum":100,"maximum":30000}}}`))
	Register(Tool{Type: "function", Function: Function{Name: "execute_sandbox_script_direct", Description: "Execute inline JavaScript in a restricted Node vm context with no require, process, filesystem, network, child-process or dynamic-code access. The script reads input/env and assigns its JSON-serializable answer to result.", Parameters: scriptParams}}, executeSandboxScriptDirect)
}

func readEnvironmentVariable(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopeID, errResult := sandboxScope(execCtx)
	if errResult != nil {
		return *errResult
	}
	if err := ensureSandboxEnvironmentTable(callCtx); err != nil {
		return ErrorResult("sandbox_env_storage_failed", "ERROR: "+err.Error())
	}
	name := strings.TrimSpace(stringArg(args, "name"))
	if name != "" {
		if err := validateSandboxEnvName(name); err != nil {
			return ErrorResult("invalid_args", "ERROR: "+err.Error())
		}
		var value string
		err := toolDB.QueryRowContext(callCtx, `SELECT value FROM sandbox_environment_variables WHERE scope_id = ? AND name = ?`, scopeID, name).Scan(&value)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrorResult("environment_variable_not_found", "ERROR: environment variable not found")
		}
		if err != nil {
			return ErrorResult("sandbox_env_read_failed", "ERROR: "+err.Error())
		}
		encoded, _ := json.Marshal(map[string]interface{}{"name": name, "value": value})
		return TextResult(string(encoded))
	}
	rows, err := toolDB.QueryContext(callCtx, `SELECT name FROM sandbox_environment_variables WHERE scope_id = ? ORDER BY name ASC LIMIT 512`, scopeID)
	if err != nil {
		return ErrorResult("sandbox_env_read_failed", "ERROR: "+err.Error())
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			names = append(names, n)
		}
	}
	encoded, _ := json.Marshal(map[string]interface{}{"names": names, "count": len(names)})
	return TextResult(string(encoded))
}

func writeEnvironmentVariable(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopeID, errResult := sandboxScope(execCtx)
	if errResult != nil {
		return *errResult
	}
	if err := ensureSandboxEnvironmentTable(callCtx); err != nil {
		return ErrorResult("sandbox_env_storage_failed", "ERROR: "+err.Error())
	}
	name := strings.TrimSpace(stringArg(args, "name"))
	if err := validateSandboxEnvName(name); err != nil {
		return ErrorResult("invalid_args", "ERROR: "+err.Error())
	}
	deleteValue, _ := args["delete"].(bool)
	if deleteValue {
		if _, err := toolDB.ExecContext(callCtx, `DELETE FROM sandbox_environment_variables WHERE scope_id = ? AND name = ?`, scopeID, name); err != nil {
			return ErrorResult("sandbox_env_write_failed", "ERROR: "+err.Error())
		}
		encoded, _ := json.Marshal(map[string]interface{}{"name": name, "deleted": true})
		result := TextResult(string(encoded))
		result.SideEffects = []ToolSideEffect{{Type: "sandbox_environment_delete", TargetID: name, Confirmed: true}}
		return result
	}
	value, ok := args["value"].(string)
	if !ok {
		return ErrorResult("invalid_args", "ERROR: value is required unless delete=true")
	}
	if len(value) > 64*1024 {
		return ErrorResult("invalid_args", "ERROR: value exceeds 65536 bytes")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := toolDB.ExecContext(callCtx, `INSERT INTO sandbox_environment_variables(scope_id,name,value,created_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(scope_id,name) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, scopeID, name, value, now, now)
	if err != nil {
		return ErrorResult("sandbox_env_write_failed", "ERROR: "+err.Error())
	}
	encoded, _ := json.Marshal(map[string]interface{}{"name": name, "written": true, "length": len(value)})
	result := TextResult(string(encoded))
	result.SideEffects = []ToolSideEffect{{Type: "sandbox_environment_write", TargetID: name, Confirmed: true}}
	return result
}

func executeSandboxScriptDirect(callCtx context.Context, execCtx ToolExecutionContext, args map[string]interface{}) ToolCallResult {
	if err := callCtx.Err(); err != nil {
		return CancelledResult(err.Error())
	}
	scopeID, errResult := sandboxScope(execCtx)
	if errResult != nil {
		return *errResult
	}
	script := stringArg(args, "script")
	if strings.TrimSpace(script) == "" {
		return ErrorResult("invalid_args", "ERROR: script is required")
	}
	if len(script) > maxSandboxScriptBytes {
		return ErrorResult("invalid_args", fmt.Sprintf("ERROR: script exceeds %d bytes", maxSandboxScriptBytes))
	}
	timeoutMS := boundedIntArg(args, "timeout_ms", 5000, 100, 30000)
	input := args["input"]
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return ErrorResult("invalid_args", "ERROR: input must be JSON serializable")
	}
	if len(inputJSON) > maxSandboxInputBytes {
		return ErrorResult("invalid_args", fmt.Sprintf("ERROR: input exceeds %d bytes", maxSandboxInputBytes))
	}
	if err := ensureSandboxEnvironmentTable(callCtx); err != nil {
		return ErrorResult("sandbox_env_storage_failed", "ERROR: "+err.Error())
	}
	env, err := readSandboxEnvironmentMap(callCtx, scopeID)
	if err != nil {
		return ErrorResult("sandbox_env_read_failed", "ERROR: "+err.Error())
	}
	envJSON, _ := json.Marshal(env)

	node, err := findNodeBinary(callCtx)
	if err != nil {
		return ErrorResult("sandbox_runtime_unavailable", "ERROR: "+err.Error())
	}
	tmpDir, err := os.MkdirTemp("", "amitia-sandbox-script-")
	if err != nil {
		return ErrorResult("sandbox_prepare_failed", "ERROR: "+err.Error())
	}
	defer os.RemoveAll(tmpDir)
	if err := os.Chmod(tmpDir, 0700); err != nil {
		return ErrorResult("sandbox_prepare_failed", "ERROR: "+err.Error())
	}
	userScript := filepath.Join(tmpDir, "user.js")
	if err := os.WriteFile(userScript, []byte(script), 0600); err != nil {
		return ErrorResult("sandbox_prepare_failed", "ERROR: "+err.Error())
	}
	inputPath := filepath.Join(tmpDir, "input.json")
	if err := os.WriteFile(inputPath, inputJSON, 0600); err != nil {
		return ErrorResult("sandbox_prepare_failed", "ERROR: "+err.Error())
	}
	envPath := filepath.Join(tmpDir, "environment.json")
	if err := os.WriteFile(envPath, envJSON, 0600); err != nil {
		return ErrorResult("sandbox_prepare_failed", "ERROR: "+err.Error())
	}
	wrapperPath := filepath.Join(tmpDir, "runner.js")
	wrapper := restrictedNodeVMRunner(timeoutMS)
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0600); err != nil {
		return ErrorResult("sandbox_prepare_failed", "ERROR: "+err.Error())
	}

	runCtx, cancel := context.WithTimeout(callCtx, time.Duration(timeoutMS+1500)*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(runCtx, node, "--max-old-space-size=64", "--disable-proto=throw", wrapperPath, userScript, inputPath, envPath)
	cmd.Dir = tmpDir
	cmd.Env = minimalSandboxProcessEnvironment(node)
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = maxSandboxOutputBytes, 64*1024
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return ErrorResult("sandbox_timeout", "ERROR: sandbox script timed out or was cancelled")
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return ErrorResult("sandbox_script_failed", "ERROR: "+message)
	}
	if stdout.exceeded {
		return ErrorResult("sandbox_output_too_large", fmt.Sprintf("ERROR: sandbox output exceeds %d bytes", maxSandboxOutputBytes))
	}
	if !json.Valid(stdout.Bytes()) {
		return ErrorResult("sandbox_invalid_result", "ERROR: sandbox returned invalid JSON")
	}
	result := TextResult(stdout.String())
	result.Audit = map[string]interface{}{"sandbox": "node_vm", "network": false, "filesystem": false, "process": false, "timeout_ms": timeoutMS}
	return result
}

func sandboxScope(execCtx ToolExecutionContext) (string, *ToolCallResult) {
	user := strings.TrimSpace(execCtx.User)
	character := strings.TrimSpace(execCtx.CharacterID)
	switch {
	case user != "" && character != "":
		return "user:" + user + "|character:" + character, nil
	case user != "":
		return "user:" + user, nil
	case character != "":
		return "character:" + character, nil
	default:
		result := ErrorResult("missing_sandbox_scope", "ERROR: user or character scope is required")
		return "", &result
	}
}

func ensureSandboxEnvironmentTable(ctx context.Context) error {
	if toolDB == nil {
		return errors.New("tool database not initialized")
	}
	_, err := toolDB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sandbox_environment_variables (scope_id TEXT NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT '', updated_at DATETIME NOT NULL DEFAULT '', PRIMARY KEY(scope_id,name))`)
	return err
}

func validateSandboxEnvName(name string) error {
	if !sandboxEnvNameRE.MatchString(name) {
		return errors.New("name must match ^[A-Za-z_][A-Za-z0-9_]{0,127}$")
	}
	upper := strings.ToUpper(name)
	blocked := []string{"PATH", "NODE_OPTIONS", "NODE_PATH", "LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES", "DYLD_LIBRARY_PATH"}
	for _, candidate := range blocked {
		if upper == candidate {
			return fmt.Errorf("environment variable %s is reserved for sandbox integrity", name)
		}
	}
	if strings.HasPrefix(upper, "AMITIA_") {
		return fmt.Errorf("environment variable %s uses the reserved AMITIA_ prefix", name)
	}
	return nil
}

func readSandboxEnvironmentMap(ctx context.Context, scopeID string) (map[string]string, error) {
	rows, err := toolDB.QueryContext(ctx, `SELECT name, value FROM sandbox_environment_variables WHERE scope_id = ? ORDER BY name ASC LIMIT 512`, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		result[name] = value
	}
	return result, rows.Err()
}

func findNodeBinary(ctx context.Context) (string, error) {
	sandboxNodeResolverMu.RLock()
	resolver := sandboxNodePathResolver
	sandboxNodeResolverMu.RUnlock()
	if resolver != nil {
		if path, err := resolver(ctx); err == nil && strings.TrimSpace(path) != "" {
			return path, nil
		}
	}

	candidates := []string{"node"}
	if runtime.GOOS == "windows" {
		candidates = []string{"node.exe", "node"}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			if abs, absErr := filepath.Abs(path); absErr == nil {
				return abs, nil
			}
			return path, nil
		}
	}
	return "", errors.New("Node.js runtime was not found on PATH")
}

func minimalSandboxProcessEnvironment(node string) []string {
	env := []string{"PATH=" + filepath.Dir(node)}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SystemRoot", "WINDIR", "TEMP", "TMP"} {
			if value := os.Getenv(key); value != "" {
				env = append(env, key+"="+value)
			}
		}
	}
	return env
}

func restrictedNodeVMRunner(timeoutMS int) string {
	return fmt.Sprintf(`"use strict";
const fs = require("node:fs");
const vm = require("node:vm");
const source = fs.readFileSync(process.argv[2], "utf8");
const inputJSON = fs.readFileSync(process.argv[3], "utf8");
const envJSON = fs.readFileSync(process.argv[4], "utf8");
const bootstrap = `+"`"+`"use strict";
const input = JSON.parse(${JSON.stringify(inputJSON)});
const env = Object.freeze(JSON.parse(${JSON.stringify(envJSON)}));
let result = null;
${source}
JSON.stringify({result});`+"`"+`;
const sandbox = Object.create(null);
const context = vm.createContext(sandbox, {name:"amitia-direct-sandbox", codeGeneration:{strings:false, wasm:false}});
let output = new vm.Script(bootstrap, {filename:"user.js"}).runInContext(context, {timeout:%d, breakOnSigint:true});
if (typeof output !== "string") output = JSON.stringify({result:null});
process.stdout.write(output);
`, timeoutMS)
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return b.Buffer.Write(p)
	}
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.exceeded = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.exceeded = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
