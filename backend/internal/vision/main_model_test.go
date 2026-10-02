package vision

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMainVisionTakeoverAndRestore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "vision.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	for _, query := range []string{
		`CREATE TABLE model_configs (id INTEGER PRIMARY KEY, name TEXT, api_type TEXT, protocol TEXT, base_url TEXT, api_key TEXT, model_name TEXT, capabilities_json TEXT, is_active INTEGER, timeout_seconds INTEGER, max_tokens INTEGER, max_output_tokens INTEGER)`,
		`CREATE TABLE vision_configs (id INTEGER PRIMARY KEY, name TEXT, api_type TEXT, base_url TEXT, api_key TEXT, model_name TEXT, is_active INTEGER, created_at TEXT, updated_at TEXT)`,
		`INSERT INTO model_configs VALUES(1,'main','openai','openai_chat','http://main','test-key','main-model','{"supportsImage":true}',0,20,1000,0)`,
		`INSERT INTO vision_configs VALUES(2,'independent','openai','http://independent','independent-test-key','vision-model',1,'','')`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := NewService(NewRepository(db))
	active, err := svc.GetActive()
	if err != nil || active.ModelName != "vision-model" {
		t.Fatalf("nondefault must not take over: %v %v", active, err)
	}
	db.Exec(`UPDATE model_configs SET is_active = 1`)
	active, err = svc.GetActive()
	if err != nil || !active.FromMainModel || active.Protocol != "openai_chat" || active.ApiKey != "test-key" {
		t.Fatalf("wrong effective model: %v %v", active, err)
	}
	configs, err := svc.List()
	if err != nil || !configs[0].Disabled || configs[0].IsActive != 0 || configs[0].ApiKey != "" || !configs[0].HasApiKey {
		t.Fatalf("suspended list leaked or enabled config: %v %v", configs, err)
	}
	for name, action := range map[string]func() error{
		"create":   func() error { _, err := svc.Create(&CreateVisionConfigRequest{Name: "blocked"}); return err },
		"update":   func() error { _, err := svc.Update(2, map[string]interface{}{"name": "blocked"}); return err },
		"activate": func() error { _, err := svc.Activate(2); return err },
		"test":     func() error { _, err := svc.TestConnection(2); return err },
		"read":     func() error { _, err := svc.GetByID(2); return err },
		"delete":   func() error { return svc.Delete(2) },
	} {
		if err := action(); err == nil || err.Error() != MainModelVisionNotice {
			t.Fatalf("%s should be blocked: %v", name, err)
		}
	}
	db.Exec(`UPDATE model_configs SET capabilities_json = '{"supportsImage":false}'`)
	active, err = svc.GetActive()
	if err != nil || active.ModelName != "vision-model" || active.ApiKey != "independent-test-key" {
		t.Fatalf("original default not restored: %v %v", active, err)
	}
	configs, err = svc.List()
	if err != nil || configs[0].Disabled || configs[0].IsActive != 1 {
		t.Fatalf("list not restored: %v %v", configs, err)
	}
}

func TestMainVisionProtocolImageRequests(t *testing.T) {
	for _, protocol := range []string{"openai_chat", "openai_responses", "anthropic_messages", "gemini_generate_content", "ollama_chat"} {
		t.Run(protocol, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				serialized, _ := json.Marshal(body)
				if !strings.Contains(string(serialized), "cGljdHVyZQ==") || !strings.Contains(string(serialized), "read image") {
					t.Errorf("image or prompt missing: %s", serialized)
				}
				switch protocol {
				case "gemini_generate_content", "anthropic_messages", "ollama_chat":
					if strings.Contains(string(serialized), "data:image/") {
						t.Errorf("expected raw base64 image: %s", serialized)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				switch protocol {
				case "openai_responses":
					fmt.Fprint(w, `{"output":[{"type":"message","content":[{"type":"output_text","text":"recognized"}]}]}`)
				case "anthropic_messages":
					fmt.Fprint(w, `{"content":[{"type":"text","text":"recognized"}]}`)
				case "gemini_generate_content":
					fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"recognized"}]}}]}`)
				case "ollama_chat":
					message := body["messages"].([]interface{})[0].(map[string]interface{})
					if message["images"] == nil || message["content"] != "read image" {
						t.Errorf("invalid Ollama message: %v", message)
					}
					fmt.Fprint(w, `{"message":{"content":"recognized"},"done":true}`)
				default:
					fmt.Fprint(w, `{"choices":[{"message":{"content":"recognized"}}]}`)
				}
			}))
			defer server.Close()
			cfg := &VisionConfig{Protocol: protocol, BaseUrl: server.URL, ModelName: "main", ApiKey: "test-key", FromMainModel: true}
			text, err := GenerateImages(context.Background(), cfg, []string{"data:image/png;base64,cGljdHVyZQ=="}, "read image", 0)
			if err != nil || text != "recognized" {
				t.Fatalf("recognition failed: %q %v", text, err)
			}
		})
	}
}

func TestMainVisionFailureDoesNotFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	_, err := GenerateImages(context.Background(), &VisionConfig{Protocol: "openai_chat", BaseUrl: server.URL, ModelName: "main", FromMainModel: true}, []string{"data:image/png;base64,cGljdHVyZQ=="}, "read", 0)
	if err == nil {
		t.Fatal("main model failure must be returned")
	}
}
