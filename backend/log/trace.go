package log

import (
	"strings"

	"github.com/sirupsen/logrus"
)

type Fields map[string]interface{}

type TraceFields struct {
	RequestID     string
	CorrelationID string
	CausationID   string
	User          string
	Character     string
	Conversation  string
	Channel       string
	StateVersion  string
	Path          string
	Stage         string
}

func (t TraceFields) Clone() TraceFields {
	return t
}

func (t TraceFields) WithStage(stage string) TraceFields {
	cp := t.Clone()
	cp.Stage = stage
	return cp
}

func (t TraceFields) toFields(extra Fields) logrus.Fields {
	fields := logrus.Fields{
		"request_id":     strings.TrimSpace(t.RequestID),
		"correlation_id": strings.TrimSpace(t.CorrelationID),
		"causation_id":   strings.TrimSpace(t.CausationID),
		"user":           strings.TrimSpace(t.User),
		"character":      strings.TrimSpace(t.Character),
		"conversation":   strings.TrimSpace(t.Conversation),
		"channel":        strings.TrimSpace(t.Channel),
		"state_version":  strings.TrimSpace(t.StateVersion),
		"path":           strings.TrimSpace(t.Path),
		"stage":          strings.TrimSpace(t.Stage),
	}
	for key, value := range extra {
		fields[key] = value
	}
	return fields
}

func TraceInfo(trace TraceFields, extra Fields, message string) {
	WithFields(trace.toFields(extra)).Info(message)
}

func TraceWarn(trace TraceFields, extra Fields, message string) {
	WithFields(trace.toFields(extra)).Warn(message)
}

func TraceError(trace TraceFields, extra Fields, err error, message string) {
	fields := trace.toFields(extra)
	if err != nil {
		fields["error"] = err.Error()
	}
	WithFields(fields).Error(message)
}
