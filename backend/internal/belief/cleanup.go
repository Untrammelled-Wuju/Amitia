package belief

import (
	"sort"
	"time"
)

type CleanupInput struct {
	Candidates []Candidate
	Now        time.Time
}

type CleanupResult struct {
	Removed   int
	Alive     int
	Expired   []string
}

func CleanupExpiredBeliefs(input CleanupInput) CleanupResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	alive := make([]Candidate, 0, len(input.Candidates))
	expired := make([]string, 0)
	for _, c := range input.Candidates {
		if !c.ExpiresAt.IsZero() && !c.ExpiresAt.After(now) {
			if c.ID != "" {
				expired = append(expired, c.ID)
			}
			continue
		}
		alive = append(alive, c)
	}
	sort.Strings(expired)
	return CleanupResult{
		Removed: len(expired),
		Alive:   len(alive),
		Expired: expired,
	}
}
