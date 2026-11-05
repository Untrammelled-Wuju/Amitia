package main

import "testing"

func TestNormalizeRuntimeListenPort(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		expected int
	}{
		{name: "dynamic sentinel", port: 1, expected: 0},
		{name: "explicit port", port: 18081, expected: 18081},
		{name: "dynamic port", port: 0, expected: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := normalizeRuntimeListenPort(test.port); actual != test.expected {
				t.Fatalf("normalizeRuntimeListenPort(%d) = %d, want %d", test.port, actual, test.expected)
			}
		})
	}
}
