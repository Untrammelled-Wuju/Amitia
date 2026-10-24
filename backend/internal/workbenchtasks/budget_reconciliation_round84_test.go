package workbenchtasks

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func round84BudgetAccess(parent *interaction.InteractionRecord, reader *round83BudgetReader, canRead *bool) *Handler {
	return NewHandler(round83ParentReader{parent: parent}).WithBudgetInspection(reader).WithConversationAccess(func(ctx context.Context, _ *auth.ActorContext, query ConversationQuery, read func(context.Context, QueryContext) error) error {
		return read(ctx, QueryContext{
			ExecutionScope: coordination.ExecutionScope{SpaceID: "space-a", RoleID: "role", ResourceOwnerID: "owner-a", CoreID: "space-a"},
			ConversationID: query.ConversationID,
			CanReadRecord:  func(context.Context, *interaction.InteractionRecord) (bool, error) { return *canRead, nil },
		})
	})
}

func round84BudgetParent() *interaction.InteractionRecord {
	parent := taskRecord("space-a", "conv-a", "coord-a", "running")
	parent.Scope.CharacterID = "role"
	parent.RecoveryDescriptor.MultiAgent.ExecutionCoreID = "space-a"
	parent.RecoveryDescriptor.MultiAgent.ResourceOwnerID = "owner-a"
	parent.RecoveryDescriptor.ComputeFingerprint()
	return parent
}

func TestRound84WorkbenchRejectsReadRevocationWithoutParentVersionChange(t *testing.T) {
	for _, kind := range []string{"model", "tool"} {
		t.Run(kind, func(t *testing.T) {
			parent := round84BudgetParent()
			canRead := true
			budget := &round83BudgetReader{revoke: func() { canRead = false }}
			h := round84BudgetAccess(parent, budget, &canRead)
			actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: "device-a"}
			response := round83BudgetRequest(h, actor, parent.ID, kind)
			if response.Code != http.StatusForbidden || budget.calls != 1 {
				t.Fatalf("read revoked during query but evidence returned: code=%d body=%s calls=%d", response.Code, response.Body.String(), budget.calls)
			}
		})
	}
}

func TestRound84WorkbenchRejectsSilentParentScopeMutation(t *testing.T) {
	for _, mutate := range []struct {
		name string
		run  func(*interaction.InteractionRecord)
	}{
		{"role", func(parent *interaction.InteractionRecord) { parent.Scope.CharacterID = "foreign-role" }},
		{"space", func(parent *interaction.InteractionRecord) { parent.Scope.SpaceID = "foreign-space" }},
		{"core", func(parent *interaction.InteractionRecord) {
			parent.RecoveryDescriptor.MultiAgent.ExecutionCoreID = "foreign-core"
		}},
		{"conversation", func(parent *interaction.InteractionRecord) { parent.Scope.ConversationID = "foreign-conversation" }},
		{"status", func(parent *interaction.InteractionRecord) { parent.Status = "cancelled" }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			parent := round84BudgetParent()
			canRead := true
			budget := &round83BudgetReader{revoke: func() { mutate.run(parent) }}
			h := round84BudgetAccess(parent, budget, &canRead)
			actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: "device-a"}
			response := round83BudgetRequest(h, actor, parent.ID, "model")
			if response.Code != http.StatusConflict || budget.calls != 1 {
				t.Fatalf("parent scope mutated without version advance but evidence returned: code=%d body=%s calls=%d", response.Code, response.Body.String(), budget.calls)
			}
		})
	}
}

func TestRound84WorkbenchBudgetEvidenceProvidesVerifiedRequestIdentity(t *testing.T) {
	parent := round84BudgetParent()
	canRead := true
	h := round84BudgetAccess(parent, &round83BudgetReader{}, &canRead)
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: "device-a"}
	response := round83BudgetRequest(h, actor, parent.ID, "model")
	if response.Code != http.StatusOK {
		t.Fatalf("scoped evidence unavailable: %d %s", response.Code, response.Body.String())
	}
	var evidence struct {
		ParentInteractionID string `json:"parentInteractionId"`
		ConversationID      string `json:"conversationId"`
		QueryContext        struct {
			ExecutionScope coordination.ExecutionScope `json:"executionScope"`
			ConversationID string                      `json:"conversationId"`
		} `json:"queryContext"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.ParentInteractionID != parent.ID || evidence.ConversationID != "conv-a" || evidence.QueryContext.ConversationID != "conv-a" || evidence.QueryContext.ExecutionScope.RoleID != "role" || evidence.QueryContext.ExecutionScope.SpaceID != "space-a" || evidence.QueryContext.ExecutionScope.CoreID != "space-a" || evidence.QueryContext.ExecutionScope.ResourceOwnerID != "owner-a" {
		t.Fatalf("budget evidence does not retain request provenance: %+v", evidence)
	}
}
