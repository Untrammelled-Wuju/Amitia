package skill

import (
	"context"
	"strings"
	"testing"
)

func TestClaudeCodeAdapter_Detect_EmptyExtra(t *testing.T) {
	adapter := NewClaudeCodeProfileAdapter()
	parsed := ParsedSkill{ExtraFrontmatter: map[string]interface{}{}}
	pkg := SkillPackageView{Parsed: parsed}
	detection, err := adapter.Detect(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(detection.Detected) != 0 {
		t.Fatalf("expected 0 detected profiles, got %d", len(detection.Detected))
	}
}

func TestClaudeCodeAdapter_Detect_WhenToUse(t *testing.T) {
	adapter := NewClaudeCodeProfileAdapter()
	parsed := ParsedSkill{ExtraFrontmatter: map[string]interface{}{"when_to_use": "Use for code review"}}
	pkg := SkillPackageView{Parsed: parsed}
	detection, err := adapter.Detect(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(detection.Detected) != 1 {
		t.Fatalf("expected 1 detected profile, got %d", len(detection.Detected))
	}
	if detection.Detected[0].ID != ProfileIDClaudeCode {
		t.Fatalf("expected profile ID %q, got %q", ProfileIDClaudeCode, detection.Detected[0].ID)
	}
}

func TestClaudeCodeAdapter_Analyze_DisableModelInvocation(t *testing.T) {
	adapter := NewClaudeCodeProfileAdapter()
	parsed := ParsedSkill{ExtraFrontmatter: map[string]interface{}{
		"disable-model-invocation": true,
		"user-invocable":           true,
	}}
	pkg := SkillPackageView{Parsed: parsed}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if overlay.InvocationPolicy == nil {
		t.Fatal("expected invocation policy override")
	}
	if overlay.InvocationPolicy.ImplicitInvocationAllowed {
		t.Fatal("expected ImplicitInvocationAllowed=false")
	}
	if !overlay.InvocationPolicy.UserInvocationAllowed {
		t.Fatal("expected UserInvocationAllowed=true")
	}
}

func TestClaudeCodeAdapter_Analyze_BothInvocationsDisabled(t *testing.T) {
	adapter := NewClaudeCodeProfileAdapter()
	parsed := ParsedSkill{ExtraFrontmatter: map[string]interface{}{
		"disable-model-invocation": true,
		"user-invocable":           false,
	}}
	pkg := SkillPackageView{Parsed: parsed}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	foundWarning := false
	for _, w := range overlay.Warnings {
		if w.Code == "CLAUDE_NO_VALID_INVOKER" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatal("expected CLAUDE_NO_VALID_INVOKER warning when both invocation paths disabled")
	}
}

func TestClaudeCodeAdapter_Analyze_DynamicShellInjection(t *testing.T) {
	adapter := NewClaudeCodeProfileAdapter()
	parsed := ParsedSkill{Body: "Run !`git status` to check status"}
	pkg := SkillPackageView{Parsed: parsed}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range overlay.Features {
		if f.Feature == "dynamic_shell_injection" && f.State == FeatureStateBlocked {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected dynamic_shell_injection feature marked as blocked")
	}
}

func TestClaudeCodeAdapter_Analyze_ContextFork(t *testing.T) {
	adapter := NewClaudeCodeProfileAdapter()
	parsed := ParsedSkill{ExtraFrontmatter: map[string]interface{}{"context": "fork"}}
	pkg := SkillPackageView{Parsed: parsed}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if overlay.ExecutionMode != ExecutionModeIsolated {
		t.Fatalf("expected execution mode %q, got %q", ExecutionModeIsolated, overlay.ExecutionMode)
	}
	if overlay.InvocationPolicy == nil || !overlay.InvocationPolicy.IsolatedExecutionRequested {
		t.Fatal("expected IsolatedExecutionRequested=true")
	}
}

func TestClaudeCodeAdapter_Analyze_HooksUnsupported(t *testing.T) {
	adapter := NewClaudeCodeProfileAdapter()
	parsed := ParsedSkill{ExtraFrontmatter: map[string]interface{}{"hooks": map[string]interface{}{"pre": "something"}}}
	pkg := SkillPackageView{Parsed: parsed}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(overlay.UnsupportedFeatures) == 0 {
		t.Fatal("expected hooks in unsupported features")
	}
	found := false
	for _, f := range overlay.UnsupportedFeatures {
		if f == "hooks" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'hooks' in unsupported features")
	}
}

func TestOpenAIAdapter_Detect_NoOpenAIFile(t *testing.T) {
	adapter := NewOpenAIProfileAdapter()
	parsed := ParsedSkill{}
	pkg := SkillPackageView{Parsed: parsed, Files: map[string][]byte{}}
	detection, err := adapter.Detect(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(detection.Detected) != 0 {
		t.Fatalf("expected 0 detected profiles, got %d", len(detection.Detected))
	}
}

func TestOpenAIAdapter_Detect_WithOpenAIYAML(t *testing.T) {
	adapter := NewOpenAIProfileAdapter()
	parsed := ParsedSkill{}
	pkg := SkillPackageView{
		Parsed: parsed,
		Files:  map[string][]byte{"agents/openai.yaml": []byte("interface:\n  display_name: Test\n")},
	}
	detection, err := adapter.Detect(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(detection.Detected) != 1 {
		t.Fatalf("expected 1 detected profile, got %d", len(detection.Detected))
	}
	if detection.Detected[0].ID != ProfileIDOpenAI {
		t.Fatalf("expected profile ID %q, got %q", ProfileIDOpenAI, detection.Detected[0].ID)
	}
}

func TestOpenAIAdapter_Analyze_Interface(t *testing.T) {
	adapter := NewOpenAIProfileAdapter()
	yaml := []byte("interface:\n  display_name: My Skill\n  short_description: A test\n  brand_color: \"#FF5733\"\n  default_prompt: hello\n")
	parsed := ParsedSkill{}
	pkg := SkillPackageView{Parsed: parsed, Files: map[string][]byte{"agents/openai.yaml": yaml}}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if overlay.UI == nil {
		t.Fatal("expected UI hints")
	}
	if overlay.UI.DisplayName != "My Skill" {
		t.Fatalf("expected display name 'My Skill', got %q", overlay.UI.DisplayName)
	}
	if overlay.UI.ShortDescription != "A test" {
		t.Fatalf("expected short desc 'A test', got %q", overlay.UI.ShortDescription)
	}
	if overlay.UI.BrandColor != "#FF5733" {
		t.Fatalf("expected brand color '#FF5733', got %q", overlay.UI.BrandColor)
	}
	if overlay.UI.DefaultPrompt != "hello" {
		t.Fatalf("expected default prompt 'hello', got %q", overlay.UI.DefaultPrompt)
	}
}

func TestOpenAIAdapter_Analyze_PolicyImplicit(t *testing.T) {
	adapter := NewOpenAIProfileAdapter()
	yaml := []byte("policy:\n  allow_implicit_invocation: false\n")
	parsed := ParsedSkill{}
	pkg := SkillPackageView{Parsed: parsed, Files: map[string][]byte{"agents/openai.yaml": yaml}}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if overlay.InvocationPolicy == nil {
		t.Fatal("expected invocation policy")
	}
	if overlay.InvocationPolicy.ImplicitInvocationAllowed {
		t.Fatal("expected ImplicitInvocationAllowed=false")
	}
}

func TestClaudeLegacyAdapter_Detect_CommandPath(t *testing.T) {
	adapter := NewClaudeLegacyCommandAdapter()
	parsed := ParsedSkill{}
	pkg := SkillPackageView{
		Parsed:     parsed,
		SourceFile: ".claude/commands/deploy.md",
	}
	detection, err := adapter.Detect(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(detection.Detected) != 1 {
		t.Fatalf("expected 1 detected profile, got %d", len(detection.Detected))
	}
	if detection.Detected[0].ID != ProfileIDClaudeCommandLegacy {
		t.Fatalf("expected profile ID %q, got %q", ProfileIDClaudeCommandLegacy, detection.Detected[0].ID)
	}
}

func TestClaudeLegacyAdapter_Analyze_DeriveName(t *testing.T) {
	adapter := NewClaudeLegacyCommandAdapter()
	parsed := ParsedSkill{Body: "Deploy the application to production."}
	pkg := SkillPackageView{
		Parsed:     parsed,
		SourceFile: ".claude/commands/deploy.md",
	}
	overlay, err := adapter.Analyze(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(overlay.Errors) > 0 {
		t.Fatalf("unexpected errors: %+v", overlay.Errors)
	}
}

func TestProfileDetector_Detect(t *testing.T) {
	detector := NewProfileDetector(
		NewClaudeCodeProfileAdapter(),
		NewOpenAIProfileAdapter(),
	)
	parsed := ParsedSkill{ExtraFrontmatter: map[string]interface{}{
		"when_to_use": "test",
	}}
	pkg := SkillPackageView{
		Parsed: parsed,
		Files:  map[string][]byte{"agents/openai.yaml": []byte("interface:\n  display_name: X\n")},
	}
	detected, overlays, err := detector.Detect(context.Background(), pkg, parsed)
	if err != nil {
		t.Fatal(err)
	}
	if len(detected) != 2 {
		t.Fatalf("expected 2 detected profiles, got %d", len(detected))
	}
	if len(overlays) != 2 {
		t.Fatalf("expected 2 overlays, got %d", len(overlays))
	}
}

func TestCompatibilityMerger_Merge(t *testing.T) {
	merger := NewCompatibilityMerger()
	overlays := []SkillCompatibilityOverlay{
		{
			Profile:        ProfileIDClaudeCode,
			AdapterVersion: AdapterVersionClaudeCode,
			InvocationPolicy: &SkillInvocationPolicy{
				UserInvocationAllowed:     true,
				ImplicitInvocationAllowed: false,
			},
			ActivationHints: []string{"hint1"},
		},
		{
			Profile:        ProfileIDOpenAI,
			AdapterVersion: AdapterVersionOpenAI,
			InvocationPolicy: &SkillInvocationPolicy{
				UserInvocationAllowed:     false,
				ImplicitInvocationAllowed: true,
			},
			UI: &SkillUIHints{DisplayName: "Test"},
		},
	}

	canonical, report := merger.Merge(overlays, DefaultInvocationPolicy, nil)

	if canonical.InvocationPolicy.UserInvocationAllowed {
		t.Fatal("expected UserInvocationAllowed=false (restrictive merge)")
	}
	if canonical.InvocationPolicy.ImplicitInvocationAllowed {
		t.Fatal("expected ImplicitInvocationAllowed=false (restrictive merge)")
	}
	if len(canonical.ActivationHints) != 1 || canonical.ActivationHints[0] != "hint1" {
		t.Fatalf("expected activation hints [hint1], got %v", canonical.ActivationHints)
	}
	if canonical.UI.DisplayName != "Test" {
		t.Fatalf("expected display name 'Test', got %q", canonical.UI.DisplayName)
	}
	if report.Status == "" {
		t.Fatal("expected non-empty report status")
	}
}

func TestMergeInvocationPolicy(t *testing.T) {
	base := SkillInvocationPolicy{
		UserInvocationAllowed:     true,
		ImplicitInvocationAllowed: true,
		BackgroundAllowed:         true,
	}
	overlay := SkillInvocationPolicy{
		UserInvocationAllowed:     false,
		ImplicitInvocationAllowed: true,
		IsolatedExecutionRequested: true,
	}
	merged := MergeInvocationPolicy(base, overlay)
	if merged.UserInvocationAllowed {
		t.Fatal("expected UserInvocationAllowed=false (restrictive AND merge)")
	}
	if !merged.ImplicitInvocationAllowed {
		t.Fatal("expected ImplicitInvocationAllowed=true (both true)")
	}
	if merged.BackgroundAllowed {
		t.Fatal("expected BackgroundAllowed=false (overlay false AND base true = false)")
	}
	if !merged.IsolatedExecutionRequested {
		t.Fatal("expected IsolatedExecutionRequested=true (overlay OR base)")
	}
}

func TestCompatibilityPipeline_Evaluate(t *testing.T) {
	pipeline := DefaultCompatibilityPipeline()
	parsed := ParsedSkill{
		Name:        "test-skill",
		Description: "A test skill",
		ExtraFrontmatter: map[string]interface{}{
			"when_to_use": "Use this for testing",
		},
	}
	pkg := SkillPackageView{
		Parsed:      parsed,
		ContentHash: "sha256:abc123",
		Files:       map[string][]byte{},
	}

	canonical, report, err := pipeline.Evaluate(context.Background(), pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.InvocationPolicy.UserInvocationAllowed != true {
		t.Fatal("expected UserInvocationAllowed=true by default")
	}
	if len(report.Detected) != 1 {
		t.Fatalf("expected 1 detected profile, got %d", len(report.Detected))
	}
	if report.Fingerprint == nil {
		t.Fatal("expected fingerprint in report")
	}
	if report.Fingerprint.ContentHash != "sha256:abc123" {
		t.Fatalf("expected content hash 'sha256:abc123', got %q", report.Fingerprint.ContentHash)
	}
	if report.Fingerprint.CapabilityGen != CapabilityGeneration {
		t.Fatalf("expected capability generation %d, got %d", CapabilityGeneration, report.Fingerprint.CapabilityGen)
	}
}

func TestComputeFingerprint(t *testing.T) {
	versions := map[string]string{"claude-code": "claude-code@1"}
	fp := ComputeFingerprint("sha256:test", versions)
	if fp.ContentHash != "sha256:test" {
		t.Fatalf("expected content hash, got %q", fp.ContentHash)
	}
	if fp.AdapterVersions["claude-code"] != "claude-code@1" {
		t.Fatalf("expected adapter version, got %q", fp.AdapterVersions["claude-code"])
	}
	if fp.CapabilityGen != CapabilityGeneration {
		t.Fatalf("expected capability generation, got %d", fp.CapabilityGen)
	}
}

func TestDeriveFirstParagraph(t *testing.T) {
	body := "First paragraph of the command.\n\nSecond paragraph."
	result := deriveFirstParagraph(body)
	if !strings.Contains(result, "First paragraph") {
		t.Fatalf("expected 'First paragraph' in result, got %q", result)
	}
	if strings.Contains(result, "Second") {
		t.Fatalf("result should not contain second paragraph, got %q", result)
	}
}

func TestDeriveNameFromSource(t *testing.T) {
	tests := []struct {
		source   string
		expected string
	}{
		{".claude/commands/deploy.md", "deploy"},
		{".claude/commands/my-command.md", "my-command"},
		{"SKILL.md", "skill"},
	}
	for _, tt := range tests {
		result := deriveNameFromSource(tt.source)
		if result != tt.expected {
			t.Fatalf("from %q: expected %q, got %q", tt.source, tt.expected, result)
		}
	}
}
