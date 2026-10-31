package util

import (
	"errors"
	"testing"
)

func TestOriginalErrorMessagePreservesError(t *testing.T) {
	original := errors.New("upstream failed")
	if got := OriginalErrorMessage(original, "fallback"); got != original.Error() {
		t.Fatalf("got %q, want %q", got, original.Error())
	}
}

func TestOriginalErrorMessageUsesFallbackWhenErrorIsNil(t *testing.T) {
	if got := OriginalErrorMessage(nil, "validation failed"); got != "validation failed" {
		t.Fatalf("got %q, want fallback", got)
	}
}
