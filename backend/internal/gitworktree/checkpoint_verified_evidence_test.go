package gitworktree

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestVerifiedTestEvidenceCompleteFailClosed(t *testing.T) {
	base := VerifiedTest{
		Profile: "go", Revision: "revision", StartedAt: time.Now().UTC(),
		DurationMillis: 12, ExitCode: 0, Passed: true, OutputDigest: strings.Repeat("a", 64),
	}
	if !VerifiedTestEvidenceComplete(base, "revision") {
		t.Fatal("valid verified evidence rejected")
	}
	cases := map[string]func(*VerifiedTest){
		"missing digest":      func(e *VerifiedTest) { e.OutputDigest = "" },
		"short digest":        func(e *VerifiedTest) { e.OutputDigest = "abcd" },
		"non-hex digest":      func(e *VerifiedTest) { e.OutputDigest = strings.Repeat("z", 64) },
		"missing start":       func(e *VerifiedTest) { e.StartedAt = time.Time{} },
		"negative duration":   func(e *VerifiedTest) { e.DurationMillis = -1 },
		"failed exit":         func(e *VerifiedTest) { e.ExitCode = 1 },
		"unsupported profile": func(e *VerifiedTest) { e.Profile = "custom" },
		"wrong revision":      func(e *VerifiedTest) { e.Revision = "other" },
		"empty revision":      func(e *VerifiedTest) { e.Revision = "" },
		"not passed":          func(e *VerifiedTest) { e.Passed = false },
		"truncated":           func(e *VerifiedTest) { e.OutputTruncated = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := base
			mutate(&e)
			if VerifiedTestEvidenceComplete(e, "revision") {
				t.Fatalf("invalid evidence accepted: %+v", e)
			}
		})
	}
}

func TestForgedTestPassedFlagCannotApproveOrMerge(t *testing.T) {
	ctx := context.Background()
	m, root, entry, revision := approvedMergeFixture(t)
	_, key, err := m.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	record, err := m.readCheckpoint(key, entry.ID)
	if err != nil || record == nil || len(record.Tests) == 0 {
		t.Fatalf("fixture checkpoint missing: %+v %v", record, err)
	}
	good := record.Tests[len(record.Tests)-1]
	if !VerifiedTestEvidenceComplete(good, revision) {
		t.Fatalf("fixture evidence unexpectedly incomplete: %+v", good)
	}
	for name, forge := range map[string]func(*VerifiedTest){
		"digest erased":  func(e *VerifiedTest) { e.OutputDigest = "" },
		"nonzero exit":   func(e *VerifiedTest) { e.ExitCode = 7 },
		"runner altered": func(e *VerifiedTest) { e.Profile = "external" },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := *record
			invalid.ApprovedAt = nil
			invalid.ApprovedBy = ""
			invalid.Tests = append([]VerifiedTest(nil), record.Tests...)
			last := &invalid.Tests[len(invalid.Tests)-1]
			forge(last)
			if err := m.writeCheckpoint(key, entry.ID, invalid); err != nil {
				t.Fatal(err)
			}
			fresh, err := New(m.dataRoot)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := fresh.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", revision, "local-owner"); !errors.Is(err, ErrTestRequired) {
				t.Fatalf("forged test evidence approved: %v", err)
			}
			when := time.Now().UTC()
			invalid.ApprovedAt = &when
			invalid.ApprovedBy = "local-owner"
			if err := m.writeCheckpoint(key, entry.ID, invalid); err != nil {
				t.Fatal(err)
			}
			if _, err := fresh.PreviewApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrTestRequired) {
				t.Fatalf("forged approval allowed merge preview: %v", err)
			}
		})
	}
	if err := m.writeCheckpoint(key, entry.ID, *record); err != nil {
		t.Fatal(err)
	}
	if _, err := m.PreviewApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil {
		t.Fatalf("original verified approval did not recover: %v", err)
	}
}
