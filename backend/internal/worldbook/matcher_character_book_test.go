package worldbook

import "testing"

func TestSelectiveCharacterBookEntryRequiresSecondaryKey(t *testing.T) {
	selective := true
	caseSensitive := true
	rule := WorldBookEntry{ID: "port", MatchType: "keyword", MatchPattern: "雨,港口", MatchScope: "user_message", InjectContent: "港口关闭", SecondaryKeys: []string{"灯塔"}, Selective: &selective, CaseSensitive: &caseSensitive}
	if matches := MatchRules([]WorldBookEntry{rule}, "雨夜港口", ""); len(matches) != 0 {
		t.Fatalf("selective rule matched without secondary key: %+v", matches)
	}
	if matches := MatchRules([]WorldBookEntry{rule}, "雨夜港口灯塔", ""); len(matches) != 1 {
		t.Fatalf("selective rule did not match with secondary key: %+v", matches)
	}
}

func TestCharacterBookCaseSensitiveKeyword(t *testing.T) {
	caseSensitive := true
	rule := WorldBookEntry{ID: "port", MatchType: "keyword", MatchPattern: "Port", MatchScope: "user_message", InjectContent: "港口", CaseSensitive: &caseSensitive}
	if matches := MatchRules([]WorldBookEntry{rule}, "port closed", ""); len(matches) != 0 {
		t.Fatalf("case-sensitive rule matched lowercase: %+v", matches)
	}
	if matches := MatchRules([]WorldBookEntry{rule}, "Port closed", ""); len(matches) != 1 {
		t.Fatalf("case-sensitive rule did not match exact case: %+v", matches)
	}
}
