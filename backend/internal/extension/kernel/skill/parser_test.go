package skill

import (
	"context"
	"strings"
	"testing"
)

func makeRoot(name string) SkillRoot {
	return SkillRoot{RootURI: "/skills/" + name, Source: "import"}
}

func TestParser_ValidBasic(t *testing.T) {
	input := "---\nname: pdf-processing\ndescription: Extract PDFs and process forms.\n---\n\n# PDF Processing\n\nInstructions here.\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("pdf-processing"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != "pdf-processing" {
		t.Fatalf("name mismatch: %s", parsed.Name)
	}
	if parsed.Description != "Extract PDFs and process forms." {
		t.Fatalf("description mismatch: %s", parsed.Description)
	}
	if !strings.Contains(parsed.Body, "# PDF Processing") {
		t.Fatalf("body mismatch: %s", parsed.Body)
	}
	if parsed.ContentHash == "" {
		t.Fatal("content hash is empty")
	}
	if len(parsed.Diagnostics) > 0 {
		for _, d := range parsed.Diagnostics {
			if d.Severity == DiagnosticSeverityError {
				t.Fatalf("unexpected error diag: %+v", d)
			}
		}
	}
}

func TestParser_WithBOM(t *testing.T) {
	input := "\xEF\xBB\xBF---\nname: my-skill\ndescription: test desc\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("my-skill"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != "my-skill" {
		t.Fatalf("name mismatch: %s", parsed.Name)
	}
}

func TestParser_CRLF(t *testing.T) {
	input := "---\r\nname: crlf-skill\r\ndescription: crlf test\r\n---\r\nbody content\r\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("crlf-skill"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != "crlf-skill" {
		t.Fatalf("name mismatch: %s", parsed.Name)
	}
	if !strings.Contains(parsed.Body, "body content") {
		t.Fatalf("body mismatch: %s", parsed.Body)
	}
}

func TestParser_NoOpeningDelimiter(t *testing.T) {
	input := "name: no-delim\ndescription: test\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("no-delim"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for missing opening delimiter")
	}
}

func TestParser_NoClosingDelimiter(t *testing.T) {
	input := "---\nname: unclosed\ndescription: test\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("unclosed"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter")
	}
}

func TestParser_EmptyFrontmatter(t *testing.T) {
	input := "---\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("empty-fm"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for empty frontmatter")
	}
}

func TestParser_NameRequired(t *testing.T) {
	input := "---\ndescription: test desc\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("test-skill"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if parsed.Diagnostics == nil {
		t.Fatal("expected diagnostics")
	}
}

func TestParser_DescriptionRequired(t *testing.T) {
	input := "---\nname: my-skill\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("my-skill"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestParser_NameInvalid_Uppercase(t *testing.T) {
	input := "---\nname: MySkill\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("MySkill"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for uppercase name")
	}
}

func TestParser_NameInvalid_Underscore(t *testing.T) {
	input := "---\nname: my_skill\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("my_skill"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for underscore in name")
	}
}

func TestParser_NameInvalid_Space(t *testing.T) {
	input := "---\nname: my skill\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("my skill"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for space in name")
	}
}

func TestParser_NameInvalid_LeadingHyphen(t *testing.T) {
	input := "---\nname: -invalid\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("-invalid"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for leading hyphen")
	}
}

func TestParser_NameInvalid_TrailingHyphen(t *testing.T) {
	input := "---\nname: invalid-\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("invalid-"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for trailing hyphen")
	}
}

func TestParser_NameInvalid_DoubleHyphen(t *testing.T) {
	input := "---\nname: my--skill\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("my--skill"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for double hyphen")
	}
}

func TestParser_NameInvalid_TooLong(t *testing.T) {
	longName := strings.Repeat("a", 65)
	input := "---\nname: " + longName + "\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot(longName), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for name too long")
	}
}

func TestParser_NameValid_MinLength(t *testing.T) {
	input := "---\nname: a\ndescription: test\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("a"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != "a" {
		t.Fatalf("name mismatch: %s", parsed.Name)
	}
}

func TestParser_NameValid_MaxLength(t *testing.T) {
	name := strings.Repeat("a", 64)
	input := "---\nname: " + name + "\ndescription: test\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot(name), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != name {
		t.Fatalf("name mismatch: %s", parsed.Name)
	}
}

func TestParser_NameDirectoryMismatch(t *testing.T) {
	input := "---\nname: correct-name\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), SkillRoot{RootURI: "/skills/wrong-name", Source: "import"}, DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for name-directory mismatch")
	}
}

func TestParser_DescriptionTooLong(t *testing.T) {
	desc := strings.Repeat("a", 1025)
	input := "---\nname: long-desc\ndescription: " + desc + "\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("long-desc"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for description too long")
	}
}

func TestParser_DescriptionEmpty(t *testing.T) {
	input := "---\nname: empty-desc\ndescription: \"\"\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("empty-desc"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestParser_DescriptionUnicode(t *testing.T) {
	input := "---\nname: unicode-desc\ndescription: 这是一个中文描述\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("unicode-desc"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Description != "这是一个中文描述" {
		t.Fatalf("description mismatch: %s", parsed.Description)
	}
}

func TestParser_LicenseTooLong(t *testing.T) {
	lic := strings.Repeat("x", 513)
	input := "---\nname: long-lic\ndescription: test\nlicense: " + lic + "\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("long-lic"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for license too long")
	}
}

func TestParser_CompatibilityTooLong(t *testing.T) {
	compat := strings.Repeat("y", 501)
	input := "---\nname: long-compat\ndescription: test\ncompatibility: " + compat + "\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("long-compat"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for compatibility too long")
	}
}

func TestParser_MetadataValid(t *testing.T) {
	input := "---\nname: has-meta\ndescription: test\nmetadata:\n  author: Alice\n  version: \"1.0\"\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("has-meta"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Metadata == nil {
		t.Fatal("metadata is nil")
	}
	if parsed.Metadata["author"] != "Alice" {
		t.Fatalf("metadata author mismatch: %s", parsed.Metadata["author"])
	}
	if parsed.Metadata["version"] != "1.0" {
		t.Fatalf("metadata version mismatch: %s", parsed.Metadata["version"])
	}
}

func TestParser_MetadataInvalid_NumericValue(t *testing.T) {
	input := "---\nname: bad-meta\ndescription: test\nmetadata:\n  version: 1.0\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("bad-meta"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for numeric metadata value")
	}
}

func TestParser_MetadataInvalid_NestedMap(t *testing.T) {
	input := "---\nname: nested-meta\ndescription: test\nmetadata:\n  nested:\n    foo: bar\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("nested-meta"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for nested metadata")
	}
}

func TestParser_MetadataEmptyKey(t *testing.T) {
	input := "---\nname: empty-key\ndescription: test\nmetadata:\n  \"\": value\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("empty-key"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for empty metadata key")
	}
}

func TestParser_AllowedToolsValid(t *testing.T) {
	input := "---\nname: tools-skill\ndescription: test\nallowed-tools: Bash(git:*) Bash(jq:) Read\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("tools-skill"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.AllowedTools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %v", len(parsed.AllowedTools), parsed.AllowedTools)
	}
	if parsed.AllowedTools[0] != "Bash(git:*)" {
		t.Fatalf("first tool mismatch: %s", parsed.AllowedTools[0])
	}
}

func TestParser_AllowedToolsEmpty(t *testing.T) {
	input := "---\nname: empty-tools\ndescription: test\nallowed-tools: \"\"\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("empty-tools"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.AllowedTools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(parsed.AllowedTools))
	}
}

func TestParser_AllowedToolsTooMany(t *testing.T) {
	tools := strings.Repeat("Tool ", 130)
	input := "---\nname: many-tools\ndescription: test\nallowed-tools: " + tools + "\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("many-tools"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for too many allowed tools")
	}
}

func TestParser_ExtraFrontmatter(t *testing.T) {
	input := "---\nname: extra-skill\ndescription: test\nmodel: claude-4\nx-amitia:\n  custom: value\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("extra-skill"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.ExtraFrontmatter == nil || len(parsed.ExtraFrontmatter) == 0 {
		t.Fatal("expected extra frontmatter to be preserved")
	}
	if _, ok := parsed.ExtraFrontmatter["model"]; !ok {
		t.Fatal("expected 'model' in extra frontmatter")
	}
}

func TestParser_BodyEmpty(t *testing.T) {
	input := "---\nname: no-body\ndescription: test\n---\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("no-body"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasWarning := false
	for _, d := range parsed.Diagnostics {
		if d.Code == "SKILL_BODY_EMPTY" {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Fatal("expected SKILL_BODY_EMPTY warning")
	}
}

func TestParser_BodyContainsHorizontalRule(t *testing.T) {
	input := "---\nname: hr-body\ndescription: test\n---\n# Title\n\n---\n\nMore content\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("hr-body"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(parsed.Body, "---") {
		t.Fatalf("body should contain horizontal rule: %s", parsed.Body)
	}
}

func TestParser_ContentHashDeterministic(t *testing.T) {
	input := "---\nname: hash-skill\ndescription: test\n---\nbody\n"
	p := NewParser()
	r1, _ := p.ParseBytes(context.Background(), makeRoot("hash-skill"), DefaultParsePolicy, []byte(input))
	r2, _ := p.ParseBytes(context.Background(), makeRoot("hash-skill"), DefaultParsePolicy, []byte(input))
	if r1.ContentHash != r2.ContentHash {
		t.Fatalf("content hash not deterministic: %s vs %s", r1.ContentHash, r2.ContentHash)
	}
	if !strings.HasPrefix(r1.ContentHash, "sha256:") {
		t.Fatalf("content hash should have sha256: prefix: %s", r1.ContentHash)
	}
}

func TestParser_ContentHashDiffers(t *testing.T) {
	input1 := "---\nname: hash-skill\ndescription: test1\n---\nbody\n"
	input2 := "---\nname: hash-skill\ndescription: test2\n---\nbody\n"
	p := NewParser()
	r1, _ := p.ParseBytes(context.Background(), makeRoot("hash-skill"), DefaultParsePolicy, []byte(input1))
	r2, _ := p.ParseBytes(context.Background(), makeRoot("hash-skill"), DefaultParsePolicy, []byte(input2))
	if r1.ContentHash == r2.ContentHash {
		t.Fatal("content hash should differ for different content")
	}
}

func TestParser_YAMLInvalid(t *testing.T) {
	input := "---\nname: bad-yaml\ndescription: test\n  invalid-indent: true\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("bad-yaml"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParser_YAMLDuplicateKey(t *testing.T) {
	input := "---\nname: dup-key\ndescription: test\ndescription: duplicate\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("dup-key"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for duplicate YAML key")
	}
}

func TestParser_YAMLSequenceRoot(t *testing.T) {
	input := "---\n- item1\n- item2\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("seq-root"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for sequence root")
	}
}

func TestParser_YAMLAlias(t *testing.T) {
	input := "---\nname: alias-skill\ndescription: &desc test\nsome-key: *desc\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("alias-skill"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error for YAML alias")
	}
}

func TestParser_YAMLTooDeep(t *testing.T) {
	input := "---\nname: deep-yaml\ndescription: test\n" +
		"a:\n  b:\n    c:\n      d:\n        e:\n          f:\n            g:\n              h:\n                i:\n                  j:\n                    k:\n                      l:\n                        m:\n                          n:\n                            o:\n                              p:\n                                q:\n                                  r: value\n---\nbody\n"
	policy := ParsePolicy{MaxYAMLDepth: 10, MaxFileBytes: 1 << 20, MaxFrontmatterBytes: 64 << 10}
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("deep-yaml"), policy, []byte(input))
	if err == nil {
		t.Fatal("expected error for YAML too deep")
	}
}

func TestParser_Preview(t *testing.T) {
	input := "---\nname: preview-skill\ndescription: test\n---\n# Body content\nSome instructions.\n"
	p := NewParser()
	preview, err := p.Preview(context.Background(), makeRoot("preview-skill"), DefaultParsePolicy, strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Name != "preview-skill" {
		t.Fatalf("name mismatch: %s", preview.Name)
	}
	if preview.BodyBytes == 0 {
		t.Fatal("expected non-zero body bytes")
	}
	if preview.BodyLines == 0 {
		t.Fatal("expected non-zero body lines")
	}
	if preview.CompatibilityStatus != SkillCompatStatusCompatible {
		t.Fatalf("expected compatible status, got %s", preview.CompatibilityStatus)
	}
}

func TestParser_CompatibilityStatus(t *testing.T) {
	tests := []struct {
		name string
		diags []SkillDiagnostic
		want string
	}{
		{"no diags", nil, SkillCompatStatusCompatible},
		{"warning only", []SkillDiagnostic{{Severity: DiagnosticSeverityWarning, Code: "test"}}, SkillCompatStatusDegraded},
		{"error", []SkillDiagnostic{{Severity: DiagnosticSeverityError, Code: "test"}}, SkillCompatStatusBlocked},
		{"mixed", []SkillDiagnostic{{Severity: DiagnosticSeverityWarning, Code: "test"}, {Severity: DiagnosticSeverityError, Code: "test"}}, SkillCompatStatusBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompatibilityStatus(tt.diags)
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParser_DiagnosticsStableCodes(t *testing.T) {
	input := "---\nname: diag-skill\ndescription: test\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("diag-skill"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, d := range parsed.Diagnostics {
		if d.Code == "" {
			t.Fatal("diagnostic code should not be empty")
		}
	}
}

func TestParser_NoPermissionGrantedByParse(t *testing.T) {
	input := "---\nname: perm-test\ndescription: test\nallowed-tools: Bash(*) Read Write\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("perm-test"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.AllowedTools) == 0 {
		t.Fatal("expected allowed tools to be parsed")
	}
}

func TestParser_AllowedToolsSorted(t *testing.T) {
	input := "---\nname: sort-tools\ndescription: test\nallowed-tools: Write Read Bash\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("sort-tools"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"Bash", "Read", "Write"}
	for i, v := range expected {
		if parsed.AllowedTools[i] != v {
			t.Fatalf("expected %s at index %d, got %s", v, i, parsed.AllowedTools[i])
		}
	}
}

func TestParser_FrontmatterNotFirstLine(t *testing.T) {
	input := "   ---\nname: bad\ndescription: test\n---\nbody\n"
	p := NewParser()
	_, err := p.ParseBytes(context.Background(), makeRoot("bad"), DefaultParsePolicy, []byte(input))
	if err == nil {
		t.Fatal("expected error when frontmatter delimiter is not on its own line")
	}
}

func TestParser_FileNameValid_AlphaNum(t *testing.T) {
	input := "---\nname: abc-123\ndescription: test\n---\nbody\n"
	p := NewParser()
	parsed, err := p.ParseBytes(context.Background(), makeRoot("abc-123"), DefaultParsePolicy, []byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Name != "abc-123" {
		t.Fatalf("name mismatch: %s", parsed.Name)
	}
}
