package system

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/mindruntime"
	"gorm.io/gorm"
)

func TestUsageReadsRealCallsAndPreservesMessages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "usage.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store, err := mindruntime.NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE messages(id TEXT,tokens INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO messages VALUES('old',7)")
	service := &service{db: db, usageStore: store}
	id, err := store.Begin(t.Context(), mindruntime.ModelUsageRecord{SpaceID: "core", DeviceID: "phone", Model: "used-model", Provider: "provider", Source: "channel"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(t.Context(), id, mindruntime.ModelUsageRecord{Status: "succeeded", TotalTokens: 120, UsageReported: true}); err != nil {
		t.Fatal(err)
	}
	filter := mindruntime.UsageFilter{SpaceID: "core", DeviceID: "phone"}
	overview, err := service.UsageContext(t.Context(), filter, "overview")
	if err != nil || overview["totalTokens"] != int64(120) || overview["totalRequests"] != int64(1) || overview["totalCost"] != nil {
		t.Fatalf("wrong actual usage: %+v %v", overview, err)
	}
	days, err := service.UsageContext(t.Context(), filter, "daily")
	if err != nil {
		t.Fatal(err)
	}
	daily := days["daily"].([]map[string]any)
	if len(daily) != 30 || daily[0]["modelCalls"] != int64(1) || daily[29]["modelCalls"] != nil || daily[29]["meteringStatus"] != "unmetered" {
		t.Fatalf("calendar days or historical unknown usage wrong: %+v", daily)
	}
	models, err := service.UsageContext(t.Context(), filter, "models")
	if err != nil {
		t.Fatal(err)
	}
	rows := models["models"].([]map[string]any)
	if len(rows) != 1 || rows[0]["name"] != "used-model" || rows[0]["modelCalls"] != int64(1) {
		t.Fatalf("models not aggregated: %+v", rows)
	}
	if _, err := service.ClearUsageContext(t.Context(), filter); err != nil {
		t.Fatal(err)
	}
	var tokens int64
	db.Raw("SELECT tokens FROM messages WHERE id='old'").Scan(&tokens)
	if tokens != 7 {
		t.Fatal("reset changed chat tokens")
	}
	unknown, _ := store.Begin(t.Context(), mindruntime.ModelUsageRecord{DeviceID: "phone", Model: "unknown"})
	store.Finish(t.Context(), unknown, mindruntime.ModelUsageRecord{Status: "succeeded"})
	overview, err = service.UsageContext(t.Context(), filter, "overview")
	if err != nil || overview["totalTokens"] != nil || overview["unknownUsageCalls"] != int64(1) {
		t.Fatalf("unknown tokens replaced with zero: %+v %v", overview, err)
	}
}

func TestUsageHTTPDeviceIsolationAndStaleReset(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "usage.db")), &gorm.Config{})
	store, err := mindruntime.NewModelUsageStore(db, "core")
	if err != nil {
		t.Fatal(err)
	}
	for _, device := range []string{"phone", "desktop"} {
		id, _ := store.Begin(t.Context(), mindruntime.ModelUsageRecord{DeviceID: device, Model: "model"})
		store.Finish(t.Context(), id, mindruntime.ModelUsageRecord{Status: "succeeded", TotalTokens: 10, UsageReported: true})
	}
	handler := &Handler{service: &service{usageStore: store}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		securityActor := &auth.ActorContext{SpaceID: "core", DeviceID: "phone", Permissions: auth.StandardPermissions()}
		c.Set("actorContext", securityActor)
		c.Request = c.Request.WithContext(auth.WithActor(c.Request.Context(), securityActor))
		c.Next()
	})
	router.GET("/overview", usageAccess, handler.UsageOverview)
	router.DELETE("/clear", usageAccess, handler.UsageClear)
	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequest("GET", "/overview", nil))
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &body); err != nil || body.Data["totalRequests"] != float64(1) {
		t.Fatalf("other device exposed: %s %v", read.Body.String(), err)
	}
	stale := httptest.NewRecorder()
	router.ServeHTTP(stale, httptest.NewRequest("DELETE", "/clear?expectedCoreId=old&expectedDeviceId=phone", nil))
	if !strings.Contains(stale.Body.String(), "409") {
		t.Fatal(stale.Body.String())
	}
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, httptest.NewRequest("GET", "/overview?scope=core", nil))
	if denied.Code != 403 {
		t.Fatal("ordinary device read core-wide usage")
	}
}
