// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
)

func TestParseIdentity(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    Identity
		wantErr bool
	}{
		{
			name: "valid qdrant response",
			body: `{"title":"qdrant","version":"1.15.0"}`,
			want: Identity{Title: "qdrant", Version: "1.15.0"},
		},
		{
			name: "valid mixed case",
			body: `{"title":"Qdrant","version":"1.14.0"}`,
			want: Identity{Title: "Qdrant", Version: "1.14.0"},
		},
		{
			name:    "invalid json",
			body:    `not json`,
			wantErr: true,
		},
		{
			name: "empty fields",
			body: `{"title":"","version":""}`,
			want: Identity{Title: "", Version: ""},
		},
		{
			name: "whitespace fields",
			body: `{"title":"  qdrant  ","version":" 1.0 "}`,
			want: Identity{Title: "qdrant", Version: "1.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIdentity([]byte(tt.body))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseIdentity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Title != tt.want.Title {
					t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
				}
				if got.Version != tt.want.Version {
					t.Errorf("Version = %q, want %q", got.Version, tt.want.Version)
				}
			}
		})
	}
}

func TestIdentityValidate(t *testing.T) {
	tests := []struct {
		name    string
		id      Identity
		wantErr error
	}{
		{
			name:    "not confirmed",
			id:      Identity{Title: "qdrant", Version: "1.0"},
			wantErr: ErrIdentityRequired,
		},
		{
			name:    "confirmed valid",
			id:      Identity{Title: "qdrant", Version: "1.15.0", Confirmed: true},
			wantErr: nil,
		},
		{
			name:    "confirmed title mismatch",
			id:      Identity{Title: "not-qdrant", Version: "1.15.0", Confirmed: true},
			wantErr: ErrIdentityTitleMismatch,
		},
		{
			name:    "confirmed no version",
			id:      Identity{Title: "qdrant", Version: "", Confirmed: true},
			wantErr: ErrIdentityVersionRequired,
		},
		{
			name:    "confirmed case insensitive",
			id:      Identity{Title: "QDRANT", Version: "1.0", Confirmed: true},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.id.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIdentityMatches(t *testing.T) {
	id1 := Identity{Title: "qdrant", Version: "1.15.0", Confirmed: true}
	id2 := Identity{Title: "qdrant", Version: "1.15.0", Confirmed: true}
	id3 := Identity{Title: "qdrant", Version: "1.14.0", Confirmed: true}
	id4 := Identity{Title: "not-qdrant", Version: "1.15.0", Confirmed: true}
	id5 := Identity{Title: "qdrant", Version: "1.15.0", Confirmed: false}

	if !id1.Matches(id2) {
		t.Error("expected identical identities to match")
	}
	if id1.Matches(id3) {
		t.Error("expected different versions not to match")
	}
	if id1.Matches(id4) {
		t.Error("expected different titles not to match")
	}
	if id1.Matches(id5) {
		t.Error("expected unconfirmed identity not to match")
	}
}

func TestIdentityString(t *testing.T) {
	unconfirmed := Identity{}
	if unconfirmed.String() != "identity:unconfirmed" {
		t.Errorf("unconfirmed identity string = %q", unconfirmed.String())
	}

	confirmed := Identity{Title: "qdrant", Version: "1.15.0", Confirmed: true}
	if confirmed.String() != "identity:qdrant@1.15.0" {
		t.Errorf("confirmed identity string = %q", confirmed.String())
	}
}

func TestProcessIdentityIsEmpty(t *testing.T) {
	pi := ProcessIdentity{}
	if !pi.IsEmpty() {
		t.Error("expected empty ProcessIdentity")
	}

	pi.Identity.Title = "qdrant"
	if pi.IsEmpty() {
		t.Error("expected non-empty ProcessIdentity with title")
	}
}

func TestIdentityVerifier(t *testing.T) {
	v := NewIdentityVerifier()

	valid := Identity{Title: "qdrant", Version: "1.15.0", Confirmed: true}
	if err := v.Verify(valid); err != nil {
		t.Errorf("Verify returned error: %v", err)
	}

	invalid := Identity{Title: "wrong", Version: "1.15.0", Confirmed: true}
	if err := v.Verify(invalid); err != ErrIdentityTitleMismatch {
		t.Errorf("Verify should return ErrIdentityTitleMismatch, got %v", err)
	}
}

func TestIdentityVerifierExpected(t *testing.T) {
	v := NewIdentityVerifier()

	got := Identity{Title: "qdrant", Version: "1.15.0", Confirmed: true}
	expected := Identity{Title: "qdrant", Version: "1.15.0", Confirmed: true}

	if err := v.VerifyExpected(got, expected); err != nil {
		t.Errorf("VerifyExpected returned error: %v", err)
	}

	different := Identity{Title: "qdrant", Version: "1.14.0", Confirmed: true}
	if err := v.VerifyExpected(different, expected); err != ErrIdentityTitleMismatch {
		t.Errorf("VerifyExpected should return mismatched error, got %v", err)
	}
}
