package card

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
)

type CHARXExtract struct {
	CardJSON []byte
	Assets   map[string][]byte
}

func parseCHARX(data []byte) (*CharacterCard, map[string]json.RawMessage, error) {
	if len(data) > MaxCHARXBytes {
		return nil, nil, ErrCHARXInvalid
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, ErrCHARXInvalid
	}

	extract, err := extractCHARX(reader)
	if err != nil {
		return nil, nil, err
	}

	return parseV3JSON(extract.CardJSON)
}

func extractCHARX(reader *zip.Reader) (*CHARXExtract, error) {
	result := &CHARXExtract{
		Assets: make(map[string][]byte),
	}

	totalAssetBytes := int64(0)
	assetCount := 0

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if !isSafePath(file.Name) {
			continue
		}
		if len(file.Name) > 256 {
			continue
		}

		lowerName := strings.ToLower(file.Name)
		if lowerName == "card.json" || lowerName == "/card.json" {
			data, err := readZipEntry(file, MaxJSONBytes)
			if err != nil {
				return nil, err
			}
			result.CardJSON = data
			continue
		}

		if assetCount >= MaxAssets {
			continue
		}

		data, err := readZipEntry(file, MaxSingleAssetBytes)
		if err != nil {
			continue
		}
		if totalAssetBytes+int64(len(data)) > MaxTotalAssetBytes {
			continue
		}

		result.Assets[file.Name] = data
		totalAssetBytes += int64(len(data))
		assetCount++
	}

	if len(result.CardJSON) == 0 {
		return nil, ErrCHARXInvalid
	}

	return result, nil
}

func isSafePath(name string) bool {
	if strings.Contains(name, "..") {
		return false
	}
	if strings.HasPrefix(name, "/") {
		return false
	}
	if strings.HasPrefix(name, `\`) {
		return false
	}
	if len(name) > 0 && (name[0] >= 'A' && name[0] <= 'Z' || name[0] >= 'a' && name[0] <= 'z') && len(name) > 1 && name[1] == ':' {
		return false
	}
	clean := filepath.Clean(name)
	if clean != name {
		return false
	}
	return true
}

func readZipEntry(file *zip.File, maxSize int) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	limitReader := io.LimitReader(rc, int64(maxSize)+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}
	if len(data) > maxSize {
		return nil, ErrAssetTooLarge
	}
	return data, nil
}

func buildCHARX(cardJSON []byte, assets map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	cardWriter, err := w.Create("card.json")
	if err != nil {
		return nil, err
	}
	if _, err := cardWriter.Write(cardJSON); err != nil {
		return nil, err
	}

	for name, data := range assets {
		if !isSafePath(name) {
			continue
		}
		fw, err := w.Create(name)
		if err != nil {
			continue
		}
		fw.Write(data)
	}

	w.Close()
	return buf.Bytes(), nil
}

func buildV3JSON(card *CharacterCard, preserved map[string]json.RawMessage) ([]byte, error) {
	output := map[string]any{
		"spec":       "chara_card_v3",
		"spec_version": "3.0",
	}

	if card.Name != "" {
		output["name"] = card.Name
	}
	if card.Description != "" {
		output["description"] = card.Description
	}
	if card.Personality != "" {
		output["personality"] = card.Personality
	}
	if card.Scenario != "" {
		output["scenario"] = card.Scenario
	}
	if card.FirstMessage != "" {
		output["first_mes"] = card.FirstMessage
	}
	if card.ExampleMessages != "" {
		output["mes_example"] = card.ExampleMessages
	}
	if len(card.AlternateGreetings) > 0 {
		output["alternate_greetings"] = card.AlternateGreetings
	}
	if card.SystemPrompt != "" {
		output["system_prompt"] = card.SystemPrompt
	}
	if card.PostHistoryInstructions != "" {
		output["post_history_instructions"] = card.PostHistoryInstructions
	}
	if card.Creator != "" {
		output["creator"] = card.Creator
	}
	if card.CreatorNotes != "" {
		output["creator_notes"] = card.CreatorNotes
	}
	if card.CharacterVersion != "" {
		output["character_version"] = card.CharacterVersion
	}
	if len(card.Tags) > 0 {
		output["tags"] = card.Tags
	}
	if card.Nickname != "" {
		output["nickname"] = card.Nickname
	}
	if len(card.GroupOnlyGreetings) > 0 {
		output["group_only_greetings"] = card.GroupOnlyGreetings
	}
	if card.Source != "" {
		output["source"] = card.Source
	}
	if card.CreationDate != nil {
		output["creation_date"] = *card.CreationDate
	}
	if card.ModificationDate != nil {
		output["modification_date"] = *card.ModificationDate
	}

	if len(card.Extensions) > 0 {
		output["extensions"] = card.Extensions
	}

	if card.CharacterBook != nil && len(card.CharacterBook.Entries) > 0 {
		entries := make([]map[string]any, 0, len(card.CharacterBook.Entries))
		for _, e := range card.CharacterBook.Entries {
			entry := map[string]any{
				"keys":            e.Keys,
				"content":         e.Content,
				"enabled":         e.Enabled,
				"insertion_order": e.InsertionOrder,
			}
			if len(e.SecondaryKeys) > 0 {
				entry["secondary_keys"] = e.SecondaryKeys
			}
			if e.CaseSensitive != nil {
				entry["case_sensitive"] = *e.CaseSensitive
			}
			if e.Selective != nil {
				entry["selective"] = *e.Selective
			}
			if e.Constant != nil {
				entry["constant"] = *e.Constant
			}
			if e.Position != nil {
				entry["position"] = *e.Position
			}
			if e.Priority != 0 {
				entry["priority"] = e.Priority
			}
			if len(e.Extensions) > 0 {
				entry["extensions"] = e.Extensions
			}
			entries = append(entries, entry)
		}
		output["character_book"] = map[string]any{"entries": entries}
	}

	for k, v := range preserved {
		output[k] = v
	}

	return json.MarshalIndent(output, "", "  ")
}

func buildV2JSON(card *CharacterCard, preserved map[string]json.RawMessage) ([]byte, error) {
	output := map[string]any{
		"spec":         "chara_card_v2",
		"spec_version": "2.0",
	}

	if card.Name != "" {
		output["name"] = card.Name
	}
	if card.Description != "" {
		output["description"] = card.Description
	}
	if card.Personality != "" {
		output["personality"] = card.Personality
	}
	if card.Scenario != "" {
		output["scenario"] = card.Scenario
	}
	if card.FirstMessage != "" {
		output["first_mes"] = card.FirstMessage
	}
	if card.ExampleMessages != "" {
		output["mes_example"] = card.ExampleMessages
	}
	if card.Creator != "" {
		output["creator"] = card.Creator
	}
	if card.CreatorNotes != "" {
		output["creator_notes"] = card.CreatorNotes
	}
	if card.CharacterVersion != "" {
		output["character_version"] = card.CharacterVersion
	}
	if len(card.Tags) > 0 {
		output["tags"] = card.Tags
	}
	if card.SystemPrompt != "" {
		output["system_prompt"] = card.SystemPrompt
	}
	if card.PostHistoryInstructions != "" {
		output["post_history_instructions"] = card.PostHistoryInstructions
	}
	if len(card.AlternateGreetings) > 0 {
		output["alternate_greetings"] = card.AlternateGreetings
	}

	if len(card.Extensions) > 0 {
		output["extensions"] = card.Extensions
	}

	if card.CharacterBook != nil && len(card.CharacterBook.Entries) > 0 {
		entries := make([]map[string]any, 0, len(card.CharacterBook.Entries))
		for _, e := range card.CharacterBook.Entries {
			entry := map[string]any{
				"keys":            e.Keys,
				"content":         e.Content,
				"enabled":         e.Enabled,
				"insertion_order": e.InsertionOrder,
			}
			if len(e.SecondaryKeys) > 0 {
				entry["secondary_keys"] = e.SecondaryKeys
			}
			if e.CaseSensitive != nil {
				entry["case_sensitive"] = *e.CaseSensitive
			}
			if e.Selective != nil {
				entry["selective"] = *e.Selective
			}
			if e.Constant != nil {
				entry["constant"] = *e.Constant
			}
			if e.Position != nil {
				entry["position"] = *e.Position
			}
			entries = append(entries, entry)
		}
		output["character_book"] = map[string]any{"entries": entries}
	}

	for k, v := range preserved {
		output[k] = v
	}

	return json.MarshalIndent(output, "", "  ")
}
