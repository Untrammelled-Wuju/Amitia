package card

import (
	"encoding/json"
	"strings"
)

type tavernCard struct {
	Name                    string         `json:"name"`
	Description             string         `json:"description"`
	Personality             string         `json:"personality"`
	Scenario                string         `json:"scenario"`
	FirstMes                string         `json:"first_mes"`
	MesExample              string         `json:"mes_example"`
	CreatorComment          string         `json:"creatorcomment"`
	CreatorNotes            string         `json:"creator_notes"`
	SystemPrompt            string         `json:"system_prompt"`
	PostHistoryInstructions string         `json:"post_history_instructions"`
	AlternateGreetings      []string       `json:"alternate_greetings"`
	Tags                    []string       `json:"tags"`
	Creator                 string         `json:"creator"`
	CharacterVersion        string         `json:"character_version"`
	Extensions              map[string]any `json:"extensions"`
	CharName                string         `json:"char_name"`
	CharPersona             string         `json:"char_persona"`
	WorldScenario           string         `json:"world_scenario"`
	CharGreeting            string         `json:"char_greeting"`
	ExampleDialogue         string         `json:"example_dialogue"`
}

func isTavernCard(data []byte) bool {
	var card tavernCard
	if err := json.Unmarshal(data, &card); err != nil {
		return false
	}
	name := firstNonEmpty(card.Name, card.CharName)
	content := firstNonEmpty(card.Description, card.Personality, card.Scenario, card.FirstMes, card.MesExample, card.CharPersona, card.WorldScenario, card.CharGreeting, card.ExampleDialogue)
	return strings.TrimSpace(name) != "" && strings.TrimSpace(content) != ""
}

func parseTavernJSON(data []byte) (*CharacterCard, map[string]json.RawMessage, error) {
	if len(data) > MaxJSONBytes {
		return nil, nil, ErrJSONInvalid
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil, ErrJSONInvalid
	}

	var card tavernCard
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, nil, ErrJSONInvalid
	}
	if !isTavernCard(data) {
		return nil, nil, ErrUnsupportedFormat
	}

	preserved := extractPreservedFields(raw, knownTavernFields())
	result := &CharacterCard{
		SourceFormat:            FormatTavernJSON,
		Name:                    firstNonEmpty(card.Name, card.CharName),
		Description:             firstNonEmpty(card.Description, card.CharPersona),
		Personality:             card.Personality,
		Scenario:                firstNonEmpty(card.Scenario, card.WorldScenario),
		FirstMessage:            firstNonEmpty(card.FirstMes, card.CharGreeting),
		ExampleMessages:         firstNonEmpty(card.MesExample, card.ExampleDialogue),
		AlternateGreetings:      card.AlternateGreetings,
		SystemPrompt:            card.SystemPrompt,
		PostHistoryInstructions: card.PostHistoryInstructions,
		CreatorNotes:            firstNonEmpty(card.CreatorNotes, card.CreatorComment),
		Creator:                 card.Creator,
		CharacterVersion:        card.CharacterVersion,
		Tags:                    card.Tags,
		Extensions:              card.Extensions,
		Preserved:               preserved,
	}

	return result, preserved, nil
}

func parseTavernPNG(data []byte) (*CharacterCard, map[string]json.RawMessage, error) {
	charaData := extractPNGTextChunk(data, "chara")
	if len(charaData) == 0 {
		return nil, nil, ErrPNGMetadataMissing
	}
	result, preserved, err := parseTavernJSON(charaData)
	if err != nil {
		return nil, nil, err
	}
	result.SourceFormat = FormatTavernPNG
	return result, preserved, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func knownTavernFields() map[string]bool {
	return map[string]bool{
		"name": true, "description": true, "personality": true, "scenario": true,
		"first_mes": true, "mes_example": true, "creatorcomment": true,
		"creator_notes": true, "system_prompt": true, "post_history_instructions": true,
		"alternate_greetings": true, "tags": true, "creator": true,
		"character_version": true, "extensions": true, "char_name": true,
		"char_persona": true, "world_scenario": true, "char_greeting": true,
		"example_dialogue": true, "avatar": true, "chat": true,
		"talkativeness": true, "fav": true,
	}
}
