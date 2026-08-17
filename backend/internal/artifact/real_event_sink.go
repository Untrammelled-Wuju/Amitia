package artifact

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type ArtifactEvent struct {
	Type       string `json:"type"`
	ArtifactID ID     `json:"artifactID"`
	OwnerID    string `json:"ownerID"`
	Kind       Kind   `json:"kind"`
	SizeBytes  int64  `json:"sizeBytes"`
	MimeType   string `json:"mimeType"`
	Filename   string `json:"filename"`
}

type RealEventSink struct {
	publisher event.DurableEventPublisher
}

func NewRealEventSink(publisher event.DurableEventPublisher) *RealEventSink {
	return &RealEventSink{publisher: publisher}
}

func (s *RealEventSink) PublishCreated(ctx context.Context, tx *sql.Tx, artifact *Artifact) error {
	if s.publisher == nil {
		return nil
	}
	evt := ArtifactEvent{
		Type:       "artifact.created",
		ArtifactID: artifact.ID,
		OwnerID:    artifact.OwnerUserID,
		Kind:       artifact.Kind,
		SizeBytes:  artifact.SizeBytes,
		MimeType:   artifact.MIMEType,
		Filename:   artifact.Filename,
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = s.publisher.PublishTx(ctx, tx, "artifact.created", 1, payload, event.PublishOptions{
		ProducerID:    "artifact-service",
		ProducerType:  event.EventProducerTypeSystem,
		AggregateType: "artifact",
		AggregateID:   string(artifact.ID),
		PartitionKey:  artifact.OwnerUserID,
	})
	return err
}

func (s *RealEventSink) PublishDeleted(ctx context.Context, tx *sql.Tx, artifact *Artifact) error {
	if s.publisher == nil {
		return nil
	}
	evt := ArtifactEvent{
		Type:       "artifact.deleted",
		ArtifactID: artifact.ID,
		OwnerID:    artifact.OwnerUserID,
		Kind:       artifact.Kind,
		SizeBytes:  artifact.SizeBytes,
		MimeType:   artifact.MIMEType,
		Filename:   artifact.Filename,
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = s.publisher.PublishTx(ctx, tx, "artifact.deleted", 1, payload, event.PublishOptions{
		ProducerID:    "artifact-service",
		ProducerType:  event.EventProducerTypeSystem,
		AggregateType: "artifact",
		AggregateID:   string(artifact.ID),
		PartitionKey:  artifact.OwnerUserID,
	})
	return err
}
