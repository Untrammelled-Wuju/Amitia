package emote

import (
	"bytes"
	"encoding/json"
	"image/color"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/chat"
)

func emoteTestRouter(service *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRouter(router.Group("/api"), service)
	return router
}

func performJSON(t *testing.T, router http.Handler, method, path string, body interface{}) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s %s 返回 %d", method, path, recorder.Code)
	}
	var response map[string]interface{}
	if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func performUpload(t *testing.T, router http.Handler, path string, files map[string][]byte, configs interface{}) map[string]interface{} {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	field := "file"
	if path == "/api/emotes/batch-upload" {
		field = "files"
	}
	for name, data := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+name+`"`)
		header.Set("Content-Type", "image/png")
		part, err := w.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if configs != nil {
		encoded, err := json.Marshal(configs)
		if err != nil {
			t.Fatal(err)
		}
		name := "config"
		if field == "files" {
			name = "configs"
		}
		if err = w.WriteField(name, string(encoded)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func responseData(t *testing.T, response map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("响应缺少 data: %#v", response)
	}
	return data
}

func TestEmoteAPIWorkflow(t *testing.T) {
	service, db := setupEmoteTest(t)
	router := emoteTestRouter(service)
	groupResponse := performJSON(t, router, http.MethodPost, "/api/emote-groups", map[string]interface{}{"name": "常用"})
	groupID, _ := responseData(t, groupResponse)["id"].(string)
	if groupID == "" {
		t.Fatal("创建分组失败")
	}
	imageData := pngBytes(t, color.RGBA{R: 20, G: 140, B: 220, A: 255})
	upload := performUpload(t, router, "/api/emotes/upload", map[string][]byte{"single.png": imageData}, ImportConfig{Name: "单图", GroupIDs: []string{groupID}})
	emoteID, _ := responseData(t, upload)["emoteId"].(string)
	if emoteID == "" {
		t.Fatalf("单文件上传失败: %#v", upload)
	}
	duplicate := performUpload(t, router, "/api/emotes/upload", map[string][]byte{"copy.png": imageData}, ImportConfig{})
	if responseData(t, duplicate)["status"] != "duplicate" {
		t.Fatalf("重复文件接口结果错误: %#v", duplicate)
	}
	batch := performUpload(t, router, "/api/emotes/batch-upload", map[string][]byte{"valid.png": pngBytes(t, color.RGBA{G: 220, A: 255}), "invalid.png": []byte("bad")}, []ImportConfig{{SourceName: "valid.png"}, {SourceName: "invalid.png"}})
	summary := responseData(t, batch)["summary"].(map[string]interface{})
	if summary["success"].(float64) != 1 || summary["failed"].(float64) != 1 {
		t.Fatalf("多文件部分成功统计错误: %#v", batch)
	}
	update := performJSON(t, router, http.MethodPut, "/api/emotes/"+emoteID, map[string]interface{}{"meaning": "表达开心", "keywords": []string{"开心", "问候"}, "roleScope": RoleScopeSelected, "characterIds": []string{"c1"}, "groupIds": []string{groupID}})
	updated := responseData(t, update)
	if updated["meaning"] != "表达开心" || updated["roleScope"] != RoleScopeSelected || len(updated["groupIds"].([]interface{})) != 1 {
		t.Fatalf("含义、角色范围或多分组修改失败: %#v", update)
	}
	if err := db.Create(&chat.Conversation{ID: "conv", CharacterID: "c1", Channel: "web"}).Error; err != nil {
		t.Fatal(err)
	}
	manual := performJSON(t, router, http.MethodPost, "/api/chat/send-emote", map[string]interface{}{"conversationId": "conv", "characterId": "c1", "emoteId": emoteID})
	if responseData(t, manual)["msgType"] != "emote" {
		t.Fatalf("手动发送失败: %#v", manual)
	}
	settings := map[string]interface{}{"enabled": true, "baseProbability": 0.12, "maxProbability": 0.3, "maxPerHour": 5, "minReplyGap": 3, "sameEmoteCooldownMinutes": 30, "allowEmoteOnly": false}
	performJSON(t, router, http.MethodPut, "/api/characters/c1/emote-settings", settings)
	readSettings := performJSON(t, router, http.MethodGet, "/api/characters/c1/emote-settings", nil)
	if responseData(t, readSettings)["baseProbability"].(float64) != 0.12 {
		t.Fatalf("角色配置读取保存失败: %#v", readSettings)
	}
	performJSON(t, router, http.MethodDelete, "/api/emote-groups/"+groupID, nil)
	if _, err := service.Get(emoteID); err != nil {
		t.Fatal("删除分组后表情应保留")
	}
	performJSON(t, router, http.MethodDelete, "/api/emotes/"+emoteID, nil)
	if _, err := service.Get(emoteID); err == nil {
		t.Fatal("删除表情接口未生效")
	}
}
