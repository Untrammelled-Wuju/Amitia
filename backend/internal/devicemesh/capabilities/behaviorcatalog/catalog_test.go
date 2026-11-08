package behaviorcatalog

import (
	"errors"
	"testing"
)

func TestParseRejectsDuplicateAndInvalidSchema(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"revision":"1","behaviors":[{"id":"wave","name":"Wave","inputSchema":{}},{"id":"wave","name":"Again","inputSchema":{}}]}`,
		`{"version":1,"revision":"1","behaviors":[{"id":"bad id","name":"Bad","inputSchema":{}}]}`,
		`{"version":1,"revision":"1","behaviors":[{"id":"wave","name":"Wave","inputSchema":[]}]}`,
	} {
		if _, err := Parse(map[string]string{MetadataKey: raw}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected invalid catalog for %s: %v", raw, err)
		}
	}
}

func TestParseSortsBehaviors(t *testing.T) {
	catalog, err := Parse(map[string]string{MetadataKey: `{
		"version":1,
		"revision":"rev-1",
		"behaviors":[
			{"id":"pause","name":"Pause","serviceId":"game","method":"game.pause","inputSchema":{"type":"object"}},
			{"id":"load_save","name":"Load","serviceId":"game","method":"game.load","inputSchema":{"type":"object"},"risk":"high","requiresConfirmation":true}
		]
	}`})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Behaviors) != 2 || catalog.Behaviors[0].ID != "load_save" || catalog.Behaviors[1].ID != "pause" {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	if behavior, ok := catalog.Find("pause"); !ok || behavior.Method != "game.pause" {
		t.Fatalf("pause behavior missing: %+v %v", behavior, ok)
	}
}
