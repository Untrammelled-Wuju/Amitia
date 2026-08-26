// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimeprofile

import (
	"errors"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	resolution, err := Resolve(ResolveInput{
		Args:             []string{"--runtime-profile=device-agent"},
		Env:              map[string]string{envKey: "cloud-core"},
		LegacyDeployMode: "desktop-local",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolution.Profile != ProfileDeviceAgent || resolution.Source != SourceCLI {
		t.Fatalf("Resolve() = %#v, want device-agent from CLI", resolution)
	}

	resolution, err = Resolve(ResolveInput{
		Env:              map[string]string{envKey: "cloud-core"},
		LegacyDeployMode: "desktop-local",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolution.Profile != ProfileCloudCore || resolution.Source != SourceEnv {
		t.Fatalf("Resolve() = %#v, want cloud-core from env", resolution)
	}
}

func TestResolveLegacyDeployModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want Profile
	}{
		{name: "desktop local", raw: "desktop-local", want: ProfileLocal},
		{name: "cloud web", raw: "cloud-web", want: ProfileCloudCore},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolution, err := Resolve(ResolveInput{Env: map[string]string{}, LegacyDeployMode: tc.raw})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if resolution.Profile != tc.want || resolution.Source != SourceLegacy {
				t.Fatalf("Resolve() = %#v, want %s from legacy", resolution, tc.want)
			}
		})
	}
}

func TestResolveDefaultsOnlyWhenUnconfigured(t *testing.T) {
	resolution, err := Resolve(ResolveInput{Env: map[string]string{}})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolution.Profile != ProfileLocal || resolution.Source != SourceDefault {
		t.Fatalf("Resolve() = %#v, want local default", resolution)
	}
}

func TestResolveRejectsExplicitInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		input ResolveInput
	}{
		{name: "invalid cli equals", input: ResolveInput{Args: []string{"--runtime-profile=cloud-corex"}, Env: map[string]string{}}},
		{name: "invalid cli separate", input: ResolveInput{Args: []string{"--runtime-profile", "cloud-corex"}, Env: map[string]string{}}},
		{name: "empty cli equals", input: ResolveInput{Args: []string{"--runtime-profile="}, Env: map[string]string{}}},
		{name: "missing cli value", input: ResolveInput{Args: []string{"--runtime-profile"}, Env: map[string]string{}}},
		{name: "invalid env", input: ResolveInput{Env: map[string]string{envKey: "cloud-corex"}}},
		{name: "empty explicit env", input: ResolveInput{Env: map[string]string{envKey: ""}}},
		{name: "invalid legacy", input: ResolveInput{Env: map[string]string{}, LegacyDeployMode: "cloud-corex"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Resolve(tc.input)
			if err == nil {
				t.Fatal("Resolve() unexpectedly succeeded")
			}
			var invalid *InvalidProfileError
			if !errors.As(err, &invalid) {
				t.Fatalf("Resolve() error = %T %v, want InvalidProfileError", err, err)
			}
		})
	}
}

func TestResolveRejectsDuplicateCLI(t *testing.T) {
	_, err := Resolve(ResolveInput{
		Args: []string{"--runtime-profile=local", "--runtime-profile", "cloud-core"},
		Env:  map[string]string{},
	})
	if err == nil {
		t.Fatal("Resolve() unexpectedly succeeded")
	}
	var duplicate *DuplicateProfileArgError
	if !errors.As(err, &duplicate) {
		t.Fatalf("Resolve() error = %T %v, want DuplicateProfileArgError", err, err)
	}
}

func TestParseIsStrictForEmptyInput(t *testing.T) {
	if _, err := Parse("  "); err == nil {
		t.Fatal("Parse() accepted an empty explicit profile")
	}
}

func TestResolveRejectsUnknownOrExtraCLIArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--runtime-profle=cloud-core"},
		{"--runtime-profile=cloud-core", "unexpected"},
	} {
		_, err := Resolve(ResolveInput{Args: args, Env: map[string]string{}})
		if err == nil {
			t.Fatalf("Resolve(%q) unexpectedly succeeded", args)
		}
		var unexpected *UnexpectedRuntimeArgError
		if !errors.As(err, &unexpected) {
			t.Fatalf("Resolve(%q) error = %T %v, want UnexpectedRuntimeArgError", args, err, err)
		}
	}
}
