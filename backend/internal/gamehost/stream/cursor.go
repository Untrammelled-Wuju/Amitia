package stream

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type StreamGeneration string

const StreamGenerationZero StreamGeneration = ""

type Cursor struct {
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  domain.ServiceID
	ChannelID  domain.ChannelID
	Generation StreamGeneration
	Sequence   Sequence
}

func NewCursor(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID, generation StreamGeneration, seq Sequence) Cursor {
	return Cursor{
		RuntimeID:  runtimeID,
		ServiceID:  serviceID,
		ChannelID:  channelID,
		Generation: generation,
		Sequence:   seq,
	}
}

func (c Cursor) Validate() error {
	if c.RuntimeID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "cursor: runtime id must not be empty")
	}
	if c.ServiceID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "cursor: service id must not be empty")
	}
	if c.ChannelID == "" {
		return ErrWrongChannel
	}
	return nil
}

func (c Cursor) Key() string {
	return string(c.RuntimeID) + "/" + string(c.ServiceID) + "/" + string(c.ChannelID)
}

func (c Cursor) Equals(other Cursor) bool {
	return c.RuntimeID == other.RuntimeID &&
		c.ServiceID == other.ServiceID &&
		c.ChannelID == other.ChannelID
}

func ValidateCursorScope(c Cursor, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.RuntimeID != runtimeID {
		return ErrWrongRuntime
	}
	if c.ServiceID != serviceID {
		return ErrWrongService
	}
	if c.ChannelID != channelID {
		return ErrWrongChannel
	}
	return nil
}

func ValidateCursorGeneration(c Cursor, expected StreamGeneration) bool {
	return c.Generation == expected
}
