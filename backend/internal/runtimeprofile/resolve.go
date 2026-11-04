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
	SourceEnv     Source = "env"
	SourceCLI     Source = "cli"
)

type Resolution struct {
	Profile Profile
	Source  Source
}

type ResolveInput struct {
	Args []string
	Env  map[string]string
}

const envKey = "AMITIA_RUNTIME_PROFILE"
const cliFlag = "--runtime-profile"

func Resolve(input ResolveInput) (Resolution, error) {
	if profile, found, err := resolveCLI(input.Args); found || err != nil {
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{Profile: profile, Source: SourceCLI}, nil
	}

	if profile, found, err := resolveEnv(input.Env); found || err != nil {
		if err != nil {
			return Resolution{}, err
		}
		return Resolution{Profile: profile, Source: SourceEnv}, nil
	}

	return Resolution{Profile: Default(), Source: SourceDefault}, nil
}

func resolveCLI(args []string) (Profile, bool, error) {
	var raw string
	found := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		var value string
		matched := false

		switch {
		case arg == cliFlag:
			matched = true
			if i+1 >= len(args) {
				value = ""
			} else {
				i++
				value = args[i]
			}
		case strings.HasPrefix(arg, cliFlag+"="):
			matched = true
			value = strings.TrimPrefix(arg, cliFlag+"=")
		}

		if !matched {
			return "", found, &UnexpectedRuntimeArgError{Value: arg}
		}
		if found {
			return "", true, &DuplicateProfileArgError{Value: value}
		}
		found = true
		raw = value
	}

	if !found {
		return "", false, nil
	}

	profile, err := Parse(raw)
	if err != nil {
		return "", true, err
	}
	return profile, true, nil
}

func resolveEnv(env map[string]string) (Profile, bool, error) {
	var (
		raw     string
		present bool
	)
	if env != nil {
		raw, present = env[envKey]
	} else {
		raw, present = os.LookupEnv(envKey)
	}
	if !present {
		return "", false, nil
	}

	profile, err := Parse(raw)
	if err != nil {
		return "", true, err
	}
	return profile, true, nil
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
