//go:build rsi_memquota

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"testing"
)

type fakeMemoryQuotaFS struct {
	mu       sync.Mutex
	files    map[string]string
	writes   int
	writeErr  error
	readErr   error
	readValue string
}

func (f *fakeMemoryQuotaFS) WriteFile(path string, data []byte, perm fs.FileMode) error {
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

func (f *fakeMemoryQuotaFS) ReadFile(path string) ([]byte, error) {
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

func TestMemoryQuotaPlanBounded(t *testing.T) {
	plan, err := PlanMemoryQuota(512 << 20)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.CgroupValue() != "536870912" || plan.Bytes != 512<<20 {
		t.Fatalf("plan=%+v", plan)
	}
	fmt.Printf("m07-s10 plan bytes=536870912 value=536870912\n")
}

func TestMemoryQuotaPlanUnlimited(t *testing.T) {
	plan, err := PlanMemoryQuota(0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.Unlimited || plan.CgroupValue() != "max" {
		t.Fatalf("plan=%+v value=%s", plan, plan.CgroupValue())
	}
	fmt.Printf("m07-s10 plan unlimited=true value=max\n")
}

func TestMemoryQuotaRejectsInvalid(t *testing.T) {
	cases := []int64{-1, MinMemoryQuotaBytes - 1, MaxMemoryQuotaBytes + 1, 1 << 40}
	for _, value := range cases {
		if _, err := PlanMemoryQuota(value); !errors.Is(err, ErrMemoryQuotaInvalid) {
			t.Fatalf("value=%d err=%v", value, err)
		}
	}
	fmt.Printf("m07-s10 reject invalid=4\n")
}

func TestMemoryQuotaApplyVerified(t *testing.T) {
	fsys := &fakeMemoryQuotaFS{}
	plan, err := ApplyMemoryQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 256<<20)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if plan.CgroupValue() != "268435456" || fsys.files["/sys/fs/cgroup/plugin/memory.max"] != "268435456" {
		t.Fatalf("plan=%+v files=%v", plan, fsys.files)
	}
	fmt.Printf("m07-s10 apply verified=true path=memory.max\n")
}

func TestMemoryQuotaMismatchRejected(t *testing.T) {
	fsys := &fakeMemoryQuotaFS{readValue: "max"}
	_, err := ApplyMemoryQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 256<<20)
	if !errors.Is(err, ErrMemoryQuotaNotApplied) {
		t.Fatalf("mismatch err=%v", err)
	}
	fmt.Printf("m07-s10 mismatch rejected=true\n")
}

func TestMemoryQuotaWriteErrorPropagates(t *testing.T) {
	sentinel := errors.New("cgroup read-only")
	fsys := &fakeMemoryQuotaFS{writeErr: sentinel}
	_, err := ApplyMemoryQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 256<<20)
	if !errors.Is(err, sentinel) {
		t.Fatalf("write err=%v", err)
	}
	fmt.Printf("m07-s10 write error_propagated=true\n")
}

func TestMemoryQuotaReadErrorPropagates(t *testing.T) {
	sentinel := errors.New("read failed")
	fsys := &fakeMemoryQuotaFS{readErr: sentinel}
	_, err := ApplyMemoryQuota(context.Background(), fsys, "/sys/fs/cgroup/plugin", 256<<20)
	if !errors.Is(err, sentinel) {
		t.Fatalf("read err=%v", err)
	}
	fmt.Printf("m07-s10 read error_propagated=true\n")
}

func TestMemoryQuotaCancelBeforeWrite(t *testing.T) {
	fsys := &fakeMemoryQuotaFS{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ApplyMemoryQuota(ctx, fsys, "/sys/fs/cgroup/plugin", 256<<20)
	if !errors.Is(err, context.Canceled) || fsys.writes != 0 {
		t.Fatalf("cancel err=%v writes=%d", err, fsys.writes)
	}
	fmt.Printf("m07-s10 cancel rejected=true writes=0\n")
}

func TestMemoryQuotaOSFilesystem(t *testing.T) {
	directory := t.TempDir()
	plan, err := ApplyMemoryQuota(context.Background(), OSMemoryQuotaFS{}, directory, 64<<20)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	data, err := OSMemoryQuotaFS{}.ReadFile(directory + "/memory.max")
	if err != nil || string(data) != plan.CgroupValue() {
		t.Fatalf("readback=%q err=%v", string(data), err)
	}
	fmt.Printf("m07-s10 osfs verified=true\n")
}

func TestMemoryQuotaNilFilesystemRejected(t *testing.T) {
	if _, err := ApplyMemoryQuota(context.Background(), nil, "/sys/fs/cgroup/plugin", 256<<20); !errors.Is(err, ErrMemoryQuotaInvalid) {
		t.Fatalf("nil err=%v", err)
	}
	if _, err := ReadOOMKills(context.Background(), nil, "/sys/fs/cgroup/plugin"); !errors.Is(err, ErrMemoryQuotaInvalid) {
		t.Fatalf("oom nil err=%v", err)
	}
	fmt.Printf("m07-s10 nil rejected=true\n")
}

func TestMemoryQuotaEmptyDirectoryRejected(t *testing.T) {
	if _, err := ApplyMemoryQuota(context.Background(), &fakeMemoryQuotaFS{}, "  ", 256<<20); !errors.Is(err, ErrMemoryQuotaInvalid) {
		t.Fatalf("empty err=%v", err)
	}
	fmt.Printf("m07-s10 empty_dir rejected=true\n")
}

func TestMemoryQuotaOOMKills(t *testing.T) {
	fsys := &fakeMemoryQuotaFS{files: map[string]string{"/sys/fs/cgroup/plugin/memory.events": "low 0\nhigh 2\nmax 4\noom 1\noom_kill 7\n"}}
	kills, err := ReadOOMKills(context.Background(), fsys, "/sys/fs/cgroup/plugin")
	if err != nil || kills != 7 {
		t.Fatalf("kills=%d err=%v", kills, err)
	}
	fmt.Printf("m07-s10 oom kills=7\n")
}

func TestMemoryQuotaOOMMissing(t *testing.T) {
	fsys := &fakeMemoryQuotaFS{files: map[string]string{"/sys/fs/cgroup/plugin/memory.events": "low 0\nhigh 2\n"}}
	kills, err := ReadOOMKills(context.Background(), fsys, "/sys/fs/cgroup/plugin")
	if err != nil || kills != 0 {
		t.Fatalf("kills=%d err=%v", kills, err)
	}
	fmt.Printf("m07-s10 oom missing=0\n")
}
