package runtime

import "testing"

func TestADRPrimaryRoute(t *testing.T) {
	if ADR.DecisionID != "ADR-001-RUNTIME" {
		t.Fatalf("expected ADR-001-RUNTIME, got %s", ADR.DecisionID)
	}
	if ADR.Status != "accepted" {
		t.Fatalf("expected accepted, got %s", ADR.Status)
	}
	if ADR.PrimaryRoute.Name != "Independent Node.js Subprocess" {
		t.Fatalf("expected Independent Node.js Subprocess, got %s", ADR.PrimaryRoute.Name)
	}
	required := []string{
		"independent_node_subprocess",
		"typescript_sdk",
		"controlled_module_loader",
		"internal_json_rpc",
		"host_api_gateway",
		"singleton_per_module",
	}
	componentSet := make(map[string]bool)
	for _, c := range ADR.PrimaryRoute.Components {
		componentSet[c] = true
	}
	for _, r := range required {
		if !componentSet[r] {
			t.Fatalf("missing required component %s", r)
		}
	}
}

func TestADRRejectedRoutes(t *testing.T) {
	rejected := []string{
		"Electron Renderer 执行插件",
		"Electron Main Process require",
		"动态 Go Plugin",
		"仅使用 WASM 作为全部插件能力",
		"宿主进程内弱隔离 JavaScript VM",
	}
	rejectedSet := make(map[string]bool)
	for _, r := range ADR.RejectedRoutes {
		rejectedSet[r.Name] = true
	}
	for _, r := range rejected {
		if !rejectedSet[r] {
			t.Fatalf("missing rejected route %s", r)
		}
	}
}

func TestADRSpecifiesSupplementaryRoutes(t *testing.T) {
	required := []string{"Task Runtime", "Trusted Service Runtime", "WASM Runtime", "Restricted UI Runtime"}
	routeSet := make(map[string]bool)
	for _, r := range ADR.SupplementaryRoutes {
		routeSet[r.Name] = true
	}
	for _, r := range required {
		if !routeSet[r] {
			t.Fatalf("missing supplementary route %s", r)
		}
	}
}

func TestDefaultBootstrapSequence(t *testing.T) {
	seq := DefaultBootstrapSequence()
	required := []string{
		"process_start",
		"read_bootstrap_spec",
		"open_rpc_channel",
		"authenticate_session",
		"verify_definition",
		"initialize_sdk",
		"load_entry_module",
		"call_activate",
		"report_ready",
	}
	stepSet := make(map[string]bool)
	for _, s := range seq.Steps {
		stepSet[s.Name] = true
	}
	for _, r := range required {
		if !stepSet[r] {
			t.Fatalf("missing bootstrap step %s", r)
		}
	}
}

func TestDefaultModuleLoaderPolicyDeniesDangerous(t *testing.T) {
	policy := DefaultModuleLoaderPolicy()
	denied := []string{"child_process", "cluster", "dgram", "vm", "worker_threads"}
	deniedSet := make(map[string]bool)
	for _, d := range policy.DeniedModules {
		deniedSet[d] = true
	}
	for _, d := range denied {
		if !deniedSet[d] {
			t.Fatalf("expected denied module %s", d)
		}
	}
	if !policy.DenyAbsolutePaths {
		t.Fatal("expected deny absolute paths")
	}
	if !policy.DenyNativeModules {
		t.Fatal("expected deny native modules")
	}
	if !policy.DenyElectronAccess {
		t.Fatal("expected deny electron access")
	}
	if !policy.DenyGoIPCAccess {
		t.Fatal("expected deny Go IPC access")
	}
	if !policy.DenyShellExecution {
		t.Fatal("expected deny shell execution")
	}
	if !policy.DenyDynamicDownload {
		t.Fatal("expected deny dynamic download")
	}
}

func TestDefaultResourceLimits(t *testing.T) {
	limits := DefaultResourceLimits()
	if limits.MaxMemoryMB <= 0 {
		t.Fatal("expected positive memory limit")
	}
	if limits.MaxConcurrentCalls <= 0 {
		t.Fatal("expected positive concurrent call limit")
	}
	if limits.MaxMessageSizeKB <= 0 {
		t.Fatal("expected positive message size limit")
	}
}

func TestHostAPIReplacementsCoverCritical(t *testing.T) {
	replacements := HostAPIReplacements()
	categories := make(map[string]bool)
	for _, r := range replacements {
		categories[r.Category] = true
	}
	required := []string{"network", "file", "storage", "secret", "process", "schedule"}
	for _, r := range required {
		if !categories[r] {
			t.Fatalf("missing Host API replacement for %s", r)
		}
	}
	for _, r := range replacements {
		if len(r.DeniedNative) == 0 {
			t.Fatalf("expected denied native modules for %s", r.Category)
		}
	}
}

func TestThreatModelComplete(t *testing.T) {
	threats := ThreatModel()
	if len(threats) < 10 {
		t.Fatalf("expected at least 10 threats, got %d", len(threats))
	}
	categories := make(map[ThreatCategory]bool)
	for _, th := range threats {
		categories[th.Category] = true
		if th.Mitigation == "" {
			t.Fatalf("threat %s missing mitigation", th.ID)
		}
	}
}

func TestSecurityBoundariesCoverThreats(t *testing.T) {
	boundaries := SecurityBoundaries()
	if len(boundaries) < 5 {
		t.Fatalf("expected at least 5 boundaries, got %d", len(boundaries))
	}
	threatsCovered := make(map[string]bool)
	for _, b := range boundaries {
		for _, tID := range b.Threats {
			threatsCovered[tID] = true
		}
	}
	threats := ThreatModel()
	for _, th := range threats {
		if !threatsCovered[th.ID] {
			t.Fatalf("threat %s not covered by any security boundary", th.ID)
		}
	}
}

func TestDevProdDifferencesComplete(t *testing.T) {
	diffs := DevProdDifferences()
	categories := make(map[string]bool)
	for _, d := range diffs {
		categories[d.Category] = true
	}
	required := []string{"source_maps", "hot_reload", "debug_port", "log_level", "secret_display", "trust_level"}
	for _, r := range required {
		if !categories[r] {
			t.Fatalf("missing dev/prod difference for %s", r)
		}
	}
}

func TestPerformanceBaselinesExist(t *testing.T) {
	baselines := PerformanceBaselines()
	if len(baselines) == 0 {
		t.Fatal("expected at least one performance baseline")
	}
	categories := make(map[string]bool)
	for _, b := range baselines {
		categories[b.Category] = true
		if b.Target == "" {
			t.Fatalf("baseline %s missing target", b.Metric)
		}
	}
	for _, required := range []string{"bootstrap", "invocation", "memory", "rpc", "host_api"} {
		if !categories[required] {
			t.Fatalf("missing performance baseline for %s", required)
		}
	}
}
