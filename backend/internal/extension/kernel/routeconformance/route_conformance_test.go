//go:build rsi_routeconformance

package routeconformance

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

const (
	selectionProviders   = 64
	selectionRuns        = 5000
	concurrencyWorkers   = 128
	concurrencyIteration = 200
)

func TestContractFreeze(t *testing.T) {
	fmt.Printf(
		"route conformance contract bindingFields=%d keyFields=%d requestFields=%d scopes=%d methods=%d golden=%s\n",
		reflect.TypeOf(plugincontract.Binding{}).NumField(),
		reflect.TypeOf(plugincontract.BindingKey{}).NumField(),
		reflect.TypeOf(plugincontract.BindingRequest{}).NumField(),
		len(FrozenScopeValues()),
		reflect.PointerTo(reflect.TypeOf(plugincontract.BindingSnapshot{})).NumMethod(),
		GoldenStatus(),
	)
	if problems := ContractProblems(); len(problems) > 0 {
		t.Fatalf("binding contract drifted: %s", strings.Join(problems, "; "))
	}
}

func TestPermissionMatrix(t *testing.T) {
	snapshot, err := PermissionSnapshot()
	if err != nil {
		t.Fatalf("permission snapshot: %v", err)
	}
	rows, allowed, denied, problems := CheckPermissions(snapshot)
	if len(problems) > 0 {
		t.Fatalf("permission matrix failed: %s", strings.Join(problems, "; "))
	}
	if rows != 10 || allowed != 6 || denied != 4 {
		t.Fatalf("permission matrix rows=%d allowed=%d denied=%d want 10/6/4", rows, allowed, denied)
	}
	fmt.Printf("route conformance permission rows=%d allowed=%d denied=%d sentinels=ok\n", rows, allowed, denied)
}

func TestSelectionOrderingAndBudget(t *testing.T) {
	providers := ProviderDescriptors(selectionProviders)
	resolution, err := SelectionResolution(providers)
	if err != nil {
		t.Fatalf("selection graph: %v", err)
	}
	snapshot, err := SelectionSnapshot(providers[0].ProviderID)
	if err != nil {
		t.Fatalf("selection snapshot: %v", err)
	}
	request := SelectionRequest()
	baseline, err := plugincontract.SelectProviderCandidates(resolution, snapshot, request, selectionService)
	if err != nil {
		t.Fatalf("baseline selection: %v", err)
	}
	if len(baseline.Candidates) != selectionProviders {
		t.Fatalf("candidates=%d want %d", len(baseline.Candidates), selectionProviders)
	}
	if !baseline.Bound || baseline.Selected.ProviderID != providers[0].ProviderID {
		t.Fatalf("selected=%q bound=%v want %q", baseline.Selected.ProviderID, baseline.Bound, providers[0].ProviderID)
	}
	if !DescendingPriorities(baseline) {
		t.Fatalf("candidates are not ordered by descending priority: %v", CandidateProviderIDs(baseline))
	}
	wantIDs := CandidateProviderIDs(baseline)
	start := time.Now()
	for run := 0; run < selectionRuns; run++ {
		observed, selectErr := plugincontract.SelectProviderCandidates(resolution, snapshot, request, selectionService)
		if selectErr != nil {
			t.Fatalf("selection run %d: %v", run, selectErr)
		}
		if strings.Join(CandidateProviderIDs(observed), ",") != strings.Join(wantIDs, ",") {
			t.Fatalf("selection run %d drifted: %v", run, CandidateProviderIDs(observed))
		}
	}
	elapsed := time.Since(start)
	allocs := testing.AllocsPerRun(50, func() {
		if _, selectErr := plugincontract.SelectProviderCandidates(resolution, snapshot, request, selectionService); selectErr != nil {
			t.Fatalf("allocation probe: %v", selectErr)
		}
	})
	if elapsed > 5*time.Second {
		t.Fatalf("selection budget exceeded: %d runs took %s", selectionRuns, elapsed)
	}
	fmt.Printf(
		"route conformance selection providers=%d candidates=%d selections=%d elapsedMs=%d allocsPerOp=%d ordered=ok deterministic=ok\n",
		len(providers), len(baseline.Candidates), selectionRuns, elapsed.Milliseconds(), int(allocs),
	)
}

func TestConcurrentSelection(t *testing.T) {
	providers := ProviderDescriptors(selectionProviders)
	resolution, err := SelectionResolution(providers)
	if err != nil {
		t.Fatalf("concurrency graph: %v", err)
	}
	snapshot, err := SelectionSnapshot(providers[0].ProviderID)
	if err != nil {
		t.Fatalf("concurrency snapshot: %v", err)
	}
	request := SelectionRequest()
	baseline, err := plugincontract.SelectProviderCandidates(resolution, snapshot, request, selectionService)
	if err != nil {
		t.Fatalf("concurrency baseline: %v", err)
	}
	want := strings.Join(CandidateProviderIDs(baseline), ",")
	var (
		wait       sync.WaitGroup
		mismatches int
		mu         sync.Mutex
	)
	for worker := 0; worker < concurrencyWorkers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < concurrencyIteration; iteration++ {
				observed, selectErr := plugincontract.SelectProviderCandidates(resolution, snapshot, request, selectionService)
				if selectErr != nil || strings.Join(CandidateProviderIDs(observed), ",") != want {
					mu.Lock()
					mismatches++
					mu.Unlock()
				}
			}
		}()
	}
	wait.Wait()
	if mismatches != 0 {
		t.Fatalf("concurrent selections mismatched=%d want 0", mismatches)
	}
	fmt.Printf("route conformance concurrency goroutines=%d iterations=%d mismatches=%d\n", concurrencyWorkers, concurrencyIteration, mismatches)
}

func TestSnapshotIsolation(t *testing.T) {
	snapshot, err := SelectionSnapshot("provider.00")
	if err != nil {
		t.Fatalf("isolation snapshot: %v", err)
	}
	digest := snapshot.Digest()
	copied := snapshot.Bindings()
	if len(copied) != 1 {
		t.Fatalf("bindings=%d want 1", len(copied))
	}
	copied[0].ProviderID = "provider.tampered"
	copied[0].Key.Scope = plugincontract.BindingScopeDevice
	if snapshot.Len() != 1 {
		t.Fatalf("snapshot length changed to %d", snapshot.Len())
	}
	observed := snapshot.Bindings()
	if observed[0].ProviderID != "provider.00" || observed[0].Key.Scope != plugincontract.BindingScopeGlobal {
		t.Fatalf("snapshot was mutated through Bindings(): %+v", observed[0])
	}
	if snapshot.Digest() != digest {
		t.Fatalf("digest changed from %s to %s", digest, snapshot.Digest())
	}
	fmt.Printf("route conformance isolation bindings=%d digest=unchanged mutate=blocked\n", snapshot.Len())
}
