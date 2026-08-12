package card

import (
	"encoding/base64"
	"encoding/json"
)

func parseV3PNG(data []byte) (*CharacterCard, map[string]json.RawMessage, error) {
	ccv3Data := extractPNGTextChunk(data, "ccv3")
	if len(ccv3Data) == 0 {
		return nil, nil, ErrPNGMetadataMissing
	}

	decoded, err := base64.StdEncoding.DecodeString(string(ccv3Data))
	if err != nil {
		decoded = ccv3Data
	}
	return parseV3JSON(decoded)
}
