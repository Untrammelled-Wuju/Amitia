package manifest_v1

import "testing"

func TestUIPlatformIsolationAllowsSinglePlatform(t *testing.T) {
	manifest := Manifest{Modules: []ModuleMeta{{
		ID: "ui",
		Contributions: []ContributionMeta{{
			ID:   "mobile-page",
			Kind: "ui_page",
			Spec: map[string]any{
				"visibility": map[string]any{"platforms": []any{"android", "ios"}},
				"entry":      map[string]any{"type": "schema_renderer", "schema_path": "ui/mobile.json", "content_hash": "sha256-mobile"},
			},
		}},
	}}}
	report := &ValidationReport{}
	validateUIPlatformIsolation(manifest, report)
	if report.HasErrors() {
		t.Fatalf("single-platform UI should pass: %v", report.Errors)
	}
}

func TestUIPlatformIsolationUsesDesktopCompatibilityFallback(t *testing.T) {
	manifest := Manifest{
		Compatibility: Compatibility{Platforms: []string{"windows-x64", "linux-x64"}},
		Modules: []ModuleMeta{{
			ID: "ui",
			Contributions: []ContributionMeta{{
				ID:   "legacy-desktop-page",
				Kind: "ui_page",
				Spec: map[string]any{
					"entry": map[string]any{"type": "web_page", "path": "ui/index.html", "content_hash": "sha256-page"},
				},
			}},
		}},
	}
	report := &ValidationReport{}
	validateUIPlatformIsolation(manifest, report)
	if report.HasErrors() {
		t.Fatalf("desktop-only compatibility fallback should pass: %v", report.Errors)
	}
}

func TestUIPlatformIsolationRejectsSharedSchema(t *testing.T) {
	manifest := Manifest{Modules: []ModuleMeta{{
		ID: "ui",
		Contributions: []ContributionMeta{
			{
				ID:   "desktop-page",
				Kind: "ui_page",
				Spec: map[string]any{
					"visibility": map[string]any{"platforms": []any{"windows"}},
					"entry":      map[string]any{"type": "schema_renderer", "schema_path": "ui/shared.json", "content_hash": "sha256-shared"},
				},
			},
			{
				ID:   "mobile-page",
				Kind: "ui_page",
				Spec: map[string]any{
					"visibility": map[string]any{"platforms": []any{"android"}},
					"entry":      map[string]any{"type": "schema_renderer", "schema_path": "ui/shared.json", "content_hash": "sha256-shared"},
				},
			},
		},
	}}}
	report := &ValidationReport{}
	validateUIPlatformIsolation(manifest, report)
	if !report.HasErrors() {
		t.Fatal("expected shared desktop/mobile schema to be rejected")
	}
}

func TestUIPlatformIsolationRejectsProviderReuse(t *testing.T) {
	manifest := Manifest{Modules: []ModuleMeta{{
		ID: "ui",
		Contributions: []ContributionMeta{{
			ID:   "page-provider",
			Kind: "ui_provider",
			Spec: map[string]any{
				"providerId": "page-provider",
				"capability": "page.provider",
				"entries": map[string]any{
					"electron_windows": map[string]any{"type": "schema_renderer", "contributionId": "page"},
					"mobile":           map[string]any{"type": "schema_renderer", "contributionId": "page"},
				},
			},
		}},
	}}}
	report := &ValidationReport{}
	validateUIPlatformIsolation(manifest, report)
	if !report.HasErrors() {
		t.Fatal("expected shared desktop/mobile provider contribution to be rejected")
	}
}
