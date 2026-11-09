package packageext

import "testing"

func TestIsArchivePathKeepsOnlyNameAliases(t *testing.T) {
	for _, value := range []string{"plugin.amitiax", "plugin.GAMEX", "plugin.petx"} {
		if !IsArchivePath(value) {
			t.Fatalf("%s must be recognized as a plugin package", value)
		}
	}
	for _, value := range []string{"plugin.zip", "plugin.game", "plugin"} {
		if IsArchivePath(value) {
			t.Fatalf("%s must not be recognized as a plugin package", value)
		}
	}
}
