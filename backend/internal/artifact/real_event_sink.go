// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package artifact

import (
	"encoding/json"
	"log"
)

type ArtifactEvent struct {
	Type       string    `json:"type"`
	ArtifactID ID        `json:"artifactID"`
	OwnerID    string    `json:"ownerID"`
	Kind       Kind      `json:"kind"`
	SizeBytes  int64     `json:"sizeBytes"`
	MimeType   string    `json:"mimeType"`
	Filename   string    `json:"filename"`
}

type RealEventSink struct {
	publisher EventPublisher
}

type EventPublisher interface {
	Publish(eventType string, payload []byte) error
}

func NewRealEventSink(publisher EventPublisher) *RealEventSink {
	return &RealEventSink{publisher: publisher}
}

func (s *RealEventSink) ArtifactCreated(artifact *Artifact) {
	if s.publisher == nil {
		return
	}
	event := ArtifactEvent{
		Type:       "artifact.created",
		ArtifactID: artifact.ID,
		OwnerID:    artifact.OwnerUserID,
		Kind:       artifact.Kind,
		SizeBytes:  artifact.SizeBytes,
		MimeType:   artifact.MIMEType,
		Filename:   artifact.Filename,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("artifact: marshal created event: %v", err)
		return
	}
	if err := s.publisher.Publish("artifact.created", payload); err != nil {
		log.Printf("artifact: publish created event: %v", err)
	}
}

func (s *RealEventSink) ArtifactDeleted(artifact *Artifact) {
	if s.publisher == nil {
		return
	}
	event := ArtifactEvent{
		Type:       "artifact.deleted",
		ArtifactID: artifact.ID,
		OwnerID:    artifact.OwnerUserID,
		Kind:       artifact.Kind,
		SizeBytes:  artifact.SizeBytes,
		MimeType:   artifact.MIMEType,
		Filename:   artifact.Filename,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("artifact: marshal deleted event: %v", err)
		return
	}
	if err := s.publisher.Publish("artifact.deleted", payload); err != nil {
		log.Printf("artifact: publish deleted event: %v", err)
	}
}

type LoggingEventPublisher struct{}

func (p *LoggingEventPublisher) Publish(eventType string, payload []byte) error {
	log.Printf("artifact: event %s: %s", eventType, string(payload))
	return nil
}
