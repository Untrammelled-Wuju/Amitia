package capability_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

const detailInvocationSpace = "space-detail"

func detailLog(t *testing.T, line string) {
	t.Helper()
	t.Logf("m05-s05 %s", line)
}

func detailInvocation() capability.ToolInvocationContext {
	return capability.ToolInvocationContext{InvocationID: "inv-detail", SpaceID: detailInvocationSpace}
}

func detailSchemaBody(required int, optional int, requiredPad int, optionalPad int) string {
	properties := make([]string, 0, required+optional)
	requiredNames := make([]string, 0, required)
	for i := 0; i < required; i++ {
		name := fmt.Sprintf("req_%02d", i)
		requiredNames = append(requiredNames, fmt.Sprintf("%q", name))
		properties = append(properties, fmt.Sprintf(`%q:{"type":"string","description":%q}`, name, strings.Repeat("r", requiredPad)))
	}
	for i := 0; i < optional; i++ {
		name := fmt.Sprintf("opt_%02d", i)
		properties = append(properties, fmt.Sprintf(`%q:{"type":"string","description":%q}`, name, strings.Repeat("o", optionalPad)))
	}
	return fmt.Sprintf(`{"type":"object","properties":{%s},"required":[%s]}`, strings.Join(properties, ","), strings.Join(requiredNames, ","))
}

func detailTool(id string, keywords ...string) capability.ToolDefinition {
	def := taskGateTool(id, strings.ReplaceAll(id, "tool-", ""), keywords...)
	def.InputSchema = json.RawMessage(detailSchemaBody(2, 10, 180, 200))
	return def
}

func detailManager(t *testing.T, defs []capability.ToolDefinition, withSchema bool) (*capability.ToolExposureManager, *capability.SchemaStore) {
	t.Helper()
	registry := taskGateRegistry(t, defs...)
	manager := capability.NewToolExposureManager(registry, nil).WithTaskSelector(capability.NewTaskSelector(registry, nil))
	if !withSchema {
		return manager, nil
	}
	store := capability.NewSchemaStore(capability.DefaultSchemaTTL)
	index, err := capability.BuildDescriptorIndex(store, defs)
	if err != nil {
		t.Fatalf("BuildDescriptorIndex: %v", err)
	}
	return manager.WithSchemaSource(index, store), store
}

func detailSelectedSchema(t *testing.T, raw json.RawMessage) ([]string, []string, int) {
	t.Helper()
	var body struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode returned schema: %v", err)
	}
	names := make([]string, 0, len(body.Properties))
	for name := range body.Properties {
		names = append(names, name)
	}
	return body.Required, names, len(raw)
}

func TestDetailTrimKeepsRequiredProperties(t *testing.T) {
	schema := json.RawMessage(detailSchemaBody(1, 10, 180, 200))
	trimmed, report, err := capability.TrimSchemaToBudget(schema, 120)
	if err != nil {
		t.Fatalf("TrimSchemaToBudget: %v", err)
	}
	required, names, bytes := detailSelectedSchema(t, trimmed)
	if len(required) != 1 || required[0] != "req_00" {
		t.Fatalf("required=%v want [req_00]", required)
	}
	kept := false
	for _, name := range names {
		if name == "req_00" {
			kept = true
		}
	}
	if !kept {
		t.Fatalf("required property req_00 missing from trimmed names %v", names)
	}
	if len(report.Dropped) == 0 {
		t.Fatal("trimming dropped no optional property")
	}
	if report.FinalTokens > 120 || (bytes+3)/4 > 120 {
		t.Fatalf("trimmed schema tokens=%d bytes=%d budget=120", report.FinalTokens, bytes)
	}
	detailLog(t, fmt.Sprintf("trim required_kept=1 dropped=%d final_tokens=%d budget=120", len(report.Dropped), report.FinalTokens))
}

func TestDetailTrimRefusesRequiredOverflow(t *testing.T) {
	schema := json.RawMessage(detailSchemaBody(3, 0, 400, 0))
	trimmed, report, err := capability.TrimSchemaToBudget(schema, 50)
	if !errors.Is(err, capability.ErrDetailBudgetExhausted) {
		t.Fatalf("err=%v want ErrDetailBudgetExhausted", err)
	}
	if trimmed != nil {
		t.Fatalf("trimmed schema returned on refusal: %d bytes", len(trimmed))
	}
	if len(report.Dropped) != 0 {
		t.Fatalf("refusal dropped %d optional properties that do not exist", len(report.Dropped))
	}
	detailLog(t, fmt.Sprintf("required_overflow=refused budget=50 need=%d", report.OriginalTokens))
}

func TestDetailTopKTokensDoNotGrowWithRegistrySize(t *testing.T) {
	tokens := map[int]int{}
	reads := map[int]int64{}
	counts := map[int]int{}
	for _, total := range []int{100, 1000, 2000} {
		defs := make([]capability.ToolDefinition, 0, total)
		for i := 0; i < total; i++ {
			id := fmt.Sprintf("tool-detail-%05d", i)
			if i < 8 {
				defs = append(defs, detailTool(id, "周报", "生成"))
				continue
			}
			defs = append(defs, detailTool(id))
		}
		manager, store := detailManager(t, defs, true)
		page, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{
			Query:  "生成周报",
			Limit:  8,
			Budget: 8192,
		})
		if err != nil {
			t.Fatalf("N=%d GetModelToolDetails: %v", total, err)
		}
		if len(page.Views) != 8 {
			t.Fatalf("N=%d views=%d want 8", total, len(page.Views))
		}
		if page.SchemaReads != int64(len(page.Views)) {
			t.Fatalf("N=%d schema reads=%d want %d", total, page.SchemaReads, len(page.Views))
		}
		if store.Reads() != int64(len(page.Views)) {
			t.Fatalf("N=%d store reads=%d want %d", total, store.Reads(), len(page.Views))
		}
		tokens[total] = page.UsedTokens
		reads[total] = page.SchemaReads
		counts[total] = len(page.Views)
	}
	if tokens[100] != tokens[1000] || tokens[1000] != tokens[2000] {
		t.Fatalf("schema tokens grew with registry size: %v", tokens)
	}
	detailLog(t, fmt.Sprintf("tokens_n100=%d tokens_n1000=%d tokens_n2000=%d reads=%d equal=true", tokens[100], tokens[1000], tokens[2000], reads[2000]))
}

func TestDetailCursorPaginatesWithoutDuplicates(t *testing.T) {
	defs := make([]capability.ToolDefinition, 0, 5)
	for i := 0; i < 5; i++ {
		defs = append(defs, detailTool(fmt.Sprintf("tool-page-%02d", i), "周报", "生成"))
	}
	manager, _ := detailManager(t, defs, false)
	seen := map[string]bool{}
	cursor := ""
	sizes := []int{}
	for pageIndex := 0; pageIndex < 4; pageIndex++ {
		page, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{
			Query:  "生成周报",
			Limit:  2,
			Budget: 8192,
			Cursor: cursor,
		})
		if err != nil {
			t.Fatalf("page %d: %v", pageIndex, err)
		}
		sizes = append(sizes, len(page.Views))
		for _, view := range page.Views {
			if seen[view.Name] {
				t.Fatalf("page %d repeated tool %s", pageIndex, view.Name)
			}
			seen[view.Name] = true
		}
		if !page.HasMore {
			if page.NextCursor != "" {
				t.Fatalf("page %d reports no more but carries a cursor", pageIndex)
			}
			break
		}
		if page.NextCursor == "" || page.NextCursor == cursor {
			t.Fatalf("page %d has_more without a fresh cursor", pageIndex)
		}
		cursor = page.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("distinct tools=%d want 5", len(seen))
	}
	if len(sizes) != 3 || sizes[0] != 2 || sizes[1] != 2 || sizes[2] != 1 {
		t.Fatalf("page sizes=%v want [2 2 1]", sizes)
	}
	detailLog(t, "pages=3 first=2 second=2 last=1 distinct=5")
}

func TestDetailCursorRejectsMismatchAndTamper(t *testing.T) {
	defs := make([]capability.ToolDefinition, 0, 4)
	for i := 0; i < 4; i++ {
		defs = append(defs, detailTool(fmt.Sprintf("tool-cursor-%02d", i), "周报", "生成"))
	}
	manager, _ := detailManager(t, defs, false)
	first, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 2, Budget: 8192})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if first.NextCursor == "" {
		t.Fatal("first page has no cursor")
	}
	if _, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "读取周报", Limit: 2, Budget: 8192, Cursor: first.NextCursor}); !errors.Is(err, capability.ErrDetailCursorInvalid) {
		t.Fatalf("other query err=%v want ErrDetailCursorInvalid", err)
	}
	if _, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 2, Budget: 8192, Cursor: "!!!"}); !errors.Is(err, capability.ErrDetailCursorInvalid) {
		t.Fatalf("garbage cursor err=%v want ErrDetailCursorInvalid", err)
	}
	wrongVersion := base64.RawURLEncoding.EncodeToString([]byte(`{"v":9,"offset":0,"digest":"sha256:00"}`))
	if _, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 2, Budget: 8192, Cursor: wrongVersion}); !errors.Is(err, capability.ErrDetailCursorInvalid) {
		t.Fatalf("wrong version err=%v want ErrDetailCursorInvalid", err)
	}
	twoTools := []capability.ToolDefinition{detailTool("tool-cursor-00", "周报", "生成"), detailTool("tool-cursor-01", "周报", "生成")}
	small, _ := detailManager(t, twoTools, false)
	if _, err := small.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 2, Budget: 8192, Cursor: first.NextCursor}); !errors.Is(err, capability.ErrDetailCursorInvalid) {
		t.Fatalf("past-end cursor err=%v want ErrDetailCursorInvalid", err)
	}
	detailLog(t, "cursor_other_query=rejected cursor_version=rejected cursor_garbage=rejected cursor_past_end=rejected")
}

func TestDetailBudgetExhaustionIsClosed(t *testing.T) {
	manager, _ := detailManager(t, []capability.ToolDefinition{detailTool("tool-tiny-00", "周报")}, false)
	page, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "周报", Limit: 8, Budget: 10})
	if !errors.Is(err, capability.ErrDetailBudgetExhausted) {
		t.Fatalf("err=%v want ErrDetailBudgetExhausted", err)
	}
	if len(page.Views) != 0 || len(page.Selected) != 0 {
		t.Fatalf("exhausted page leaked views=%d selected=%d", len(page.Views), len(page.Selected))
	}
	if page.NextCursor != "" {
		t.Fatalf("exhausted page offered cursor %q", page.NextCursor)
	}
	detailLog(t, "exhausted=closed views=0 budget=10")
}

func TestDetailTrimmedPageStaysWithinBudget(t *testing.T) {
	defs := make([]capability.ToolDefinition, 0, 3)
	for i := 0; i < 3; i++ {
		defs = append(defs, detailTool(fmt.Sprintf("tool-trim-%02d", i), "周报", "生成"))
	}
	manager, _ := detailManager(t, defs, false)
	ctx := context.Background()
	page, err := manager.GetModelToolDetails(ctx, detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 3, Budget: 220})
	if err != nil {
		t.Fatalf("trimmed page: %v", err)
	}
	if page.UsedTokens > page.Budget {
		t.Fatalf("used=%d exceeds budget=%d", page.UsedTokens, page.Budget)
	}
	if page.Trimmed < 1 || len(page.Dropped) == 0 {
		t.Fatalf("trimmed=%d dropped=%d want both non-zero", page.Trimmed, len(page.Dropped))
	}
	if !page.HasMore || page.NextCursor == "" {
		t.Fatalf("has_more=%v cursor=%q want a continuation", page.HasMore, page.NextCursor)
	}
	seen := map[string]bool{}
	for _, view := range page.Views {
		seen[view.Name] = true
	}
	cursor := page.NextCursor
	pages := 1
	for cursor != "" {
		next, err := manager.GetModelToolDetails(ctx, detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 3, Budget: 220, Cursor: cursor})
		if err != nil {
			t.Fatalf("continuation page: %v", err)
		}
		pages++
		for _, view := range next.Views {
			seen[view.Name] = true
		}
		cursor = next.NextCursor
	}
	if len(seen) != 3 {
		t.Fatalf("paginated distinct tools=%d want 3", len(seen))
	}
	detailLog(t, fmt.Sprintf("trimmed_page views=%d used_within_budget=true dropped>=1 has_more=true pages=%d", len(page.Views), pages))
}

func TestDetailDuplicateRequestIsIdempotent(t *testing.T) {
	defs := make([]capability.ToolDefinition, 0, 4)
	for i := 0; i < 4; i++ {
		defs = append(defs, detailTool(fmt.Sprintf("tool-dup-%02d", i), "周报", "生成"))
	}
	manager, store := detailManager(t, defs, true)
	request := capability.DetailRequest{Query: "生成周报", Limit: 2, Budget: 8192}
	first, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), request)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), request)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if strings.Join(first.Names(), ",") != strings.Join(second.Names(), ",") {
		t.Fatalf("names drifted: %v vs %v", first.Names(), second.Names())
	}
	if first.UsedTokens != second.UsedTokens || first.NextCursor != second.NextCursor || first.HasMore != second.HasMore || first.Trimmed != second.Trimmed {
		t.Fatalf("duplicate request drifted: %+v vs %+v", first, second)
	}
	if first.SchemaReads != int64(len(first.Views)) || second.SchemaReads != int64(len(second.Views)) {
		t.Fatalf("per-call schema reads=%d/%d want %d/%d", first.SchemaReads, second.SchemaReads, len(first.Views), len(second.Views))
	}
	if store.Reads() != int64(2*len(first.Views)) {
		t.Fatalf("store reads=%d want %d", store.Reads(), 2*len(first.Views))
	}
	detailLog(t, fmt.Sprintf("repeat_equal=true reads_per_call=%d", first.SchemaReads))
}

func TestDetailCancelAndRestart(t *testing.T) {
	defs := make([]capability.ToolDefinition, 0, 3)
	for i := 0; i < 3; i++ {
		defs = append(defs, detailTool(fmt.Sprintf("tool-restart-%02d", i), "周报", "生成"))
	}
	manager, store := detailManager(t, defs, true)
	first, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 1, Budget: 8192})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.NextCursor == "" {
		t.Fatal("first page has no cursor")
	}
	before := store.Reads()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	canceledPage, err := manager.GetModelToolDetails(canceled, detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 1, Budget: 8192})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled err=%v want context.Canceled", err)
	}
	if len(canceledPage.Views) != 0 {
		t.Fatalf("canceled page returned %d views", len(canceledPage.Views))
	}
	if store.Reads() != before {
		t.Fatalf("canceled call changed schema reads %d -> %d", before, store.Reads())
	}
	restarted, _ := detailManager(t, defs, false)
	restartedPage, err := restarted.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 1, Budget: 8192, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("restart continuation: %v", err)
	}
	if len(restartedPage.Views) != 1 || restartedPage.Views[0].Name != "restart-01" {
		t.Fatalf("restart page names=%v want [restart-01]", restartedPage.Names())
	}
	detailLog(t, "cancel=passed_through store_untouched=true restart_page_equal=true")
}

func TestDetailRejectsInvalidRequests(t *testing.T) {
	defs := make([]capability.ToolDefinition, 0, 14)
	for i := 0; i < 14; i++ {
		defs = append(defs, detailTool(fmt.Sprintf("tool-limit-%02d", i), "周报", "生成"))
	}
	registry := taskGateRegistry(t, defs...)
	withoutSelector := capability.NewToolExposureManager(registry, nil)
	if _, err := withoutSelector.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报"}); !errors.Is(err, capability.ErrDetailRequestInvalid) {
		t.Fatalf("selector-less manager err=%v want ErrDetailRequestInvalid", err)
	}
	manager := capability.NewToolExposureManager(registry, nil).WithTaskSelector(capability.NewTaskSelector(registry, nil))
	if _, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: -1}); !errors.Is(err, capability.ErrDetailRequestInvalid) {
		t.Fatalf("negative limit err=%v want ErrDetailRequestInvalid", err)
	}
	if _, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Budget: -1}); !errors.Is(err, capability.ErrDetailRequestInvalid) {
		t.Fatalf("negative budget err=%v want ErrDetailRequestInvalid", err)
	}
	clamped, err := manager.GetModelToolDetails(context.Background(), detailInvocation(), capability.DetailRequest{Query: "生成周报", Limit: 99, Budget: 100000})
	if err != nil {
		t.Fatalf("clamped limit: %v", err)
	}
	if len(clamped.Views) != capability.MaxDetailTopK {
		t.Fatalf("clamped views=%d want %d", len(clamped.Views), capability.MaxDetailTopK)
	}
	detailLog(t, fmt.Sprintf("no_selector=rejected negative_limit=rejected negative_budget=rejected clamp=%d", len(clamped.Views)))
}
