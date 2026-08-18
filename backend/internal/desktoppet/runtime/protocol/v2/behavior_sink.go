package v2

import (
	"fmt"
	"time"
)

type BehaviorEventSink struct {
	states ActualStateService
	events EventService
}

func NewBehaviorEventSink(states ActualStateService, events EventService) *BehaviorEventSink {
	return &BehaviorEventSink{
		states: states,
		events: events,
	}
}

func (s *BehaviorEventSink) UpdateActualState(state *RuntimeActualState) error {
	if state == nil {
		return nil
	}
	return s.states.Upsert(state)
}

func (s *BehaviorEventSink) RecordHealthChange(runtimeSessionID, previousStatus, currentStatus, reason string) error {
	now := time.Now().Format("2006-01-02 15:04:05")

	seq := int64(0)
	evtSeq, err := s.events.GetLatestEventSeq(runtimeSessionID)
	if err != nil {
		return err
	}
	if evtSeq > 0 {
		seq = evtSeq + 1
	} else {
		seq = 1
	}

	payload := []byte(`{"previousStatus":"` + previousStatus + `","currentStatus":"` + currentStatus + `","reason":"` + reason + `","changedAt":"` + now + `"}`)

	_, err = s.events.Append(EventHealthChanged, payload, runtimeSessionID, seq, TriggerSourceRuntimeCommand, nil)
	return err
}

func (s *BehaviorEventSink) RecordPlaybackEvent(runtimeSessionID, cmdID, eventType string, payload []byte) error {
	seq := int64(0)
	evtSeq, err := s.events.GetLatestEventSeq(runtimeSessionID)
	if err != nil {
		return err
	}
	if evtSeq > 0 {
		seq = evtSeq + 1
	} else {
		seq = 1
	}

	var commandID *string
	if cmdID != "" {
		commandID = &cmdID
	}

	_, err = s.events.Append(eventType, payload, runtimeSessionID, seq, TriggerSourceRuntimeCommand, commandID)
	return err
}

func (s *BehaviorEventSink) RecordDesiredApplied(runtimeSessionID, cmdID string, desiredRevision int64) error {
	now := time.Now().Format(time.RFC3339)

	seq := int64(0)
	evtSeq, err := s.events.GetLatestEventSeq(runtimeSessionID)
	if err != nil {
		return err
	}
	if evtSeq > 0 {
		seq = evtSeq + 1
	} else {
		seq = 1
	}

	payload := []byte(`{"desiredRevision":` + fmt.Sprintf("%d", desiredRevision) + `,"appliedAt":"` + now + `"}`)

	var commandID *string
	if cmdID != "" {
		commandID = &cmdID
	}

	_, err = s.events.Append(EventStateDesiredApplied, payload, runtimeSessionID, seq, TriggerSourceRuntimeCommand, commandID)
	return err
}
