package uitree

import "testing"

func TestUiAutomatorXmlParserParsesStructuredNodes(t *testing.T) {
	xml := `<?xml version='1.0' encoding='UTF-8' standalone='yes' ?><hierarchy rotation="0"><node index="0" text="" resource-id="root" class="android.widget.FrameLayout" package="com.example" content-desc="" checkable="false" checked="false" clickable="false" enabled="true" focusable="false" focused="false" scrollable="false" long-clickable="false" password="false" selected="false" bounds="[0,0][1080,1920]"><node index="0" text="Submit" resource-id="com.example:id/submit" class="android.widget.Button" package="com.example" content-desc="Submit order" checkable="false" checked="false" clickable="true" enabled="true" focusable="true" focused="false" scrollable="false" long-clickable="false" password="false" selected="false" bounds="[100,200][400,300]" /></node></hierarchy>`

	windows, nodes, err := (UiAutomatorXmlParser{}).Parse(xml, SourceTypeADB)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if got := nodes[1]["text"]; got != "Submit" {
		t.Fatalf("expected Submit text, got %#v", got)
	}
	if got := nodes[1]["resourceId"]; got != "com.example:id/submit" {
		t.Fatalf("unexpected resourceId: %#v", got)
	}
	if got := nodes[1]["clickable"]; got != true {
		t.Fatalf("expected clickable=true, got %#v", got)
	}
	if got := nodes[1]["parentId"]; got != nodes[0]["nodeId"] {
		t.Fatal("expected child parentId to match root nodeId")
	}
}

func TestUiAutomatorXmlParserRejectsInvalidXML(t *testing.T) {
	if _, _, err := (UiAutomatorXmlParser{}).Parse("<hierarchy><node", SourceTypeRoot); err == nil {
		t.Fatal("expected invalid XML error")
	}
}
