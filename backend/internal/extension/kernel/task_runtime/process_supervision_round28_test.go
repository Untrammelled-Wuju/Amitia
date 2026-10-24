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
)

func TestRound28InheritedStdoutIsBoundedAndNotAcknowledged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows job-object suspension requires separate coverage")
	}
	host := fixtureProcessHost(t)
	fixture := strings.Replace(processFixture,
		"send('task.finished',{task_run_id:id,status:'succeeded'",
		"const child=require('node:child_process').spawn(process.execPath,['-e','setTimeout(()=>{}, 7000)'],{stdio:['ignore','inherit','ignore']});child.unref();send('task.finished',{task_run_id:id,status:'succeeded'", 1)
	if err := os.WriteFile(host.config.HostPath, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		t.Fatalf("stdout descendant must cause bounded failure, not success: err=%v confirmed=%d elapsed=%s", err, confirmed.Load(), time.Since(started))
	}
}
