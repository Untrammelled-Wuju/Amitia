package workbenchterminal

import (
	"errors"
	"strconv"
	"testing"
)

func TestStreamCursorAcceptsUnsignedDecimalOnly(t *testing.T) {
	valid := map[string]int64{"0": 0, "7": 7, "0007": 7, "9223372036854775807": 9223372036854775807}
	for input, expected := range valid {
		actual, err := parseTerminalStreamCursor(input)
		if err != nil || actual != expected {
			t.Fatalf("%q: cursor=%d err=%v", input, actual, err)
		}
	}
	for _, input := range []string{"", "-1", "+1", " 1", "1 ", "1.0", "0x10", "1\n", "9223372036854775808", "999999999999999999999", strconv.FormatUint(^uint64(0), 10)} {
		if _, err := parseTerminalStreamCursor(input); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid cursor %q accepted: %v", input, err)
		}
	}
}
