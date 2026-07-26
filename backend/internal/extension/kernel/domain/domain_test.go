package domain

import (
	"context"
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"1.0.0", true},
		{"1.2.3", true},
		{"1.0.0-alpha", true},
		{"1.0.0-alpha.1+build.1", true},
		{"1.0", false},
		{"1.0.0.0", false},
		{"v1.0.0", false},
		{"", false},
	}
	for _, tc := range tests {
		_, err := ParseVersion(tc.input)
		if (err == nil) != tc.valid {
			t.Errorf("ParseVersion(%q) valid=%v, got err=%v", tc.input, tc.valid, err)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	v1, _ := ParseVersion("1.0.0")
	v2, _ := ParseVersion("1.0.1")
	v3, _ := ParseVersion("1.1.0")
	v4, _ := ParseVersion("2.0.0")
	v5, _ := ParseVersion("1.0.0-alpha")
	if v1.Compare(v2) >= 0 {
		t.Errorf("1.0.0 should be < 1.0.1")
	}
	if v2.Compare(v3) >= 0 {
		t.Errorf("1.0.1 should be < 1.1.0")
	}
	if v3.Compare(v4) >= 0 {
		t.Errorf("1.1.0 should be < 2.0.0")
	}
	if v5.Compare(v1) >= 0 {
		t.Errorf("1.0.0-alpha should be < 1.0.0")
	}
}

func TestValidateExtensionID(t *testing.T) {
	valid := []string{
		"com.example/weather",
		"top.untrammelled/amitia-tools",
		"org.open-source/local-memory",
		"local.user/my-ext",
	}
	invalid := []string{
		"",
		"weather",
		"com.example.weather",
		"com.example/Weather",
		"COM.EXAMPLE/weather",
		"com.example/weather@1.0.0",
	}
	for _, id := range valid {
		if err := ValidateExtensionID(ExtensionID(id)); err != nil {
			t.Errorf("expected %s to be valid, got %v", id, err)
		}
	}
	for _, id := range invalid {
		if err := ValidateExtensionID(ExtensionID(id)); err == nil {
			t.Errorf("expected %s to be invalid", id)
		}
	}
}

func TestExtensionDefinitionValidate(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Test"},
		Modules: []ModuleDefinition{
			{
				ID:          "main",
				ExtensionID: "com.example/test",
				Type:        ModuleTypeBuiltin,
				Contributions: []ContributionDefinition{
					{ID: "tool1", ModuleID: "main", Kind: ContributionKindTool},
				},
			},
		},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if _, ok := def.FindModule("main"); !ok {
		t.Errorf("expected to find module main")
	}
}

func TestExtensionDefinitionValidateMissingModule(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Test"},
		Modules:         []ModuleDefinition{},
	}
	if err := def.Validate(); err == nil {
		t.Errorf("expected error for empty modules")
	}
}

func TestExtensionDefinitionValidateDuplicateModule(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID:              "com.example/test",
		Version:         v,
		ManifestVersion: 2,
		Name:            LocalizedText{Default: "Test"},
		Modules: []ModuleDefinition{
			{ID: "main", ExtensionID: "com.example/test", Type: ModuleTypeBuiltin},
			{ID: "main", ExtensionID: "com.example/test", Type: ModuleTypeBuiltin},
		},
	}
	if err := def.Validate(); err == nil {
		t.Errorf("expected error for duplicate module")
	}
}

func TestDependencyGraphNoCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.Add(DependencyNode{
		ExtensionID: "com.example/a", ModuleID: "main",
		Dependencies: []DependencyDefinition{{Type: DependencyTypeExtension, ID: "com.example/b"}},
	})
	g.Add(DependencyNode{
		ExtensionID: "com.example/b", ModuleID: "main",
	})
	if cycle := g.DetectCycle(); cycle != nil {
		t.Errorf("expected no cycle, got %v", cycle)
	}
	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort: %v", err)
	}
	if len(sorted) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(sorted))
	}
}

func TestDependencyGraphWithCycle(t *testing.T) {
	g := NewDependencyGraph()
	g.Add(DependencyNode{
		ExtensionID: "com.example/a", ModuleID: "main",
		Dependencies: []DependencyDefinition{{Type: DependencyTypeExtension, ID: "com.example/b"}},
	})
	g.Add(DependencyNode{
		ExtensionID: "com.example/b", ModuleID: "main",
		Dependencies: []DependencyDefinition{{Type: DependencyTypeExtension, ID: "com.example/a"}},
	})
	if cycle := g.DetectCycle(); cycle == nil {
		t.Errorf("expected cycle")
	}
	if _, err := g.TopologicalSort(); err == nil {
		t.Errorf("expected error for cycle")
	}
}

func TestInMemoryRepositories(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	def := ExtensionDefinition{
		ID: "com.example/test", Version: v, ManifestVersion: 2,
		Name: LocalizedText{Default: "Test"},
		Modules: []ModuleDefinition{
			{ID: "main", ExtensionID: "com.example/test", Type: ModuleTypeBuiltin},
		},
	}
	defRepo := NewInMemoryDefinitionRepository()
	if err := defRepo.PutExtension(context.Background(), def); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := defRepo.GetExtension(context.Background(), "com.example/test", v)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != def.ID {
		t.Errorf("expected %s, got %s", def.ID, got.ID)
	}
	list, _ := defRepo.ListExtensions(context.Background())
	if len(list) != 1 {
		t.Errorf("expected 1, got %d", len(list))
	}

	instRepo := NewInMemoryInstallationRepository()
	inst := ExtensionInstallation{
		InstallationID: "inst1", ExtensionID: "com.example/test",
		InstalledVersion: v, PackageID: "pkg1",
		InstallationState: InstallationStateInstalled,
		EnablementState:   EnablementEnabled,
	}
	_ = instRepo.PutInstallation(context.Background(), inst)
	gotInst, _ := instRepo.GetInstallation(context.Background(), "com.example/test")
	if gotInst.InstallationID != "inst1" {
		t.Errorf("expected inst1, got %s", gotInst.InstallationID)
	}

	pkgRepo := NewInMemoryPackageRepository()
	pkg := ExtensionPackage{
		PackageID: "pkg1", ExtensionID: "com.example/test", Version: v,
	}
	_ = pkgRepo.PutPackage(context.Background(), pkg)
	gotPkg, _ := pkgRepo.GetPackage(context.Background(), "pkg1")
	if gotPkg.PackageID != "pkg1" {
		t.Errorf("expected pkg1")
	}
	pkgs, _ := pkgRepo.ListPackages(context.Background(), "com.example/test")
	if len(pkgs) != 1 {
		t.Errorf("expected 1 package, got %d", len(pkgs))
	}

	rtRepo := NewInMemoryRuntimeRepository()
	rt := RuntimeInstance{
		InstanceID: "rt1", ExtensionID: "com.example/test",
		ModuleID: "main", RuntimeType: RuntimeTypeBuiltin,
	}
	_ = rtRepo.PutInstance(context.Background(), rt)
	gotRt, _ := rtRepo.GetInstance(context.Background(), "rt1")
	if gotRt.InstanceID != "rt1" {
		t.Errorf("expected rt1")
	}
	rts, _ := rtRepo.ListInstances(context.Background(), "com.example/test")
	if len(rts) != 1 {
		t.Errorf("expected 1 instance")
	}
}

func TestVersionIsCompatible(t *testing.T) {
	v1, _ := ParseVersion("1.0.0")
	v2, _ := ParseVersion("1.5.0")
	v3, _ := ParseVersion("2.0.0")
	if !v1.IsCompatibleWith(v2) {
		t.Errorf("1.0.0 should be compatible with 1.5.0")
	}
	if v1.IsCompatibleWith(v3) {
		t.Errorf("1.0.0 should not be compatible with 2.0.0")
	}
	if !v1.IsCompatibleWith(v1) {
		t.Errorf("1.0.0 should be compatible with itself")
	}
}
