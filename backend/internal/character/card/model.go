package card

import "encoding/json"

type CharacterCardFormat string

const (
	FormatV2JSON  CharacterCardFormat = "v2_json"
	FormatV2PNG   CharacterCardFormat = "v2_png"
	FormatV3JSON  CharacterCardFormat = "v3_json"
	FormatV3PNG   CharacterCardFormat = "v3_png"
	FormatV3CHARX CharacterCardFormat = "v3_charx"
)

type CharacterCard struct {
	SourceFormat CharacterCardFormat
	Name         string
	Description  string
	Personality  string
	Scenario     string

	FirstMessage       string
	ExampleMessages    string
	AlternateGreetings []string

	SystemPrompt            string
	PostHistoryInstructions string

	CreatorNotes     string
	Creator          string
	CharacterVersion string
	Tags             []string

	CharacterBook *CharacterBook

	Assets []CharacterCardAsset

	Extensions map[string]any

	CreationDate     *int64
	ModificationDate *int64

	Nickname           string
	GroupOnlyGreetings []string
	Source             string

	Preserved map[string]json.RawMessage
}

type CharacterBook struct {
	Entries []CharacterBookEntry `json:"entries"`
}

type CharacterBookEntry struct {
	Keys           []string               `json:"keys"`
	SecondaryKeys  []string               `json:"secondary_keys"`
	Content        string                 `json:"content"`
	Enabled        bool                   `json:"enabled"`
	InsertionOrder int                    `json:"insertion_order"`
	CaseSensitive  *bool                  `json:"case_sensitive"`
	Selective      *bool                  `json:"selective"`
	Constant       *bool                  `json:"constant"`
	Position       *string                `json:"position"`
	Priority       int                    `json:"priority"`
	Extensions     map[string]any         `json:"extensions"`
	Preserved      map[string]json.RawMessage `json:"-"`
}

type CharacterCardAsset struct {
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	MIMEType string         `json:"mimeType"`
	Size     int64          `json:"size"`
	Data     []byte         ` json:"-"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CardImportRisk struct {
	Category string `json:"category"`
	Level    string `json:"level"`
	Message  string `json:"message"`
}

type CharacterCardPreview struct {
	Format             string           `json:"format"`
	SpecVersion        string           `json:"specVersion"`
	Name               string           `json:"name"`
	Creator            string           `json:"creator"`
	CharacterVersion   string           `json:"characterVersion"`
	DescriptionLength  int              `json:"descriptionLength"`
	PersonalityLength  int              `json:"personalityLength"`
	HasSystemPrompt    bool             `json:"hasSystemPrompt"`
	HasPostHistory     bool             `json:"hasPostHistory"`
	GreetingCount      int              `json:"greetingCount"`
	LorebookEntryCount int              `json:"lorebookEntryCount"`
	AssetCount         int              `json:"assetCount"`
	UnknownFieldCount  int              `json:"unknownFieldCount"`
	Risks              []CardImportRisk `json:"risks"`
}

type CharacterCardExportResult struct {
	ResourceURI string `json:"resourceUri"`
	Format      string `json:"format"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentHash string `json:"contentHash"`
}
