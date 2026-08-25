package artifact

import "testing"

func TestVersionMatchesRequiresVersionWhenConstrained(t *testing.T) {
	if versionMatches([]string{"1.21.x"}, "") {
		t.Fatal("constrained artifact matched an empty compatibility version")
	}
	if !versionMatches(nil, "") {
		t.Fatal("unconstrained artifact should match an empty compatibility version")
	}
}

func TestVersionMatchesRangesAndOpaqueExact(t *testing.T) {
	tests := []struct {
		name        string
		constraints []string
		version     string
		want        bool
	}{
		{"exact", []string{"1.21.4"}, "1.21.4", true},
		{"opaque exact", []string{"build-2026-A"}, "BUILD-2026-a", true},
		{"wildcard", []string{"1.21.x"}, "1.21.7", true},
		{"wildcard reject other minor", []string{"1.21.*"}, "1.22.0", false},
		{"comparator range", []string{">=1.20.1 <1.22"}, "1.21.4", true},
		{"comparator upper bound", []string{">=1.20.1 <1.22"}, "1.22.0", false},
		{"caret", []string{"^1.20.1"}, "1.99.0", true},
		{"caret major bound", []string{"^1.20.1"}, "2.0.0", false},
		{"caret zero", []string{"^0.3.2"}, "0.4.0", false},
		{"tilde", []string{"~1.20.2"}, "1.20.99", true},
		{"tilde bound", []string{"~1.20.2"}, "1.21.0", false},
		{"hyphen", []string{"1.20.1 - 1.21.4"}, "1.21.4", true},
		{"or", []string{"<1.10 || >=2.0 <3.0"}, "2.6.1", true},
		{"prerelease lower", []string{">=1.2.0-beta.1 <1.2.0"}, "1.2.0-beta.2", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionMatches(tc.constraints, tc.version); got != tc.want {
				t.Fatalf("versionMatches(%v, %q) = %v, want %v", tc.constraints, tc.version, got, tc.want)
			}
		})
	}
}

func TestValidateCompatibilityConstraint(t *testing.T) {
	valid := []string{"*", "1.21.x", "1.20.x || 1.21.x", ">=1.20.1 <1.22", "^1.20.1", "~1.20", "1.20 - 1.21.4", "build-2026-a", "fabric-loader-0.16 || neoforge-21"}
	for _, value := range valid {
		if err := validateCompatibilityConstraint(value); err != nil {
			t.Fatalf("validateCompatibilityConstraint(%q) unexpected error: %v", value, err)
		}
	}
	invalid := []string{"", ">=", "1.x.3", ">=1.2 ||"}
	for _, value := range invalid {
		if err := validateCompatibilityConstraint(value); err == nil {
			t.Fatalf("validateCompatibilityConstraint(%q) expected error", value)
		}
	}
}
