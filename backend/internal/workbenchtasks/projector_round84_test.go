package workbenchtasks

import (
	"context"
	"github.com/u-ai/backend/internal/interaction"
	"testing"
)

func TestRound84WorkbenchTaskContainsDurableParentInteractionIdentity(t *testing.T) {
	parent := round84BudgetParent()
	items, err := List(context.Background(), taskRecordReader{records: []*interaction.InteractionRecord{parent}}, "space-a", "conv-a")
	if err != nil || len(items.Tasks) != 1 || items.Tasks[0].ParentInteractionID != parent.ID || items.Tasks[0].CoordinationID != "coord-a" {
		t.Fatalf("budget evidence cannot be linked to durable parent: %+v %v", items, err)
	}
}
