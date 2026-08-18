package v2

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Reconciler struct {
	sessions SessionService
	commands CommandService
	events   EventService
	states   ActualStateService
	handler  *Handler
}

func NewReconciler(services *Services, handler *Handler) *Reconciler {
	return &Reconciler{
		sessions: services.Sessions,
		commands: services.Commands,
		events:   services.Events,
		states:   services.ActualStates,
		handler:  handler,
	}
}

func (r *Reconciler) ReconcileSession(ctx context.Context, sessionID string, desiredRevision int64) error {
	session, err := r.sessions.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("get session failed: %v", err)
	}

	if session.LastAppliedDesiredRevision >= desiredRevision {
		return nil
	}

	conn := r.handler.GetConnection(session.UserID, session.DeviceID, session.RuntimeID)
	if conn == nil {
		return errors.New("connection not found")
	}

	if err := r.sessions.UpdateLastAppliedRevision(sessionID, desiredRevision); err != nil {
		return fmt.Errorf("update revision failed: %v", err)
	}

	return nil
}

func (r *Reconciler) ExpireCommands(now time.Time) (int64, error) {
	expired := int64(0)
	cmds, err := r.commands.ListExpiredCommands(100, 60)
	if err != nil {
		return 0, err
	}

	for _, cmd := range cmds {
		if cmd.Status == string(CommandStatusExpired) {
			continue
		}
		if err := r.commands.MarkExpired(cmd.ID, now); err != nil {
			return expired, fmt.Errorf("mark command %s expired: %w", cmd.ID, err)
		}
		expired++

		payload, err := marshalJSON(map[string]interface{}{
			"commandId": cmd.ID,
			"expiredAt": now.Format(time.RFC3339),
		})
		if err != nil {
			return expired, fmt.Errorf("marshal command timeout event: %w", err)
		}
		evtSeq, err := r.events.GetLatestEventSeq(cmd.RuntimeSessionID)
		if err != nil {
			return expired, fmt.Errorf("load latest event sequence: %w", err)
		}
		seq := evtSeq + 1
		if seq < 1 {
			seq = 1
		}
		if _, err := r.events.Append(EventCommandTimeout, payload, cmd.RuntimeSessionID, seq, TriggerSourceSystemRecovery, &cmd.ID); err != nil {
			return expired, fmt.Errorf("append command timeout event: %w", err)
		}
	}

	return expired, nil
}
