package system

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWebChatMessageStyleRejectsInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{}
	for name, handle := range map[string]gin.HandlerFunc{"submit": handler.WebChatSubmitMessage, "retry": handler.WebChatRetryTurn} {
		router := gin.New()
		router.POST("/test", handle)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"content":"你好","messageStyle":"unknown"}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if !strings.Contains(recorder.Body.String(), "聊天界面风格无效") {
			t.Fatalf("%s accepted invalid message style: %s", name, recorder.Body.String())
		}
	}
}
