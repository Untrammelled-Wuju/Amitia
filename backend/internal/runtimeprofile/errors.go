// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimeprofile

import "fmt"

type InvalidProfileError struct {
	Raw string
}

func (e *InvalidProfileError) Error() string {
	return fmt.Sprintf("invalid runtime profile: %q", e.Raw)
}

type DuplicateProfileArgError struct {
	Value string
}

func (e *DuplicateProfileArgError) Error() string {
	return fmt.Sprintf("duplicate runtime profile argument: %q", e.Value)
}

type UnexpectedRuntimeArgError struct {
	Value string
}

func (e *UnexpectedRuntimeArgError) Error() string {
	return fmt.Sprintf("unexpected runtime argument: %q", e.Value)
}

type ProfileSecurityConflictError struct {
	Profile Profile
	Detail  string
}

func (e *ProfileSecurityConflictError) Error() string {
	return fmt.Sprintf("profile %s security conflict: %s", e.Profile, e.Detail)
}
