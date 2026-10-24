package mindruntime

import "testing"

func TestRound78TrustedTokenSettlement(t *testing.T) {
	cases := []struct {
		reported                     bool
		input, output, cached, total int64
		valid                        bool
	}{
		{true, 12, 5, 2, 17, true},
		{false, 12, 5, 0, 17, false},
		{true, 12, 5, 0, 0, false},
		{true, 12, 5, 0, 13, false},
		{true, -1, 0, 0, 10, false},
		{true, 0, -1, 0, 10, false},
		{true, 0, 0, -1, 10, false},
	}
	for _, tc := range cases {
		if result := trustworthyOwnedTokenUsage(tc.reported, tc.input, tc.output, tc.cached, tc.total); result != tc.valid {
			t.Fatalf("untrusted usage passed: %+v", tc)
		}
	}
}
