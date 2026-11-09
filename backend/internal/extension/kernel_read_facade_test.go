package extension

import "testing"

func TestClassifyPackageFile(t *testing.T) {
	tests := map[string]string{
		"modules/runtime/dist/index.js": "script",
		"assets/pet/frames/idle.png":    "asset",
		"preview.webp":                  "asset",
		"references/guide.pdf":          "reference",
		"manifest.json":                 "file",
	}
	for input, want := range tests {
		if got := classifyPackageFile(input); got != want {
			t.Fatalf("classifyPackageFile(%q)=%q, want %q", input, got, want)
		}
	}
}
