package gitworktree

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMergeAuditRejectsTamperedFingerprintAndContradictoryState(t *testing.T) {
	ctx := context.Background()
	m, root, entry, _ := approvedMergeFixture(t)
	plan, err := m.PreviewApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	valid := MergeAudit{Version: 1, Plan: *plan, Actor: plan.ApprovedBy, StartedAt: time.Now().UTC(), Result: "pending"}
	cases := map[string]func(*MergeAudit){
		"invalid plan fingerprint": func(a *MergeAudit) { a.Plan.PlanID = strings.Repeat("0", 64) },
		"target identity changed":  func(a *MergeAudit) { a.Plan.TargetBranch = "another" },
		"task identity changed":    func(a *MergeAudit) { a.Plan.TaskID = "other" },
		"actor changed":            func(a *MergeAudit) { a.Actor = "another-user" },
		"invented applied":         func(a *MergeAudit) { a.Result = "applied"; a.AppliedHead = a.Plan.SourceHead },
		"unknown result":           func(a *MergeAudit) { a.Result = "succeeded" },
		"pending with completion":  func(a *MergeAudit) { now := time.Now().UTC(); a.FinishedAt = &now },
		"applied with wrong head": func(a *MergeAudit) {
			now := time.Now().UTC()
			a.FinishedAt = &now
			a.Result = "applied"
			a.AppliedHead = a.Plan.TargetHead
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			audit := valid
			corrupt(&audit)
			if err := m.writeMergeAudit(key, entry.ID, audit); err != nil {
				t.Fatal(err)
			}
			fresh, err := New(m.dataRoot)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := fresh.ReadMergeAudit(ctx, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("tampered audit accepted: %v", err)
			}
		})
	}
	if err := m.writeMergeAudit(key, entry.ID, valid); err != nil {
		t.Fatal(err)
	}
	fresh, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	got, err := fresh.ReadMergeAudit(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || got.Result != "pending" {
		t.Fatalf("valid pending audit not readable: %+v %v", got, err)
	}
}
