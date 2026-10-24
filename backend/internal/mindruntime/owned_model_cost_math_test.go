package mindruntime

import "testing"

func TestRound79CostCeilingIntegerArithmetic(t *testing.T) {
	cases := []struct {
		input, output, inRate, outRate, expected int64
		ok                                       bool
	}{
		{1000000, 500000, 1000000, 2000000, 2000000, true},
		{1, 1, 1, 1, 1, true},
		{100000000, 100000000, 100000000, 100000000, 20000000000, true},
		{0, 1, 0, 1, 0, false},
		{-1, 1, 1, 1, 0, false},
		{100000001, 1, 1, 1, 0, false},
	}
	for _, tc := range cases {
		got, ok := CeilingOwnedModelCost(tc.input, tc.output, tc.inRate, tc.outRate)
		if got != tc.expected || ok != tc.ok {
			t.Fatalf("%+v got %d %v", tc, got, ok)
		}
	}
}
