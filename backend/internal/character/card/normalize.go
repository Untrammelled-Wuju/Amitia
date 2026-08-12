package card

import "encoding/json"

type CharacterCardData struct {
	FirstMessage       string         `json:"firstMessage,omitempty"`
	AlternateGreetings []string       `json:"alternateGreetings,omitempty"`
	ExampleMessages    string         `json:"exampleMessages,omitempty"`
	Scenario           string         `json:"scenario,omitempty"`
	Creator            string         `json:"creator,omitempty"`
	CreatorNotes       string         `json:"creatorNotes,omitempty"`
	CharacterVersion   string         `json:"characterVersion,omitempty"`
	Tags               []string       `json:"tags,omitempty"`
	SystemPrompt       string         `json:"systemPrompt,omitempty"`
	PostHistoryInstructions string   `json:"postHistoryInstructions,omitempty"`
	Nickname           string         `json:"nickname,omitempty"`
	GroupOnlyGreetings []string       `json:"groupOnlyGreetings,omitempty"`
	ExternalExtensions map[string]any `json:"externalExtensions,omitempty"`
	SourceFormat       string         `json:"sourceFormat,omitempty"`
	Preserved          map[string]json.RawMessage `json:"preserved,omitempty"`
}

func (card *CharacterCard) ToCardData() *CharacterCardData {
	cd := &CharacterCardData{
		FirstMessage:            card.FirstMessage,
		AlternateGreetings:      card.AlternateGreetings,
		ExampleMessages:         card.ExampleMessages,
		Scenario:                card.Scenario,
		Creator:                 card.Creator,
		CreatorNotes:            card.CreatorNotes,
		CharacterVersion:        card.CharacterVersion,
		Tags:                    card.Tags,
		SystemPrompt:            card.SystemPrompt,
		PostHistoryInstructions: card.PostHistoryInstructions,
		Nickname:                card.Nickname,
		GroupOnlyGreetings:      card.GroupOnlyGreetings,
		SourceFormat:            string(card.SourceFormat),
	}

	if len(card.Extensions) > 0 {
		cd.ExternalExtensions = card.Extensions
	}
	if len(card.Preserved) > 0 {
		cd.Preserved = card.Preserved
	}

	return cd
}

func (card *CharacterCard) HasFutureSpec() bool {
	switch card.SourceFormat {
	case FormatV2JSON, FormatV2PNG:
		return false
	case FormatV3JSON, FormatV3PNG, FormatV3CHARX:
		return false
	}
	return true
}

func (card *CharacterCard) Clone() *CharacterCard {
	clone := *card
	if card.AlternateGreetings != nil {
		clone.AlternateGreetings = append([]string{}, card.AlternateGreetings...)
	}
	if card.GroupOnlyGreetings != nil {
		clone.GroupOnlyGreetings = append([]string{}, card.GroupOnlyGreetings...)
	}
	if card.Tags != nil {
		clone.Tags = append([]string{}, card.Tags...)
	}
	if card.Assets != nil {
		clone.Assets = append([]CharacterCardAsset{}, card.Assets...)
	}
	if card.Extensions != nil {
		clone.Extensions = make(map[string]any, len(card.Extensions))
		for k, v := range card.Extensions {
			clone.Extensions[k] = v
		}
	}
	if card.Preserved != nil {
		clone.Preserved = make(map[string]json.RawMessage, len(card.Preserved))
		for k, v := range card.Preserved {
			clone.Preserved[k] = v
		}
	}
	return &clone
}
