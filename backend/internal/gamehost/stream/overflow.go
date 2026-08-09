package stream

type OverflowHandler struct {
	policy OverflowPolicy
}

func NewOverflowHandler(policy OverflowPolicy) *OverflowHandler {
	return &OverflowHandler{policy: policy}
}

func (h *OverflowHandler) Apply(queue *BoundedQueue, entry QueueEntry) (*BoundedQueue, OverflowResult) {
	switch h.policy {
	case OverflowReject:
		return queue, OverflowResult{Action: OverflowActionReject, Entry: entry}
	case OverflowDropOldest:
		if queue.Len() >= queue.Cap() {
			oldest := queue.PopOldest()
			queue.Push(entry)
			return queue, OverflowResult{Action: OverflowActionDropOldest, Dropped: oldest, Entry: entry}
		}
		queue.Push(entry)
		return queue, OverflowResult{Action: OverflowActionNone, Entry: entry}
	case OverflowDropNewest:
		if queue.Len() >= queue.Cap() {
			return queue, OverflowResult{Action: OverflowActionDropNewest, Dropped: entry}
		}
		queue.Push(entry)
		return queue, OverflowResult{Action: OverflowActionNone, Entry: entry}
	case OverflowCoalesce:
		queue.Push(entry)
		return queue, OverflowResult{Action: OverflowActionCoalesce, Entry: entry}
	case OverflowBlock:
		queue.Push(entry)
		return queue, OverflowResult{Action: OverflowActionNone, Entry: entry}
	default:
		return queue, OverflowResult{Action: OverflowActionReject, Entry: entry}
	}
}

type OverflowAction string

const (
	OverflowActionNone       OverflowAction = "none"
	OverflowActionReject     OverflowAction = "reject"
	OverflowActionDropOldest OverflowAction = "drop_oldest"
	OverflowActionDropNewest OverflowAction = "drop_newest"
	OverflowActionCoalesce   OverflowAction = "coalesce"
)

type OverflowResult struct {
	Action  OverflowAction
	Entry   QueueEntry
	Dropped QueueEntry
}

func (r OverflowResult) IsRejected() bool {
	return r.Action == OverflowActionReject || r.Action == OverflowActionDropNewest
}

func (r OverflowResult) IsDropped() bool {
	return r.Action == OverflowActionDropOldest || r.Action == OverflowActionDropNewest
}

func (r OverflowResult) IsCoalesced() bool {
	return r.Action == OverflowActionCoalesce
}
