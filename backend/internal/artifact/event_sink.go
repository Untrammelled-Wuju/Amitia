package artifact

type EventSink interface {
	ArtifactCreated(artifact *Artifact)
	ArtifactDeleted(artifact *Artifact)
}

type noopEventSink struct{}

func (noopEventSink) ArtifactCreated(artifact *Artifact) {}
func (noopEventSink) ArtifactDeleted(artifact *Artifact) {}

var _ EventSink = (*noopEventSink)(nil)
