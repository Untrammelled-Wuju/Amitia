package requestidentity

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	authctx "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/spaceidentity"
)

func TestResolveGinUsesAuthenticatedSpaceAndIgnoresClientIdentityHints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := spaceidentity.InitializeDefault(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	canonical := store.SpaceID()

	t.Run("unauthenticated context resolves process canonical space", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/?spaceId=attacker", nil)
		ctx.Request.Header.Set("X-User-ID", "attacker")
		ctx.Set("spaceId", "attacker")
		if got := ResolveGin(ctx); got != canonical {
			t.Fatalf("expected canonical %q, got %q", canonical, got)
		}
	})

	t.Run("authenticated actor space wins", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/?spaceId=attacker", nil)
		ctx.Set("actorContext", &authctx.ActorContext{SpaceID: runtimeidentity.SpaceID("space_authenticated")})
		if got := ResolveGin(ctx); got != "space_authenticated" {
			t.Fatalf("expected authenticated space, got %q", got)
		}
	})
}

func TestNormalizeSpaceIDNeverPersistsLegacyDefault(t *testing.T) {
	store, err := spaceidentity.InitializeDefault(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	canonical := store.SpaceID()
	for _, input := range []string{"", LegacySpaceID, "  default  "} {
		if got := NormalizeSpaceID(input); got != canonical {
			t.Fatalf("NormalizeSpaceID(%q) = %q, want %q", input, got, canonical)
		}
	}
	if got := NormalizeSpaceID("space_external"); got != "space_external" {
		t.Fatalf("explicit valid space changed: %q", got)
	}
}
