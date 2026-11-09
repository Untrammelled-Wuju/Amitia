package schema_ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspacePluginSchemaCompiles(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "..", "..", "..", "plugins-code", "workspace", "ui", "schema", "workspace.json"),
		filepath.Join("..", "..", "..", "..", "..", "plugins-code", "chat-import", "ui", "schema", "chat-import.json"),
		filepath.Join("..", "..", "..", "..", "..", "plugins-code", "reminders", "ui", "schema", "reminders.json"),
		filepath.Join("..", "..", "..", "..", "..", "plugins-code", "storage", "ui", "schema", "storage.json"),
		filepath.Join("..", "..", "..", "..", "..", "plugins-code", "chat-logs", "ui", "schema", "chat-logs.json"),
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
