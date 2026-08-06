// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"encoding/json"
	"strings"
	"time"
)

const ExpectedIdentityTitle = "qdrant"

type IdentityResponse struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type Identity struct {
	Title     string
	Version   string
	Confirmed bool
	ConfirmedAt time.Time
}

func ParseIdentity(body []byte) (Identity, error) {
	var resp IdentityResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return Identity{}, err
	}
	return Identity{
		Title:   strings.TrimSpace(resp.Title),
		Version: strings.TrimSpace(resp.Version),
	}, nil
}

func (id Identity) Validate() error {
	if !id.Confirmed {
		return ErrIdentityRequired
	}
	if id.Title == "" || !strings.EqualFold(id.Title, ExpectedIdentityTitle) {
		return ErrIdentityTitleMismatch
	}
	if id.Version == "" {
		return ErrIdentityVersionRequired
	}
	return nil
}

func (id Identity) Matches(other Identity) bool {
	if !id.Confirmed || !other.Confirmed {
		return false
	}
	return strings.EqualFold(id.Title, other.Title) && id.Version == other.Version
}

func (id Identity) String() string {
	if !id.Confirmed {
		return "identity:unconfirmed"
	}
	return "identity:" + id.Title + "@" + id.Version
}

type ProcessIdentity struct {
	Identity
	ContainerID string
	ProcessID   int
}

func (pi ProcessIdentity) IsEmpty() bool {
	return pi.Identity.Title == "" && pi.ContainerID == "" && pi.ProcessID == 0
}

func NewIdentityVerifier() *IdentityVerifier {
	return &IdentityVerifier{}
}

type IdentityVerifier struct{}

func (v *IdentityVerifier) Verify(id Identity) error {
	return id.Validate()
}

func (v *IdentityVerifier) VerifyExpected(got, expected Identity) error {
	if err := got.Validate(); err != nil {
		return err
	}
	if !got.Matches(expected) {
		return ErrIdentityTitleMismatch
	}
	return nil
}
