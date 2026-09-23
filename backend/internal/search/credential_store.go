package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CredentialDefinition struct {
	EngineID string `json:"engineId"`
	Name     string `json:"name"`
	KeyURL   string `json:"keyUrl"`
}

type CredentialStatus struct {
	EngineID   string `json:"engineId"`
	Name       string `json:"name"`
	KeyURL     string `json:"keyUrl"`
	Configured bool   `json:"configured"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
}

type credentialRecord struct {
	EngineID  string
	Value     string
	UpdatedAt time.Time
}

// CredentialVault keeps search-engine secrets out of SQLite. The database only
// stores an opaque secret:// reference when a vault is configured.
type CredentialVault interface {
	Store(ctx context.Context, namespace string, value []byte) (string, error)
	Resolve(ctx context.Context, ref string) ([]byte, error)
	Delete(ctx context.Context, ref string) error
}

type CredentialStore struct {
	db    *sql.DB
	vault CredentialVault
}

var credentialDefinitions = []CredentialDefinition{
	{EngineID: "google_cse", Name: "Google CSE API Key", KeyURL: "https://console.cloud.google.com/apis/credentials"},
	{EngineID: "google_cse_cx", Name: "Google CSE CX", KeyURL: "https://programmablesearchengine.google.com/controlpanel/all"},
	{EngineID: "youtube", Name: "YouTube Data API Key", KeyURL: "https://console.cloud.google.com/apis/credentials"},
	{EngineID: "vimeo", Name: "Vimeo Access Token", KeyURL: "https://developer.vimeo.com/apps"},
	{EngineID: "serper", Name: "Serper API Key", KeyURL: "https://serper.dev/api-key"},
	{EngineID: "pexels", Name: "Pexels API Key", KeyURL: "https://www.pexels.com/api/"},
	{EngineID: "unsplash", Name: "Unsplash Access Key", KeyURL: "https://unsplash.com/oauth/applications"},
	{EngineID: "ebay_browse", Name: "eBay OAuth Bearer Token", KeyURL: "https://developer.ebay.com/my/keys"},
	{EngineID: "500px", Name: "500px Consumer Key", KeyURL: "https://500px.com/settings/applications"},
	{EngineID: "genius", Name: "Genius Client Access Token", KeyURL: "https://genius.com/api-clients"},
	{EngineID: "soundcloud", Name: "SoundCloud client_id", KeyURL: "https://developers.soundcloud.com/docs/api/guide#authentication"},
	{EngineID: "pinterest", Name: "Pinterest Access Token", KeyURL: "https://developers.pinterest.com/apps/"},
	{EngineID: "wordnik", Name: "Wordnik API Key", KeyURL: "https://www.wordnik.com/account/api"},
}

func NewCredentialStore(db *sql.DB) *CredentialStore { return &CredentialStore{db: db} }

func (s *CredentialStore) WithVault(vault CredentialVault) *CredentialStore {
	if s != nil {
		s.vault = vault
	}
	return s
}

func (s *CredentialStore) List(ctx context.Context) ([]CredentialStatus, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("search credential store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, "SELECT engine_id, api_key, updated_at FROM search_api_keys")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := make(map[string]credentialRecord)
	for rows.Next() {
		var record credentialRecord
		if err := rows.Scan(&record.EngineID, &record.Value, &record.UpdatedAt); err != nil {
			return nil, err
		}
		byID[record.EngineID] = record
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]CredentialStatus, 0, len(credentialDefinitions))
	for _, definition := range credentialDefinitions {
		item := CredentialStatus{EngineID: definition.EngineID, Name: definition.Name, KeyURL: definition.KeyURL}
		if record, ok := byID[definition.EngineID]; ok {
			item.Configured = strings.TrimSpace(record.Value) != ""
			if !record.UpdatedAt.IsZero() {
				item.UpdatedAt = record.UpdatedAt.UTC().Format(time.RFC3339)
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *CredentialStore) Set(ctx context.Context, engineID, value string) (CredentialStatus, error) {
	if s == nil || s.db == nil {
		return CredentialStatus{}, fmt.Errorf("search credential store is unavailable")
	}
	definition, ok := credentialDefinitionByID(engineID)
	if !ok {
		return CredentialStatus{}, fmt.Errorf("unsupported search engine")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return CredentialStatus{}, fmt.Errorf("credential value is required")
	}
	if len(value) > 8192 {
		return CredentialStatus{}, fmt.Errorf("credential value is too long")
	}

	storedValue := value
	if s.vault != nil {
		ref, err := s.vault.Store(ctx, "search/"+definition.EngineID, []byte(value))
		if err != nil {
			return CredentialStatus{}, err
		}
		storedValue = ref
	}
	oldValue, _ := s.rawValue(ctx, definition.EngineID)
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO search_api_keys (engine_id, api_key, updated_at)
        VALUES (?, ?, ?)
        ON CONFLICT(engine_id) DO UPDATE SET api_key = excluded.api_key, updated_at = excluded.updated_at`,
		definition.EngineID, storedValue, now)
	if err != nil {
		if s.vault != nil && isSecretReference(storedValue) {
			_ = s.vault.Delete(ctx, storedValue)
		}
		return CredentialStatus{}, err
	}
	if s.vault != nil && isSecretReference(oldValue) && oldValue != storedValue {
		_ = s.vault.Delete(ctx, oldValue)
	}
	return CredentialStatus{EngineID: definition.EngineID, Name: definition.Name, KeyURL: definition.KeyURL, Configured: true, UpdatedAt: now.Format(time.RFC3339)}, nil
}

func (s *CredentialStore) Delete(ctx context.Context, engineID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("search credential store is unavailable")
	}
	if _, ok := credentialDefinitionByID(engineID); !ok {
		return fmt.Errorf("unsupported search engine")
	}
	oldValue, _ := s.rawValue(ctx, engineID)
	if _, err := s.db.ExecContext(ctx, "DELETE FROM search_api_keys WHERE engine_id = ?", engineID); err != nil {
		return err
	}
	if s.vault != nil && isSecretReference(oldValue) {
		return s.vault.Delete(ctx, oldValue)
	}
	return nil
}

// MigrateLegacyCredentials moves plaintext values from the legacy SQLite table
// into the encrypted SecretBroker store. It is idempotent and intentionally
// keeps the existing column for backward-compatible schema upgrades.
func (s *CredentialStore) MigrateLegacyCredentials(ctx context.Context) error {
	if s == nil || s.db == nil || s.vault == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, "SELECT engine_id, api_key FROM search_api_keys")
	if err != nil {
		return err
	}
	records := make([]credentialRecord, 0)
	for rows.Next() {
		var record credentialRecord
		if err := rows.Scan(&record.EngineID, &record.Value); err != nil {
			rows.Close()
			return err
		}
		if strings.TrimSpace(record.Value) != "" && !isSecretReference(record.Value) {
			records = append(records, record)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, record := range records {
		ref, err := s.vault.Store(ctx, "search/"+normalizeEngineCredentialID(record.EngineID), []byte(record.Value))
		if err != nil {
			return err
		}
		result, err := s.db.ExecContext(ctx, "UPDATE search_api_keys SET api_key = ?, updated_at = ? WHERE engine_id = ? AND api_key = ?", ref, time.Now().UTC(), record.EngineID, record.Value)
		if err != nil {
			_ = s.vault.Delete(ctx, ref)
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			_ = s.vault.Delete(ctx, ref)
		}
	}
	return nil
}

// EngineCredentialSourceFactory returns a lazy source: only configuration
// metadata is loaded up-front; the selected engine secret is resolved on demand.
func (s *CredentialStore) EngineCredentialSourceFactory(ctx context.Context, providerID, invocation string) (EngineCredentialSource, func(), error) {
	if s == nil || s.db == nil {
		return nil, func() {}, nil
	}
	rows, err := s.db.QueryContext(ctx, "SELECT engine_id FROM search_api_keys WHERE TRIM(api_key) <> ''")
	if err != nil {
		return nil, func() {}, err
	}
	configured := map[string]bool{}
	for rows.Next() {
		var engineID string
		if err := rows.Scan(&engineID); err != nil {
			rows.Close()
			return nil, func() {}, err
		}
		configured[normalizeEngineCredentialID(engineID)] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, func() {}, err
	}
	rows.Close()
	source := &storeEngineCredentialSource{store: s, configured: configured}
	return source, func() {}, nil
}

type storeEngineCredentialSource struct {
	store      *CredentialStore
	configured map[string]bool
}

func (s *storeEngineCredentialSource) Available(ctx context.Context, engineID string) bool {
	if s == nil || s.store == nil {
		return false
	}
	engineID = normalizeEngineCredentialID(engineID)
	if engineID == "google_cse" {
		return s.configured["google_cse"] && s.configured["google_cse_cx"]
	}
	return s.configured[engineID]
}

func (s *storeEngineCredentialSource) Resolve(ctx context.Context, engineID string) (string, func(), error) {
	if s == nil || s.store == nil {
		return "", func() {}, nil
	}
	engineID = normalizeEngineCredentialID(engineID)
	if engineID == "google_cse" {
		apiKey, err := s.store.resolveValue(ctx, "google_cse")
		if err != nil {
			return "", func() {}, err
		}
		cx, err := s.store.resolveValue(ctx, "google_cse_cx")
		if err != nil {
			return "", func() {}, err
		}
		payload, err := json.Marshal(map[string]string{"apiKey": apiKey, "cx": cx})
		if err != nil {
			return "", func() {}, err
		}
		return string(payload), func() {}, nil
	}
	value, err := s.store.resolveValue(ctx, engineID)
	return value, func() {}, err
}

// ResolveEngineCredentials remains for compatibility with older integrations.
// Production wiring uses EngineCredentialSourceFactory so secrets are resolved
// per selected engine instead of loading all keys into a request context.
func (s *CredentialStore) ResolveEngineCredentials(ctx context.Context, providerID, invocation string) (map[string]string, func(), error) {
	source, release, err := s.EngineCredentialSourceFactory(ctx, providerID, invocation)
	if err != nil || source == nil {
		return nil, release, err
	}
	resolved := map[string]string{}
	for _, definition := range credentialDefinitions {
		if definition.EngineID == "google_cse_cx" || !source.Available(ctx, definition.EngineID) {
			continue
		}
		value, _, err := source.Resolve(ctx, definition.EngineID)
		if err != nil {
			return nil, release, err
		}
		if value != "" {
			resolved[definition.EngineID] = value
		}
	}
	return resolved, release, nil
}

func (s *CredentialStore) rawValue(ctx context.Context, engineID string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("search credential store is unavailable")
	}
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT api_key FROM search_api_keys WHERE engine_id = ?", normalizeEngineCredentialID(engineID)).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *CredentialStore) resolveValue(ctx context.Context, engineID string) (string, error) {
	raw, err := s.rawValue(ctx, engineID)
	if err != nil || raw == "" {
		return raw, err
	}
	if s.vault == nil || !isSecretReference(raw) {
		return raw, nil
	}
	value, err := s.vault.Resolve(ctx, raw)
	if err != nil {
		return "", err
	}
	defer zeroCredentialBytes(value)
	return string(value), nil
}

func isSecretReference(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "secret://") || strings.HasPrefix(strings.TrimSpace(value), "mcp-secret://")
}

func zeroCredentialBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func SupportedEngineCredentialIDs() []string {
	ids := make([]string, 0, len(credentialDefinitions))
	for _, definition := range credentialDefinitions {
		ids = append(ids, definition.EngineID)
	}
	return ids
}

func IsSupportedEngineCredentialID(engineID string) bool {
	_, ok := credentialDefinitionByID(engineID)
	return ok
}

func credentialDefinitionByID(engineID string) (CredentialDefinition, bool) {
	engineID = normalizeEngineCredentialID(engineID)
	for _, definition := range credentialDefinitions {
		if definition.EngineID == engineID {
			return definition, true
		}
	}
	return CredentialDefinition{}, false
}
