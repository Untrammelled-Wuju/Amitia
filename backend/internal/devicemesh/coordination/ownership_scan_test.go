package coordination_test

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedResourceBodySQLStorageTypes(t *testing.T) {
	for _, storage := range []string{"TEXT", "BLOB"} {
		t.Run(storage, func(t *testing.T) {
			db, _ := setup(t)
			body := `{"title":"中文会话","createdAt":"2026-10-23T00:00:00Z"}`
			_, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES('a','conversation','chat','role',1,CAST(? AS `+storage+`),'2026-10-23T00:00:00Z')`, body)
			if err != nil {
				t.Fatal(err)
			}
			store := coordination.NewOwnershipStore(db, "a")
			rows, err := store.List(t.Context(), "conversation", "role", false)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || string(rows[0].Body) != body {
				t.Fatalf("list=%v", rows)
			}
			page, _, err := store.ListPage(t.Context(), "conversation", "role", coordination.DataQuery{ListConversations: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(page) != 1 || string(page[0].Body) != body {
				t.Fatalf("page=%v", page)
			}
			row, err := store.Get(t.Context(), "conversation", "chat")
			if err != nil {
				t.Fatal(err)
			}
			if row == nil || string(row.Body) != body {
				t.Fatalf("get=%v", row)
			}
		})
	}
}

func TestOwnedSemanticSearchTextBody(t *testing.T) {
	db, _ := setup(t)
	for _, row := range []struct{ kind, id, source, body string }{
		{"memory", "memory", "", `{"content":{"text":"中文记忆"}}`},
		{"vector", "vector", "memory", `{"content":{"values":[1,0],"modelFingerprint":"model"}}`},
	} {
		if _, err := db.Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,source_id,role_id,revision,body,updated_at) VALUES('a',?,?,?,'role',1,CAST(? AS TEXT),'2026-10-23T00:00:00Z')`, row.kind, row.id, row.source, row.body); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := coordination.NewOwnershipStore(db, "a").SemanticSearch(t.Context(), "role", coordination.DataQuery{Vector: []float32{1, 0}, VectorModel: "model"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != "memory" || rows[1].ID != "vector" {
		t.Fatalf("results=%v", rows)
	}
}
