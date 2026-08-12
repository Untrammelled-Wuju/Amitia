package card

import (
	"encoding/json"
	"strconv"
)

type v2Card struct {
	Spec             string         `json:"spec"`
	SpecVersion      string         `json:"spec_version"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Personality      string         `json:"personality"`
	Scenario         string         `json:"scenario"`
	FirstMes         string         `json:"first_mes"`
	MesExample       string         `json:"mes_example"`
	CreatorNotes     string         `json:"creator_notes"`
	SystemPrompt     string         `json:"system_prompt"`
	PostHistoryInstructions string  `json:"post_history_instructions"`
	AlternateGreetings []string     `json:"alternate_greetings"`
	CharacterBook    *v2CharacterBook `json:"character_book"`
	Tags             []string       `json:"tags"`
	Creator          string         `json:"creator"`
	CharacterVersion string         `json:"character_version"`
	Extensions       map[string]any `json:"extensions"`
}

type v2CharacterBook struct {
	Entries []v2BookEntry `json:"entries"`
}

type v2BookEntry struct {
	Keys           []string `json:"keys"`
	SecondaryKeys  []string `json:"secondary_keys"`
	Content        string   `json:"content"`
	Enabled        bool     `json:"enabled"`
	InsertionOrder int      `json:"insertion_order"`
	CaseSensitive  *bool    `json:"case_sensitive"`
	Selective      *bool    `json:"selective"`
	Constant       *bool    `json:"constant"`
	Position       *string  `json:"position"`
}

func parseV2JSON(data []byte) (*CharacterCard, map[string]json.RawMessage, error) {
	if len(data) > MaxJSONBytes {
		return nil, nil, ErrJSONInvalid
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, ErrJSONInvalid
	}

	safeRaw := make(map[string]json.RawMessage)
	for k, v := range raw {
		safeRaw[k] = v
	}

	preserved := extractPreservedFields(raw, knownV2Fields())

	var card v2Card
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, nil, ErrJSONInvalid
	}

	result := &CharacterCard{
		SourceFormat:            FormatV2JSON,
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
		Preserved:               preserved,
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
			})
		}
	}

	return result, preserved, nil
}

func knownV2Fields() map[string]bool {
	return map[string]bool{
		"spec": true, "spec_version": true, "name": true, "description": true,
		"personality": true, "scenario": true, "first_mes": true, "mes_example": true,
		"creator_notes": true, "system_prompt": true, "post_history_instructions": true,
		"alternate_greetings": true, "character_book": true, "tags": true,
		"creator": true, "character_version": true, "extensions": true,
	}
}

func extractPreservedFields(raw map[string]json.RawMessage, known map[string]bool) map[string]json.RawMessage {
	preserved := make(map[string]json.RawMessage)
	for k, v := range raw {
		if !known[k] {
			preserved[k] = v
		}
	}
	return preserved
}

func specVersionString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(int(val))
	}
	return ""
}
