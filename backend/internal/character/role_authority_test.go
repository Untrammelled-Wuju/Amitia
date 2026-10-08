package character

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/app"
)

func TestRoleAuthorityRejectsRetargetedCharacterMutation(t *testing.T) {
	var routers [2]*gin.Engine
	var services [2]*service
	for i := range routers {
		db := newTestDB(t)
		ctx := app.NewAppContext(db, nil)
		services[i] = NewService(NewRepository(ctx), ctx).(*service)
		if err := db.Create(&Character{ID: "same-role", SpaceID: "device-space", Name: "unchanged", Status: "enabled"}).Error; err != nil {
			t.Fatal(err)
		}
		routers[i] = gin.New()
		routers[i].Use(func(c *gin.Context) {
			c.Set("actorContext", &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("device-space"), Permissions: []string{auth.PermSystemAdmin}})
			c.Next()
		})
		RegisterCharacterRouter(routers[i].Group("/api"), ctx, nil)
	}
	read := httptest.NewRecorder()
	routers[0].ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/characters/same-role", nil))
	var body struct {
		Data Character `json:"data"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &body); err != nil || len(body.Data.RoleAuthority) != 64 {
		t.Fatalf("missing read authority: status=%d err=%v", read.Code, err)
	}
	mutate := func(router *gin.Engine, token string) int {
		req := httptest.NewRequest(http.MethodPut, "/api/characters/same-role", bytes.NewBufferString(`{"name":"changed"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(RoleAuthorityHeader, token)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		return out.Code
	}
	if status := mutate(routers[1], body.Data.RoleAuthority); status != http.StatusConflict {
		t.Fatalf("retarget mutation status=%d", status)
	}
	if status := mutate(routers[0], ""); status != http.StatusConflict {
		t.Fatalf("trusted mutation without owner intent accepted: %d", status)
	}
	other, err := services[1].GetByIDForSpace("same-role", "device-space")
	if err != nil || other.Name != "unchanged" {
		t.Fatalf("new owner modified: err=%v", err)
	}
	if status := mutate(routers[0], body.Data.RoleAuthority); status != http.StatusOK {
		t.Fatalf("original owner mutation status=%d", status)
	}
	wrongSpace, err := services[0].RoleAuthority("other-space")
	if err != nil {
		t.Fatal(err)
	}
	if status := mutate(routers[0], wrongSpace); status != http.StatusConflict {
		t.Fatalf("foreign space accepted: %d", status)
	}
	if status := mutate(routers[0], body.Data.RoleAuthority+" "); status != http.StatusConflict {
		t.Fatalf("malformed token accepted: %d", status)
	}
}

func TestRoleAuthorityConcurrentServicesShareConnection(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	var group sync.WaitGroup
	tokens := make(chan string, 32)
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			svc := NewService(NewRepository(ctx), ctx).(*service)
			token, err := svc.RoleAuthority("device-space")
			if err != nil {
				t.Error(err)
				return
			}
			tokens <- token
		}()
	}
	group.Wait()
	close(tokens)
	first := ""
	for token := range tokens {
		if first == "" {
			first = token
		}
		if token != first {
			t.Fatal("same connection has competing owner tokens")
		}
	}
}

func TestRoleAuthorityMultipartRejectsOwnerChangeBeforeHandler(t *testing.T) {
	db := newTestDB(t)
	ctx := app.NewAppContext(db, nil)
	svc := NewService(NewRepository(ctx), ctx).(*service)
	handler := NewHandler(svc)
	token, err := svc.RoleAuthority("")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	foreign := "0" + token[1:]
	if token[0] == '0' {
		foreign = "1" + token[1:]
	}
	calls := 0
	router.POST("/characters/:id/avatar", handler.guardRoleAuthority(), func(c *gin.Context) { calls++; c.Status(200) })
	for _, scenario := range []struct {
		field, header string
		size, status  int
	}{
		{field: token, size: 16, status: 200},
		{field: foreign, size: 16, status: 409},
		{field: token, header: "different", size: 16, status: 409},
		{field: token, size: 9 * 1024 * 1024, status: 413},
	} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		if err := form.WriteField("roleAuthority", scenario.field); err != nil {
			t.Fatal(err)
		}
		file, err := form.CreateFormFile("avatar", "avatar.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(bytes.Repeat([]byte{'a'}, scenario.size)); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/characters/same-role/avatar", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		if scenario.header != "" {
			req.Header.Set(RoleAuthorityHeader, scenario.header)
		}
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != scenario.status {
			t.Fatalf("multipart status=%d expected=%d", out.Code, scenario.status)
		}
	}
	if calls != 1 {
		t.Fatalf("rejected upload reached mutation handler: calls=%d", calls)
	}
}
