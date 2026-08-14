// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimeprofile

import (
	"fmt"
	"os"
	"strings"
)

type Source string

const (
	SourceDefault Source = "default"
	SourceLegacy  Source = "legacy"
	SourceEnv     Source = "env"
	SourceCLI     Source = "cli"
)

type Resolution struct {
	Profile Profile
	Source  Source
}

type ResolveInput struct {
	Args             []string
	Env              map[string]string
	LegacyDeployMode string
}

const envKey = "AMITIA_RUNTIME_PROFILE"
const cliFlag = "--runtime-profile"

func Resolve(input ResolveInput) (Resolution, error) {
	if profile, src, found := resolveCLI(input.Args); found {
		if err := validateUniqueCLI(input.Args, profile); err != nil {
			return Resolution{}, err
		}
		return Resolution{Profile: profile, Source: src}, nil
	}

	if profile, found := resolveEnv(input.Env); found {
		return Resolution{Profile: profile, Source: SourceEnv}, nil
	}

	if profile, found := resolveLegacy(input.LegacyDeployMode); found {
		return Resolution{Profile: profile, Source: SourceLegacy}, nil
	}

	return Resolution{Profile: Default(), Source: SourceDefault}, nil
}

func resolveCLI(args []string) (Profile, Source, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == cliFlag {
			if i+1 >= len(args) {
				return "", "", false
			}
			p, err := Parse(args[i+1])
			if err != nil {
				return "", "", false
			}
			return p, SourceCLI, true
		}
		if strings.HasPrefix(arg, cliFlag+"=") {
			val := strings.TrimPrefix(arg, cliFlag+"=")
			p, err := Parse(val)
			if err != nil {
				return "", "", false
			}
			return p, SourceCLI, true
		}
	}
	return "", "", false
}

func validateUniqueCLI(args []string, first Profile) error {
	found := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var val string
		if arg == cliFlag {
			if i+1 >= len(args) {
				continue
			}
			val = args[i+1]
		} else if strings.HasPrefix(arg, cliFlag+"=") {
			val = strings.TrimPrefix(arg, cliFlag+"=")
		} else {
			continue
		}
		if val == "" {
			continue
		}
		if found {
			return &DuplicateProfileArgError{Value: val}
		}
		found = true
	}
	return nil
}

func resolveEnv(env map[string]string) (Profile, bool) {
	var raw string
	if env != nil {
		raw = env[envKey]
	} else {
		raw = os.Getenv(envKey)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	p, err := Parse(raw)
	if err != nil {
		return "", false
	}
	return p, true
}

func resolveLegacy(legacyDeployMode string) (Profile, bool) {
	p, err := Parse(legacyDeployMode)
	if err != nil {
		return "", false
	}
	return p, true
}

func ResolveOrExit(input ResolveInput) Resolution {
	res, err := Resolve(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if !res.Profile.IsValid() {
		fmt.Fprintf(os.Stderr, "error: invalid runtime profile: %q\n", string(res.Profile))
		os.Exit(1)
	}
	return res
}
