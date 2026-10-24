package chat

import "testing"

func TestRound78ConservativeProviderOutputLimits(t *testing.T) {
	cases := []struct {
		configured, legacy int
		anthropic          bool
		thinking           int
		want               int64
	}{
		{64, 128, false, 0, 128},
		{128, 64, false, 0, 128},
		{0, 0, false, 0, 4096},
		{64, 64, true, 8192, 10240},
		{12000, 12000, true, 8192, 12000},
	}
	for _, tc := range cases {
		got := conservativeModelOutputCeiling(tc.configured, tc.legacy, tc.anthropic, tc.thinking)
		if got != tc.want {
			t.Fatalf("output ceiling %d want %d", got, tc.want)
		}
	}
}
