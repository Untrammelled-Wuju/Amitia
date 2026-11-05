package knowledge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestRecoveryReviewPreservesBindingErrorsQA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ name, body, original string }{
		{"malformed", "{", "unexpected EOF"},
		{"empty", "", "EOF"},
		{"type", `{"recheckId":123}`, "json: cannot unmarshal"},
		{"required", `{}`, "failed on the 'required' tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("actorContext", phase37Actor("qa-device", "qa-session")) })
			router.POST("/review", NewHandler(nil).CreateRecoveryReview)
			request := httptest.NewRequest(http.MethodPost, "/review", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), tc.original) {
				t.Fatalf("missing original binding error %q: status=%d body=%s", tc.original, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRecoveryReviewReadErrorsQA(t *testing.T) {
	for _, tc := range []struct{ value, original string }{
		{"abc", `strconv.Atoi: parsing "abc": invalid syntax`},
		{"0", "limit must be between 1 and 30: 0"},
		{"31", "limit must be between 1 and 30: 31"},
	} {
		t.Run(tc.value, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("actorContext", phase37Actor("qa-device", "qa-session")) })
			router.GET("/reviews", NewHandler(nil).ListRecoveryReviews)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reviews?limit="+tc.value, nil))
			var body struct {
				Message string `json:"msg"`
			}
			parseErr := json.Unmarshal(recorder.Body.Bytes(), &body)
			if recorder.Code != http.StatusBadRequest || parseErr != nil || body.Message != tc.original {
				t.Fatalf("lost limit error %q: status=%d body=%s", tc.original, recorder.Code, recorder.Body.String())
			}
		})
	}
	t.Run("missing-record", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		writeRecoveryReviewError(context, gorm.ErrRecordNotFound)
		if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), gorm.ErrRecordNotFound.Error()) {
			t.Fatalf("lost original not-found error: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})
}
