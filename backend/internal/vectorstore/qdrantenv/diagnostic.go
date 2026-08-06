// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantenv

import "time"

type DetectionState string

const (
	DetectionStateNotStarted   DetectionState = "not-started"
	DetectionStateReady        DetectionState = "ready"
	DetectionStateNotInstalled DetectionState = "not-installed"
	DetectionStateFailed       DetectionState = "failed"
)

type CandidateResult string

const (
	CandidateResultSelected                CandidateResult = "selected"
	CandidateResultNotFound                CandidateResult = "not-found"
	CandidateResultInvalidFile             CandidateResult = "invalid-file"
	CandidateResultNotExecutable           CandidateResult = "not-executable"
	CandidateResultResourceRootUnavailable CandidateResult = "resource-root-unavailable"
	CandidateResultSkipped                 CandidateResult = "skipped"
	CandidateResultInstallTarget           CandidateResult = "install-target"
)

type CandidateDiagnostic struct {
	Source Source
	Path   string
	Result CandidateResult
	Error  string
}

type DetectionSnapshot struct {
	State       DetectionState
	Environment Environment
	Diagnostics []CandidateDiagnostic
	LastError   string
	DetectedAt  time.Time
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
