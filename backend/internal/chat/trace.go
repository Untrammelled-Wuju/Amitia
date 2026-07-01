package chat

import (
	"strings"

	applog "github.com/u-ai/backend/log"
)

const chatTraceStateVersion = "chat-runtime-trace-v1"

func newProcessTrace(requestID, convID, charID, channel string) applog.TraceFields {
	requestID = strings.TrimSpace(requestID)
	return applog.TraceFields{
		RequestID:     requestID,
		CorrelationID: "corr-" + requestID,
		CausationID:   "cause-" + requestID,
		User:          "default",
		Character:     strings.TrimSpace(charID),
		Conversation:  strings.TrimSpace(convID),
		Channel:       strings.TrimSpace(channel),
		StateVersion:  chatTraceStateVersion,
		Path:          "chat.process_message",
	}
}

func updateProcessTraceScope(trace applog.TraceFields, convID, charID, channel string) applog.TraceFields {
	trace.Conversation = strings.TrimSpace(convID)
	trace.Character = strings.TrimSpace(charID)
	trace.Channel = strings.TrimSpace(channel)
	return trace
}
