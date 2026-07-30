package legacy_deprecation

import (
	"path/filepath"
	"testing"
)

func TestPackageLegacyProductionBoundary(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePackageLegacyBoundary(root); err != nil {
		t.Fatal(err)
	}
}
