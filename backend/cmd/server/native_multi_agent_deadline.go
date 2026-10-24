package main

import (
	"fmt"
	"time"
)

func validateMultiAgentDeadlineMinutes(minutes *int) error {
	if minutes != nil && (*minutes < 1 || *minutes > 240) {
		return fmt.Errorf("multi_agent: deadline_minutes must be between 1 and 240 when specified")
	}
	return nil
}

func multiAgentDeadlineFromMinutes(now time.Time, minutes int) time.Time {
	return now.Add(time.Duration(minutes) * time.Minute).UTC()
}
