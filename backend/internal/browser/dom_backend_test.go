package browser

import "testing"

func TestConvertCDPDOMNodePreservesNestedTree(t *testing.T) {
	node := convertCDPDOMNode(cdpDOMNode{
		NodeID:   1,
		NodeName: "#document",
		Children: []cdpDOMNode{{
			NodeID:   2,
			NodeName: "html",
			Children: []cdpDOMNode{{
				NodeID:   3,
				NodeName: "body",
				Children: []cdpDOMNode{{
					NodeID:    4,
					NodeName:  "#text",
					NodeValue: "Example Domain",
				}},
			}},
		}},
	})
	if len(node.Children) != 1 || len(node.Children[0].Children) != 1 || len(node.Children[0].Children[0].Children) != 1 {
		t.Fatalf("nested DOM tree was not preserved: %#v", node)
	}
	if node.Children[0].Children[0].Children[0].NodeValue != "Example Domain" {
		t.Fatalf("unexpected text node: %#v", node)
	}
}
