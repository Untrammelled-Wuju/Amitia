//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	hotCategoryAssistant = "builtin.assistant"
	hotCategoryWorker    = "builtin.worker"
)

func hotPolicy() HotPolicy {
	return HotPolicy{
		AllowedCategories: map[string]bool{hotCategoryAssistant: true, hotCategoryWorker: true},
		MinMemoryBytes:    64 * 1024 * 1024,
		MinConcurrent:     1,
		MaxHot:            2,
	}
}

func hotProfile() HotProfile {
	return HotProfile{MaxMemoryBytes: 128 * 1024 * 1024, MaxConcurrent: 2, Source: ProfileSourceHost}
}

type hotFixture struct {
	installer *Installer
	reaper    *IdleReaper
	residency *HotResidency
	clock     *time.Time
}

func newHotFixture(t *testing.T, policy HotPolicy) *hotFixture {
	t.Helper()
	supervisor, _, instanceID := idleDrainer(t)
	spawner := &executeSpawner{}
	installer := NewInstaller(spawner)
	clock := time.Date(2026, 11, 25, 13, 0, 0, 0, time.UTC)
	reaper, err := NewIdleReaperWithClock(installer, IdlePolicy{TTL: time.Minute, DrainTimeout: time.Second}, supervisor, func(string) (string, bool) {
		return instanceID, true
	}, func() time.Time { return clock })
	if err != nil {
		t.Fatalf("NewIdleReaperWithClock: %v", err)
	}
	residency, err := NewHotResidency(installer, policy)
	if err != nil {
		t.Fatalf("NewHotResidency: %v", err)
	}
	installer.WithActivity(reaper.Observe)
	reaper.WithHotCheck(residency.IsHot)
	if _, err := installer.InstallBatch(context.Background(), metadataManifests(8)); err != nil {
		t.Fatalf("install: %v", err)
	}
	for index := 0; index < 8; index++ {
		id := fmt.Sprintf("plugin-%04d", index)
		if err := installer.Enable(context.Background(), id, 1); err != nil {
			t.Fatalf("enable %s: %v", id, err)
		}
	}
	return &hotFixture{installer: installer, reaper: reaper, residency: residency, clock: &clock}
}

func (f *hotFixture) activate(t *testing.T, id string) {
	t.Helper()
	if _, err := f.installer.Execute(context.Background(), id, ExecuteRequest{ID: "activate-" + id}); err != nil {
		t.Fatalf("execute %s: %v", id, err)
	}
}

func (f *hotFixture) advance(duration time.Duration) {
	*f.clock = f.clock.Add(duration)
}

func hotLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s05 "+format, args...)
}

func hotRequest(id string) HotRequest {
	return HotRequest{PluginID: id, Category: hotCategoryAssistant, Granted: true, Profile: hotProfile()}
}

func TestPluginHotAuthorizedCategoryResides(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	fixture.advance(time.Minute + time.Second)
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(report.Released) != 0 {
		t.Fatalf("hot plugin was released: %v", report.Released)
	}
	if !fixture.residency.IsHot("plugin-0000") || fixture.installer.Snapshot().Processes != 1 {
		t.Fatalf("hot plugin lost its runtime")
	}
	hotLog(t, "resident authorized=true released=0 processes=1")
}

func TestPluginHotRequiresUserGrant(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	request := hotRequest("plugin-0000")
	request.Granted = false
	err := fixture.residency.Acquire(ctx, request)
	if !errors.Is(err, ErrHotDenied) {
		t.Fatalf("err=%v want ErrHotDenied", err)
	}
	reason, ok := fixture.residency.Denial("plugin-0000")
	if !ok || reason != "authorization_missing" {
		t.Fatalf("denial=%q ok=%v", reason, ok)
	}
	if fixture.residency.IsHot("plugin-0000") {
		t.Fatal("unauthorized plugin became hot")
	}
	hotLog(t, "denied reason=authorization_missing hot=false")
}

func TestPluginHotRejectsPluginDeclaredProfile(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	request := hotRequest("plugin-0000")
	request.Profile.Source = ProfileSourcePlugin
	err := fixture.residency.Acquire(ctx, request)
	if !errors.Is(err, ErrHotDenied) {
		t.Fatalf("err=%v want ErrHotDenied", err)
	}
	reason, _ := fixture.residency.Denial("plugin-0000")
	if reason != "profile_source_not_host" {
		t.Fatalf("reason=%q", reason)
	}
	if fixture.residency.IsHot("plugin-0000") {
		t.Fatal("plugin-declared profile was accepted")
	}
	hotLog(t, "denied reason=profile_source_not_host")
}

func TestPluginHotRejectsProfileBelowFloor(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	small := hotRequest("plugin-0000")
	small.Profile.MaxMemoryBytes = 1024
	if err := fixture.residency.Acquire(ctx, small); !errors.Is(err, ErrHotDenied) {
		t.Fatalf("memory err=%v", err)
	}
	if reason, _ := fixture.residency.Denial("plugin-0000"); reason != "profile_memory_below_floor" {
		t.Fatalf("reason=%q", reason)
	}
	thin := hotRequest("plugin-0000")
	thin.Profile.MaxConcurrent = 0
	if err := fixture.residency.Acquire(ctx, thin); !errors.Is(err, ErrHotDenied) {
		t.Fatalf("concurrency err=%v", err)
	}
	if reason, _ := fixture.residency.Denial("plugin-0000"); reason != "profile_concurrency_below_floor" {
		t.Fatalf("reason=%q", reason)
	}
	hotLog(t, "denied profile floors=2")
}

func TestPluginHotRejectsUnknownCategory(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	request := hotRequest("plugin-0000")
	request.Category = "plugin.selfdeclared"
	if err := fixture.residency.Acquire(ctx, request); !errors.Is(err, ErrHotDenied) {
		t.Fatalf("err=%v", err)
	}
	if reason, _ := fixture.residency.Denial("plugin-0000"); reason != "category_not_allowed" {
		t.Fatalf("reason=%q", reason)
	}
	hotLog(t, "denied reason=category_not_allowed")
}

func TestPluginHotInvalidPolicyAndRequest(t *testing.T) {
	ctx := context.Background()
	installer := NewInstaller(&executeSpawner{})
	if _, err := NewHotResidency(installer, HotPolicy{}); !errors.Is(err, ErrHotInvalid) {
		t.Fatalf("empty allowlist err=%v", err)
	}
	if _, err := NewHotResidency(nil, hotPolicy()); !errors.Is(err, ErrHotInvalid) {
		t.Fatalf("nil installer err=%v", err)
	}
	residency, err := NewHotResidency(installer, hotPolicy())
	if err != nil {
		t.Fatalf("NewHotResidency: %v", err)
	}
	if err := residency.Acquire(ctx, HotRequest{}); !errors.Is(err, ErrHotInvalid) {
		t.Fatalf("empty request err=%v", err)
	}
	hotLog(t, "invalid policy=ErrHotInvalid request=ErrHotInvalid")
}

func TestPluginHotRequiresActivatedRuntime(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); !errors.Is(err, ErrHotInvalid) {
		t.Fatalf("installed-only err=%v", err)
	}
	if fixture.residency.IsHot("plugin-0000") {
		t.Fatal("plugin without a runtime became hot")
	}
	fixture.activate(t, "plugin-0000")
	if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); err != nil {
		t.Fatalf("after activation: %v", err)
	}
	hotLog(t, "requires_runtime=true")
}

func TestPluginHotIdempotentAndCategoryConflict(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if fixture.residency.Len() != 1 {
		t.Fatalf("len=%d want 1", fixture.residency.Len())
	}
	other := hotRequest("plugin-0000")
	other.Category = hotCategoryWorker
	if err := fixture.residency.Acquire(ctx, other); !errors.Is(err, ErrHotInvalid) {
		t.Fatalf("category conflict err=%v", err)
	}
	hotLog(t, "idempotent=true conflict=ErrHotInvalid")
}

func TestPluginHotCapacityLimitAndRelease(t *testing.T) {
	ctx := context.Background()
	policy := hotPolicy()
	policy.MaxHot = 1
	fixture := newHotFixture(t, policy)
	fixture.activate(t, "plugin-0000")
	fixture.activate(t, "plugin-0001")
	if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := hotRequest("plugin-0001")
	if err := fixture.residency.Acquire(ctx, second); !errors.Is(err, ErrHotDenied) {
		t.Fatalf("capacity err=%v", err)
	}
	if reason, _ := fixture.residency.Denial("plugin-0001"); reason != "capacity_exhausted" {
		t.Fatalf("reason=%q", reason)
	}
	fixture.residency.Release("plugin-0000")
	if err := fixture.residency.Acquire(ctx, second); err != nil {
		t.Fatalf("after release: %v", err)
	}
	if fixture.residency.Len() != 1 || !fixture.residency.IsHot("plugin-0001") {
		t.Fatalf("len=%d", fixture.residency.Len())
	}
	hotLog(t, "capacity max=1 denied=capacity_exhausted release_frees=true")
}

func TestPluginHotReleaseAllowsIdleReclaim(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	if err := fixture.residency.Acquire(ctx, hotRequest("plugin-0000")); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	fixture.advance(time.Minute + time.Second)
	if report, err := fixture.reaper.Sweep(ctx); err != nil || len(report.Released) != 0 {
		t.Fatalf("sweep while hot: report=%+v err=%v", report, err)
	}
	fixture.residency.Release("plugin-0000")
	report, err := fixture.reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep after release: %v", err)
	}
	if len(report.Released) != 1 || report.Released[0] != "plugin-0000" {
		t.Fatalf("released=%v", report.Released)
	}
	if fixture.installer.Snapshot().Processes != 0 {
		t.Fatalf("runtime survived release")
	}
	hotLog(t, "release reclaims=true released=1")
}

func TestPluginHotCancelAndConcurrency(t *testing.T) {
	ctx := context.Background()
	policy := hotPolicy()
	policy.MaxHot = 1
	fixture := newHotFixture(t, policy)
	for index := 0; index < 8; index++ {
		fixture.activate(t, fmt.Sprintf("plugin-%04d", index))
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := fixture.residency.Acquire(cancelled, hotRequest("plugin-0000")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
	if fixture.residency.Len() != 0 {
		t.Fatalf("cancelled acquire registered a lease")
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for index := 0; index < 8; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			id := fmt.Sprintf("plugin-%04d", index)
			if err := fixture.residency.Acquire(ctx, hotRequest(id)); err == nil {
				wins.Add(1)
			}
		}(index)
	}
	wg.Wait()
	if wins.Load() != 1 || fixture.residency.Len() != 1 {
		t.Fatalf("wins=%d len=%d want 1/1", wins.Load(), fixture.residency.Len())
	}
	hotLog(t, "cancel=context.Canceled concurrent_wins=1")
}

func TestPluginHotDenialKeepsErrorChain(t *testing.T) {
	ctx := context.Background()
	fixture := newHotFixture(t, hotPolicy())
	fixture.activate(t, "plugin-0000")
	request := hotRequest("plugin-0000")
	request.Category = "plugin.selfdeclared"
	err := fixture.residency.Acquire(ctx, request)
	if !strings.Contains(err.Error(), "category_not_allowed") {
		t.Fatalf("error does not expose the reason: %v", err)
	}
	if !errors.Is(err, ErrHotDenied) {
		t.Fatalf("error chain lost ErrHotDenied: %v", err)
	}
	hotLog(t, "reason_visible=true chain=true")
}
