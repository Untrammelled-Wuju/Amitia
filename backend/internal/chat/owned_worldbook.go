package chat

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/worldbook"
)

func ownedWorldbookMatches(scope coordination.ExecutionScope, resources []coordination.Resource, text string) []map[string]any {
	rules := make([]worldbook.WorldBookEntry, 0)
	metadata := map[string]struct {
		revision int64
		roleID   string
	}{}
	for _, resource := range resources {
		if resource.Kind != "worldbook" || resource.Deleted {
			continue
		}
		if resource.RoleID != scope.RoleID && resource.RoleID != coordination.GlobalWorldbookRole {
			continue
		}
		var stored business.WorldbookRule
		if json.Unmarshal(resource.Body, &stored) != nil || !stored.Enabled {
			continue
		}
		rules = append(rules, worldbook.WorldBookEntry{
			ID: resource.ID, MatchType: stored.MatchType, MatchPattern: stored.MatchPattern,
			MatchScope: stored.MatchScope, InjectContent: stored.InjectContent,
			Priority: stored.Priority, SecondaryKeys: stored.SecondaryKeys,
			CaseSensitive: stored.CaseSensitive, Selective: stored.Selective,
			Constant: stored.Constant,
		})
		metadata[resource.ID] = struct {
			revision int64
			roleID   string
		}{resource.Revision, resource.RoleID}
	}
	matches := worldbook.MatchRules(rules, text, "")
	result := make([]map[string]any, 0, len(matches))
	for _, match := range matches {
		meta := metadata[match.Entry.ID]
		ruleScope := "role"
		if meta.roleID == coordination.GlobalWorldbookRole {
			ruleScope = "global"
		}
		result = append(result, map[string]any{
			"id": match.Entry.ID, "scope": ruleScope, "revision": meta.revision,
			"matchScope": match.MatchScope, "hitText": match.HitText,
			"content":  strings.TrimSpace(match.Entry.InjectContent),
			"priority": match.Entry.Priority,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, _ := result[i]["priority"].(int)
		right, _ := result[j]["priority"].(int)
		if left != right {
			return left > right
		}
		return result[i]["id"].(string) < result[j]["id"].(string)
	})
	return result
}
