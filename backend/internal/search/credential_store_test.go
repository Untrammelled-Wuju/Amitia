package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
)

func newSearchCredentialTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "search.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	_, err = db.Exec(`CREATE TABLE search_api_keys (
		engine_id TEXT PRIMARY KEY,
		api_key TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	)`)
	require.NoError(t, err)
	return db
}

func TestCredentialStoreStoresAndResolvesGoogleCredentials(t *testing.T) {
	db := newSearchCredentialTestDB(t)
	store := NewCredentialStore(db)
	ctx := context.Background()

	_, err := store.Set(ctx, "google_cse", "google-api-key")
	require.NoError(t, err)
	_, err = store.Set(ctx, "google_cse_cx", "google-cx")
	require.NoError(t, err)
	_, err = store.Set(ctx, "brave_api", "brave-key")
	require.NoError(t, err)

	rows, err := db.Query("SELECT api_key FROM search_api_keys ORDER BY engine_id")
	require.NoError(t, err)
	defer rows.Close()
	var stored []string
	for rows.Next() {
		var value string
		require.NoError(t, rows.Scan(&value))
		stored = append(stored, value)
	}
	require.ElementsMatch(t, []string{"brave-key", "google-api-key", "google-cx"}, stored)

	credentials, release, err := store.ResolveEngineCredentials(ctx, ProviderNative, "test")
	require.NoError(t, err)
	require.NotNil(t, release)
	require.Equal(t, "brave-key", credentials["brave_api"])
	require.NotContains(t, credentials, "google_cse_cx")
	var googleConfig struct {
		APIKey string `json:"apiKey"`
		CX     string `json:"cx"`
	}
	require.NoError(t, json.Unmarshal([]byte(credentials["google_cse"]), &googleConfig))
	require.Equal(t, "google-api-key", googleConfig.APIKey)
	require.Equal(t, "google-cx", googleConfig.CX)
}

func TestCredentialStoreListAndDelete(t *testing.T) {
	db := newSearchCredentialTestDB(t)
	store := NewCredentialStore(db)
	ctx := context.Background()

	_, err := store.Set(ctx, "serper", "serper-key")
	require.NoError(t, err)
	items, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, len(credentialDefinitions))
	require.True(t, itemByEngineID(items, "serper").Configured)

	require.NoError(t, store.Delete(ctx, "serper"))
	items, err = store.List(ctx)
	require.NoError(t, err)
	require.False(t, itemByEngineID(items, "serper").Configured)
}

func TestCredentialStoreRejectsUnknownEngine(t *testing.T) {
	store := NewCredentialStore(newSearchCredentialTestDB(t))
	_, err := store.Set(context.Background(), "unknown", "value")
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "unsupported"))
}

func itemByEngineID(items []CredentialStatus, engineID string) CredentialStatus {
	for _, item := range items {
		if item.EngineID == engineID {
			return item
		}
	}
	return CredentialStatus{}
}
