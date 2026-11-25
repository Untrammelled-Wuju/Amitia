package plugincontract

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestBindingCommitCASAdvancesRevision(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Commit(0, []Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Commit rev1: %v", err)
	}
	before := store.Version()
	if !before.Installed || before.Revision != 1 || before.Epoch != "boot-1" || before.Digest == "" {
		t.Fatalf("version after first commit = %+v", before)
	}
	committed, err := store.Commit(1, []Binding{globalBinding("memory.search", "next.host")})
	if err != nil {
		t.Fatalf("Commit rev2: %v", err)
	}
	after := store.Version()
	if committed.Revision() != 2 || after.Revision != 2 || !after.Installed {
		t.Fatalf("committed=%d version=%+v want revision 2", committed.Revision(), after)
	}
	if after.Epoch != "boot-1" {
		t.Fatalf("epoch moved to %q", after.Epoch)
	}
	if after.Digest == before.Digest {
		t.Fatalf("digest did not change after a real commit: %s", after.Digest)
	}
	fmt.Printf("binding commit cas epoch=%s from=1 to=2 installed=%v digestChanged=%v\n", after.Epoch, after.Installed, after.Digest != before.Digest)
}

func TestBindingCommitRejectsStaleRevision(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Commit(0, []Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Commit rev1: %v", err)
	}
	if _, err := store.Commit(1, []Binding{globalBinding("memory.search", "next.host")}); err != nil {
		t.Fatalf("Commit rev2: %v", err)
	}
	digest := store.Version().Digest
	_, err = store.Commit(1, []Binding{globalBinding("memory.search", "stale.host")})
	if err == nil {
		t.Fatalf("stale commit must fail")
	}
	var conflict *BindingCommitConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("stale commit error = %v, want *BindingCommitConflictError", err)
	}
	if !errors.Is(err, ErrBindingCommitConflict) {
		t.Fatalf("stale commit lost the sentinel: %v", err)
	}
	if conflict.Expected != 1 || conflict.Actual != 2 || conflict.Epoch != "boot-1" {
		t.Fatalf("conflict detail = %+v", conflict)
	}
	if !strings.Contains(err.Error(), "expected revision 1") || !strings.Contains(err.Error(), "installed revision is 2") {
		t.Fatalf("conflict message lost the numbers: %v", err)
	}
	version := store.Version()
	if version.Revision != 2 || version.Digest != digest {
		t.Fatalf("stale commit mutated the store: %+v", version)
	}
	resolution, resolveErr := store.Current().Resolve(BindingRequest{CapabilityID: "memory.search"})
	if resolveErr != nil || resolution.Binding.ProviderID != "next.host" {
		t.Fatalf("stale commit moved the provider: %v %s", resolveErr, resolution.Binding.ProviderID)
	}
	fmt.Printf("binding commit stale expected=%d actual=%d conflicts=1 chain=ok unchanged=true\n", conflict.Expected, conflict.Actual)
}

func TestBindingCommitRejectsInvalidWithoutMutation(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Commit(0, []Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Commit rev1: %v", err)
	}
	if _, err := store.Commit(1, []Binding{globalBinding("memory.search", "next.host")}); err != nil {
		t.Fatalf("Commit rev2: %v", err)
	}
	before := store.Version()
	invalid := []Binding{{Key: BindingKey{CapabilityID: "memory.search", Scope: BindingScopeGlobal}, ProviderInstanceID: "missing:1"}}
	_, err = store.Commit(2, invalid)
	if err == nil {
		t.Fatalf("invalid bindings must be rejected")
	}
	var invalidErr *BindingInvalidError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("invalid commit error = %v, want *BindingInvalidError", err)
	}
	if !errors.Is(err, ErrBindingInvalid) {
		t.Fatalf("invalid commit lost the sentinel: %v", err)
	}
	after := store.Version()
	if after.Revision != before.Revision || after.Digest != before.Digest {
		t.Fatalf("invalid commit mutated the store: before=%+v after=%+v", before, after)
	}
	retry, err := store.Commit(2, []Binding{globalBinding("memory.search", "retry.host")})
	if err != nil {
		t.Fatalf("retry commit: %v", err)
	}
	if retry.Revision() != before.Revision+1 {
		t.Fatalf("retry revision=%d want %d", retry.Revision(), before.Revision+1)
	}
	fmt.Printf(
		"binding commit invalid field=%s rejected=1 revisionBefore=%d revisionAfter=%d retry=%d\n",
		invalidErr.Field, before.Revision, after.Revision, retry.Revision(),
	)
}

func TestBindingCommitFromEmptyStore(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	empty := store.Version()
	if empty.Installed || empty.Revision != 0 || empty.Digest != "" || empty.Epoch != "boot-1" {
		t.Fatalf("empty version = %+v", empty)
	}
	if _, err := store.Commit(0, []Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("first commit: %v", err)
	}
	_, err = store.Commit(0, []Binding{globalBinding("memory.search", "second.host")})
	if !errors.Is(err, ErrBindingCommitConflict) {
		t.Fatalf("second empty-expectation commit = %v, want conflict", err)
	}
	version := store.Version()
	if version.Revision != 1 || !version.Installed {
		t.Fatalf("version after empty-store commit = %+v", version)
	}
	fmt.Printf("binding commit empty from=0 to=%d installedBefore=%v\n", version.Revision, empty.Installed)
}

func TestBindingCommitPinsInflightProvider(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Commit(0, []Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Commit rev1: %v", err)
	}
	handle, err := store.Acquire()
	if err != nil {
		t.Fatalf("Acquire rev1: %v", err)
	}
	if _, err := store.Commit(1, []Binding{globalBinding("memory.search", "next.host")}); err != nil {
		t.Fatalf("Commit rev2: %v", err)
	}
	stable := true
	for attempt := 0; attempt < 2; attempt++ {
		resolution, resolveErr := handle.Resolve(BindingRequest{CapabilityID: "memory.search"})
		if resolveErr != nil {
			t.Fatalf("inflight resolve %d: %v", attempt, resolveErr)
		}
		if resolution.Revision != 1 || resolution.Binding.ProviderID != "global.host" {
			stable = false
			t.Fatalf("inflight call moved to revision=%d provider=%s", resolution.Revision, resolution.Binding.ProviderID)
		}
	}
	current, err := store.Current().Resolve(BindingRequest{CapabilityID: "memory.search"})
	if err != nil {
		t.Fatalf("current resolve: %v", err)
	}
	if current.Revision != 2 || current.Binding.ProviderID != "next.host" {
		t.Fatalf("current snapshot revision=%d provider=%s want 2/next.host", current.Revision, current.Binding.ProviderID)
	}
	retained := store.RetainedRevisions()
	if len(retained) != 1 || retained[0] != 1 {
		t.Fatalf("retained revisions = %v want [1]", retained)
	}
	handle.Release()
	releasedRetained := store.RetainedCount()
	if releasedRetained != 0 {
		t.Fatalf("retained after release = %v want empty", store.RetainedRevisions())
	}
	reacquired, err := store.Acquire()
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	defer reacquired.Release()
	if reacquired.Revision() != 2 {
		t.Fatalf("reacquired revision=%d want 2", reacquired.Revision())
	}
	fmt.Printf(
		"binding commit pinned provider=global.host revision=1 currentProvider=%s currentRevision=%d retained=%d releasedRetained=%d stable=%v\n",
		current.Binding.ProviderID, current.Revision, retained[0], releasedRetained, stable,
	)
}

func TestBindingCommitCancellationSemantics(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Commit(0, []Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Commit rev1: %v", err)
	}
	if _, err := store.Commit(1, []Binding{globalBinding("memory.search", "next.host")}); err != nil {
		t.Fatalf("Commit rev2: %v", err)
	}
	handle, err := store.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	handle.Release()
	handle.Release()
	_, err = handle.Resolve(BindingRequest{CapabilityID: "memory.search"})
	if !errors.Is(err, ErrSnapshotRelease) {
		t.Fatalf("released handle resolve = %v, want ErrSnapshotRelease", err)
	}
	if store.RetainedCount() != 0 {
		t.Fatalf("released handle still retained: %v", store.RetainedRevisions())
	}
	reacquired, err := store.Acquire()
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	defer reacquired.Release()
	fmt.Printf(
		"binding commit cancellation resolvedAfterRelease=%s doubleRelease=idempotent acquireAfterRelease=%d\n",
		"ErrSnapshotRelease", reacquired.Revision(),
	)
}

func TestBindingCommitConcurrentSingleWinner(t *testing.T) {
	store, err := NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	if _, err := store.Commit(0, []Binding{globalBinding("memory.search", "global.host")}); err != nil {
		t.Fatalf("Commit rev1: %v", err)
	}
	const workers = 32
	var (
		wait      sync.WaitGroup
		mu        sync.Mutex
		winners   []string
		conflicts int
		other     []error
	)
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			provider := fmt.Sprintf("worker.%02d", index)
			_, commitErr := store.Commit(1, []Binding{globalBinding("memory.search", provider)})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case commitErr == nil:
				winners = append(winners, provider)
			case errors.Is(commitErr, ErrBindingCommitConflict):
				conflicts++
			default:
				other = append(other, commitErr)
			}
		}(worker)
	}
	wait.Wait()
	if len(other) > 0 {
		t.Fatalf("unexpected commit errors: %v", other)
	}
	if len(winners) != 1 || conflicts != workers-1 {
		t.Fatalf("winners=%v conflicts=%d want 1/%d", winners, conflicts, workers-1)
	}
	version := store.Version()
	if version.Revision != 2 {
		t.Fatalf("revision=%d want 2", version.Revision)
	}
	resolution, err := store.Current().Resolve(BindingRequest{CapabilityID: "memory.search"})
	if err != nil {
		t.Fatalf("current resolve: %v", err)
	}
	if resolution.Binding.ProviderID != winners[0] {
		t.Fatalf("installed provider=%s winner=%s", resolution.Binding.ProviderID, winners[0])
	}
	fmt.Printf(
		"binding commit concurrent workers=%d successes=%d conflicts=%d revision=%d winners=%d\n",
		workers, len(winners), conflicts, version.Revision, len(winners),
	)
}
