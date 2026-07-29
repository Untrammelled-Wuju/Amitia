package capability

import (
	"context"
	"testing"
)

func TestToolRegistryIsolation(t *testing.T) {
	reg := NewToolRegistry()

	if reg == nil {
		t.Fatal("expected non-nil registry")
	}

	count := reg.Count()
	if count != 0 {
		t.Fatalf("expected empty registry, got %d tools", count)
	}

	all := reg.List(context.Background(), ToolFilter{})
	if len(all) != 0 {
		t.Fatalf("expected empty list, got %d", len(all))
	}
}

func TestToolRegistryRegister(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:        BuildToolID(ToolSourceBuiltin, "test", "hello"),
		ModelName: "hello",
		Source:    ToolSourceBuiltin,
		Name:      "hello",
		Enabled:   true,
	}

	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	if reg.Count() != 1 {
		t.Fatalf("expected 1 tool, got %d", reg.Count())
	}

	if err := reg.Register(context.Background(), def); err == nil {
		t.Fatal("expected duplicate error")
	}

	retrieved, ok := reg.Get(context.Background(), def.ID)
	if !ok {
		t.Fatal("expected found")
	}
	if retrieved.Name != "hello" {
		t.Fatalf("expected name hello, got %s", retrieved.Name)
	}
}

func TestToolRegistryUnregister(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:      BuildToolID(ToolSourceInternal, "system", "ping"),
		Source:  ToolSourceInternal,
		Name:    "ping",
		Enabled: true,
	}

	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	if err := reg.Unregister(context.Background(), def.ID); err != nil {
		t.Fatalf("unexpected Unregister error: %v", err)
	}

	if reg.Count() != 0 {
		t.Fatalf("expected empty registry, got %d", reg.Count())
	}

	if err := reg.Unregister(context.Background(), def.ID); err == nil {
		t.Fatal("expected not exists error")
	}
}

func TestToolRegistrySetEnabled(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:      BuildToolID(ToolSourceBuiltin, "test", "toggle"),
		Source:  ToolSourceBuiltin,
		Name:    "toggle",
		Enabled: true,
	}

	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	if err := reg.SetEnabled(context.Background(), def.ID, false); err != nil {
		t.Fatalf("unexpected SetEnabled error: %v", err)
	}

	retrieved, _ := reg.Get(context.Background(), def.ID)
	if retrieved.Enabled {
		t.Fatal("expected disabled")
	}

	if err := reg.SetEnabled(context.Background(), def.ID, true); err != nil {
		t.Fatalf("unexpected SetEnabled error: %v", err)
	}

	retrieved, _ = reg.Get(context.Background(), def.ID)
	if !retrieved.Enabled {
		t.Fatal("expected enabled")
	}
}

func TestBuildToolIDFormat(t *testing.T) {
	cases := []struct {
		source    ToolSource
		namespace string
		name      string
		expected  string
	}{
		{ToolSourceBuiltin, "files", "read", "builtin/files/read"},
		{ToolSourceMCP, "server-uuid", "search", "mcp/server-uuid/search"},
		{ToolSourcePlugin, "com.example.w", "query", "plugin/com.example.w/query"},
		{ToolSourceInternal, "agent-skill", "activate", "internal/agent-skill/activate"},
		{ToolSourceLegacy, "amitia", "get-current-time", "legacy_tool/amitia/get-current-time"},
		{ToolSourceWorkflow, "com.example.ds", "run", "workflow/com.example.ds/run"},
	}

	for _, c := range cases {
		result := BuildToolID(c.source, c.namespace, c.name)
		if result != c.expected {
			t.Errorf("BuildToolID(%s, %s, %s) = %s, expected %s",
				c.source, c.namespace, c.name, result, c.expected)
		}
	}
}

func TestToolRegistryReplace(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:        BuildToolID(ToolSourceBuiltin, "test", "replace"),
		ModelName: "replace_test",
		Source:    ToolSourceBuiltin,
		Name:      "original",
		Enabled:   true,
	}

	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	updated := def
	updated.Name = "updated"
	if err := reg.Replace(context.Background(), updated); err != nil {
		t.Fatalf("unexpected Replace error: %v", err)
	}

	retrieved, ok := reg.Get(context.Background(), def.ID)
	if !ok {
		t.Fatal("expected found after replace")
	}
	if retrieved.Name != "updated" {
		t.Fatalf("expected updated name, got %s", retrieved.Name)
	}
}

func TestToolRegistryReplaceOwnerConflict(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:          "test/tool",
		ExtensionID: "ext-a",
		Source:      ToolSourcePlugin,
		Name:        "test",
	}

	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	conflict := ToolDefinition{
		ID:          "test/tool",
		ExtensionID: "ext-b",
		Source:      ToolSourcePlugin,
		Name:        "conflict",
	}

	if err := reg.Replace(context.Background(), conflict); err == nil {
		t.Fatal("expected owner conflict error")
	}
}

func TestToolRegistryBatchRegister(t *testing.T) {
	reg := NewToolRegistry()

	defs := []ToolDefinition{
		{ID: "batch/1", Source: ToolSourceBuiltin, Name: "b1", Enabled: true},
		{ID: "batch/2", Source: ToolSourceBuiltin, Name: "b2", Enabled: true},
		{ID: "batch/3", Source: ToolSourceBuiltin, Name: "b3", Enabled: true},
	}

	if err := reg.BatchRegister(context.Background(), defs); err != nil {
		t.Fatalf("unexpected BatchRegister error: %v", err)
	}

	if reg.Count() != 3 {
		t.Fatalf("expected 3 tools, got %d", reg.Count())
	}
}

func TestToolRegistryBatchRegisterAtomic(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:     "batch/atomic",
		Source: ToolSourceBuiltin,
		Name:   "existing",
	}

	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	defs := []ToolDefinition{
		{ID: "batch/new1", Source: ToolSourceBuiltin, Name: "new1"},
		{ID: "batch/atomic", Source: ToolSourceBuiltin, Name: "duplicate"},
		{ID: "batch/new2", Source: ToolSourceBuiltin, Name: "new2"},
	}

	if err := reg.BatchRegister(context.Background(), defs); err == nil {
		t.Fatal("expected duplicate error")
	}

	if reg.Count() != 1 {
		t.Fatalf("expected 1 tool (atomic rollback), got %d", reg.Count())
	}
}

func TestToolRegistryUnregisterByOwner(t *testing.T) {
	reg := NewToolRegistry()

	def := ToolDefinition{
		ID:          "owner/test1",
		ExtensionID: "com.test.owner",
		Source:      ToolSourcePlugin,
		Name:        "test1",
		Enabled:     true,
	}

	if err := reg.Register(context.Background(), def); err != nil {
		t.Fatalf("unexpected Register error: %v", err)
	}

	ids, _ := reg.UnregisterByOwner(context.Background(), "extension:com.test.owner")
	if len(ids) != 1 {
		t.Fatalf("expected 1 removed id, got %d", len(ids))
	}

	if reg.Count() != 0 {
		t.Fatalf("expected empty registry, got %d", reg.Count())
	}
}

func TestToolRegistryListBySource(t *testing.T) {
	reg := NewToolRegistry()

	def1 := ToolDefinition{ID: "src/b1", Source: ToolSourceBuiltin, Name: "b1", Enabled: true}
	def2 := ToolDefinition{ID: "src/p1", Source: ToolSourcePlugin, Name: "p1", Enabled: true}

	reg.Register(context.Background(), def1)
	reg.Register(context.Background(), def2)

	builtins := reg.ListBySource(context.Background(), ToolSourceBuiltin)
	if len(builtins) != 1 {
		t.Fatalf("expected 1 builtin, got %d", len(builtins))
	}

	plugins := reg.ListBySource(context.Background(), ToolSourcePlugin)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
}

func TestModelNameFromCapabilityID(t *testing.T) {
	cases := []struct {
		id       CapabilityID
		expected string
	}{
		{"builtin/files/read", "builtin_files__read"},
		{"plugin/com.example.w/query", "plugin_com_example_w__query"},
		{"mcp/server-uuid/search", "mcp_server_uuid__search"},
		{"workflow/com.example.ds/run", "workflow_com_example_ds__run"},
		{"internal/agent-skill/activate", "internal_agent_skill__activate"},
	}

	for _, c := range cases {
		result := ModelNameFromCapabilityID(c.id)
		if result != c.expected {
			t.Errorf("ModelNameFromCapabilityID(%s) = %s, expected %s", c.id, result, c.expected)
		}
	}
}

func TestToolStateVisibleToModel(t *testing.T) {
	s := ToolState{
		Installed:         true,
		ModuleEnabled:     true,
		CapabilityEnabled: true,
		ScopeAllowed:      true,
	}

	if !s.VisibleToModel() {
		t.Fatal("expected visible")
	}

	s.ModuleEnabled = false
	if s.VisibleToModel() {
		t.Fatal("expected not visible when module disabled")
	}
}

func TestToolStateExecutable(t *testing.T) {
	s := ToolState{
		Installed:         true,
		ModuleEnabled:     true,
		CapabilityEnabled: true,
		ScopeAllowed:      true,
		PermissionGranted: true,
		RuntimeReady:      true,
		DependencyReady:   true,
		Health:            HealthReady,
	}

	if !s.Executable() {
		t.Fatal("expected executable")
	}

	s.PermissionGranted = false
	if s.Executable() {
		t.Fatal("expected not executable when permission denied")
	}
}

func TestDefaultAvailabilityEvaluator(t *testing.T) {
	e := &DefaultAvailabilityEvaluator{}

	tool := ToolDefinition{
		ID:   "test/tool",
		Name: "test",
		State: ToolState{
			Installed:         true,
			ModuleEnabled:     true,
			CapabilityEnabled: false,
			ScopeAllowed:      true,
			PermissionGranted: true,
			RuntimeReady:      true,
			DependencyReady:   true,
			Health:            HealthReady,
		},
	}

	inv := ToolInvocationContext{InvocationID: "inv-1"}
	result := e.Evaluate(context.Background(), tool, inv)

	if result.Visible {
		t.Fatal("expected not visible when capability disabled")
	}
	if len(result.Reasons) == 0 {
		t.Fatal("expected reasons")
	}
}

func TestToolDefinitionComputedState(t *testing.T) {
	td := ToolDefinition{
		ID:      "test/computed",
		Enabled: true,
	}

	state := td.ComputedState()
	if !state.Executable() {
		t.Fatal("expected computed state to be executable")
	}
}

func TestCapabilitySourceToToolSource(t *testing.T) {
	cases := []struct {
		capSrc  CapabilitySource
		toolSrc ToolSource
	}{
		{CapabilitySourceBuiltin, ToolSourceBuiltin},
		{CapabilitySourceMCP, ToolSourceMCP},
		{CapabilitySourceLegacy, ToolSourceLegacy},
	}

	for _, c := range cases {
		result := CapabilitySourceToToolSource(c.capSrc)
		if result != c.toolSrc {
			t.Errorf("CapabilitySourceToToolSource(%s) = %s, expected %s", c.capSrc, result, c.toolSrc)
		}
	}
}

func TestToolDefinitionOwner(t *testing.T) {
	b := ToolDefinition{Source: ToolSourceBuiltin, Name: "test"}
	if b.Owner().OwnerType != OwnerTypeSystem {
		t.Fatal("expected system owner for builtin")
	}

	p := ToolDefinition{Source: ToolSourcePlugin, ExtensionID: "com.example", Name: "test"}
	if p.Owner().OwnerType != OwnerTypeExtension {
		t.Fatal("expected extension owner for plugin")
	}
	if p.Owner().ExtensionID != "com.example" {
		t.Fatalf("expected extension id com.example, got %s", p.Owner().ExtensionID)
	}
}
