package character

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGenerateCardPreservesParseErrorsQA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, body, original string }{
		{"malformed", "{", "unexpected EOF"},
		{"empty", "", "EOF"},
		{"type", `{"messages":123}`, "json: cannot unmarshal"},
		{"oversized", `{"padding":"` + strings.Repeat("x", 131072) + `"}`, "http: request body too large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(nil)
			router := gin.New()
			router.POST("/generate", h.GenerateCard)
			request := httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if !strings.Contains(recorder.Body.String(), tc.original) {
				t.Fatalf("missing original error %q: %s", tc.original, recorder.Body.String())
			}
		})
	}
}
