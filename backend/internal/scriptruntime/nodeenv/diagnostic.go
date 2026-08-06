// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import "time"

type DetectionState string

const (
	DetectionStateNotStarted DetectionState = "not-started"
	DetectionStateReady      DetectionState = "ready"
	DetectionStatePartial    DetectionState = "partial"
	DetectionStateFailed     DetectionState = "failed"
)

type CandidateKind string

const (
	CandidateKindNode    CandidateKind = "node"
	CandidateKindNPMCLI  CandidateKind = "npm-cli"
	CandidateKindNPXCLI  CandidateKind = "npx-cli"
	CandidateKindWorkDir CandidateKind = "work-dir"
)

type CandidateResult string

const (
	CandidateResultSelected         CandidateResult = "selected"
	CandidateResultNotFound         CandidateResult = "not-found"
	CandidateResultInvalidFile      CandidateResult = "invalid-file"
	CandidateResultNotExecutable    CandidateResult = "not-executable"
	CandidateResultUnsupportedWrapper CandidateResult = "unsupported-wrapper"
	CandidateResultRootUnavailable  CandidateResult = "root-unavailable"
	CandidateResultSkipped          CandidateResult = "skipped"
)

type CandidateDiagnostic struct {
	Kind   CandidateKind
	Source Source
	Path   string
	Result CandidateResult
	Error  string
}

type DetectionSnapshot struct {
	State      DetectionState
	Environment Environment
	Diagnostics []CandidateDiagnostic
	LastError  string
	DetectedAt time.Time
}

func (s DetectionSnapshot) clone() DetectionSnapshot {
	diags := make([]CandidateDiagnostic, len(s.Diagnostics))
	copy(diags, s.Diagnostics)
	return DetectionSnapshot{
		State:       s.State,
		Environment: s.Environment.Clone(),
		Diagnostics: diags,
		LastError:   s.LastError,
		DetectedAt:  s.DetectedAt,
	}
}
