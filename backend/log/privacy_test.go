// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package log

import (
	"testing"
)

func TestMaskSensitive_EmptyString(t *testing.T) {
	result := MaskSensitive("")
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestMaskSensitive_APIToken(t *testing.T) {
	input := "apiKey=sk-abc123def456ghi789"
	result := MaskSensitive(input)
	if result == "apiKey=sk-abc123def456ghi789" {
		t.Error("api key should be masked")
	}
}

func TestMaskSensitive_BearerToken(t *testing.T) {
	input := "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	result := MaskSensitive(input)
	if result == input {
		t.Error("bearer token should be masked")
	}
}

func TestMaskSensitive_SkPrefix(t *testing.T) {
	input := "sk-proj-abcdefghijklmnop123456"
	result := MaskSensitive(input)
	if result == input {
		t.Error("sk- prefix should be masked")
	}
}

func TestMaskSensitive_PhoneNumber(t *testing.T) {
	input := "13812345678"
	result := MaskSensitive(input)
	if result == "13812345678" {
		t.Error("phone number should be masked")
	}
}

func TestMaskSensitive_IDCard(t *testing.T) {
	input := "110101199001011234"
	result := MaskSensitive(input)
	if result == "110101199001011234" {
		t.Error("ID card should be masked")
	}
}

func TestMaskSensitive_NoSensitiveContent(t *testing.T) {
	input := "这是一条普通的日志消息"
	result := MaskSensitive(input)
	if result != input {
		t.Errorf("non-sensitive content should be unchanged: got %s", result)
	}
}
