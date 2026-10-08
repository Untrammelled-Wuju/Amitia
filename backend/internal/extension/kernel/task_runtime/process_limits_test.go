package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
)

func TestTaskProcessLimitsCannotBeRaisedByPluginDeclarations(t *testing.T) {
	support := process.ResourceLimitsSupported()
	for _, definition := range []*TaskDefinition{nil, {ResourceLimits: TaskResourceLimits{MaxMemoryMB: 8192, MaxCPUPercent: 100}}, {ResourceLimits: TaskResourceLimits{MaxMemoryMB: -1, MaxCPUPercent: -1}}} {
		limits := taskProcessLimits(definition)
		if support.Memory && limits.MaxMemoryBytes != 512<<20 || support.CPU && limits.MaxCPUPercent != 50 || support.Processes && limits.MaxProcesses != 1 {
			t.Fatalf("plugin raised host policy: %+v", limits)
		}
		if !support.Memory && limits.MaxMemoryBytes != 0 || !support.CPU && limits.MaxCPUPercent != 0 || !support.Processes && limits.MaxProcesses != 0 {
			t.Fatalf("unsupported limits advertised: %+v", limits)
		}
	}
	limits := taskProcessLimits(&TaskDefinition{ResourceLimits: TaskResourceLimits{MaxMemoryMB: 256, MaxCPUPercent: 25}})
	if support.Memory && limits.MaxMemoryBytes != 256<<20 || support.CPU && limits.MaxCPUPercent != 25 {
		t.Fatalf("stricter plugin limits ignored: %+v", limits)
	}
}

func TestTaskProcessNativeMemoryLimitStopsActualNodeBeforeConfirmingSuccess(t *testing.T) {
	if !process.ResourceLimitsSupported().Memory {
		t.Skip("当前平台没有可验证的原生内存限制")
	}
	for _, oversized := range []bool{false, true} {
		t.Run(fmt.Sprintf("oversized_%t", oversized), func(t *testing.T) {
			host := fixtureProcessHost(t)
			host.config.NativeLimits = process.ResourceLimits{MaxMemoryBytes: 192 << 20}
			body := strings.Replace(processFixture, "const id=p.input", "const retained=[]; for(let index=0;index<p.input.allocations;index++)retained.push(Buffer.alloc(16*1024*1024,255));globalThis.retained=retained;const id=p.input", 1)
			if err := os.WriteFile(host.config.HostPath, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			allocations := 1
			if oversized {
				allocations = 64
			}
			input, _ := json.Marshal(map[string]any{"mode": "normal", "allocations": allocations})
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			var completed atomic.Int32
			if err := host.Start(ctx, input, nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) }}); err != nil {
				t.Fatal(err)
			}
			code, err := host.Wait()
			if ctx.Err() != nil {
				t.Fatal("memory enforcement was not confirmed before timeout")
			}
			if oversized {
				if err == nil || code == 0 || completed.Load() != 0 {
					t.Fatalf("native limit did not terminate the actual allocator: code=%d err=%v callbacks=%d", code, err, completed.Load())
				}
			} else if err != nil || code != 0 || completed.Load() != 1 {
				t.Fatalf("bounded allocator could not complete: code=%d err=%v callbacks=%d", code, err, completed.Load())
			}
		})
	}
}

func TestTaskProcessNativeCountLimitBlocksChildProcessesBeforeTheyExecute(t *testing.T) {
	if !process.ResourceLimitsSupported().Processes {
		t.Skip("当前平台没有可验证的原生进程数量限制")
	}
	host := fixtureProcessHost(t)
	host.config.NativeLimits = process.ResourceLimits{MaxProcesses: 1}
	body := strings.Replace(processFixture, "const id=p.input", `const child=require('node:child_process').spawnSync(process.execPath,['-e','process.stdout.write("escaped")']);if(!child.error)throw new Error('child escaped process policy');const id=p.input`, 1)
	if err := os.WriteFile(host.config.HostPath, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var completed atomic.Int32
	if err := host.Start(ctx, json.RawMessage(`{"mode":"normal"}`), nil, nil, 1, 1, ProcessCallbacks{OnFinished: func(_ string, _ json.RawMessage, _, _, _ string) { completed.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	code, err := host.Wait()
	if err != nil || code != 0 || completed.Load() != 1 {
		t.Fatalf("child process escaped native count policy or parent failed: code=%d err=%v callbacks=%d", code, err, completed.Load())
	}
}
