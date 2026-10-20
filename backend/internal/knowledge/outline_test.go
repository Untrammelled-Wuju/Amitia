package knowledge

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceOutlineIsTenantScopedPaginatedAndBodyFree(t *testing.T) {
	repo := newKnowledgeTestRepository(t)
	seedReadableKnowledgeSource(t, repo, "tenant-a", "source-a", 1, RevisionStatusReady)
	seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 0, "unit-0", "SECRET BODY ZERO", `{"page":1}`)
	seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 1, "unit-1", "SECRET BODY ONE", `{"page":2}`)
	seedReaderUnit(t, repo, "tenant-a", "source-a", 1, 2, "unit-2", "SECRET BODY TWO", `{"page":3}`)

	svc := &service{repo: repo}
	first, err := svc.GetSourceOutline(context.Background(), "tenant-a", "source-a", 1, -1, 2)
	if err != nil {
		t.Fatalf("outline first page: %v", err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != "unit-0" || first.Items[1].ID != "unit-1" || first.NextAfterOrdinal == nil {
		t.Fatalf("unexpected first outline page: %#v", first)
	}
	if first.Items[0].Locator["page"] != float64(1) && first.Items[0].Locator["page"] != 1 {
		t.Fatalf("locator missing from outline: %#v", first.Items[0])
	}
	payload, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal outline: %v", err)
	}
	if strings.Contains(string(payload), "SECRET BODY") {
		t.Fatalf("outline must never expose canonical unit body: %s", payload)
	}
	second, err := svc.GetSourceOutline(context.Background(), "tenant-a", "source-a", 1, *first.NextAfterOrdinal, 2)
	if err != nil {
		t.Fatalf("outline second page: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "unit-2" || second.NextAfterOrdinal != nil {
		t.Fatalf("unexpected second outline page: %#v", second)
	}
	if _, err := svc.GetSourceOutline(context.Background(), "tenant-b", "source-a", 1, -1, 2); err == nil {
		t.Fatal("cross-tenant source outline must be rejected")
	}
}
