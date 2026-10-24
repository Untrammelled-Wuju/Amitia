package workbenchtasks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type round83ParentReader struct {
	parent *interaction.InteractionRecord
}

func (r round83ParentReader) Get(_ context.Context, stringID string) (*interaction.InteractionRecord, bool, error) {
	return r.parent, r.parent != nil && r.parent.ID == stringID, nil
}

func (r round83ParentReader) ListByScope(context.Context, interaction.InteractionScope) ([]*interaction.InteractionRecord, error) {
	return []*interaction.InteractionRecord{r.parent}, nil
}

type round83BudgetReader struct {
	calls  int
	revoke func()
}

func (r *round83BudgetReader) PendingOwnedModelReconciliationPage(_ context.Context, scope mindruntime.OwnedBudgetReconciliationScope, _ int64, _ int) ([]mindruntime.ModelUsageRecord, error) {
	r.calls++
	if scope.SpaceID != "space-a" || scope.RoleID != "role" || scope.OwnerID != "owner-a" || scope.ParentInteractionID == "" || scope.CoordinationID != "coord-a" {
		panic("unscoped model budget read")
	}
	if r.revoke != nil {
		r.revoke()
	}
	return []mindruntime.ModelUsageRecord{{Sequence: 10, ID: "model-1", Status: "interrupted", ReservedTokens: 128}}, nil
}

func (r *round83BudgetReader) ReservedOwnedToolReconciliationPage(_ context.Context, scope mindruntime.OwnedBudgetReconciliationScope, _ time.Time, _ string, _ int) ([]mindruntime.OwnedToolCallRecord, error) {
	r.calls++
	if r.revoke != nil {
		r.revoke()
	}
	if scope.SpaceID != "space-a" || scope.RoleID != "role" || scope.OwnerID != "owner-a" || scope.CoordinationID != "coord-a" {
		panic("unscoped tool budget read")
	}
	return []mindruntime.OwnedToolCallRecord{{EffectID: "effect-1", ToolName: "read_file", CreatedAt: time.Now().UTC()}}, nil
}

func round83BudgetRequest(h *Handler, actor *auth.ActorContext, parentID string, kind string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	address := "/api/workbench/tasks/budget-reconciliation?characterId=role&conversationId=conv-a&conversationOwnerId=device-a&expectedCoreId=space-a&coordinationId=coord-a&parentInteractionId=" + url.QueryEscape(parentID) + "&kind=" + url.QueryEscape(kind)
	request := httptest.NewRequest(http.MethodGet, address, nil)
	if actor != nil {
		request = request.WithContext(auth.WithActor(request.Context(), actor))
	}
	ctx.Request = request
	h.budgetReconciliation(ctx)
	return response
}

func TestRound83WorkbenchBudgetReconciliationRequiresAuthorizedParent(t *testing.T) {
	parent := taskRecord("space-a", "conv-a", "coord-a", "running")
	parent.Scope.CharacterID = "role"
	parent.RecoveryDescriptor.MultiAgent.ExecutionCoreID = "space-a"
	parent.RecoveryDescriptor.MultiAgent.ResourceOwnerID = "owner-a"
	parent.RecoveryDescriptor.ComputeFingerprint()
	budget := &round83BudgetReader{}
	readAllowed := true
	h := NewHandler(round83ParentReader{parent: parent}).WithBudgetInspection(budget).WithConversationAccess(func(ctx context.Context, actor *auth.ActorContext, query ConversationQuery, read func(context.Context, QueryContext) error) error {
		if actor.DeviceID != "device-a" {
			return coordination.ErrWrongOwner
		}
		return read(ctx, QueryContext{ExecutionScope: coordination.ExecutionScope{SpaceID: "space-a", RoleID: "role", ResourceOwnerID: "owner-a", CoreID: "space-a"}, ConversationID: query.ConversationID, CanReadRecord: func(context.Context, *interaction.InteractionRecord) (bool, error) { return readAllowed, nil }})
	})
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: "device-a"}
	if got := round83BudgetRequest(h, nil, parent.ID, "model"); got.Code != http.StatusForbidden {
		t.Fatalf("anonymous reconciliation admitted: %d", got.Code)
	}
	if got := round83BudgetRequest(h, actor, "unknown-parent", "model"); got.Code != http.StatusNotFound {
		t.Fatalf("unknown parent response %d: %s", got.Code, got.Body.String())
	}
	if budget.calls != 0 {
		t.Fatal("budget was read without a valid parent")
	}
	readAllowed = false
	if got := round83BudgetRequest(h, actor, parent.ID, "model"); got.Code != http.StatusForbidden {
		t.Fatalf("unauthorized owner admitted: %d: %s", got.Code, got.Body.String())
	}
	readAllowed = true
	result := round83BudgetRequest(h, actor, parent.ID, "model")
	if result.Code != http.StatusOK || result.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("valid model evidence response %d: %s", result.Code, result.Body.String())
	}
	var body struct {
		Kind    string                         `json:"kind"`
		Source  string                         `json:"source"`
		Records []mindruntime.ModelUsageRecord `json:"records"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &body); err != nil || body.Kind != "model" || body.Source != "durable-budget-ledger" || len(body.Records) != 1 || body.Records[0].ID != "model-1" {
		t.Fatalf("invalid evidence: %+v %v", body, err)
	}
	if got := round83BudgetRequest(h, actor, parent.ID, "tool"); got.Code != http.StatusOK {
		t.Fatalf("valid tool evidence response %d: %s", got.Code, got.Body.String())
	}
	if budget.calls != 2 {
		t.Fatalf("unexpected budget read count %d", budget.calls)
	}
}

func TestRound83WorkbenchBudgetReadRejectsParentRevisionChange(t *testing.T) {
	parent := taskRecord("space-a", "conv-a", "coord-a", "running")
	parent.Scope.CharacterID = "role"
	parent.RecoveryDescriptor.MultiAgent.ExecutionCoreID = "space-a"
	parent.RecoveryDescriptor.MultiAgent.ResourceOwnerID = "owner-a"
	parent.RecoveryDescriptor.ComputeFingerprint()
	reader := &round83BudgetReader{revoke: func() { parent.StatusVersion++ }}
	h := NewHandler(round83ParentReader{parent: parent}).WithBudgetInspection(reader).WithConversationAccess(func(ctx context.Context, _ *auth.ActorContext, query ConversationQuery, read func(context.Context, QueryContext) error) error {
		return read(ctx, QueryContext{ExecutionScope: coordination.ExecutionScope{SpaceID: "space-a", RoleID: "role", ResourceOwnerID: "owner-a", CoreID: "space-a"}, ConversationID: query.ConversationID, CanReadRecord: func(context.Context, *interaction.InteractionRecord) (bool, error) { return true, nil }})
	})
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: "device-a"}
	if got := round83BudgetRequest(h, actor, parent.ID, "model"); got.Code != http.StatusConflict {
		t.Fatalf("revoked parent evidence exposed %d: %s", got.Code, got.Body.String())
	}
}

func TestRound83BudgetRouteOnlyRegistersWithDurableLedger(t *testing.T) {
	withBudget := gin.New()
	NewHandler(round83ParentReader{}).WithBudgetInspection(&round83BudgetReader{}).RegisterRoutes(withBudget.Group("/api"))
	found := false
	for _, route := range withBudget.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/workbench/tasks/budget-reconciliation" {
			found = true
		}
	}
	if !found {
		t.Fatal("budget evidence route not registered")
	}
	withoutBudget := gin.New()
	NewHandler(round83ParentReader{}).RegisterRoutes(withoutBudget.Group("/api"))
	for _, route := range withoutBudget.Routes() {
		if route.Path == "/api/workbench/tasks/budget-reconciliation" {
			t.Fatal("budget endpoint must be disabled without a ledger")
		}
	}
}
