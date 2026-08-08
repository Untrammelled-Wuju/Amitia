package decision

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type GoalType string

const (
	GoalTypeConnection     GoalType = "connection"
	GoalTypeSupport        GoalType = "support"
	GoalTypeGrowth         GoalType = "growth"
	GoalTypeAutonomy       GoalType = "autonomy"
	GoalTypeClarification  GoalType = "clarification"
	GoalTypeConflictRepair GoalType = "conflict_repair"
	GoalTypeInformation    GoalType = "information"
)

type GoalPriority string

const (
	GoalPriorityLow      GoalPriority = "low"
	GoalPriorityNormal   GoalPriority = "normal"
	GoalPriorityHigh     GoalPriority = "high"
	GoalPriorityCritical GoalPriority = "critical"
)

type GoalStatus string

const (
	GoalStatusPending   GoalStatus = "pending"
	GoalStatusActive    GoalStatus = "active"
	GoalStatusSuspended GoalStatus = "suspended"
	GoalStatusAchieved  GoalStatus = "achieved"
	GoalStatusAbandoned GoalStatus = "abandoned"
	GoalStatusWish      GoalStatus = "wish"
)

type GoalTriggerKind string

const (
	GoalTriggerUserMessage GoalTriggerKind = "user_message"
	GoalTriggerVoice       GoalTriggerKind = "voice"
	GoalTriggerProactive   GoalTriggerKind = "proactive"
	GoalTriggerInternal    GoalTriggerKind = "internal"
	GoalTriggerRecovery    GoalTriggerKind = "recovery"
)

type GoalTrigger struct {
	Kind          GoalTriggerKind `json:"kind"`
	Source        string          `json:"source,omitempty"`
	RequestID     string          `json:"requestId,omitempty"`
	InteractionID string          `json:"interactionId,omitempty"`
	SessionID     string          `json:"sessionId,omitempty"`
}

type Goal struct {
	ID             string         `json:"id"`
	UserID         string         `json:"userId,omitempty"`
	CharacterID    string         `json:"characterId,omitempty"`
	ConversationID string         `json:"conversationId,omitempty"`
	Type           GoalType       `json:"type"`
	Priority       GoalPriority   `json:"priority"`
	Status         GoalStatus     `json:"status"`
	Progress       float64        `json:"progress"`
	Description    string         `json:"description,omitempty"`
	Revision       int64          `json:"revision"`
	Trigger        GoalTrigger    `json:"trigger"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	ExpiresAt      time.Time      `json:"expiresAt,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type GoalCreateRequest struct {
	UserID         string
	CharacterID    string
	ConversationID string
	Type           GoalType
	Priority       GoalPriority
	Description    string
	Trigger        GoalTrigger
	ExpiresAt      time.Time
	Metadata       map[string]any
}

func NewGoal(request GoalCreateRequest, now time.Time) Goal {
	return Goal{
		ID:             uuid.NewString(),
		UserID:         request.UserID,
		CharacterID:    request.CharacterID,
		ConversationID: request.ConversationID,
		Type:           request.Type,
		Priority:       request.Priority,
		Status:         GoalStatusActive,
		Progress:       0,
		Description:    request.Description,
		Revision:       1,
		Trigger:        request.Trigger,
		CreatedAt:      now,
		UpdatedAt:      now,
		ExpiresAt:      request.ExpiresAt,
		Metadata:       request.Metadata,
	}
}

type Wish struct {
	Goal
	StagnantSince time.Time `json:"stagnantSince"`
	ReviewCount   int       `json:"reviewCount"`
}

type GoalRegistry struct {
	mu    sync.RWMutex
	goals map[string]Goal
}

func NewGoalRegistry() *GoalRegistry {
	return &GoalRegistry{
		goals: make(map[string]Goal),
	}
}

func (r *GoalRegistry) Register(goal Goal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goals[goal.ID] = goal
	return nil
}

func (r *GoalRegistry) Get(id string) (Goal, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.goals[id]
	return g, ok
}

func (r *GoalRegistry) UpdateStatus(id string, status GoalStatus, progress float64) bool {
	return r.UpdateStatusAt(id, status, progress, time.Now().UTC())
}

func (r *GoalRegistry) UpdateStatusAt(id string, status GoalStatus, progress float64, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	goal, ok := r.goals[id]
	if !ok {
		return false
	}
	if !canTransitionGoalStatus(goal.Status, status) {
		return false
	}
	goal.Status = status
	if progress >= 0 && progress <= 1 {
		goal.Progress = progress
	}
	goal.Revision++
	goal.UpdatedAt = now
	r.goals[id] = goal
	return true
}

func (r *GoalRegistry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.goals[id]
	if ok {
		delete(r.goals, id)
	}
	return ok
}

func (r *GoalRegistry) ByUser(userID string) []Goal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Goal, 0)
	for _, g := range r.goals {
		if g.UserID == userID {
			result = append(result, g)
		}
	}
	sortGoalsStable(result)
	return result
}

func (r *GoalRegistry) Active() []Goal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Goal, 0)
	for _, g := range r.goals {
		if g.Status == GoalStatusActive || g.Status == GoalStatusPending {
			result = append(result, g)
		}
	}
	sortGoalsStable(result)
	return result
}

func (r *GoalRegistry) ActiveForScope(userID string, characterID string, conversationID string) []Goal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Goal, 0)
	for _, g := range r.goals {
		if g.UserID != userID {
			continue
		}
		if characterID != "" && g.CharacterID != "" && g.CharacterID != characterID {
			continue
		}
		if g.Status != GoalStatusActive && g.Status != GoalStatusPending && g.Status != GoalStatusWish {
			continue
		}
		if conversationID != "" && g.ConversationID != "" && g.ConversationID != conversationID {
			continue
		}
		result = append(result, g)
	}
	sortGoalsStable(result)
	return result
}

func (r *GoalRegistry) Wishes() []Wish {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Wish, 0)
	for _, g := range r.goals {
		if g.Status == GoalStatusWish {
			w := Wish{
				Goal:          g,
				StagnantSince: g.UpdatedAt,
				ReviewCount:   0,
			}
			result = append(result, w)
		}
	}
	return result
}

func (r *GoalRegistry) ExpireStale(now time.Time) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := make([]string, 0)
	for id, g := range r.goals {
		if !g.ExpiresAt.IsZero() && now.After(g.ExpiresAt) {
			delete(r.goals, id)
			removed = append(removed, id)
		}
	}
	return removed
}

func (r *GoalRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.goals)
}

func (r *GoalRegistry) PromoteToWish(goalID string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.goals[goalID]
	if !ok {
		return false
	}
	g.Status = GoalStatusWish
	g.UpdatedAt = now
	r.goals[goalID] = g
	return true
}

func sortGoalsStable(goals []Goal) {
	sort.SliceStable(goals, func(i, j int) bool {
		pi := priorityOrder(goals[i].Priority)
		pj := priorityOrder(goals[j].Priority)
		if pi != pj {
			return pi < pj
		}
		if !goals[i].CreatedAt.Equal(goals[j].CreatedAt) {
			return goals[i].CreatedAt.Before(goals[j].CreatedAt)
		}
		return goals[i].ID < goals[j].ID
	})
}

func priorityOrder(p GoalPriority) int {
	switch p {
	case GoalPriorityCritical:
		return 0
	case GoalPriorityHigh:
		return 1
	case GoalPriorityNormal:
		return 2
	case GoalPriorityLow:
		return 3
	default:
		return 4
	}
}

func isTerminalGoalStatus(status GoalStatus) bool {
	return status == GoalStatusAchieved || status == GoalStatusAbandoned
}

func canTransitionGoalStatus(from GoalStatus, to GoalStatus) bool {
	if from == GoalStatusAchieved {
		return false
	}
	if from == GoalStatusAbandoned {
		return false
	}
	return true
}
