//go:build rsi_scale

package capability_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const (
	scalePlugins      = 2000
	scaleSlots        = 5
	scaleZones        = 40
	scaleOperations   = 200
	scaleQualityCases = 20
	scaleQualityTopK  = 5
	scaleWarmup       = 20
	scaleDetailTopK   = 8
)

func scaleLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m05-s07 %s", fmt.Sprintf(format, args...))
}

func scaleDetail(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m05-s07-detail %s", fmt.Sprintf(format, args...))
}

func scaleZone(plugin int) string {
	return fmt.Sprintf("zone%02d", plugin%scaleZones)
}

func scaleMarker(plugin int) string {
	return fmt.Sprintf("marker%05d", plugin)
}

func scaleQuery(plugin int) string {
	return scaleZone(plugin) + " " + scaleMarker(plugin)
}

func scaleTools(plugins int) []capability.ToolDefinition {
	defs := make([]capability.ToolDefinition, 0, plugins*scaleSlots)
	for plugin := 0; plugin < plugins; plugin++ {
		for slot := 0; slot < scaleSlots; slot++ {
			index := plugin*scaleSlots + slot
			name := fmt.Sprintf("svc_%05d_%d", plugin, slot)
			keywords := []string{}
			if slot == 0 {
				keywords = []string{scaleZone(plugin), scaleMarker(plugin)}
			}
			def := taskGateTool(fmt.Sprintf("tool-%05d", index), name, keywords...)
			def.InputSchema = json.RawMessage(detailSchemaBody(2, 10, 180, 200))
			defs = append(defs, def)
		}
	}
	return defs
}

func scaleRegistry(t *testing.T, defs []capability.ToolDefinition) *capability.ToolRegistry {
	t.Helper()
	registry := capability.NewToolRegistry()
	if err := registry.BatchRegister(context.Background(), defs); err != nil {
		t.Fatalf("BatchRegister: %v", err)
	}
	return registry
}

func scaleDetailManager(t *testing.T, defs []capability.ToolDefinition) (*capability.ToolExposureManager, *capability.SchemaStore) {
	t.Helper()
	registry := scaleRegistry(t, defs)
	manager := capability.NewToolExposureManager(registry, nil).WithTaskSelector(capability.NewTaskSelector(registry, nil))
	store := capability.NewSchemaStore(capability.DefaultSchemaTTL)
	index, err := capability.BuildDescriptorIndex(store, defs)
	if err != nil {
		t.Fatalf("BuildDescriptorIndex: %v", err)
	}
	return manager.WithSchemaSource(index, store), store
}

func scaleToolIndex(id string) int {
	if !strings.HasPrefix(id, "tool-") {
		return -1
	}
	index, err := strconv.Atoi(strings.TrimPrefix(id, "tool-"))
	if err != nil {
		return -1
	}
	return index
}

func scaleZoneOf(id string) string {
	index := scaleToolIndex(id)
	if index < 0 {
		return ""
	}
	return scaleZone(index / scaleSlots)
}

func scaleExpected(plugin int) string {
	return fmt.Sprintf("tool-%05d", plugin*scaleSlots)
}

func scalePercentile(sorted []time.Duration, ratio float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	position := int(math.Ceil(float64(len(sorted))*ratio)) - 1
	if position < 0 {
		position = 0
	}
	if position >= len(sorted) {
		position = len(sorted) - 1
	}
	return sorted[position]
}

func TestScaleColdIndexBuild(t *testing.T) {
	defs := scaleTools(scalePlugins)
	started := time.Now()
	registry := scaleRegistry(t, defs)
	registerCost := time.Since(started)
	store := capability.NewSchemaStore(capability.DefaultSchemaTTL)
	started = time.Now()
	index, err := capability.BuildDescriptorIndex(store, defs)
	if err != nil {
		t.Fatalf("BuildDescriptorIndex: %v", err)
	}
	indexCost := time.Since(started)
	if registry.Count() != scalePlugins*scaleSlots {
		t.Fatalf("registry count=%d want %d", registry.Count(), scalePlugins*scaleSlots)
	}
	if index.Len() != scalePlugins*scaleSlots {
		t.Fatalf("index entries=%d want %d", index.Len(), scalePlugins*scaleSlots)
	}
	if store.Reads() != 0 {
		t.Fatalf("cold build touched schema reads=%d", store.Reads())
	}
	scaleDetail(t, "cold register_ms=%d index_ms=%d total_ms=%d", registerCost.Milliseconds(), indexCost.Milliseconds(), (registerCost + indexCost).Milliseconds())
	scaleLog(t, "cold plugins=%d tools=%d entries=%d schema_reads=0", scalePlugins, registry.Count(), index.Len())
}

func TestScaleHotQueryLatency(t *testing.T) {
	defs := scaleTools(scalePlugins)
	registry := scaleRegistry(t, defs)
	selector := capability.NewTaskSelector(registry, nil)
	ctx := context.Background()
	plugin := 123
	query := scaleQuery(plugin)
	want := scaleExpected(plugin)
	for attempt := 0; attempt < scaleWarmup; attempt++ {
		if _, err := selector.Select(ctx, detailInvocation(), query, scaleQualityTopK); err != nil {
			t.Fatalf("warmup Select: %v", err)
		}
	}
	durations := make([]time.Duration, 0, scaleOperations)
	first := ""
	for attempt := 0; attempt < scaleOperations; attempt++ {
		started := time.Now()
		report, err := selector.Select(ctx, detailInvocation(), query, scaleQualityTopK)
		elapsed := time.Since(started)
		if err != nil {
			t.Fatalf("Select #%d: %v", attempt, err)
		}
		if attempt == 0 {
			first = report.Selected[0].Tool.ID
		}
		durations = append(durations, elapsed)
	}
	sort.Slice(durations, func(left, right int) bool { return durations[left] < durations[right] })
	p50 := scalePercentile(durations, 0.50)
	p95 := scalePercentile(durations, 0.95)
	p99 := scalePercentile(durations, 0.99)
	max := durations[len(durations)-1]
	if first != want {
		t.Fatalf("hot top1=%s want %s", first, want)
	}
	if p99 > p50*8+8*time.Millisecond {
		t.Fatalf("hot latency tail p50=%s p99=%s exceeds 8x p50 + 8ms", p50, p99)
	}
	scaleDetail(t, "hot p50_us=%d p95_us=%d p99_us=%d max_us=%d",
		p50.Microseconds(), p95.Microseconds(), p99.Microseconds(), max.Microseconds())
	scaleLog(t, "hot ops=%d top1=%s tail_guard=true", scaleOperations, first)
}

func TestScaleTop1Top5Quality(t *testing.T) {
	defs := scaleTools(scalePlugins)
	registry := scaleRegistry(t, defs)
	selector := capability.NewTaskSelector(registry, nil)
	ctx := context.Background()
	top1 := 0
	top5 := 0
	injected := 0
	for testCase := 0; testCase < scaleQualityCases; testCase++ {
		plugin := (testCase*97 + 13) % scalePlugins
		report, err := selector.Select(ctx, detailInvocation(), scaleQuery(plugin), scaleQualityTopK)
		if err != nil {
			t.Fatalf("Select plugin=%d: %v", plugin, err)
		}
		want := scaleExpected(plugin)
		zone := scaleZone(plugin)
		if len(report.Selected) == 0 {
			t.Fatalf("plugin=%d selected nothing", plugin)
		}
		if report.Selected[0].Tool.ID == want {
			top1++
		}
		found := false
		for _, selected := range report.Selected {
			if selected.Tool.ID == want {
				found = true
			}
			if scaleZoneOf(selected.Tool.ID) != zone {
				injected++
			}
		}
		if found {
			top5++
		}
	}
	rate := float64(injected) / float64(scaleQualityCases*scaleQualityTopK)
	if top1 != scaleQualityCases {
		t.Fatalf("top1=%d want %d", top1, scaleQualityCases)
	}
	if top5 != scaleQualityCases {
		t.Fatalf("top5=%d want %d", top5, scaleQualityCases)
	}
	if injected != 0 {
		t.Fatalf("false injection=%d want 0", injected)
	}
	scaleLog(t, "quality queries=%d top1=%d top5=%d false_injection=%d rate=%.3f",
		scaleQualityCases, top1, top5, injected, rate)
}

func TestScaleSchemaTokenCurve(t *testing.T) {
	sizes := []int{200, 1000, scalePlugins}
	tokens := make(map[int]int, len(sizes))
	reads := make(map[int]int64, len(sizes))
	views := 0
	for _, plugins := range sizes {
		manager, _ := scaleDetailManager(t, scaleTools(plugins))
		page, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{
			Query:  scaleQuery(123),
			Limit:  scaleDetailTopK,
			Budget: capability.DefaultSchemaTokenBudget,
		})
		if err != nil {
			t.Fatalf("GetModelToolDetails n=%d: %v", plugins, err)
		}
		if len(page.Views) == 0 || len(page.Views) > scaleDetailTopK {
			t.Fatalf("n=%d views=%d want 1..%d", plugins, len(page.Views), scaleDetailTopK)
		}
		if page.UsedTokens > capability.DefaultSchemaTokenBudget {
			t.Fatalf("n=%d used tokens=%d over budget", plugins, page.UsedTokens)
		}
		if page.SchemaReads < int64(len(page.Views)) || page.SchemaReads > scaleDetailTopK {
			t.Fatalf("n=%d schema reads=%d views=%d topk=%d", plugins, page.SchemaReads, len(page.Views), scaleDetailTopK)
		}
		tokens[plugins] = page.UsedTokens
		reads[plugins] = page.SchemaReads
		views = len(page.Views)
	}
	equal := tokens[sizes[0]] == tokens[sizes[1]] && tokens[sizes[1]] == tokens[sizes[2]]
	if !equal {
		t.Fatalf("token curve not flat: %v", tokens)
	}
	if reads[sizes[0]] != reads[sizes[1]] || reads[sizes[1]] != reads[sizes[2]] {
		t.Fatalf("schema reads not flat: %v", reads)
	}
	scaleLog(t, "tokens n200=%d n1000=%d n10000=%d views=%d reads=%d,%d,%d equal=true",
		tokens[200], tokens[1000], tokens[scalePlugins], views,
		reads[200], reads[1000], reads[scalePlugins])
}

func TestScalePermissionChange(t *testing.T) {
	defs := scaleTools(scalePlugins)
	registry := scaleRegistry(t, defs)
	ctx := context.Background()
	plugin := 123
	want := scaleExpected(plugin)
	open := capability.NewTaskSelector(registry, nil)
	before, err := open.Select(ctx, detailInvocation(), scaleQuery(plugin), scaleQualityTopK)
	if err != nil {
		t.Fatalf("Select open: %v", err)
	}
	if before.Selected[0].Tool.ID != want {
		t.Fatalf("permission open top1=%s want %s", before.Selected[0].Tool.ID, want)
	}
	closed := capability.NewTaskSelector(registry, nil).WithGrantResolver(taskGateGrants{denied: map[string]bool{want: true}})
	after, err := closed.Select(ctx, detailInvocation(), scaleQuery(plugin), scaleQualityTopK)
	if err != nil {
		t.Fatalf("Select closed: %v", err)
	}
	if after.Selected[0].Tool.ID == want {
		t.Fatalf("denied tool %s still ranked first", want)
	}
	if scaleZoneOf(after.Selected[0].Tool.ID) != scaleZone(plugin) {
		t.Fatalf("denied fallback left zone: %s", after.Selected[0].Tool.ID)
	}
	denied := 0
	for _, rejected := range after.Rejected {
		if rejected.ToolID != want {
			continue
		}
		denied++
		if !taskGateHasReason(rejected.Reasons, capability.ReasonPermissionMissing) {
			t.Fatalf("denied tool reasons=%v want permission_missing", rejected.Reasons)
		}
	}
	if denied != 1 {
		t.Fatalf("denied rejections=%d want 1", denied)
	}
	scaleLog(t, "permission before=%s after=%s rejected=1 reason=permission_missing zone_kept=true",
		before.Selected[0].Tool.ID, after.Selected[0].Tool.ID)
}

func TestScaleRepeatCancelAndRestart(t *testing.T) {
	defs := scaleTools(scalePlugins)
	registry := scaleRegistry(t, defs)
	ctx := context.Background()
	plugin := 123
	query := scaleQuery(plugin)
	first, err := capability.NewTaskSelector(registry, nil).Select(ctx, detailInvocation(), query, scaleQualityTopK)
	if err != nil {
		t.Fatalf("Select first: %v", err)
	}
	second, err := capability.NewTaskSelector(registry, nil).Select(ctx, detailInvocation(), query, scaleQualityTopK)
	if err != nil {
		t.Fatalf("Select second: %v", err)
	}
	if !equalIDs(first.SelectedIDs(), second.SelectedIDs()) {
		t.Fatalf("restart ids differ: %v vs %v", first.SelectedIDs(), second.SelectedIDs())
	}
	repeat, err := capability.NewTaskSelector(registry, nil).Select(ctx, detailInvocation(), query, scaleQualityTopK)
	if err != nil {
		t.Fatalf("Select repeat: %v", err)
	}
	if !equalIDs(first.SelectedIDs(), repeat.SelectedIDs()) {
		t.Fatalf("repeat ids differ")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := capability.NewTaskSelector(registry, nil).Select(cancelled, detailInvocation(), query, scaleQualityTopK); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v want context.Canceled", err)
	}
	manager, _ := scaleDetailManager(t, defs)
	if _, err := manager.GetModelToolDetails(ctx, detailInvocation(), capability.DetailRequest{Query: query, Limit: -1}); !errors.Is(err, capability.ErrDetailRequestInvalid) {
		t.Fatalf("negative limit err=%v want ErrDetailRequestInvalid", err)
	}
	scaleLog(t, "repeat equal=true restart_equal=true cancel=context.Canceled invalid_limit=ErrDetailRequestInvalid")
}

func equalIDs(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
