package decision

import "time"

type CommitmentStrength string

const (
	CommitmentWeak     CommitmentStrength = "weak"
	CommitmentModerate CommitmentStrength = "moderate"
	CommitmentStrong   CommitmentStrength = "strong"
	CommitmentAbsolute CommitmentStrength = "absolute"
)

type IntentionStatus string

const (
	IntentionStatusFormed    IntentionStatus = "formed"
	IntentionStatusExecuting IntentionStatus = "executing"
	IntentionStatusSuspended IntentionStatus = "suspended"
	IntentionStatusCompleted IntentionStatus = "completed"
	IntentionStatusAbandoned IntentionStatus = "abandoned"
)

type IntentionPlan struct {
	Steps      []string `json:"steps,omitempty"`
	Strategy   string   `json:"strategy,omitempty"`
	ExpectedID string   `json:"expectedId,omitempty"`
}

type Intention struct {
	ID          string             `json:"id"`
	GoalID      string             `json:"goalId"`
	GoalType    GoalType           `json:"goalType"`
	GoalDesc    string             `json:"goalDesc,omitempty"`
	UserID      string             `json:"userId,omitempty"`
	CharacterID string             `json:"characterId,omitempty"`
	Commitment  CommitmentStrength `json:"commitment"`
	Status      IntentionStatus    `json:"status"`
	Deadline    time.Time          `json:"deadline,omitempty"`
	Plan        IntentionPlan      `json:"plan"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

func DeriveIntention(goal Goal, commitment CommitmentStrength, deadline time.Time) Intention {
	return DeriveIntentionAt(goal, commitment, deadline, time.Now().UTC())
}

func DeriveIntentionAt(goal Goal, commitment CommitmentStrength, deadline time.Time, now time.Time) Intention {
	strength := commitment
	if strength == "" {
		strength = deriveCommitmentFromPriority(goal.Priority)
	}
	dl := deadline
	if dl.IsZero() {
		if !goal.ExpiresAt.IsZero() {
			dl = goal.ExpiresAt
		} else {
			dl = now.Add(24 * time.Hour)
		}
	}
	return Intention{
		ID:          "intention-" + goal.ID,
		GoalID:      goal.ID,
		GoalType:    goal.Type,
		GoalDesc:    goal.Description,
		UserID:      goal.UserID,
		CharacterID: goal.CharacterID,
		Commitment:  strength,
		Status:      IntentionStatusFormed,
		Deadline:    dl,
		Plan:        IntentionPlan{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (i *Intention) IsExpired(now time.Time) bool {
	return !i.Deadline.IsZero() && now.After(i.Deadline)
}

func (i *Intention) IsOverdue(now time.Time, grace time.Duration) bool {
	return !i.Deadline.IsZero() && now.After(i.Deadline.Add(grace))
}

func (i *Intention) CommitmentValue() float64 {
	switch i.Commitment {
	case CommitmentAbsolute:
		return 1.0
	case CommitmentStrong:
		return 0.80
	case CommitmentModerate:
		return 0.50
	case CommitmentWeak:
		return 0.25
	default:
		return 0.30
	}
}

func (i *Intention) Urgency(now time.Time) float64 {
	if i.Deadline.IsZero() {
		return i.CommitmentValue()
	}
	remaining := i.Deadline.Sub(now)
	if remaining <= 0 {
		return 1.0
	}
	urgency := 1.0 - (remaining.Hours() / 48.0)
	if urgency < 0 {
		urgency = 0
	}
	if urgency > 1 {
		urgency = 1
	}
	return urgency
}

func deriveCommitmentFromPriority(p GoalPriority) CommitmentStrength {
	switch p {
	case GoalPriorityCritical:
		return CommitmentAbsolute
	case GoalPriorityHigh:
		return CommitmentStrong
	case GoalPriorityNormal:
		return CommitmentModerate
	case GoalPriorityLow:
		return CommitmentWeak
	default:
		return CommitmentModerate
	}
}
