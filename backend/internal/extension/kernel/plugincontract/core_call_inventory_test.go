package plugincontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const coreCallGoldenPath = "testdata/core-call-inventory.json"

type routeSnapshotFile struct {
	Routes struct {
		Entries []RouteRef `json:"entries"`
	} `json:"routes"`
}

type coreCallGolden struct {
	SchemaVersion int              `json:"schemaVersion"`
	Routes        int              `json:"routes"`
	Mapped        int              `json:"mapped"`
	Unmapped      int              `json:"unmapped"`
	Domains       []DomainCoverage `json:"domains"`
	HandlerKinds  map[string]int   `json:"handlerKinds"`
	SideEffects   map[string]int   `json:"sideEffects"`
	UIHosts       []UIHostCoverage `json:"uiHosts"`
	Digest        string           `json:"digest"`
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "..", "..")
}

func loadRouteSnapshot(t *testing.T) []RouteRef {
	t.Helper()
	path := filepath.Join(repoRootForTest(t), "artifacts", "rsi", "baseline", "final-1.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read route snapshot %s: %v", path, err)
	}
	var snapshot routeSnapshotFile
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode route snapshot %s: %v", path, err)
	}
	if len(snapshot.Routes.Entries) == 0 {
		t.Fatalf("route snapshot %s has no entries", path)
	}
	return snapshot.Routes.Entries
}

func loadCoreCallSources(t *testing.T) *CoreCallSources {
	t.Helper()
	sources, err := LoadCoreCallSources(filepath.Join(repoRootForTest(t), "backend"))
	if err != nil {
		t.Fatalf("LoadCoreCallSources: %v", err)
	}
	return sources
}

func buildGolden(inventory CoreCallInventory, rows []CoreCallRow, hosts []UIHostCoverage) coreCallGolden {
	kinds := map[string]int{}
	effects := map[string]int{}
	for _, row := range rows {
		kinds[row.HandlerKind]++
		for _, effect := range row.SideEffects {
			effects[effect]++
		}
	}
	return coreCallGolden{
		SchemaVersion: 1,
		Routes:        inventory.Routes,
		Mapped:        inventory.Mapped,
		Unmapped:      inventory.Unmapped,
		Domains:       inventory.Domains,
		HandlerKinds:  kinds,
		SideEffects:   effects,
		UIHosts:       hosts,
		Digest:        inventory.Digest,
	}
}

func TestCoreCallInventoryCoversEveryBusinessDomain(t *testing.T) {
	routes := loadRouteSnapshot(t)
	sources := loadCoreCallSources(t)
	started := time.Now()
	inventory := BuildCoreCallInventory(routes, sources)
	elapsed := time.Since(started)
	if inventory.Routes != len(routes) {
		t.Fatalf("inventory routes=%d, snapshot routes=%d", inventory.Routes, len(routes))
	}
	if inventory.Routes != 1201 {
		t.Fatalf("route surface drifted: %d routes, baseline pins 1201", inventory.Routes)
	}
	if inventory.Unmapped != 0 {
		for _, row := range inventory.UnmappedRoutes {
			t.Logf("unmapped %s %s handler=%s reason=%s", row.Method, row.Route, row.Handler, row.Reason)
		}
		t.Fatalf("inventory unmapped=%d, want 0", inventory.Unmapped)
	}
	if !inventory.FullCoverage() {
		t.Fatalf("FullCoverage()=false with unmapped=%d domains=%d", inventory.Unmapped, len(inventory.Domains))
	}
	if len(inventory.Domains) != 46 {
		t.Fatalf("business domains=%d, want 46", len(inventory.Domains))
	}
	for _, domain := range inventory.Domains {
		if domain.Routes != domain.Mapped {
			t.Fatalf("domain %s routes=%d mapped=%d", domain.Domain, domain.Routes, domain.Mapped)
		}
	}
	if sources.Files < 3000 || sources.FunctionsRead < 20000 || len(sources.Registrations) < 1000 {
		t.Fatalf("source surface too small: files=%d functions=%d registrations=%d", sources.Files, sources.FunctionsRead, len(sources.Registrations))
	}
	t.Logf("routes=%d mapped=%d domains=%d files=%d functions=%d registrations=%d elapsed=%s",
		inventory.Routes, inventory.Mapped, len(inventory.Domains), sources.Files, sources.FunctionsRead, len(sources.Registrations), elapsed)
	if elapsed > 30*time.Second {
		t.Fatalf("inventory build took %s, want under 30s", elapsed)
	}
}

func TestCoreCallInventoryMatchesGolden(t *testing.T) {
	routes := loadRouteSnapshot(t)
	sources := loadCoreCallSources(t)
	inventory := BuildCoreCallInventory(routes, sources)
	rows := make([]CoreCallRow, 0, len(routes))
	for _, route := range routes {
		rows = append(rows, resolveRoute(route, sources))
	}
	hosts, err := BuildUIHostAudit(repoRootForTest(t), routes)
	if err != nil {
		t.Fatalf("BuildUIHostAudit: %v", err)
	}
	rendered, err := json.MarshalIndent(buildGolden(inventory, rows, hosts), "", "  ")
	if err != nil {
		t.Fatalf("marshal golden: %v", err)
	}
	rendered = append(rendered, '\n')
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(coreCallGoldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(coreCallGoldenPath, rendered, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("rewrote %s (%d bytes)", coreCallGoldenPath, len(rendered))
		return
	}
	expected, err := os.ReadFile(coreCallGoldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", coreCallGoldenPath, err)
	}
	if string(expected) != string(rendered) {
		t.Fatalf("core call inventory drifted from golden %s\n--- expected ---\n%s\n--- actual ---\n%s", coreCallGoldenPath, expected, rendered)
	}
}

func TestCoreCallInventoryRecordsSideEffects(t *testing.T) {
	routes := loadRouteSnapshot(t)
	sources := loadCoreCallSources(t)
	effects := map[string][]string{}
	for _, route := range routes {
		row := resolveRoute(route, sources)
		if len(row.Calls) == 0 {
			t.Fatalf("row %s %s has no call surface", row.Method, row.Route)
		}
		for _, effect := range row.SideEffects {
			effects[effect] = append(effects[effect], row.Method+" "+row.Route)
		}
	}
	for _, kind := range []string{"db.read", "db.write", "file.write", "stream.write"} {
		if len(effects[kind]) == 0 {
			t.Fatalf("no route records side effect %q", kind)
		}
		sort.Strings(effects[kind])
		t.Logf("side effect %s routes=%d sample=%s", kind, len(effects[kind]), effects[kind][0])
	}
	foundStream := false
	foundUpload := false
	for _, route := range routes {
		row := resolveRoute(route, sources)
		if row.Method == "POST" && row.Route == "/api/artifacts/v1" {
			foundUpload = true
			if len(row.SideEffects) != 1 || row.SideEffects[0] != "db.write" {
				t.Fatalf("artifact upload side effects=%v, want [db.write]", row.SideEffects)
			}
		}
		if row.Method == "GET" && row.Route == "/api/device-mesh/v1/pairing/approvals/stream" {
			foundStream = true
			if len(row.SideEffects) != 1 || row.SideEffects[0] != "stream.write" {
				t.Fatalf("pairing approval stream side effects=%v, want [stream.write]", row.SideEffects)
			}
			if row.HandlerKind != HandlerKindFactory {
				t.Fatalf("pairing approval stream kind=%s, want %s", row.HandlerKind, HandlerKindFactory)
			}
		}
	}
	if !foundStream || !foundUpload {
		t.Fatalf("expected routes missing from snapshot: stream=%v upload=%v", foundStream, foundUpload)
	}
}

func TestSideEffectTokens(t *testing.T) {
	cases := []struct{ target, want string }{
		{"db.Insert", "db.write"},
		{"tx.Commit", "db.write"},
		{"store.Query", "db.read"},
		{"os.WriteFile", "file.write"},
		{"exec.Command", "process.exec"},
		{"c.Writer.Flush", "stream.write"},
		{"c.JSON", ""},
		{"Insert", "db.write"},
	}
	for _, testCase := range cases {
		if got := sideEffectFor(testCase.target); got != testCase.want {
			t.Fatalf("sideEffectFor(%q)=%q, want %q", testCase.target, got, testCase.want)
		}
	}
}

func TestCoreCallInventoryMarksUnknownEntries(t *testing.T) {
	sources := &CoreCallSources{
		Functions: map[string]*FunctionSurface{
			"internal/demo::Handler.List":  {Name: "Handler.List", File: "internal/demo/handler.go", Calls: []CallSite{{Target: "store.Query"}}},
			"internal/demo::Handler.Empty": {Name: "Handler.Empty", File: "internal/demo/handler.go"},
		},
		Registrations: []RouteRegistration{
			{Method: "GET", Path: "/api/demo/items", Package: "demo", PackageDir: "internal/demo", PathLiteral: true, Owner: "Register", HandlerText: "(func)"},
			{Method: "GET", Path: "/api/demo/items", Package: "demo", PackageDir: "internal/demo", PathLiteral: true, Owner: "Register", HandlerText: "(func)"},
		},
	}
	routes := []RouteRef{
		{Method: "GET", Path: "/api/demo/items", Handler: "github.com/u-ai/backend/internal/demo.(*Handler).List-fm"},
		{Method: "GET", Path: "/api/demo/items", Handler: "github.com/u-ai/backend/internal/demo.(*Handler).Missing-fm"},
		{Method: "GET", Path: "/api/demo/items", Handler: "github.com/u-ai/backend/internal/demo.(*Handler).Empty-fm"},
		{Method: "GET", Path: "/api/demo/items", Handler: "example.com/other.Handler"},
		{Method: "GET", Path: "/api/demo/items", Handler: "main.registerDemo.func1"},
	}
	byHandler := map[string]CoreCallRow{}
	for _, route := range routes {
		byHandler[route.Handler] = resolveRoute(route, sources)
	}
	if row := byHandler["github.com/u-ai/backend/internal/demo.(*Handler).List-fm"]; row.Coverage != CoverageMapped || row.HandlerKind != HandlerKindMethod {
		t.Fatalf("method row=%+v", row)
	}
	if row := byHandler["github.com/u-ai/backend/internal/demo.(*Handler).Missing-fm"]; row.Coverage != CoverageUnmapped || row.Reason != ReasonHandlerMissing {
		t.Fatalf("missing row=%+v", row)
	}
	if row := byHandler["github.com/u-ai/backend/internal/demo.(*Handler).Empty-fm"]; row.Coverage != CoverageUnmapped || row.Reason != ReasonCallSurfaceEmpty {
		t.Fatalf("empty surface row=%+v", row)
	}
	if row := byHandler["example.com/other.Handler"]; row.Coverage != CoverageUnmapped || row.Reason != ReasonHandlerUnparsed {
		t.Fatalf("external handler row=%+v", row)
	}
	if row := byHandler["main.registerDemo.func1"]; row.Coverage != CoverageUnmapped || row.Reason != ReasonRegistrationMissing {
		t.Fatalf("main closure row=%+v", row)
	}
	inventory := BuildCoreCallInventory(routes, sources)
	if inventory.Unmapped != 4 || inventory.Mapped != 1 {
		t.Fatalf("inventory unmapped=%d mapped=%d", inventory.Unmapped, inventory.Mapped)
	}
	if inventory.FullCoverage() {
		t.Fatalf("FullCoverage() must be false while unknown entries exist")
	}
	if inventory.Digest == "" {
		t.Fatalf("digest must be recorded even when coverage is incomplete")
	}
}

func TestCoreCallInventoryRejectsNilSources(t *testing.T) {
	inventory := BuildCoreCallInventory([]RouteRef{{Method: "get", Path: "/api/demo", Handler: "main.demo"}}, nil)
	if inventory.Routes != 1 || inventory.Unmapped != 1 {
		t.Fatalf("inventory=%+v", inventory)
	}
	row := inventory.UnmappedRoutes[0]
	if row.Method != "GET" || row.Reason != ReasonHandlerMissing {
		t.Fatalf("row=%+v", row)
	}
	if inventory.FullCoverage() {
		t.Fatalf("FullCoverage() must be false without sources")
	}
	empty := BuildCoreCallInventory(nil, nil)
	if empty.Routes != 0 || empty.FullCoverage() {
		t.Fatalf("empty inventory=%+v", empty)
	}
}

func TestBuildCoreCallInventoryKeepsInputsAndOrder(t *testing.T) {
	sources := &CoreCallSources{Functions: map[string]*FunctionSurface{
		"internal/demo::Handler.List": {Name: "Handler.List", File: "internal/demo/handler.go", Calls: []CallSite{{Target: "store.Query", SideEffect: "db.read"}}},
	}}
	routes := []RouteRef{
		{Method: "GET", Path: "/api/zeta/items", Handler: "github.com/u-ai/backend/internal/demo.(*Handler).List-fm"},
		{Method: "GET", Path: "/api/alpha/items", Handler: "example.com/other.Handler"},
	}
	inventory := BuildCoreCallInventory(routes, sources)
	if routes[0].Path != "/api/zeta/items" || routes[1].Handler != "example.com/other.Handler" {
		t.Fatalf("input routes mutated: %+v", routes)
	}
	if len(inventory.Domains) != 2 {
		t.Fatalf("domains=%+v", inventory.Domains)
	}
	if inventory.Domains[0].Domain != "alpha" || inventory.Domains[1].Domain != "demo" {
		t.Fatalf("domains must be sorted by name: %+v", inventory.Domains)
	}
	if inventory.Mapped != 1 || inventory.Unmapped != 1 || inventory.FullCoverage() {
		t.Fatalf("inventory mapped=%d unmapped=%d full=%v", inventory.Mapped, inventory.Unmapped, inventory.FullCoverage())
	}
	mappedOnly := BuildCoreCallInventory(routes[:1], sources)
	if !mappedOnly.FullCoverage() {
		t.Fatalf("FullCoverage()=false for fully mapped synthetic input")
	}
}

func TestSplitInternalHandlerForms(t *testing.T) {
	cases := []struct {
		handler    string
		wantDir    string
		wantName   string
		wantParsed bool
	}{
		{"github.com/u-ai/backend/internal/system.(*Handler).About-fm", "internal/system", "Handler.About", true},
		{"github.com/u-ai/backend/internal/devicemesh/server.RegisterCloudRoutes.func12", "internal/devicemesh/server", "RegisterCloudRoutes.func12", true},
		{"github.com/u-ai/backend/pkg/sse.SSEHandler", "pkg/sse", "SSEHandler", true},
		{"github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v1.RegisterUserRoutes.func1", "internal/desktoppet/runtime/protocol/v1", "RegisterUserRoutes.func1", true},
		{"main.setupRouter.func3", "", "", false},
		{"example.com/other.(*Handler).List-fm", "", "", false},
		{"github.com/u-ai/backend/internal/broken", "", "", false},
	}
	for _, testCase := range cases {
		dir, name, parsed := splitInternalHandler(testCase.handler)
		if parsed != testCase.wantParsed || dir != testCase.wantDir || name != testCase.wantName {
			t.Fatalf("splitInternalHandler(%q)=(%q,%q,%v), want (%q,%q,%v)", testCase.handler, dir, name, parsed, testCase.wantDir, testCase.wantName, testCase.wantParsed)
		}
	}
}

func TestLoadCoreCallSourcesWalksOnlyProductionGoFiles(t *testing.T) {
	root := t.TempDir()
	write := func(relative, contents string) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	write("internal/demo/runtime/handler.go", "package runtime\n\nfunc Handle() { println(1) }\n")
	write("internal/demo/handler.go", "package demo\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc Register(router *gin.Engine) {\n\trouter.GET(\"/api/demo\", Handle)\n}\n\nfunc Handle(c *gin.Context) { c.JSON(200, gin.H{}) }\n")
	write("internal/demo/handler_test.go", "package demo\n\nfunc TestIgnored() {}\n")
	write("runtime/ignored.go", "package runtime\n\nfunc Ignored() {}\n")
	write("vendor/thirdparty/thing.go", "package thirdparty\n\nfunc Thing() {}\n")
	sources, err := LoadCoreCallSources(root)
	if err != nil {
		t.Fatalf("LoadCoreCallSources: %v", err)
	}
	if sources.Files != 2 {
		t.Fatalf("files=%d, want 2 (test, root runtime and vendor skipped)", sources.Files)
	}
	if _, ok := sources.FindFunction("internal/demo/runtime", "Handle"); !ok {
		t.Fatalf("nested runtime package must be indexed")
	}
	if _, ok := sources.FindFunction("internal/demo", "Handle"); !ok {
		t.Fatalf("internal/demo Handle must be indexed")
	}
	if _, ok := sources.FindFunction("runtime", "Ignored"); ok {
		t.Fatalf("root runtime directory must be skipped")
	}
	if len(sources.Registrations) != 1 || sources.Registrations[0].Path != "/api/demo" {
		t.Fatalf("registrations=%+v", sources.Registrations)
	}
}

func TestLoadCoreCallSourcesFailsOnBrokenSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "broken.go")
	if err := os.WriteFile(path, []byte("package broken\n\nfunc Broken( {\n"), 0o644); err != nil {
		t.Fatalf("write broken source: %v", err)
	}
	_, err := LoadCoreCallSources(root)
	if err == nil {
		t.Fatalf("LoadCoreCallSources must fail on unparsable source")
	}
	if !strings.Contains(err.Error(), "broken.go") {
		t.Fatalf("error must name the offending file: %v", err)
	}
	_, missingErr := LoadCoreCallSources(filepath.Join(root, "absent"))
	if missingErr == nil {
		t.Fatalf("LoadCoreCallSources must fail on missing root")
	}
}

func BuildUIHostAudit(root string, routes []RouteRef) ([]UIHostCoverage, error) {
	calls, err := LoadUIHostCalls(root)
	if err != nil {
		return nil, err
	}
	return BuildUIHostCoverage(calls, routes), nil
}

func TestUIHostAuditTracksWebAndFlutter(t *testing.T) {
	routes := loadRouteSnapshot(t)
	root := repoRootForTest(t)
	calls, err := LoadUIHostCalls(root)
	if err != nil {
		t.Fatalf("LoadUIHostCalls: %v", err)
	}
	if len(calls) == 0 {
		t.Fatalf("no ui host calls discovered")
	}
	coverage := BuildUIHostCoverage(calls, routes)
	if len(coverage) != 2 {
		t.Fatalf("hosts=%d, want web and flutter", len(coverage))
	}
	for _, entry := range coverage {
		if entry.Files == 0 || entry.Calls == 0 {
			t.Fatalf("host %s has no tracked calls: %+v", entry.Host, entry)
		}
		if entry.Linked == 0 {
			t.Fatalf("host %s links to no backend route", entry.Host)
		}
		if entry.Digest == "" {
			t.Fatalf("host %s digest missing", entry.Host)
		}
		t.Logf("host=%s files=%d calls=%d linked=%d unlinked=%d undigested=%d sample=%v",
			entry.Host, entry.Files, entry.Calls, entry.Linked, entry.Unlinked, len(entry.UnlinkedPaths), firstFew(entry.UnlinkedPaths, 5))
	}
	seen := map[string]bool{}
	for _, entry := range coverage {
		seen[entry.Host] = true
	}
	if !seen[UIHostWeb] || !seen[UIHostFlutter] {
		t.Fatalf("hosts=%v", seen)
	}
}

func firstFew(values []string, count int) []string {
	if len(values) < count {
		return values
	}
	return values[:count]
}

func TestUIHostEndpointNormalization(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"/api/sessions", "/api/sessions"},
		{"/api/sessions?limit=1", "/api/sessions"},
		{"/api/sessions/${session.id}/messages", "/api/sessions/:param/messages"},
		{"/api/devices/$deviceId/presence", "/api/devices/:param/presence"},
		{"/api/artifacts/", "/api/artifacts"},
		{"/api/sessions#fragment", "/api/sessions"},
	}
	for _, testCase := range cases {
		if got := normalizeUIEndpoint(testCase.raw); got != testCase.want {
			t.Fatalf("normalizeUIEndpoint(%q)=%q, want %q", testCase.raw, got, testCase.want)
		}
	}
}

func TestUIHostCoverageMarksUnknownEndpoints(t *testing.T) {
	routes := []RouteRef{
		{Method: "GET", Path: "/api/sessions/:sessionId", Handler: "x"},
		{Method: "ANY", Path: "/api/wasm/*wasmPath", Handler: "y"},
	}
	calls := []UIHostCall{
		{Host: UIHostWeb, File: "front/src/api/sessions.ts", Line: 1, Endpoint: "/api/sessions/:param"},
		{Host: UIHostWeb, File: "front/src/api/sessions.ts", Line: 2, Endpoint: "/api/sessions/:param/ghost"},
		{Host: UIHostFlutter, File: "mobile_app/lib/api.dart", Line: 3, Endpoint: "/api/wasm/blob"},
	}
	coverage := BuildUIHostCoverage(calls, routes)
	if len(coverage) != 2 || coverage[0].Host != UIHostWeb || coverage[1].Host != UIHostFlutter {
		t.Fatalf("coverage=%+v", coverage)
	}
	if coverage[0].Linked != 1 || coverage[0].Unlinked != 1 {
		t.Fatalf("web coverage=%+v", coverage[0])
	}
	if len(coverage[0].UnlinkedPaths) != 1 || coverage[0].UnlinkedPaths[0] != "/api/sessions/:param/ghost" {
		t.Fatalf("web unlinked=%v", coverage[0].UnlinkedPaths)
	}
	if coverage[1].Linked != 1 || coverage[1].Unlinked != 0 {
		t.Fatalf("flutter coverage=%+v", coverage[1])
	}
}

func TestLoadUIHostCallsFailsOnMissingRoot(t *testing.T) {
	if _, err := LoadUIHostCalls(t.TempDir()); err == nil {
		t.Fatalf("LoadUIHostCalls must fail when ui host sources are absent")
	}
}
