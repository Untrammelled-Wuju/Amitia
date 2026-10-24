package interaction

import (
	"fmt"
	"time"
)

func validateCoordinationTimeBudget(now, goalExpiresAt time.Time, deadlineAt *time.Time) error {
	if !goalExpiresAt.IsZero() && !now.Before(goalExpiresAt) {
		return fmt.Errorf("multi_agent: parent goal has expired; worker dispatch is prohibited")
	}
	if deadlineAt != nil && !now.Before(*deadlineAt) {
		return fmt.Errorf("multi_agent: coordination deadline has expired; worker dispatch is prohibited")
	}
	return nil
}
