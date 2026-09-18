// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type cascadeVoiceReply struct {
	InteractionMode   string            `json:"interaction_mode"`
	SpeechInstruction string            `json:"speech_instruction"`
	SpeechText        string            `json:"speech_text"`
	UserAffect        CascadeUserAffect `json:"user_affect"`
}

const cascadeVoiceJSONMaxBytes = 128 * 1024

type cascadeVoiceJSONParser struct {
	raw              string
	instructionReady bool
	instruction      string
	textStart        int
	textScan         int
	textDone         bool
	textDecoded      strings.Builder
}

func (p *cascadeVoiceJSONParser) Feed(delta string) (string, bool, error) {
	if delta == "" {
		return "", p.instructionReady, nil
	}
	if len(p.raw)+len(delta) > cascadeVoiceJSONMaxBytes {
		return "", p.instructionReady, fmt.Errorf("voice reply JSON exceeds size limit")
	}
	p.raw += delta
	instructionBecameReady := false
	if !p.instructionReady {
		value, _, ok, err := extractCascadeJSONStringField(p.raw, "speech_instruction", 0)
		if err != nil {
			return "", false, err
		}
		if ok {
			p.instruction = strings.TrimSpace(value)
			if p.instruction == "" {
				return "", false, fmt.Errorf("speech_instruction is empty")
			}
			p.instructionReady = true
			instructionBecameReady = true
		}
	}
	if p.textDone {
		return "", instructionBecameReady, nil
	}
	if p.textStart == 0 {
		if start, found := findCascadeJSONStringValueStart(p.raw, "speech_text", 0); found {
			p.textStart = start
			p.textScan = start
		}
	}
	if p.textStart == 0 || p.textScan >= len(p.raw) {
		return "", instructionBecameReady, nil
	}
	fragment, consumed, done, err := decodeCascadeJSONStringFragment(p.raw[p.textScan:])
	if err != nil {
		return "", instructionBecameReady, err
	}
	p.textScan += consumed
	if fragment != "" {
		p.textDecoded.WriteString(fragment)
	}
	if done {
		p.textDone = true
	}
	return fragment, instructionBecameReady, nil
}

func (p *cascadeVoiceJSONParser) Instruction() string {
	return p.instruction
}

func (p *cascadeVoiceJSONParser) Finalize() (*cascadeVoiceReply, error) {
	normalized := strings.TrimSpace(p.raw)
	if normalized == "" {
		return nil, fmt.Errorf("voice reply output is empty")
	}
	if first := strings.Index(normalized, "{"); first >= 0 {
		if last := strings.LastIndex(normalized, "}"); last >= first {
			normalized = normalized[first : last+1]
		}
	}
	modeKey := strings.Index(normalized, strconv.Quote("interaction_mode"))
	instructionKey := strings.Index(normalized, strconv.Quote("speech_instruction"))
	textKey := strings.Index(normalized, strconv.Quote("speech_text"))
	if modeKey < 0 || instructionKey < 0 || textKey < 0 {
		return nil, fmt.Errorf("voice reply JSON is missing required fields")
	}
	var reply cascadeVoiceReply
	if err := json.Unmarshal([]byte(normalized), &reply); err != nil {
		return nil, fmt.Errorf("invalid voice reply JSON: %w", err)
	}
	reply.InteractionMode = normalizeCascadeInteractionMode(reply.InteractionMode)
	reply.SpeechInstruction = strings.TrimSpace(reply.SpeechInstruction)
	reply.SpeechText = strings.TrimSpace(reply.SpeechText)
	if reply.SpeechInstruction == "" {
		return nil, fmt.Errorf("speech_instruction is empty")
	}
	if reply.SpeechInstruction != p.instruction {
		return nil, fmt.Errorf("streamed speech_instruction does not match final JSON")
	}
	if reply.InteractionMode != "WAIT" && reply.SpeechText == "" {
		return nil, fmt.Errorf("speech_text is empty")
	}
	if strings.TrimSpace(p.textDecoded.String()) != reply.SpeechText {
		return nil, fmt.Errorf("streamed speech_text does not match final JSON")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(normalized), &raw); err != nil {
		return nil, err
	}
	required := []string{"interaction_mode", "speech_instruction", "speech_text", "user_affect"}
	if len(raw) != len(required) {
		return nil, fmt.Errorf("voice reply JSON must contain exactly four unique fields")
	}
	for _, key := range required {
		if _, ok := raw[key]; !ok || strings.Count(normalized, strconv.Quote(key)) != 1 {
			return nil, fmt.Errorf("voice reply JSON field %s is invalid", key)
		}
	}
	reply.UserAffect = normalizeCascadeUserAffect(reply.UserAffect)
	return &reply, nil
}

func normalizeCascadeInteractionMode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "WAIT", "MICRO_REACTION", "NORMAL", "EMOTIONAL_CHECK", "CARE", "CARE_OVERRIDE", "SHARED_JOY", "PLAYFUL", "FLIRT", "CONFLICT", "CONFLICT_SOFTEN", "SPACE", "FULL_ANSWER":
		return value
	default:
		return "NORMAL"
	}
}

func extractCascadeJSONStringField(raw, key string, from int) (string, int, bool, error) {
	start, ok := findCascadeJSONStringValueStart(raw, key, from)
	if !ok {
		return "", 0, false, nil
	}
	end, ok := findCascadeJSONStringEnd(raw, start)
	if !ok {
		return "", 0, false, nil
	}
	encoded := raw[start:end]
	value, err := strconv.Unquote("\"" + encoded + "\"")
	if err != nil {
		return "", 0, false, err
	}
	return value, end + 1, true, nil
}

func findCascadeJSONStringValueStart(raw, key string, from int) (int, bool) {
	if from < 0 {
		from = 0
	}
	needle := strconv.Quote(key)
	idx := strings.Index(raw[from:], needle)
	if idx < 0 {
		return 0, false
	}
	idx += from + len(needle)
	for idx < len(raw) && (raw[idx] == ' ' || raw[idx] == '\n' || raw[idx] == '\r' || raw[idx] == '\t') {
		idx++
	}
	if idx >= len(raw) || raw[idx] != ':' {
		return 0, false
	}
	idx++
	for idx < len(raw) && (raw[idx] == ' ' || raw[idx] == '\n' || raw[idx] == '\r' || raw[idx] == '\t') {
		idx++
	}
	if idx >= len(raw) || raw[idx] != '"' {
		return 0, false
	}
	return idx + 1, true
}

func findCascadeJSONStringEnd(raw string, start int) (int, bool) {
	escaped := false
	for i := start; i < len(raw); i++ {
		if escaped {
			escaped = false
			continue
		}
		if raw[i] == '\\' {
			escaped = true
			continue
		}
		if raw[i] == '"' {
			return i, true
		}
	}
	return 0, false
}

func decodeCascadeJSONStringFragment(input string) (string, int, bool, error) {
	var out strings.Builder
	for i := 0; i < len(input); {
		b := input[i]
		if b == '"' {
			return out.String(), i + 1, true, nil
		}
		if b != '\\' {
			if b < utf8.RuneSelf {
				out.WriteByte(b)
				i++
				continue
			}
			_, size := utf8.DecodeRuneInString(input[i:])
			if size == 1 && !utf8.FullRuneInString(input[i:]) {
				return out.String(), i, false, nil
			}
			out.WriteString(input[i : i+size])
			i += size
			continue
		}
		if i+1 >= len(input) {
			return out.String(), i, false, nil
		}
		switch input[i+1] {
		case '"', '\\', '/':
			out.WriteByte(input[i+1])
			i += 2
		case 'b':
			out.WriteByte('\b')
			i += 2
		case 'f':
			out.WriteByte('\f')
			i += 2
		case 'n':
			out.WriteByte('\n')
			i += 2
		case 'r':
			out.WriteByte('\r')
			i += 2
		case 't':
			out.WriteByte('\t')
			i += 2
		case 'u':
			if i+6 > len(input) {
				return out.String(), i, false, nil
			}
			first, err := decodeCascadeHexRune(input[i+2 : i+6])
			if err != nil {
				return "", i, false, err
			}
			consumed := 6
			if utf16.IsSurrogate(first) {
				if i+12 > len(input) {
					return out.String(), i, false, nil
				}
				if input[i+6:i+8] != "\\u" {
					return "", i, false, fmt.Errorf("invalid JSON surrogate pair")
				}
				second, err := decodeCascadeHexRune(input[i+8 : i+12])
				if err != nil {
					return "", i, false, err
				}
				combined := utf16.DecodeRune(first, second)
				if combined == utf8.RuneError {
					return "", i, false, fmt.Errorf("invalid JSON surrogate pair")
				}
				first = combined
				consumed = 12
			}
			out.WriteRune(first)
			i += consumed
		default:
			return "", i, false, fmt.Errorf("invalid JSON escape sequence")
		}
	}
	return out.String(), len(input), false, nil
}

func decodeCascadeHexRune(value string) (rune, error) {
	data, err := hex.DecodeString(value)
	if err != nil || len(data) != 2 {
		return 0, fmt.Errorf("invalid JSON unicode escape")
	}
	return rune(uint16(data[0])<<8 | uint16(data[1])), nil
}

type cascadeTTSTextBuffer struct {
	text    strings.Builder
	started bool
}

func (b *cascadeTTSTextBuffer) Push(fragment string) []string {
	if fragment == "" {
		return nil
	}
	b.text.WriteString(fragment)
	return b.take(false)
}

func (b *cascadeTTSTextBuffer) Flush() []string {
	return b.take(true)
}

func (b *cascadeTTSTextBuffer) take(force bool) []string {
	current := b.text.String()
	if current == "" {
		return nil
	}
	runes := []rune(current)
	chunkLimit := 24
	if !b.started {
		chunkLimit = 2
	}
	chunks := make([]string, 0, 2)
	start := 0
	for i, r := range runes {
		length := i - start + 1
		boundary := strings.ContainsRune("。！？；!?;\n", r) || (length >= 2 && strings.ContainsRune("，,、", r))
		if boundary || length >= chunkLimit {
			chunk := string(runes[start : i+1])
			if strings.TrimSpace(chunk) != "" {
				chunks = append(chunks, chunk)
			}
			start = i + 1
			b.started = true
			chunkLimit = 24
		}
	}
	if force && start < len(runes) {
		chunk := string(runes[start:])
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
		start = len(runes)
	}
	b.text.Reset()
	if start < len(runes) {
		b.text.WriteString(string(runes[start:]))
	}
	if len(chunks) > 0 {
		b.started = true
	}
	return chunks
}

func normalizeCascadeVoiceDirective(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "悄悄话")
	return strings.TrimSpace(strings.TrimLeft(value, "，,。；;：:、 \t\r\n"))
}

func cascadeContainsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
