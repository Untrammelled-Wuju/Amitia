package coordination_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedSemanticSearchEnforcesSourceRoleModelAndTTL(t *testing.T) {
	db, _ := setup(t)
	add := func(owner, role, id, model string, values []float32, disabled bool, expiry string) {
		t.Helper()
		memory, _ := json.Marshal(map[string]any{"content": map[string]any{"text": id}, "allowContextUse": !disabled, "expiresAt": expiry})
		vector, _ := json.Marshal(map[string]any{"content": map[string]any{"values": values, "modelFingerprint": model}})
		for _, row := range []struct {
			kind, id, source string
			body             []byte
		}{{"memory", id, "", memory}, {"vector", "v/" + id, id, vector}} {
			if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,role_id,kind,resource_id,source_id,revision,body,updated_at) VALUES(?,?,?,?,?,1,?,'2020-01-01T00:00:00Z')`, owner, role, row.kind, row.id, row.source, row.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("a", "role", "old-relevant", "model", []float32{1, 0}, false, "")
	add("a", "role", "less-relevant", "model", []float32{0.4, 0.9}, false, "")
	add("b", "role", "other-device", "model", []float32{1, 0}, false, "")
	add("a", "another-role", "other-role", "model", []float32{1, 0}, false, "")
	add("a", "role", "wrong-model", "other-model", []float32{1, 0}, false, "")
	add("a", "role", "wrong-dimension", "model", []float32{1, 0, 0}, false, "")
	add("a", "role", "disabled", "model", []float32{1, 0}, true, "")
	add("a", "role", "expired", "model", []float32{1, 0}, false, time.Now().Add(-time.Hour).Format(time.RFC3339))
	store := coordination.NewOwnershipStore(db, "a")
	query := coordination.DataQuery{Vector: []float32{1, 0}, VectorModel: "model"}
	rows, err := store.SemanticSearch(t.Context(), "role", query, 1)
	if err != nil || len(rows) != 2 || rows[0].ID != "old-relevant" || rows[1].SourceID != rows[0].ID {
		t.Fatalf("semantic results=%+v err=%v", rows, err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE kernel_device_owned_resources SET deleted=1,body='null' WHERE owner_id='a' AND kind='memory' AND resource_id='old-relevant'`); err != nil {
		t.Fatal(err)
	}
	rows, err = store.SemanticSearch(t.Context(), "role", query, 1)
	if err != nil || len(rows) != 2 || rows[0].ID != "less-relevant" {
		t.Fatalf("deleted source still retrieved: %+v %v", rows, err)
	}
	query.Vector = []float32{float32(math.Inf(1)), 0}
	if _, err := store.SemanticSearch(t.Context(), "role", query, 1); err == nil {
		t.Fatal("nonfinite query accepted")
	}
}
