package card

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCHARXExportOmitsFirstMessage(t *testing.T) {
	cardJSON, err := buildV3JSON(&CharacterCard{
		Name:            "测试角色",
		Description:     "角色描述",
		ExampleMessages: "示例对话",
		Preserved: map[string]json.RawMessage{
			"first_mes": json.RawMessage(`"不应保留"`),
		},
	}, map[string]json.RawMessage{
		"first_mes": json.RawMessage(`"不应保留"`),
	})
	if err != nil {
		t.Fatalf("buildV3JSON() error = %v", err)
	}
	if bytes.Contains(cardJSON, []byte("first_mes")) {
		t.Fatalf("buildV3JSON() retained first_mes: %s", string(cardJSON))
	}

	charxData, err := buildCHARX(cardJSON, nil)
	if err != nil {
		t.Fatalf("buildCHARX() error = %v", err)
	}
	parsed, _, err := parseCHARX(charxData)
	if err != nil {
		t.Fatalf("parseCHARX() error = %v", err)
	}
	if parsed.Name != "测试角色" || parsed.Description != "角色描述" {
		t.Fatalf("parseCHARX() returned unexpected card: %+v", parsed)
	}
}

func TestOnlyCHARXExportIsSupported(t *testing.T) {
	exporter := NewExporter(t.TempDir())
	for _, format := range []string{"v2_json", "v3_json", "v2_png", "v3_png"} {
		if _, _, err := exporter.Export(ExportInput{Name: "测试角色"}, format); err != ErrUnsupportedFormat {
			t.Fatalf("Export(%q) error = %v, want %v", format, err, ErrUnsupportedFormat)
		}
	}
	if _, _, err := exporter.Export(ExportInput{Name: "测试角色"}, "v3_charx"); err != nil {
		t.Fatalf("Export(v3_charx) error = %v", err)
	}
}

func TestTavernRemainsSupported(t *testing.T) {
	data := []byte(`{"name":"酒馆角色","description":"角色描述","mes_example":"示例对话"}`)
	format, err := DetectFormat(data, "tavern.json")
	if err != nil {
		t.Fatalf("DetectFormat() error = %v", err)
	}
	if format != FormatTavernJSON {
		t.Fatalf("DetectFormat() = %s, want %s", format, FormatTavernJSON)
	}
	if !strings.Contains(string(data), "酒馆角色") {
		t.Fatal("unexpected test fixture")
	}
}
