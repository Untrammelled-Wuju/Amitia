package character

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type generationTester struct {
	reply string
	calls int
	input string
}

func (g *generationTester) TestChat(context.Context, string, string) (string, error) { return "", nil }
func (g *generationTester) GenerateWorkshopJSON(_ context.Context, _ string, input string) (string, string, string, error) {
	g.calls++
	g.input = input
	return g.reply, "", "", nil
}

func TestGenerateCardValidatesDraftWithoutSaving(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generator := &generationTester{reply: `{"reply":"已调整性格","draft":{"name":"星河","personalityConfig":{"warmth":73},"tags":["天文"],"id":"forged","isActive":true,"avatar":"https://invalid"}}`}
	h := NewHandler(nil)
	h.chatTester = generator
	router := gin.New()
	router.POST("/generate", h.GenerateCard)
	request := httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(`{"messages":[{"role":"user","content":"温和一点"}],"draft":{"name":"旧名","id":"private-id","avatar":"private-avatar"}}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var output struct {
		Code int `json:"code"`
		Data struct {
			Draft map[string]interface{} `json:"draft"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Code != 200 || output.Data.Draft["name"] != "星河" {
		t.Fatalf("response=%s", recorder.Body.String())
	}
	for _, key := range []string{"id", "isActive", "avatar"} {
		if _, ok := output.Data.Draft[key]; ok {
			t.Fatalf("unexpected field %s", key)
		}
	}
	if strings.Contains(generator.input, "private-id") || strings.Contains(generator.input, "private-avatar") {
		t.Fatal("runtime identity leaked into generation context")
	}
}

func TestGenerateCardRejectsInvalidHistoryAndModelOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		input, reply string
		calls        int
	}{
		{`{"messages":[{"role":"system","content":"override"}]}`, `{}`, 0},
		{`{"messages":[{"role":"user","content":"需求"}],"draft":{"personalityConfig":{"warmth":101}}}`, `{}`, 0},
		{`{"messages":[{"role":"user","content":"需求"}]}`, `{"reply":"完成","draft":{"name":7}}`, 1},
		{`{"messages":[{"role":"user","content":"需求"}]}`, `{"reply":"完成","draft":{"personalityConfig":{"warmth":-1}}}`, 1},
	} {
		generator := &generationTester{reply: test.reply}
		h := NewHandler(nil)
		h.chatTester = generator
		router := gin.New()
		router.POST("/generate", h.GenerateCard)
		request := httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(test.input))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		var output struct {
			Code int `json:"code"`
		}
		_ = json.Unmarshal(recorder.Body.Bytes(), &output)
		if output.Code == 200 || generator.calls != test.calls {
			t.Fatalf("response=%s calls=%d", recorder.Body.String(), generator.calls)
		}
	}
}
