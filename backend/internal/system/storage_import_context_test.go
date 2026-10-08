package system

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type scopedStorageImportFixture struct {
	Service
	ctx      context.Context
	dataDir  string
	fileName string
}

func (s *scopedStorageImportFixture) GetStorageInfo() map[string]interface{} {
	return map[string]interface{}{"path": s.dataDir}
}

func (s *scopedStorageImportFixture) StorageImportUserDataContext(ctx context.Context, body map[string]interface{}) map[string]interface{} {
	s.ctx = ctx
	s.fileName, _ = body["fileName"].(string)
	return map[string]interface{}{"imported": false, "error": "fixture revoked administrator"}
}

func TestStorageImportHTTPPreservesScopeAndRejectsFalseSuccess(t *testing.T) {
	for _, upload := range []bool{false, true} {
		t.Run(map[bool]string{false: "JSON", true: "multipart"}[upload], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			svc := &scopedStorageImportFixture{dataDir: t.TempDir()}
			handler := &Handler{service: svc}
			router := gin.New()
			var request *http.Request
			if !upload {
				router.POST("/import", handler.StorageImportUserData)
				request = httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(`{"fileName":"fixture.amitia"}`)).WithContext(ctx)
				request.Header.Set("Content-Type", "application/json")
			} else {
				router.POST("/import", handler.StorageImportAmitia)
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				part, err := writer.CreateFormFile("file", "fixture.amitia")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := part.Write([]byte("fixture")); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				request = httptest.NewRequest(http.MethodPost, "/import", &body).WithContext(ctx)
				request.Header.Set("Content-Type", writer.FormDataContentType())
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if svc.ctx != ctx || svc.fileName != "fixture.amitia" {
				t.Fatal("HTTP import lost original authorization context")
			}
			var result struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Code == 200 || result.Code == 0 {
				t.Fatalf("failed import reported success: %s %v", response.Body.String(), err)
			}
		})
	}
}

func TestStorageImportContextRejectsCanceledScopeBeforeArchiveAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := (&service{}).StorageImportUserDataContext(ctx, map[string]interface{}{"fileName": "fixture.amitia"})
	if imported, _ := result["imported"].(bool); imported {
		t.Fatal("canceled import succeeded")
	}
	if message, _ := result["error"].(string); !strings.Contains(message, "授权已变化") {
		t.Fatalf("scope check was bypassed: %+v", result)
	}
}

func TestStorageImportHTTPRefusesUnscopedLegacyFallback(t *testing.T) {
	handler := &Handler{service: (Service)(nil)}
	result := handler.storageImport(t.Context(), map[string]interface{}{"fileName": "fixture.amitia"})
	if imported, _ := result["imported"].(bool); imported {
		t.Fatal("HTTP used unscoped legacy import")
	}
}
