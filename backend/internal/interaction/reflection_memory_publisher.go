package interaction

import (
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/outbox"
)

const (
	ReflectionMemoryAbstractionEventType = "reflection.memory_abstraction.created"
	ReflectionCandidateApprovedEventType = "reflection.candidate.approved"
)

type reflectionMemoryCreator interface {
	CreateReflectionMemory(req ReflectionMemoryCreateRequest) error
}

type ReflectionMemoryCreateRequest struct {
	CharacterID           string
	MemoryType            string
	Key                   string
	Value                 string
	Importance            int
	Confidence            int
	SourceMsgID           string
	SourceConvID          string
	VerifiedStatus        string
	Source                string
	SensitivityLevel      string
	AllowProactiveMention bool
	RequiresConfirmation  bool
	Scope                 string
}

type ReflectionMemoryPublisher struct {
	memory reflectionMemoryCreator
	next   outbox.Publisher
}

type reflectionMemoryPayload struct {
	CandidateID        string                         `json:"candidateId"`
	CharacterID        string                         `json:"characterId"`
	ConversationID     string                         `json:"conversationId"`
	RequestID          string                         `json:"requestId"`
	Confidence         float64                        `json:"confidence"`
	Topic              string                         `json:"topic"`
	Abstract           string                         `json:"abstract"`
	SourceIDs          []string                       `json:"sourceIds"`
	Abstraction        *reflectionMemoryAbstraction   `json:"abstraction"`
	MemoryAbstractions []reflectionMemoryAbstraction  `json:"memoryAbstractions"`
	Candidate          *reflectionCandidateProjection `json:"candidate"`
}

type reflectionCandidateProjection struct {
	ID                 string                        `json:"id"`
	CharacterID        string                        `json:"characterId"`
	Confidence         float64                       `json:"confidence"`
	MemoryAbstractions []reflectionMemoryAbstraction `json:"memoryAbstractions"`
}

type reflectionMemoryAbstraction struct {
	SourceIDs []string `json:"sourceIds"`
	Topic     string   `json:"topic"`
	Abstract  string   `json:"abstract"`
}

func NewReflectionMemoryPublisher(memory reflectionMemoryCreator, next outbox.Publisher) *ReflectionMemoryPublisher {
	return &ReflectionMemoryPublisher{memory: memory, next: next}
}

func (p *ReflectionMemoryPublisher) Publish(record outbox.OutboxRecord) error {
	if !isReflectionMemoryEvent(record.EventType) {
		return publishNext(p.next, record)
	}
	if err := p.publishReflectionMemory(record); err != nil {
		return err
	}
	return publishNext(p.next, record)
}

func isReflectionMemoryEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case ReflectionMemoryAbstractionEventType, ReflectionCandidateApprovedEventType:
		return true
	default:
		return false
	}
}

func (p *ReflectionMemoryPublisher) publishReflectionMemory(record outbox.OutboxRecord) error {
	if p.memory == nil {
		return publishNext(p.next, record)
	}
	var payload reflectionMemoryPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		return err
	}
	characterID := firstNonEmpty(payload.CharacterID, nestedCharacterID(payload.Candidate))
	candidateID := firstNonEmpty(payload.CandidateID, nestedCandidateID(payload.Candidate), record.AggregateID)
	confidence := firstNonZero(payload.Confidence, nestedConfidence(payload.Candidate))
	abstractions := collectReflectionAbstractions(payload)
	for _, abstraction := range abstractions {
		req, ok := buildReflectionMemoryRequest(characterID, candidateID, payload.ConversationID, payload.RequestID, confidence, abstraction)
		if !ok {
			continue
		}
		if err := p.memory.CreateReflectionMemory(req); err != nil {
			return err
		}
	}
	return nil
}

func collectReflectionAbstractions(payload reflectionMemoryPayload) []reflectionMemoryAbstraction {
	abstractions := make([]reflectionMemoryAbstraction, 0)
	if payload.Abstraction != nil {
		abstractions = append(abstractions, *payload.Abstraction)
	}
	if strings.TrimSpace(payload.Topic) != "" || strings.TrimSpace(payload.Abstract) != "" || len(payload.SourceIDs) > 0 {
		abstractions = append(abstractions, reflectionMemoryAbstraction{
			Topic:     payload.Topic,
			Abstract:  payload.Abstract,
			SourceIDs: payload.SourceIDs,
		})
	}
	abstractions = append(abstractions, payload.MemoryAbstractions...)
	if payload.Candidate != nil {
		abstractions = append(abstractions, payload.Candidate.MemoryAbstractions...)
	}
	return abstractions
}

func buildReflectionMemoryRequest(characterID, candidateID, conversationID, requestID string, confidence float64, abstraction reflectionMemoryAbstraction) (ReflectionMemoryCreateRequest, bool) {
	topic := strings.TrimSpace(abstraction.Topic)
	abstract := strings.TrimSpace(abstraction.Abstract)
	if characterID == "" || topic == "" || abstract == "" {
		return ReflectionMemoryCreateRequest{}, false
	}
	return ReflectionMemoryCreateRequest{
		CharacterID:           characterID,
		MemoryType:            "reflection",
		Key:                   topic,
		Value:                 abstract,
		Importance:            reflectionImportance(confidence),
		Confidence:            reflectionConfidence(confidence),
		SourceMsgID:           firstNonEmpty(candidateID, requestID),
		SourceConvID:          conversationID,
		VerifiedStatus:        "verified",
		Source:                "reflection",
		SensitivityLevel:      "internal",
		AllowProactiveMention: true,
		RequiresConfirmation:  false,
		Scope:                 "character",
	}, true
}

func reflectionImportance(confidence float64) int {
	if confidence <= 0 {
		return 5
	}
	value := int(confidence*10 + 0.5)
	if value < 1 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}

func reflectionConfidence(confidence float64) int {
	if confidence <= 0 {
		return 70
	}
	value := int(confidence*100 + 0.5)
	if value < 1 {
		return 1
	}
	if value > 100 {
		return 100
	}
	return value
}

func nestedCharacterID(candidate *reflectionCandidateProjection) string {
	if candidate == nil {
		return ""
	}
	return candidate.CharacterID
}

func nestedCandidateID(candidate *reflectionCandidateProjection) string {
	if candidate == nil {
		return ""
	}
	return candidate.ID
}

func nestedConfidence(candidate *reflectionCandidateProjection) float64 {
	if candidate == nil {
		return 0
	}
	return candidate.Confidence
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func publishNext(next outbox.Publisher, record outbox.OutboxRecord) error {
	if next == nil {
		return nil
	}
	return next.Publish(record)
}
