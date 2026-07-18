package requestidentity

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveGinPriority(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("X-User-ID", "header-user")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	if got := ResolveGin(ctx, "envelope-user"); got != "envelope-user" {
		t.Fatalf("unexpected envelope result %s", got)
	}
	ctx.Set("userID", "auth-user")
	if got := ResolveGin(ctx, "envelope-user"); got != "auth-user" {
		t.Fatalf("unexpected auth result %s", got)
	}
}

func TestResolveGinFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	if got := ResolveGin(ctx, ""); got != DefaultUserID {
		t.Fatalf("unexpected fallback %s", got)
	}
}
