package decision

import (
	"testing"
	"time"
)

func TestDeriveIntentionFromGoal(t *testing.T) {
	goal := Goal{
		ID:          "goal-10",
		Type:        GoalTypeConnection,
		Priority:    GoalPriorityHigh,
		Description: "与用户建立深层次连接",
		UserID:      "user-1",
		CharacterID: "char-1",
	}
	deadline := time.Now().UTC().Add(12 * time.Hour)
	intention := DeriveIntention(goal, CommitmentStrong, deadline)
	if intention.GoalID != "goal-10" {
		t.Fatalf("GoalID 不匹配: %s", intention.GoalID)
	}
	if intention.GoalType != GoalTypeConnection {
		t.Fatalf("GoalType 不匹配: %s", intention.GoalType)
	}
	if intention.Commitment != CommitmentStrong {
		t.Fatalf("Commitment 不匹配: %s", intention.Commitment)
	}
	if intention.Status != IntentionStatusFormed {
		t.Fatalf("Status 应为 formed, 实际 %s", intention.Status)
	}
	if !intention.Deadline.Equal(deadline) {
		t.Fatalf("Deadline 不匹配: %v vs %v", intention.Deadline, deadline)
	}
}

func TestDeriveIntentionDefaultCommitment(t *testing.T) {
	criticalGoal := Goal{ID: "g-crit", Priority: GoalPriorityCritical}
	highGoal := Goal{ID: "g-high", Priority: GoalPriorityHigh}
	normalGoal := Goal{ID: "g-normal", Priority: GoalPriorityNormal}
	lowGoal := Goal{ID: "g-low", Priority: GoalPriorityLow}
	ci := DeriveIntention(criticalGoal, "", time.Time{})
	if ci.Commitment != CommitmentAbsolute {
		t.Fatal("Critical 应派生 Absolute")
	}
	hi := DeriveIntention(highGoal, "", time.Time{})
	if hi.Commitment != CommitmentStrong {
		t.Fatal("High 应派生 Strong")
	}
	ni := DeriveIntention(normalGoal, "", time.Time{})
	if ni.Commitment != CommitmentModerate {
		t.Fatal("Normal 应派生 Moderate")
	}
	li := DeriveIntention(lowGoal, "", time.Time{})
	if li.Commitment != CommitmentWeak {
		t.Fatal("Low 应派生 Weak")
	}
}

func TestIntentionIsExpired(t *testing.T) {
	now := time.Now().UTC()
	intention := Intention{Deadline: now.Add(-1 * time.Hour)}
	if !intention.IsExpired(now) {
		t.Fatal("已过期的 intention 应返回 true")
	}
	intention.Deadline = now.Add(1 * time.Hour)
	if intention.IsExpired(now) {
		t.Fatal("未过期的 intention 应返回 false")
	}
}

func TestIntentionIsOverdue(t *testing.T) {
	now := time.Now().UTC()
	intention := Intention{Deadline: now.Add(-30 * time.Minute)}
	if intention.IsOverdue(now, 1*time.Hour) {
		t.Fatal("在宽限期内不应视为 over due")
	}
	if !intention.IsOverdue(now, 0) {
		t.Fatal("无宽限期时应返回 true")
	}
}

func TestIntentionCommitmentValue(t *testing.T) {
	ia := Intention{Commitment: CommitmentAbsolute}
	if ia.CommitmentValue() != 1.0 {
		t.Fatal("Absolute = 1.0")
	}
	ib := Intention{Commitment: CommitmentStrong}
	if ib.CommitmentValue() != 0.80 {
		t.Fatal("Strong = 0.80")
	}
	ic := Intention{Commitment: CommitmentModerate}
	if ic.CommitmentValue() != 0.50 {
		t.Fatal("Moderate = 0.50")
	}
	id := Intention{Commitment: CommitmentWeak}
	if id.CommitmentValue() != 0.25 {
		t.Fatal("Weak = 0.25")
	}
}

func TestIntentionUrgency(t *testing.T) {
	now := time.Now().UTC()
	nearDeadline := Intention{
		Commitment: CommitmentStrong,
		Deadline:   now.Add(1 * time.Hour),
	}
	urgency := nearDeadline.Urgency(now)
	if urgency < 0.9 || urgency > 1.01 {
		t.Fatalf("即将到期的 urgency 应接近 1.0, 实际 %f", urgency)
	}
	farDeadline := Intention{
		Commitment: CommitmentStrong,
		Deadline:   now.Add(100 * time.Hour),
	}
	urgencyFar := farDeadline.Urgency(now)
	if urgencyFar >= urgency {
		t.Fatalf("远期 deadline urgency 应低于近期: %f vs %f", urgencyFar, urgency)
	}
	noDeadline := Intention{Commitment: CommitmentModerate}
	urgencyNo := noDeadline.Urgency(now)
	if urgencyNo != 0.50 {
		t.Fatalf("无 deadline 时 urgency=commitment value=0.50, 实际 %f", urgencyNo)
	}
}
