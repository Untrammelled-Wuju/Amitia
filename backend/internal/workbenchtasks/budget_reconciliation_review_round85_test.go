package workbenchtasks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type round85ReviewReader struct {
	round83BudgetReader
	reviews []mindruntime.OwnedBudgetEvidenceReview
}

func (r *round85ReviewReader) AttachOwnedBudgetEvidence(ctx context.Context, review mindruntime.OwnedBudgetEvidenceReview) error {
	actor, _ := auth.FromContext(ctx)
	if actor == nil || actor.DeviceID.String() != review.Scope.OwnerID {
		return mindruntime.ErrOwnedBudgetReviewForbidden
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	r.reviews = append(r.reviews, review)
	return nil
}

func round85BudgetReviewRequest(t *testing.T, h *Handler, actor *auth.ActorContext, parent string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	address := "/api/workbench/tasks/budget-reconciliation/review?characterId=role&conversationId=conv-a&conversationOwnerId=device-a&expectedCoreId=space-a&coordinationId=coord-a&parentInteractionId=" + url.QueryEscape(parent)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, address, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if actor != nil {
		req = req.WithContext(auth.WithActor(req.Context(), actor))
	}
	ctx.Request = req
	h.budgetReconciliationReview(ctx)
	return response
}

func TestRound85WorkbenchReviewRequiresOwnerAndParentScope(t *testing.T) {
	parent := taskRecord("space-a", "conv-a", "coord-a", "running")
	parent.Scope.CharacterID = "role"
	parent.RecoveryDescriptor.MultiAgent.ExecutionCoreID = "space-a"
	parent.RecoveryDescriptor.MultiAgent.ResourceOwnerID = "owner-a"
	parent.RecoveryDescriptor.ComputeFingerprint()
	writer := &round85ReviewReader{}
	canRead := true
	h := NewHandler(round83ParentReader{parent: parent}).WithBudgetInspection(writer).WithConversationAccess(func(ctx context.Context, _ *auth.ActorContext, query ConversationQuery, read func(context.Context, QueryContext) error) error {
		return read(ctx, QueryContext{ExecutionScope: coordination.ExecutionScope{SpaceID: "space-a", RoleID: "role", ResourceOwnerID: "owner-a", CoreID: "space-a"}, ConversationID: query.ConversationID, CanReadRecord: func(context.Context, *interaction.InteractionRecord) (bool, error) { return canRead, nil }})
	})
	actor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: runtimeidentity.DeviceID("device-a")}
	body := map[string]any{"kind": "model", "recordId": "record-1", "evidenceRef": "case:provider-1234"}
	if got := round85BudgetReviewRequest(t, h, actor, parent.ID, body); got.Code != http.StatusForbidden {
		t.Fatalf("read permission allowed writing: %d", got.Code)
	}
	actor.DeviceID = runtimeidentity.DeviceID("owner-a")
	canRead = false
	if got := round85BudgetReviewRequest(t, h, actor, parent.ID, body); got.Code != http.StatusForbidden {
		t.Fatalf("revoked owner allowed writing: %d", got.Code)
	}
	canRead = true
	if got := round85BudgetReviewRequest(t, h, actor, "unknown-parent", body); got.Code != http.StatusNotFound {
		t.Fatalf("wrong parent allowed writing: %d", got.Code)
	}
	if got := round85BudgetReviewRequest(t, h, actor, parent.ID, map[string]any{"kind": "model", "recordId": "record-1", "evidenceRef": "bad"}); got.Code != http.StatusBadRequest {
		t.Fatalf("invalid review evidence accepted: %d", got.Code)
	}
	result := round85BudgetReviewRequest(t, h, actor, parent.ID, body)
	if result.Code != http.StatusOK {
		t.Fatalf("valid review failed: %d %s", result.Code, result.Body.String())
	}
	var payload struct {
		BudgetReleased     bool   `json:"budgetReleased"`
		ExecutionConfirmed bool   `json:"executionConfirmed"`
		Effect             string `json:"effect"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &payload); err != nil || payload.BudgetReleased || payload.ExecutionConfirmed || payload.Effect != "evidence-only" {
		t.Fatalf("review response asserted side effects: %+v %v", payload, err)
	}
	if len(writer.reviews) != 1 || writer.reviews[0].Scope.ParentInteractionID != parent.ID {
		t.Fatalf("wrong review persisted: %+v", writer.reviews)
	}
}

func TestRound85ReviewRouteDisabledWithoutEvidenceWriter(t *testing.T) {
	for _, reader := range []BudgetReconciliationReader{&round83BudgetReader{}, &round85ReviewReader{}} {
		router := gin.New()
		NewHandler(round83ParentReader{}).WithBudgetInspection(reader).RegisterRoutes(router.Group("/api"))
		count := 0
		for _, entry := range router.Routes() {
			if entry.Method == http.MethodPost && entry.Path == "/api/workbench/tasks/budget-reconciliation/review" {
				count++
			}
		}
		if _, ok := reader.(*round85ReviewReader); ok && count != 1 {
			t.Fatalf("review route missing from authorized writer")
		}
		if _, ok := reader.(*round83BudgetReader); ok && count != 0 {
			t.Fatalf("review route allowed read-only ledger")
		}
	}
}
