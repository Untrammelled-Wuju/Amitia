package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

func TestRound27LinuxCgroupPIDBudgetAllowsNodeRuntimeThreads(t *testing.T) {
	if runtime.GOOS != "linux" || !process.ResourceLimitsSupported().Processes {
		t.Skip("requires writable Linux cgroup v2 with pids controller")
	}
	host := fixtureProcessHost(t)
	host.config.NativeLimits = process.ResourceLimits{MaxProcesses: taskProcessPIDBudget(runtime.GOOS)}
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	var confirmed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{"mode":"normal"}`), nil, nil, 1, 1, ProcessCallbacks{
		OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { confirmed.Add(1) },
	}); err != nil {
		t.Fatal(err)
	}
	code, err := host.Wait()
	if err != nil || code != 0 || confirmed.Load() != 1 {
		t.Fatalf("Node runtime failed under the Linux pids budget: code=%d err=%v confirmed=%d", code, err, confirmed.Load())
	}
}

func TestRound27InheritedStderrCannotHangOrConfirmTask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows suspended-job execution requires separate isolation tests")
	}
	host := fixtureProcessHost(t)
	fixture := strings.Replace(processFixture,
		"send('task.finished',{task_run_id:id,status:'succeeded'",
		"const child=require('node:child_process').spawn(process.execPath,['-e','setTimeout(()=>{}, 6500)'],{stdio:['ignore','ignore','inherit']});child.unref();send('task.finished',{task_run_id:id,status:'succeeded'", 1)
	if err := os.WriteFile(host.config.HostPath, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 9*time.Second)
	defer cancel()
	var confirmed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{"mode":"normal"}`), nil, nil, 1, 1, ProcessCallbacks{
		OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { confirmed.Add(1) },
	}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err := host.Wait()
	if !errors.Is(err, exec.ErrWaitDelay) || confirmed.Load() != 0 || time.Since(started) > 6*time.Second {
		t.Fatalf("inherited worker descriptor prevented bounded failure: err=%v confirmed=%d elapsed=%v", err, confirmed.Load(), time.Since(started))
	}
}
