package card

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
)

func extractPNGTextChunk(data []byte, key string) []byte {
	if len(data) < 8 {
		return nil
	}
	offset := 8
	for offset+8 <= len(data) {
		chunkLength := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
		chunkType := string(data[offset+4 : offset+8])
		if chunkType == "tEXt" {
			chunkData := data[offset+8 : offset+8+chunkLength]
			if idx := bytes.IndexByte(chunkData, 0); idx > 0 {
				k := string(chunkData[:idx])
				v := string(chunkData[idx+1:])
				if strings.ToLower(k) == strings.ToLower(key) {
					decoded, err := base64.StdEncoding.DecodeString(v)
					if err == nil {
						return decoded
					}
					return []byte(v)
				}
			}
		}
		if chunkType == "IEND" {
			break
		}
		offset += 12 + chunkLength
	}
	return nil
}

func parseV2PNG(data []byte) (*CharacterCard, map[string]json.RawMessage, error) {
	charaData := extractPNGTextChunk(data, "chara")
	if len(charaData) == 0 {
		return nil, nil, ErrPNGMetadataMissing
	}
	return parseV2JSON(charaData)
}
