//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"testing"
	"time"
)

type baselineSample struct {
	Plugins    int
	Duration   time.Duration
	HeapBytes  uint64
	Processes  int
	Workers    int
	SpawnCalls int
	Goroutines int
}

func baselineLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s07 "+format, args...)
}

func baselineDetail(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s07-detail "+format, args...)
}

func baselineManifests(count int, prefix string) []Manifest {
	manifests := make([]Manifest, 0, count)
	for index := 0; index < count; index++ {
		manifests = append(manifests, Manifest{
			ID:          fmt.Sprintf("%s-%04d", prefix, index),
			Version:     "1.0.0",
			Permissions: []string{"memory.read"},
			Entry:       fmt.Sprintf("plugins/%s/%04d.js", prefix, index),
		})
	}
	return manifests
}

func takeHeap() uint64 {
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

func measureInstall(t *testing.T, count int) baselineSample {
	t.Helper()
	gcBefore := runtime.NumGoroutine()
	heapBefore := takeHeap()
	spawner := &leakSpawner{}
	installer := NewInstaller(spawner)
	started := time.Now()
	if _, err := installer.InstallBatch(context.Background(), baselineManifests(count, "base")); err != nil {
		t.Fatalf("install %d: %v", count, err)
	}
	elapsed := time.Since(started)
	heapAfter := takeHeap()
	delta := int64(heapAfter) - int64(heapBefore)
	if delta < 0 {
		delta = 0
	}
	snapshot := installer.Snapshot()
	sample := baselineSample{
		Plugins:    snapshot.Plugins,
		Duration:   elapsed,
		HeapBytes:  uint64(delta),
		Processes:  snapshot.Processes,
		Workers:    snapshot.Workers,
		SpawnCalls: snapshot.SpawnCalls,
		Goroutines: runtime.NumGoroutine() - gcBefore,
	}
	if installer.Snapshot().Plugins != count {
		t.Fatalf("installed=%d want %d", installer.Snapshot().Plugins, count)
	}
	return sample
}

func percentileDuration(samples []time.Duration, ratio float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]time.Duration{}, samples...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	position := int(float64(len(sorted))*ratio+0.999) - 1
	if position < 0 {
		position = 0
	}
	if position >= len(sorted) {
		position = len(sorted) - 1
	}
	return sorted[position]
}

func TestPluginBaselineZeroInstalled(t *testing.T) {
	spawner := &leakSpawner{}
	installer := NewInstaller(spawner)
	heapBefore := takeHeap()
	installer.InstallBatch(context.Background(), nil)
	heapAfter := takeHeap()
	delta := int64(heapAfter) - int64(heapBefore)
	if delta < 0 {
		delta = 0
	}
	snapshot := installer.Snapshot()
	if snapshot.Plugins != 0 || snapshot.Processes != 0 || snapshot.Workers != 0 || snapshot.SpawnCalls != 0 {
		t.Fatalf("empty installer snapshot=%+v", snapshot)
	}
	baselineDetail(t, "zero heap_delta=%d goroutines=%d", int64(heapAfter)-int64(heapBefore), runtime.NumGoroutine())
	baselineLog(t, "zero plugins=0 processes=0 workers=0 spawn_calls=0")
}

func TestPluginBaselineThousand(t *testing.T) {
	durations := make([]time.Duration, 0, 5)
	var heapPerPlugin uint64
	for run := 0; run < 5; run++ {
		sample := measureInstall(t, 1000)
		durations = append(durations, sample.Duration)
		heapPerPlugin = sample.HeapBytes / 1000
		if sample.Processes != 0 || sample.Workers != 0 || sample.SpawnCalls != 0 {
			t.Fatalf("install created runtimes: %+v", sample)
		}
		if sample.Goroutines > 2 {
			t.Fatalf("install leaked goroutines: %d", sample.Goroutines)
		}
	}
	if heapPerPlugin == 0 || heapPerPlugin > 64*1024 {
		t.Fatalf("heap per plugin=%d bytes outside the 1..65536 bound", heapPerPlugin)
	}
	baselineDetail(t, "thousand p50_us=%d p95_us=%d heap_per_plugin=%d",
		percentileDuration(durations, 0.50).Microseconds(),
		percentileDuration(durations, 0.95).Microseconds(),
		heapPerPlugin)
	baselineLog(t, "thousand installed=1000 processes=0 workers=0 spawn_calls=0")
}

func TestPluginBaselineTwoThousand(t *testing.T) {
	first := measureInstall(t, 1000)
	second := measureInstall(t, 2000)
	firstPerPlugin := float64(first.HeapBytes) / 1000
	secondPerPlugin := float64(second.HeapBytes) / 2000
	ratio := 0.0
	if firstPerPlugin > 0 {
		ratio = secondPerPlugin / firstPerPlugin
	}
	if ratio > 2.5 {
		t.Fatalf("per-plugin heap ratio=%.2f (1000=%.0f 2000=%.0f)", ratio, firstPerPlugin, secondPerPlugin)
	}
	timeRatio := float64(second.Duration) / float64(first.Duration)
	if timeRatio > 4 {
		t.Fatalf("install time ratio=%.2f (1000=%s 2000=%s)", timeRatio, first.Duration, second.Duration)
	}
	if second.Processes != 0 || second.SpawnCalls != 0 {
		t.Fatalf("2000 install created runtimes: %+v", second)
	}
	baselineDetail(t, "scaling heap_ratio=%.2f time_ratio=%.2f", ratio, timeRatio)
	baselineLog(t, "scaling linear=true per_plugin_ratio<=2.5 time_ratio<=4")
	baselineLog(t, "two_thousand installed=2000 processes=0 workers=0 spawn_calls=0")
}

func TestPluginBaselineRealPluginCounts(t *testing.T) {
	processes := 0
	for _, count := range []int{1, 3, 5, 10} {
		spawner := &leakSpawner{}
		installer := NewInstaller(spawner)
		if _, err := installer.InstallBatch(context.Background(), baselineManifests(count, "real")); err != nil {
			t.Fatalf("install %d: %v", count, err)
		}
		for index := 0; index < count; index++ {
			id := fmt.Sprintf("real-%04d", index)
			if err := installer.Enable(context.Background(), id, 1); err != nil {
				t.Fatalf("enable %s: %v", id, err)
			}
		}
		snapshot := installer.Snapshot()
		if snapshot.Plugins != count || snapshot.Processes != 0 {
			t.Fatalf("count %d snapshot=%+v", count, snapshot)
		}
		processes += snapshot.Processes
	}
	if processes != 0 {
		t.Fatalf("real-plugin installs created %d runtimes", processes)
	}
	baselineLog(t, "real_plugins counts=1,3,5,10 processes=0")
}

func TestPluginBaselineEnableStaysCold(t *testing.T) {
	spawner := &leakSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(context.Background(), baselineManifests(1000, "enable")); err != nil {
		t.Fatalf("install: %v", err)
	}
	started := time.Now()
	for index := 0; index < 1000; index++ {
		if err := installer.Enable(context.Background(), fmt.Sprintf("enable-%04d", index), 1); err != nil {
			t.Fatalf("enable %d: %v", index, err)
		}
	}
	elapsed := time.Since(started)
	snapshot := installer.Snapshot()
	if snapshot.Processes != 0 || snapshot.Workers != 0 || spawner.Starts() != 0 {
		t.Fatalf("enable created runtimes: %+v", snapshot)
	}
	baselineDetail(t, "enable_1000 duration_us=%d", elapsed.Microseconds())
	baselineLog(t, "enable plugins=1000 processes=0")
}

func TestPluginBaselineActivationIsExplicit(t *testing.T) {
	spawner := &leakSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(context.Background(), baselineManifests(1000, "activate")); err != nil {
		t.Fatalf("install: %v", err)
	}
	for index := 0; index < 1000; index++ {
		if err := installer.Enable(context.Background(), fmt.Sprintf("activate-%04d", index), 1); err != nil {
			t.Fatalf("enable %d: %v", index, err)
		}
	}
	for index := 0; index < 5; index++ {
		if _, err := installer.Execute(context.Background(), fmt.Sprintf("activate-%04d", index), ExecuteRequest{ID: fmt.Sprintf("req-%d", index)}); err != nil {
			t.Fatalf("execute %d: %v", index, err)
		}
	}
	snapshot := installer.Snapshot()
	if snapshot.Processes != 5 || spawner.Starts() != 5 {
		t.Fatalf("processes=%d starts=%d want 5/5", snapshot.Processes, spawner.Starts())
	}
	baselineLog(t, "activate instances=5 spawn_calls=5")
}

func TestPluginBaselineIdempotentInstall(t *testing.T) {
	spawner := &leakSpawner{}
	installer := NewInstaller(spawner)
	manifests := baselineManifests(1000, "repeat")
	if _, err := installer.InstallBatch(context.Background(), manifests); err != nil {
		t.Fatalf("first install: %v", err)
	}
	heapBefore := takeHeap()
	report, err := installer.InstallBatch(context.Background(), manifests)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	heapAfter := takeHeap()
	if report.Installed != 0 || report.Skipped != 1000 {
		t.Fatalf("report=%+v", report)
	}
	if installer.Snapshot().Plugins != 1000 || len(installer.handles) != 0 {
		t.Fatalf("repeat install changed state")
	}
	baselineDetail(t, "repeat heap_delta=%d", int64(heapAfter)-int64(heapBefore))
	baselineLog(t, "idempotent skipped=1000 handles=0")
}

func TestPluginBaselineCancelKeepsZero(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	spawner := &leakSpawner{}
	installer := NewInstaller(spawner)
	if _, err := installer.InstallBatch(ctx, baselineManifests(1000, "cancel")); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if installer.Snapshot().Plugins != 0 {
		t.Fatalf("cancelled install left %d entries", installer.Snapshot().Plugins)
	}
	baselineLog(t, "cancel entries=0 err=context.Canceled")
}

func TestPluginBaselineMachineInfo(t *testing.T) {
	if runtime.GOOS == "" || runtime.GOARCH == "" || runtime.NumCPU() < 1 {
		t.Fatal("machine info is incomplete")
	}
	baselineDetail(t, "machine goos=%s goarch=%s cpus=%d go_version=%s", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version())
	baselineLog(t, "machine reported=true")
}
