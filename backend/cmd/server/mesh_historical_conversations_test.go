package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestMeshHistoricalConversationPages(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: "device-a", CoreID: "core", RoleOwnerID: "core", ResourceOwnerID: "core", RoleID: "cloud-role", Coordinated: true, TargetProviderEpoch: 2, TargetPermissionRevision: 3, ModeRevision: 4}
	for i := 0; i < 151; i++ {
		id := fmt.Sprintf("owned-%04d", i)
		raw, err := json.Marshal(map[string]any{"id": id, "title": id})
		if err != nil {
			t.Fatal(err)
		}
		_, err = p.services.KernelContainer.DeviceRegistry.Database().ExecContext(t.Context(), `INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,body,updated_at) VALUES(?,?,?,?,?,?,?)`, p.ownerID, "conversation", id, "old-role", 1, raw, "2026-10-04T01:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		legacy := chat.Conversation{ID: fmt.Sprintf("legacy-%04d", i), SpaceID: p.legacySpaceID, Title: "old", Channel: "web", UpdatedAt: "2026-10-04T01:00:00Z"}
		if err := p.services.DB.Create(&legacy).Error; err != nil {
			t.Fatal(err)
		}
	}
	query := coordination.DataQuery{ListConversations: true, Limit: 50}
	seen := map[string]bool{}
	first := ""
	for i := 0; i < 5; i++ {
		page, err := p.HistoricalConversationPage(t.Context(), scope, query)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range page.Conversations {
			var row struct {
				ID      string `json:"id"`
				OwnerID string `json:"ownerId"`
			}
			if err := json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			if row.OwnerID != p.ownerID || seen[row.ID] {
				t.Fatalf("invalid page row: %+v", row)
			}
			seen[row.ID] = true
		}
		if i == 0 {
			first = page.NextCursor
		}
		query.HistoricalListCursor = page.NextCursor
		if page.NextCursor == "" {
			break
		}
	}
	if len(seen) != 302 || first == "" {
		t.Fatalf("historical list truncated: %d", len(seen))
	}
	query.HistoricalListCursor = first
	scope.TargetPermissionRevision++
	if _, err := p.HistoricalConversationPage(t.Context(), scope, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("old permission cursor accepted: %v", err)
	}
	scope.TargetPermissionRevision--
	scope.CoreID, scope.ResourceOwnerID, scope.RoleOwnerID = "other-core", "other-core", "other-core"
	if _, err := p.HistoricalConversationPage(t.Context(), scope, query); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("foreign Core cursor accepted: %v", err)
	}
}
