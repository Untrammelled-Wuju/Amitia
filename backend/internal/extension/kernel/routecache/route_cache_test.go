//go:build rsi_routecache

package routecache

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const (
	memoryService = "memory.search"
	emoteService  = "com.amitia/emote/ai"
)

func descriptor(providerID, serviceID string, priority int) plugincontract.ServiceDescriptor {
	return plugincontract.ServiceDescriptor{
		ProviderID: providerID,
		Module:     plugincontract.ModuleRef{PluginID: "plugin." + providerID, ModuleID: "module.main"},
		ServiceID:  serviceID,
		Version:    "1.0.0",
		Scopes:     []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
		Placement:  plugincontract.ServicePlacementCore,
		Priority:   priority,
	}
}

func graphFor(t *testing.T, providers []plugincontract.ServiceDescriptor, services ...string) plugincontract.ServiceGraphResolution {
	t.Helper()
	requirements := make([]plugincontract.ServiceRequirement, 0, len(services))
	for _, service := range services {
		requirements = append(requirements, plugincontract.ServiceRequirement{
			ServiceID:    service,
			VersionRange: ">=1.0.0",
			Selection:    plugincontract.ServiceSelectionAll,
		})
	}
	resolution := plugincontract.ServiceGraphResolver{}.Resolve(plugincontract.ServiceGraphRequest{
		Providers: providers,
		Consumers: []plugincontract.ServiceConsumer{{
			Module:       plugincontract.ModuleRef{PluginID: "consumer.plugin", ModuleID: "module.main"},
			Requirements: requirements,
		}},
	})
	if !resolution.Valid() {
		t.Fatalf("graph must be valid, issues: %v", resolution.Issues)
	}
	return resolution
}

func globalBinding(capabilityID, providerID string) plugincontract.Binding {
	return plugincontract.Binding{
		Key: plugincontract.BindingKey{
			CapabilityID: capabilityID,
			Scope:        plugincontract.BindingScopeGlobal,
		},
		ProviderID:         providerID,
		ProviderInstanceID: providerID + ":instance",
	}
}

func snapshotAt(t *testing.T, revision uint64, bindings ...plugincontract.Binding) *plugincontract.BindingSnapshot {
	t.Helper()
	snapshot, err := plugincontract.NewBindingSnapshot("boot-1", revision, bindings)
	if err != nil {
		t.Fatalf("NewBindingSnapshot: %v", err)
	}
	return snapshot
}

func staticLoader(t *testing.T, resolution plugincontract.ServiceGraphResolution) (GraphLoader, *int) {
	t.Helper()
	loads := 0
	return func() (plugincontract.ServiceGraphResolution, error) {
		loads++
		return resolution, nil
	}, &loads
}

func bindingRows(serviceID, providerID, version string, priority int) plugincontract.ProviderBinding {
	return plugincontract.ProviderBinding{
		ProviderID: providerID,
		PluginID:   "plugin." + providerID,
		ModuleID:   "module.main",
		ServiceID:  serviceID,
		Version:    version,
		Scopes:     []plugincontract.ServiceScope{plugincontract.ServiceScopeGlobal},
		Placement:  plugincontract.ServicePlacementCore,
		Priority:   priority,
		Enabled:    true,
	}
}

func TestRouteCacheParsesGraphOnceForThousandQueries(t *testing.T) {
	resolution := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
		descriptor("provider.b", memoryService, 5),
	}, memoryService)
	loader, loads := staticLoader(t, resolution)
	cache := New(loader)
	snapshot := snapshotAt(t, 7, globalBinding(memoryService, "provider.b"))
	request := plugincontract.BindingRequest{CapabilityID: memoryService}

	started := time.Now()
	var first plugincontract.ProviderCandidates
	for index := 0; index < 1000; index++ {
		candidates, err := cache.Lookup(snapshot, request, memoryService)
		if err != nil {
			t.Fatalf("lookup %d: %v", index, err)
		}
		if index == 0 {
			first = candidates
			continue
		}
		if !reflect.DeepEqual(first, candidates) {
			t.Fatalf("lookup %d diverged from first result", index)
		}
	}
	elapsed := time.Since(started)
	stats := cache.Stats()
	if *loads != 1 || stats.GraphLoads != 1 {
		t.Fatalf("graph loads = %d (loader %d), want 1", stats.GraphLoads, *loads)
	}
	if stats.Computations != 1 {
		t.Fatalf("computations = %d, want 1", stats.Computations)
	}
	if stats.Hits != 999 || stats.Misses != 1 {
		t.Fatalf("hits/misses = %d/%d, want 999/1", stats.Hits, stats.Misses)
	}
	if first.Selected.ProviderID != "provider.b" || first.Revision != 7 {
		t.Fatalf("unexpected selection: %s rev=%d", first.Selected.ProviderID, first.Revision)
	}
	if len(first.Candidates) != 2 || first.Candidates[0].ProviderID != "provider.a" {
		t.Fatalf("candidates not ordered by priority: %+v", first.Candidates)
	}
	t.Logf("route cache evidence lookups=%d graphLoads=%d computations=%d hits=%d elapsed=%s",
		1000, stats.GraphLoads, stats.Computations, stats.Hits, elapsed)
}

func TestRouteCacheKeepsRevisionForInFlightRequests(t *testing.T) {
	resolutionOne := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
		descriptor("provider.b", memoryService, 5),
	}, memoryService)
	resolutionTwo := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
		descriptor("provider.b", memoryService, 5),
		descriptor("provider.c", emoteService, 7),
	}, memoryService, emoteService)
	store, err := plugincontract.NewBindingSnapshotStore("boot-1")
	if err != nil {
		t.Fatalf("NewBindingSnapshotStore: %v", err)
	}
	snapshotOne, err := store.Install([]plugincontract.Binding{
		globalBinding(memoryService, "provider.b"),
		globalBinding(emoteService, "provider.c"),
	})
	if err != nil {
		t.Fatalf("install revision 1: %v", err)
	}
	cache := New(func() (plugincontract.ServiceGraphResolution, error) { return resolutionOne, nil })
	request := plugincontract.BindingRequest{CapabilityID: memoryService}
	if _, err := cache.Lookup(snapshotOne, request, memoryService); err != nil {
		t.Fatalf("warm revision 1: %v", err)
	}

	results := make([]uint64, 100)
	handles := make([]uint64, 100)
	errs := make([]error, 100)
	started := make(chan struct{})
	switched := make(chan struct{})
	var wg sync.WaitGroup
	for index := 0; index < 100; index++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			<-started
			handle, acquireErr := store.Acquire()
			if acquireErr != nil {
				errs[slot] = acquireErr
				return
			}
			snapshot := handle.Snapshot()
			handles[slot] = snapshot.Revision()
			for attempt := 0; attempt < 5; attempt++ {
				if attempt == 3 {
					<-switched
				}
				candidates, lookupErr := cache.Lookup(snapshot, request, memoryService)
				if lookupErr != nil {
					errs[slot] = lookupErr
					handle.Release()
					return
				}
				results[slot] = candidates.Revision
			}
			handle.Release()
		}(index)
	}
	close(started)
	time.Sleep(2 * time.Millisecond)
	snapshotTwo, err := store.Install([]plugincontract.Binding{
		globalBinding(memoryService, "provider.a"),
		globalBinding(emoteService, "provider.c"),
	})
	if err != nil {
		t.Fatalf("install revision 2: %v", err)
	}
	cache.Apply(
		[]plugincontract.ProviderBinding{
			bindingRows(memoryService, "provider.b", "1.0.0", 5),
			bindingRows(emoteService, "provider.c", "1.0.0", 7),
		},
		[]plugincontract.ProviderBinding{
			bindingRows(memoryService, "provider.a", "1.0.0", 10),
			bindingRows(emoteService, "provider.c", "1.0.0", 7),
		},
		snapshotTwo,
		resolutionTwo,
	)
	close(switched)
	wg.Wait()

	mixed := 0
	revisions := make(map[uint64]struct{}, 2)
	for index := 0; index < 100; index++ {
		if errs[index] != nil {
			t.Fatalf("goroutine %d: %v", index, errs[index])
		}
		revisions[results[index]] = struct{}{}
		if results[index] != handles[index] {
			mixed++
		}
	}
	if mixed != 0 {
		t.Fatalf("mixed revisions = %d, want 0", mixed)
	}
	if len(revisions) != 1 {
		t.Fatalf("in-flight revisions = %v, want only revision 1", revisions)
	}
	current, err := cache.Lookup(snapshotTwo, request, memoryService)
	if err != nil {
		t.Fatalf("lookup revision 2: %v", err)
	}
	if current.Revision != 2 || current.Selected.ProviderID != "provider.a" {
		t.Fatalf("revision 2 selection = %s rev=%d", current.Selected.ProviderID, current.Revision)
	}
	t.Logf("route cache evidence concurrent=%d revisions=%d mixed=%d", 100, len(revisions)+1, mixed)
}

func TestRouteCacheInvalidatesOnlyAffectedService(t *testing.T) {
	resolutionOne := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
		descriptor("provider.c", emoteService, 7),
	}, memoryService, emoteService)
	cache := New(func() (plugincontract.ServiceGraphResolution, error) { return resolutionOne, nil })
	snapshot := snapshotAt(t, 4, globalBinding(memoryService, "provider.a"), globalBinding(emoteService, "provider.c"))
	memoryRequest := plugincontract.BindingRequest{CapabilityID: memoryService}
	emoteRequest := plugincontract.BindingRequest{CapabilityID: emoteService}
	if _, err := cache.Lookup(snapshot, memoryRequest, memoryService); err != nil {
		t.Fatalf("warm memory: %v", err)
	}
	if _, err := cache.Lookup(snapshot, emoteRequest, emoteService); err != nil {
		t.Fatalf("warm emote: %v", err)
	}

	resolutionTwo := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
		descriptor("provider.c", emoteService, 9),
	}, memoryService, emoteService)
	nextSnapshot := snapshotAt(t, 5, globalBinding(memoryService, "provider.a"), globalBinding(emoteService, "provider.c"))
	stats := cache.Apply(
		[]plugincontract.ProviderBinding{
			bindingRows(memoryService, "provider.a", "1.0.0", 10),
			bindingRows(emoteService, "provider.c", "1.0.0", 7),
		},
		[]plugincontract.ProviderBinding{
			bindingRows(memoryService, "provider.a", "1.0.0", 10),
			bindingRows(emoteService, "provider.c", "1.1.0", 9),
		},
		nextSnapshot,
		resolutionTwo,
	)
	if stats.Invalidated != 1 {
		t.Fatalf("invalidated = %d, want 1", stats.Invalidated)
	}
	if stats.Entries != 1 {
		t.Fatalf("entries after apply = %d, want 1 (memory.search carried)", stats.Entries)
	}

	before := cache.Stats()
	if _, err := cache.Lookup(nextSnapshot, memoryRequest, memoryService); err != nil {
		t.Fatalf("lookup carried service: %v", err)
	}
	after := cache.Stats()
	if after.Computations != before.Computations {
		t.Fatalf("carried service recomputed: %d -> %d", before.Computations, after.Computations)
	}
	if after.Hits != before.Hits+1 {
		t.Fatalf("carried service did not hit cache: %d -> %d", before.Hits, after.Hits)
	}
	if _, err := cache.Lookup(nextSnapshot, emoteRequest, emoteService); err != nil {
		t.Fatalf("lookup affected service: %v", err)
	}
	final := cache.Stats()
	if final.Computations != before.Computations+1 {
		t.Fatalf("affected service not recomputed: %d -> %d", before.Computations, final.Computations)
	}
	t.Logf("route cache evidence invalidated=%d carried=%d recomputed=%d",
		stats.Invalidated, stats.Entries, final.Computations)
}

func TestRouteCacheReloadsGraphPerRevision(t *testing.T) {
	resolution := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
	}, memoryService)
	loader, loads := staticLoader(t, resolution)
	cache := New(loader)
	request := plugincontract.BindingRequest{CapabilityID: memoryService}
	for revision := uint64(1); revision <= 3; revision++ {
		snapshot := snapshotAt(t, revision, globalBinding(memoryService, "provider.a"))
		if _, err := cache.Lookup(snapshot, request, memoryService); err != nil {
			t.Fatalf("lookup revision %d: %v", revision, err)
		}
	}
	if *loads != 3 {
		t.Fatalf("graph loads = %d, want 3 (one per revision)", *loads)
	}
	if stats := cache.Stats(); stats.GraphLoads != 3 || stats.Revision != 3 {
		t.Fatalf("stats = %+v, want 3 loads at revision 3", stats)
	}
}

func TestRouteCacheRejectsInvalidInputs(t *testing.T) {
	resolution := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
	}, memoryService)
	loader, _ := staticLoader(t, resolution)
	cache := New(loader)
	snapshot := snapshotAt(t, 6, globalBinding(memoryService, "provider.a"))
	request := plugincontract.BindingRequest{CapabilityID: memoryService}

	if _, err := cache.Lookup(nil, request, memoryService); !errors.Is(err, ErrRouteCacheSnapshotMissing) {
		t.Fatalf("nil snapshot error = %v", err)
	}
	if _, err := cache.Lookup(snapshot, request, "   "); err == nil || !errors.Is(err, plugincontract.ErrBindingInvalid) {
		t.Fatalf("empty service error = %v", err)
	}
	bare := New(nil)
	if _, err := bare.Lookup(snapshot, request, memoryService); !errors.Is(err, ErrRouteCacheLoaderMissing) {
		t.Fatalf("missing loader error = %v", err)
	}
	if _, err := (*Cache)(nil).Lookup(snapshot, request, memoryService); !errors.Is(err, ErrRouteCacheLoaderMissing) {
		t.Fatalf("nil cache error = %v", err)
	}
	if _, err := cache.Lookup(snapshot, request, memoryService); err != nil {
		t.Fatalf("prime revision 6: %v", err)
	}
	seventh := snapshotAt(t, 7, globalBinding(memoryService, "provider.a"))
	if _, err := cache.Lookup(seventh, request, memoryService); err != nil {
		t.Fatalf("prime revision 7: %v", err)
	}
	newer := snapshotAt(t, 9, globalBinding(memoryService, "provider.a"))
	if _, err := cache.Lookup(newer, request, memoryService); err != nil {
		t.Fatalf("prime revision 9: %v", err)
	}
	if _, err := cache.Lookup(snapshot, request, memoryService); !errors.Is(err, ErrRouteCacheRevisionStale) {
		t.Fatalf("stale revision error = %v", err)
	}
	invalid := cache.Apply(nil, nil, newer, plugincontract.ServiceGraphResolution{
		Issues: []plugincontract.ServiceGraphIssue{{Code: "cycle", Detail: "consumer.plugin/module.main"}},
	})
	if invalid.Invalidated == 0 || invalid.Entries != 0 {
		t.Fatalf("invalid graph did not drop entries: %+v", invalid)
	}
	if _, err := cache.Lookup(newer, request, memoryService); err == nil || !errors.Is(err, plugincontract.ErrServiceGraphInvalid) {
		t.Fatalf("invalid graph error = %v", err)
	}
}

func TestRouteCacheReturnsDefensiveCopies(t *testing.T) {
	resolution := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
		descriptor("provider.b", memoryService, 5),
	}, memoryService)
	cache := New(func() (plugincontract.ServiceGraphResolution, error) { return resolution, nil })
	snapshot := snapshotAt(t, 3, globalBinding(memoryService, "provider.a"))
	request := plugincontract.BindingRequest{CapabilityID: memoryService}
	first, err := cache.Lookup(snapshot, request, memoryService)
	if err != nil {
		t.Fatalf("first lookup: %v", err)
	}
	first.Candidates[0].ProviderID = "tampered"
	first.Candidates[0].Scopes[0] = plugincontract.ServiceScopeDevice
	first.Trail = append(first.Trail, "tampered")
	second, err := cache.Lookup(snapshot, request, memoryService)
	if err != nil {
		t.Fatalf("second lookup: %v", err)
	}
	if second.Candidates[0].ProviderID != "provider.a" || second.Candidates[0].Scopes[0] != plugincontract.ServiceScopeGlobal {
		t.Fatalf("cache leaked mutable state: %+v", second.Candidates[0])
	}
	for _, entry := range second.Trail {
		if entry == "tampered" {
			t.Fatalf("trail leaked mutable state: %v", second.Trail)
		}
	}
}

func TestRouteCacheConcurrentLookupsAgree(t *testing.T) {
	resolution := graphFor(t, []plugincontract.ServiceDescriptor{
		descriptor("provider.a", memoryService, 10),
		descriptor("provider.b", memoryService, 5),
	}, memoryService)
	loader, loads := staticLoader(t, resolution)
	cache := New(loader)
	snapshot := snapshotAt(t, 11, globalBinding(memoryService, "provider.b"))
	request := plugincontract.BindingRequest{CapabilityID: memoryService}
	baseline, err := cache.Lookup(snapshot, request, memoryService)
	if err != nil {
		t.Fatalf("baseline lookup: %v", err)
	}
	var wg sync.WaitGroup
	mismatch := make([]int, 100)
	for worker := 0; worker < 100; worker++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			for round := 0; round < 10; round++ {
				candidates, lookupErr := cache.Lookup(snapshot, request, memoryService)
				if lookupErr != nil {
					mismatch[slot]++
					return
				}
				if !reflect.DeepEqual(baseline, candidates) {
					mismatch[slot]++
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	for slot, count := range mismatch {
		if count != 0 {
			t.Fatalf("worker %d saw %d mismatches", slot, count)
		}
	}
	stats := cache.Stats()
	if *loads != 1 || stats.Computations != 1 {
		t.Fatalf("loads=%d computations=%d, want 1/1", *loads, stats.Computations)
	}
	if stats.Hits != 1000 {
		t.Fatalf("hits = %d, want 1000", stats.Hits)
	}
}

func TestRouteCacheRealScaleProviderGraph(t *testing.T) {
	for _, providers := range []int{56, 57} {
		rows := make([]plugincontract.ServiceDescriptor, 0, providers)
		for index := 0; index < providers; index++ {
			rows = append(rows, descriptor(fmt.Sprintf("provider.real.%02d", index), memoryService, providers-index))
		}
		resolution := graphFor(t, rows, memoryService)
		loader, loads := staticLoader(t, resolution)
		cache := New(loader)
		request := plugincontract.BindingRequest{CapabilityID: memoryService}
		snapshot := snapshotAt(t, 17, globalBinding(memoryService, rows[0].ProviderID))
		started := time.Now()
		first, err := cache.Lookup(snapshot, request, memoryService)
		if err != nil {
			t.Fatalf("providers %d first lookup: %v", providers, err)
		}
		if first.Selected.ProviderID != rows[0].ProviderID {
			t.Fatalf("providers %d selected %s, want %s", providers, first.Selected.ProviderID, rows[0].ProviderID)
		}
		for index := 0; index < 999; index++ {
			if _, err := cache.Lookup(snapshot, request, memoryService); err != nil {
				t.Fatalf("providers %d lookup %d: %v", providers, index, err)
			}
		}
		stats := cache.Stats()
		if *loads != 1 || stats.Computations != 1 || stats.GraphLoads != 1 {
			t.Fatalf("providers %d loads=%d computations=%d graphLoads=%d", providers, *loads, stats.Computations, stats.GraphLoads)
		}
		if stats.Hits != 999 {
			t.Fatalf("providers %d hits = %d, want 999", providers, stats.Hits)
		}
		t.Logf("route cache realscale providers=%d graphLoads=%d lookups=%d elapsed=%s",
			providers, stats.GraphLoads, 1000, time.Since(started))
	}
}

func TestRouteCachePluginTierScaling(t *testing.T) {
	tiers := []int{0, 1, 3, 5, 10}
	for _, tier := range tiers {
		providers := make([]plugincontract.ServiceDescriptor, 0, tier)
		bindings := make([]plugincontract.Binding, 0, 1)
		for index := 0; index < tier; index++ {
			providerID := fmt.Sprintf("provider.%02d", index)
			providers = append(providers, descriptor(providerID, memoryService, 100-index))
		}
		if tier > 0 {
			bindings = append(bindings, globalBinding(memoryService, providers[0].ProviderID))
		}
		var resolution plugincontract.ServiceGraphResolution
		if tier == 0 {
			resolution = plugincontract.ServiceGraphResolver{}.Resolve(plugincontract.ServiceGraphRequest{})
			if !resolution.Valid() {
				t.Fatalf("empty graph must be valid, issues: %v", resolution.Issues)
			}
		} else {
			resolution = graphFor(t, providers, memoryService)
		}
		loader, loads := staticLoader(t, resolution)
		cache := New(loader)
		request := plugincontract.BindingRequest{CapabilityID: memoryService}
		var snapshot *plugincontract.BindingSnapshot
		if len(bindings) == 0 {
			snapshot = snapshotAt(t, 1)
		} else {
			snapshot = snapshotAt(t, 1, bindings...)
		}
		started := time.Now()
		firstStarted := time.Now()
		if _, err := cache.Lookup(snapshot, request, memoryService); err != nil {
			if len(bindings) == 0 {
				t.Logf("route cache tier plugins=%d providers=%d graphLoads=%d empty=true", tier, len(providers), cache.Stats().GraphLoads)
				continue
			}
			t.Fatalf("tier %d first lookup: %v", tier, err)
		}
		firstUs := time.Since(firstStarted).Microseconds()
		for index := 0; index < 200; index++ {
			if _, err := cache.Lookup(snapshot, request, memoryService); err != nil {
				t.Fatalf("tier %d lookup %d: %v", tier, index, err)
			}
		}
		stats := cache.Stats()
		if *loads != 1 || stats.Computations != 1 {
			t.Fatalf("tier %d loads=%d computations=%d, want 1/1", tier, *loads, stats.Computations)
		}
		if stats.Hits != 200 {
			t.Fatalf("tier %d hits = %d, want 200", tier, stats.Hits)
		}
		t.Logf("route cache tier plugins=%d providers=%d graphLoads=%d firstLookupUs=%d queryUs=%d",
			tier, len(providers), stats.GraphLoads, firstUs, time.Since(started).Microseconds())
	}
}
