package card

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

type CardParser struct{}

func NewCardParser() *CardParser {
	return &CardParser{}
}

func (p *CardParser) Parse(data []byte, filename string) (*CharacterCard, map[string]json.RawMessage, error) {
	if len(data) == 0 {
		return nil, nil, ErrInvalidCard
	}
	if len(data) > MaxInputBytes {
		return nil, nil, ErrCardTooLarge
	}

	format, err := DetectFormat(data, filename)
	if err != nil {
		return nil, nil, err
	}

	return p.ParseWithFormat(data, format)
}

func (p *CardParser) ParseWithFormat(data []byte, format CharacterCardFormat) (*CharacterCard, map[string]json.RawMessage, error) {
	switch format {
	case FormatV2JSON:
		return parseV2JSON(data)
	case FormatV2PNG:
		return parseV2PNG(data)
	case FormatV3JSON:
		return parseV3JSON(data)
	case FormatV3PNG:
		return parseV3PNG(data)
	case FormatV3CHARX:
		return parseCHARX(data)
	}
	return nil, nil, ErrUnsupportedFormat
}

func ComputeSourceHash(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

type CharacterMapping struct {
	Name         string
	Description  string
	Personality  string
	Scenario     string
	SystemPrompt string
}

func (card *CharacterCard) ToCharacterMapping() CharacterMapping {
	systemPrompt := strings.TrimSpace(card.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = strings.TrimSpace(card.BasePromptCandidate())
	}

	return CharacterMapping{
		Name:         strings.TrimSpace(card.Name),
		Description:  strings.TrimSpace(card.Description),
		Personality:  strings.TrimSpace(card.Personality),
		Scenario:     strings.TrimSpace(card.Scenario),
		SystemPrompt: systemPrompt,
	}
}

func (card *CharacterCard) BasePromptCandidate() string {
	var parts []string
	if card.ExampleMessages != "" {
		parts = append(parts, card.ExampleMessages)
	}
	return strings.Join(parts, "\n")
}

func (card *CharacterCard) ValidateForImport() error {
	if len(card.Description) > MaxDescriptionBytes {
		return fmt.Errorf("%w: description too large", ErrPromptTooLarge)
	}
	if len(card.Personality) > MaxPersonalityBytes {
		return fmt.Errorf("%w: personality too large", ErrPromptTooLarge)
	}
	if len(card.Scenario) > MaxScenarioBytes {
		return fmt.Errorf("%w: scenario too large", ErrPromptTooLarge)
	}
	if len(card.SystemPrompt) > MaxSystemPromptBytes {
		return fmt.Errorf("%w: system prompt too large", ErrPromptTooLarge)
	}
	if len(card.PostHistoryInstructions) > MaxPostHistoryBytes {
		return fmt.Errorf("%w: post history too large", ErrPromptTooLarge)
	}
	if len(card.ExampleMessages) > MaxExampleMsgBytes {
		return fmt.Errorf("%w: example messages too large", ErrPromptTooLarge)
	}

	if card.CharacterBook != nil {
		totalSize := 0
		for _, entry := range card.CharacterBook.Entries {
			if len(entry.Content) > MaxLorebookEntryBytes {
				return fmt.Errorf("%w: lorebook entry too large", ErrLorebookTooLarge)
			}
			totalSize += len(entry.Content)
		}
		if totalSize > MaxTotalLorebookBytes {
			return fmt.Errorf("%w: lorebook total too large", ErrLorebookTooLarge)
		}
		if len(card.CharacterBook.Entries) > MaxLorebookEntries {
			return fmt.Errorf("%w: lorebook entry count too large", ErrLorebookTooLarge)
		}
	}

	return nil
}

func (card *CharacterCard) SanitizeName() string {
	name := strings.TrimSpace(card.Name)
	if name == "" {
		name = "导入的角色"
	}
	if len([]rune(name)) > 64 {
		name = string([]rune(name)[:64])
	}
	return name
}

func (card *CharacterCard) IsEmpty() bool {
	return card.Name == "" && card.Description == "" && card.Personality == "" &&
		card.Scenario == "" && card.FirstMessage == "" && card.ExampleMessages == "" &&
		card.SystemPrompt == "" && card.PostHistoryInstructions == ""
}

func EstimateMemoryUsage(data []byte) int64 {
	return int64(len(data) * 3)
}

func ValidateImageBytes(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if bytes.Equal(data[:8], pngMagic) {
		return true
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return true
	}
	if len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")) {
		return true
	}
	if len(data) >= 6 && (bytes.Equal(data[0:4], []byte("GIF8")) || bytes.Equal(data[0:6], []byte("GIF89a"))) {
		return true
	}
	return false
}

func IsExecutableAsset(filename string, data []byte) bool {
	exeExtensions := []string{".exe", ".dll", ".bat", ".cmd", ".sh", ".py", ".js", ".ts"}
	lower := strings.ToLower(filename)
	for _, ext := range exeExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	if len(data) >= 2 && data[0] == '#' && data[1] == '!' {
		return true
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x4D, 0x5A, 0x90, 0x00}) {
		return true
	}
	return false
}
