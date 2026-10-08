package trusted_service

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

func TestTaskSandboxRejectsInvalidBudgetBeforeLaunch(t *testing.T) {
	for _, budget := range []process.ResourceLimits{{}, {MaxMemoryBytes: 513 << 20, MaxCPUPercent: 50, MaxProcesses: 1}, {MaxMemoryBytes: 128 << 20, MaxCPUPercent: 51, MaxProcesses: 1}, {MaxMemoryBytes: 128 << 20, MaxCPUPercent: 50, MaxProcesses: 2}} {
		if _, err := PrepareTaskSandbox("invalid", nil, "invalid", "invalid", "invalid", budget); err == nil {
			t.Fatal("invalid task budget admitted")
		}
	}
}

func TestActualTaskSandboxBlocksPrivateFilesNetworkAndChildProcesses(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows AppContainer boundary")
	}
	node := os.Getenv("AMITIA_TEST_NODE")
	if node == "" {
		t.Skip("AMITIA_TEST_NODE required")
	}
	work, host, bundle, private := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	secret := filepath.Join(private, "core-private.json")
	if err := os.WriteFile(secret, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	readonly := filepath.Join(bundle, "entry.js")
	if err := os.WriteFile(readonly, []byte("bundle"), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	args, _ := json.Marshal(map[string]any{"secret": secret, "readonly": readonly, "work": filepath.Join(work, "saved.json"), "port": port})
	script := `const fs=require('node:fs');const net=require('node:net');const cp=require('node:child_process');const cfg=JSON.parse(process.argv[1]);let result={};try{fs.readFileSync(cfg.secret);result.privateDenied=false}catch{result.privateDenied=true}try{fs.writeFileSync(cfg.readonly,'overwritten');result.bundleReadOnly=false}catch{result.bundleReadOnly=true}result.bundleReadable=fs.readFileSync(cfg.readonly,'utf8')==='bundle';fs.writeFileSync(cfg.work,'owned');result.workWritable=true;const child=cp.spawnSync(process.execPath,['-e','process.stdout.write("escaped")'],{timeout:2000});result.childDenied=!!child.error;const socket=net.connect({host:'127.0.0.1',port:cfg.port});let done=false;function finish(denied){if(done)return;done=true;result.networkDenied=denied;socket.destroy();console.log(JSON.stringify(result));}socket.on('connect',()=>finish(false));socket.on('error',()=>finish(true));socket.setTimeout(3000,()=>finish(true));`
	script = strings.Replace(script, "const child=cp.spawnSync(process.execPath,['-e','process.stdout.write(\"escaped\")'],{timeout:2000});result.childDenied=!!child.error;", "try{cp.spawnSync(process.execPath,['-e','process.stdout.write(\"escaped\")'],{timeout:2000});result.childDenied=false}catch(error){result.childDenied=error.code==='ERR_ACCESS_DENIED'}", 1)
	plan, err := PrepareTaskSandbox(node, []string{"--permission", "--allow-fs-read=*", "--allow-fs-write=*", "-e", script, string(args)}, work, host, bundle, process.ResourceLimits{MaxMemoryBytes: 256 << 20, MaxCPUPercent: 50, MaxProcesses: 1})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Cleanup != nil {
		defer plan.Cleanup()
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, plan.Path, plan.Args...)
	cmd.Dir = plan.WorkingDir
	cmd.Env = process.NewEnvironmentBuilder().Build()
	process.ConfigureProcess(cmd)
	var outputBuffer bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outputBuffer, &outputBuffer
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tree, err := process.AttachProcessTreeWithLimits(cmd, plan.SupervisorLimits)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	defer process.CloseProcessTree(tree)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("sandbox failed: %v output=%s", err, outputBuffer.String())
	}
	var result map[string]bool
	resultLine := strings.TrimSpace(outputBuffer.String())
	if newline := strings.LastIndexByte(resultLine, '\n'); newline >= 0 {
		resultLine = resultLine[newline+1:]
	}
	if err := json.Unmarshal([]byte(resultLine), &result); err != nil {
		t.Fatalf("sandbox output: %s: %v", outputBuffer.String(), err)
	}
	for _, key := range []string{"privateDenied", "bundleReadOnly", "bundleReadable", "workWritable", "childDenied", "networkDenied"} {
		if !result[key] {
			t.Fatalf("sandbox boundary not enforced: %s %v", key, result)
		}
	}
}
