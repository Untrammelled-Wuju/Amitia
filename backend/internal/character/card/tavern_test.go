package card

import (
	"encoding/base64"
	"encoding/binary"
	"testing"
)

func TestDetectAndParseTavernJSON(t *testing.T) {
	data := []byte(`{
		"name":"酒馆角色",
		"description":"角色背景",
		"personality":"角色性格",
		"scenario":"所在场景",
		"first_mes":"初次问候",
		"mes_example":"示例对话",
		"creatorcomment":"作者说明",
		"future_field":"保留内容"
	}`)

	format, err := DetectFormat(data, "character.json")
	if err != nil {
		t.Fatalf("DetectFormat() error = %v", err)
	}
	if format != FormatTavernJSON {
		t.Fatalf("DetectFormat() = %s, want %s", format, FormatTavernJSON)
	}

	parsed, preserved, err := NewCardParser().Parse(data, "character.json")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Name != "酒馆角色" || parsed.CreatorNotes != "作者说明" {
		t.Fatalf("Parse() mapped unexpected card: %+v", parsed)
	}
	if _, ok := preserved["future_field"]; !ok {
		t.Fatal("Parse() did not preserve an unknown field")
	}
}

func TestParseTavernLegacyAliases(t *testing.T) {
	data := []byte(`{
		"char_name":"旧版角色",
		"char_persona":"角色设定",
		"world_scenario":"世界场景",
		"char_greeting":"角色问候",
		"example_dialogue":"对话示例"
	}`)

	parsed, _, err := NewCardParser().Parse(data, "legacy.json")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Name != "旧版角色" || parsed.Description != "角色设定" || parsed.Scenario != "世界场景" || parsed.FirstMessage != "角色问候" || parsed.ExampleMessages != "对话示例" {
		t.Fatalf("Parse() alias mapping unexpected card: %+v", parsed)
	}
}

func TestDetectTavernPNG(t *testing.T) {
	metadata := base64.StdEncoding.EncodeToString([]byte(`{"name":"PNG角色","description":"PNG角色背景"}`))
	png := append([]byte{}, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}...)
	chunkData := append([]byte("chara\x00"), []byte(metadata)...)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(chunkData)))
	png = append(png, length...)
	png = append(png, []byte("tEXt")...)
	png = append(png, chunkData...)
	png = append(png, 0, 0, 0, 0)

	format, err := DetectFormat(png, "character.png")
	if err != nil {
		t.Fatalf("DetectFormat() error = %v", err)
	}
	if format != FormatTavernPNG {
		t.Fatalf("DetectFormat() = %s, want %s", format, FormatTavernPNG)
	}

	parsed, _, err := NewCardParser().Parse(png, "character.png")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.SourceFormat != FormatTavernPNG || parsed.Name != "PNG角色" {
		t.Fatalf("Parse() returned unexpected card: %+v", parsed)
	}
}

func TestRejectUnrelatedJSONAsTavernCard(t *testing.T) {
	data := []byte(`{"name":"普通配置","enabled":true}`)
	if _, err := DetectFormat(data, "config.json"); err != ErrUnsupportedFormat {
		t.Fatalf("DetectFormat() error = %v, want %v", err, ErrUnsupportedFormat)
	}
}

func TestParseCanonicalV2Envelope(t *testing.T) {
	data := []byte(`{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"V2角色","description":"V2角色背景","first_mes":"V2问候","creator_notes":"V2说明"}}`)
	parsed, _, err := NewCardParser().Parse(data, "v2.json")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Name != "V2角色" || parsed.Description != "V2角色背景" || parsed.FirstMessage != "V2问候" || parsed.CreatorNotes != "V2说明" {
		t.Fatalf("Parse() returned unexpected V2 card: %+v", parsed)
	}
}

func TestParseCanonicalV3Envelope(t *testing.T) {
	data := []byte(`{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"V3角色","description":"V3角色背景","nickname":"V3昵称","group_only_greetings":["群聊问候"]}}`)
	parsed, _, err := NewCardParser().Parse(data, "v3.json")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Name != "V3角色" || parsed.Description != "V3角色背景" || parsed.Nickname != "V3昵称" || len(parsed.GroupOnlyGreetings) != 1 {
		t.Fatalf("Parse() returned unexpected V3 card: %+v", parsed)
	}
}
