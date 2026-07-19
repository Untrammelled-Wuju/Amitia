package requestidentity

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveGinSingleUserCanonicalIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("body userId ignored, returns default", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/", nil)
		if got := ResolveGin(ctx, "user-abc"); got != DefaultUserID {
			t.Fatalf("expected %q, got %q", DefaultUserID, got)
		}
	})

	t.Run("X-User-ID header ignored, returns default", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/", nil)
		request.Header.Set("X-User-ID", "header-user")
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = request
		if got := ResolveGin(ctx, ""); got != DefaultUserID {
			t.Fatalf("expected %q, got %q", DefaultUserID, got)
		}
	})

	t.Run("Gin context userID ignored, returns default", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/", nil)
		ctx.Set("userID", "auth-user")
		if got := ResolveGin(ctx, ""); got != DefaultUserID {
			t.Fatalf("expected %q, got %q", DefaultUserID, got)
		}
	})

	t.Run("empty request returns default", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/", nil)
		if got := ResolveGin(ctx, ""); got != DefaultUserID {
			t.Fatalf("expected %q, got %q", DefaultUserID, got)
		}
	})
}
