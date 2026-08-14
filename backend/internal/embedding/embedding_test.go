package embedding

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"gorm.io/gorm"
)

func TestEmbedWithRawErrorPreservesProviderResponse(t *testing.T) {
	rawBody := `{"error":"` + strings.Repeat("x", 350) + `tail"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(rawBody))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE embedding_configs (base_url TEXT, api_key TEXT, model_name TEXT, api_type TEXT, provider_config_json TEXT, is_active INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO embedding_configs(base_url, api_key, model_name, api_type, is_active) VALUES (?, ?, ?, 'volcengine', 1)", server.URL, "test-key", "test-model").Error; err != nil {
		t.Fatal(err)
	}

	vector, rawError, err := NewService(db).EmbedWithRawError("测试文本")
	if err == nil {
		t.Fatal("expected error on provider failure, got nil")
	}
	if vector != nil {
		t.Fatalf("expected nil vector on failure, got %#v", vector)
	}
	if !strings.Contains(rawError, rawBody) || !strings.Contains(rawError, "tail") {
		t.Fatalf("raw provider response was truncated: %s", rawError)
	}
}

func TestEmbedAcceptsProviderDataShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "object", body: `{"data":{"embedding":[0.25,-0.5]},"object":"list"}`},
		{name: "array", body: `{"data":[{"embedding":[0.25,-0.5]}],"object":"list"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("CREATE TABLE embedding_configs (base_url TEXT, api_key TEXT, model_name TEXT, api_type TEXT, provider_config_json TEXT, is_active INTEGER)").Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("INSERT INTO embedding_configs(base_url, api_key, model_name, api_type, is_active) VALUES (?, ?, ?, 'volcengine', 1)", server.URL, "test-key", "test-model").Error; err != nil {
				t.Fatal(err)
			}

			vector, rawError, err := NewService(db).EmbedWithRawError("测试文本")
			if err != nil {
				t.Fatal(err)
			}
			if rawError != "" {
				t.Fatalf("unexpected raw error: %s", rawError)
			}
			if len(vector) != 2 || vector[0] != 0.25 || vector[1] != -0.5 {
				t.Fatalf("unexpected vector: %#v", vector)
			}
		})
	}
}

func TestFitEmbeddingDimension(t *testing.T) {
	previous := config.AppCfg
	config.AppCfg = &config.Config{}
	config.AppCfg.Providers.VectorStore.Qdrant.VectorDim = 3
	t.Cleanup(func() {
		config.AppCfg = previous
	})

	vector, err := fitEmbeddingDimension([]float32{1, 2, 3, 4, 5})
	if err == nil {
		t.Fatal("expected error on dimension mismatch, got nil")
	}
	if vector != nil {
		t.Fatalf("expected nil vector on mismatch, got %#v", vector)
	}

	vector, err = fitEmbeddingDimension([]float32{1, 2, 3})
	if err != nil {
		t.Fatalf("unexpected error on matching dimension: %v", err)
	}
	if len(vector) != 3 {
		t.Fatalf("unexpected dimension: %d", len(vector))
	}
}
