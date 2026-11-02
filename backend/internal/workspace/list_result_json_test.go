package workspace

import (
	"encoding/json"
	"testing"
)

func TestListResultJSONContract(t *testing.T) {
	raw, err := json.Marshal(ListResult{
		Entries:    []WorkspaceEntry{{URI: "amitia://workspace/@one/file.txt", Name: "file.txt", Type: WorkspaceEntryTypeFile}},
		NextCursor: "next",
		HasMore:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["entries"]; !ok {
		t.Fatalf("missing entries: %s", raw)
	}
	if body["nextCursor"] != "next" {
		t.Fatalf("unexpected nextCursor: %s", raw)
	}
	if body["hasMore"] != true {
		t.Fatalf("unexpected hasMore: %s", raw)
	}
	if _, ok := body["Entries"]; ok {
		t.Fatalf("unexpected exported field name: %s", raw)
	}
}
