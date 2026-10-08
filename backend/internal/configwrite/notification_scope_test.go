package configwrite_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func configurationNotificationRouter(db *gorm.DB, svc *coordination.Service, policy coordination.Policy, administrator bool) *gin.Engine {
	router := gin.New()
	permissions := auth.StandardPermissions()
	if administrator {
		permissions = append(permissions, auth.PermSystemAdmin)
	}
	router.Use(func(c *gin.Context) {
		actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice, SpaceID: runtimeidentity.SpaceID("space"), DeviceID: runtimeidentity.DeviceID("admin"), Permissions: permissions, RequestID: "notification-request"}
		c.Set("actorContext", actor)
		c.Set("authenticatedConfigurationPolicy", policy)
		c.Set("authenticatedConfigurationPolicyService", svc)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), actor))
		c.Next()
	})
	handler := system.NewHandler(system.NewService(&app.AppContext{DB: db}, ""), db, nil, nil, nil, nil, nil)
	router.GET("/settings", handler.NotificationsSettings)
	router.GET("/status", handler.NotificationsStatus)
	router.PUT("/settings", handler.UpdateNotificationsSettings)
	router.POST("/subscribe", handler.NotificationsSubscribe)
	router.POST("/unsubscribe", handler.NotificationsUnsubscribe)
	router.POST("/test", handler.NotificationsTest)
	return router
}

func TestConfigurationNotificationsRejectCrossDeviceOrdinaryRequests(t *testing.T) {
	db, svc := configurationDB(t)
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	router := configurationNotificationRouter(db, svc, policy, false)
	for _, request := range []struct{ method, path, body string }{
		{http.MethodGet, "/settings?deviceId=other", `{}`},
		{http.MethodGet, "/status?deviceId=other", `{}`},
		{http.MethodPut, "/settings", `{"deviceId":"other","enabled":false}`},
		{http.MethodPost, "/subscribe?deviceId=other", `{}`},
		{http.MethodPost, "/unsubscribe", `{"deviceId":"other"}`},
		{http.MethodPost, "/test", `{"deviceId":"other"}`},
	} {
		if code := configurationHTTPResult(t, router, t.Context(), request.method, request.path, request.body); code != 403 {
			t.Fatalf("ordinary cross-device notification %s %s returned %d", request.method, request.path, code)
		}
	}
	var count int64
	if err := db.Table("app_settings").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("ordinary actor modified another device preferences")
	}
	if code := configurationHTTPResult(t, router, t.Context(), http.MethodPut, "/settings", `{"enabled":false}`); code != 200 {
		t.Fatalf("own preferences rejected: %d", code)
	}
}

func TestConfigurationNotificationsCrossDeviceAdministratorMustRemainAuthorized(t *testing.T) {
	db, svc := configurationDB(t)
	policy, err := svc.Get(t.Context(), "space", "admin")
	if err != nil {
		t.Fatal(err)
	}
	router := configurationNotificationRouter(db, svc, policy, true)
	if code := configurationHTTPResult(t, router, t.Context(), http.MethodPut, "/settings", `{"deviceId":"other","enabled":false}`); code != 200 {
		t.Fatalf("current administrator rejected: %d", code)
	}
	var before int64
	if err := db.Table("app_settings").Count(&before).Error; err != nil || before != 2 {
		t.Fatal("administrator did not use Core preference storage")
	}
	policy, err = svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GrantAdministrator(t.Context(), "space", "admin", policy.PermissionRevision, true); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path, body string }{
		{http.MethodGet, "/settings?deviceId=other", `{}`},
		{http.MethodPut, "/settings", `{"deviceId":"other","enabled":true}`},
		{http.MethodPost, "/subscribe", `{"deviceId":"other"}`},
	} {
		if code := configurationHTTPResult(t, router, t.Context(), request.method, request.path, request.body); code == 200 || code == 0 {
			t.Fatalf("old administrator notification request survived ABA: %s", request.path)
		}
	}
}
