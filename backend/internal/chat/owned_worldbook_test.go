package chat

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedWorldbookMatchesOnlyEnabledRulesAndKeepsUntrustedContextStructured(t *testing.T) {
	scope := coordination.ExecutionScope{RoleID: "one"}
	encode := func(rule business.WorldbookRule) json.RawMessage {
		data, err := json.Marshal(rule)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	resources := []coordination.Resource{
		{Kind: "worldbook", ID: "role", RoleID: "one", Revision: 3, Body: encode(business.WorldbookRule{ID: "role", MatchType: "keyword", MatchPattern: "晨星", MatchScope: "full_context", InjectContent: "角色规则", Priority: 2, Enabled: true})},
		{Kind: "worldbook", ID: "global", RoleID: coordination.GlobalWorldbookRole, Revision: 4, Body: encode(business.WorldbookRule{ID: "global", MatchType: "keyword", MatchPattern: "晨星联盟", MatchScope: "full_context", InjectContent: "全局规则", Priority: 1, Enabled: true})},
		{Kind: "worldbook", ID: "disabled", RoleID: "one", Revision: 1, Body: encode(business.WorldbookRule{ID: "disabled", MatchType: "keyword", MatchPattern: "晨星", InjectContent: "不应注入", Enabled: false})},
		{Kind: "worldbook", ID: "other", RoleID: "two", Revision: 1, Body: encode(business.WorldbookRule{ID: "other", MatchType: "keyword", MatchPattern: "晨星联盟", InjectContent: "其他角色", Enabled: true})},
	}
	matches := ownedWorldbookMatches(scope, resources, "请说明晨星和晨星联盟")
	if len(matches) != 2 || matches[0]["id"] != "role" || matches[1]["id"] != "global" {
		t.Fatalf("unexpected worldbook matches: %#v", matches)
	}
	if matches[0]["revision"] != int64(3) || matches[1]["revision"] != int64(4) || matches[0]["scope"] != "role" || matches[1]["scope"] != "global" {
		t.Fatalf("worldbook authority metadata missing: %#v", matches)
	}
}
