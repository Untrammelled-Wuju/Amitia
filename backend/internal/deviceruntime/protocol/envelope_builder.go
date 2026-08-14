package protocol

import (
	"fmt"
	"time"
)

type EnvelopeInput struct {
	Descriptor           Descriptor
	MessageType          MessageType
	MessageName          string
	MessageID            string
	CorrelationID        string
	CausationID          string
	Identity             SessionIdentity
	ConnectionGeneration int64
	Sequence             int64
	PayloadSchemaVersion int
	OccurredAt           time.Time
	SentAt               time.Time
	Payload              []byte
}

func BuildEnvelope(input EnvelopeInput) (*Envelope, error) {
	if input.MessageID == "" {
		return nil, fmt.Errorf("messageId is required")
	}
	sentAt := input.SentAt
	if sentAt.IsZero() {
		sentAt = time.Now()
	}
	env := &Envelope{
		EnvelopeVersion:      input.Descriptor.EnvelopeVersion,
		Protocol:             input.Descriptor.Name,
		MessageType:          input.MessageType,
		MessageName:          input.MessageName,
		MessageID:            input.MessageID,
		CorrelationID:        input.CorrelationID,
		CausationID:          input.CausationID,
		UserID:               input.Identity.UserID,
		DeviceID:             input.Identity.DeviceID,
		RuntimeID:            input.Identity.RuntimeID,
		RuntimeSessionID:     input.Identity.RuntimeSessionID,
		ConnectionGeneration: input.ConnectionGeneration,
		Sequence:             input.Sequence,
		PayloadSchemaVersion: input.PayloadSchemaVersion,
		PayloadHash:          ComputePayloadHash(input.Payload),
		OccurredAt:           input.OccurredAt,
		SentAt:               sentAt,
		Payload:              input.Payload,
	}
	return env, nil
}
