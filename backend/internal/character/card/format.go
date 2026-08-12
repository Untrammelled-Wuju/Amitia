package card

import (
	"bytes"
	"encoding/json"
	"strings"
)

type rawCardHeader struct {
	Spec      string `json:"spec"`
	SpecVersion string `json:"spec_version"`
}

func DetectFormat(data []byte, filename string) (CharacterCardFormat, error) {
	if len(data) == 0 {
		return "", ErrInvalidCard
	}

	ext := strings.ToLower(filename)
	if strings.HasSuffix(ext, ".charx") {
		return FormatV3CHARX, nil
	}

	if isPNG(data) {
		return detectPNGFormat(data)
	}

	if isJSON(data) {
		return detectJSONFormat(data)
	}

	if strings.HasSuffix(ext, ".png") || strings.HasSuffix(ext, ".apng") {
		return "", ErrPNGMetadataMissing
	}

	return "", ErrUnsupportedFormat
}

func isPNG(data []byte) bool {
	return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
}

func isJSON(data []byte) bool {
	for _, b := range data {
		if b == '{' {
			return true
		}
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return false
		}
	}
	return false
}

func detectPNGFormat(data []byte) (CharacterCardFormat, error) {
	ccv3Data := extractPNGTextChunk(data, "ccv3")
	if len(ccv3Data) > 0 {
		return FormatV3PNG, nil
	}
	charaData := extractPNGTextChunk(data, "chara")
	if len(charaData) > 0 {
		return FormatV2PNG, nil
	}
	return "", ErrPNGMetadataMissing
}

func detectJSONFormat(data []byte) (CharacterCardFormat, error) {
	if len(data) > MaxJSONBytes {
		return "", ErrJSONInvalid
	}

	var header rawCardHeader
	if err := json.Unmarshal(data, &header); err != nil {
		return "", ErrJSONInvalid
	}

	spec := strings.TrimSpace(header.Spec)
	switch spec {
	case "chara_card_v2":
		return FormatV2JSON, nil
	case "chara_card_v3":
		return FormatV3JSON, nil
	}
	return "", ErrUnsupportedFormat
}
