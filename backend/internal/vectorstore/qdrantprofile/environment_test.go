// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprofile

import (
	"testing"
)

func TestSanitizer_KeepsNormalVariables(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"HOME=/root", "PATH=/usr/bin", "LANG=en_US.UTF-8"}
	out := s.Sanitize(input)
	if len(out) != 3 {
		t.Errorf("expected 3 entries, got %d", len(out))
	}
}

func TestSanitizer_RemovesQDRANTServiceHost(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"HOME=/root", "QDRANT__SERVICE__HOST=0.0.0.0", "PATH=/usr/bin"}
	out := s.Sanitize(input)
	for _, e := range out {
		if e == "QDRANT__SERVICE__HOST=0.0.0.0" {
			t.Error("QDRANT__SERVICE__HOST should be removed")
		}
	}
	if len(out) != 2 {
		t.Errorf("expected 2 entries, got %d", len(out))
	}
}

func TestSanitizer_RemovesQDRANTSearchThreads(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"QDRANT__STORAGE__PERFORMANCE__MAX_SEARCH_THREADS=8"}
	out := s.Sanitize(input)
	if len(out) != 0 {
		t.Errorf("expected 0 entries, got %d", len(out))
	}
}

func TestSanitizer_RemovesQDRANTResourceProfile(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"QDRANT_RESOURCE_PROFILE=mobile-compact"}
	out := s.Sanitize(input)
	if len(out) != 0 {
		t.Errorf("expected 0 entries, got %d", len(out))
	}
}

func TestSanitizer_CaseInsensitive(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"qdrant__service__host=0.0.0.0", "Qdrant__Storage__Wal=64"}
	out := s.Sanitize(input)
	if len(out) != 0 {
		t.Error("sanitizer should remove case variants of QDRANT__ prefix")
	}
}

func TestSanitizer_ValueWithEquals(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"QDRANT__SERVICE__HOST=0.0.0.0=extra"}
	out := s.Sanitize(input)
	if len(out) != 0 {
		t.Error("key with QDRANT__ prefix should be removed even with = in value")
	}
}

func TestSanitizer_InvalidNoEquals(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"NOEQUALS"}
	out := s.Sanitize(input)
	if len(out) != 0 {
		t.Error("entries without = should be discarded")
	}
}

func TestSanitizer_NULCharacter(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"NUL\x00ENTRY=value"}
	out := s.Sanitize(input)
	if len(out) != 0 {
		t.Error("entries with NUL should be discarded")
	}
}

func TestSanitizer_DoesNotModifyInput(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"HOME=/root", "QDRANT__SERVICE__HOST=0.0.0.0"}
	orig := make([]string, len(input))
	copy(orig, input)
	s.Sanitize(input)
	for i := range input {
		if input[i] != orig[i] {
			t.Error("input slice should not be modified")
		}
	}
}

func TestSanitizer_OutputOrderStable(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"A=1", "QDRANT__X=1", "B=2", "QDRANT__Y=2", "C=3"}
	out := s.Sanitize(input)
	if len(out) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(out))
	}
	if out[0] != "A=1" || out[1] != "B=2" || out[2] != "C=3" {
		t.Errorf("order not stable: %v", out)
	}
}

func TestSanitizer_EmptyInput(t *testing.T) {
	s := NewEnvironmentSanitizer()
	out := s.Sanitize(nil)
	if out != nil {
		t.Errorf("expected nil for nil input, got %v", out)
	}
}

func TestSanitizer_DuplicateKeys(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"HOME=/a", "HOME=/b"}
	out := s.Sanitize(input)
	if len(out) != 2 {
		t.Errorf("duplicate non-QDRANT keys should be kept, got %d", len(out))
	}
}

func TestSanitizer_ConcurrentSafe(t *testing.T) {
	s := NewEnvironmentSanitizer()
	input := []string{"A=1", "B=2", "QDRANT__X=1"}
	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_ = s.Sanitize(input)
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
