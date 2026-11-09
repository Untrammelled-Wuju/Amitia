package plugincontract

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSourceHasSingleManifestAndExplicitAdapters(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	targets := []string{
		filepath.Join(root, "backend/internal/extension/kernel"),
		filepath.Join(root, "sdk/plugin-sdk/src"),
		filepath.Join(root, "sdk/plugin-cli/src"),
	}
	forbidden := []string{"AmitiaxManifestV1", "ContributionAdapterForKind", "manifest_v1"}
	for _, target := range targets {
		err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				name := entry.Name()
				if name == "node_modules" || name == "dist" || name == "build" || name == "coverage" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(entry.Name(), "_test.go") || strings.HasSuffix(entry.Name(), ".test.ts") {
				return nil
			}
			switch filepath.Ext(entry.Name()) {
			case ".go", ".ts", ".tsx":
			default:
				return nil
			}
			payload, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			source := string(payload)
			for _, symbol := range forbidden {
				if strings.Contains(source, symbol) {
					t.Errorf("legacy plugin source remains in %s: %s", path, symbol)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", target, err)
		}
	}
}
