package modelerror

import "sync"

type Event struct {
	ModelType      string
	ConversationID string
	RequestID      string
	Channel        string
	RawError       string
}

type Reporter func(Event)

var reporterMu sync.RWMutex
var reporter Reporter

func SetReporter(next Reporter) {
	reporterMu.Lock()
	reporter = next
	reporterMu.Unlock()
}

func Report(event Event) {
	reporterMu.RLock()
	current := reporter
	reporterMu.RUnlock()
	if current != nil && event.ConversationID != "" && event.RawError != "" {
		current(event)
	}
}
