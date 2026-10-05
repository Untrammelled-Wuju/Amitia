package audioformat

import (
	"bytes"
	"encoding/binary"
	"errors"
)

func Validate(data []byte, mimeType string) error {
	switch mimeType {
	case "audio/wav":
		return ValidateWAV(data)
	case "audio/webm":
		if len(data) >= 32 && bytes.Equal(data[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
			return nil
		}
		return errors.New("WebM 音频头无效")
	default:
		return errors.New("音频类型不受支持")
	}
}

func ValidateWAV(data []byte) error {
	invalid := errors.New("音频必须是完整的单声道 PCM16 / 16 kHz WAV")
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		return invalid
	}
	formatFound, dataFound := false, false
	position, chunks := 12, 0
	for position < len(data) {
		chunks++
		if chunks > 512 || len(data)-position < 8 {
			return invalid
		}
		length := uint64(binary.LittleEndian.Uint32(data[position+4 : position+8]))
		start := position + 8
		if length > uint64(len(data)-start) {
			return invalid
		}
		end := start + int(length)
		switch string(data[position : position+4]) {
		case "fmt ":
			if formatFound || length < 16 || length > 8192 || binary.LittleEndian.Uint16(data[start:start+2]) != 1 || binary.LittleEndian.Uint16(data[start+2:start+4]) != 1 || binary.LittleEndian.Uint32(data[start+4:start+8]) != 16000 || binary.LittleEndian.Uint32(data[start+8:start+12]) != 32000 || binary.LittleEndian.Uint16(data[start+12:start+14]) != 2 || binary.LittleEndian.Uint16(data[start+14:start+16]) != 16 {
				return invalid
			}
			formatFound = true
		case "data":
			if !formatFound || dataFound || length == 0 || length%2 != 0 {
				return invalid
			}
			dataFound = true
		}
		position = end + int(length%2)
		if position > len(data) {
			return invalid
		}
	}
	if !formatFound || !dataFound {
		return invalid
	}
	return nil
}
