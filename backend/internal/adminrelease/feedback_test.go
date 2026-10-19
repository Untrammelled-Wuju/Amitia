// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFeedbackIngestLimiterBlocksBursts(t *testing.T) {
	limiter := newIngestLimiter(2, time.Minute)
	if !limiter.allow("1.2.3.4") || !limiter.allow("1.2.3.4") {
		t.Fatal("first two submissions must be accepted")
	}
	if limiter.allow("1.2.3.4") {
		t.Fatal("third submission in the window must be rejected")
	}
	if !limiter.allow("5.6.7.8") {
		t.Fatal("independent clients must not share the limit")
	}
}

func TestFeedbackPayloadParsesCoreSnapshot(t *testing.T) {
	raw := `{
		"ticketId":"t-1","category":"bug","title":"闪退","description":"启动即退出",
		"platform":"android","appVersion":"26.2.0-beta.3","coreVersion":"26.2.0-beta.3",
		"logLineCount":12,"redactedCount":3,
		"errorLines":[{"file":"app.log","line":"error: boom"}]
	}`
	var payload feedbackPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.TicketID != "t-1" || payload.Title != "闪退" || len(payload.ErrorLines) != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.RedactedCount != 3 || payload.LogLineCount != 12 {
		t.Fatalf("diagnostic counters lost: %+v", payload)
	}
}

func TestFeedbackHelpersNormalizeValues(t *testing.T) {
	if truncateText("  hello  ", 32) != "hello" {
		t.Fatal("truncateText must trim whitespace")
	}
	long := make([]rune, 0, 40)
	for index := 0; index < 40; index++ {
		long = append(long, 'a')
	}
	if len([]rune(truncateText(string(long), 10))) != 10 {
		t.Fatal("truncateText must enforce the byte limit")
	}
	if screenshotContentType(".PNG") != "image/png" || screenshotContentType(".unknown") != "application/octet-stream" {
		t.Fatal("screenshot content type resolution is wrong")
	}
}

func TestFeedbackResolvedToggle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dsn := "file:feedback_resolved_test_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&UserFeedback{}); err != nil {
		t.Fatalf("migrate feedback table: %v", err)
	}
	record := UserFeedback{
		TicketID:   "ticket-resolved",
		Category:   "bug",
		Title:      "闪退",
		Payload:    "{}",
		ReceivedAt: time.Now(),
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create feedback: %v", err)
	}
	server := &Server{db: db}
	resolve := func(resolved bool) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]bool{"resolved": resolved})
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/feedbacks/"+strconv.FormatUint(record.ID, 10), strings.NewReader(string(body)))
		context.Request.Header.Set("Content-Type", "application/json")
		context.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(record.ID, 10)}}
		context.Set(actorContextKey, adminActor{Username: "admin"})
		server.handleSetFeedbackResolved(context)
		return recorder
	}
	if recorder := resolve(true); recorder.Code != http.StatusOK {
		t.Fatalf("resolve feedback status: %d %s", recorder.Code, recorder.Body.String())
	}
	var updated UserFeedback
	if err := db.First(&updated, record.ID).Error; err != nil {
		t.Fatalf("read feedback: %v", err)
	}
	if !updated.Resolved || updated.ResolvedAt == nil || updated.ResolvedBy != "admin" {
		t.Fatalf("feedback not resolved: %+v", updated)
	}
	if recorder := resolve(false); recorder.Code != http.StatusOK {
		t.Fatalf("reopen feedback status: %d %s", recorder.Code, recorder.Body.String())
	}
	updated = UserFeedback{}
	if err := db.First(&updated, record.ID).Error; err != nil {
		t.Fatalf("read feedback: %v", err)
	}
	if updated.Resolved || updated.ResolvedAt != nil || updated.ResolvedBy != "" {
		t.Fatalf("feedback not reopened: %+v", updated)
	}
}
