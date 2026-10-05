package embedding

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEmbeddingAuthorityAppliesToFreshServicesAndDatabaseSessions(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"data":[{"embedding":[1,0]}]}`))
	}))
	defer server.Close()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE embedding_configs (base_url TEXT, api_key TEXT, model_name TEXT, api_type TEXT, provider_config_json TEXT, is_active INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO embedding_configs(base_url,api_key,model_name,api_type,is_active) VALUES(?,?,?,'volcengine',1)", server.URL, "test", "model").Error; err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("bound device requires Core embedding")
	if err := SetInferenceAuthority(db, func(context.Context) (context.Context, func(), error) { return nil, nil, rejected }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetInferenceAuthority(db, nil) })
	for _, session := range []*gorm.DB{db, db.WithContext(t.Context())} {
		_, err := NewService(session).EmbedContext(t.Context(), "query")
		if !errors.Is(err, rejected) {
			t.Fatalf("authority bypassed: %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("device invoked embedding provider while bound")
	}
}

func TestEmbeddingFingerprintUsesConfigurationThatProducedVector(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE embedding_configs (base_url TEXT, api_key TEXT, model_name TEXT, api_type TEXT, provider_config_json TEXT, is_active INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := db.Exec("UPDATE embedding_configs SET model_name='new-model'").Error; err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"data":[{"embedding":[1,0]}]}`))
	}))
	defer server.Close()
	if err := db.Exec("INSERT INTO embedding_configs(base_url,api_key,model_name,api_type,is_active) VALUES(?,?,?,'volcengine',1)", server.URL, "test", "old-model").Error; err != nil {
		t.Fatal(err)
	}
	vector, fingerprint, err := NewService(db).EmbedContextWithFingerprint(t.Context(), "query")
	expected := embeddingConfigFingerprint("old-model\x00volcengine\x00"+server.URL, "")
	if err != nil || len(vector) != 2 || fingerprint != expected {
		t.Fatalf("vector=%v fingerprint=%s expected=%s err=%v", vector, fingerprint, expected, err)
	}
}
