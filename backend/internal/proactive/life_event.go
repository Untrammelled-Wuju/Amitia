package proactive

import "time"

type LifeEventType string

const (
	LifeEventTypeLifeChange  LifeEventType = "life_change"
	LifeEventTypeTimeChange  LifeEventType = "time_change"
	LifeEventTypeStateChange LifeEventType = "state_change"
)

type LifeEvent struct {
	Type           LifeEventType `json:"type"`
	Timestamp      time.Time     `json:"timestamp"`
	CharacterID    string        `json:"characterId"`
	InfluenceScore float64       `json:"influenceScore"`
}
