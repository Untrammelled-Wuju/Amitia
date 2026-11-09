package agentplatform

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestRespondErrorClassifiesWrappedRecordNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"wrapped record not found", fmt.Errorf("读取团队: %w", gorm.ErrRecordNotFound), http.StatusNotFound},
		{"wrapped template not found", fmt.Errorf("读取团队模板: %w", gorm.ErrRecordNotFound), http.StatusNotFound},
		{"chinese not found", errors.New("团队 不存在"), http.StatusNotFound},
		{"validation failure", errors.New("团队名称不能为空"), http.StatusBadRequest},
		{"storage failure", fmt.Errorf("读取团队列表: %w", errors.New("database is closed")), http.StatusInternalServerError},
	}

	handler := &HTTPHandler{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			engine.GET("/boom", func(c *gin.Context) { handler.respondError(c, tc.err) })

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/boom", nil))

			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d, body = %s", recorder.Code, tc.want, recorder.Body.String())
			}
		})
	}
}
