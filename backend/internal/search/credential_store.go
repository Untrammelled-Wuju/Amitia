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

type CredentialStore struct {
	db *sql.DB
}

var credentialDefinitions = []CredentialDefinition{
	{EngineID: "brave_api", Name: "Brave Search API Key", KeyURL: "https://brave.com/search/api/"},
	{EngineID: "google_cse", Name: "Google CSE API Key", KeyURL: "https://console.cloud.google.com/apis/credentials"},
	{EngineID: "google_cse_cx", Name: "Google CSE CX", KeyURL: "https://programmablesearchengine.google.com/controlpanel/all"},
	{EngineID: "youtube", Name: "YouTube Data API Key", KeyURL: "https://console.cloud.google.com/apis/credentials"},
	{EngineID: "serper", Name: "Serper API Key", KeyURL: "https://serper.dev/api-key"},
	{EngineID: "tavily", Name: "Tavily API Key", KeyURL: "https://app.tavily.com/home"},
	{EngineID: "exa", Name: "Exa API Key", KeyURL: "https://dashboard.exa.ai/api-keys"},
	{EngineID: "pexels", Name: "Pexels API Key", KeyURL: "https://www.pexels.com/api/"},
	{EngineID: "unsplash", Name: "Unsplash Access Key", KeyURL: "https://unsplash.com/oauth/applications"},
	{EngineID: "ebay_browse", Name: "eBay OAuth Bearer Token", KeyURL: "https://developer.ebay.com/my/keys"},
	{EngineID: "500px", Name: "500px Consumer Key", KeyURL: "https://500px.com/settings/applications"},
	{EngineID: "genius", Name: "Genius Client Access Token", KeyURL: "https://genius.com/api-clients"},
	{EngineID: "soundcloud", Name: "SoundCloud client_id", KeyURL: "https://developers.soundcloud.com/docs/api/guide#authentication"},
	{EngineID: "pinterest", Name: "Pinterest Access Token", KeyURL: "https://developers.pinterest.com/apps/"},
	{EngineID: "wordnik", Name: "Wordnik API Key", KeyURL: "https://www.wordnik.com/account/api"},
}

func NewCredentialStore(db *sql.DB) *CredentialStore {
	return &CredentialStore{db: db}
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
		item := CredentialStatus{
			EngineID: definition.EngineID,
			Name:     definition.Name,
			KeyURL:   definition.KeyURL,
		}
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
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO search_api_keys (engine_id, api_key, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(engine_id) DO UPDATE SET api_key = excluded.api_key, updated_at = excluded.updated_at`,
		definition.EngineID,
		value,
		now,
	)
	if err != nil {
		return CredentialStatus{}, err
	}
	return CredentialStatus{
		EngineID:   definition.EngineID,
		Name:       definition.Name,
		KeyURL:     definition.KeyURL,
		Configured: true,
		UpdatedAt:  now.Format(time.RFC3339),
	}, nil
}

func (s *CredentialStore) Delete(ctx context.Context, engineID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("search credential store is unavailable")
	}
	if _, ok := credentialDefinitionByID(engineID); !ok {
		return fmt.Errorf("unsupported search engine")
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM search_api_keys WHERE engine_id = ?", engineID)
	return err
}

func (s *CredentialStore) ResolveEngineCredentials(ctx context.Context, providerID, invocation string) (map[string]string, func(), error) {
	if s == nil || s.db == nil {
		return nil, func() {}, nil
	}
	rows, err := s.db.QueryContext(ctx, "SELECT engine_id, api_key FROM search_api_keys")
	if err != nil {
		return nil, func() {}, err
	}
	defer rows.Close()
	resolved := make(map[string]string)
	for rows.Next() {
		var engineID string
		var value string
		if err := rows.Scan(&engineID, &value); err != nil {
			return nil, func() {}, err
		}
		if _, ok := credentialDefinitionByID(engineID); !ok {
			continue
		}
		if strings.TrimSpace(value) != "" {
			resolved[engineID] = value
		}
	}
	if err := rows.Err(); err != nil {
		return nil, func() {}, err
	}
	if apiKey, ok := resolved["google_cse"]; ok {
		if cx, cxOK := resolved["google_cse_cx"]; cxOK {
			payload, err := json.Marshal(map[string]string{"apiKey": apiKey, "cx": cx})
			if err != nil {
				return nil, func() {}, err
			}
			resolved["google_cse"] = string(payload)
		}
	}
	delete(resolved, "google_cse_cx")
	return resolved, func() {}, nil
}

func credentialDefinitionByID(engineID string) (CredentialDefinition, bool) {
	engineID = strings.TrimSpace(engineID)
	for _, definition := range credentialDefinitions {
		if definition.EngineID == engineID {
			return definition, true
		}
	}
	return CredentialDefinition{}, false
}
