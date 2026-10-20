package system

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestCitationSnapshotAndEventReplayRejectForeignSpace(t *testing.T) {
	h, db := newWebChatScopeTestHandler(t)
	if err := db.Exec("INSERT INTO conversations (id, space_id, title, channel) VALUES (?, ?, ?, ?)", "private-conv", "space-owner", "private", "web").Error; err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct {
		name string
		run  func(*gin.Context)
	}{
		{name: "snapshot", run: h.WebChatConversationSnapshot},
		{name: "events_replay", run: h.WebChatConversationEvents},
	} {
		t.Run(route.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/web-chat/conversations/private-conv/"+route.name, nil)
			ctx.Params = gin.Params{{Key: "id", Value: "private-conv"}}
			ctx.Set("actorContext", &auth.ActorContext{SpaceID: runtimeidentity.SpaceID("space-foreign")})
			route.run(ctx)
			if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "private") {
				t.Fatalf("foreign space received conversation or citation data: status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(recorder.Header().Get("Content-Type"), "text/event-stream") {
				t.Fatal("unauthorized replay subscribed to event stream")
			}
		})
	}
}
