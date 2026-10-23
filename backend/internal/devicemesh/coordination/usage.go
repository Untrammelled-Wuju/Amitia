package coordination

import "context"

type BusinessUsageDay struct {
	Date          string `json:"date"`
	Messages      int64  `json:"messages"`
	Conversations int64  `json:"conversations"`
	Memories      int64  `json:"memories"`
	Feedback      int64  `json:"feedback"`
}

type BusinessUsage struct {
	Totals  BusinessUsageDay   `json:"totals"`
	OwnerID string             `json:"ownerId"`
	Daily   []BusinessUsageDay `json:"daily"`
}

type BusinessUsagePort interface {
	BusinessUsage(context.Context, ExecutionScope, string) (BusinessUsage, error)
}
