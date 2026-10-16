// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "user-feedback.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&Ticket{}); err != nil {
		t.Fatalf("migrate tickets: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS app_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		value TEXT DEFAULT '',
		revision INTEGER NOT NULL DEFAULT 1,
		deleted_at DATETIME,
		updated_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatalf("create app_settings: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

func TestRedactionMasksSensitiveValues(t *testing.T) {
	line := `post https://api.example.com auth=Bearer abcdef1234567890 email=user@example.com phone=13800001111 dir=C:\Users\pengxu\data`
	cleaned, masked := redactLine(line)
	for _, leaked := range []string{"abcdef1234567890", "user@example.com", "13800001111", `C:\Users\pengxu`} {
		if strings.Contains(cleaned, leaked) {
			t.Fatalf("sensitive value %q was not masked: %s", leaked, cleaned)
		}
	}
	if masked == 0 {
		t.Fatal("expected redaction count to be reported")
	}
}

func TestDeviceIdentifierIsHashed(t *testing.T) {
	hashed := hashDeviceID("device-abc")
	if !strings.HasPrefix(hashed, "sha256:") || len(hashed) != len("sha256:")+16 {
		t.Fatalf("unexpected device hash: %q", hashed)
	}
	if hashed != hashDeviceID("device-abc") {
		t.Fatal("device hash must be stable")
	}
	if hashDeviceID("device-abd") == hashed {
		t.Fatal("different device identifiers must not collide")
	}
	if hashDeviceID("") != "" {
		t.Fatal("empty device identifier must stay empty")
	}
}

func TestUploadClientsAlwaysTryDirectConnectionFirst(t *testing.T) {
	clients := uploadClients("https://amitia.untrammelled.top/api/v1/feedbacks")
	if len(clients) == 0 || len(clients) > 2 {
		t.Fatalf("unexpected upload client count: %d", len(clients))
	}
	transport, ok := clients[0].Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatal("the first upload attempt must bypass any environment proxy")
	}
}

func TestPreviewDescribesUploadedPayload(t *testing.T) {
	service := NewService(openTestDB(t))
	includeLogs := false
	result, err := service.Preview(&TicketInput{
		Category:    CategoryBug,
		Title:       "聊天窗口空白",
		Description: "打开会话后内容不显示",
		Contact:     "user@example.com",
		IncludeLogs: &includeLogs,
		DeviceID:    "device-abc",
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if !strings.Contains(result.Text, "聊天窗口空白") ||
		!strings.Contains(result.Text, "是否上传报错日志：否") ||
		!strings.Contains(result.Text, "设备标识：sha256:") {
		t.Fatalf("preview text missing expected content: %s", result.Text)
	}
	if result.Payload["deviceHash"] != hashDeviceID("device-abc") {
		t.Fatalf("preview payload must expose only the hashed device identifier: %v", result.Payload["deviceHash"])
	}
	if result.Payload["uploadUrl"] == "" {
		t.Fatal("preview must report the effective upload url")
	}
}

func TestUserFeedbackTicketUploadsToConfiguredEndpoint(t *testing.T) {
	t.Setenv("AMITIA_DATA_DIR", t.TempDir())
	var (
		mu       sync.Mutex
		received map[string]any
		fileName string
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(8 << 20); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload map[string]any
		_ = json.Unmarshal([]byte(request.FormValue("payload")), &payload)
		mu.Lock()
		received = payload
		if header := request.MultipartForm.File["screenshot"]; len(header) > 0 {
			fileName = header[0].Filename
		}
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"code":200,"msg":"ok","data":{"id":"remote-42"}}`)
	}))
	defer server.Close()

	service := NewService(openTestDB(t))
	if _, err := service.SaveUploadURL(server.URL); err != nil {
		t.Fatalf("save upload url: %v", err)
	}
	includeLogs := false
	ticket, err := service.Create("space_test", &TicketInput{
		Category:    CategoryBug,
		Title:       "启动后闪退",
		Description: "打开应用后立即退出",
		IncludeLogs: &includeLogs,
		Screenshot: &ScreenshotInput{
			Name:       "shot.png",
			MediaType:  "image/png",
			DataBase64: "aGVsbG8=",
		},
	})
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if !ticket.HasScreenshot || ticket.ScreenshotSize != 5 {
		t.Fatalf("screenshot was not stored: %+v", ticket)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		stored, loadErr := service.List("space_test", 5)
		if loadErr == nil && len(stored) == 1 && stored[0].Status == StatusUploaded {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	stored, err := service.List("space_test", 5)
	if err != nil || len(stored) != 1 {
		t.Fatalf("ticket not persisted: %v %v", stored, err)
	}
	if stored[0].Status != StatusUploaded || stored[0].RemoteID != "remote-42" {
		t.Fatalf("ticket was not uploaded: %+v", stored[0])
	}
	if stored[0].ScreenshotPath == "" {
		t.Fatal("screenshot path must be recorded locally")
	}
	mu.Lock()
	defer mu.Unlock()
	if received == nil || received["ticketId"] != stored[0].ID || received["title"] != "启动后闪退" {
		t.Fatalf("endpoint did not receive the ticket payload: %v", received)
	}
	if fileName != "shot.png" {
		t.Fatalf("screenshot file name not forwarded: %q", fileName)
	}
}

func TestUserFeedbackRejectsInvalidInput(t *testing.T) {
	service := NewService(openTestDB(t))
	if _, err := service.Create("space_test", &TicketInput{Category: CategoryBug, Title: "", Description: "d"}); err == nil {
		t.Fatal("empty title must be rejected")
	}
	if _, err := service.Create("space_test", &TicketInput{Category: "unknown", Title: "t", Description: "d"}); err == nil {
		t.Fatal("unknown category must be rejected")
	}
	if _, err := service.SaveUploadURL("ftp://example.com"); err == nil {
		t.Fatal("non-http upload url must be rejected")
	}
}
