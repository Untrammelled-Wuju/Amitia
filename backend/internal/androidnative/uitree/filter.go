package uitree

import "strings"

const (
	MatchModeExact      = "exact"
	MatchModeContains   = "contains"
	MatchModeContainsCI = "contains_ci"
)

func MatchNode(node *UINode, req FindRequest) bool {
	if req.Text != "" {
		if !matchText(node.Text, req.Text, req.MatchMode) && !matchText(node.ContentDescription, req.Text, req.MatchMode) {
			return false
		}
	}

	if req.ResourceID != "" {
		if !matchText(node.ResourceID, req.ResourceID, req.MatchMode) {
			return false
		}
	}

	if req.ClassName != "" {
		if !matchText(node.ClassName, req.ClassName, req.MatchMode) {
			return false
		}
	}

	if req.Role != "" && node.Role != req.Role {
		return false
	}

	if req.Clickable != nil && node.Clickable != *req.Clickable {
		return false
	}

	if req.Editable != nil && node.Editable != *req.Editable {
		return false
	}

	if req.Scrollable != nil && node.Scrollable != *req.Scrollable {
		return false
	}

	if req.Visible != nil && node.VisibleToUser != *req.Visible {
		return false
	}

	return true
}

func matchText(haystack, needle, mode string) bool {
	switch mode {
	case MatchModeExact:
		return haystack == needle
	case MatchModeContains:
		return strings.Contains(haystack, needle)
	case MatchModeContainsCI:
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	default:
		return strings.Contains(haystack, needle)
	}
}

func FilterNodes(nodes []UINode, req FindRequest) []string {
	var matched []string
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultMaxFindLimit
	}

	for i := range nodes {
		if limit > 0 && len(matched) >= limit {
			break
		}
		if MatchNode(&nodes[i], req) {
			matched = append(matched, nodes[i].NodeID)
		}
	}

	return matched
}
