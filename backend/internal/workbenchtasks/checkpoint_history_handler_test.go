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
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type errorRecordReader struct{ err error }

func (r errorRecordReader) ListByScope(context.Context, interaction.InteractionScope) ([]*interaction.InteractionRecord, error) {
	return nil, r.err
}

func checkpointRequest(handler *Handler, actor *auth.ActorContext, conv, task string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	target := "/api/workbench/tasks/checkpoints?characterId=role&conversationOwnerId=device-a&expectedCoreId=space-a&conversationId=" + url.QueryEscape(conv) + "&coordinationId=" + url.QueryEscape(task)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	if actor != nil {
		request = request.WithContext(auth.WithActor(request.Context(), actor))
	}
	ctx.Request = request
	handler.checkpoints(ctx)
	return recorder
}

func TestCheckpointHandlerAuthorizationAndScope(t *testing.T) {
	record := taskRecord("space-a", "conv-a", "task-a", "running")
	record.UpdatedAt = time.Now()
	record.Scope.CharacterID = "role"
	h := NewHandler(taskRecordReader{records: []*interaction.InteractionRecord{record}}).WithConversationAccess(func(ctx context.Context, actor *auth.ActorContext, query ConversationQuery, read func(context.Context, QueryContext) error) error {
		if actor.DeviceID != "device-a" {
			return coordination.ErrWrongOwner
		}
		if query.ConversationID != "conv-a" {
			return business.ErrConversationUnavailable
		}
		return read(ctx, QueryContext{ExecutionScope: coordination.ExecutionScope{SpaceID: "space-a", RoleID: "role"}, ConversationID: "conv-a", CanReadRecord: func(context.Context, *interaction.InteractionRecord) (bool, error) { return true, nil }})
	})
	validActor := &auth.ActorContext{PrincipalType: auth.PrincipalLocalUI, SpaceID: runtimeidentity.SpaceID("space-a"), DeviceID: "device-a"}
	if got := checkpointRequest(h, nil, "conv-a", "task-a"); got.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated %d", got.Code)
	}
	otherActor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("space-a")}
	if got := checkpointRequest(h, otherActor, "conv-a", "task-a"); got.Code != http.StatusForbidden {
		t.Fatalf("device %d", got.Code)
	}
	if got := checkpointRequest(h, validActor, "meshconv1:bad", "task-a"); got.Code != http.StatusBadRequest {
		t.Fatalf("bad conversation %d", got.Code)
	}
	if got := checkpointRequest(h, validActor, "conv-a", "unknown"); got.Code != http.StatusNotFound {
		t.Fatalf("foreign task leaked %d", got.Code)
	}
	if got := checkpointRequest(h, validActor, "conv-b", "task-a"); got.Code != http.StatusNotFound {
		t.Fatalf("foreign conversation leaked %d", got.Code)
	}
	good := checkpointRequest(h, validActor, "conv-a", "task-a")
	if good.Code != http.StatusOK || good.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("valid response %d %+v", good.Code, good.Header())
	}
	var decoded TaskCheckpointHistory
	if err := json.Unmarshal(good.Body.Bytes(), &decoded); err != nil || len(decoded.Records) != 1 || decoded.CoordinationID != "task-a" {
		t.Fatalf("response invalid: %+v %v", decoded, err)
	}
	validActor.PrincipalType = auth.PrincipalTrustedDevice
	if got := checkpointRequest(h, validActor, "conv-a", "task-a"); got.Code != 200 {
		t.Fatalf("authorized device rejected: %d %s", got.Code, got.Body.String())
	}
}
