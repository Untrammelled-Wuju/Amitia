package javascript_main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectMJS(t *testing.T) {
	d := NewModuleFormatDetector()
	if got := d.Detect("index.mjs"); got != FormatESM {
		t.Fatalf("expected esm, got %s", got)
	}
}

func TestDetectCJS(t *testing.T) {
	d := NewModuleFormatDetector()
	if got := d.Detect("index.cjs"); got != FormatCJS {
		t.Fatalf("expected cjs, got %s", got)
	}
}

func TestDetectEmpty(t *testing.T) {
	d := NewModuleFormatDetector()
	if got := d.Detect(""); got != FormatUnknown {
		t.Fatalf("expected unknown, got %s", got)
	}
}

func TestDetectUnknownExt(t *testing.T) {
	d := NewModuleFormatDetector()
	if got := d.Detect("index.py"); got != FormatUnknown {
		t.Fatalf("expected unknown, got %s", got)
	}
}

func TestDetectJSWithoutPackageJSON(t *testing.T) {
	d := NewModuleFormatDetector()
	tmp := t.TempDir()
	entry := filepath.Join(tmp, "index.js")
	if err := os.WriteFile(entry, []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := d.Detect(entry); got != FormatCJS {
		t.Fatalf("expected cjs (default), got %s", got)
	}
}

func TestDetectJSWithModulePackageJSON(t *testing.T) {
	d := NewModuleFormatDetector()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"type":"module"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(tmp, "index.js")
	if err := os.WriteFile(entry, []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := d.Detect(entry); got != FormatESM {
		t.Fatalf("expected esm, got %s", got)
	}
}

func TestDetectJSWithCommonJSPackageJSON(t *testing.T) {
	d := NewModuleFormatDetector()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"type":"commonjs"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(tmp, "index.js")
	if err := os.WriteFile(entry, []byte("module.exports = {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := d.Detect(entry); got != FormatCJS {
		t.Fatalf("expected cjs, got %s", got)
	}
}

func TestDetectSubdirectoryInheritsPackageJSON(t *testing.T) {
	d := NewModuleFormatDetector()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"type":"module"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "deep", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(sub, "index.js")
	if err := os.WriteFile(entry, []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := d.Detect(entry); got != FormatESM {
		t.Fatalf("expected esm (inherited), got %s", got)
	}
}

func TestIsTypeScript(t *testing.T) {
	d := NewModuleFormatDetector()
	tests := map[string]bool{
		"index.ts":  true,
		"index.tsx": true,
		"index.mts": true,
		"index.cts": true,
		"index.js":  false,
		"index.mjs": false,
		"index.cjs": false,
	}
	for path, expected := range tests {
		if got := d.IsTypeScript(path); got != expected {
			t.Fatalf("IsTypeScript(%s): expected %v, got %v", path, expected, got)
		}
	}
}

func TestNormalizedExtension(t *testing.T) {
	d := NewModuleFormatDetector()
	tests := map[ModuleFormat]string{
		FormatESM:     ".mjs",
		FormatCJS:     ".cjs",
		FormatUnknown: ".js",
	}
	for format, expected := range tests {
		if got := d.NormalizedExtension(format); got != expected {
			t.Fatalf("NormalizedExtension(%s): expected %s, got %s", format, expected, got)
		}
	}
}

