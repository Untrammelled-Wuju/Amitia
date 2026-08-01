package v2

import (
	"time"
)

type ResultCommitter struct {
	commands CommandService
	events   EventService
	states   ActualStateService
}

func NewResultCommitter(commands CommandService, events EventService, states ActualStateService) *ResultCommitter {
	return &ResultCommitter{
		commands: commands,
		events:   events,
		states:   states,
	}
}

func (c *ResultCommitter) CommitResult(result *CommandResult, eventType string, payload []byte, t time.Time) error {
	if result == nil {
		return nil
	}

	if err := c.commands.SaveResult(result); err != nil {
		return err
	}

	seq := int64(0)
	evtSeq, _ := c.events.GetLatestEventSeq(result.RuntimeSessionID)
	if evtSeq > 0 {
		seq = evtSeq + 1
	} else {
		seq = 1
	}

	if _, err := c.events.Append(eventType, payload, result.RuntimeSessionID, seq, TriggerSourceRuntimeCommand, &result.CommandID); err != nil {
		return err
	}

	return nil
}

func (c *ResultCommitter) MarkCompleted(commandID, playbackID string, sessionID string, t time.Time) error {
	if err := c.commands.MarkCompleted(commandID, playbackID, t); err != nil {
		return err
	}

	return nil
}

func (c *ResultCommitter) MarkFailed(commandID, errCode, errMsg string, sessionID string, t time.Time) error {
	if err := c.commands.MarkFailed(commandID, errCode, errMsg, t); err != nil {
		return err
	}

	return nil
}
