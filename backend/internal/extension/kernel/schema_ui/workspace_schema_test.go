package schema_ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspacePluginSchemaCompiles(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "..", "..", "..", "plugins-builtin", "plugin-market", "ui", "schema", "extensions-hub.json"),
		filepath.Join("..", "..", "..", "..", "..", "plugins-builtin", "plugin-market", "ui", "schema", "kernel-hub.json"),
		filepath.Join("..", "..", "..", "..", "..", "plugins-builtin", "plugin-market", "ui", "schema", "desktop.json"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document SchemaUIDocument
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		if _, err := Compile(&document); err != nil {
			t.Fatal(err)
		}
	}
}
