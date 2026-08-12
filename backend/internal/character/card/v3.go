package card

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type v3Card struct {
	Spec                   string         `json:"spec"`
	SpecVersion            string         `json:"spec_version"`
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	Personality            string         `json:"personality"`
	Scenario               string         `json:"scenario"`
	FirstMes               string         `json:"first_mes"`
	MesExample             string         `json:"mes_example"`
	SystemPrompt           string         `json:"system_prompt"`
	PostHistoryInstructions string        `json:"post_history_instructions"`
	AlternateGreetings     []string       `json:"alternate_greetings"`
	CharacterBook          *v3CharacterBook `json:"character_book"`
	Tags                   []string       `json:"tags"`
	Creator                string         `json:"creator"`
	CreatorNotes           string         `json:"creator_notes"`
	CharacterVersion       string         `json:"character_version"`
	Extensions             map[string]any `json:"extensions"`
	Assets                 []v3Asset      `json:"assets"`
	Nickname               string         `json:"nickname"`
	GroupOnlyGreetings     []string       `json:"group_only_greetings"`
	Source                 string         `json:"source"`

	CreationDate     *int64 `json:"creation_date"`
	ModificationDate *int64 `json:"modification_date"`
}

type v3CharacterBook struct {
	Entries []v3BookEntry `json:"entries"`
	Extensions map[string]any `json:"extensions"`
	Name     string `json:"name"`
}

type v3BookEntry struct {
	Keys           []string               `json:"keys"`
	SecondaryKeys  []string               `json:"secondary_keys"`
	Content        string                 `json:"content"`
	Enabled        bool                   `json:"enabled"`
	InsertionOrder int                    `json:"insertion_order"`
	CaseSensitive  *bool                  `json:"case_sensitive"`
	Selective      *bool                  `json:"selective"`
	Constant       *bool                  `json:"constant"`
	Position       *string                `json:"position"`
	Extensions     map[string]any         `json:"extensions"`
	Priority       int                    `json:"priority"`
	Name           string                 `json:"name"`
	ID             any                    `json:"id"`
}

type v3Asset struct {
	Type     string `json:"type"`
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Legend   any    `json:"legend"`
}

func parseV3JSON(data []byte) (*CharacterCard, map[string]json.RawMessage, error) {
	if len(data) > MaxJSONBytes {
		return nil, nil, ErrJSONInvalid
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, ErrJSONInvalid
	}

	preserved := extractPreservedFields(raw, knownV3Fields())

	var card v3Card
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, nil, ErrJSONInvalid
	}

	result := &CharacterCard{
		SourceFormat:            FormatV3JSON,
		Name:                    card.Name,
		Description:             card.Description,
		Personality:             card.Personality,
		Scenario:                card.Scenario,
		FirstMessage:            card.FirstMes,
		ExampleMessages:          card.MesExample,
		AlternateGreetings:       card.AlternateGreetings,
		SystemPrompt:            card.SystemPrompt,
		PostHistoryInstructions: card.PostHistoryInstructions,
		CreatorNotes:            card.CreatorNotes,
		Creator:                 card.Creator,
		CharacterVersion:        card.CharacterVersion,
		Tags:                    card.Tags,
		Extensions:              card.Extensions,
		Nickname:                card.Nickname,
		GroupOnlyGreetings:      card.GroupOnlyGreetings,
		Source:                  card.Source,
		Preserved:               preserved,
	}

	if card.CreationDate != nil {
		result.CreationDate = card.CreationDate
	}
	if card.ModificationDate != nil {
		result.ModificationDate = card.ModificationDate
	}

	if card.CharacterBook != nil {
		result.CharacterBook = &CharacterBook{
			Entries: make([]CharacterBookEntry, 0, len(card.CharacterBook.Entries)),
		}
		for _, e := range card.CharacterBook.Entries {
			result.CharacterBook.Entries = append(result.CharacterBook.Entries, CharacterBookEntry{
				Keys:           e.Keys,
				SecondaryKeys:  e.SecondaryKeys,
				Content:        e.Content,
				Enabled:        e.Enabled,
				InsertionOrder: e.InsertionOrder,
				CaseSensitive:  e.CaseSensitive,
				Selective:      e.Selective,
				Constant:       e.Constant,
				Position:       e.Position,
				Extensions:     e.Extensions,
				Priority:       e.Priority,
			})
		}
	}

	for _, a := range card.Assets {
		result.Assets = append(result.Assets, CharacterCardAsset{
			Type: a.Type,
			Name: a.Name,
			Metadata: map[string]any{
				"uri":      a.URI,
				"mimeType": a.MIMEType,
				"legend":   a.Legend,
			},
		})
	}

	return result, preserved, nil
}

func knownV3Fields() map[string]bool {
	return map[string]bool{
		"spec": true, "spec_version": true, "name": true, "description": true,
		"personality": true, "scenario": true, "first_mes": true, "mes_example": true,
		"creator_notes": true, "system_prompt": true, "post_history_instructions": true,
		"alternate_greetings": true, "character_book": true, "tags": true,
		"creator": true, "character_version": true, "extensions": true,
		"assets": true, "nickname": true, "group_only_greetings": true,
		"source": true, "creation_date": true, "modification_date": true,
	}
}

func v3SpecVersion(spec string) (major, minor int, future bool) {
	parts := strings.Split(spec, ".")
	if len(parts) != 2 {
		return 3, 0, false
	}
	major, _ = strconv.Atoi(parts[0])
	minor, _ = strconv.Atoi(parts[1])
	return major, minor, major > 3
}

func daysToUnixDays(days int64) *int64 {
	t := time.Unix(days*86400, 0)
	unixMs := t.UnixMilli()
	return &unixMs
}

func unixMsToUnixDays(ms *int64) int64 {
	if ms == nil {
		return 0
	}
	return *ms / 86400000
}
