package schema_ui

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigratedPluginSchemasValidate(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	files := []string{
		"plugins/channels/wechat-ipad860/ui/schema/desktop.json",
		"plugins/channels/wechat-ipad860/ui/schema/mobile.json",
		"plugins/emote/ui/schema/desktop-index.json",
		"plugins/emote/ui/schema/mobile-index.json",
		"plugins/emote/ui/schema/desktop-composer.json",
		"plugins/emote/ui/schema/mobile-composer.json",
		"plugins/emote/ui/schema/desktop-message.json",
		"plugins/emote/ui/schema/mobile-message.json",
		"plugins/lifestyle/ui/schema/desktop.json",
		"plugins/lifestyle/ui/schema/mobile.json",
		"plugins/proactive/ui/schema/desktop.json",
		"plugins/proactive/ui/schema/mobile.json",
	}
	validator := NewValidator()
	for _, relative := range files {
		t.Run(relative, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			var document SchemaUIDocument
			if err := json.Unmarshal(data, &document); err != nil {
				t.Fatalf("unmarshal schema: %v", err)
			}
			result := validator.Validate(&document)
			if !result.Valid {
				t.Fatalf("schema invalid: %v", result.Errors)
			}
		})
	}
}

func TestPackagedMinecraftSchemaValidates(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	packagePath := filepath.Join(root, "plugin", "Game", "Minecraft_1.21.11_Amitiax_V1_1.0.0.gamex")
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		t.Fatalf("open game package: %v", err)
	}
	defer archive.Close()
	var schemaData []byte
	for _, entry := range archive.File {
		if entry.Name != "assets/ui/minecraft-dashboard.schema.json" {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatalf("open schema: %v", err)
		}
		schemaData, err = io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}
		break
	}
	if len(schemaData) == 0 {
		t.Fatal("minecraft schema missing")
	}
	var document SchemaUIDocument
	if err := json.Unmarshal(schemaData, &document); err != nil {
		t.Fatalf("unmarshal minecraft schema: %v", err)
	}
	result := NewValidator().Validate(&document)
	if !result.Valid {
		t.Fatalf("minecraft schema invalid: %v", result.Errors)
	}
}
