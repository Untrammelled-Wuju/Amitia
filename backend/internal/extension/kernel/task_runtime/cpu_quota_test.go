//go:build rsi_cpuquota

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"testing"
)

type fakeCPUQuotaFS struct {
	mu        sync.Mutex
	files     map[string]string
	writes    int
	writeErr  error
	readErr   error
	readValue string
}

func (f *fakeCPUQuotaFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	if f.files == nil {
		f.files = map[string]string{}
	}
	f.files[path] = string(data)
	f.writes++
	return nil
}

func (f *fakeCPUQuotaFS) ReadFile(path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readErr != nil {
		return nil, f.readErr
	}
	if f.readValue != "" {
		return []byte(f.readValue), nil
	}
	return []byte(f.files[path]), nil
}

func TestCPUQuotaPlanSingleCore(t *testing.T) {
	plan, err := PlanCPUQuota(50)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.CgroupValue() != "50000 100000" || plan.Cores() != 0.5 {
		t.Fatalf("plan=%+v value=%s", plan, plan.CgroupValue())
	}
	t.Logf("m07-s09-detail plan=50 cores=0.5")
	fmt.Printf("m07-s09 plan percent=50 quota=50000 period=100000\n")
}

func TestCPUQuotaPlanMultiCore(t *testing.T) {
	plan, err := PlanCPUQuota(250)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.CgroupValue() != "250000 100000" || plan.Cores() != 2.5 {
		t.Fatalf("plan=%+v", plan)
	}
	fmt.Printf("m07-s09 plan percent=250 quota=250000 multicore=true\n")
}

func TestCPUQuotaPlanMax(t *testing.T) {
	plan, err := PlanCPUQuota(MaxCPUQuotaPercent)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.CgroupValue() != "800000 100000" {
		t.Fatalf("plan=%+v", plan)
	}
	fmt.Printf("m07-s09 plan percent=800 quota=800000 max=true\n")
}

func TestCPUQuotaRejectsInvalid(t *testing.T) {
	cases := []float64{0, -1, 801, MaxCPUQuotaPercent + 0.1, -0.5}
	for _, value := range cases {
		if _, err := PlanCPUQuota(value); !errors.Is(err, ErrCPUQuotaInvalid) {
			t.Fatalf("value=%v err=%v", value, err)
		}
	}
	fmt.Printf("m07-s09 reject invalid=5\n")
}

func TestCPUQuotaApplyVerified(t *testing.T) {
	fsys := &fakeCPUQuotaFS{}
	plan, err := ApplyCPUQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 75)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if plan.CgroupValue() != "75000 100000" || fsys.files["/sys/fs/cgroup/plugin/cpu.max"] != "75000 100000" {
		t.Fatalf("plan=%+v files=%v", plan, fsys.files)
	}
	fmt.Printf("m07-s09 apply verified=true path=cpu.max\n")
}

func TestCPUQuotaMismatchRejected(t *testing.T) {
	fsys := &fakeCPUQuotaFS{readValue: "max 100000"}
	_, err := ApplyCPUQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 50)
	if !errors.Is(err, ErrCPUQuotaNotApplied) {
		t.Fatalf("mismatch err=%v", err)
	}
	fmt.Printf("m07-s09 mismatch rejected=true\n")
}

func TestCPUQuotaWriteErrorPropagates(t *testing.T) {
	sentinel := errors.New("read-only cgroup")
	fsys := &fakeCPUQuotaFS{writeErr: sentinel}
	_, err := ApplyCPUQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 50)
	if !errors.Is(err, sentinel) {
		t.Fatalf("write err=%v", err)
	}
	fmt.Printf("m07-s09 write error_propagated=true\n")
}

func TestCPUQuotaReadErrorPropagates(t *testing.T) {
	sentinel := errors.New("read failed")
	fsys := &fakeCPUQuotaFS{readErr: sentinel}
	_, err := ApplyCPUQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 50)
	if !errors.Is(err, sentinel) {
		t.Fatalf("read err=%v", err)
	}
	fmt.Printf("m07-s09 read error_propagated=true\n")
}

func TestCPUQuotaCancelBeforeWrite(t *testing.T) {
	fsys := &fakeCPUQuotaFS{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ApplyCPUQuota(ctx, fsys, "/sys/fs/cgroup/plugin", 50)
	if !errors.Is(err, context.Canceled) || fsys.writes != 0 {
		t.Fatalf("cancel err=%v writes=%d", err, fsys.writes)
	}
	fmt.Printf("m07-s09 cancel rejected=true writes=0\n")
}

func TestCPUQuotaRounding(t *testing.T) {
	plan, err := PlanCPUQuota(33.3)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.QuotaMicros != 33300 {
		t.Fatalf("quota=%d", plan.QuotaMicros)
	}
	fmt.Printf("m07-s09 rounding percent=33.3 quota=33300\n")
}

func TestCPUQuotaOSFilesystem(t *testing.T) {
	directory := t.TempDir()
	plan, err := ApplyCPUQuota(context.Background(), OSCPUQuotaFS{}, directory, 125)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	data, err := osReadFile(directory)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if plan.CgroupValue() != data {
		t.Fatalf("plan=%s readback=%s", plan.CgroupValue(), data)
	}
	fmt.Printf("m07-s09 osfs verified=true\n")
}

func osReadFile(directory string) (string, error) {
	data, err := OSCPUQuotaFS{}.ReadFile(directory + "/cpu.max")
	return string(data), err
}

func TestCPUQuotaNilFilesystemRejected(t *testing.T) {
	if _, err := ApplyCPUQuota(context.Background(), nil, "/sys/fs/cgroup/plugin", 50); !errors.Is(err, ErrCPUQuotaInvalid) {
		t.Fatalf("nil err=%v", err)
	}
	fmt.Printf("m07-s09 nil rejected=true\n")
}
