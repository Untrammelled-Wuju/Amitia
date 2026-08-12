package uitree

import "testing"

func TestMatchNode_Text(t *testing.T) {
	node := &UINode{
		NodeID: "node_1",
		Text:   "Login Button",
	}

	req := FindRequest{Text: "Login", MatchMode: MatchModeContains}
	if !MatchNode(node, req) {
		t.Fatal("expected match for contains")
	}

	req = FindRequest{Text: "login", MatchMode: MatchModeContainsCI}
	if !MatchNode(node, req) {
		t.Fatal("expected match for contains_ci")
	}

	req = FindRequest{Text: "login", MatchMode: MatchModeContains}
	if MatchNode(node, req) {
		t.Fatal("expected no match for case-sensitive contains")
	}

	req = FindRequest{Text: "Login Button", MatchMode: MatchModeExact}
	if !MatchNode(node, req) {
		t.Fatal("expected match for exact")
	}

	req = FindRequest{Text: "Register", MatchMode: MatchModeContains}
	if MatchNode(node, req) {
		t.Fatal("expected no match")
	}
}

func TestMatchNode_ResourceID(t *testing.T) {
	node := &UINode{
		NodeID:     "node_1",
		ResourceID: "com.example:id/submit",
	}

	req := FindRequest{ResourceID: "submit"}
	if !MatchNode(node, req) {
		t.Fatal("expected match for resourceId contains")
	}

	req = FindRequest{ResourceID: "cancel"}
	if MatchNode(node, req) {
		t.Fatal("expected no match")
	}
}

func TestMatchNode_Role(t *testing.T) {
	node := &UINode{
		NodeID: "node_1",
		Role:   "button",
	}

	req := FindRequest{Role: "button"}
	if !MatchNode(node, req) {
		t.Fatal("expected match for role")
	}

	req = FindRequest{Role: "input"}
	if MatchNode(node, req) {
		t.Fatal("expected no match")
	}
}

func TestMatchNode_Boolean(t *testing.T) {
	clickable := true
	editable := false

	node := &UINode{
		NodeID:    "node_1",
		Clickable: true,
		Editable:  false,
	}

	req := FindRequest{Clickable: &clickable}
	if !MatchNode(node, req) {
		t.Fatal("expected match for clickable")
	}

	req = FindRequest{Editable: &editable}
	if !MatchNode(node, req) {
		t.Fatal("expected match for editable=false")
	}

	req = FindRequest{Editable: &clickable}
	if MatchNode(node, req) {
		t.Fatal("expected no match for editable=true")
	}
}

func TestMatchNode_MultipleConditions(t *testing.T) {
	node := &UINode{
		NodeID:    "node_1",
		Text:      "Submit",
		Role:      "button",
		Clickable: true,
	}

	clickable := true
	req := FindRequest{
		Text:      "Submit",
		Role:      "button",
		Clickable: &clickable,
	}
	if !MatchNode(node, req) {
		t.Fatal("expected match for multiple conditions")
	}

	req.Text = "Cancel"
	if MatchNode(node, req) {
		t.Fatal("expected no match when one condition fails")
	}
}

func TestFilterNodes(t *testing.T) {
	nodes := []UINode{
		{NodeID: "node_1", Text: "Login", Role: "button"},
		{NodeID: "node_2", Text: "Username", Role: "input"},
		{NodeID: "node_3", Text: "Password", Role: "input"},
		{NodeID: "node_4", Text: "Submit", Role: "button"},
	}

	req := FindRequest{Role: "button"}
	result := FilterNodes(nodes, req)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}

	req = FindRequest{Text: "Login", MatchMode: MatchModeExact}
	result = FilterNodes(nodes, req)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0] != "node_1" {
		t.Fatalf("expected node_1, got %s", result[0])
	}
}

func TestFilterNodes_Limit(t *testing.T) {
	nodes := []UINode{
		{NodeID: "node_1", Role: "button"},
		{NodeID: "node_2", Role: "button"},
		{NodeID: "node_3", Role: "button"},
		{NodeID: "node_4", Role: "button"},
	}

	req := FindRequest{Role: "button", Limit: 2}
	result := FilterNodes(nodes, req)
	if len(result) != 2 {
		t.Fatalf("expected 2 results with limit, got %d", len(result))
	}
}

func TestFilterNodes_DefaultLimit(t *testing.T) {
	nodes := make([]UINode, 200)
	for i := range nodes {
		nodes[i] = UINode{NodeID: "node_" + string(rune('a'+i%26)) + string(rune('0'+i/26)), Role: "button"}
	}

	req := FindRequest{Role: "button"}
	result := FilterNodes(nodes, req)
	if len(result) > DefaultMaxFindLimit {
		t.Fatalf("expected at most %d results, got %d", DefaultMaxFindLimit, len(result))
	}
}
