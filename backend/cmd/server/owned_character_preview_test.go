package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/character/card"
)

func TestCardBookWorldbookMappingPreservesSelectiveEntryMetadata(t *testing.T) {
	selective := true
	caseSensitive := true
	position := "before_char"
	entry := card.CharacterBookEntry{
		Name: "港口雨夜", ID: json.RawMessage(`17`),
		Keys: []string{"雨", "港口"}, SecondaryKeys: []string{"闭港", "灯塔"},
		Content: "雨夜港口关闭", Enabled: true, InsertionOrder: 12,
		CaseSensitive: &caseSensitive, Selective: &selective,
		Position: &position, Priority: 8,
		Extensions: map[string]any{"source": "tavern"},
	}
	rule, valid := cardBookWorldbookRule(entry, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", 3)
	if !valid {
		t.Fatal("entry was rejected")
	}
	if rule.MatchType != "keyword" || rule.MatchPattern != "雨,港口" || !rule.Enabled || rule.Priority != 8 {
		t.Fatalf("unexpected primary mapping: %+v", rule)
	}
	if len(rule.SecondaryKeys) != 2 || rule.Selective == nil || !*rule.Selective || rule.CaseSensitive == nil || !*rule.CaseSensitive {
		t.Fatalf("selective metadata missing: %+v", rule)
	}
	if rule.CharacterBookEntryName != "港口雨夜" || string(rule.CharacterBookEntryID) != "17" || rule.SourceEntryIndex != 3 || rule.InsertionOrder != 12 {
		t.Fatalf("roundtrip metadata missing: %+v", rule)
	}
	exported := cardBookEntry(rule)
	if exported.Name != "港口雨夜" || len(exported.Keys) != 2 || len(exported.SecondaryKeys) != 2 || !exported.Enabled || exported.Priority != 8 {
		t.Fatalf("unexpected exported entry: %+v", exported)
	}
}

func TestCardBookWorldbookMappingHandlesConstantAndUnmatchableEntries(t *testing.T) {
	constant := true
	constantRule, valid := cardBookWorldbookRule(card.CharacterBookEntry{Content: "全局事实", Enabled: true, Constant: &constant}, "", 0)
	if !valid || constantRule.MatchType != "regex" || constantRule.MatchPattern != "(?s:.*)" || !constantRule.Enabled {
		t.Fatalf("constant mapping invalid: %+v", constantRule)
	}
	blockedRule, valid := cardBookWorldbookRule(card.CharacterBookEntry{Content: "无触发词", Enabled: true}, "", 1)
	if !valid || blockedRule.Enabled || blockedRule.MatchPattern != "a^" {
		t.Fatalf("unmatchable mapping invalid: %+v", blockedRule)
	}
	if cardWorldbookRequestID("import", 9) != cardWorldbookRequestID("import", 9) {
		t.Fatal("request id is not deterministic")
	}
	if cardWorldbookRequestID("import", 9) == cardWorldbookRequestID("import", 10) {
		t.Fatal("request id ignores entry index")
	}
}
